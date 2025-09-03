package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/alias"
	"jvm/internal/download"
	"jvm/internal/sources"
)

// aliasCmd 定义了 "jvm alias" 命令
var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "管理版本别名",
	Long: `管理和查看版本别名。

支持的别名：
- latest: 最新的稳定版本
- lts: 最新的 LTS 版本
- lts-1: 上一个 LTS 版本
- stable: 最新的稳定版本 (等同于 latest)
- current: 当前推荐版本 (通常是最新 LTS)

特殊格式：
- <source>-latest: 特定源的最新版本 (如 adoptium-latest)
- lts-<n>: LTS 版本偏移 (如 lts-2 表示倒数第三个 LTS)

子命令：
  list      列出所有支持的别名
  resolve   解析别名为具体版本
  explain   解释别名的含义`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认显示别名列表
		return listAliases()
	},
}

// aliasListCmd 列出所有别名
var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有支持的版本别名",
	Long: `列出所有支持的版本别名及其描述。

显示每个别名的名称、描述和当前解析结果。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listAliases()
	},
}

// aliasResolveCmd 解析别名
var aliasResolveCmd = &cobra.Command{
	Use:   "resolve <alias>",
	Short: "解析别名为具体版本",
	Long: `将版本别名解析为具体的 Java 版本。

示例：
  jvm alias resolve latest      # 解析 latest 别名
  jvm alias resolve lts         # 解析 lts 别名
  jvm alias resolve lts-1       # 解析上一个 LTS 版本`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasName := args[0]
		
		// 创建下载器和别名解析器
		downloader := download.NewDownloader()
		resolver := alias.NewResolver()
		
		// 获取可用版本
		color.Blue("Fetching available versions...")
		releases, err := downloader.GetAvailableVersions()
		if err != nil {
			return fmt.Errorf("failed to get available versions: %w", err)
		}
		
		// 解析别名
		resolved, err := resolver.ResolveAlias(aliasName, releases, "")
		if err != nil {
			return fmt.Errorf("failed to resolve alias '%s': %w", aliasName, err)
		}
		
		// 显示解析结果
		color.Green("Alias Resolution:")
		fmt.Printf("  Alias: %s\n", aliasName)
		fmt.Printf("  Resolves to: Java %s\n", resolved.FullVersion)
		fmt.Printf("  Major version: %d\n", resolved.MajorVersion)
		fmt.Printf("  Source: %s\n", resolved.Vendor)
		fmt.Printf("  LTS: %v\n", resolved.LTS)
		
		if resolved.ReleaseDate != "" {
			fmt.Printf("  Release date: %s\n", resolved.ReleaseDate)
		}
		
		fmt.Println()
		color.Cyan("You can install this version with:")
		color.Cyan("  jvm install %s", aliasName)
		
		return nil
	},
}

// aliasExplainCmd 解释别名
var aliasExplainCmd = &cobra.Command{
	Use:   "explain <alias>",
	Short: "解释别名的含义",
	Long: `解释版本别名的含义和用途。

示例：
  jvm alias explain latest      # 解释 latest 别名
  jvm alias explain lts-1       # 解释 lts-1 别名`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasName := args[0]
		
		// 创建解析器
		resolver := alias.NewResolver()
		
		// 获取可用版本（用于解析）
		downloader := download.NewDownloader()
		releases, err := downloader.GetAvailableVersions()
		if err != nil {
			color.Yellow("Warning: Could not fetch versions for resolution")
			releases = []sources.JavaRelease{} // 空列表，仍然可以显示描述
		}
		
		// 解释别名
		explanation, err := resolver.ExplainAlias(aliasName, releases)
		if err != nil {
			return fmt.Errorf("failed to explain alias '%s': %w", aliasName, err)
		}
		
		color.Blue("Alias Explanation:")
		fmt.Printf("  %s\n", explanation)
		
		// 显示相关建议
		fmt.Println()
		color.Cyan("Related aliases:")
		suggestions := resolver.GetVersionSuggestions(aliasName, releases)
		for _, suggestion := range suggestions {
			if suggestion != aliasName {
				fmt.Printf("  %s\n", suggestion)
			}
		}
		
		return nil
	},
}

