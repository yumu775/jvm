package sources

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testArchiveName() string {
	if runtime.GOOS == "windows" {
		return "jdk.zip"
	}
	return "jdk.tar.gz"
}

func TestZuluMetadataIsLazyAndUsesRealDetailChecksum(t *testing.T) {
	name := testArchiveName()
	detailCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/packages/package-id/") {
			detailCalls++
			osName, _, _, _ := zuluPlatform()
			arch := "x86"
			bits := 64
			if runtime.GOARCH == "arm64" {
				arch = "arm"
			}
			if runtime.GOARCH == "386" {
				bits = 32
			}
			json.NewEncoder(w).Encode(zuluPackage{Name: name, DownloadURL: "https://cdn.azul.com/" + name, SHA256: strings.Repeat("a", 64), Size: 123, JavaVersion: []int{17, 0, 20, 1}, Build: 1, OS: osName, Arch: arch, Bitness: bits})
			return
		}
		osName, arch, archive, _ := zuluPlatform()
		q := r.URL.Query()
		if q.Get("os") != osName || q.Get("arch") != arch || q.Get("archive_type") != archive || q.Get("java_version") != "17" || q.Get("latest") != "true" || q.Get("crac_supported") != "false" {
			t.Error(r.URL.String())
		}
		json.NewEncoder(w).Encode([]zuluPackage{{Name: name, DownloadURL: "https://cdn.azul.com/" + name, JavaVersion: []int{17, 0, 20, 1}, Build: 1, UUID: "package-id"}})
	}))
	defer server.Close()
	sm := NewSourceManager()
	releases, err := sm.fetchVersions(JavaSource{Name: "zulu", APIType: "zulu", BaseURL: server.URL}, 17, false)
	if err != nil || len(releases) != 1 {
		t.Fatalf("%+v %v", releases, err)
	}
	if detailCalls != 0 || releases[0].Checksum != "" || releases[0].Version != "17.0.20.1+1" {
		t.Fatalf("eager or fabricated metadata: %+v", releases)
	}
	if err = sm.PrepareRelease(&releases[0]); err != nil {
		t.Fatal(err)
	}
	if detailCalls != 1 || releases[0].Checksum != strings.Repeat("a", 64) || releases[0].FileSize != 0 || releases[0].Metadata["reported_size"] != "123" {
		t.Fatal("detail was not used")
	}
}

func TestAdoptiumFallsBackToSameVendorOnly(t *testing.T) {
	name := testArchiveName()
	detailCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/info/available_releases" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ids/") {
			detailCalls++
			fmt.Fprintf(w, `{"result":[{"filename":%q,"direct_download_uri":"https://example.test/download","checksum":%q,"checksum_type":"sha256"}]}`, name, strings.Repeat("b", 64))
			return
		}
		q := r.URL.Query()
		if q.Get("distribution") != "temurin" || q.Get("version") != "17" || q.Get("latest") != "available" {
			t.Error(r.URL.String())
		}
		osName, arch := getOSArch()
		if osName == "mac" {
			osName = "macos"
		}
		archive := "tar.gz"
		if runtime.GOOS == "windows" {
			archive = "zip"
		}
		good := discoPackage{ID: "temurin-id", Distribution: "temurin", Major: 17, JavaVersion: "17.0.20.1+1", Filename: name, Size: 123, Support: "lts", OS: osName, Architecture: arch, Archive: archive, PackageType: "jdk", Status: "ga"}
		other := good
		other.Distribution = "corretto"
		other.ID = "wrong-vendor"
		json.NewEncoder(w).Encode(struct {
			Result []discoPackage `json:"result"`
		}{[]discoPackage{good, other}})
	}))
	defer server.Close()
	sm := NewSourceManager()
	sm.foojayBase = server.URL
	releases, err := sm.fetchVersions(JavaSource{Name: "adoptium", APIType: "adoptium", BaseURL: server.URL}, 17, false)
	if err != nil || len(releases) != 1 {
		t.Fatalf("%+v %v", releases, err)
	}
	r := &releases[0]
	if r.Source != "adoptium" || r.Version != "17.0.20.1+1" || r.Metadata["metadata_provider"] != "foojay" || r.Metadata["fallback_reason"] == "" || detailCalls != 0 {
		t.Fatalf("unexpected fallback: %+v", r)
	}
	if err = sm.PrepareRelease(r); err != nil {
		t.Fatal(err)
	}
	if detailCalls != 1 || r.Checksum != strings.Repeat("b", 64) {
		t.Fatal("detail checksum missing")
	}
}

