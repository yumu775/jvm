package setup_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type invocation struct {
	Executable string
	Arguments  []string
}

// 原生测试程序充当 jvm.exe，绝不调用真实配置命令或访问注册表/profile。
func TestMain(m *testing.M) {
	if os.Getenv("JVM_SETUP_FIXTURE") == "1" {
		fixtureMain()
		return
	}
	os.Exit(m.Run())
}

func fixtureMain() {
	program, _ := os.Executable()
	log, err := os.OpenFile(os.Getenv("JVM_SETUP_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	json.NewEncoder(log).Encode(invocation{program, os.Args[1:]})
	log.Close()
	if len(os.Args) < 2 {
		os.Exit(2)
	}
	if os.Getenv("JVM_SETUP_FAIL_STAGE") == os.Args[1] {
		fmt.Fprintln(os.Stderr, "fixture failure")
		os.Exit(7)
	}
	switch os.Args[1] {
	case "setup":
		for i, arg := range os.Args[2:] {
			if arg == "--path" {
				dir := os.Args[i+3]
				if err := os.MkdirAll(dir, 0755); err != nil {
					os.Exit(2)
				}
				data, err := os.ReadFile(program)
				if err != nil {
					os.Exit(2)
				}
				if err := os.WriteFile(filepath.Join(dir, "jvm.exe"), data, 0755); err != nil {
					os.Exit(2)
				}
			}
		}
		fmt.Println("fixture setup complete")
	case "init":
		fmt.Printf("function global:jvm { & %s @args }\n", psQuote(program))
	case "--version":
		fmt.Println("fixture-jvm 1.0")
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func psQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func prepareFixture(t *testing.T) (string, string, string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Windows setup scripts")
	}
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	_, file, _, _ := runtime.Caller(0)
	source := filepath.Join(filepath.Dir(file), "..", "..", "quick-setup.ps1")
	dir := filepath.Join(t.TempDir(), "owner's $JVM tools")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "quick-setup.ps1"), data, 0600); err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jvm.exe"), data, 0755); err != nil {
		t.Fatal(err)
	}
	return shell, dir, filepath.Join(t.TempDir(), "calls.jsonl")
}

func runFixture(t *testing.T, shell, log, script, fail string) (string, error) {
	t.Helper()
	command := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.Env = append(os.Environ(), "JVM_SETUP_FIXTURE=1", "JVM_SETUP_LOG="+log, "JVM_SETUP_FAIL_STAGE="+fail)
	data, err := command.CombinedOutput()
	return string(data), err
}

func readInvocations(t *testing.T, path string) []invocation {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result []invocation
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var call invocation
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatal(err)
		}
		result = append(result, call)
	}
	return result
}

func TestQuickSetupConfiguresProfileAndActivatesInstalledCopy(t *testing.T) {
	shell, dir, log := prepareFixture(t)
	target := filepath.Join(t.TempDir(), "custom $installed tools")
	script := "$ErrorActionPreference='Stop'; & " + psQuote(filepath.Join(dir, "quick-setup.ps1")) + " -InstallPath " + psQuote(target) + "; if (($env:PATH -split ';')[0] -ne " + psQuote(target) + ") { throw 'target PATH missing' }; jvm --version"
	output, err := runFixture(t, shell, log, script, "")
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, output)
	}
	calls := readInvocations(t, log)
	if len(calls) != 4 {
		t.Fatalf("unexpected commands: %+v", calls)
	}
	args := strings.Join(calls[0].Arguments, "|")
	if !strings.Contains(args, "--force") || !strings.Contains(args, "--powershell-profile") || !strings.Contains(args, target) {
		t.Fatalf("setup omitted configuration: %s", args)
	}
	for _, call := range calls[1:] {
		if !strings.EqualFold(call.Executable, filepath.Join(target, "jvm.exe")) {
			t.Fatalf("used source executable instead of installed copy: %+v", call)
		}
	}
	if !strings.Contains(output, "JVM is ready in this PowerShell session") {
		t.Fatalf("missing readiness: %s", output)
	}
}