// aliasSuggestCmd 获取别名建议
var aliasSuggestCmd = &cobra.Command{
	Use:   "suggest [partial]",
	Short: "获取版本别名建议",
	Long: `根据输入获取版本别名建议。

示例：
  jvm alias suggest             # 显示所有别名
  jvm alias suggest lt          # 显示以 'lt' 开头的别名
  jvm alias suggest 17          # 显示包含 '17' 的版本`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input string
		if len(args) > 0 {
			input = args[0]
		}
		
		// 创建解析器和下载器
		resolver := alias.NewResolver()
		downloader := download.NewDownloader()
		
		// 获取可用版本
		releases, err := downloader.GetAvailableVersions()
		if err != nil {
			color.Yellow("Warning: Could not fetch versions")
			releases = []sources.JavaRelease{}
		}
		
		// 获取建议
		suggestions := resolver.GetVersionSuggestions(input, releases)
		
		if len(suggestions) == 0 {
			color.Yellow("No suggestions found for '%s'", input)
			return nil
		}
		
		color.Blue("Version suggestions:")
		
		// 分类显示建议
		aliases := []string{}
		versions := []string{}
		sources := []string{}
		
		for _, suggestion := range suggestions {
			if strings.Contains(suggestion, "-latest") {
				sources = append(sources, suggestion)
			} else if strings.Contains(suggestion, ".") || len(suggestion) <= 2 {
				versions = append(versions, suggestion)
			} else {
				aliases = append(aliases, suggestion)
			}
		}
		
		if len(aliases) > 0 {
			fmt.Printf("\n  Aliases:\n")
			for _, alias := range aliases {
				fmt.Printf("    %s\n", alias)
			}
		}
		
		if len(versions) > 0 {
			fmt.Printf("\n  Versions:\n")
			for _, version := range versions {
				fmt.Printf("    %s\n", version)
			}
		}
		
		if len(sources) > 0 {
			fmt.Printf("\n  Source-specific:\n")
			for _, source := range sources {
				fmt.Printf("    %s\n", source)
			}
		}
		
		return nil
	},
}

// listAliases 列出所有别名
func listAliases() error {
	resolver := alias.NewResolver()
	aliases := resolver.GetSupportedAliases()
	
	// 获取可用版本用于解析
	downloader := download.NewDownloader()
	releases, err := downloader.GetAvailableVersions()
	if err != nil {
		color.Yellow("Warning: Could not fetch versions for resolution")
		releases = []sources.JavaRelease{}
	}
	
	color.Blue("=== Supported Version Aliases ===")
	fmt.Println()
	
	// 按字母顺序排序
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].Alias < aliases[j].Alias
	})
	
	for _, alias := range aliases {
		fmt.Printf("%s\n", color.CyanString(alias.Alias))
		fmt.Printf("  Description: %s\n", alias.Description)
		
		// 尝试解析别名
		if len(releases) > 0 {
			if resolved, err := resolver.ResolveAlias(alias.Alias, releases, ""); err == nil {
				fmt.Printf("  Current: Java %s (%s)\n", resolved.FullVersion, resolved.Vendor)
			}
		}
		
		fmt.Println()
	}
	
	// 显示特殊格式说明
	color.Blue("Special Formats:")
	fmt.Printf("  <source>-latest    Latest version from specific source (e.g., adoptium-latest)\n")
	fmt.Printf("  lts-<n>           LTS version offset (e.g., lts-2 for 3rd newest LTS)\n")
	fmt.Println()
	
	color.Cyan("Usage:")
	fmt.Printf("  jvm install latest           # Install latest version\n")
	fmt.Printf("  jvm install lts              # Install latest LTS\n")
	fmt.Printf("  jvm alias resolve latest     # See what 'latest' resolves to\n")
	fmt.Printf("  jvm alias explain lts        # Explain what 'lts' means\n")
	
	return nil
}

// init 函数初始化 alias 命令和子命令
func init() {
	// 添加子命令
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasResolveCmd)
	aliasCmd.AddCommand(aliasExplainCmd)
	aliasCmd.AddCommand(aliasSuggestCmd)
}
