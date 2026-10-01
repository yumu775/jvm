package cmd

import (
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/env"
)

// platformCmd 定义了 "jvm platform" 命令
// 这个命令显示平台支持信息和兼容性
var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "显示平台支持信息和兼容性",
	Long: `显示当前平台的支持信息和 JVM 工具的兼容性。

这个命令会显示：
1. 当前操作系统和架构信息
2. 支持的功能列表
3. Shell 兼容性信息
4. 已知的限制和注意事项

子命令：
  info      显示平台信息
  features  显示支持的功能
  shells    显示 Shell 支持信息`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showPlatformInfo()
	},
}

// platformInfoCmd 显示平台信息
var platformInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "显示平台信息",
	Long:  `显示当前平台的详细信息。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showPlatformInfo()
	},
}

// platformFeaturesCmd 显示支持的功能
var platformFeaturesCmd = &cobra.Command{
	Use:   "features",
	Short: "显示支持的功能",
	Long:  `显示当前平台支持的功能列表。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showPlatformFeatures()
	},
}

// platformShellsCmd 显示 Shell 支持信息
var platformShellsCmd = &cobra.Command{
	Use:   "shells",
	Short: "显示 Shell 支持信息",
	Long:  `显示当前平台的 Shell 支持信息。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showShellSupport()
	},
}

// showPlatformInfo 显示平台信息
func showPlatformInfo() error {
	color.Blue("=== Platform Information ===")
	fmt.Println()

	// 基本系统信息
	color.Green("System Information:")
	fmt.Printf("  Operating System: %s\n", runtime.GOOS)
	fmt.Printf("  Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("  Go Runtime: %s\n", runtime.Version())
	fmt.Printf("  CPU Cores: %d\n", runtime.NumCPU())

	// 平台特定信息
	fmt.Println()
	color.Green("Platform Details:")

	switch runtime.GOOS {
	case "windows":
		fmt.Printf("  Platform: Microsoft Windows\n")
		fmt.Printf("  Package Format: ZIP archives\n")
		fmt.Printf("  Path Separator: \\\n")
		fmt.Printf("  Executable Extension: .exe\n")
		fmt.Printf("  Default Shell: PowerShell/CMD\n")

	case "darwin":
		fmt.Printf("  Platform: macOS\n")
		fmt.Printf("  Package Format: tar.gz archives\n")
		fmt.Printf("  Path Separator: /\n")
		fmt.Printf("  Executable Extension: (none)\n")
		fmt.Printf("  Default Shell: zsh/bash\n")

	case "linux":
		fmt.Printf("  Platform: Linux\n")
		fmt.Printf("  Package Format: tar.gz archives\n")
		fmt.Printf("  Path Separator: /\n")
		fmt.Printf("  Executable Extension: (none)\n")
		fmt.Printf("  Default Shell: bash\n")

	default:
		fmt.Printf("  Platform: %s (experimental support)\n", runtime.GOOS)
		color.Yellow("  Warning: This platform may have limited support")
	}

	return nil
}

// showPlatformFeatures 显示支持的功能
func showPlatformFeatures() error {
	color.Blue("=== Platform Features ===")
	fmt.Println()

	features := getPlatformFeatures()

	color.Green("Supported Features:")
	for _, feature := range features.Supported {
		color.Green("  ✓ %s", feature)
	}

	if len(features.Limited) > 0 {
		fmt.Println()
		color.Yellow("Limited Support:")
		for _, feature := range features.Limited {
			color.Yellow("  ⚠ %s", feature)
		}
	}

	if len(features.Unsupported) > 0 {
		fmt.Println()
		color.Red("Unsupported Features:")
		for _, feature := range features.Unsupported {
			color.Red("  ✗ %s", feature)
		}
	}

	return nil
}

// showShellSupport 显示 Shell 支持信息
func showShellSupport() error {
	color.Blue("=== Shell Support ===")
	fmt.Println()

	envManager := env.NewManager()

	// 检测当前 Shell
	shell, err := envManager.DetectShell()
	if err != nil {
		return fmt.Errorf("detect shell: %w", err)
	}

	color.Green("Current Shell:")
	fmt.Printf("  Name: %s\n", shell.Name)
	fmt.Printf("  Config File: %s\n", shell.ConfigFile)
	fmt.Printf("  Set Command: %s\n", shell.SetCommand)

	fmt.Println()
	color.Green("Supported Shells:")

	shellSupport := getShellSupport()
	for shellName, support := range shellSupport {
		status := color.GreenString("✓ Full")
		if support.Limited {
			status = color.YellowString("⚠ Limited")
		}
		if !support.Supported {
			status = color.RedString("✗ No")
		}

		fmt.Printf("  %s: %s\n", shellName, status)
		if support.Notes != "" {
			fmt.Printf("    Notes: %s\n", support.Notes)
		}
	}

	return nil
}

// PlatformFeatures 平台功能支持信息
type PlatformFeatures struct {
	Supported   []string
	Limited     []string
	Unsupported []string
}

// getPlatformFeatures 获取平台功能支持信息
func getPlatformFeatures() PlatformFeatures {
	features := PlatformFeatures{
		Supported: []string{
			"Java version management",
			"Download and installation",
			"Version switching",
			"Environment variable management",
			"System Java scanning",
			"Project configuration (.jvmrc)",
			"Temurin/Zulu/Corretto/GraalVM catalogs and verified downloads",
			"Version aliases",
			"Uninstallation",
		},
	}

	switch runtime.GOOS {
	case "windows":
		features.Supported = append(features.Supported,
			"PowerShell integration",
			"ZIP archive extraction",
			"Persistent user JAVA_HOME and PATH",
		)
		features.Limited = []string{
			"Existing terminals require explicit shell activation",
			"IDE and Gradle JDK settings may override JAVA_HOME",
			"User PATH does not override entries in machine PATH",
		}

	case "darwin":
		features.Supported = append(features.Supported,
			"Bash/Zsh integration",
			"tar.gz archive extraction",
			"Symbolic links",
			"External JDK registration without copying",
		)

	case "linux":
		features.Supported = append(features.Supported,
			"Bash/Zsh/Fish integration",
			"tar.gz archive extraction",
			"Symbolic links",
			"External JDK registration without copying",
		)

	default:
		features.Limited = append(features.Limited,
			"Experimental platform support",
			"Limited shell integration",
		)
		features.Unsupported = []string{
			"Automatic environment variable setup",
		}
	}

	return features
}

// ShellSupport Shell 支持信息
type ShellSupport struct {
	Supported bool
	Limited   bool
	Notes     string
}

// getShellSupport 获取 Shell 支持信息
func getShellSupport() map[string]ShellSupport {
	support := make(map[string]ShellSupport)

	switch runtime.GOOS {
	case "windows":
		support["PowerShell"] = ShellSupport{
			Supported: true,
			Notes:     "Persistent user environment; activate existing sessions with jvm env --shell powershell",
		}
		support["CMD"] = ShellSupport{
			Supported: true,
			Notes:     "Persistent user environment; activate existing sessions with jvm env --shell cmd",
		}
		support["Git Bash"] = ShellSupport{
			Supported: true,
			Limited:   true,
			Notes:     "Use Windows CMD/PowerShell activation; MSYS path conversion is not automatically configured",
		}

	case "darwin", "linux":
		support["Bash"] = ShellSupport{
			Supported: true,
			Notes:     "Full automatic configuration support",
		}
		support["Zsh"] = ShellSupport{
			Supported: true,
			Notes:     "Full automatic configuration support",
		}
		support["Fish"] = ShellSupport{
			Supported: true,
			Notes:     "Full automatic configuration support",
		}

	default:
		support["Default"] = ShellSupport{
			Supported: true,
			Limited:   true,
			Notes:     "Basic support, manual configuration may be required",
		}
	}

	return support
}

// init 函数初始化 platform 命令和子命令
func init() {
	// 添加子命令
	platformCmd.AddCommand(platformInfoCmd)
	platformCmd.AddCommand(platformFeaturesCmd)
	platformCmd.AddCommand(platformShellsCmd)
}
