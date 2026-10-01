package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetupPowerShellProfileUsesInstalledExecutableAndPreservesUserContent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.exe")
	if err := os.WriteFile(source, []byte("tool"), 0700); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "主人's $tools")
	profile := filepath.Join(root, "profile.ps1")
	personal := append([]byte{0xef, 0xbb, 0xbf}, []byte("$personal = '保留'\r\n")...)
	if err := os.WriteFile(profile, personal, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	configure := func(dir string, remove bool) error {
		calls++
		if dir != destination || remove {
			t.Fatal("unexpected PATH action")
		}
		return nil
	}
	options := setupOptions{customPath: destination, profile: profile, profileSet: true}
	if _, err := performSetup(options, source, "windows", configure); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(destination, "jvm.exe")
	content, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	wanted := "& '" + strings.ReplaceAll(target, "'", "''") + "' init powershell | Out-String | Invoke-Expression"
	if !bytes.HasPrefix(content, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(content, []byte("$personal = '保留'")) || !strings.Contains(string(content), wanted) {
		t.Fatalf("wrong integration: %s", content)
	}
	if _, err := performSetup(options, target, "windows", configure); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(profile)
	if strings.Count(string(content), "# JVM Shell Integration - START") != 1 {
		t.Fatal("duplicate shell integration")
	}
	options.uninstall = true
	if _, err := performSetup(options, target, "windows", func(dir string, remove bool) error {
		if !remove {
			t.Fatal("uninstall did not remove PATH")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(profile)
	if strings.Contains(string(content), shellIntegrationMarker) || !bytes.HasPrefix(content, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(content, []byte("$personal = '保留'")) {
		t.Fatalf("uninstall damaged profile: %s", content)
	}
	if calls != 2 {
		t.Fatal("unexpected PATH call count")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("setup uninstall deleted executable")
	}
}

func TestSetupRejectsProfileErrorsBeforeCopyOrPATH(t *testing.T) {
	for _, name := range []string{"relative", "empty", "platform", "utf16", "marker", "combination"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.exe")
			os.WriteFile(source, []byte("tool"), 0700)
			destination := filepath.Join(root, "destination")
			profile := filepath.Join(root, "profile.ps1")
			options := setupOptions{customPath: destination, profile: profile, profileSet: true}
			platform := "windows"
			switch name {
			case "relative":
				options.profile = "relative.ps1"
			case "empty":
				options.profile = ""
			case "platform":
				platform = "linux"
			case "utf16":
				os.WriteFile(profile, []byte{0xff, 0xfe, 'a', 0}, 0600)
			case "marker":
				os.WriteFile(profile, []byte("# JVM Shell Integration - START\nunfinished"), 0600)
			case "combination":
				options.force = true
				options.uninstall = true
			}
			if _, err := performSetup(options, source, platform, func(string, bool) error { t.Fatal("invalid setup changed PATH"); return nil }); err == nil {
				t.Fatal("invalid setup accepted")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("invalid setup copied executable")
			}
		})
	}
}

func TestSetupDoesNotWriteProfileWhenPATHFails(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "jvm.exe")
	profile := filepath.Join(root, "profile.ps1")
	if err := os.WriteFile(profile, []byte("personal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options := setupOptions{profile: profile, profileSet: true}
	if _, err := performSetup(options, source, "windows", func(string, bool) error { return errors.New("denied") }); err == nil {
		t.Fatal("PATH failure ignored")
	}
	content, _ := os.ReadFile(profile)
	if string(content) != "personal\n" {
		t.Fatal("profile written before successful PATH configuration")
	}
}

func TestCopyExecutableProtectsExistingFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	os.WriteFile(source, []byte("new"), 0700)
	os.WriteFile(target, []byte("old"), 0700)
	if err := copyExecutable(source, target, false); err == nil {
		t.Fatal("overwrote without force")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "old" {
		t.Fatal("existing file damaged")
	}
	if err := copyExecutable(source, target, true); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(target)
	if string(content) != "new" {
		t.Fatal("replacement failed")
	}
	if err := copyExecutable(source, root, true); err == nil {
		t.Fatal("directory replacement accepted")
	}
}

func TestCopyExecutableBusyWindowsDestinationIsPreserved(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable replacement semantics")
	}
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	os.WriteFile(source, []byte("new"), 0700)
	os.WriteFile(target, []byte("old"), 0700)
	handle, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err = copyExecutable(source, target, true); err == nil {
		t.Fatal("expected sharing violation on open destination")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "old" {
		t.Fatal("busy destination was damaged")
	}
}
