package cmd

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/sources"
)

// sourcesCmd 定义了 "jvm sources" 命令
var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "管理 Java 下载源",
	Long: `管理 Java 下载源配置。

支持的下载源：
- adoptium: Eclipse Adoptium (Temurin)
- corretto: Amazon Corretto
- zulu: Azul Zulu
- oracle: Oracle JDK
- graalvm: GraalVM

子命令：
  list      列出所有可用的下载源
  enable    启用指定的下载源
  disable   禁用指定的下载源`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认显示源列表
		return listSources()
	},
}

// sourcesListCmd 列出所有下载源
var sourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有可用的下载源",
	Long: `列出所有可用的 Java 下载源及其状态。

显示每个源的名称、状态、描述和网站信息。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listSources()
	},
}

// sourcesEnableCmd 启用下载源
var sourcesEnableCmd = &cobra.Command{
	Use:   "enable <source>",
	Short: "启用指定的下载源",
	Long: `启用指定的 Java 下载源。

示例：
  jvm sources enable oracle     # 启用 Oracle JDK 源
  jvm sources enable graalvm    # 启用 GraalVM 源`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceName := args[0]
		
		// TODO: 实现启用源的逻辑
		color.Green("Enabled source: %s", sourceName)
		color.Yellow("Note: Source management is not yet fully implemented")
		
		return nil
	},
}

// sourcesDisableCmd 禁用下载源
var sourcesDisableCmd = &cobra.Command{
	Use:   "disable <source>",
	Short: "禁用指定的下载源",
	Long: `禁用指定的 Java 下载源。

示例：
  jvm sources disable oracle    # 禁用 Oracle JDK 源
  jvm sources disable graalvm   # 禁用 GraalVM 源`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceName := args[0]
		
		// TODO: 实现禁用源的逻辑
		color.Yellow("Disabled source: %s", sourceName)
		color.Yellow("Note: Source management is not yet fully implemented")
		
		return nil
	},
}

// listSources 列出所有下载源
func listSources() error {
	sourceManager := sources.NewSourceManager()
	javaSources := sourceManager.GetDefaultSources()
	
	// 按优先级排序
	sort.Slice(javaSources, func(i, j int) bool {
		return javaSources[i].Priority < javaSources[j].Priority
	})
	
	color.Blue("=== Java Download Sources ===")
	fmt.Println()
	
	for _, source := range javaSources {
		// 显示状态
		status := color.RedString("Disabled")
		if source.Enabled {
			status = color.GreenString("Enabled")
		}
		
		fmt.Printf("%s (%s)\n", color.CyanString(source.DisplayName), status)
		fmt.Printf("  Name: %s\n", source.Name)
		fmt.Printf("  Priority: %d\n", source.Priority)
		fmt.Printf("  API Type: %s\n", source.APIType)
		
		if description, exists := source.Metadata["description"]; exists {
			fmt.Printf("  Description: %s\n", description)
		}
		
		if website, exists := source.Metadata["website"]; exists {
			fmt.Printf("  Website: %s\n", website)
		}
		
		if license, exists := source.Metadata["license"]; exists {
			color.Yellow("  License: %s", license)
		}
		
		fmt.Println()
	}
	
	// 显示使用提示
	color.Cyan("Usage:")
	fmt.Printf("  jvm install 17 --source adoptium    # 从特定源安装\n")
	fmt.Printf("  jvm list-remote --source corretto   # 列出特定源的版本\n")
	fmt.Printf("  jvm sources enable oracle           # 启用源\n")
	fmt.Printf("  jvm sources disable graalvm         # 禁用源\n")
	
	return nil
}

// init 函数初始化 sources 命令和子命令
func init() {
	// 添加子命令
	sourcesCmd.AddCommand(sourcesListCmd)
	sourcesCmd.AddCommand(sourcesEnableCmd)
	sourcesCmd.AddCommand(sourcesDisableCmd)
}
