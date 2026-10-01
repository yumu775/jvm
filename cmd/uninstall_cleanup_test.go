package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jvm/internal/config"
)

func TestUninstallCleanPreviewsThenRemovesOnlyStaleReferences(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Installations = map[string]config.Installation{"17": {Path: filepath.Join(t.TempDir(), "missing"), Managed: false}}
	cfg.CurrentVersion = "17"
	if err := cfg.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	path, _ := config.GetConfigPath()
	before, _ := os.ReadFile(path)
	stdout, stderr, err := runCLI(t, "uninstall", "clean", "--dry-run")
	if err != nil || !strings.Contains(stdout, "Would remove registration") || !strings.Contains(stdout, "Dry run") {
		t.Fatalf("preview: %q %q %v", stdout, stderr, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("preview modified configuration")
	}
	stdout, stderr, err = runCLI(t, "uninstall", "clean")
	if err != nil || !strings.Contains(stdout, "Removed registration") || !strings.Contains(stdout, "caches retained") {
		t.Fatalf("cleanup: %q %q %v", stdout, stderr, err)
	}
	cfg, err = config.LoadConfig()
	if err != nil || len(cfg.Installations) != 0 || cfg.CurrentVersion != "" {
		t.Fatalf("stale references remain: %+v %v", cfg, err)
	}
}
