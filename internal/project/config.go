package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// Manager 负责管理项目级配置
type Manager struct{}

// NewManager 创建一个新的项目配置管理器实例
func NewManager() *Manager {
	return &Manager{}
}

// ProjectConfig 表示项目配置信息
type ProjectConfig struct {
	JavaVersion string // 项目要求的 Java 版本
	ConfigPath  string // 配置文件路径
	ProjectDir  string // 项目根目录
}

// FindProjectConfig 在当前目录及其父目录中查找 .jvmrc 文件
func (m *Manager) FindProjectConfig() (*ProjectConfig, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	
	return m.findConfigInPath(currentDir)
}

// findConfigInPath 在指定路径及其父目录中查找配置文件
func (m *Manager) findConfigInPath(startPath string) (*ProjectConfig, error) {
	configFiles := []string{".jvmrc", ".java-version", ".sdkmanrc"}
	
	currentPath := startPath
	for {
		// 检查每个可能的配置文件
		for _, configFile := range configFiles {
			configPath := filepath.Join(currentPath, configFile)
			if _, err := os.Stat(configPath); err == nil {
				// 找到配置文件，读取内容
				version, err := m.readConfigFile(configPath)
				if err != nil {
					continue // 尝试下一个文件
				}
				
				return &ProjectConfig{
					JavaVersion: version,
					ConfigPath:  configPath,
					ProjectDir:  currentPath,
				}, nil
			}
		}
		
		// 移动到父目录
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// 已经到达根目录
			break
		}
		currentPath = parentPath
	}
	
	return nil, fmt.Errorf("no project configuration found")
}

// readConfigFile 读取配置文件内容
func (m *Manager) readConfigFile(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	
	// 解析配置文件内容
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// 处理不同格式的配置文件
		if strings.Contains(line, "=") {
			// 处理 key=value 格式（如 .sdkmanrc）
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "java" {
				return strings.TrimSpace(parts[1]), nil
			}
		} else {
			// 处理纯版本号格式（如 .jvmrc, .java-version）
			return line, nil
		}
	}
	
	return "", fmt.Errorf("no valid Java version found in config file")
}

// CreateProjectConfig 在当前目录创建 .jvmrc 文件
func (m *Manager) CreateProjectConfig(javaVersion string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	configPath := filepath.Join(currentDir, ".jvmrc")
	
	// 检查文件是否已存在
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("project configuration already exists: %s", configPath)
	}
	
	// 创建配置文件内容
	content := fmt.Sprintf("# JVM project configuration\n# Java version for this project\n%s\n", javaVersion)
	
	// 写入文件
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create project config: %w", err)
	}
	
	color.Green("Created project configuration: %s", configPath)
	return nil
}

// UpdateProjectConfig 更新项目配置文件
func (m *Manager) UpdateProjectConfig(javaVersion string) error {
	config, err := m.FindProjectConfig()
	if err != nil {
		return fmt.Errorf("no project configuration found: %w", err)
	}
	
	// 读取现有文件内容
	content, err := os.ReadFile(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	updated := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// 跳过空行和注释，保持原样
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		
		// 更新版本行
		if !updated {
			if strings.Contains(line, "=") {
				// 处理 key=value 格式
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == "java" {
					newLines = append(newLines, fmt.Sprintf("java=%s", javaVersion))
					updated = true
					continue
				}
			} else {
				// 处理纯版本号格式
				newLines = append(newLines, javaVersion)
				updated = true
				continue
			}
		}
		
		newLines = append(newLines, line)
	}
	
	// 如果没有找到版本行，添加一个
	if !updated {
		newLines = append(newLines, javaVersion)
	}
	
	// 写回文件
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(config.ConfigPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to update config file: %w", err)
	}
	
	color.Green("Updated project configuration: %s", config.ConfigPath)
	return nil
}

// AutoSwitchToProjectVersion 自动切换到项目配置的 Java 版本
func (m *Manager) AutoSwitchToProjectVersion() (*ProjectConfig, error) {
	config, err := m.FindProjectConfig()
	if err != nil {
		return nil, err
	}
	
	color.Blue("Found project configuration: %s", config.ConfigPath)
	color.Blue("Required Java version: %s", config.JavaVersion)
	
	return config, nil
}

// ValidateProjectConfig 验证项目配置是否有效
func (m *Manager) ValidateProjectConfig(config *ProjectConfig) error {
	if config == nil {
		return fmt.Errorf("project config is nil")
	}
	
	if config.JavaVersion == "" {
		return fmt.Errorf("Java version is empty")
	}
	
	// 检查配置文件是否仍然存在
	if _, err := os.Stat(config.ConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("config file no longer exists: %s", config.ConfigPath)
	}
	
	return nil
}

// GetProjectInfo 获取项目信息摘要
func (m *Manager) GetProjectInfo() (map[string]string, error) {
	info := make(map[string]string)
	
	// 获取当前目录
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	info["current_directory"] = currentDir
	
	// 查找项目配置
	config, err := m.FindProjectConfig()
	if err != nil {
		info["project_config"] = "Not found"
		info["java_version"] = "Not specified"
	} else {
		info["project_config"] = config.ConfigPath
		info["java_version"] = config.JavaVersion
		info["project_directory"] = config.ProjectDir
		
		// 计算相对路径
		if relPath, err := filepath.Rel(currentDir, config.ProjectDir); err == nil {
			if relPath == "." {
				info["config_location"] = "Current directory"
			} else {
				info["config_location"] = fmt.Sprintf("Parent directory (%s)", relPath)
			}
		}
	}
	
	return info, nil
}
