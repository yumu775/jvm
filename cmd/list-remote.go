package cmd

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/download"
	"jvm/internal/sources"
)

var (
	remoteSourceFilter string
	showLTSOnly        bool
)

// listRemoteCmd 定义了 "jvm list-remote" 命令
// 这个命令用于列出可下载的 Java 版本
var listRemoteCmd = &cobra.Command{
	Use:   "list-remote",
	Short: "列出可下载的 Java 版本",
	Long: `列出可下载的 Java 版本。

这个命令会从官方源获取可用的 Java 版本列表，
包括版本号、下载链接等信息。

示例：
  jvm list-remote                    # 列出所有可下载版本
  jvm list-remote --lts              # 只显示 LTS 版本
  jvm list-remote --source corretto  # 只显示 Amazon Corretto 版本`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建下载器
		downloader := download.NewDownloader()
		
		// 获取可用版本
		color.Blue("Fetching available Java versions...")
		var versions []sources.JavaRelease
		var err error

		if remoteSourceFilter != "" {
			versions, err = downloader.GetAvailableVersionsFromSources([]string{remoteSourceFilter})
		} else {
			versions, err = downloader.GetAvailableVersions()
		}

		if err != nil {
			return fmt.Errorf("failed to fetch available versions: %w", err)
		}
		
		if len(versions) == 0 {
			color.Yellow("No versions available for download.")
			return nil
		}
		
		// 过滤 LTS 版本（如果需要）
		if showLTSOnly {
			var ltsVersions []sources.JavaRelease
			for _, version := range versions {
				if version.LTS {
					ltsVersions = append(ltsVersions, version)
				}
			}
			versions = ltsVersions
		}

		// 按主版本号排序
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].MajorVersion > versions[j].MajorVersion
		})

		// 显示版本列表
		if remoteSourceFilter != "" {
			color.Green("Available Java versions from %s:", remoteSourceFilter)
		} else {
			color.Green("Available Java versions for download:")
		}
		fmt.Println()

		// 按源分组显示
		sourceGroups := make(map[string][]sources.JavaRelease)
		for _, version := range versions {
			sourceGroups[version.Source] = append(sourceGroups[version.Source], version)
		}

		for sourceName, sourceVersions := range sourceGroups {
			if len(sourceGroups) > 1 {
				color.Cyan("From %s:", sourceName)
			}

			for _, version := range sourceVersions {
				// 标记 LTS 版本
				ltsMarker := ""
				if version.LTS {
					ltsMarker = color.YellowString(" (LTS)")
				}

				fmt.Printf("  %s%s\n", version.FullVersion, ltsMarker)
				if len(sourceGroups) > 1 {
					fmt.Printf("    Source: %s\n", version.Vendor)
				}
				fmt.Printf("    Download URL: %s\n", version.DownloadURL)
				fmt.Printf("    File: %s\n", version.FileName)
				fmt.Println()
			}
		}
		
		color.Cyan("Use 'jvm install <version>' to install a specific version")
		
		return nil
	},
}

// init 函数初始化 list-remote 命令的标志
func init() {
	// 添加源过滤标志
	listRemoteCmd.Flags().StringVarP(&remoteSourceFilter, "source", "s", "", "只显示指定源的版本")

	// 添加 LTS 过滤标志
	listRemoteCmd.Flags().BoolVar(&showLTSOnly, "lts", false, "只显示 LTS 版本")
}