func TestPrepareRejectsPackageSubstitutionAndWeakChecksum(t *testing.T) {
	for _, typ := range []string{"sha1", "sha256"} {
		t.Run(typ, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"result":[{"filename":"wrong.zip","direct_download_uri":"https://example.test/a","checksum":%q,"checksum_type":%q}]}`, strings.Repeat("a", 64), typ)
			}))
			defer server.Close()
			r := JavaRelease{Version: "17.0.1+1", MajorVersion: 17, FileName: testArchiveName(), Metadata: map[string]string{"metadata_provider": "foojay", "detail_url": server.URL}}
			if err := NewSourceManager().PrepareRelease(&r); err == nil {
				t.Fatal("package substitution or weak hash accepted")
			}
		})
	}
}

func TestSourceSettingsMigrateAndKeepDefaultSeparateFromPriority(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	if err := os.WriteFile(filepath.Join(root, "sources.json"), []byte(`{"adoptium":true,"zulu":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	sm := NewSourceManager()
	if err := sm.SetDefault("zulu"); err == nil {
		t.Fatal("disabled default accepted")
	}
	if err := sm.SetPriority("zulu", 0); err != nil {
		t.Fatal(err)
	}
	if got, err := sm.DefaultSource(); err != nil || got != "adoptium" {
		t.Fatal("priority changed default")
	}
	if err := sm.SetEnabled("zulu", true); err != nil {
		t.Fatal(err)
	}
	if err := sm.SetDefault("zulu"); err != nil {
		t.Fatal(err)
	}
	if got, err := sm.DefaultSource(); err != nil || got != "zulu" {
		t.Fatalf("default not persisted %s %v", got, err)
	}
	all, err := sm.LoadSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.Name == "zulu" && (!s.Enabled || s.Priority != 0) {
			t.Fatalf("migration lost settings: %+v", s)
		}
	}
}

func TestCorrettoUsesExactOfficialReleaseChecksumRow(t *testing.T) {
	name := testArchiveName()
	downloadURL := "https://corretto.aws/downloads/resources/17.0.20.8.1/" + name
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ids/pkg" {
			fmt.Fprintf(w, `{"result":[{"filename":%q,"direct_download_uri":%q,"checksum":"","checksum_type":""}]}`, name, downloadURL)
			return
		}
		if r.URL.Path != "/repos/corretto/corretto-17/releases/tags/17.0.20.8.1" {
			t.Error(r.URL.Path)
		}
		body := "|[other](https://example.test/other)|" + strings.Repeat("f", 64) + "|\n|[" + name + "](" + downloadURL + ")|" + strings.Repeat("a", 32) + " / " + strings.Repeat("c", 64) + "|"
		json.NewEncoder(w).Encode(map[string]string{"body": body})
	}))
	defer server.Close()
	sm := NewSourceManager()
	sm.githubBase = server.URL
	r := JavaRelease{Version: "17.0.20", MajorVersion: 17, FileName: name, Metadata: map[string]string{"metadata_provider": "foojay", "distribution": "corretto", "detail_url": server.URL + "/ids/pkg"}}
	if err := sm.PrepareRelease(&r); err != nil {
		t.Fatal(err)
	}
	if r.Checksum != strings.Repeat("c", 64) {
		t.Fatal("used wrong package or MD5 checksum")
	}
}

func TestPublishedChecksumFileNamesAreChecked(t *testing.T) {
	name := testArchiveName()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s other.zip\n%s *%s\n", strings.Repeat("a", 64), strings.Repeat("d", 64), name)
	}))
	defer server.Close()
	got, err := NewSourceManager().readPublishedChecksum(server.URL, name)
	if err != nil || got != strings.Repeat("d", 64) {
		t.Fatalf("%s %v", got, err)
	}
}

// 此测试只有显式开启时读取公开元数据，从不下载或执行 JDK。
func TestLiveProviderMetadata(t *testing.T) {
	if os.Getenv("JVM_LIVE_METADATA") != "1" {
		t.Skip("set JVM_LIVE_METADATA=1 to check public provider metadata")
	}
	sm := NewSourceManager()
	for _, s := range sm.GetDefaultSources() {
		if s.Name != "zulu" && s.Name != "corretto" && s.Name != "graalvm" {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			major := 17
			if s.Name == "graalvm" {
				major = 21
			}
			releases, err := sm.fetchVersions(s, major, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(releases) == 0 {
				t.Fatal("empty catalog")
			}
			r := releases[0]
			if err = sm.PrepareRelease(&r); err != nil {
				t.Fatal(err)
			}
			t.Logf("provider=%s source=%s version=%s file=%s size=%d sha256=%s", r.Metadata["metadata_provider"], r.Source, r.Version, r.FileName, r.FileSize, r.Checksum)
		})
	}
}
