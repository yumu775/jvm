package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/uninstall"
)

var (
	forceUninstall bool
	keepDownloads  bool
	cleanConfig    bool
	dryRun         bool
)

// uninstallCmd 定义了 "jvm uninstall" 命令
var uninstallCmd = &cobra.Command{
	Use:   "uninstall <version>",
	Short: "卸载指定版本的 Java",
	Long: `卸载指定版本的 Java。

这个命令会：
1. 删除由 JVM 安装的 Java 目录；外部导入只取消登记
2. 保留缺少可靠归属信息的旧下载缓存
3. 清理配置文件中的版本引用
4. 如果卸载的是当前版本，会清除当前版本设置

示例：
  jvm uninstall 17                    # 卸载 Java 17
  jvm uninstall 11 --force           # 强制卸载，即使是当前版本
  jvm uninstall 8 --clean-config     # 卸载并清理配置
  jvm uninstall 17 --dry-run         # 预览将要删除的内容`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		version := args[0]

		// 创建卸载管理器
		uninstallManager, err := uninstall.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize uninstall manager: %w", err)
		}

		// 配置卸载选项
		options := uninstall.UninstallOptions{
			Version:       version,
			Force:         forceUninstall,
			KeepDownloads: keepDownloads,
			CleanConfig:   cleanConfig,
			DryRun:        dryRun,
		}

		// 执行卸载
		_, err = uninstallManager.Uninstall(options)
		if err != nil {
			return fmt.Errorf("failed to uninstall Java %s: %w", version, err)
		}

		// 如果不是 dry run，显示后续建议
		if !dryRun {
			fmt.Println()
			color.Cyan("Recommendations:")

			// 检查是否还有其他版本
			versions, err := uninstallManager.ListUninstallableVersions()
			if err == nil && len(versions) > 0 {
				fmt.Printf("  Available versions: ")
				var versionNames []string
				for _, v := range versions {
					versionNames = append(versionNames, v.Version)
				}
				fmt.Printf("%s\n", strings.Join(versionNames, ", "))
				fmt.Printf("  Use 'jvm use <version>' to activate one\n")
			} else {
				fmt.Printf("  No Java versions remaining\n")
				fmt.Printf("  Use 'jvm install <version>' to install a new version\n")
			}
		}

		return nil
	},
}

// uninstallListCmd 列出可卸载的版本
var uninstallListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出可卸载的 Java 版本",
	Long: `列出所有已安装且可以卸载的 Java 版本。

显示每个版本的安装路径和大小信息。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建卸载管理器
		uninstallManager, err := uninstall.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize uninstall manager: %w", err)
		}

		// 获取可卸载的版本
		versions, err := uninstallManager.ListUninstallableVersions()
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}

		if len(versions) == 0 {
			color.Yellow("No Java versions installed.")
			fmt.Println("Use 'jvm install <version>' to install a Java version.")
			return nil
		}

		color.Blue("Installed Java versions:")
		fmt.Println()

		for _, version := range versions {
			// 获取卸载信息
			info, err := uninstallManager.GetUninstallInfo(version.Version)
			if err != nil {
				color.Red("  %s - Error getting info: %v", version.Version, err)
				continue
			}

			status := ""
			if version.Current {
				status = color.GreenString(" (current)")
			}

			fmt.Printf("  %s%s\n", version.Version, status)
			fmt.Printf("    Path: %s\n", version.Path)
			fmt.Printf("    Files: %d, Size: %s\n", len(info.RemovedFiles), formatBytes(info.BytesFreed))
			fmt.Println()
		}

		color.Cyan("Use 'jvm uninstall <version>' to remove a specific version")
		color.Cyan("Use 'jvm uninstall <version> --dry-run' to preview removal")

		return nil
	},
}

// uninstallCleanCmd 只清理确认失效的配置引用，保留磁盘文件。
var uninstallCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "清理确认不存在的安装登记与失效版本引用",
	Long: `只清理配置：删除已确认路径不存在的安装登记，并清除失效 current/default 引用。
权限错误、正在安装的目标及无法确认的路径会保留并说明原因。
不删除外部 Java、安装目录或归属不明的旧下载缓存，不修改系统环境。
使用 --dry-run 预览。`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("dry-run")
		result, err := uninstall.CleanupReferences(preview)
		if err != nil {
			return err
		}
		prefix := "Removed"
		if preview {
			prefix = "Would remove"
		}
		for _, item := range result.RemovedRegistrations {
			fmt.Fprintf(cmd.OutOrStdout(), "%s registration: %s\n", prefix, item)
		}
		for _, item := range result.ClearedReferences {
			fmt.Fprintf(cmd.OutOrStdout(), "%s reference: %s\n", prefix, item)
		}
		for _, reason := range result.Retained {
			fmt.Fprintf(cmd.OutOrStdout(), "Retained: %s\n", reason)
		}
		if len(result.RemovedRegistrations) == 0 && len(result.ClearedReferences) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No confirmed stale configuration references found.")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Installation files and external Java directories retained: cleanup only changes configuration references.")
		fmt.Fprintln(cmd.OutOrStdout(), "Download and metadata caches retained: legacy cache ownership cannot be reliably inferred.")
		fmt.Fprintln(cmd.OutOrStdout(), "Existing terminal and persistent system environments are unchanged; use a valid Java version to refresh them.")
		if preview {
			fmt.Fprintln(cmd.OutOrStdout(), "Dry run: configuration unchanged.")
		}
		return nil
	},
}

// formatBytes 格式化字节数为人类可读格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// init 函数初始化 uninstall 命令和子命令
func init() {
	// 添加子命令
	uninstallCmd.AddCommand(uninstallListCmd)
	uninstallCmd.AddCommand(uninstallCleanCmd)
	uninstallCleanCmd.Flags().Bool("dry-run", false, "预览失效登记和引用，不写配置")

	// 添加标志
	uninstallCmd.Flags().BoolVarP(&forceUninstall, "force", "f", false, "强制卸载，即使是当前激活版本")
	uninstallCmd.Flags().BoolVar(&keepDownloads, "keep-downloads", false, "保留下载文件")
	uninstallCmd.Flags().BoolVar(&cleanConfig, "clean-config", false, "清理配置文件中的引用")
	uninstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "预览将要删除的内容，不实际删除")
}
