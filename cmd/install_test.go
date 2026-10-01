package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitInstallationRollsBackOnRegistrationFailure(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	root := t.TempDir()
	stage, target := filepath.Join(root, "stage"), filepath.Join(root, "jdk")
	writeInstallMarker(t, stage, "new")
	writeInstallMarker(t, target, "old")
	want := errors.New("config is not writable")
	err := commitInstallation(stage, target, func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("expected registration error, got %v", err)
	}
	assertInstallMarker(t, target, "old")
	assertInstallMarker(t, stage, "new")
}

func TestCommitInstallationKeepsPreviousWhenStageMissing(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	root := t.TempDir()
	target := filepath.Join(root, "jdk")
	writeInstallMarker(t, target, "old")
	called := false
	err := commitInstallation(filepath.Join(root, "missing"), target, func() error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("missing stage must fail before registration: err=%v called=%v", err, called)
	}
	assertInstallMarker(t, target, "old")
}

func TestCommitInstallationReplacesOnlyAfterStaging(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	root := t.TempDir()
	stage, target := filepath.Join(root, "stage"), filepath.Join(root, "jdk")
	writeInstallMarker(t, stage, "new")
	writeInstallMarker(t, target, "old")
	err := commitInstallation(stage, target, func() error { assertInstallMarker(t, target, "new"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	assertInstallMarker(t, target, "new")
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("leftover stage/backup: %v, %v", entries, err)
	}
}

func TestCommitFirstInstallationFailureLeavesNoTarget(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	root := t.TempDir()
	stage, target := filepath.Join(root, "stage"), filepath.Join(root, "jdk")
	writeInstallMarker(t, stage, "new")
	err := commitInstallation(stage, target, func() error { return errors.New("cannot register") })
	if err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("unexpected destination: %v", err)
	}
	assertInstallMarker(t, stage, "new")
}

func TestCommitInstallationDoesNotReplaceFile(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	root := t.TempDir()
	stage, target := filepath.Join(root, "stage"), filepath.Join(root, "jdk")
	writeInstallMarker(t, stage, "new")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := commitInstallation(stage, target, func() error { return nil }); err == nil {
		t.Fatal("must reject file")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "keep" {
		t.Fatal("existing file changed")
	}
}

func writeInstallMarker(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "marker"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertInstallMarker(t *testing.T, dir, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, "marker"))
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, want %q, err=%v", dir, got, want, err)
	}
}
