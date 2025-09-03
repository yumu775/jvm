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
1. 删除 Java 安装目录
2. 可选择清理下载文件
3. 可选择清理配置文件中的引用
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

// uninstallCleanCmd 清理孤立的文件和配置
var uninstallCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "清理孤立的下载文件和配置",
	Long: `清理不再需要的下载文件和配置引用。

这个命令会：
1. 删除没有对应安装版本的下载文件
2. 清理配置文件中的无效引用
3. 清理空的目录`,
	RunE: func(cmd *cobra.Command, args []string) error {
		color.Blue("Cleaning up orphaned files and configurations...")
		
		// TODO: 实现清理逻辑
		color.Yellow("Clean functionality not yet implemented")
		
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
	
	// 添加标志
	uninstallCmd.Flags().BoolVarP(&forceUninstall, "force", "f", false, "强制卸载，即使是当前激活版本")
	uninstallCmd.Flags().BoolVar(&keepDownloads, "keep-downloads", false, "保留下载文件")
	uninstallCmd.Flags().BoolVar(&cleanConfig, "clean-config", false, "清理配置文件中的引用")
	uninstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "预览将要删除的内容，不实际删除")
}
