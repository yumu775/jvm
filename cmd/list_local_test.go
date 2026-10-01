package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"jvm/internal/config"
	"jvm/internal/scanner"
)

func TestLocalListHonorsAutoScanWithoutScanningManagedVersions(t *testing.T) {
	previousScan := scanListedSystemJava
	previousSystem, previousAll := showSystemVersions, showAllVersions
	t.Cleanup(func() {
		scanListedSystemJava = previousScan
		showSystemVersions = previousSystem
		showAllVersions = previousAll
	})
	for _, test := range []struct {
		name                             string
		managed, auto, system, all, scan bool
	}{
		{name: "empty auto enabled", auto: true, scan: true},
		{name: "empty auto disabled"},
		{name: "managed auto enabled", managed: true, auto: true},
		{name: "explicit system", managed: true, system: true, scan: true},
		{name: "explicit all", all: true, scan: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JVM_HOME", t.TempDir())
			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			cfg.AutoScan = test.auto
			if err = cfg.SaveConfig(); err != nil {
				t.Fatal(err)
			}
			if test.managed {
				bin := filepath.Join(cfg.InstallDir, "java-17", "bin")
				if err = os.MkdirAll(bin, 0755); err != nil {
					t.Fatal(err)
				}
				name := "java"
				if runtime.GOOS == "windows" {
					name += ".exe"
				}
				if err = os.WriteFile(filepath.Join(bin, name), nil, 0755); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			scanListedSystemJava = func() ([]scanner.JavaInstallation, error) { called = true; return nil, nil }
			showSystemVersions, showAllVersions = test.system, test.all
			if err = listCmd.RunE(listCmd, nil); err != nil {
				t.Fatal(err)
			}
			if called != test.scan {
				t.Fatalf("scanned=%v want %v", called, test.scan)
			}
		})
	}
}
