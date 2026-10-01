package env

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdatePathKeepsUnrelatedTools(t *testing.T) {
	input := `E:\tools\javascript\bin;D:\sdk\jdk-17\bin;C:\Windows\System32;D:\sdk\jdk-21\bin`
	got := UpdatePath(input, `D:\sdk\jdk-21\bin`, `D:\sdk\jdk-17\bin`, true)
	want := `D:\sdk\jdk-21\bin;E:\tools\javascript\bin;C:\Windows\System32`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if again := UpdatePath(got, `D:\sdk\jdk-21\bin`, `D:\sdk\jdk-17\bin`, true); again != got {
		t.Fatalf("not idempotent: %s", again)
	}
}

func TestToolPathUpdatePreservesJavaHomeReference(t *testing.T) {
	input := `%JAVA_HOME%\bin;C:\Windows\System32`
	got := UpdatePath(input, `D:\tools\jvm`, "", true)
	if got != `D:\tools\jvm;`+input {
		t.Fatalf("setup removed Java PATH: %q", got)
	}
	got = UpdatePath(got, "", `D:\tools\jvm`, true)
	if got != input {
		t.Fatalf("setup uninstall removed Java PATH: %q", got)
	}
}

func TestPowerShellActivationQuotesLiteralPaths(t *testing.T) {
	home := `D:\SDK's\$Java`
	script, err := ActivationScript("powershell", home, `C:\Windows\System32`, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `$env:JAVA_HOME = 'D:\SDK''s\$Java'`) {
		t.Fatalf("unsafe quoting: %s", script)
	}
	if strings.Contains(script, "success") || strings.Contains(script, "Warning") {
		t.Fatal("diagnostics in script")
	}
}

func TestActivationRejectsExecutableControlCharacters(t *testing.T) {
	for _, shell := range []string{"powershell", "cmd", "bash", "fish"} {
		if _, err := ActivationScript(shell, "jdk\nmalicious", "", ""); err == nil {
			t.Errorf("%s accepted newline", shell)
		}
	}
	for _, value := range []string{`D:\%bad%`, `D:\!bad!`} {
		if _, err := ActivationScript("cmd", value, "", ""); err == nil {
			t.Errorf("cmd accepted %q", value)
		}
	}
}

func TestProfileBlockPreservesOtherSettingsAndRejectsDamage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile")
	original := "personal setting\n# JVM Java Version Manager - START\nold\n# JVM Java Version Manager - END\nlast setting\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfileBlock(path, "JVM Java Version Manager", "new"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "personal setting") || !strings.Contains(string(data), "last setting") || strings.Contains(string(data), "\nold\n") {
		t.Fatalf("bad profile: %s", data)
	}
	damaged := "personal\n# JVM Java Version Manager - START\nunfinished"
	if err := os.WriteFile(path, []byte(damaged), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfileBlock(path, "JVM Java Version Manager", "new"); err == nil {
		t.Fatal("accepted damaged profile")
	}
	data, _ = os.ReadFile(path)
	if string(data) != damaged {
		t.Fatal("damaged profile overwritten")
	}
}

type memoryEnvironment struct {
	values   map[string]savedValue
	failName string
}

func (s *memoryEnvironment) Read(name string) (string, uint32, bool, error) {
	v, ok := s.values[name]
	return v.value, v.kind, ok, nil
}
func (s *memoryEnvironment) Write(name, value string, kind uint32) error {
	if s.failName == name {
		return errors.New("injected failure")
	}
	s.values[name] = savedValue{value, kind, true}
	return nil
}
func (s *memoryEnvironment) Delete(name string) error { delete(s.values, name); return nil }

func TestPersistentEnvironmentRollsBackJavaHomeOnPathFailure(t *testing.T) {
	original := savedValue{`D:\jdk-old`, 2, true}
	store := &memoryEnvironment{values: map[string]savedValue{"JAVA_HOME": original, "Path": {`C:\tools`, 2, true}}, failName: "Path"}
	if err := persistJava(store, `D:\jdk-new`); err == nil {
		t.Fatal("failure swallowed")
	}
	if store.values["JAVA_HOME"] != original {
		t.Fatal("JAVA_HOME not restored")
	}
	if store.values["Path"].value != `C:\tools` {
		t.Fatal("Path altered")
	}
}

