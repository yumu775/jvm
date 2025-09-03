package version

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jvm/internal/config"
)

// Manager 结构体负责管理 Java 版本
// 它包含了配置信息，用于执行各种版本管理操作
type Manager struct {
	config *config.Config
}

// NewManager 创建一个新的版本管理器实例
// 这是 Go 中常见的构造函数模式
func NewManager() (*Manager, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	return &Manager{
		config: cfg,
	}, nil
}

// JavaVersion 表示一个 Java 版本的信息
type JavaVersion struct {
	Version string // 版本号，如 "17.0.8"
	Path    string // 安装路径
	Current bool   // 是否为当前激活版本
}

// ListInstalled 列出所有已安装的 Java 版本
// 这个方法展示了如何遍历目录和处理文件系统操作
func (m *Manager) ListInstalled() ([]JavaVersion, error) {
	versionsDir, err := config.GetVersionsDir()
	if err != nil {
		return nil, err
	}
	
	// 检查版本目录是否存在
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		// 如果目录不存在，返回空列表
		return []JavaVersion{}, nil
	}
	
	// 读取版本目录中的所有条目
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}
	
	var versions []JavaVersion
	
	// 遍历每个条目
	for _, entry := range entries {
		// 处理目录和符号链接
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 跳过普通文件，但允许目录和符号链接
		if !entry.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		versionName := entry.Name()
		versionPath := filepath.Join(versionsDir, versionName)

		// 验证这是一个有效的 Java 安装
		if !m.isValidJavaInstallation(versionPath) {
			continue
		}

		// 提取版本号
		var version string
		if strings.HasPrefix(versionName, "java-") {
			// 标准格式：java-17, java-11.0.19
			version = strings.TrimPrefix(versionName, "java-")
		} else {
			// 直接使用目录名作为版本号
			version = versionName
		}

		// 检查是否为当前激活版本
		isCurrent := version == m.config.CurrentVersion

		versions = append(versions, JavaVersion{
			Version: version,
			Path:    versionPath,
			Current: isCurrent,
		})
	}
	
	// 按版本号排序
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) < 0
	})
	
	return versions, nil
}

// GetCurrent 获取当前激活的 Java 版本
func (m *Manager) GetCurrent() (string, error) {
	if m.config.CurrentVersion == "" {
		return "", fmt.Errorf("no Java version is currently active")
	}
	
	// 验证当前版本是否仍然存在
	if !m.IsInstalled(m.config.CurrentVersion) {
		return "", fmt.Errorf("current version %s is no longer installed", m.config.CurrentVersion)
	}
	
	return m.config.CurrentVersion, nil
}

// IsInstalled 检查指定版本是否已安装
func (m *Manager) IsInstalled(version string) bool {
	versionsDir, err := config.GetVersionsDir()
	if err != nil {
		return false
	}

	// 尝试标准格式 java-version
	versionPath := filepath.Join(versionsDir, "java-"+version)
	if m.isValidJavaInstallation(versionPath) {
		return true
	}

	// 尝试直接使用版本号作为目录名
	versionPath = filepath.Join(versionsDir, version)
	return m.isValidJavaInstallation(versionPath)
}

// GetVersionPath 获取指定版本的安装路径
func (m *Manager) GetVersionPath(version string) (string, error) {
	versionsDir, err := config.GetVersionsDir()
	if err != nil {
		return "", err
	}

	// 尝试标准格式 java-version
	versionPath := filepath.Join(versionsDir, "java-"+version)
	if m.isValidJavaInstallation(versionPath) {
		return versionPath, nil
	}

	// 尝试直接使用版本号作为目录名
	versionPath = filepath.Join(versionsDir, version)
	if m.isValidJavaInstallation(versionPath) {
		return versionPath, nil
	}

	return "", fmt.Errorf("Java version %s is not installed", version)
}

// SetCurrent 设置当前激活的 Java 版本
func (m *Manager) SetCurrent(version string) error {
	if !m.IsInstalled(version) {
		return fmt.Errorf("Java version %s is not installed", version)
	}
	
	m.config.CurrentVersion = version
	return m.config.SaveConfig()
}

// isValidJavaInstallation 检查指定路径是否包含有效的 Java 安装
// 这个方法展示了如何验证文件和目录的存在
func (m *Manager) isValidJavaInstallation(path string) bool {
	// 检查基本目录结构
	binDir := filepath.Join(path, "bin")
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return false
	}
	
	// 检查 java 可执行文件
	javaExe := "java"
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		javaExe = "java.exe"
	}
	
	javaPath := filepath.Join(binDir, javaExe)
	if _, err := os.Stat(javaPath); os.IsNotExist(err) {
		return false
	}
	
	return true
}

// compareVersions 比较两个版本号
// 返回值：< 0 表示 v1 < v2，0 表示相等，> 0 表示 v1 > v2
func compareVersions(v1, v2 string) int {
	// 简化的版本比较逻辑
	// 在实际项目中，你可能需要更复杂的版本比较算法
	
	// 分割版本号
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	
	// 比较每个部分
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	
	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		} else {
			p1 = "0"
		}
		if i < len(parts2) {
			p2 = parts2[i]
		} else {
			p2 = "0"
		}
		
		if p1 < p2 {
			return -1
		} else if p1 > p2 {
			return 1
		}
	}
	
	return 0
}
