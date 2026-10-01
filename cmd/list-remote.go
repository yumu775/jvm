package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/sources"
	"jvm/internal/version"
)

type availableOptions struct {
	Version                                   string
	Sources                                   []string
	All, LTS, JSON, Refresh, Offline, Details bool
}

type availableRow struct {
	sources.JavaRelease
	Installed string `json:"installed"`
}

type availableInstalled struct{ Version, Source string }
type availableQuery func(sources.QueryOptions) (sources.CatalogResult, error)

var listRemoteCmd = newAvailableCommand("list-remote [version]")

// 两个命令共享实现，各自保存参数，避免父命令的 --all 改变远程列表语义。
func newAvailableCommand(use string) *cobra.Command {
	return newAvailableCommandWithQuery(use, func(options sources.QueryOptions) (sources.CatalogResult, error) {
		return sources.NewCatalog().Query(options)
	})
}

func newAvailableCommandWithQuery(use string, query availableQuery) *cobra.Command {
	options := availableOptions{}
	command := &cobra.Command{
		Use: use, Short: "列出当前平台可安装的 Java 版本",
		Long: `列出可安装的 Java 版本，默认每个主版本、每个发行版显示最新补丁。

示例：
  jvm list available
  jvm list available 17 --all
  jvm list available --lts --source adoptium
  jvm list available --offline --json
  jvm list-remote --details`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.Version = args[0]
			}
			return runAvailable(cmd, options, query)
		},
	}
	if strings.HasPrefix(use, "list-remote") {
		command.Aliases = []string{"ls-remote", "available"}
	}
	flags := command.Flags()
	flags.StringSliceVarP(&options.Sources, "source", "s", nil, "只显示指定发行版，可重复或以逗号分隔")
	flags.BoolVar(&options.LTS, "lts", false, "只显示长期支持版本")
	flags.BoolVar(&options.All, "all", false, "显示所有可用补丁版本")
	flags.BoolVar(&options.JSON, "json", false, "输出结构化 JSON")
	flags.BoolVar(&options.Refresh, "refresh", false, "忽略缓存，重新查询发行版目录")
	flags.BoolVar(&options.Offline, "offline", false, "只读取本地目录缓存，不访问网络")
	flags.BoolVar(&options.Details, "details", false, "额外显示下载地址、文件和校验信息")
	command.MarkFlagsMutuallyExclusive("refresh", "offline")
	return command
}

