package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"jvm/internal/alias"
	"jvm/internal/download"
	"jvm/internal/sources"
)

type aliasOptions struct {
	Source           string
	Refresh, Offline bool
}
type aliasQuery func(sources.QueryOptions) (sources.CatalogResult, error)

var aliasCmd = newAliasCommand(download.NewDownloader().QueryCatalog, sources.NewSourceManager().DefaultSource)

// 所有入口共享缓存选项和默认发行版，解析结果与 install 的选择保持一致。
func newAliasCommand(query aliasQuery, defaultSource func() (string, error)) *cobra.Command {
	options := aliasOptions{}
	root := &cobra.Command{Use: "alias", Short: "查看和解析版本别名", Args: cobra.NoArgs,
		Long: `查看 latest、stable、lts、current、lts-<n> 和 <source>-latest 别名。
默认使用安装设置中的默认发行版，可用 --source 显式选择。
--offline 只读取目录缓存；--refresh 强制刷新目录。`,
	}
	root.PersistentFlags().StringVarP(&options.Source, "source", "s", "", "发行版；省略时使用 sources default")
	root.PersistentFlags().BoolVar(&options.Refresh, "refresh", false, "强制刷新发行版目录")
	root.PersistentFlags().BoolVar(&options.Offline, "offline", false, "只读取缓存，不访问网络")
	root.MarkFlagsMutuallyExclusive("refresh", "offline")
	root.RunE = func(cmd *cobra.Command, args []string) error { return runAliasList(cmd, options, query, defaultSource) }
	list := &cobra.Command{Use: "list", Short: "列出别名定义与当前解析结果", Args: cobra.NoArgs, RunE: root.RunE}
	resolve := &cobra.Command{Use: "resolve <alias>", Short: "解析别名或版本为具体发行版", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		releases, source, err := queryAliasCatalog(name, options, query, defaultSource)
		if err != nil {
			return fmt.Errorf("cannot resolve %s: %w", name, err)
		}
		release, err := alias.NewResolver().ResolveAlias(name, releases, source)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Alias: %s\nVersion: %s\nSource: %s\nVendor: %s\nLTS: %t\n\nInstall: jvm install %s --source %s\n", name, release.FullVersion, release.Source, release.Vendor, release.LTS, release.Version, release.Source)
		return err
	}}
	explain := &cobra.Command{Use: "explain <alias>", Short: "解释别名；缓存不可用时仍显示定义", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		definition, err := aliasDefinition(name)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), definition)
		releases, source, err := queryAliasCatalog(name, options, query, defaultSource)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Resolution unavailable: %v\n", err)
			return nil
		}
		release, err := alias.NewResolver().ResolveAlias(name, releases, source)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Resolution unavailable: %v\n", err)
			return nil
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Current resolution: Java %s (%s)\n", release.FullVersion, release.Source)
		return err
	}}
	suggest := &cobra.Command{Use: "suggest [partial]", Short: "根据输入建议别名和版本", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		input := ""
		if len(args) > 0 {
			input = strings.ToLower(strings.TrimSpace(args[0]))
		}
		releases, source, err := queryAliasCatalog(input, options, query, defaultSource)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Version suggestions unavailable: %v; showing known aliases only.\n", err)
			releases = nil
		}
		filtered := []sources.JavaRelease{}
		for _, release := range releases {
			if release.Source == source {
				filtered = append(filtered, release)
			}
		}
		suggestions := alias.NewResolver().GetVersionSuggestions(input, filtered)
		sort.Strings(suggestions)
		if len(suggestions) == 0 {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "No suggestions match %q.\n", input)
			return err
		}
		for _, suggestion := range suggestions {
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), suggestion); err != nil {
				return err
			}
		}
		return nil
	}}
	root.AddCommand(list, resolve, explain, suggest)
	return root
}

func queryAliasCatalog(name string, options aliasOptions, query aliasQuery, defaultSource func() (string, error)) ([]sources.JavaRelease, string, error) {
	source := strings.ToLower(strings.TrimSpace(options.Source))
	if strings.HasSuffix(name, "-latest") {
		named := strings.TrimSuffix(name, "-latest")
		if source != "" && source != named {
			return nil, "", fmt.Errorf("conflicting sources: %s and %s", source, named)
		}
		source = named
	}
	if source == "" {
		var err error
		source, err = defaultSource()
		if err != nil {
			return nil, "", err
		}
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '+' })
	major := 0
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	all := major > 0 && strings.ContainsAny(name, ".+")
	result, err := query(sources.QueryOptions{Sources: []string{source}, Major: major, AllVersions: all, Refresh: options.Refresh, Offline: options.Offline})
	return result.Releases, source, err
}

func runAliasList(cmd *cobra.Command, options aliasOptions, query aliasQuery, defaultSource func() (string, error)) error {
	releases, source, err := queryAliasCatalog("", options, query, defaultSource)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Current resolutions unavailable: %v\n", err)
		releases = nil
	}
	if source != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Source: %s\n", source)
	}
	resolver := alias.NewResolver()
	definitions := resolver.GetSupportedAliases()
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Alias < definitions[j].Alias })
	table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ALIAS\tRESOLVES TO\tDESCRIPTION")
	for _, definition := range definitions {
		resolved := "unavailable"
		if release, e := resolver.ResolveAlias(definition.Alias, releases, source); e == nil {
			resolved = release.FullVersion
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", definition.Alias, resolved, definition.Description)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nSpecial formats: lts-<n>, <source>-latest\nResolve: jvm alias resolve lts\nInstall: jvm install ALIAS --source SOURCE")
	return err
}

func aliasDefinition(name string) (string, error) {
	if strings.HasPrefix(name, "lts-") {
		offset, err := strconv.Atoi(strings.TrimPrefix(name, "lts-"))
		if err != nil || offset < 0 {
			return "", fmt.Errorf("invalid LTS offset: %s", name)
		}
		return fmt.Sprintf("%s: 选择第 %d 个最新的 LTS 主版本，并使用该主版本的最新补丁。", name, offset+1), nil
	}
	if strings.HasSuffix(name, "-latest") && strings.TrimSuffix(name, "-latest") != "" {
		return fmt.Sprintf("%s: 使用 %s 发行版提供的最新稳定版本。", name, strings.TrimSuffix(name, "-latest")), nil
	}
	return alias.NewResolver().ExplainAlias(name, nil)
}
