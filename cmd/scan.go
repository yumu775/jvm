package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/scanner"
)

var (
	scanImportAll bool
	showDetails   bool
	scanPaths     []string
	customPath    string
)

// scanCmd 定义了 "jvm scan" 命令
// 这个命令用于扫描系统中已安装的 Java 版本
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "扫描系统中已安装的 Java 版本",
	Long: `扫描系统中已安装的 Java 版本。

这个命令会：
1. 扫描常见的 Java 安装路径
2. 检查环境变量中的 Java 安装
3. 分析每个安装的版本信息
4. 可选择导入到 jvm 管理

示例：
  jvm scan                           # 扫描并显示已安装的 Java 版本
  jvm scan --details                # 显示详细信息
  jvm scan --import-all             # 扫描并导入所有找到的版本
  jvm scan --path E:\\Java          # 扫描指定目录
  jvm scan --path E:\\Java --path D:\\JDK  # 扫描多个目录`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建扫描器
		javaScanner := scanner.NewScanner()
		
		// 扫描系统中的 Java 安装
		var installations []scanner.JavaInstallation
		var err error

		if len(scanPaths) > 0 {
			// 扫描指定路径
			color.Blue("Scanning specified paths for Java installations...")
			installations, err = javaScanner.ScanCustomPaths(scanPaths)
		} else {
			// 扫描系统默认路径
			color.Blue("Scanning for Java installations...")
			installations, err = javaScanner.ScanSystemJava()
		}

		if err != nil {
			return fmt.Errorf("failed to scan Java installations: %w", err)
		}
		
		if len(installations) == 0 {
			color.Yellow("No Java installations found on this system.")
			fmt.Println("You can install Java using 'jvm install <version>'")
			return nil
		}
		
		// 显示找到的安装
		color.Green("Found %d Java installation(s):", len(installations))
		fmt.Println()
		
		for i, installation := range installations {
			fmt.Printf("%d. Java %s\n", i+1, installation.Version)
			if showDetails {
				fmt.Printf("   Path: %s\n", installation.Path)
				fmt.Printf("   Vendor: %s\n", installation.Vendor)
				fmt.Printf("   Type: %s\n", installation.Type)
				fmt.Printf("   Architecture: %s\n", installation.Architecture)
				fmt.Printf("   Source: %s\n", installation.Source)
			} else {
				fmt.Printf("   Path: %s\n", installation.Path)
				fmt.Printf("   Vendor: %s (%s)\n", installation.Vendor, installation.Type)
			}
			fmt.Println()
		}
		
		// 如果用户选择导入所有版本
		if scanImportAll {
			return importInstallations(javaScanner, installations)
		}
		
		// 提示用户可以导入版本
		color.Cyan("To import a specific version, use:")
		color.Cyan("  jvm import <version>")
		color.Cyan("To import all versions, use:")
		color.Cyan("  jvm scan --import-all")
		
		return nil
	},
}

// importInstallations 导入 Java 安装到 jvm 管理
func importInstallations(javaScanner *scanner.Scanner, installations []scanner.JavaInstallation) error {
	versionsDir, err := config.GetVersionsDir()
	if err != nil {
		return fmt.Errorf("failed to get versions directory: %w", err)
	}
	
	var imported, skipped int
	
	for _, installation := range installations {
		color.Blue("Importing Java %s...", installation.Version)
		
		if err := javaScanner.ImportInstallation(installation, versionsDir); err != nil {
			color.Red("Failed to import Java %s: %v", installation.Version, err)
			skipped++
		} else {
			imported++
		}
	}
	
	fmt.Println()
	color.Green("Import completed: %d imported, %d skipped", imported, skipped)
	
	if imported > 0 {
		fmt.Println("Use 'jvm list' to see all managed versions")
		fmt.Println("Use 'jvm use <version>' to switch to a version")
	}
	
	return nil
}

// init 函数初始化 scan 命令的标志
func init() {
	// 添加导入所有版本标志
	scanCmd.Flags().BoolVar(&scanImportAll, "import-all", false, "自动导入所有找到的 Java 版本")
	
	// 添加显示详细信息标志
	scanCmd.Flags().BoolVar(&showDetails, "details", false, "显示详细的版本信息")

	// 添加自定义扫描路径标志
	scanCmd.Flags().StringSliceVar(&scanPaths, "path", []string{}, "指定要扫描的目录路径（可多次使用）")
}