func TestQuickSetupNoProfilePreservesOtherPathsAndDeduplicatesTarget(t *testing.T) {
	shell, dir, log := prepareFixture(t)
	initial := `C:\unrelated Java tools;` + dir + `;` + dir + `\`
	script := "$ErrorActionPreference='Stop'; $env:PATH=" + psQuote(initial) + "; & " + psQuote(filepath.Join(dir, "quick-setup.ps1")) + " -NoProfile -Force:$false; if ($env:PATH -ne " + psQuote(dir+`;C:\unrelated Java tools`) + ") { throw ('unexpected PATH: '+$env:PATH) }"
	output, err := runFixture(t, shell, log, script, "")
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, output)
	}
	calls := readInvocations(t, log)
	args := strings.Join(calls[0].Arguments, "|")
	if strings.Contains(args, "--powershell-profile") || strings.Contains(args, "--force") {
		t.Fatalf("ignored opt-out: %s", args)
	}
}

func TestQuickSetupFailureDoesNotActivateOrClaimSuccess(t *testing.T) {
	shell, dir, log := prepareFixture(t)
	script := "$ErrorActionPreference='Stop'; $before=$env:PATH; try { & " + psQuote(filepath.Join(dir, "quick-setup.ps1")) + " -NoProfile; throw 'expected setup failure' } catch { if ($env:PATH -ne $before) { throw 'PATH changed after failed setup' }; if ($_.Exception.Message -notlike '*JVM setup failed*') { throw }; Write-Output 'expected-failure' }"
	output, err := runFixture(t, shell, log, script, "setup")
	if err != nil || !strings.Contains(output, "expected-failure") {
		t.Fatalf("failure handling: %v\n%s", err, output)
	}
	if strings.Contains(output, "JVM is ready") {
		t.Fatal("failed setup reported readiness")
	}
	if calls := readInvocations(t, log); len(calls) != 1 {
		t.Fatalf("activated after failed setup: %+v", calls)
	}
}

func TestCommandLineBatchPreservesArgumentsAndFailureCode(t *testing.T) {
	_, dir, log := prepareFixture(t)
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "setup-jvm.bat"))
	if err != nil {
		t.Fatal(err)
	}
	batch := filepath.Join(dir, "setup-jvm.bat")
	if err := os.WriteFile(batch, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "cmd.exe", "/d", "/c", "setup-jvm.bat", "--force")
	command.Dir = dir
	command.Env = append(os.Environ(), "JVM_SETUP_FIXTURE=1", "JVM_SETUP_LOG="+log, "JVM_SETUP_FAIL_STAGE=setup")
	output, err := command.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 7 {
		t.Fatalf("batch did not preserve exit code: %v\n%s", err, output)
	}
	if ctx.Err() != nil {
		t.Fatal("command-line batch waited for input")
	}
	calls := readInvocations(t, log)
	if len(calls) != 1 || strings.Join(calls[0].Arguments, " ") != "setup --force" {
		t.Fatalf("arguments changed: %+v", calls)
	}
}

func TestDoubleClickLauncherDisplaysFailureAndReturnsNonzero(t *testing.T) {
	_, dir, log := prepareFixture(t)
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "install-jvm.bat"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install-jvm.bat"), data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "cmd.exe", "/d", "/c", "install-jvm.bat")
	command.Dir = dir
	command.Env = append(os.Environ(), "JVM_SETUP_FIXTURE=1", "JVM_SETUP_LOG="+log, "JVM_SETUP_FAIL_STAGE=setup")
	command.Stdin = strings.NewReader("\n")
	output, err := command.CombinedOutput()
	if err == nil || ctx.Err() != nil || !strings.Contains(string(output), "JVM installation failed") {
		t.Fatalf("launcher failure: %v\n%s", err, output)
	}
	if calls := readInvocations(t, log); len(calls) != 1 {
		t.Fatalf("unexpected launcher commands: %+v", calls)
	}
}
