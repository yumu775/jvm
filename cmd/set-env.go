package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/env"
	"jvm/internal/scanner"
)

var (
	envTempOnly   bool
	envPersistent bool
	envAutoDetect bool
	setEnvShell   string
)

// setEnvCmd 定义了 "jvm set-env" 命令
// 这个命令用于直接设置环境变量到指定的 Java 路径
var setEnvCmd = &cobra.Command{
	Use:   "set-env <java-path>",
	Short: "直接设置环境变量到指定的 Java 路径",
	Long: `直接设置环境变量到指定的 Java 安装路径。

这个命令允许你：
1. 直接指定 Java 安装路径设置环境变量
2. 自动检测路径中的 Java 版本信息
3. 支持临时或持久化设置
4. 无需通过 JVM 管理即可使用系统中的 Java

示例：
  jvm set-env E:\\Java\\jdk-17.0.8        # 设置到指定路径
  jvm set-env E:\\Java\\jdk-11 --temp --shell powershell # 输出临时激活脚本
  jvm set-env E:\\Java\\jdk-21 --persistent # 持久化设置
  jvm set-env --auto-detect E:\\Java       # 自动检测并选择版本`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		javaPath := args[0]
		if setEnvShell != "" && !envTempOnly {
			return fmt.Errorf("--shell requires --temp")
		}
		if envTempOnly {
			if envAutoDetect {
				return fmt.Errorf("--temp cannot be combined with --auto-detect; specify the Java home")
			}
			if setEnvShell == "" {
				return fmt.Errorf("--temp requires --shell")
			}
			home, err := env.ValidateJavaHome(javaPath)
			if err != nil {
				return err
			}
			script, err := env.ActivationScript(setEnvShell, home, os.Getenv("PATH"), os.Getenv("JAVA_HOME"))
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), script)
			return err
		}

		// 验证路径是否存在
		if _, err := os.Stat(javaPath); os.IsNotExist(err) {
			return fmt.Errorf("Java path does not exist: %s", javaPath)
		}

		// 如果启用自动检测，扫描路径中的 Java 版本
		if envAutoDetect {
			return autoDetectAndSetEnv(javaPath)
		}

		// 验证是否是有效的 Java 安装
		javaScanner := scanner.NewScanner()
		installation := javaScanner.AnalyzeJavaInstallation(javaPath)
		if installation == nil {
			return fmt.Errorf("invalid Java installation path: %s", javaPath)
		}

		// 显示检测到的 Java 信息
		color.Green("Detected Java installation:")
		fmt.Printf("  Version: %s\n", installation.Version)
		fmt.Printf("  Vendor: %s\n", installation.Vendor)
		fmt.Printf("  Type: %s\n", installation.Type)
		fmt.Printf("  Architecture: %s\n", installation.Architecture)
		fmt.Printf("  Path: %s\n", installation.Path)
		fmt.Println()

		// 设置环境变量
		return setEnvironmentVariables(installation.Path, envTempOnly, envPersistent)
	},
}

// autoDetectAndSetEnv 自动检测目录中的 Java 版本并让用户选择
func autoDetectAndSetEnv(basePath string) error {
	javaScanner := scanner.NewScanner()

	color.Blue("Auto-detecting Java installations in: %s", basePath)

	// 扫描指定路径
	installations, err := javaScanner.ScanCustomPaths([]string{basePath})
	if err != nil {
		return fmt.Errorf("failed to scan path: %w", err)
	}

	if len(installations) == 0 {
		return fmt.Errorf("no Java installations found in: %s", basePath)
	}

	// 如果只有一个版本，直接使用
	if len(installations) == 1 {
		installation := installations[0]
		color.Green("Found single Java installation: %s", installation.Version)
		return setEnvironmentVariables(installation.Path, envTempOnly, envPersistent)
	}

	// 多个版本，显示列表让用户选择
	color.Blue("Found multiple Java installations:")
	for i, installation := range installations {
		fmt.Printf("  %d. Java %s (%s) - %s\n", i+1, installation.Version, installation.Vendor, installation.Path)
	}

	fmt.Print("\nSelect version (1-", len(installations), "): ")
	var choice int
	if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(installations) {
		return fmt.Errorf("invalid selection")
	}

	selectedInstallation := installations[choice-1]
	color.Green("Selected: Java %s", selectedInstallation.Version)

	return setEnvironmentVariables(selectedInstallation.Path, envTempOnly, envPersistent)
}

// setEnvironmentVariables 设置环境变量
func setEnvironmentVariables(javaPath string, tempOnly, persistent bool) error {
	if err := env.NewManager().SetJavaEnvironment(javaPath, false); err != nil {
		return err
	}
	fmt.Printf("Persistent JAVA_HOME: %s\n", javaPath)
	fmt.Println("For this external Java, activate with: jvm env --shell powershell --java-home <path> | Out-String | Invoke-Expression")
	fmt.Println("Existing terminals and IDEs need explicit activation or restart; the managed default version is unchanged.")
	return nil
}

// setEnvListCmd 列出可以设置环境变量的 Java 安装
var setEnvListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "列出可以设置环境变量的 Java 安装",
	Long: `列出系统中或指定路径中可以设置环境变量的 Java 安装。

示例：
  jvm set-env list              # 列出系统中的 Java 安装
  jvm set-env list E:\\Java     # 列出指定路径中的 Java 安装`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		javaScanner := scanner.NewScanner()

		var installations []scanner.JavaInstallation
		var err error

		if len(args) > 0 {
			// 扫描指定路径
			path := args[0]
			color.Blue("Scanning for Java installations in: %s", path)
			installations, err = javaScanner.ScanCustomPaths([]string{path})
		} else {
			// 扫描系统路径
			color.Blue("Scanning for system Java installations...")
			installations, err = javaScanner.ScanSystemJava()
		}

		if err != nil {
			return fmt.Errorf("failed to scan Java installations: %w", err)
		}

		if len(installations) == 0 {
			color.Yellow("No Java installations found.")
			return nil
		}

		color.Green("Available Java installations:")
		fmt.Println()

		for i, installation := range installations {
			fmt.Printf("%d. Java %s\n", i+1, installation.Version)
			fmt.Printf("   Vendor: %s (%s)\n", installation.Vendor, installation.Type)
			fmt.Printf("   Architecture: %s\n", installation.Architecture)
			fmt.Printf("   Path: %s\n", installation.Path)
			fmt.Printf("   Command: jvm set-env \"%s\"\n", installation.Path)
			fmt.Println()
		}

		color.Cyan("Use 'jvm set-env <path>' to set environment variables")

		return nil
	},
}

// init 函数初始化 set-env 命令和子命令
func init() {
	// 添加子命令
	setEnvCmd.AddCommand(setEnvListCmd)

	// 添加标志
	setEnvCmd.Flags().BoolVarP(&envTempOnly, "temp", "t", false, "输出临时激活脚本，需指定 --shell 并执行输出")
	setEnvCmd.Flags().BoolVarP(&envPersistent, "persistent", "p", false, "保存持久环境（默认行为）")
	setEnvCmd.Flags().BoolVar(&envAutoDetect, "auto-detect", false, "自动检测路径中的 Java 版本")
	setEnvCmd.Flags().StringVar(&setEnvShell, "shell", "", "临时激活脚本格式")
	setEnvCmd.MarkFlagsMutuallyExclusive("temp", "persistent")
}
