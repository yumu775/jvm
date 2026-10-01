package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"jvm/internal/sources"
)

func aliasFixtures() []sources.JavaRelease {
	return []sources.JavaRelease{
		{Version: "26+12", FullVersion: "26+12", MajorVersion: 26, Source: "adoptium", Vendor: "Temurin"},
		{Version: "21.0.2+13", FullVersion: "21.0.2+13", MajorVersion: 21, Source: "zulu", Vendor: "Azul", LTS: true},
		{Version: "17.0.10+7", FullVersion: "17.0.10+7", MajorVersion: 17, Source: "zulu", Vendor: "Azul", LTS: true},
	}
}

func TestAliasResolveUsesInstallDefaultAndSourcePinnedInstruction(t *testing.T) {
	var got sources.QueryOptions
	command := newAliasCommand(func(options sources.QueryOptions) (sources.CatalogResult, error) {
		got = options
		return sources.CatalogResult{Releases: aliasFixtures()}, nil
	}, func() (string, error) { return "zulu", nil })
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"resolve", "latest", "--offline"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || got.Sources[0] != "zulu" || !got.Offline || got.AllVersions {
		t.Fatalf("query: %+v", got)
	}
	if !strings.Contains(output.String(), "Version: 21.0.2+13") || strings.Contains(output.String(), "26+12") {
		t.Fatalf("resolved outside default source: %s", output.String())
	}
	if !strings.Contains(output.String(), "jvm install 21.0.2+13 --source zulu") {
		t.Fatal("install instruction changes source")
	}
}

func TestAliasCatalogQueriesHistoryOnlyForSpecificVersion(t *testing.T) {
	for _, test := range []struct {
		input string
		major int
		all   bool
	}{{"latest", 0, false}, {"lts-2", 0, false}, {"17", 17, false}, {"17.0.10+7", 17, true}} {
		_, _, err := queryAliasCatalog(test.input, aliasOptions{Source: "zulu", Refresh: true}, func(got sources.QueryOptions) (sources.CatalogResult, error) {
			if got.Major != test.major || got.AllVersions != test.all || !got.Refresh {
				t.Fatalf("%s query: %+v", test.input, got)
			}
			return sources.CatalogResult{}, nil
		}, func() (string, error) { t.Fatal("explicit source called default"); return "", nil })
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAliasNamedSourceOverridesDefaultAndRejectsConflicts(t *testing.T) {
	called := false
	query := func(got sources.QueryOptions) (sources.CatalogResult, error) {
		called = true
		if got.Sources[0] != "adoptium" {
			t.Fatalf("wrong source: %+v", got)
		}
		return sources.CatalogResult{}, nil
	}
	defaultSource := func() (string, error) { t.Fatal("source-specific alias consulted default"); return "", nil }
	if _, _, err := queryAliasCatalog("adoptium-latest", aliasOptions{}, query, defaultSource); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("catalog not queried")
	}
	called = false
	if _, _, err := queryAliasCatalog("adoptium-latest", aliasOptions{Source: "zulu"}, query, defaultSource); err == nil {
		t.Fatal("conflicting source accepted")
	}
	if called {
		t.Fatal("conflicting source queried catalog")
	}
}

func TestAliasOfflineExplainKeepsDefinitionWhenCacheMissing(t *testing.T) {
	command := newAliasCommand(func(got sources.QueryOptions) (sources.CatalogResult, error) {
		if !got.Offline {
			t.Fatal("explain did not preserve offline")
		}
		return sources.CatalogResult{}, errors.New("offline cache unavailable")
	}, func() (string, error) { return "zulu", nil })
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--offline", "explain", "lts-2"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "lts-2:") {
		t.Fatal("definition unavailable without catalog")
	}
	if !strings.Contains(stderr.String(), "Resolution unavailable: offline cache unavailable") {
		t.Fatalf("missing resolution error: %s", stderr.String())
	}
}

func TestAliasListAndSuggestInheritSourceFlags(t *testing.T) {
	for _, action := range []string{"list", "suggest"} {
		command := newAliasCommand(func(got sources.QueryOptions) (sources.CatalogResult, error) {
			if got.Sources[0] != "zulu" || !got.Refresh || got.AllVersions {
				t.Fatalf("%s query: %+v", action, got)
			}
			return sources.CatalogResult{Releases: aliasFixtures()}, nil
		}, func() (string, error) { t.Fatal("explicit source consulted default"); return "", nil })
		var stdout bytes.Buffer
		command.SetOut(&stdout)
		command.SetErr(&bytes.Buffer{})
		command.SetArgs([]string{"--source", "zulu", action, "--refresh"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stdout.String(), "26+12") || strings.Contains(stdout.String(), "adoptium-latest") {
			t.Fatalf("%s mixed default vendors: %s", action, stdout.String())
		}
	}
}

func TestAliasConflictingCacheFlagsFailBeforeQuery(t *testing.T) {
	command := newAliasCommand(func(sources.QueryOptions) (sources.CatalogResult, error) {
		t.Fatal("conflicting options queried catalog")
		return sources.CatalogResult{}, nil
	}, func() (string, error) { return "zulu", nil })
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"resolve", "latest", "--refresh", "--offline"})
	if err := command.Execute(); err == nil {
		t.Fatal("conflicting cache options accepted")
	}
}