func TestPersistentEnvironmentPreservesUserPathTypeAndContents(t *testing.T) {
	store := &memoryEnvironment{values: map[string]savedValue{"Path": {`%TOOLS%\bin;E:\javascript\bin`, 2, true}}}
	if err := persistJava(store, `D:\jdk-new`); err != nil {
		t.Fatal(err)
	}
	value := store.values["Path"]
	if value.kind != 2 || !strings.Contains(value.value, `%TOOLS%\bin;E:\javascript\bin`) {
		t.Fatalf("lost existing environment: %+v", value)
	}
	if store.values["JAVA_HOME"].kind != 1 {
		t.Fatal("JAVA_HOME should be a literal string")
	}
}

func TestClearEnvironmentOnlyRemovesSelectedHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "jdk")
	store := &memoryEnvironment{values: map[string]savedValue{
		"JAVA_HOME": {home, 1, true},
		"Path":      {filepath.Join(home, "bin") + `;C:\tools`, 2, true},
	}}
	if err := clearJava(store, home+"-other"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.values["JAVA_HOME"]; !ok {
		t.Fatal("cleared unrelated Java home")
	}
	if err := clearJava(store, home); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.values["JAVA_HOME"]; ok {
		t.Fatal("selected Java home remains")
	}
	if got := store.values["Path"].value; got != `C:\tools` {
		t.Fatalf("unexpected remaining PATH: %q", got)
	}
}

func TestPowerShellIntegrationUsesLiteralExecutable(t *testing.T) {
	script := PowerShellIntegration(`D:\owner's $tools\jvm.exe`)
	if !strings.Contains(script, `& 'D:\owner''s $tools\jvm.exe' @jvmArguments`) {
		t.Fatal("executable was not safely quoted")
	}
	if !strings.Contains(script, "$jvmStatus -eq 0") {
		t.Fatal("activation must depend on command success")
	}
}

func TestPersistenceRejectsUnrepresentablePathsBeforeWriting(t *testing.T) {
	for _, home := range []string{`D:\jdk;other`, `D:\%JDK_HOME%`} {
		original := savedValue{`D:\old`, 1, true}
		store := &memoryEnvironment{values: map[string]savedValue{"JAVA_HOME": original, "Path": {`%TOOLS%\bin`, 2, true}}}
		if err := persistJava(store, home); err == nil {
			t.Fatalf("accepted ambiguous path %q", home)
		}
		if store.values["JAVA_HOME"] != original || store.values["Path"].value != `%TOOLS%\bin` {
			t.Fatal("rejected path modified environment")
		}
	}
	if _, err := ActivationScript("powershell", `D:\jdk;other`, "", ""); err == nil {
		t.Fatal("accepted Windows PATH separator in Java home")
	}
	if _, err := ActivationScript("bash", "/opt/jdk:other", "", ""); err == nil {
		t.Fatal("accepted Unix PATH separator in Java home")
	}
}

func TestPowerShellIntegrationAppliesInCallingSession(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell integration")
	}
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	dir := filepath.Join(t.TempDir(), "owner's $tools")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "fake-jvm.ps1")
	fixture := "if ($args[0] -eq 'use') { exit 0 }\nif ($args[0] -eq 'env') { Write-Output \"`$env:JAVA_HOME = 'fixture-jdk'\"; exit 0 }\nexit 1\n"
	if err := os.WriteFile(program, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	script := PowerShellIntegration(program) + "\njvm use 17\nif ($env:JAVA_HOME -ne 'fixture-jdk') { throw 'activation did not reach calling session' }\nWrite-Output 'activation-ok'\n"
	script += "$env:JAVA_HOME = 'keep-temporary'\njvm use 17 --temp=true --shell powershell\nif ($env:JAVA_HOME -ne 'keep-temporary') { throw 'temporary output activated the saved default' }\n"
	// 只启动无 profile 的隔离子进程；fixture 不访问注册表。
	output, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil || !strings.Contains(string(output), "activation-ok") {
		t.Fatalf("integration: %v\n%s", err, output)
	}
}
