package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
)

// configCmd 定义了 "jvm config" 命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理 JVM 工具配置",
	Long: `管理 JVM 工具的配置选项。

支持的配置项：
- scan-paths: 自定义 Java 扫描路径
- auto-scan: 是否自动扫描系统 Java
- download-sources: 下载源配置

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
	for i, source := range cfg.DownloadSources {
		fmt.Printf("  %d. %s - %s\n", i+1, source.Name, source.URL)
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
	case "auto-scan":
		if value == "true" {
			cfg.AutoScan = true
		} else if value == "false" {
			cfg.AutoScan = false
		} else {
			return fmt.Errorf("invalid value for auto-scan: %s (use true or false)", value)
		}
	case "default-version":
		cfg.DefaultVersion = value
	default:
		return fmt.Errorf("config key '%s' is not settable", key)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	color.Green("Configuration updated: %s = %s", key, value)
	return nil
}

// addScanPath 添加扫描路径
func addScanPath(path string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 检查路径是否已存在
	for _, existingPath := range cfg.CustomScanPaths {
		if existingPath == path {
			color.Yellow("Path already exists: %s", path)
			return nil
		}
	}

	// 添加路径
	cfg.CustomScanPaths = append(cfg.CustomScanPaths, path)

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	color.Green("Added scan path: %s", path)
	color.Cyan("Use 'jvm scan' or 'jvm import --from %s' to scan this path", path)

	return nil
}

// removeScanPath 移除扫描路径
func removeScanPath(path string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 查找并移除路径
	found := false
	var newPaths []string
	for _, existingPath := range cfg.CustomScanPaths {
		if existingPath != path {
			newPaths = append(newPaths, existingPath)
		} else {
			found = true
		}
	}

	if !found {
		color.Yellow("Path not found: %s", path)
		return nil
	}

	cfg.CustomScanPaths = newPaths

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	color.Green("Removed scan path: %s", path)
	return nil
}

// getValueOrDefault 获取值或默认值
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
