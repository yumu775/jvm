package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("JVM_SCANNER_TEST_HELPER"); mode != "" {
		if mode == "hang" {
			time.Sleep(time.Minute)
		}
		fmt.Fprintln(os.Stderr, `openjdk version "17.0.10" 2024-01-16`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperJDK(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	name := "java"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err = os.WriteFile(filepath.Join(bin, name), data, 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCustomPathIncludesJDKRoot(t *testing.T) {
	root := helperJDK(t)
	t.Setenv("JVM_SCANNER_TEST_HELPER", "version")
	items, err := NewScanner().ScanCustomPaths([]string{root, root})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Version != "17.0.10" {
		t.Fatalf("root scan: %+v", items)
	}
}

func TestUnresponsiveJavaHasTimeout(t *testing.T) {
	root := helperJDK(t)
	t.Setenv("JVM_SCANNER_TEST_HELPER", "hang")
	start := time.Now()
	if item := NewScanner().AnalyzeJavaInstallation(root); item != nil {
		t.Fatal("accepted hanging executable")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("scan exceeded timeout: %v", elapsed)
	}
}

func TestMacBundleMetadataWithoutExecutingJava(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "jdk.jdk", "Contents", "Home")
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"java", "javac"} {
		if err := os.WriteFile(filepath.Join(home, "bin", name+suffix), []byte("not executable"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "release"), []byte("JAVA_VERSION=\"21.0.12\"\nIMPLEMENTOR=\"Eclipse Adoptium\"\nOS_ARCH=\"aarch64\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewScanner()
	item := s.AnalyzeJavaInstallation(filepath.Join(root, "jdk.jdk"))
	if item == nil || item.Path != home || item.Type != "JDK" || item.Architecture != "aarch64" {
		t.Fatalf("%+v", item)
	}
	items, err := s.ScanCustomPaths([]string{root, home})
	if err != nil || len(items) != 1 {
		t.Fatalf("%+v %v", items, err)
	}
}
