package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/env"
)

var (
	setupForce      bool
	setupUninstall  bool
	setupSystemWide bool
	setupCustomPath string
)

// setupCmd 定义了 "jvm setup" 命令
// 这个命令用于配置 JVM 工具的环境变量
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "配置 JVM 工具的环境变量",
	Long: `配置 JVM 工具的环境变量，使你可以在任何地方直接使用 'jvm' 命令。

这个命令会：
1. 检测当前 jvm.exe 的位置
2. 将其添加到系统 PATH 环境变量
3. 配置 shell 启动脚本
4. 验证配置是否成功

示例：
  jvm setup                    # 自动配置环境变量
  jvm setup --force           # 强制重新配置
  jvm setup --system-wide     # 系统级配置（需要管理员权限）
  jvm setup --path C:\\tools   # 复制到指定目录并配置
  jvm setup --uninstall       # 移除环境变量配置`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if setupUninstall {
			return uninstallJVMFromPath()
		}
		
		return setupJVMEnvironment()
	},
}

// setupJVMEnvironment 配置 JVM 工具环境变量
func setupJVMEnvironment() error {
	color.Blue("=== JVM Tool Environment Setup ===")
	fmt.Println()
	
	// 获取当前可执行文件路径
	currentExePath, err := getCurrentExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	
	color.Green("Current JVM executable: %s", currentExePath)
	
	var targetPath string
	var targetDir string
	
	if setupCustomPath != "" {
		// 使用自定义路径
		targetDir = setupCustomPath
		targetPath = filepath.Join(targetDir, "jvm.exe")
		
		// 确保目录存在
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
		
		// 复制可执行文件
		color.Blue("Copying JVM executable to: %s", targetPath)
		if err := copyFile(currentExePath, targetPath); err != nil {
			return fmt.Errorf("failed to copy executable: %w", err)
		}
		
		color.Green("✓ JVM executable copied successfully")
		
	} else {
		// 使用当前路径
		targetPath = currentExePath
		targetDir = filepath.Dir(currentExePath)
	}
	
	// 检查是否已在 PATH 中
	if isInPath(targetDir) && !setupForce {
		color.Green("✓ JVM tool is already in PATH")
		return verifySetup()
	}
	
	// 添加到 PATH
	color.Blue("Adding JVM tool to PATH...")
	if err := addToPath(targetDir); err != nil {
		return fmt.Errorf("failed to add to PATH: %w", err)
	}
	
	// 验证设置
	return verifySetup()
}

// uninstallJVMFromPath 从 PATH 中移除 JVM 工具
func uninstallJVMFromPath() error {
	color.Blue("=== Removing JVM Tool from Environment ===")
	fmt.Println()
	
	// 获取当前可执行文件路径
	currentExePath, err := getCurrentExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	
	currentDir := filepath.Dir(currentExePath)
	
	// 从 PATH 中移除
	color.Blue("Removing from PATH...")
	if err := removeFromPath(currentDir); err != nil {
		return fmt.Errorf("failed to remove from PATH: %w", err)
	}
	
	color.Green("✓ JVM tool removed from environment")
	color.Yellow("Note: You may need to restart your terminal for changes to take effect")
	
	return nil
}

// getCurrentExecutablePath 获取当前可执行文件路径
func getCurrentExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	
	// 解析符号链接
	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return exePath, nil // 如果无法解析，使用原路径
	}
	
	return realPath, nil
}

// isInPath 检查目录是否已在 PATH 中
func isInPath(dir string) bool {
	pathEnv := os.Getenv("PATH")
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	
	paths := strings.Split(pathEnv, separator)
	for _, path := range paths {
		if strings.EqualFold(strings.TrimSpace(path), dir) {
			return true
		}
	}
	
	return false
}

