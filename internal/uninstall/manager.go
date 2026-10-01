package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"jvm/internal/config"
	"jvm/internal/env"
	"jvm/internal/version"
)

// Manager 负责卸载 Java 版本
type Manager struct {
	versionManager   *version.Manager
	clearEnvironment func(string) error
	readJavaUsage    func() ([]javaUsage, error)
}

// NewManager 创建一个新的卸载管理器
func NewManager() (*Manager, error) {
	versionManager, err := version.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize version manager: %w", err)
	}

	return &Manager{
		versionManager:   versionManager,
		clearEnvironment: env.NewManager().ClearJavaEnvironment,
		readJavaUsage:    readJavaUsage,
	}, nil
}

type javaUsage struct{ source, home string }

// readJavaUsage 同时检查当前进程继承的 Java 与用户持久选择，避免仅依赖配置记录。
func readJavaUsage() ([]javaUsage, error) {
	stored, err := env.ReadPersistentEnvironment()
	if err != nil {
		return nil, err
	}
	result := []javaUsage{{"persistent user JAVA_HOME", stored["JAVA_HOME"]}, {"current process JAVA_HOME", os.Getenv("JAVA_HOME")}}
	if executable, err := exec.LookPath("java"); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		result = append(result, javaUsage{"current process PATH", filepath.Dir(filepath.Dir(executable))})
	}
	return result, nil
}

func sameJavaHome(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// UninstallOptions 卸载选项
type UninstallOptions struct {
	Version       string // 要卸载的版本
	Force         bool   // 强制卸载，即使是当前版本
	KeepDownloads bool   // 保留下载文件
	CleanConfig   bool   // 清理配置文件中的引用
	DryRun        bool   // 仅显示将要删除的内容，不实际删除
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
	canonical, err := m.versionManager.Resolve(options.Version)
	if err != nil {
		return nil, err
	}
	options.Version = canonical
	record, err := m.versionManager.GetRecord(canonical)
	if err != nil {
		return nil, err
	}
	if !options.DryRun && record.Managed {
		// 与安装使用同一目标锁，防止卸载和重装同时操作目录。
		lockPath := record.Path + ".install-lock"
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("cannot acquire installation lock %s: %w", lockPath, err)
		}
		lock.Close()
		defer os.Remove(lockPath)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	result := &UninstallResult{Version: canonical}
	activeReasons := []string{}
	if cfg.CurrentVersion == canonical {
		activeReasons = append(activeReasons, "managed current selection")
		result.Warnings = append(result.Warnings, "Current selection will be cleared; existing terminal environments must be refreshed")
	}
	usage, err := m.readJavaUsage()
	if err != nil {
		return nil, fmt.Errorf("cannot verify active Java environment before uninstall: %w", err)
	}
	for _, item := range usage {
		if sameJavaHome(item.home, record.Path) {
			activeReasons = append(activeReasons, item.source)
		}
	}
	if len(activeReasons) > 0 {
		if !options.Force {
			return nil, fmt.Errorf("cannot uninstall active Java %s (%s); use --force to override", canonical, strings.Join(activeReasons, ", "))
		}
		result.Warnings = append(result.Warnings, "Java is in use by "+strings.Join(activeReasons, ", ")+"; matching persistent environment will be cleared, existing processes retain their inherited environment")
	}
	if record.Managed {
		if err := version.ValidateRemovalPath(record.Path); err != nil {
			return nil, err
		}
		if err := m.calculateRemovalSize(record.Path, result); err != nil {
			return nil, err
		}
		if options.DryRun {
			return m.performDryRun(record.Path, result)
		}
		if err := m.clearEnvironment(record.Path); err != nil {
			return nil, fmt.Errorf("failed to clear matching Java environment: %w", err)
		}
		if err := os.RemoveAll(record.Path); err != nil {
			return nil, err
		}
	} else {
		result.Warnings = append(result.Warnings, "External Java installation retained; only registration is removed")
		if options.DryRun {
			color.Yellow("Would unregister Java %s; external files remain unchanged", canonical)
			return result, nil
		}
		if err := m.clearEnvironment(record.Path); err != nil {
			return nil, fmt.Errorf("failed to clear matching Java environment: %w", err)
		}
		dir, err := config.GetVersionsDir()
		if err != nil {
			return nil, err
		}
		for _, name := range []string{"java-" + canonical, canonical} {
			p := filepath.Join(dir, name)
			info, e := os.Lstat(p)
			if e == nil && info.Mode()&os.ModeSymlink != 0 {
				if e = os.Remove(p); e != nil {
					return nil, e
				}
			}
		}
	}
	if err := m.versionManager.Unregister(canonical); err != nil {
		return nil, err
	}
	result.CleanedConfig = true
	if !options.KeepDownloads {
		result.Warnings = append(result.Warnings, "Download cache retained because legacy files have no reliable ownership metadata")
	}
	m.displaySummary(result)
	return result, nil
}

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
		Force:   true,
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
