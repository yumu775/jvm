package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/sources"
	"jvm/internal/version"
)

// configCmd 定义了 "jvm config" 命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理 JVM 工具配置",
	Long: `管理 JVM 工具的配置选项。

支持的配置项：
- install-dir: Java 安装目录（修改后保留旧版本登记，不移动文件）
- download-dir: 下载缓存目录
- scan-paths: 自定义 Java 扫描路径
- auto-scan: list 在没有已管理安装时是否补充扫描系统 Java
- default-version: jvm use default 使用的已安装版本

下载源使用 jvm sources 管理；config list 显示同一份实际来源配置。

子命令：
  get       获取配置值
  set       设置配置值
  list      列出所有配置
  add-path  添加自定义扫描路径
  remove-path 移除扫描路径`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认显示所有配置
		return listConfig()
	},
}

// configGetCmd 获取配置值
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "获取配置值",
	Long: `获取指定的配置值。

示例：
  jvm config get scan-paths    # 获取扫描路径
  jvm config get auto-scan     # 获取自动扫描设置`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		return getConfig(key)
	},
}

// configSetCmd 设置配置值
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置值",
	Long: `设置指定的配置值。

示例：
  jvm config set install-dir D:\JavaVersions
  jvm config set download-dir D:\JavaCache
  jvm config set auto-scan true           # 启用自动扫描
  jvm config set auto-scan false          # 禁用自动扫描`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		return setConfig(key, value)
	},
}

// configListCmd 列出所有配置
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有配置",
	Long:  `列出所有当前的配置选项和值。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listConfig()
	},
}

// configAddPathCmd 添加扫描路径
var configAddPathCmd = &cobra.Command{
	Use:   "add-path <path>",
	Short: "添加自定义扫描路径",
	Long: `添加自定义的 Java 扫描路径。

这些路径会在扫描和导入时被包含。

示例：
  jvm config add-path E:\\JavaVersions    # 添加自定义路径
  jvm config add-path /opt/java           # 添加 Linux 路径`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		return addScanPath(path)
	},
}

// configRemovePathCmd 移除扫描路径
var configRemovePathCmd = &cobra.Command{
	Use:   "remove-path <path>",
	Short: "移除扫描路径",
	Long: `移除指定的自定义扫描路径。

示例：
  jvm config remove-path E:\\JavaVersions  # 移除指定路径`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		return removeScanPath(path)
	},
}

// listConfig 列出所有配置
func listConfig() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	color.Blue("=== JVM Tool Configuration ===")
	fmt.Println()

	color.Green("Basic Settings:")
	fmt.Printf("  Current Version: %s\n", getValueOrDefault(cfg.CurrentVersion, "(none)"))
	fmt.Printf("  Default Version: %s\n", getValueOrDefault(cfg.DefaultVersion, "(none)"))
	fmt.Printf("  Auto Scan: %t\n", cfg.AutoScan)
	fmt.Printf("  Install Directory: %s\n", cfg.InstallDir)
	fmt.Printf("  Download Directory: %s\n", cfg.DownloadDir)

	fmt.Println()
	color.Green("Custom Scan Paths:")
	if len(cfg.CustomScanPaths) == 0 {
		fmt.Printf("  (none configured)\n")
	} else {
		for i, path := range cfg.CustomScanPaths {
			fmt.Printf("  %d. %s\n", i+1, path)
		}
	}

	fmt.Println()
	color.Green("Download Sources:")
	manager := sources.NewSourceManager()
	allSources, err := manager.LoadSources()
	if err != nil {
		return err
	}
	defaultSource, err := manager.DefaultSource()
	if err != nil {
		return err
	}
	if err := writeSourcesTable(os.Stdout, allSources, defaultSource); err != nil {
		return err
	}

	fmt.Println()
	color.Cyan("Usage:")
	fmt.Printf("  jvm config add-path <path>      # Add custom scan path\n")
	fmt.Printf("  jvm config set auto-scan true   # Enable auto scan\n")
	fmt.Printf("  jvm config get scan-paths       # Get scan paths\n")

	return nil
}

