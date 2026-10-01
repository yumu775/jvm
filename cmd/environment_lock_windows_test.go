//go:build windows

package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestEnvironmentMutexHelper(t *testing.T) {
	name := os.Getenv("JVM_MUTEX_TEST_NAME")
	if name == "" {
		return
	}
	mode := os.Getenv("JVM_MUTEX_TEST_MODE")
	if mode == "name" {
		actual, err := environmentCommandMutexName()
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		fmt.Println(actual)
		os.Exit(0)
	}
	release, err := acquireNamedEnvironmentLock(name, 200*time.Millisecond)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println("locked")
	if mode == "hold" {
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	if err := release(); err != nil {
		fmt.Println(err)
		os.Exit(3)
	}
	os.Exit(0)
}

func mutexChild(t *testing.T, name, mode, home string) *exec.Cmd {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(program, "-test.run=^TestEnvironmentMutexHelper$")
	command.Env = append(os.Environ(), "JVM_MUTEX_TEST_NAME="+name, "JVM_MUTEX_TEST_MODE="+mode, "JVM_HOME="+home)
	return command
}

func uniqueMutexName(t *testing.T) string {
	return fmt.Sprintf(`Local\JVM.Test.Environment.%d.%d`, os.Getpid(), time.Now().UnixNano())
}

func TestEnvironmentMutexSerializesDifferentHomesAndReleases(t *testing.T) {
	name := uniqueMutexName(t)
	first := mutexChild(t, name, "hold", filepath.Join(t.TempDir(), "home-A"))
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	first.Stderr = &diagnostics
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Process.Kill() })
	reader := bufio.NewScanner(stdout)
	if !reader.Scan() || reader.Text() != "locked" {
		t.Fatalf("holder failed: %s %s", reader.Text(), diagnostics.String())
	}
	second := mutexChild(t, name, "try", filepath.Join(t.TempDir(), "home-B"))
	output, err := second.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "timed out") {
		t.Fatalf("concurrent command was not blocked: %v %s", err, output)
	}
	fmt.Fprintln(stdin)
	stdin.Close()
	if err := first.Wait(); err != nil {
		t.Fatal(err)
	}
	output, err = mutexChild(t, name, "try", t.TempDir()).CombinedOutput()
	if err != nil || !strings.Contains(string(output), "locked") {
		t.Fatalf("lock not released: %v %s", err, output)
	}
}

func TestEnvironmentMutexIdentityIgnoresJVMHome(t *testing.T) {
	name := uniqueMutexName(t)
	a, err := mutexChild(t, name, "name", t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	b, err := mutexChild(t, name, "name", t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || !strings.Contains(string(a), `Global\JVM.Environment.S-`) {
		t.Fatalf("mutex is not user scoped: %q %q", a, b)
	}
}

func TestEnvironmentWrapperReleasesMutexWhenCommandFails(t *testing.T) {
	name := uniqueMutexName(t)
	failure := errors.New("fixture mutation failure")
	command := &cobra.Command{Use: "fixture", RunE: func(*cobra.Command, []string) error { return failure }}
	wrapEnvironmentCommand(command, func() (func() error, error) { return acquireNamedEnvironmentLock(name, time.Second) })
	if err := command.RunE(command, nil); !errors.Is(err, failure) {
		t.Fatalf("lost command failure: %v", err)
	}
	output, err := mutexChild(t, name, "try", t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatalf("failed command leaked mutex: %v %s", err, output)
	}
}
