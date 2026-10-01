package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/env"
	"jvm/internal/version"
)

// envCmd 定义了 "jvm env" 命令
// 这个命令显示当前的 Java 环境信息
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "显示当前的 Java 环境信息",
	Long: `显示当前的 Java 环境信息。

这个命令会显示：
1. 配置中记录的默认 Java 版本（不代表当前终端已激活）
2. JAVA_HOME 环境变量
3. PATH 中的 Java 路径
4. 实际运行的 java 命令位置
5. Shell 配置信息

示例：
  jvm env                      # 显示完整的环境信息
  jvm env --show-shell        # 显示 shell 配置信息
  jvm env --shell powershell  # 输出供当前 PowerShell 执行的纯激活脚本
  jvm env --shell cmd         # 输出供 CMD call 执行的批处理脚本`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if shell, _ := cmd.Flags().GetString("shell"); shell != "" {
			return emitActivation(cmd, shell)
		}
		if cmd.Flags().Changed("java-version") || cmd.Flags().Changed("java-home") {
			return fmt.Errorf("--java-version and --java-home require --shell")
		}
		// 创建管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}

		envManager := env.NewManager()

		color.Blue("=== Java Environment Information ===")
		fmt.Println()
		if runtime.GOOS == "windows" {
			stored, err := env.ReadPersistentEnvironment()
			if err != nil {
				return err
			}
			fmt.Printf("Persistent user JAVA_HOME: %s\n", stored["JAVA_HOME"])
			if stored["JAVA_HOME"] != "" && !sameJavaPath(stored["JAVA_HOME"], os.Getenv("JAVA_HOME")) {
				color.Yellow("Current terminal JAVA_HOME differs from persistent user environment; activate explicitly.")
			}
		}
		if actual, err := exec.LookPath("java"); err == nil && os.Getenv("JAVA_HOME") != "" {
			if !sameJavaPath(filepath.Dir(actual), filepath.Join(os.Getenv("JAVA_HOME"), "bin")) {
				color.Yellow("PATH resolves Java outside JAVA_HOME/bin. Another installation or system PATH entry takes precedence.")
			}
		}

		// 显示 JVM 管理的当前版本
		color.Green("JVM Managed Version:")
		if currentVersion, err := manager.GetCurrent(); err == nil {
			fmt.Printf("  Managed default: %s\n", currentVersion)
			if versionPath, err := manager.GetVersionPath(currentVersion); err == nil {
				fmt.Printf("  Path: %s\n", versionPath)
			}
		} else {
			color.Yellow("  No version currently managed by JVM")
		}
		fmt.Println()

		// 显示环境变量信息
		color.Green("Environment Variables:")
		envInfo, err := envManager.GetCurrentJavaInfo()
		if err != nil {
			color.Red("  Failed to get environment info: %v", err)
		} else {
			if javaHome, exists := envInfo["JAVA_HOME"]; exists {
				fmt.Printf("  JAVA_HOME = %s\n", javaHome)
			} else {
				color.Yellow("  JAVA_HOME is not set")
			}

			if javaBinPath, exists := envInfo["JAVA_BIN_PATH"]; exists {
				fmt.Printf("  Java in PATH = %s\n", javaBinPath)
			} else {
				color.Yellow("  No Java found in PATH")
			}
		}
		fmt.Println()

		// 显示实际运行的 Java 信息
		color.Green("Active Java Runtime:")
		runtimeErr := showActiveJavaInfo()
		if runtimeErr != nil {
			color.Red("  Failed to get Java runtime info: %v", runtimeErr)
		}
		fmt.Println()

		// 显示 Shell 信息
		if showShell, _ := cmd.Flags().GetBool("show-shell"); showShell {
			color.Green("Shell Configuration:")
			if err := showShellInfo(envManager); err != nil {
				color.Red("  Failed to get shell info: %v", err)
			}
			fmt.Println()
		}

		// 显示系统信息
		color.Green("System Information:")
		fmt.Printf("  OS: %s\n", runtime.GOOS)
		fmt.Printf("  Architecture: %s\n", runtime.GOARCH)

		return runtimeErr
	},
}

