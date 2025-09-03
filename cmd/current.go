package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/version"
)

// currentCmd 定义了 "jvm current" 命令
// 这个命令显示当前激活的 Java 版本
var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "显示当前激活的 Java 版本",
	Long: `显示当前激活的 Java 版本。

如果没有激活的版本，会显示相应的提示信息。

示例：
  jvm current`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建版本管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}
		
		// 获取当前版本
		currentVersion, err := manager.GetCurrent()
		if err != nil {
			// 如果没有当前版本，显示提示信息
			color.Yellow("No Java version is currently active.")
			fmt.Println("Use 'jvm use <version>' to activate a version.")
			return nil
		}
		
		// 显示当前版本
		color.Green("Current Java version: %s", currentVersion)
		
		// 获取版本路径并显示
		versionPath, err := manager.GetVersionPath(currentVersion)
		if err != nil {
			return fmt.Errorf("failed to get version path: %w", err)
		}
		
		fmt.Printf("Installation path: %s\n", versionPath)
		
		return nil
	},
}
