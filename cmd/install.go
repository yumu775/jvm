package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/download"
	"jvm/internal/install"
	"jvm/internal/version"
)

var (
	installDir string
	forceInstall bool
	sourceFilter string
)

// installCmd 定义了 "jvm install" 命令
// 这个命令用于安装指定版本的 Java
var installCmd = &cobra.Command{
	Use:   "install <version>",
	Short: "安装指定版本的 Java",
	Long: `安装指定版本的 Java。

这个命令会：
1. 从官方源下载指定版本的 Java
2. 验证下载文件的完整性
3. 解压并安装到指定目录
4. 配置必要的文件和权限

示例：
  jvm install 17                    # 安装 Java 17 最新版本
  jvm install latest               # 安装最新版本
  jvm install lts                  # 安装最新 LTS 版本
  jvm install 11.0.19              # 安装特定版本
  jvm install 17 --source corretto # 从 Amazon Corretto 安装
  jvm install 17 --dir /custom/path # 安装到自定义目录
  jvm install 17 --force           # 强制重新安装`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetVersion := args[0]

		// 创建版本管理器实例
		manager, err := version.NewManager()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}

		// 检查版本是否已经安装（除非强制安装）
		if !forceInstall && manager.IsInstalled(targetVersion) {
			color.Yellow("Java version %s is already installed.", targetVersion)
			fmt.Printf("Use 'jvm use %s' to switch to this version.\n", targetVersion)
			fmt.Printf("Use --force to reinstall.\n")
			return nil
		}

		// 确定安装目录
		var versionDir string
		if installDir != "" {
			// 使用自定义安装目录
			versionDir = filepath.Join(installDir, "java-"+targetVersion)
		} else {
			// 使用默认安装目录
			versionsDir, err := config.GetVersionsDir()
			if err != nil {
				return fmt.Errorf("failed to get versions directory: %w", err)
			}
			versionDir = filepath.Join(versionsDir, "java-"+targetVersion)
		}

		// 如果强制安装，先删除现有安装
		if forceInstall {
			if _, err := os.Stat(versionDir); err == nil {
				color.Yellow("Removing existing installation...")
				if err := os.RemoveAll(versionDir); err != nil {
					return fmt.Errorf("failed to remove existing installation: %w", err)
				}
			}
		}

		// 创建下载器和安装器
		downloader := download.NewDownloader()
		installer := install.NewInstaller()

		// 查找版本信息
		color.Blue("Looking up Java version %s...", targetVersion)
		release, err := downloader.FindVersion(targetVersion, sourceFilter)
		if err != nil {
			return fmt.Errorf("failed to find Java version: %w", err)
		}

		color.Green("Found Java %s", release.Version)

		// 获取下载目录
		downloadDir, err := config.GetDownloadsDir()
		if err != nil {
			return fmt.Errorf("failed to get download directory: %w", err)
		}

		// 下载 Java
		archivePath, err := downloader.DownloadJava(release, downloadDir)
		if err != nil {
			return fmt.Errorf("failed to download Java: %w", err)
		}

		// 安装 Java
		if err := installer.InstallJava(archivePath, versionDir, release.Version); err != nil {
			return fmt.Errorf("failed to install Java: %w", err)
		}

		// 规范化安装结构
		if err := installer.NormalizeInstallation(versionDir); err != nil {
			color.Yellow("Warning: failed to normalize installation: %v", err)
		}

		// 验证安装
		if err := installer.ValidateInstallation(versionDir); err != nil {
			return fmt.Errorf("installation validation failed: %w", err)
		}

		color.Green("Successfully installed Java %s", release.Version)
		fmt.Printf("Installation path: %s\n", versionDir)
		fmt.Printf("Use 'jvm use %s' to switch to this version.\n", release.Version)

		// 可选：清理下载文件
		if !cmd.Flag("keep-download").Changed {
			color.Blue("Cleaning up download file...")
			if err := os.Remove(archivePath); err != nil {
				color.Yellow("Warning: failed to remove download file: %v", err)
			}
		}

		return nil
	},
}



// init 函数初始化 install 命令的标志
func init() {
	// 添加自定义安装目录标志
	installCmd.Flags().StringVarP(&installDir, "dir", "d", "", "自定义安装目录")

	// 添加强制安装标志
	installCmd.Flags().BoolVarP(&forceInstall, "force", "f", false, "强制重新安装已存在的版本")

	// 添加保留下载文件标志
	installCmd.Flags().Bool("keep-download", false, "保留下载的安装包文件")

	// 添加源过滤标志
	installCmd.Flags().StringVarP(&sourceFilter, "source", "s", "", "指定下载源 (adoptium, corretto, zulu, oracle, graalvm)")
}