// getConfig 获取配置值
func getConfig(key string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch key {
	case "current-version":
		fmt.Println(getValueOrDefault(cfg.CurrentVersion, "(none)"))
	case "default-version":
		fmt.Println(getValueOrDefault(cfg.DefaultVersion, "(none)"))
	case "auto-scan":
		fmt.Println(cfg.AutoScan)
	case "install-dir":
		fmt.Println(cfg.InstallDir)
	case "download-dir":
		fmt.Println(cfg.DownloadDir)
	case "scan-paths":
		if len(cfg.CustomScanPaths) == 0 {
			fmt.Println("(none)")
		} else {
			fmt.Println(strings.Join(cfg.CustomScanPaths, "\n"))
		}
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return nil
}

// setConfig 设置配置值
func setConfig(key, value string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch key {
	case "install-dir", "download-dir":
		path, err := config.AbsolutePath(value)
		if err != nil {
			return err
		}
		if key == "install-dir" {
			// 修改仓库前登记旧版本，避免目录切换后丢失可见性。
			manager, err := version.NewManager()
			if err != nil {
				return err
			}
			installed, err := manager.ListInstalled()
			if err != nil {
				return err
			}
			for _, item := range installed {
				record, err := manager.GetRecord(item.Version)
				if err != nil {
					return err
				}
				if err = manager.Register(item.Version, item.Path, record.Managed); err != nil {
					return err
				}
			}
			cfg, err = config.LoadConfig()
			if err != nil {
				return err
			}
			cfg.InstallDir = path
		} else {
			cfg.DownloadDir = path
		}
	case "auto-scan":
		if value == "true" {
			cfg.AutoScan = true
		} else if value == "false" {
			cfg.AutoScan = false
		} else {
			return fmt.Errorf("invalid value for auto-scan: %s (use true or false)", value)
		}
	case "default-version":
		if strings.EqualFold(value, "default") {
			return fmt.Errorf("default-version cannot refer to itself; select an installed version")
		}
		if err := version.ValidateVersion(value); err != nil {
			return err
		}
		manager, err := version.NewManager()
		if err != nil {
			return err
		}
		resolved, err := manager.Resolve(value)
		if err != nil {
			return err
		}
		if _, err := manager.GetVersionPath(resolved); err != nil {
			return err
		}
		cfg.DefaultVersion = resolved
		value = resolved
	default:
		return fmt.Errorf("config key '%s' is not settable", key)
	}

	if err := config.Update(func(current *config.Config) error {
		switch key {
		case "install-dir":
			current.InstallDir = cfg.InstallDir
		case "download-dir":
			current.DownloadDir = cfg.DownloadDir
		case "auto-scan":
			current.AutoScan = cfg.AutoScan
		case "default-version":
			current.DefaultVersion = cfg.DefaultVersion
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	color.Green("Configuration updated: %s = %s", key, value)
	return nil
}

// addScanPath 添加扫描路径
func addScanPath(path string) error {
	normalized, err := config.AbsolutePath(path)
	if err != nil {
		return err
	}
	err = config.Update(func(c *config.Config) error {
		for _, p := range c.CustomScanPaths {
			if sameScanPath(p, normalized) {
				return nil
			}
		}
		c.CustomScanPaths = append(c.CustomScanPaths, normalized)
		return nil
	})
	if err == nil {
		color.Green("Added scan path: %s", normalized)
	}
	return err
}

func removeScanPath(path string) error {
	normalized, err := config.AbsolutePath(path)
	if err != nil {
		return err
	}
	return config.Update(func(c *config.Config) error {
		var paths []string
		for _, p := range c.CustomScanPaths {
			if !sameScanPath(p, normalized) {
				paths = append(paths, p)
			}
		}
		c.CustomScanPaths = paths
		return nil
	})
}

func sameScanPath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// init 函数初始化 config 命令和子命令
func init() {
	// 添加子命令
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configAddPathCmd)
	configCmd.AddCommand(configRemovePathCmd)
}
