package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

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
1. 当前激活的 Java 版本
2. JAVA_HOME 环境变量
3. PATH 中的 Java 路径
4. 实际运行的 java 命令位置
5. Shell 配置信息

示例：
  jvm env                      # 显示完整的环境信息
  jvm env --shell             # 显示 shell 配置信息`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}
		
		envManager := env.NewManager()
		
		color.Blue("=== Java Environment Information ===")
		fmt.Println()
		
		// 显示 JVM 管理的当前版本
		color.Green("JVM Managed Version:")
		if currentVersion, err := manager.GetCurrent(); err == nil {
			fmt.Printf("  Current: %s\n", currentVersion)
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
		if err := showActiveJavaInfo(); err != nil {
			color.Red("  Failed to get Java runtime info: %v", err)
		}
		fmt.Println()
		
		// 显示 Shell 信息
		if showShell, _ := cmd.Flags().GetBool("shell"); showShell {
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
		
		return nil
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
		color.Yellow("  Java command not found in PATH")
		return nil
	}
	
	fmt.Printf("  Java executable: %s\n", javaPath)
	
	// 获取 Java 版本信息
	cmd := exec.Command(javaPath, "-version")
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
	for _, line := range []string{output} {
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
	envCmd.Flags().Bool("shell", false, "显示 shell 配置信息")
}
