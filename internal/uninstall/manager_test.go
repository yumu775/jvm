package uninstall

import (
	"errors"
	"jvm/internal/config"
	"jvm/internal/version"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUninstallOwnershipAndCurrentCleanup(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(map[bool]string{false: "external", true: "managed"}[managed], func(t *testing.T) {
			t.Setenv("JVM_HOME", t.TempDir())
			jdk := filepath.Join(t.TempDir(), "java-17")
			if err := os.MkdirAll(filepath.Join(jdk, "bin"), 0755); err != nil {
				t.Fatal(err)
			}
			exe := "java"
			if runtime.GOOS == "windows" {
				exe += ".exe"
			}
			if err := os.WriteFile(filepath.Join(jdk, "bin", exe), nil, 0755); err != nil {
				t.Fatal(err)
			}
			vm, _ := version.NewManager()
			if err := vm.Register("17", jdk, managed); err != nil {
				t.Fatal(err)
			}
			if err := vm.SetCurrent("17"); err != nil {
				t.Fatal(err)
			}
			um, _ := NewManager()
			um.readJavaUsage = func() ([]javaUsage, error) { return nil, nil }
			cleared := false
			um.clearEnvironment = func(string) error { cleared = true; return nil }
			if _, err := um.Uninstall(UninstallOptions{Version: "17"}); err == nil {
				t.Fatal("active uninstall did not require force")
			}
			if _, err := um.Uninstall(UninstallOptions{Version: "17", Force: true, DryRun: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(jdk); err != nil {
				t.Fatal("dry run deleted installation")
			}
			if cleared {
				t.Fatal("dry run changed environment")
			}
			if managed {
				lock := jdk + ".install-lock"
				if err := os.WriteFile(lock, nil, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := um.Uninstall(UninstallOptions{Version: "17", Force: true}); err == nil {
					t.Fatal("uninstall ignored installation lock")
				}
				if cleared {
					t.Fatal("locked uninstall changed environment")
				}
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := um.Uninstall(UninstallOptions{Version: "17", Force: true}); err != nil {
				t.Fatal(err)
			}
			if !cleared {
				t.Fatal("active environment not cleared")
			}
			_, err := os.Stat(jdk)
			if managed && !os.IsNotExist(err) {
				t.Fatal("managed installation retained")
			}
			if !managed && err != nil {
				t.Fatal("external installation deleted")
			}
			c, _ := config.LoadConfig()
			if c.CurrentVersion != "" || len(c.Installations) != 0 {
				t.Fatalf("stale config: %+v", c)
			}
		})
	}
}

func setupUninstallFixture(t *testing.T, managed bool) (*Manager, string) {
	t.Helper()
	t.Setenv("JVM_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "jdk-B")
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
	vm, err := version.NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Register("B", home, managed); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error { c.CurrentVersion = "A"; return nil }); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	manager.readJavaUsage = func() ([]javaUsage, error) { return nil, nil }
	manager.clearEnvironment = func(string) error { return nil }
	return manager, home
}

func TestUninstallProtectsActualEnvironmentWhenManagedSelectionDiffers(t *testing.T) {
	for _, source := range []string{"persistent user JAVA_HOME", "current process JAVA_HOME", "current process PATH"} {
		for _, managed := range []bool{false, true} {
			t.Run(source+map[bool]string{false: "/external", true: "/managed"}[managed], func(t *testing.T) {
				manager, home := setupUninstallFixture(t, managed)
				manager.readJavaUsage = func() ([]javaUsage, error) { return []javaUsage{{source, home}}, nil }
				clearCalls := 0
				manager.clearEnvironment = func(path string) error {
					clearCalls++
					if !sameJavaHome(path, home) {
						t.Fatalf("wrong environment target: %s", path)
					}
					if _, err := os.Stat(home); err != nil {
						t.Fatal("directory deleted before clearing environment")
					}
					return nil
				}
				if _, err := manager.Uninstall(UninstallOptions{Version: "B"}); err == nil || !strings.Contains(err.Error(), source) {
					t.Fatalf("actual environment did not require force: %v", err)
				}
				if _, err := manager.Uninstall(UninstallOptions{Version: "B", Force: true, DryRun: true}); err != nil {
					t.Fatal(err)
				}
				if clearCalls != 0 {
					t.Fatal("blocked/dry-run uninstall changed environment")
				}
				if _, err := manager.Uninstall(UninstallOptions{Version: "B", Force: true}); err != nil {
					t.Fatal(err)
				}
				if clearCalls != 1 {
					t.Fatalf("matching environment not cleared: %d", clearCalls)
				}
				_, err := os.Stat(home)
				if managed && !os.IsNotExist(err) {
					t.Fatal("managed installation remains")
				}
				if !managed && err != nil {
					t.Fatal("external files deleted")
				}
				cfg, err := config.LoadConfig()
				if err != nil {
					t.Fatal(err)
				}
				if cfg.CurrentVersion != "A" {
					t.Fatal("unrelated selected version changed")
				}
			})
		}
	}
}

func TestUninstallKeepsFilesAndRegistrationWhenEnvironmentCannotBeVerifiedOrCleared(t *testing.T) {
	for _, stage := range []string{"read", "clear"} {
		t.Run(stage, func(t *testing.T) {
			manager, home := setupUninstallFixture(t, true)
			if stage == "read" {
				manager.readJavaUsage = func() ([]javaUsage, error) { return nil, errors.New("fixture permission failure") }
			} else {
				manager.clearEnvironment = func(string) error { return errors.New("fixture write failure") }
			}
			if _, err := manager.Uninstall(UninstallOptions{Version: "B", Force: true}); err == nil {
				t.Fatal("environment failure swallowed")
			}
			if _, err := os.Stat(home); err != nil {
				t.Fatal("environment failure deleted installation")
			}
			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := cfg.Installations["B"]; !ok {
				t.Fatal("environment failure removed registration")
			}
		})
	}
}
