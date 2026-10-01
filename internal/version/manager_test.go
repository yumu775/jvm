package version

import (
	"jvm/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseIdentitiesPreserveVendorAndRejectAmbiguousUse(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	m, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"adoptium", "zulu"} {
		path := fakeJDK(t, filepath.Join(t.TempDir(), "jdk"))
		if err := m.RegisterRelease("17.0.12+7-"+source, path, "17.0.12+7", source, source); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Resolve("17"); err == nil || !strings.Contains(err.Error(), "multiple distributions") {
		t.Fatalf("ambiguous use silently changed vendor: %v", err)
	}
	id := "17.0.12+7-zulu"
	if actual, err := m.Resolve(id); err != nil || actual != id {
		t.Fatalf("explicit vendor ID failed: %s %v", actual, err)
	}
	record, err := m.GetRecord(id)
	if err != nil || record.Source != "zulu" || record.Version != "17.0.12+7" {
		t.Fatalf("lost metadata: %+v %v", record, err)
	}
	if err := config.Update(func(c *config.Config) error { c.DefaultVersion = id; return nil }); err != nil {
		t.Fatal(err)
	}
	if actual, err := m.Resolve("default"); err != nil || actual != id {
		t.Fatalf("default alias failed: %s %v", actual, err)
	}
}

func fakeJDK(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(p, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	exe := "java"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(filepath.Join(p, "bin", exe), []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestExternalRegistrationAndVersionResolution(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	m, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"17.0.9+9", "17.0.10+7", "21.0.1"} {
		if err = m.Register(v, fakeJDK(t, filepath.Join(t.TempDir(), v)), false); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := m.Resolve("17"); err != nil || got != "17.0.10+7" {
		t.Fatalf("got %s %v", got, err)
	}
	if err = m.SetCurrent("17"); err != nil {
		t.Fatal(err)
	}
	if got, err := m.GetCurrent(); err != nil || got != "17.0.10+7" {
		t.Fatalf("current %s %v", got, err)
	}
	if err = m.Register("17.0.10+7", fakeJDK(t, t.TempDir()), true); err == nil {
		t.Fatal("duplicate ID changed its path")
	}
	if err = m.Unregister("17.0.10+7"); err != nil {
		t.Fatal(err)
	}
	c, _ := config.LoadConfig()
	if c.CurrentVersion != "" {
		t.Fatal("current reference retained")
	}
}
func TestTraversalAndProtectedDirectories(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	m, _ := NewManager()
	for _, v := range []string{"../jdk", "..\\jdk", "C:\\jdk", "/jdk", "", "17/../../other"} {
		if _, err := m.GetVersionPath(v); err == nil {
			t.Fatalf("accepted unsafe version %q", v)
		}
	}
	root, _ := config.GetJVMDir()
	fakeJDK(t, root)
	if err := m.Register("17", root, true); err == nil {
		t.Fatal("accepted managed root")
	}
}
func TestLegacyDirectoryCompatibility(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	fakeJDK(t, filepath.Join(root, "versions", "java-17"))
	m, _ := NewManager()
	if !m.IsInstalled("17") {
		t.Fatal("legacy installation lost")
	}
	r, err := m.GetRecord("17")
	if err != nil || !r.Managed {
		t.Fatalf("legacy record: %+v %v", r, err)
	}
}

func TestRemovalProtectsNestedRegisteredInstallation(t *testing.T) {
	t.Setenv("JVM_HOME", t.TempDir())
	parent := fakeJDK(t, filepath.Join(t.TempDir(), "parent"))
	child := fakeJDK(t, filepath.Join(parent, "external"))
	m, _ := NewManager()
	if err := m.Register("17", parent, true); err != nil {
		t.Fatal(err)
	}
	if err := m.Register("21", child, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRemovalPath(parent); err == nil {
		t.Fatal("external child was not protected")
	}
	if err := m.Unregister("21"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRemovalPath(parent); err == nil {
		t.Fatal("unregistered external child lost protection")
	}
}

func TestUnregisteredExternalInsideRepositoryIsNotAdoptedAsManaged(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	p := fakeJDK(t, filepath.Join(root, "versions", "java-17"))
	m, _ := NewManager()
	if err := m.Register("17", p, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Unregister("17"); err != nil {
		t.Fatal(err)
	}
	items, err := m.ListInstalled()
	if err != nil || len(items) != 0 {
		t.Fatalf("external path rediscovered: %+v %v", items, err)
	}
	if _, err := m.GetRecord("17"); err == nil {
		t.Fatal("external directory adopted as managed")
	}
	if err := m.Register("17", p, false); err != nil {
		t.Fatal(err)
	}
	c, _ := config.LoadConfig()
	if len(c.IgnoredLegacyPaths) != 0 {
		t.Fatal("explicit import did not clear ignored path")
	}
	r, err := m.GetRecord("17")
	if err != nil || r.Managed {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestMissingExternalRecordShadowsLegacyDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JVM_HOME", root)
	fakeJDK(t, filepath.Join(root, "versions", "java-17"))
	missing := filepath.Join(t.TempDir(), "missing")
	if err := config.Update(func(c *config.Config) error {
		c.Installations["17"] = config.Installation{Path: missing, Managed: false}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	m, _ := NewManager()
	r, err := m.GetRecord("17")
	if err != nil || r.Managed || r.Path != missing {
		t.Fatalf("fallback stole external identity: %+v %v", r, err)
	}
	items, err := m.ListInstalled()
	if err != nil || len(items) != 0 {
		t.Fatalf("shadowed directory listed: %+v %v", items, err)
	}
}
