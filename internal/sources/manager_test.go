package sources

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseMetadataRejectsInstallerAndMalformedIdentity(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	for _, tc := range []struct {
		version, name string
		valid         bool
	}{
		{"17.0.20+101", "OpenJDK17U-jdk_x64_windows_hotspot_17.0.20.1_1.zip", true},
		{"17.0.20+101", "jdk.msi", false},
		{"17.0.20+101", "../jdk.zip", false},
		{"../17", "jdk.zip", false},
		{"21.0.1+1", "jdk.zip", false},
	} {
		if got := validReleaseMetadata(tc.version, 17, 17, tc.name, checksum, "https://example.test/jdk.zip", "windows"); got != tc.valid {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
	if !validReleaseMetadata("21.0.12+101.0.LTS", 21, 21, "jdk.tar.gz", checksum, "https://example.test/jdk.tar.gz", "mac") {
		t.Fatal("valid macOS LTS build metadata rejected")
	}
}

func TestMetadataAndPersistence(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	sm := NewSourceManager()
	if err := sm.SetEnabled("adoptium", false); err != nil {
		t.Fatal(err)
	}
	all, err := sm.LoadSources()
	if err != nil || all[0].Enabled {
		t.Fatalf("persistence %v %v", all, err)
	}
	if sm.SetEnabled("typo", true) == nil || sm.SetEnabled("oracle", true) == nil {
		t.Fatal("unsupported source accepted")
	}
	if err := sm.SetEnabled("adoptium", true); err != nil {
		t.Fatal(err)
	}
	all, err = sm.LoadSources()
	if err != nil || !all[0].Enabled {
		t.Fatal("reenable was not persisted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/info/available_releases" {
			fmt.Fprint(w, `{"available_releases":[17],"available_lts_releases":[17]}`)
			return
		}
		if r.URL.Query().Get("image_type") != "jdk" || r.URL.Query().Get("page") != "0" {
			t.Error(r.URL)
		}
		name := "jdk.tar.gz"
		platform, _ := getOSArch()
		if platform == "windows" {
			name = "jdk.zip"
		}
		fmt.Fprintf(w, `[{"version_data":{"semver":"17.0.10+7","major":17},"binaries":[{"package":{"name":%q,"link":"https://example.test/jdk.zip","checksum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":42}}]}]`, name)
	}))
	defer server.Close()
	result, err := sm.GetAvailableVersions([]JavaSource{{Name: "adoptium", APIType: "adoptium", BaseURL: server.URL, Enabled: true}})
	if err != nil || len(result) != 1 || result[0].Version != "17.0.10+7" || !result[0].LTS {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestDedupKeepsPatchVersionsAndNumericOrder(t *testing.T) {
	releases := []JavaRelease{
		{Source: "adoptium", FullVersion: "17.0.9+9"},
		{Source: "adoptium", FullVersion: "17.0.10+7"},
		{Source: "adoptium", FullVersion: "17.0.10+7"},
	}
	got := NewSourceManager().deduplicateReleases(releases)
	if len(got) != 2 || got[0].FullVersion != "17.0.10+7" {
		t.Fatalf("%+v", got)
	}
}

func TestMetadataHTTPFailure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer s.Close()
	sm := NewSourceManager()
	sm.foojayBase = s.URL
	if _, err := sm.GetAvailableVersions([]JavaSource{{Name: "adoptium", APIType: "adoptium", BaseURL: s.URL, Enabled: true}}); err == nil {
		t.Fatal("HTTP failure ignored")
	}
}
