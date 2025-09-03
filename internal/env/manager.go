package env

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

// Manager 负责管理环境变量和 shell 集成
type Manager struct{}

// NewManager 创建一个新的环境变量管理器实例
func NewManager() *Manager {
	return &Manager{}
}

// ShellInfo 包含 shell 的信息
type ShellInfo struct {
	Name        string // shell 名称，如 "bash", "zsh", "powershell"
	ConfigFile  string // 配置文件路径
	SetCommand  string // 设置环境变量的命令格式
	ExportCmd   string // 导出命令格式
}

// DetectShell 检测当前使用的 shell
func (m *Manager) DetectShell() (*ShellInfo, error) {
	switch runtime.GOOS {
	case "windows":
		return m.detectWindowsShell()
	case "darwin", "linux":
		return m.detectUnixShell()
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// detectWindowsShell 检测 Windows 上的 shell
func (m *Manager) detectWindowsShell() (*ShellInfo, error) {
	// 检查是否在 PowerShell 中
	if psModulePath := os.Getenv("PSModulePath"); psModulePath != "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		
		// 检查 PowerShell 版本
		profilePath := filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			// 尝试 Windows PowerShell 5.x 路径
			profilePath = filepath.Join(homeDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
		}
		
		return &ShellInfo{
			Name:       "powershell",
			ConfigFile: profilePath,
			SetCommand: "$env:%s = \"%s\"",
			ExportCmd:  "$env:%s = \"%s\"",
		}, nil
	}
	
	// 默认使用 Command Prompt
	return &ShellInfo{
		Name:       "cmd",
		ConfigFile: "", // CMD 没有持久化配置文件
		SetCommand: "set %s=%s",
		ExportCmd:  "set %s=%s",
	}, nil
}

// detectUnixShell 检测 Unix-like 系统上的 shell
func (m *Manager) detectUnixShell() (*ShellInfo, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // 默认使用 bash
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	shellName := filepath.Base(shell)
	var configFile string
	
	switch shellName {
	case "bash":
		// 优先使用 .bashrc，如果不存在则使用 .bash_profile
		configFile = filepath.Join(homeDir, ".bashrc")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			configFile = filepath.Join(homeDir, ".bash_profile")
		}
	case "zsh":
		configFile = filepath.Join(homeDir, ".zshrc")
	case "fish":
		configFile = filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		// 对于未知 shell，尝试使用 .profile
		configFile = filepath.Join(homeDir, ".profile")
	}
	
	return &ShellInfo{
		Name:       shellName,
		ConfigFile: configFile,
		SetCommand: "export %s=\"%s\"",
		ExportCmd:  "export %s=\"%s\"",
	}, nil
}

// SetJavaEnvironment 设置 Java 环境变量
func (m *Manager) SetJavaEnvironment(javaHome string, temporary bool) error {
	if temporary {
		return m.setTemporaryEnvironment(javaHome)
	}
	return m.setPersistentEnvironment(javaHome)
}

// setTemporaryEnvironment 设置临时环境变量（仅当前会话）
func (m *Manager) setTemporaryEnvironment(javaHome string) error {
	// 设置 JAVA_HOME
	if err := os.Setenv("JAVA_HOME", javaHome); err != nil {
		return fmt.Errorf("failed to set JAVA_HOME: %w", err)
	}
	
	// 更新 PATH
	javaBin := filepath.Join(javaHome, "bin")
	currentPath := os.Getenv("PATH")
	
	// 移除现有的 Java 路径
	newPath := m.removeJavaFromPath(currentPath)
	
	// 添加新的 Java 路径到开头
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	
	newPath = javaBin + separator + newPath
	
	if err := os.Setenv("PATH", newPath); err != nil {
		return fmt.Errorf("failed to set PATH: %w", err)
	}
	
	color.Green("✓ Temporary environment variables set for current session")
	color.Blue("  JAVA_HOME = %s", javaHome)
	color.Blue("  PATH updated to include %s", javaBin)
	
	return nil
}

// setPersistentEnvironment 设置持久化环境变量
func (m *Manager) setPersistentEnvironment(javaHome string) error {
	shell, err := m.DetectShell()
	if err != nil {
		return fmt.Errorf("failed to detect shell: %w", err)
	}
	
	color.Blue("Detected shell: %s", shell.Name)
	
	switch shell.Name {
	case "powershell":
		return m.setPowerShellEnvironment(shell, javaHome)
	case "cmd":
		return m.setCmdEnvironment(javaHome)
	case "bash", "zsh":
		return m.setUnixShellEnvironment(shell, javaHome)
	case "fish":
		return m.setFishEnvironment(shell, javaHome)
	default:
		return m.setUnixShellEnvironment(shell, javaHome)
	}
}

// setPowerShellEnvironment 设置 PowerShell 环境变量
func (m *Manager) setPowerShellEnvironment(shell *ShellInfo, javaHome string) error {
	// 确保 PowerShell profile 目录存在
	profileDir := filepath.Dir(shell.ConfigFile)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create PowerShell profile directory: %w", err)
	}
	
	// 读取现有的 profile 内容
	var existingContent []string
	if content, err := os.ReadFile(shell.ConfigFile); err == nil {
		existingContent = strings.Split(string(content), "\n")
	}
	
	// 移除现有的 JVM 相关设置
	var newContent []string
	inJVMBlock := false
	
	for _, line := range existingContent {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Java Version Manager - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Java Version Manager - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 添加新的 JVM 设置
	javaBin := filepath.Join(javaHome, "bin")
	jvmBlock := []string{
		"",
		"# JVM Java Version Manager - START",
		fmt.Sprintf("$env:JAVA_HOME = \"%s\"", javaHome),
		fmt.Sprintf("$env:PATH = \"%s;\" + ($env:PATH -replace \"[^;]*java[^;]*;?\", \"\")", javaBin),
		"# JVM Java Version Manager - END",
		"",
	}
	
	newContent = append(newContent, jvmBlock...)
	
	// 写入文件
	finalContent := strings.Join(newContent, "\n")
	if err := os.WriteFile(shell.ConfigFile, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write PowerShell profile: %w", err)
	}
	
	color.Green("✓ PowerShell profile updated: %s", shell.ConfigFile)
	color.Yellow("Please restart PowerShell or run: . $PROFILE")
	
	return nil
}