func runAvailable(cmd *cobra.Command, options availableOptions, query availableQuery) error {
	major := 0
	if options.Version != "" {
		if !regexp.MustCompile(`^[0-9]+(\.[0-9]+)*(\+[0-9]+)?$`).MatchString(options.Version) {
			return fmt.Errorf("expected a Java version such as 17 or 17.0.12+7")
		}
		major, _ = strconv.Atoi(strings.Split(strings.Split(options.Version, ".")[0], "+")[0])
		if major < 1 {
			return fmt.Errorf("Java major version must be positive")
		}
	}
	result, queryErr := query(sources.QueryOptions{Sources: options.Sources, Major: major, AllVersions: options.All || strings.ContainsAny(options.Version, ".+"), Refresh: options.Refresh, Offline: options.Offline})
	for _, report := range result.Reports {
		if report.Error != "" || report.Status == "stale" || report.Status == "error" || report.Status == "disabled" {
			message := report.Error
			if message == "" && report.Status == "stale" {
				message = "using cached metadata; refresh when online"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s (%s): %s\n", report.Source, report.Status, message)
		}
	}
	manager, err := version.NewManager()
	if err != nil {
		return err
	}
	installed, err := manager.ListInstalled()
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	local := make([]availableInstalled, 0, len(installed))
	for _, item := range installed {
		record := cfg.Installations[item.Version]
		v := record.Version
		if v == "" {
			v = item.Version
		}
		local = append(local, availableInstalled{Version: v, Source: record.Source})
	}
	rows := availableRows(result.Releases, local, options)
	if queryErr != nil && len(rows) == 0 && !options.JSON {
		return queryErr
	}
	if err := renderAvailable(cmd.OutOrStdout(), rows, result.Reports, options); err != nil {
		return err
	}
	return queryErr
}

func releaseVersion(release sources.JavaRelease) string {
	if release.FullVersion != "" {
		return release.FullVersion
	}
	return release.Version
}

func releaseMajor(release sources.JavaRelease) int {
	if release.MajorVersion > 0 {
		return release.MajorVersion
	}
	major, _ := strconv.Atoi(strings.Split(releaseVersion(release), ".")[0])
	return major
}

// 先筛选再选择最新补丁，保留所有主版本和发行版。
func availableRows(releases []sources.JavaRelease, installed []availableInstalled, options availableOptions) []availableRow {
	selected := make([]sources.JavaRelease, 0, len(releases))
	for _, release := range releases {
		if options.LTS && !release.LTS {
			continue
		}
		v := releaseVersion(release)
		if options.Version != "" && v != options.Version && !strings.HasPrefix(v, options.Version+".") && !strings.HasPrefix(v, options.Version+"+") {
			continue
		}
		if len(options.Sources) > 0 {
			matched := false
			for _, source := range options.Sources {
				if release.Source == source {
					matched = true
				}
			}
			if !matched {
				continue
			}
		}
		selected = append(selected, release)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if releaseMajor(selected[i]) != releaseMajor(selected[j]) {
			return releaseMajor(selected[i]) > releaseMajor(selected[j])
		}
		pi, _ := strconv.Atoi(selected[i].Metadata["source_priority"])
		pj, _ := strconv.Atoi(selected[j].Metadata["source_priority"])
		if pi != pj {
			return pi < pj
		}
		if result := sources.CompareVersions(releaseVersion(selected[i]), releaseVersion(selected[j])); result != 0 {
			return result > 0
		}
		return selected[i].Source < selected[j].Source
	})
	rows := []availableRow{}
	seen := map[string]bool{}
	for _, release := range selected {
		key := release.Source + "/" + strconv.Itoa(releaseMajor(release))
		if options.All {
			key = release.Source + "/" + releaseVersion(release)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		marker := ""
		for _, local := range installed {
			if local.Version != releaseVersion(release) && local.Version != release.Version {
				continue
			}
			if local.Source == release.Source {
				marker = "yes"
				break
			}
			if local.Source == "" {
				marker = "version-only"
			}
		}
		rows = append(rows, availableRow{JavaRelease: release, Installed: marker})
	}
	return rows
}

func renderAvailable(output io.Writer, rows []availableRow, reports []sources.SourceReport, options availableOptions) error {
	if options.JSON {
		if reports == nil {
			reports = []sources.SourceReport{}
		}
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(struct {
			Releases []availableRow         `json:"releases"`
			Reports  []sources.SourceReport `json:"reports"`
		}{rows, reports})
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(output, "No installable Java versions match these filters.")
		return err
	}
	if !options.All {
		fmt.Fprintln(output, "Latest patch per major version and vendor; use --all for patch history.")
	}
	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "VERSION\tLTS\tVENDOR\tINSTALLED")
	versionOnly := false
	for _, row := range rows {
		lts := "-"
		if row.LTS {
			lts = "LTS"
		}
		vendor := row.Source
		if vendor == "" {
			vendor = row.Vendor
		}
		installed := "-"
		if row.Installed != "" {
			installed = row.Installed
		}
		if installed == "version-only" {
			versionOnly = true
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", releaseVersion(row.JavaRelease), lts, vendor, installed)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if versionOnly {
		fmt.Fprintln(output, "version-only: an exact local version exists; its vendor is not recorded.")
	}
	if options.Details {
		for _, row := range rows {
			checksum := row.Checksum
			if checksum == "" {
				checksum = "resolved and verified before download"
			}
			fmt.Fprintf(output, "\n%s (%s)\n  Vendor: %s\n  File: %s\n  URL: %s\n  SHA256: %s\n", releaseVersion(row.JavaRelease), row.Source, row.Vendor, row.FileName, row.DownloadURL, checksum)
		}
		for _, report := range reports {
			fmt.Fprintf(output, "\nSource: %s  Status: %s  Releases: %d\n", report.Source, report.Status, report.Count)
			if !report.CachedAt.IsZero() {
				fmt.Fprintf(output, "  Cached: %s\n", report.CachedAt.Format("2006-01-02 15:04:05 MST"))
			}
		}
	}
	_, err := fmt.Fprintln(output, "\nInstall: jvm install VERSION --source SOURCE (use the VENDOR column)")
	return err
}
