package env

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileUTF8BOMMarkerIsReplacedAndRemoved(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "profile.ps1")
	marker := "JVM Shell Integration"
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("# "+marker+" - START\nold\n# "+marker+" - END\n$personal = '中文'\n")...)
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProfileBlock(profile, marker); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfileBlock(profile, marker, "new"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(profile)
	if !bytes.HasPrefix(got, []byte{0xef, 0xbb, 0xbf}) || strings.Count(string(got), " - START") != 1 || strings.Contains(string(got), "\nold\n") {
		t.Fatalf("bad BOM marker replacement: %q", got)
	}
	if err := WriteProfileBlock(profile, marker, ""); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(profile)
	if !bytes.HasPrefix(got, []byte{0xef, 0xbb, 0xbf}) || strings.Contains(string(got), marker) || !strings.Contains(string(got), "$personal = '中文'") {
		t.Fatalf("bad removal: %q", got)
	}
}

func TestProfileUnsupportedEncodingIsUnchanged(t *testing.T) {
	for _, data := range [][]byte{{0xff, 0xfe, 'a', 0}, {0xfe, 0xff, 0, 'a'}, {'a', 0, 'b', 0}, {0x80, 'x'}} {
		path := filepath.Join(t.TempDir(), "profile.ps1")
		os.WriteFile(path, data, 0600)
		if ValidateProfileBlock(path, "JVM Shell Integration") == nil {
			t.Fatal("invalid encoding passed preflight")
		}
		if WriteProfileBlock(path, "JVM Shell Integration", "new") == nil {
			t.Fatal("invalid encoding overwritten")
		}
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, data) {
			t.Fatal("invalid encoding content changed")
		}
	}
}

func TestProfileRemovalDoesNotCreateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "profile.ps1")
	if err := WriteProfileBlock(path, "JVM Shell Integration", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("removal created profile")
	}
}

func TestWindowsUnicodeProfileUsesUTF8BOM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell 5 encoding")
	}
	path := filepath.Join(t.TempDir(), "profile.ps1")
	if err := WriteProfileBlock(path, "JVM Shell Integration", "& 'E:/工具/jvm.exe' init powershell"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if !bytes.HasPrefix(content, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("missing UTF-8 BOM")
	}
}
