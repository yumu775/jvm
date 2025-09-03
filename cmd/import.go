package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/scanner"
)

var (
	importFromPath string
	importAll      bool
)

// importCmd 定义了 "jvm import" 命令
// 这个命令用于导入已存在的 Java 安装到 jvm 管理
var importCmd = &cobra.Command{
	Use:   "import <version|path>",
	Short: "导入已存在的 Java 安装到 jvm 管理",
	Long: `导入已存在的 Java 安装到 jvm 管理。

你可以通过版本号或路径来导入：
- 通过版本号：从扫描结果中导入指定版本
- 通过路径：直接导入指定路径的 Java 安装

示例：
  jvm import 17                        # 导入扫描到的 Java 17
  jvm import /path/to/java             # 导入指定路径的 Java
  jvm import 8 --from E:\\JavaVersions # 从指定目录导入 Java 8
  jvm import --all --from E:\\Java     # 导入指定目录下的所有 Java 版本`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建扫描器
		javaScanner := scanner.NewScanner()

		var installations []scanner.JavaInstallation
		var err error

		// 根据参数决定扫描方式
		if importFromPath != "" {
			// 从指定路径扫描
			color.Blue("Scanning for Java installations in: %s", importFromPath)
			installations, err = javaScanner.ScanCustomPaths([]string{importFromPath})
		} else {
			// 扫描系统默认路径
			color.Blue("Scanning for Java installations...")
			installations, err = javaScanner.ScanSystemJava()
		}

		if err != nil {
			return fmt.Errorf("failed to scan Java installations: %w", err)
		}

		// 如果没有参数且设置了 --all 标志，导入所有找到的版本
		if len(args) == 0 && importAll {
			return importAllVersions(installations)
		}

		// 如果没有参数，显示可用的安装
		if len(args) == 0 {
			return showAvailableInstallations(installations)
		}

		target := args[0]
		
		var targetInstallation *scanner.JavaInstallation
		
		// 尝试按版本号匹配
		if majorVersion, err := strconv.Atoi(target); err == nil {
			// 按主版本号匹配
			for _, installation := range installations {
				if installation.MajorVersion == majorVersion {
					targetInstallation = &installation
					break
				}
			}
		} else {
			// 尝试精确版本匹配
			for _, installation := range installations {
				if installation.Version == target {
					targetInstallation = &installation
					break
				}
			}
		}
		
		// 如果没有找到版本匹配，尝试路径匹配
		if targetInstallation == nil {
			// 检查是否是路径
			if strings.Contains(target, "/") || strings.Contains(target, "\\") {
				// 分析指定路径的 Java 安装
				if installation := javaScanner.AnalyzeJavaInstallation(target); installation != nil {
					targetInstallation = installation
				}
			}
		}
		
		if targetInstallation == nil {
			color.Red("Java installation not found: %s", target)
			fmt.Println()
			fmt.Println("Available installations:")
			
			if len(installations) == 0 {
				fmt.Println("  No Java installations found")
				fmt.Println("  Use 'jvm scan' to scan for installations")
			} else {
				for i, installation := range installations {
					fmt.Printf("  %d. Java %s (%s)\n", i+1, installation.Version, installation.Path)
				}
			}
			
			return nil
		}
		
		// 导入找到的安装
		color.Blue("Importing Java %s from %s...", targetInstallation.Version, targetInstallation.Path)
		
		versionsDir, err := config.GetVersionsDir()
		if err != nil {
			return fmt.Errorf("failed to get versions directory: %w", err)
		}
		
		if err := javaScanner.ImportInstallation(*targetInstallation, versionsDir); err != nil {
			return fmt.Errorf("failed to import Java installation: %w", err)
		}
		
		color.Green("Successfully imported Java %s", targetInstallation.Version)
		fmt.Printf("Installation path: %s\n", targetInstallation.Path)
		fmt.Printf("Use 'jvm use %s' to switch to this version.\n", targetInstallation.Version)
		
		return nil
	},
}

// importAllVersions 导入所有找到的 Java 版本
func importAllVersions(installations []scanner.JavaInstallation) error {
	if len(installations) == 0 {
		color.Yellow("No Java installations found.")
		return nil
	}

	color.Blue("Found %d Java installation(s), importing all...", len(installations))

	successCount := 0
	for _, installation := range installations {
		color.Blue("Importing Java %s from %s...", installation.Version, installation.Path)

		if err := performImport(&installation); err != nil {
			color.Red("Failed to import Java %s: %v", installation.Version, err)
		} else {
			color.Green("✓ Successfully imported Java %s", installation.Version)
			successCount++
		}
	}

	color.Green("Import completed: %d/%d versions imported successfully", successCount, len(installations))
	return nil
}

// showAvailableInstallations 显示可用的安装
func showAvailableInstallations(installations []scanner.JavaInstallation) error {
	if len(installations) == 0 {
		color.Yellow("No Java installations found.")
		fmt.Println("Try specifying a custom path with --from flag:")
		fmt.Println("  jvm import --from /path/to/java/directory")
		return nil
	}

	color.Green("Available Java installations:")
	fmt.Println()

	for i, installation := range installations {
		fmt.Printf("%d. Java %s\n", i+1, installation.Version)
		fmt.Printf("   Path: %s\n", installation.Path)
		fmt.Printf("   Vendor: %s (%s)\n", installation.Vendor, installation.Type)
		fmt.Printf("   Command: jvm import %s\n", installation.Version)
		fmt.Println()
	}

	color.Cyan("Usage:")
	fmt.Printf("  jvm import <version>     # Import specific version\n")
	fmt.Printf("  jvm import --all         # Import all found versions\n")

	return nil
}

// performImport 执行实际的导入操作
func performImport(installation *scanner.JavaInstallation) error {
	// 这里重用现有的导入逻辑
	// 创建符号链接或复制文件到 JVM 管理目录

	// 获取 JVM 版本目录
	versionsDir, err := config.GetVersionsDir()
	if err != nil {
		return err
	}

	// 创建版本目录名
	versionDirName := "java-" + installation.Version
	targetPath := filepath.Join(versionsDir, versionDirName)

	// 检查是否已经存在
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("version %s is already imported", installation.Version)
	}

	// 创建符号链接（Windows 上可能需要管理员权限）
	if err := os.Symlink(installation.Path, targetPath); err != nil {
		// 如果符号链接失败，尝试复制（这里简化处理）
		color.Yellow("Symbolic link failed, this version needs manual setup")
		return fmt.Errorf("failed to create symbolic link: %w", err)
	}

	return nil
}

// init 函数初始化 import 命令的标志
func init() {
	// 添加从指定路径导入的标志
	importCmd.Flags().StringVar(&importFromPath, "from", "", "从指定路径扫描 Java 安装")

	// 添加导入所有版本的标志
	importCmd.Flags().BoolVar(&importAll, "all", false, "导入所有找到的 Java 版本")
}
