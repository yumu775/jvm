package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"jvm/internal/config"
	"jvm/internal/version"
)

// Manager 负责卸载 Java 版本
type Manager struct {
	versionManager *version.Manager
}

// NewManager 创建一个新的卸载管理器
func NewManager() (*Manager, error) {
	versionManager, err := version.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize version manager: %w", err)
	}
	
	return &Manager{
		versionManager: versionManager,
	}, nil
}

// UninstallOptions 卸载选项
type UninstallOptions struct {
	Version        string // 要卸载的版本
	Force          bool   // 强制卸载，即使是当前版本
	KeepDownloads  bool   // 保留下载文件
	CleanConfig    bool   // 清理配置文件中的引用
	DryRun         bool   // 仅显示将要删除的内容，不实际删除
}

// UninstallResult 卸载结果
type UninstallResult struct {
	Version       string   // 卸载的版本
	RemovedPaths  []string // 删除的路径
	RemovedFiles  []string // 删除的文件
	CleanedConfig bool     // 是否清理了配置
	Warnings      []string // 警告信息
	BytesFreed    int64    // 释放的字节数
}

// Uninstall 卸载指定版本的 Java
func (m *Manager) Uninstall(options UninstallOptions) (*UninstallResult, error) {
	result := &UninstallResult{
		Version: options.Version,
	}
	
	// 验证版本是否存在
	if !m.versionManager.IsInstalled(options.Version) {
		return nil, fmt.Errorf("Java version %s is not installed", options.Version)
	}
	
	// 检查是否是当前激活版本
	if currentVersion, err := m.versionManager.GetCurrent(); err == nil && currentVersion == options.Version {
		if !options.Force {
			return nil, fmt.Errorf("cannot uninstall currently active version %s (use --force to override)", options.Version)
		}
		result.Warnings = append(result.Warnings, "Uninstalling currently active version")
	}
	
	// 获取版本路径
	versionPath, err := m.versionManager.GetVersionPath(options.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version path: %w", err)
	}
	
	color.Blue("Preparing to uninstall Java %s from %s", options.Version, versionPath)
	
	// 计算将要删除的内容
	if err := m.calculateRemovalSize(versionPath, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to calculate size: %v", err))
	}
	
	// 如果是 dry run，只显示信息
	if options.DryRun {
		return m.performDryRun(versionPath, result)
	}
	
	// 执行实际卸载
	return m.performUninstall(versionPath, options, result)
}

// calculateRemovalSize 计算将要删除的内容大小
func (m *Manager) calculateRemovalSize(versionPath string, result *UninstallResult) error {
	return filepath.Walk(versionPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续处理
		}
		
		if !info.IsDir() {
			result.BytesFreed += info.Size()
			result.RemovedFiles = append(result.RemovedFiles, path)
		} else {
			result.RemovedPaths = append(result.RemovedPaths, path)
		}
		
		return nil
	})
}

