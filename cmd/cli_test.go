package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"jvm/internal/config"
)

// TestCLIHelper 在独立进程中运行命令，避免 Cobra 全局参数在用例间残留。
func TestCLIHelper(t *testing.T) {
	if os.Getenv("JVM_CLI_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			rootCmd.SetArgs(os.Args[i+1:])
			if err := Execute(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	arguments := append([]string{"-test.run=^TestCLIHelper$", "--"}, args...)
	command := exec.Command(executable, arguments...)
	command.Env = append(os.Environ(), "JVM_CLI_TEST_HELPER=1", "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	return stdout.String(), stderr.String(), err
}

func TestCLITemporaryActivationPreservesSavedSelection(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	exe := "java"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	javaHome := filepath.Join(t.TempDir(), "owner's $JDK")
	if err := os.MkdirAll(filepath.Join(javaHome, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(javaHome, "bin", exe), nil, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.CurrentVersion = "21"
	cfg.Installations = map[string]config.Installation{"17.0.12+7": {Path: javaHome, Managed: false}}
	if err := cfg.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	path, _ := config.GetConfigPath()
	before, _ := os.ReadFile(path)
	stdout, stderr, err := runCLI(t, "use", "17", "--temp", "--shell", "powershell")
	if err != nil {
		t.Fatalf("temporary activation: %v %s", err, stderr)
	}
	if !strings.HasPrefix(stdout, "$env:JAVA_HOME = '") || !strings.Contains(stdout, "owner''s $JDK") {
		t.Fatalf("not a pure, correctly quoted activation script: %s", stdout)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("temporary activation changed persistent selection")
	}
}

func TestCLIFailedActivationReturnsNonzeroAndNoScript(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	stdout, stderr, err := runCLI(t, "use", "999", "--temp", "--shell", "powershell")
	if err == nil || stdout != "" || !strings.Contains(stderr, "not installed") {
		t.Fatalf("expected clean failure, stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

func TestCLIUnsupportedSourceCannotClaimSuccess(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	stdout, stderr, err := runCLI(t, "sources", "enable", "oracle")
	if err == nil || strings.Contains(stdout, "enabled") || (!strings.Contains(stderr, "not supported") && !strings.Contains(stderr, "manual import")) {
		t.Fatalf("expected unsupported error, stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}
