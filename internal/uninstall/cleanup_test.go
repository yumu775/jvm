package uninstall

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jvm/internal/config"
)

func TestCleanupDryRunAndTransactionPreserveExistingFiles(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-jdk")
	if err := os.MkdirAll(external, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(external, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-jdk")
	cfg.Installations = map[string]config.Installation{"17": {Path: missing, Managed: true}, "21": {Path: external, Managed: false}}
	cfg.CurrentVersion = "17"
	cfg.DefaultVersion = "21"
	if err := cfg.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	path, err := config.GetConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := CleanupReferences(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.RemovedRegistrations) != 1 || len(preview.ClearedReferences) != 1 {
		t.Fatalf("wrong preview: %+v", preview)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("dry run changed configuration")
	}
	result, err := CleanupReferences(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedRegistrations) != 1 {
		t.Fatalf("wrong result: %+v", result)
	}
	cfg, err = config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentVersion != "" || cfg.DefaultVersion != "21" || len(cfg.Installations) != 1 {
		t.Fatalf("wrong retained references: %+v", cfg)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "keep" {
		t.Fatal("external file was changed")
	}
}

func TestCleanupRetainsPermissionErrorsAndActiveInstallations(t *testing.T) {
	root := t.TempDir()
	denied := filepath.Join(root, "denied")
	installing := filepath.Join(root, "installing")
	if err := os.WriteFile(installing+".install-lock", nil, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{InstallDir: root, CurrentVersion: "17", DefaultVersion: "21", Installations: map[string]config.Installation{"17": {Path: denied}, "21": {Path: installing}}}
	stat := func(path string) (os.FileInfo, error) {
		if path == denied {
			return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrPermission}
		}
		return os.Stat(path)
	}
	result := cleanReferences(cfg, stat, os.ReadDir)
	if len(result.RemovedRegistrations) != 0 || len(result.ClearedReferences) != 0 || len(cfg.Installations) != 2 {
		t.Fatalf("uncertain paths removed: %+v", result)
	}
	joined := strings.Join(result.Retained, "\n")
	if !strings.Contains(joined, "cannot inspect") || !strings.Contains(joined, "in progress") {
		t.Fatalf("missing retention reason: %s", joined)
	}
}

func TestCleanupPreservesLegacyVersionReferences(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "java-17.0.12"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{InstallDir: root, CurrentVersion: "17", DefaultVersion: "999", Installations: map[string]config.Installation{}}
	result := cleanReferences(cfg, os.Stat, os.ReadDir)
	if cfg.CurrentVersion != "17" || cfg.DefaultVersion != "" || len(result.ClearedReferences) != 1 {
		t.Fatalf("incorrect legacy cleanup: %+v %+v", cfg, result)
	}
}

func TestCleanupCannotTreatUnreadableLegacyDirectoryAsEmpty(t *testing.T) {
	cfg := &config.Config{InstallDir: t.TempDir(), CurrentVersion: "17", Installations: map[string]config.Installation{}}
	readDir := func(path string) ([]os.DirEntry, error) {
		return nil, &os.PathError{Op: "readdir", Path: path, Err: os.ErrPermission}
	}
	result := cleanReferences(cfg, os.Stat, readDir)
	if cfg.CurrentVersion != "17" || len(result.ClearedReferences) != 0 || len(result.Retained) == 0 {
		t.Fatalf("permission failure treated as empty directory: %+v", result)
	}
}
