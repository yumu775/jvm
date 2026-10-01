package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConfiguredDirectoriesAndLegacyDefaults(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"current_version":"17"}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.InstallDir != filepath.Join(root, "versions") || !c.AutoScan {
		t.Fatalf("legacy defaults missing: %+v", c)
	}
	c.InstallDir = filepath.Join(root, "other disk", "jdk")
	c.DownloadDir = filepath.Join(root, "cache")
	if err = c.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetVersionsDir(); got != c.InstallDir {
		t.Fatalf("install-dir ignored: %s", got)
	}
	if got, _ := GetDownloadsDir(); got != c.DownloadDir {
		t.Fatalf("download-dir ignored: %s", got)
	}
}

func TestConcurrentUpdatesDoNotLoseRegistrations(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	var wg sync.WaitGroup
	for _, v := range []string{"8", "11", "17", "21"} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			err := Update(func(c *Config) error {
				if c.Installations == nil {
					c.Installations = map[string]Installation{}
				}
				c.Installations[v] = Installation{Path: filepath.Join(os.TempDir(), v)}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}(v)
	}
	wg.Wait()
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Installations) != 4 {
		t.Fatalf("lost updates: %+v", c.Installations)
	}
}
