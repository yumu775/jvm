package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/env"
	"jvm/internal/version"
)

// useCmd 定义了 "jvm use" 命令
// 这个命令用于切换到指定的 Java 版本
var (
	tempOnly bool
	persistent bool
)

var useCmd = &cobra.Command{
	Use:   "use <version>",
	Short: "切换到指定的 Java 版本",
	Long: `切换到指定的 Java 版本。

这个命令会：
1. 验证指定的版本是否已安装
2. 设置该版本为当前激活版本
3. 自动更新环境变量和 shell 配置

示例：
  jvm use 17                    # 切换到 Java 17 并更新 shell 配置
  jvm use 11.0.19 --temp       # 仅在当前会话中切换
  jvm use 17 --persistent      # 强制更新 shell 配置文件`,
	// Args 字段定义了命令参数的验证规则
	// cobra.ExactArgs(1) 表示这个命令需要恰好一个参数
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 获取用户指定的版本号
		targetVersion := args[0]
		
		// 创建版本管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}
		
		// 检查版本是否已安装
		if !manager.IsInstalled(targetVersion) {
			color.Red("Java version %s is not installed.", targetVersion)
			fmt.Printf("Use 'jvm install %s' to install this version.\n", targetVersion)
			return nil
		}
		
		// 切换到指定版本
		if err := manager.SetCurrent(targetVersion); err != nil {
			return fmt.Errorf("failed to set current version: %w", err)
		}
		
		// 获取版本路径
		versionPath, err := manager.GetVersionPath(targetVersion)
		if err != nil {
			return fmt.Errorf("failed to get version path: %w", err)
		}

		// 显示成功信息
		color.Green("Now using Java %s", targetVersion)
		fmt.Printf("Installation path: %s\n", versionPath)

		// 创建环境变量管理器
		envManager := env.NewManager()

		// 设置环境变量
		if tempOnly {
			// 仅设置临时环境变量
			if err := envManager.SetJavaEnvironment(versionPath, true); err != nil {
				color.Yellow("Warning: failed to set temporary environment variables: %v", err)
			}
		} else {
			// 设置临时环境变量（立即生效）
			if err := envManager.SetJavaEnvironment(versionPath, true); err != nil {
				color.Yellow("Warning: failed to set temporary environment variables: %v", err)
			}

			// 如果用户明确要求持久化，或者默认行为
			if persistent || !tempOnly {
				fmt.Println()
				color.Blue("Updating shell configuration for persistent environment variables...")
				if err := envManager.SetJavaEnvironment(versionPath, false); err != nil {
					color.Yellow("Warning: failed to update shell configuration: %v", err)
					fmt.Println()
					color.Yellow("You can manually set environment variables:")
					color.Yellow("  JAVA_HOME=%s", versionPath)
					color.Yellow("  PATH=%s:$PATH", filepath.Join(versionPath, "bin"))
				}
			}
		}
		
		return nil
	},
}

// init 函数初始化 use 命令的标志
func init() {
	// 添加仅临时设置标志
	useCmd.Flags().BoolVarP(&tempOnly, "temp", "t", false, "仅在当前会话中设置环境变量")

	// 添加强制持久化标志
	useCmd.Flags().BoolVarP(&persistent, "persistent", "p", false, "强制更新 shell 配置文件")
}