// addToPath 添加目录到 PATH
func addToPath(dir string) error {
	envManager := env.NewManager()
	
	// 检测 shell
	shell, err := envManager.DetectShell()
	if err != nil {
		return fmt.Errorf("failed to detect shell: %w", err)
	}
	
	color.Blue("Detected shell: %s", shell.Name)
	
	switch runtime.GOOS {
	case "windows":
		return addToWindowsPath(dir, shell)
	case "darwin", "linux":
		return addToUnixPath(dir, shell)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// addToWindowsPath 在 Windows 上添加到 PATH
func addToWindowsPath(dir string, shell *env.ShellInfo) error {
	if setupSystemWide {
		// 系统级配置（需要管理员权限）
		color.Yellow("System-wide configuration requires administrator privileges")
		color.Yellow("Please run as administrator or use user-level configuration")
		return fmt.Errorf("administrator privileges required for system-wide setup")
	}
	
	// 用户级配置
	if shell.Name == "powershell" {
		return addToPowerShellProfile(dir, shell.ConfigFile)
	}
	
	// 对于其他 shell，提供手动配置说明
	color.Yellow("Manual configuration required for %s", shell.Name)
	color.Yellow("Please add the following to your PATH:")
	color.Yellow("  %s", dir)
	
	return nil
}

// addToPowerShellProfile 添加到 PowerShell profile
func addToPowerShellProfile(dir, profilePath string) error {
	// 确保 profile 目录存在
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create PowerShell profile directory: %w", err)
	}
	
	// 读取现有 profile
	var existingContent []string
	if content, err := os.ReadFile(profilePath); err == nil {
		existingContent = strings.Split(string(content), "\n")
	}
	
	// 检查是否已经配置
	jvmMarker := "# JVM Tool PATH Configuration"
	for _, line := range existingContent {
		if strings.Contains(line, jvmMarker) {
			if !setupForce {
				color.Green("✓ JVM tool already configured in PowerShell profile")
				return nil
			}
			break
		}
	}
	
	// 移除现有的 JVM 配置
	var newContent []string
	inJVMBlock := false
	
	for _, line := range existingContent {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Tool PATH Configuration - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Tool PATH Configuration - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 添加新的 JVM 配置
	jvmBlock := []string{
		"",
		"# JVM Tool PATH Configuration - START",
		fmt.Sprintf("$env:PATH = \"%s;\" + $env:PATH", dir),
		"# JVM Tool PATH Configuration - END",
		"",
	}
	
	newContent = append(newContent, jvmBlock...)
	
	// 写入文件
	finalContent := strings.Join(newContent, "\n")
	if err := os.WriteFile(profilePath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write PowerShell profile: %w", err)
	}
	
	color.Green("✓ PowerShell profile updated: %s", profilePath)
	color.Yellow("Please restart PowerShell or run: . $PROFILE")
	
	return nil
}

// addToUnixPath 在 Unix 系统上添加到 PATH
func addToUnixPath(dir string, shell *env.ShellInfo) error {
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
	
	// 检查是否已经配置
	for _, line := range existingContent {
		if strings.Contains(line, "# JVM Tool PATH Configuration") {
			if !setupForce {
				color.Green("✓ JVM tool already configured in %s", shell.ConfigFile)
				return nil
			}
			break
		}
	}
	
	// 移除现有的 JVM 配置
	var newContent []string
	inJVMBlock := false
	
	for _, line := range existingContent {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Tool PATH Configuration - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Tool PATH Configuration - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 添加新的 JVM 配置
	jvmBlock := []string{
		"",
		"# JVM Tool PATH Configuration - START",
		fmt.Sprintf("export PATH=\"%s:$PATH\"", dir),
		"# JVM Tool PATH Configuration - END",
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

// removeFromPath 从 PATH 中移除目录
func removeFromPath(dir string) error {
	envManager := env.NewManager()
	
	// 检测 shell
	shell, err := envManager.DetectShell()
	if err != nil {
		return fmt.Errorf("failed to detect shell: %w", err)
	}
	
	// 读取配置文件
	content, err := os.ReadFile(shell.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read shell config: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newContent []string
	inJVMBlock := false
	
	// 移除 JVM 配置块
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# JVM Tool PATH Configuration - START" {
			inJVMBlock = true
			continue
		}
		if trimmed == "# JVM Tool PATH Configuration - END" {
			inJVMBlock = false
			continue
		}
		if !inJVMBlock {
			newContent = append(newContent, line)
		}
	}
	
	// 写回文件
	finalContent := strings.Join(newContent, "\n")
	if err := os.WriteFile(shell.ConfigFile, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell config: %w", err)
	}
	
	return nil
}

// verifySetup 验证设置是否成功
func verifySetup() error {
	fmt.Println()
	color.Blue("Verifying setup...")
	
	// 检查是否在 PATH 中
	currentExePath, _ := getCurrentExecutablePath()
	currentDir := filepath.Dir(currentExePath)
	
	if isInPath(currentDir) {
		color.Green("✓ JVM tool directory is in PATH")
	} else {
		color.Yellow("⚠ JVM tool directory not found in current PATH")
		color.Yellow("  You may need to restart your terminal")
	}
	
	// 显示使用说明
	fmt.Println()
	color.Blue("=== Setup Complete ===")
	color.Green("You can now use 'jvm' command from anywhere!")
	fmt.Println()
	color.Cyan("Next steps:")
	fmt.Printf("  1. Restart your terminal or run the source command shown above\n")
	fmt.Printf("  2. Test with: jvm --version\n")
	fmt.Printf("  3. Start managing Java versions: jvm list\n")
	
	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return err
	}
	
	// 复制权限
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	return os.Chmod(dst, sourceInfo.Mode())
}

// init 函数初始化 setup 命令的标志
func init() {
	// 添加标志
	setupCmd.Flags().BoolVarP(&setupForce, "force", "f", false, "强制重新配置，即使已经配置过")
	setupCmd.Flags().BoolVar(&setupUninstall, "uninstall", false, "从环境变量中移除 JVM 工具")
	setupCmd.Flags().BoolVar(&setupSystemWide, "system-wide", false, "系统级配置（需要管理员权限）")
	setupCmd.Flags().StringVar(&setupCustomPath, "path", "", "复制 JVM 工具到指定目录并配置")
}