// performDryRun 执行 dry run
func (m *Manager) performDryRun(versionPath string, result *UninstallResult) (*UninstallResult, error) {
	color.Yellow("=== DRY RUN - No files will be deleted ===")
	fmt.Println()
	
	color.Blue("Would remove directory:")
	fmt.Printf("  %s\n", versionPath)
	fmt.Println()
	
	color.Blue("Summary:")
	fmt.Printf("  Files to remove: %d\n", len(result.RemovedFiles))
	fmt.Printf("  Directories to remove: %d\n", len(result.RemovedPaths))
	fmt.Printf("  Space to free: %s\n", formatBytes(result.BytesFreed))
	
	if len(result.Warnings) > 0 {
		fmt.Println()
		color.Yellow("Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	
	fmt.Println()
	color.Cyan("Use 'jvm uninstall %s' to actually remove this version", result.Version)
	
	return result, nil
}

// performUninstall 执行实际卸载
func (m *Manager) performUninstall(versionPath string, options UninstallOptions, result *UninstallResult) (*UninstallResult, error) {
	// 显示卸载信息
	color.Yellow("Uninstalling Java %s...", options.Version)
	fmt.Printf("Removing: %s\n", versionPath)
	fmt.Printf("Files: %d, Space: %s\n", len(result.RemovedFiles), formatBytes(result.BytesFreed))
	
	// 删除版本目录
	if err := os.RemoveAll(versionPath); err != nil {
		return nil, fmt.Errorf("failed to remove version directory: %w", err)
	}
	
	color.Green("✓ Removed installation directory")
	
	// 清理下载文件（如果需要）
	if !options.KeepDownloads {
		if err := m.cleanDownloads(options.Version, result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to clean downloads: %v", err))
		}
	}
	
	// 清理配置（如果需要）
	if options.CleanConfig {
		if err := m.cleanConfiguration(options.Version, result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to clean configuration: %v", err))
		}
	}
	
	// 如果卸载的是当前版本，清除当前版本设置
	if currentVersion, err := m.versionManager.GetCurrent(); err == nil && currentVersion == options.Version {
		if err := m.clearCurrentVersion(result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to clear current version: %v", err))
		}
	}
	
	color.Green("Successfully uninstalled Java %s", options.Version)
	
	// 显示摘要
	m.displaySummary(result)
	
	return result, nil
}

// cleanDownloads 清理下载文件
func (m *Manager) cleanDownloads(version string, result *UninstallResult) error {
	downloadDir, err := config.GetDownloadsDir()
	if err != nil {
		return err
	}
	
	// 查找相关的下载文件
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return err
	}
	
	var removedDownloads []string
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fileName := entry.Name()
		// 检查文件名是否包含版本号
		if strings.Contains(fileName, version) || strings.Contains(fileName, fmt.Sprintf("java-%s", version)) {
			filePath := filepath.Join(downloadDir, fileName)
			
			if info, err := entry.Info(); err == nil {
				result.BytesFreed += info.Size()
			}
			
			if err := os.Remove(filePath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to remove download file %s: %v", fileName, err))
			} else {
				removedDownloads = append(removedDownloads, filePath)
			}
		}
	}
	
	if len(removedDownloads) > 0 {
		color.Green("✓ Cleaned %d download file(s)", len(removedDownloads))
		result.RemovedFiles = append(result.RemovedFiles, removedDownloads...)
	}
	
	return nil
}

// cleanConfiguration 清理配置文件中的引用
func (m *Manager) cleanConfiguration(version string, result *UninstallResult) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	
	configChanged := false
	
	// 如果当前版本是被卸载的版本，清除它
	if cfg.CurrentVersion == version {
		cfg.CurrentVersion = ""
		configChanged = true
	}
	
	// 如果默认版本是被卸载的版本，清除它
	if cfg.DefaultVersion == version {
		cfg.DefaultVersion = ""
		configChanged = true
	}
	
	if configChanged {
		if err := cfg.SaveConfig(); err != nil {
			return err
		}
		
		color.Green("✓ Cleaned configuration references")
		result.CleanedConfig = true
	}
	
	return nil
}

// clearCurrentVersion 清除当前版本设置
func (m *Manager) clearCurrentVersion(result *UninstallResult) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	
	cfg.CurrentVersion = ""
	
	if err := cfg.SaveConfig(); err != nil {
		return err
	}
	
	color.Yellow("Cleared current version setting")
	result.Warnings = append(result.Warnings, "No Java version is currently active")
	
	return nil
}

// displaySummary 显示卸载摘要
func (m *Manager) displaySummary(result *UninstallResult) {
	fmt.Println()
	color.Blue("=== Uninstall Summary ===")
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Files removed: %d\n", len(result.RemovedFiles))
	fmt.Printf("Space freed: %s\n", formatBytes(result.BytesFreed))
	
	if result.CleanedConfig {
		fmt.Printf("Configuration cleaned: Yes\n")
	}
	
	if len(result.Warnings) > 0 {
		fmt.Println()
		color.Yellow("Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	
	// 建议下一步操作
	fmt.Println()
	color.Cyan("Next steps:")
	fmt.Printf("  - Use 'jvm list' to see remaining versions\n")
	fmt.Printf("  - Use 'jvm use <version>' to activate another version\n")
}

// ListUninstallableVersions 列出可卸载的版本
func (m *Manager) ListUninstallableVersions() ([]version.JavaVersion, error) {
	return m.versionManager.ListInstalled()
}

// GetUninstallInfo 获取卸载信息（不实际卸载）
func (m *Manager) GetUninstallInfo(version string) (*UninstallResult, error) {
	options := UninstallOptions{
		Version: version,
		DryRun:  true,
	}
	
	return m.Uninstall(options)
}

// formatBytes 格式化字节数为人类可读格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
