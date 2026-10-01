package cmd

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"jvm/internal/sources"
)

var sourcesCmd = &cobra.Command{
	Use: "sources", Short: "查看下载源状态，设置默认发行版与优先级",
	Long: `管理 Java 下载源。默认显示紧凑列表，不访问网络。

sources check [name] 刷新 Java 17 元数据，逐源显示可用性与错误。
sources default [name] 查看或设置默认安装来源。
sources priority <name> <n> 调整列表与查询优先级（数字越小越靠前）。

默认来源或显式 --source 失败时，安装不会静默切换 Java 厂商。
Oracle 暂不支持自动下载，请从官方获取 JDK 后使用 jvm import。`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return listSourcesTo(cmd.OutOrStdout()) },
}

var sourcesListCmd = &cobra.Command{
	Use: "list", Short: "列出配置状态（不进行网络请求）", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return listSourcesTo(cmd.OutOrStdout()) },
}

var sourcesEnableCmd = &cobra.Command{
	Use: "enable <source>", Short: "启用指定下载源", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sources.NewSourceManager().SetEnabled(args[0], true); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Enabled source: %s\n", args[0])
		return nil
	},
}

var sourcesDisableCmd = &cobra.Command{
	Use: "disable <source>", Short: "禁用指定下载源", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := sources.NewSourceManager()
		if err := manager.SetEnabled(args[0], false); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Disabled source: %s\n", args[0])
		selected, err := manager.DefaultSource()
		if err == nil && selected == args[0] {
			fmt.Fprintln(cmd.OutOrStdout(), "The default source is disabled. Choose another with jvm sources default <name> before installing.")
		}
		return nil
	},
}

var sourcesDefaultCmd = &cobra.Command{
	Use: "default [source]", Short: "查看或设置默认安装发行版", Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := sources.NewSourceManager()
		if len(args) == 0 {
			name, err := manager.DefaultSource()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), name)
			return nil
		}
		if err := manager.SetDefault(args[0]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Default installation source: %s\n", args[0])
		fmt.Fprintln(cmd.OutOrStdout(), "If this source fails, installation returns an error; another vendor is never selected silently.")
		return nil
	},
}

var sourcesPriorityCmd = &cobra.Command{
	Use: "priority <source> <number>", Short: "设置来源优先级，数字越小越靠前", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		priority, err := strconv.Atoi(args[1])
		if err != nil || priority < 0 {
			return fmt.Errorf("priority must be a non-negative integer")
		}
		if err := sources.NewSourceManager().SetPriority(args[0], priority); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Source priority: %s = %d\n", args[0], priority)
		fmt.Fprintln(cmd.OutOrStdout(), "Priority controls ordering; it does not change the default installation source.")
		return nil
	},
}

var sourcesCheckCmd = newSourcesCheckCommand(func(options sources.QueryOptions) (sources.CatalogResult, error) {
	return sources.NewCatalog().Query(options)
})

func newSourcesCheckCommand(query func(sources.QueryOptions) (sources.CatalogResult, error)) *cobra.Command {
	command := &cobra.Command{
		Use: "check [source]", Short: "逐源检查当前平台的 Java 17 元数据可用性", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			major, _ := cmd.Flags().GetInt("major")
			if major < 1 {
				return fmt.Errorf("major version must be positive")
			}
			options := sources.QueryOptions{Major: major, Refresh: true}
			if len(args) > 0 {
				options.Sources = []string{args[0]}
			}
			result, queryErr := query(options)
			if len(result.Reports) > 0 {
				if err := writeSourceReports(cmd.OutOrStdout(), result.Reports); err != nil {
					return err
				}
			}
			if queryErr != nil {
				return queryErr
			}
			for _, report := range result.Reports {
				if report.Status == "error" || report.Status == "stale" {
					return fmt.Errorf("one or more sources could not provide fresh metadata; see source status above")
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Metadata check completed for Java %d. This checks the catalog, not archive download availability.\n", major)
			return nil
		},
	}
	command.Flags().Int("major", 17, "检查指定 Java 主版本的元数据")
	return command
}

func listSourcesTo(out io.Writer) error {
	manager := sources.NewSourceManager()
	all, err := manager.LoadSources()
	if err != nil {
		return err
	}
	selected, err := manager.DefaultSource()
	if err != nil {
		return err
	}
	return writeSourcesTable(out, all, selected)
}

func writeSourcesTable(out io.Writer, all []sources.JavaSource, selected string) error {
	ordered := append([]sources.JavaSource(nil), all...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].Name < ordered[j].Name
		}
		return ordered[i].Priority < ordered[j].Priority
	})
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "DEFAULT\tSOURCE\tENABLED\tPRIORITY\tDOWNLOAD\tDISTRIBUTION")
	for _, source := range ordered {
		mark, status := "-", "no"
		if source.Name == selected {
			mark = "*"
		}
		if source.Enabled {
			status = "yes"
		}
		capability := "manual import"
		if sources.IsSupportedSource(source) {
			capability = "automatic"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", mark, source.Name, status, source.Priority, capability, source.DisplayName)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, "\n* Default installation source. Run jvm sources check to test metadata access.\nPriority never authorizes switching to another vendor. Oracle requires manual import.")
	return err
}

func writeSourceReports(out io.Writer, reports []sources.SourceReport) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SOURCE\tSTATUS\tVERSIONS\tDETAIL")
	for _, report := range reports {
		detail := strings.Join(strings.Fields(report.Error), " ")
		if detail == "" && !report.CachedAt.IsZero() {
			detail = "cached " + report.CachedAt.Format("2006-01-02 15:04:05Z07:00")
		}
		if detail == "" {
			detail = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", report.Source, report.Status, report.Count, detail)
	}
	return w.Flush()
}

func init() {
	sourcesCmd.AddCommand(sourcesListCmd, sourcesEnableCmd, sourcesDisableCmd, sourcesDefaultCmd, sourcesPriorityCmd, sourcesCheckCmd)
}
