package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/scanner"
	"jvm/internal/version"
)

var (
	showSystemVersions bool
	showAllVersions    bool
)

var scanListedSystemJava = func() ([]scanner.JavaInstallation, error) {
	return scanner.NewScanner().ScanSystemJava()
}

// listCmd 定义了 "jvm list" 命令
// 这个命令用于列出所有已安装的 Java 版本
var listCmd = &cobra.Command{
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	Use:     "list",
	Short:   "列出所有已安装的 Java 版本",
	Long: `列出所有已安装的 Java 版本。

已保存的版本选择会用绿色显示并标记为 "selected"；实际终端环境请查看 jvm env。
没有托管版本且 auto-scan 开启时，自动查找系统 Java 并提示导入。

示例：
  jvm list                    # 列出 JVM 管理的版本
  jvm list --system          # 同时显示系统中的 Java 版本
  jvm list --all             # 显示所有版本（管理的+系统的）`,
	// RunE 函数定义了命令的执行逻辑
	// 使用 RunE 而不是 Run 是因为我们需要返回错误
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建版本管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}

		// 获取 JVM 管理的版本列表
		managedVersions, err := manager.ListInstalled()
		if err != nil {
			return fmt.Errorf("failed to list installed versions: %w", err)
		}
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		scanSystem := showSystemVersions || showAllVersions || (len(managedVersions) == 0 && cfg.AutoScan)

		// 显示 JVM 管理的版本
		if len(managedVersions) > 0 {
			color.Blue("JVM Managed Versions:")
			for _, v := range managedVersions {
				if v.Current {
					color.Green("  * %s (selected)", v.Version)
				} else {
					fmt.Printf("    %s\n", v.Version)
				}
				fmt.Printf("      %s\n", v.Path)
			}
		} else {
			color.Yellow("No JVM managed versions found.")
		}

		// 如果需要显示系统版本
		if scanSystem {
			fmt.Println()

			// 扫描系统版本
			color.Blue("Scanning for system Java installations...")
			systemVersions, err := scanListedSystemJava()
			if err != nil {
				color.Yellow("Warning: failed to scan system versions: %v", err)
			} else if len(systemVersions) > 0 {
				color.Blue("System Java Installations:")
				for _, installation := range systemVersions {
					status := ""
					if installation.Source == "system" {
						status = color.CyanString(" (system)")
					}

					fmt.Printf("    %s%s\n", installation.Version, status)
					fmt.Printf("      Path: %s\n", installation.Path)
					fmt.Printf("      Vendor: %s (%s)\n", installation.Vendor, installation.Type)
				}

				fmt.Println()
				color.Cyan("Use 'jvm import <version>' to import a system version")
			} else {
				color.Yellow("No system Java installations found.")
			}
		}

		// 如果没有任何版本
		if len(managedVersions) == 0 {
			fmt.Println("Use 'jvm list available' to browse installable Java versions.")
			if !scanSystem {
				fmt.Println("Use 'jvm list --system' to see system installations.")
			}
		}

		return nil
	},
}

// init 函数初始化 list 命令的标志
func init() {
	listCmd.AddCommand(newAvailableCommand("available [version]"))
	// 添加显示系统版本标志
	listCmd.Flags().BoolVar(&showSystemVersions, "system", false, "同时显示系统中的 Java 版本")

	// 添加显示所有版本标志
	listCmd.Flags().BoolVar(&showAllVersions, "all", false, "显示所有版本（管理的+系统的）")
}
