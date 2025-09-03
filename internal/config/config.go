package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Config 结构体定义了应用程序的配置
// 在 Go 中，结构体字段的首字母大写表示它们是导出的（public）
type Config struct {
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
	jvmDir, err := GetJVMDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(jvmDir, "versions"), nil
}

// GetDownloadsDir 返回存储下载文件的目录
func GetDownloadsDir() (string, error) {
	jvmDir, err := GetJVMDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(jvmDir, "downloads"), nil
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
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// SaveConfig 将配置保存到文件
func (c *Config) SaveConfig() error {
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
	return os.WriteFile(configPath, data, 0644)
}

// getDefaultConfig 返回默认配置
func getDefaultConfig() (*Config, error) {
	jvmDir, err := GetJVMDir()
	if err != nil {
		return nil, err
	}

	return &Config{
		CurrentVersion:   "",
		DefaultVersion:   "",
		InstallDir:       filepath.Join(jvmDir, "versions"),
		DownloadDir:      filepath.Join(jvmDir, "downloads"),
		AutoScan:         true,
		CustomScanPaths:  []string{},
		DownloadSources:  getDefaultDownloadSources(),
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
