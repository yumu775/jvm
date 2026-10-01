package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArchiveEscape(t *testing.T) {
	i := NewInstaller()
	root := t.TempDir()
	for _, name := range []string{"../escaped", "sub/../../escaped", "C:/escaped"} {
		if err := i.extractTarFile(tar.NewReader(bytes.NewReader(nil)), &tar.Header{Name: name, Typeflag: tar.TypeReg}, root); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	if err := i.extractTarFile(tar.NewReader(bytes.NewReader(nil)), &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside"}, root); err == nil {
		t.Fatal("external symlink accepted")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("../escape")
	f.Write([]byte("bad"))
	zw.Close()
	z := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(z, buf.Bytes(), 0600)
	if err := i.InstallJava(z, root, "17"); err == nil {
		t.Fatal("zip escape accepted")
	}
}

func TestArchiveDoesNotFollowSymlink(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	if err := os.Symlink(out, filepath.Join(root, "link")); err != nil {
		t.Skip("symbolic links unavailable")
	}
	if _, err := safeArchivePath(root, "link/file"); err == nil {
		t.Fatal("followed link")
	}
}

func TestMacLayoutAndRelease(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "jdk", "Contents", "Home")
	os.MkdirAll(filepath.Join(home, "bin"), 0755)
	os.MkdirAll(filepath.Join(home, "lib"), 0755)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"java", "javac"} {
		os.WriteFile(filepath.Join(home, "bin", name+suffix), []byte("fixture"), 0755)
	}
	os.WriteFile(filepath.Join(home, "release"), []byte("JAVA_VERSION=\"17.0.10\"\n"), 0600)
	i := NewInstaller()
	if err := i.NormalizeInstallation(root); err != nil {
		t.Fatal(err)
	}
	if err := i.ValidateRelease(root, "17.0.10+7"); err != nil {
		t.Fatal(err)
	}
	if i.ValidateRelease(root, "21.0.1+1") == nil {
		t.Fatal("wrong major accepted")
	}
}

func makeLinkTestJDK(t *testing.T, path string) {
	t.Helper()
	for _, dir := range []string{"bin", "lib"} {
		if err := os.MkdirAll(filepath.Join(path, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"java", "javac"} {
		if err := os.WriteFile(filepath.Join(path, "bin", name+suffix), []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "release"), []byte("JAVA_VERSION=\"17.0.10\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizationRejectsLinkThatEscapesAfterFlattening(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "jdk")
	makeLinkTestJDK(t, nested)
	if err := os.Symlink(filepath.Join("..", "..", "outside"), filepath.Join(nested, "lib", "link")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := validateInstallationLinks(root); err != nil {
		t.Fatalf("original link should remain within extraction root: %v", err)
	}
	if err := NewInstaller().NormalizeInstallation(root); err == nil {
		t.Fatal("flattening allowed dangling link to escape")
	}
	if err := NewInstaller().ValidateRelease(root, "17.0.10"); err == nil {
		t.Fatal("validation accepted escaped link")
	}
}

func TestNormalizationPreservesValidInternalAndDanglingLinks(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "jdk")
	makeLinkTestJDK(t, nested)
	for name, target := range map[string]string{"release-link": filepath.Join("..", "release"), "optional-link": "missing-optional-file"} {
		if err := os.Symlink(target, filepath.Join(nested, "lib", name)); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
	}
	if err := NewInstaller().NormalizeInstallation(root); err != nil {
		t.Fatal(err)
	}
	if err := NewInstaller().ValidateRelease(root, "17.0.10"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "lib", "release-link"))
	if err != nil || !bytes.Contains(got, []byte("17.0.10")) {
		t.Fatalf("internal link damaged: %q %v", got, err)
	}
}

func TestLinkResolutionPreservesDotDotAfterSymlinkExpansion(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", filepath.Join(root, "redirect")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "redirect")+string(filepath.Separator)+".."+string(filepath.Separator)+"outside", filepath.Join(root, "nested", "link")); err != nil {
		t.Fatal(err)
	}
	if err := validateInstallationLinks(root); err == nil {
		t.Fatal("cleaning before link expansion hid escape")
	}
}

func TestDanglingRelativeTargetBoundaryWithoutSymlinkPrivileges(t *testing.T) {
	root := t.TempDir()
	if err := resolveInstallationLink(root, "missing/../../outside"); err == nil {
		t.Fatal("dangling target escaped")
	}
	if err := resolveInstallationLink(root, "missing/../inside"); err != nil {
		t.Fatal(err)
	}
}