// showActiveJavaInfo 显示当前激活的 Java 运行时信息
func showActiveJavaInfo() error {
	// 检查 java 命令是否可用
	javaCmd := "java"
	if runtime.GOOS == "windows" {
		javaCmd = "java.exe"
	}

	// 获取 java 命令的位置
	javaPath, err := exec.LookPath(javaCmd)
	if err != nil {
		return fmt.Errorf("Java command not found in PATH: %w", err)
	}

	fmt.Printf("  Java executable: %s\n", javaPath)

	// 获取 Java 版本信息
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, javaPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get Java version: %w", err)
	}

	// 解析并显示版本信息
	versionLines := parseJavaVersionOutput(string(output))
	for _, line := range versionLines {
		fmt.Printf("  %s\n", line)
	}

	return nil
}

// parseJavaVersionOutput 解析 java -version 的输出
func parseJavaVersionOutput(output string) []string {
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			// 简化输出，只显示关键信息
			if len(line) > 100 {
				line = line[:100] + "..."
			}
			lines = append(lines, line)
		}
	}
	return lines
}

// showShellInfo 显示 Shell 配置信息
func showShellInfo(envManager *env.Manager) error {
	shell, err := envManager.DetectShell()
	if err != nil {
		return err
	}

	fmt.Printf("  Detected shell: %s\n", shell.Name)
	fmt.Printf("  Config file: %s\n", shell.ConfigFile)

	// 检查配置文件是否存在
	if _, err := os.Stat(shell.ConfigFile); err == nil {
		fmt.Printf("  Config file exists: Yes\n")

		// 检查是否包含 JVM 配置
		if hasJVMConfig, err := checkJVMConfigInFile(shell.ConfigFile); err == nil {
			if hasJVMConfig {
				color.Green("  JVM configuration: Found")
			} else {
				color.Yellow("  JVM configuration: Not found")
			}
		}
	} else {
		color.Yellow("  Config file exists: No")
	}

	return nil
}

// checkJVMConfigInFile 检查文件中是否包含 JVM 配置
func checkJVMConfigInFile(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	contentStr := string(content)
	return strings.Contains(contentStr, "JVM Java Version Manager"), nil
}

// init 函数初始化 env 命令的标志
func init() {
	// 添加显示 shell 信息标志
	envCmd.Flags().String("shell", "", "输出纯激活脚本：powershell/cmd/bash/zsh/sh/fish")
	envCmd.Flags().Bool("show-shell", false, "显示 shell 配置信息")
	envCmd.Flags().String("java-version", "", "激活指定已安装版本，不保存默认选择")
	envCmd.Flags().String("java-home", "", "激活指定 Java 目录，不保存默认选择")
	envCmd.MarkFlagsMutuallyExclusive("java-version", "java-home")
}

func emitActivation(cmd *cobra.Command, shell string) error {
	home, _ := cmd.Flags().GetString("java-home")
	target, _ := cmd.Flags().GetString("java-version")
	if home == "" && target == "" && runtime.GOOS == "windows" {
		stored, err := env.ReadPersistentEnvironment()
		if err != nil {
			return err
		}
		home = stored["JAVA_HOME"]
	}
	if home == "" {
		manager, err := version.NewManager()
		if err != nil {
			return err
		}
		if target == "" {
			target, err = manager.GetCurrent()
			if err != nil {
				return err
			}
		}
		home, err = manager.GetVersionPath(target)
		if err != nil {
			return err
		}
	}
	home, err := env.ValidateJavaHome(home)
	if err != nil {
		return err
	}
	script, err := env.ActivationScript(shell, home, os.Getenv("PATH"), os.Getenv("JAVA_HOME"))
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), script)
	return err
}

func sameJavaPath(a, b string) bool {
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
