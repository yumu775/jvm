package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"jvm/internal/sources"
)

func availableFixtures() []sources.JavaRelease {
	return []sources.JavaRelease{
		{Version: "17.0.9+9", FullVersion: "17.0.9+9", MajorVersion: 17, Source: "adoptium", Vendor: "Eclipse Temurin", LTS: true},
		{Version: "17.0.10+7", FullVersion: "17.0.10+7", MajorVersion: 17, Source: "adoptium", Vendor: "Eclipse Temurin", LTS: true, DownloadURL: "https://example.invalid/long-download", FileName: "jdk.zip", Checksum: "checksum"},
		{Version: "17.0.10+7", FullVersion: "17.0.10+7", MajorVersion: 17, Source: "zulu", Vendor: "Azul Zulu", LTS: true},
		{Version: "21.0.2+13", FullVersion: "21.0.2+13", MajorVersion: 21, Source: "adoptium", LTS: true},
		{Version: "22+36", FullVersion: "22+36", MajorVersion: 22, Source: "adoptium"},
	}
}

func TestAvailableSummaryKeepsEveryMajorAndVendor(t *testing.T) {
	rows := availableRows(availableFixtures(), nil, availableOptions{})
	if len(rows) != 4 {
		t.Fatalf("summary rows: %+v", rows)
	}
	if rows[0].Version != "22+36" || rows[1].MajorVersion != 21 || rows[2].Version != "17.0.10+7" {
		t.Fatalf("numeric order or latest patch wrong: %+v", rows)
	}
	if rows[2].Source == rows[3].Source {
		t.Fatal("one vendor was hidden")
	}
	var output bytes.Buffer
	if err := renderAvailable(&output, rows, nil, availableOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"VERSION", "LTS", "VENDOR", "INSTALLED"} {
		if !strings.Contains(output.String(), label) {
			t.Fatalf("missing column %s", label)
		}
	}
	if strings.Contains(output.String(), "https://") || strings.Contains(output.String(), "SHA256") {
		t.Fatal("compact list contains download details")
	}
	output.Reset()
	if err := renderAvailable(&output, rows, nil, availableOptions{Details: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "https://example.invalid/long-download") || !strings.Contains(output.String(), "SHA256: checksum") {
		t.Fatal("details omitted download metadata")
	}
}

func TestAvailableRejectsConflictingCacheFlagsWithoutQuery(t *testing.T) {
	queried := false
	command := newAvailableCommandWithQuery("available [version]", func(sources.QueryOptions) (sources.CatalogResult, error) {
		queried = true
		return sources.CatalogResult{}, nil
	})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--refresh", "--offline"})
	if err := command.Execute(); err == nil {
		t.Fatal("accepted conflicting cache flags")
	}
	if queried {
		t.Fatal("queried catalog despite conflicting flags")
	}
}

func TestAvailableFiltersBeforeGroupingAndMatchesExactInstalledVendor(t *testing.T) {
	local := []availableInstalled{{Version: "17.0.10+7", Source: "zulu"}, {Version: "17", Source: ""}}
	rows := availableRows(availableFixtures(), local, availableOptions{Version: "17", All: true, LTS: true})
	if len(rows) != 3 {
		t.Fatalf("all patches: %+v", rows)
	}
	for _, row := range rows {
		if row.Source == "zulu" && row.Installed != "yes" {
			t.Fatal("exact vendor not marked")
		}
		if row.Source == "adoptium" && row.Installed != "" {
			t.Fatal("different vendor or major alias marked installed")
		}
	}
	rows = availableRows(availableFixtures(), []availableInstalled{{Version: "17.0.10+7"}}, availableOptions{Version: "17", Sources: []string{"adoptium"}})
	if len(rows) != 1 || rows[0].Installed != "version-only" {
		t.Fatalf("legacy installation marker: %+v", rows)
	}
	rows = availableRows(availableFixtures(), nil, availableOptions{Version: "17.0.9"})
	if len(rows) != 1 || rows[0].Version != "17.0.9+9" {
		t.Fatal("filtered older patch disappeared during grouping")
	}
}

func TestAvailableJSONAndEmptyResult(t *testing.T) {
	var output bytes.Buffer
	rows := availableRows(availableFixtures(), nil, availableOptions{Version: "999"})
	if err := renderAvailable(&output, rows, nil, availableOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Releases []availableRow `json:"releases"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("not pure JSON: %s", output.String())
	}
	if result.Releases == nil {
		t.Fatal("empty releases should be []")
	}
	output.Reset()
	if err := renderAvailable(&output, rows, nil, availableOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No installable Java versions match") {
		t.Fatal("missing empty-filter explanation")
	}
}

func TestAvailableCommandPassesCatalogFlagsAndKeepsWarningsOffJSON(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	var got sources.QueryOptions
	query := func(options sources.QueryOptions) (sources.CatalogResult, error) {
		got = options
		return sources.CatalogResult{Releases: availableFixtures(), Reports: []sources.SourceReport{{Source: "adoptium", Status: "stale", Error: "network unavailable"}}}, nil
	}
	command := newAvailableCommandWithQuery("available [version]", query)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"17", "--all", "--lts", "--source", "adoptium", "--offline", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Major != 17 || !got.AllVersions || !got.Offline || got.Refresh || len(got.Sources) != 1 {
		t.Fatalf("query: %+v", got)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not JSON: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "network unavailable") {
		t.Fatal("partial failure warning missing")
	}
}

func TestAvailableLiveWarningsRemainVisibleWithoutPollutingJSON(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	for _, message := range []string{"using same-vendor fallback metadata: primary unavailable", "catalogue fetched, but cache could not be saved: access denied"} {
		command := newAvailableCommandWithQuery("available [version]", func(sources.QueryOptions) (sources.CatalogResult, error) {
			return sources.CatalogResult{Releases: availableFixtures(), Reports: []sources.SourceReport{{Source: "adoptium", Status: "live", Error: message}}}, nil
		})
		var stdout, stderr bytes.Buffer
		command.SetOut(&stdout)
		command.SetErr(&stderr)
		command.SetArgs([]string{"--json"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr.String(), message) {
			t.Fatalf("live warning hidden: %s", stderr.String())
		}
		if !json.Valid(stdout.Bytes()) {
			t.Fatal("warning polluted JSON output")
		}
	}
}

func TestAvailableAliasesAndLocalAllFlagRemainSeparate(t *testing.T) {
	for _, args := range [][]string{{"list", "available"}, {"ls", "available"}, {"list-remote"}, {"ls-remote"}, {"available"}} {
		command, remaining, err := rootCmd.Find(args)
		if err != nil || len(remaining) != 0 || command.Flags().Lookup("offline") == nil {
			t.Fatalf("command %v not wired: %v", args, err)
		}
	}
	t.Setenv("JVM_HOME", t.TempDir())
	localAll := false
	parent := &cobra.Command{Use: "list"}
	parent.Flags().BoolVar(&localAll, "all", false, "")
	child := newAvailableCommandWithQuery("available [version]", func(options sources.QueryOptions) (sources.CatalogResult, error) {
		if !options.AllVersions {
			t.Fatal("child --all not forwarded")
		}
		return sources.CatalogResult{}, nil
	})
	parent.AddCommand(child)
	parent.SetOut(&bytes.Buffer{})
	parent.SetArgs([]string{"available", "--all"})
	if err := parent.Execute(); err != nil {
		t.Fatal(err)
	}
	if localAll {
		t.Fatal("remote --all changed local list behavior")
	}
}
