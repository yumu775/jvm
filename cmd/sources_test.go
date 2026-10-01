package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"jvm/internal/config"
	"jvm/internal/sources"
)

func TestSourcesTableShowsSelectionAndCapabilitiesWithoutChangingOrder(t *testing.T) {
	all := []sources.JavaSource{
		{Name: "oracle", DisplayName: "Oracle JDK", APIType: "oracle", Priority: 5},
		{Name: "adoptium", DisplayName: "Temurin", APIType: "adoptium", Priority: 1, Enabled: true},
	}
	var output bytes.Buffer
	if err := writeSourcesTable(&output, all, "adoptium"); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "automatic") || !strings.Contains(text, "manual import") || !strings.Contains(text, "*        adoptium") {
		t.Fatalf("missing state or capability:\n%s", text)
	}
	if strings.Index(text, "adoptium") > strings.Index(text, "oracle") {
		t.Fatal("priority order ignored")
	}
	if all[0].Name != "oracle" {
		t.Fatal("rendering changed caller's source ordering")
	}
}

func TestSourceReportsPreservePartialSuccessAndExplainFailure(t *testing.T) {
	reports := []sources.SourceReport{
		{Source: "adoptium", Status: "live", Count: 4},
		{Source: "zulu", Status: "stale", Count: 2, CachedAt: time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC), Error: "request failed\nconnection timeout"},
	}
	var output bytes.Buffer
	if err := writeSourceReports(&output, reports); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, part := range []string{"adoptium", "live", "zulu", "stale", "request failed connection timeout"} {
		if !strings.Contains(text, part) {
			t.Fatalf("missing %q in report:\n%s", part, text)
		}
	}
}

func TestSourcePriorityRejectsMalformedInput(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	for _, value := range []string{"-1", "abc"} {
		stdout, stderr, err := runCLI(t, "sources", "priority", "adoptium", "--", value)
		if err == nil || stdout != "" || !strings.Contains(stderr, "non-negative integer") {
			t.Fatalf("value %q: stdout=%q stderr=%q err=%v", value, stdout, stderr, err)
		}
	}
}

func TestSourceDefaultAndPriorityPersistIndependently(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	for _, args := range [][]string{{"sources", "enable", "zulu"}, {"sources", "default", "zulu"}, {"sources", "priority", "adoptium", "0"}} {
		_, stderr, err := runCLI(t, args...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, stderr)
		}
	}
	stdout, stderr, err := runCLI(t, "sources", "default")
	if err != nil || strings.TrimSpace(stdout) != "zulu" {
		t.Fatalf("priority changed default: %q %q %v", stdout, stderr, err)
	}
	_, stderr, err = runCLI(t, "sources", "default", "does-not-exist")
	if err == nil {
		t.Fatalf("unknown source accepted: %s", stderr)
	}
	stdout, _, err = runCLI(t, "sources", "default")
	if err != nil || strings.TrimSpace(stdout) != "zulu" {
		t.Fatal("failed selection changed default")
	}
}

func TestSourceCheckForcesRefreshAndReturnsFailureWithPartialReports(t *testing.T) {
	var output bytes.Buffer
	command := newSourcesCheckCommand(func(options sources.QueryOptions) (sources.CatalogResult, error) {
		if options.Major != 17 || !options.Refresh || options.Offline || options.AllVersions {
			t.Fatalf("unexpected check query: %+v", options)
		}
		return sources.CatalogResult{Reports: []sources.SourceReport{
			{Source: "adoptium", Status: "live", Count: 3},
			{Source: "zulu", Status: "stale", Count: 2, Error: "timeout"},
		}}, nil
	})
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(nil)
	if err := command.Execute(); err == nil {
		t.Fatal("partial failure was reported as a healthy check")
	}
	for _, part := range []string{"adoptium", "live", "zulu", "stale", "timeout"} {
		if !strings.Contains(output.String(), part) {
			t.Fatalf("missing %q: %s", part, output.String())
		}
	}
	if strings.Contains(output.String(), "check completed") {
		t.Fatal("false success message")
	}
}

func TestConfigListUsesActualSourcesInsteadOfLegacyField(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DownloadSources = []config.DownloadSource{{Name: "legacy-only", URL: "https://invalid.example"}}
	if err := cfg.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	if err := sources.NewSourceManager().SetPriority("adoptium", 42); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCLI(t, "config", "list")
	if err != nil {
		t.Fatalf("config list: %v %s", err, stderr)
	}
	if strings.Contains(stdout, "legacy-only") || !strings.Contains(stdout, "adoptium") || !strings.Contains(stdout, "42") {
		t.Fatalf("config did not show actual source settings:\n%s", stdout)
	}
}
