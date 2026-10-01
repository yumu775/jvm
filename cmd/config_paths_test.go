package cmd

import (
	"jvm/internal/config"
	"jvm/internal/version"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestChangingInstallDirectoryKeepsLegacyVersions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	old := filepath.Join(root, "versions", "java-17")
	if err := os.MkdirAll(filepath.Join(old, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	exe := "java"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(filepath.Join(old, "bin", exe), nil, 0755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "new storage")
	if err := setConfig("install-dir", destination); err != nil {
		t.Fatal(err)
	}
	got, err := config.GetVersionsDir()
	if err != nil || got != destination {
		t.Fatalf("directory: %s %v", got, err)
	}
	manager, _ := version.NewManager()
	got, err = manager.GetVersionPath("17")
	if err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(old)
	if got != real {
		t.Fatalf("legacy path lost: %s", got)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("directory switch moved legacy files")
	}
}
