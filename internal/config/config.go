package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config 结构体定义了应用程序的配置
// 在 Go 中，结构体字段的首字母大写表示它们是导出的（public）
type Installation struct {
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
	Path    string `json:"path"`
	Managed bool   `json:"managed"`
}

type Config struct {
	Installations map[string]Installation `json:"installations,omitempty"`
	// IgnoredLegacyPaths 防止解除登记的外部安装被再次推定为受管目录。
	IgnoredLegacyPaths []string `json:"ignored_legacy_paths,omitempty"`
	// CurrentVersion 存储当前激活的 Java 版本
	CurrentVersion string `json:"current_version"`
	// DefaultVersion 存储默认的 Java 版本
	DefaultVersion string `json:"default_version"`
	// InstallDir 存储 Java 版本的安装目录
	InstallDir string `json:"install_dir"`
	// DownloadDir 存储下载文件的临时目录
	DownloadDir string `json:"download_dir"`
	// AutoScan 是否自动扫描系统中已安装的 Java 版本
	AutoScan bool `json:"auto_scan"`
	// CustomScanPaths 用户自定义的扫描路径
	CustomScanPaths []string `json:"custom_scan_paths"`
	// DownloadSources 存储可用的下载源
	DownloadSources []DownloadSource `json:"download_sources"`
}

// DownloadSource 定义了 Java 下载源的信息
type DownloadSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// GetJVMDir 返回 JVM 工具的主目录
// 这个函数演示了 Go 中的错误处理模式
func GetJVMDir() (string, error) {
	if root := os.Getenv("JVM_HOME"); root != "" {
		if !filepath.IsAbs(root) {
			return "", fmt.Errorf("JVM_HOME must be an absolute path")
		}
		return AbsolutePath(root)
	}
	// 获取用户主目录
	// os.UserHomeDir() 返回两个值：目录路径和可能的错误
	// 这是 Go 中常见的错误处理模式
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// filepath.Join 用于跨平台的路径拼接
	// 它会根据操作系统使用正确的路径分隔符
	jvmDir := filepath.Join(homeDir, ".jvm")
	return jvmDir, nil
}

// GetConfigPath 返回配置文件的完整路径
func GetConfigPath() (string, error) {
	jvmDir, err := GetJVMDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(jvmDir, "config.json"), nil
}

// GetVersionsDir 返回存储 Java 版本的目录
func GetVersionsDir() (string, error) {
	c, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return c.InstallDir, nil
}

func GetDownloadsDir() (string, error) {
	c, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return c.DownloadDir, nil
}

// AbsolutePath 拒绝空路径，统一持久化绝对路径。
func AbsolutePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	return filepath.Abs(filepath.Clean(path))
}

// LoadConfig 从文件加载配置
// 如果配置文件不存在，返回默认配置
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 如果不存在，返回默认配置
		return getDefaultConfig()
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 配置
	cfg, err := getDefaultConfig()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig 将配置保存到文件
func (c *Config) SaveConfig() error {
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	return c.save()
}

// Update 在同一锁内读取和写入配置，保留其他命令的更新。
func Update(change func(*Config) error) error {
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	c, err := LoadConfig()
	if err != nil {
		return err
	}
	if err := change(c); err != nil {
		return err
	}
	return c.save()
}

func acquireLock() (func(), error) {
	root, err := GetJVMDir()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(root, "config.lock")
	deadline := time.Now().Add(3 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("configuration is locked: %s (if no jvm process is running, remove the stale lock)", path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (c *Config) save() error {
	if err := c.normalize(); err != nil {
		return err
	}
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// 确保配置目录存在
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// 将配置序列化为 JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	tmp, err := os.CreateTemp(configDir, ".config-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, configPath)
}

// getDefaultConfig 返回默认配置
func getDefaultConfig() (*Config, error) {
	jvmDir, err := GetJVMDir()
	if err != nil {
		return nil, err
	}

	return &Config{
		Installations:   map[string]Installation{},
		CurrentVersion:  "",
		DefaultVersion:  "",
		InstallDir:      filepath.Join(jvmDir, "versions"),
		DownloadDir:     filepath.Join(jvmDir, "downloads"),
		AutoScan:        true,
		CustomScanPaths: []string{},
		DownloadSources: getDefaultDownloadSources(),
	}, nil
}

// getDefaultDownloadSources 返回默认的下载源配置
func getDefaultDownloadSources() []DownloadSource {
	return []DownloadSource{
		{
			Name: "Adoptium",
			URL:  "https://api.adoptium.net/v3/binary/latest/{version}/ga/{os}/{arch}/jdk/hotspot/normal/eclipse",
		},
		{
			Name: "Amazon Corretto",
			URL:  "https://corretto.aws/downloads/latest/amazon-corretto-{version}-{arch}-{os}-jdk.{ext}",
		},
	}
}

// GetOSArch 返回当前操作系统和架构信息
// 这个函数展示了如何处理跨平台的差异
func GetOSArch() (string, string) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// 将 Go 的架构名称映射到 Java 下载源使用的名称
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "x32"
	case "arm64":
		arch = "aarch64"
	}

	// 将 Go 的操作系统名称映射到 Java 下载源使用的名称
	switch osName {
	case "darwin":
		osName = "mac"
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	}

	return osName, arch
}

func (c *Config) normalize() error {
	defaults, err := getDefaultConfig()
	if err != nil {
		return err
	}
	if c.InstallDir == "" {
		c.InstallDir = defaults.InstallDir
	}
	if c.DownloadDir == "" {
		c.DownloadDir = defaults.DownloadDir
	}
	root, err := GetJVMDir()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(c.InstallDir) {
		c.InstallDir = filepath.Join(root, c.InstallDir)
	}
	if !filepath.IsAbs(c.DownloadDir) {
		c.DownloadDir = filepath.Join(root, c.DownloadDir)
	}
	if c.InstallDir, err = AbsolutePath(c.InstallDir); err != nil {
		return err
	}
	if c.DownloadDir, err = AbsolutePath(c.DownloadDir); err != nil {
		return err
	}
	if c.Installations == nil {
		c.Installations = make(map[string]Installation)
	}
	return nil
}
