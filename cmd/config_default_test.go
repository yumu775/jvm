package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"jvm/internal/config"
)

func TestDefaultVersionResolvesInstalledIDAndRejectsInvalidSelection(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "jdk")
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	executable := "java"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(filepath.Join(home, "bin", executable), nil, 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Installations = map[string]config.Installation{"17.0.12+7": {Path: home, Managed: false}}
	if err := cfg.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	if err := setConfig("default-version", "17"); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadConfig()
	if err != nil || cfg.DefaultVersion != "17.0.12+7" {
		t.Fatalf("not resolved: %#v %v", cfg, err)
	}
	for _, value := range []string{"default", "../jdk", "999"} {
		if err := setConfig("default-version", value); err == nil {
			t.Fatalf("accepted invalid selection %q", value)
		}
		cfg, err = config.LoadConfig()
		if err != nil || cfg.DefaultVersion != "17.0.12+7" {
			t.Fatalf("failed selection changed default: %#v %v", cfg, err)
		}
	}
}
