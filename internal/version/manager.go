package version

import (
	"fmt"
	"jvm/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type Manager struct{ config *config.Config }
type JavaVersion struct {
	Version string
	Path    string
	Current bool
}

func NewManager() (*Manager, error) {
	c, e := config.LoadConfig()
	if e != nil {
		return nil, e
	}
	return &Manager{config: c}, nil
}

// ValidateVersion 防止版本标识被解释为文件路径。
func ValidateVersion(v string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]*$`).MatchString(v) || strings.Contains(v, "..") {
		return fmt.Errorf("invalid version: %q", v)
	}
	return nil
}
func (m *Manager) ListInstalled() ([]JavaVersion, error) {
	c, e := config.LoadConfig()
	if e != nil {
		return nil, e
	}
	m.config = c
	found := map[string]string{}
	dir, e := config.GetVersionsDir()
	if e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(dir)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	for _, entry := range entries {
		v := strings.TrimPrefix(entry.Name(), "java-")
		p := filepath.Join(dir, entry.Name())
		if _, registered := c.Installations[v]; registered || ignoredLegacyPath(c, p) {
			continue
		}
		if ValidateVersion(v) == nil && m.isValidJavaInstallation(p) {
			found[v] = p
		}
	}
	for v, r := range c.Installations {
		if ValidateVersion(v) == nil && m.isValidJavaInstallation(r.Path) {
			found[v] = r.Path
		}
	}
	result := []JavaVersion{}
	for v, p := range found {
		result = append(result, JavaVersion{v, p, v == c.CurrentVersion})
	}
	sort.Slice(result, func(i, j int) bool { return compareVersions(result[i].Version, result[j].Version) < 0 })
	return result, nil
}
func (m *Manager) GetRecord(v string) (config.Installation, error) {
	resolved, e := m.Resolve(v)
	if e != nil {
		return config.Installation{}, e
	}
	v = resolved
	if e := ValidateVersion(v); e != nil {
		return config.Installation{}, e
	}
	c, e := config.LoadConfig()
	if e != nil {
		return config.Installation{}, e
	}
	m.config = c
	if r, ok := c.Installations[v]; ok {
		if !filepath.IsAbs(r.Path) {
			return r, fmt.Errorf("registered path must be absolute")
		}
		return r, nil
	}
	dir, e := config.GetVersionsDir()
	if e != nil {
		return config.Installation{}, e
	}
	for _, name := range []string{"java-" + v, v} {
		p := filepath.Join(dir, name)
		if ignoredLegacyPath(c, p) {
			continue
		}
		if m.isValidJavaInstallation(p) {
			info, e := os.Lstat(p)
			if e != nil {
				return config.Installation{}, e
			}
			return config.Installation{Path: p, Managed: info.Mode()&os.ModeSymlink == 0}, nil
		}
	}
	return config.Installation{}, fmt.Errorf("Java version %s is not installed", v)
}
func (m *Manager) GetVersionPath(v string) (string, error) {
	r, e := m.GetRecord(v)
	if e != nil {
		return "", e
	}
	if !m.isValidJavaInstallation(r.Path) {
		return "", fmt.Errorf("invalid Java installation: %s", r.Path)
	}
	return r.Path, nil
}
func (m *Manager) IsInstalled(v string) bool { _, e := m.GetVersionPath(v); return e == nil }
func (m *Manager) GetCurrent() (string, error) {
	c, e := config.LoadConfig()
	if e != nil {
		return "", e
	}
	if c.CurrentVersion == "" || !m.IsInstalled(c.CurrentVersion) {
		return "", fmt.Errorf("no installed Java version is currently active")
	}
	return c.CurrentVersion, nil
}
func (m *Manager) SetCurrent(v string) error {
	resolved, e := m.Resolve(v)
	if e != nil {
		return e
	}
	v = resolved
	if !m.IsInstalled(v) {
		return fmt.Errorf("Java version %s is not installed", v)
	}
	return config.Update(func(c *config.Config) error { c.CurrentVersion = v; return nil })
}

// Register 登记真实安装位置，外部导入不取得文件删除权。
func (m *Manager) Register(v, path string, managed bool) error {
	return m.register(v, path, managed, nil)
}

// RegisterRelease 保存厂商与真实版本，避免不同发行版同版本互相覆盖。
func (m *Manager) RegisterRelease(id, path, actual, source, vendor string) error {
	return m.register(id, path, true, &config.Installation{Version: actual, Source: source, Vendor: vendor})
}

func (m *Manager) register(v, path string, managed bool, metadata *config.Installation) error {
	if e := ValidateVersion(v); e != nil {
		return e
	}
	p, e := config.AbsolutePath(path)
	if e != nil {
		return e
	}
	if !m.isValidJavaInstallation(p) {
		return fmt.Errorf("invalid Java installation: %s", p)
	}
	if managed {
		if info, err := os.Lstat(p); err != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed installation must be a real directory: %s", p)
		}
		p, e = filepath.EvalSymlinks(p)
		if e != nil {
			return e
		}
		if e := ValidateRemovalPath(p); e != nil {
			return e
		}
	}
	return config.Update(func(c *config.Config) error {
		if c.Installations == nil {
			c.Installations = map[string]config.Installation{}
		}
		if old, ok := c.Installations[v]; ok {
			if !samePath(old.Path, p) {
				return fmt.Errorf("version %s is already registered at %s", v, old.Path)
			}
			managed = old.Managed
		}
		for other, r := range c.Installations {
			if other != v && samePath(r.Path, p) && (managed || r.Managed) {
				return fmt.Errorf("installation path already owned by version %s", other)
			}
		}
		record := c.Installations[v]
		record.Path, record.Managed = p, managed
		if metadata != nil {
			record.Version, record.Source, record.Vendor = metadata.Version, metadata.Source, metadata.Vendor
		}
		c.Installations[v] = record
		kept := c.IgnoredLegacyPaths[:0]
		for _, ignored := range c.IgnoredLegacyPaths {
			if !sameInstallationPath(ignored, p) {
				kept = append(kept, ignored)
			}
		}
		c.IgnoredLegacyPaths = kept
		return nil
	})
}
func (m *Manager) Unregister(v string) error {
	if e := ValidateVersion(v); e != nil {
		return e
	}
	return config.Update(func(c *config.Config) error {
		if record, ok := c.Installations[v]; ok && !record.Managed && !ignoredLegacyPath(c, record.Path) {
			c.IgnoredLegacyPaths = append(c.IgnoredLegacyPaths, record.Path)
		}
		delete(c.Installations, v)
		if c.CurrentVersion == v {
			c.CurrentVersion = ""
		}
		if c.DefaultVersion == v {
			c.DefaultVersion = ""
		}
		return nil
	})
}

func ignoredLegacyPath(c *config.Config, path string) bool {
	for _, ignored := range c.IgnoredLegacyPaths {
		if sameInstallationPath(ignored, path) {
			return true
		}
	}
	return false
}

func sameInstallationPath(a, b string) bool {
	if real, err := filepath.EvalSymlinks(a); err == nil {
		a = real
	}
	if real, err := filepath.EvalSymlinks(b); err == nil {
		b = real
	}
	return samePath(a, b)
}

// ValidateRemovalPath 拒绝根目录、受保护目录和被重定向的路径。
func ValidateRemovalPath(p string) error {
	if !filepath.IsAbs(p) || filepath.Dir(p) == p {
		return fmt.Errorf("unsafe installation path: %s", p)
	}
	for _, get := range []func() (string, error){os.UserHomeDir, config.GetJVMDir, config.GetVersionsDir, config.GetDownloadsDir} {
		root, e := get()
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(p, root)
		if samePath(root, p) || (err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fmt.Errorf("refusing to remove protected directory: %s", p)
		}
	}
	resolved, e := filepath.EvalSymlinks(p)
	if e != nil {
		return e
	}
	if !samePath(resolved, p) {
		return fmt.Errorf("managed installation contains a redirected path: %s", p)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	for _, ignored := range cfg.IgnoredLegacyPaths {
		other := ignored
		if real, err := filepath.EvalSymlinks(other); err == nil {
			other = real
		}
		rel, err := filepath.Rel(p, other)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("installation contains retained external Java at %s", ignored)
		}
	}
	for id, record := range cfg.Installations {
		other := record.Path
		if real, err := filepath.EvalSymlinks(other); err == nil {
			other = real
		}
		rel, err := filepath.Rel(p, other)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if samePath(p, other) && record.Managed {
			continue
		}
		return fmt.Errorf("installation contains protected registered Java %s at %s", id, record.Path)
	}
	return nil
}
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func (m *Manager) isValidJavaInstallation(p string) bool {
	exe := "java"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	info, e := os.Stat(filepath.Join(p, "bin", exe))
	return e == nil && !info.IsDir()
}
func compareVersions(a, b string) int {
	digits := regexp.MustCompile(`[0-9]+`)
	aa := digits.FindAllString(a, -1)
	bb := digits.FindAllString(b, -1)
	for i := 0; i < len(aa) || i < len(bb); i++ {
		var x, y uint64
		if i < len(aa) {
			x, _ = strconv.ParseUint(aa[i], 10, 64)
		}
		if i < len(bb) {
			y, _ = strconv.ParseUint(bb[i], 10, 64)
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return strings.Compare(a, b)
}

// Resolve 优先精确版本，数字简写选择已安装的最高补丁版本。
func (m *Manager) Resolve(v string) (string, error) {
	if err := ValidateVersion(v); err != nil {
		return "", err
	}
	c, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	if v == "default" {
		if c.DefaultVersion == "" || c.DefaultVersion == "default" {
			return "", fmt.Errorf("no default alias configured; use jvm config set default-version <installed-version>")
		}
		return m.Resolve(c.DefaultVersion)
	}
	if _, ok := c.Installations[v]; ok {
		return v, nil
	}
	items, err := m.ListInstalled()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Version == v {
			return v, nil
		}
	}
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]+)*(\+[0-9A-Za-z.-]+)?$`).MatchString(v) {
		return "", fmt.Errorf("Java version %s is not installed", v)
	}
	best := ""
	bestVersion := ""
	matchedSources := map[string]bool{}
	choices := []string{}
	for _, item := range items {
		record := c.Installations[item.Version]
		actual := record.Version
		if actual == "" {
			actual = item.Version
		}
		if actual == v || strings.HasPrefix(actual, v+".") || strings.HasPrefix(actual, v+"+") {
			matchedSources[record.Source] = true
			choices = append(choices, item.Version)
			if best == "" || compareVersions(actual, bestVersion) > 0 {
				best = item.Version
				bestVersion = actual
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("Java version %s is not installed", v)
	}
	if len(matchedSources) > 1 {
		return "", fmt.Errorf("Java %s matches multiple distributions; use an exact ID: %s", v, strings.Join(choices, ", "))
	}
	return best, nil
}
