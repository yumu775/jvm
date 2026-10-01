package download

import (
	"errors"
	"testing"

	"jvm/internal/sources"
)

func TestDefaultInstallDoesNotSilentlyChangeDistribution(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	d := NewDownloader()
	d.query = func(options sources.QueryOptions) (sources.CatalogResult, error) {
		if len(options.Sources) != 1 || options.Sources[0] != "adoptium" || options.Major != 17 {
			t.Fatalf("unexpected default selection: %+v", options)
		}
		return sources.CatalogResult{}, errors.New("Temurin temporarily unavailable")
	}
	if _, err := d.FindVersion("17", ""); err == nil {
		t.Fatal("silently substituted a different distribution")
	}
}

func TestInstallUsesConfiguredDefaultAndLimitsQuery(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	d := NewDownloader()
	if err := d.sourceManager.SetDefault("zulu"); err != nil {
		t.Fatal(err)
	}
	d.query = func(options sources.QueryOptions) (sources.CatalogResult, error) {
		if len(options.Sources) != 1 || options.Sources[0] != "zulu" || options.Major != 17 || !options.AllVersions {
			t.Fatalf("unexpected query: %+v", options)
		}
		return sources.CatalogResult{Releases: []sources.JavaRelease{{Version: "17.0.12+7", FullVersion: "17.0.12+7", MajorVersion: 17, Source: "zulu"}}}, nil
	}
	r, err := d.FindVersion("17.0.12", "")
	if err != nil || r.Source != "zulu" {
		t.Fatalf("default not respected: %+v %v", r, err)
	}
}