// setCmdEnvironment 设置 Windows CMD 环境变量
func (m *Manager) setCmdEnvironment(javaHome string) error {
	color.Yellow("CMD does not support persistent environment variables through configuration files.")
	color.Yellow("Please set environment variables manually through System Properties or use PowerShell.")
	color.Yellow("")
	color.Yellow("Manual steps:")
	color.Yellow("1. Open System Properties > Advanced > Environment Variables")
	color.Yellow("2. Set JAVA_HOME = %s", javaHome)
	color.Yellow("3. Add %s to PATH", filepath.Join(javaHome, "bin"))
	
	return nil
}

// setUnixShellEnvironment 设置 Unix shell 环境变量
func (m *Manager) setUnixShellEnvironment(shell *ShellInfo, javaHome string) error {
	// 确保配置文件目录存在
	configDir := filepath.Dir(shell.ConfigFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// 读取现有配置
	var existingContent []string
	if content, err := os.ReadFile(shell.ConfigFile); err == nil {
		existingContent = strings.Split(string(content), "\n")
	}
	
	// 移除现有的 JVM 相关设置
	var newContent []string
	inJVMBlock := false
	
	for _, line := range existingContent {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Java Version Manager - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Java Version Manager - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 添加新的 JVM 设置
	javaBin := filepath.Join(javaHome, "bin")
	jvmBlock := []string{
		"",
		"# JVM Java Version Manager - START",
		fmt.Sprintf("export JAVA_HOME=\"%s\"", javaHome),
		fmt.Sprintf("export PATH=\"%s:$PATH\"", javaBin),
		"# JVM Java Version Manager - END",
		"",
	}
	
	newContent = append(newContent, jvmBlock...)
	
	// 写入文件
	finalContent := strings.Join(newContent, "\n")
	if err := os.WriteFile(shell.ConfigFile, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}
	
	color.Green("✓ Shell configuration updated: %s", shell.ConfigFile)
	color.Yellow("Please restart your terminal or run: source %s", shell.ConfigFile)
	
	return nil
}

// setFishEnvironment 设置 Fish shell 环境变量
func (m *Manager) setFishEnvironment(shell *ShellInfo, javaHome string) error {
	// Fish shell 使用不同的语法
	configDir := filepath.Dir(shell.ConfigFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create fish config directory: %w", err)
	}
	
	var existingContent []string
	if content, err := os.ReadFile(shell.ConfigFile); err == nil {
		existingContent = strings.Split(string(content), "\n")
	}
	
	// 移除现有的 JVM 相关设置
	var newContent []string
	inJVMBlock := false
	
	for _, line := range existingContent {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Java Version Manager - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Java Version Manager - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 添加新的 JVM 设置（Fish 语法）
	javaBin := filepath.Join(javaHome, "bin")
	jvmBlock := []string{
		"",
		"# JVM Java Version Manager - START",
		fmt.Sprintf("set -gx JAVA_HOME \"%s\"", javaHome),
		fmt.Sprintf("set -gx PATH \"%s\" $PATH", javaBin),
		"# JVM Java Version Manager - END",
		"",
	}
	
	newContent = append(newContent, jvmBlock...)
	
	// 写入文件
	finalContent := strings.Join(newContent, "\n")
	if err := os.WriteFile(shell.ConfigFile, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write fish config: %w", err)
	}
	
	color.Green("✓ Fish shell configuration updated: %s", shell.ConfigFile)
	color.Yellow("Please restart your terminal or run: source %s", shell.ConfigFile)
	
	return nil
}

// removeJavaFromPath 从 PATH 中移除现有的 Java 路径
func (m *Manager) removeJavaFromPath(path string) string {
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	
	paths := strings.Split(path, separator)
	var newPaths []string
	
	for _, p := range paths {
		// 跳过包含 java 的路径（简单的启发式方法）
		if !strings.Contains(strings.ToLower(p), "java") {
			newPaths = append(newPaths, p)
		}
	}
	
	return strings.Join(newPaths, separator)
}

// GetCurrentJavaInfo 获取当前 Java 环境信息
func (m *Manager) GetCurrentJavaInfo() (map[string]string, error) {
	info := make(map[string]string)
	
	// 获取 JAVA_HOME
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		info["JAVA_HOME"] = javaHome
	}
	
	// 获取 PATH 中的 Java
	if path := os.Getenv("PATH"); path != "" {
		separator := ":"
		if runtime.GOOS == "windows" {
			separator = ";"
		}
		
		paths := strings.Split(path, separator)
		for _, p := range paths {
			if strings.Contains(strings.ToLower(p), "java") && strings.Contains(p, "bin") {
				info["JAVA_BIN_PATH"] = p
				break
			}
		}
	}
	
	return info, nil
}
