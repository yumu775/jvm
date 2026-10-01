package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/project"
	"jvm/internal/version"
)

var (
	autoSwitch  bool
	forceCreate bool
)

// projectCmd 定义了 "jvm project" 命令
// 这个命令用于管理项目级的 Java 版本配置
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "管理项目级的 Java 版本配置",
	Long: `管理项目级的 Java 版本配置。

支持 .jvmrc, .java-version, .sdkmanrc 等配置文件格式。
配置文件会在当前目录及其父目录中查找。

子命令：
  init <version>    创建项目配置文件
  set <version>     设置项目 Java 版本
  get               显示项目配置信息
  use               切换到项目配置的 Java 版本`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 如果没有子命令，显示项目信息
		return showProjectInfo()
	},
}

// projectInitCmd 创建项目配置文件
var projectInitCmd = &cobra.Command{
	Use:   "init <version>",
	Short: "创建项目配置文件",
	Long: `在当前目录创建 .jvmrc 配置文件。

示例：
  jvm project init 17        # 创建配置文件，指定 Java 17
  jvm project init 11.0.19   # 创建配置文件，指定具体版本`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		javaVersion := args[0]

		projectManager := project.NewManager()

		// 检查版本是否已安装
		versionManager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}

		if !versionManager.IsInstalled(javaVersion) {
			if autoSwitch {
				return fmt.Errorf("Java %s is not installed; install it before using --switch", javaVersion)
			}
			color.Yellow("Warning: Java version %s is not installed", javaVersion)
			fmt.Printf("Use 'jvm install %s' to install this version.\n", javaVersion)
		}

		// 创建项目配置
		if err := projectManager.CreateProjectConfig(javaVersion); err != nil {
			if forceCreate && errors.Is(err, os.ErrExist) {
				// 强制创建，先更新现有配置
				if updateErr := projectManager.UpdateProjectConfig(javaVersion); updateErr != nil {
					return fmt.Errorf("failed to update project config: %w", updateErr)
				}
			} else {
				return fmt.Errorf("failed to create project config: %w", err)
			}
		}

		color.Green("Project configured to use Java %s", javaVersion)

		// 如果启用自动切换，立即切换版本
		if autoSwitch && versionManager.IsInstalled(javaVersion) {
			return switchToProjectVersion(javaVersion)
		}

		return nil
	},
}

// projectSetCmd 设置项目 Java 版本
var projectSetCmd = &cobra.Command{
	Use:   "set <version>",
	Short: "设置项目 Java 版本",
	Long: `更新现有的项目配置文件中的 Java 版本。

示例：
  jvm project set 17         # 更新项目配置为 Java 17
  jvm project set 11.0.19    # 更新为具体版本`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		javaVersion := args[0]

		projectManager := project.NewManager()

		// 更新项目配置
		if err := projectManager.UpdateProjectConfig(javaVersion); err != nil {
			return fmt.Errorf("failed to update project config: %w", err)
		}

		color.Green("Project updated to use Java %s", javaVersion)

		// 如果启用自动切换，立即切换版本
		if autoSwitch {
			versionManager, err := version.NewManager()
			if err != nil {
				return fmt.Errorf("failed to initialize version manager: %w", err)
			}

			if versionManager.IsInstalled(javaVersion) {
				return switchToProjectVersion(javaVersion)
			} else {
				return fmt.Errorf("project config saved, but Java version %s is not installed; cannot switch", javaVersion)
			}
		}

		return nil
	},
}

// projectGetCmd 显示项目配置信息
var projectGetCmd = &cobra.Command{
	Use:   "get",
	Short: "显示项目配置信息",
	Long: `显示当前项目的 Java 版本配置信息。

会在当前目录及其父目录中查找配置文件。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showProjectInfo()
	},
}

// projectUseCmd 切换到项目配置的 Java 版本
var projectUseCmd = &cobra.Command{
	Use:   "use",
	Short: "切换到项目配置的 Java 版本",
	Long: `自动切换到项目配置文件中指定的 Java 版本。

会在当前目录及其父目录中查找配置文件，
然后切换到配置文件中指定的 Java 版本。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectManager := project.NewManager()

		// 查找项目配置
		config, err := projectManager.AutoSwitchToProjectVersion()
		if err != nil {
			return fmt.Errorf("failed to find project configuration: %w", err)
		}

		// 切换到项目版本
		return switchToProjectVersion(config.JavaVersion)
	},
}

// showProjectInfo 显示项目信息
func showProjectInfo() error {
	projectManager := project.NewManager()

	color.Blue("=== Project Configuration ===")
	fmt.Println()

	// 获取项目信息
	info, err := projectManager.GetProjectInfo()
	if err != nil {
		return fmt.Errorf("failed to get project info: %w", err)
	}

	// 显示基本信息
	color.Green("Current Directory:")
	fmt.Printf("  %s\n", info["current_directory"])
	fmt.Println()

	color.Green("Project Configuration:")
	if info["project_config"] == "Not found" {
		color.Yellow("  No project configuration found")
		fmt.Println("  Use 'jvm project init <version>' to create one")
	} else {
		fmt.Printf("  Config file: %s\n", info["project_config"])
		fmt.Printf("  Java version: %s\n", info["java_version"])
		fmt.Printf("  Location: %s\n", info["config_location"])

		// 检查版本是否已安装
		versionManager, err := version.NewManager()
		if err == nil {
			if versionManager.IsInstalled(info["java_version"]) {
				color.Green("  Status: Java %s is installed", info["java_version"])
			} else {
				color.Yellow("  Status: Java %s is not installed", info["java_version"])
				fmt.Printf("  Use 'jvm install %s' to install\n", info["java_version"])
			}
		}
	}

	return nil
}

// switchToProjectVersion 切换到指定的项目版本
func switchToProjectVersion(javaVersion string) error {
	return selectJavaVersion(javaVersion)
}

// init 函数初始化 project 命令和子命令
func init() {
	// 添加子命令
	projectCmd.AddCommand(projectInitCmd)
	projectCmd.AddCommand(projectSetCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectUseCmd)

	// 添加标志
	projectInitCmd.Flags().BoolVar(&autoSwitch, "switch", false, "创建配置后自动切换到指定版本")
	projectInitCmd.Flags().BoolVar(&forceCreate, "force", false, "强制创建，覆盖现有配置")

	projectSetCmd.Flags().BoolVar(&autoSwitch, "switch", false, "设置配置后自动切换到指定版本")
}
