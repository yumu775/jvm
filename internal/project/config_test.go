package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNearestConfigAndMalformedChildDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "app")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".jvmrc"), []byte("17\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	got, err := m.findConfigInPath(child)
	if err != nil || got.JavaVersion != "17" {
		t.Fatalf("find parent: %+v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(child, ".jvmrc"), []byte("../../outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.findConfigInPath(child); err == nil {
		t.Fatal("invalid child must not silently use parent's Java")
	}
}

func TestProjectConfigFormats(t *testing.T) {
	m := NewManager()
	for _, tc := range []struct{ name, text, want string }{
		{".jvmrc", "# comment\n17.0.12+7\n", "17.0.12+7"},
		{".java-version", "21\r\n", "21"},
		{".sdkmanrc", "maven=3.9.9\njava=21.0.4-tem\n", "21.0.4-tem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := m.readConfigFile(path)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestProjectCreateRejectsInjectedVersion(t *testing.T) {
	if err := NewManager().CreateProjectConfig("17\n21"); err == nil {
		t.Fatal("accepted multi-line version")
	}
}
