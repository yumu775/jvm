package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd 是应用程序的根命令
// 在 Cobra 中，所有的子命令都会添加到这个根命令上
var rootCmd = &cobra.Command{
	Use:   "jvm",
	Short: "Java Version Manager - 类似 nvm 的 Java 版本管理工具",
	Long: `JVM 是一个简单易用的 Java 版本管理工具，灵感来自 nvm。

它允许你轻松地：
- 安装多个 Java 版本
- 在不同版本之间快速切换
- 管理项目特定的 Java 版本
- 自动处理环境变量

示例用法：
  jvm install 17        # 安装 Java 17
  jvm use 17           # 切换到 Java 17
  jvm list             # 列出已安装的版本`,
	// Run 函数定义了当用户只输入 "jvm" 时的行为
	Run: func(cmd *cobra.Command, args []string) {
		// 如果没有提供子命令，显示帮助信息
		cmd.Help()
	},
}

// Execute 函数执行根命令
// 这个函数会被 main.go 调用
func Execute() error {
	return rootCmd.Execute()
}

// init 函数在包被导入时自动执行
// 在 Go 中，init 函数用于初始化包级别的变量和设置
func init() {
	// 在这里我们可以添加全局标志（flags）
	// 例如：rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// 设置版本信息
	rootCmd.Version = "1.0.0"

	// 添加子命令
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(currentCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(listRemoteCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(sourcesCmd)
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(setEnvCmd)
	rootCmd.AddCommand(platformCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
}
