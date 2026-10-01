package download

import (
	"crypto/sha256"
	"fmt"
	"jvm/internal/sources"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApproximatePublisherSizeStillRequiresExactSHA256(t *testing.T) {
	content := strings.Repeat("x", 1965)
	hash := sha256.Sum256([]byte(content))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, content) }))
	defer server.Close()
	// 模拟升级前缓存：已有 SHA-256 和错误的精确 size，因此不会再 Prepare。
	release := &sources.JavaRelease{FileName: "jdk.zip", DownloadURL: server.URL, FileSize: 2000, Checksum: fmt.Sprintf("%x", hash), Metadata: map[string]string{"metadata_provider": "azul", "reported_size": "2000"}}
	if _, err := NewDownloader().DownloadJava(release, t.TempDir()); err != nil {
		t.Fatalf("rounded publisher size rejected verified content: %v", err)
	}
	release.Checksum = strings.Repeat("0", 64)
	if _, err := NewDownloader().DownloadJava(release, t.TempDir()); err == nil {
		t.Fatal("missing exact size must not bypass SHA-256")
	}
}

func TestTransientDownloadFailureRetriesWithoutChangingPackage(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "archive")
	}))
	defer s.Close()
	hash := sha256.Sum256([]byte("archive"))
	d := NewDownloader()
	_, err := d.DownloadJava(&sources.JavaRelease{FileName: "jdk.zip", DownloadURL: s.URL, Checksum: fmt.Sprintf("%x", hash)}, t.TempDir())
	if err != nil || calls != 3 {
		t.Fatalf("retry failed: calls=%d err=%v", calls, err)
	}
}

func TestMissingArchiveIsNotRetriedOrSubstituted(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusNotFound) }))
	defer s.Close()
	hash := sha256.Sum256(nil)
	_, err := NewDownloader().DownloadJava(&sources.JavaRelease{FileName: "jdk.zip", DownloadURL: s.URL, Checksum: fmt.Sprintf("%x", hash)}, t.TempDir())
	if err == nil || calls != 1 {
		t.Fatalf("404 unexpectedly substituted/retried: calls=%d err=%v", calls, err)
	}
}

func TestDownloadIntegrityAndCache(t *testing.T) {
	content := "archive bytes"
	hash := sha256.Sum256([]byte(content))
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, content) }))
	defer s.Close()
	release := &sources.JavaRelease{FileName: "jdk.zip", DownloadURL: s.URL, Checksum: fmt.Sprintf("%x", hash), FileSize: int64(len(content))}
	d := NewDownloader()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "jdk.zip"), []byte("broken"), 0600)
	path, err := d.DownloadJava(release, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != content {
		t.Fatal("bad cache reused")
	}
	if _, err = d.DownloadJava(release, dir); err != nil || calls != 1 {
		t.Fatalf("valid cache not reused %v", err)
	}
	release.FileName = "bad.zip"
	release.Checksum = fmt.Sprintf("%064d", 0)
	if _, err = d.DownloadJava(release, dir); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("failed download left files")
	}
	release.FileName = "../escape.zip"
	if _, err = d.DownloadJava(release, dir); err == nil {
		t.Fatal("path escape accepted")
	}
}
