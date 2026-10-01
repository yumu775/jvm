package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"jvm/internal/config"
	"jvm/internal/download"
	"jvm/internal/install"
	"jvm/internal/version"
)

var installCmd = &cobra.Command{
	Use:   "install <version>",
	Short: "下载、验证并安装 JDK",
	Long: `安装指定主版本、完整版本或 latest/lts 别名。
使用真实版本号登记安装；--dir 指定版本仓库，该位置会纳入管理。
强制重装会在新 JDK 验证成功后替换旧安装，失败时保留旧版本。`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	requested := args[0]
	if err := version.ValidateVersion(requested); err != nil {
		return err
	}
	source, _ := cmd.Flags().GetString("source")
	customDir, _ := cmd.Flags().GetString("dir")
	force, _ := cmd.Flags().GetBool("force")
	keepDownload, _ := cmd.Flags().GetBool("keep-download")
	d := download.NewDownloader()
	release, err := d.FindVersion(requested, source)
	if err != nil {
		return fmt.Errorf("resolve Java %s: %w", requested, err)
	}
	identity := release.Version + "-" + release.Source
	if err := version.ValidateVersion(identity); err != nil {
		return fmt.Errorf("invalid release identity: %w", err)
	}
	manager, err := version.NewManager()
	if err != nil {
		return err
	}
	root := customDir
	if root == "" {
		root, err = config.GetVersionsDir()
		if err != nil {
			return err
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	target := filepath.Join(root, "java-"+identity)
	// 注册位置是已安装版本的唯一地址，重装不能默默改到另一个目录。
	if record, recordErr := manager.GetRecord(identity); recordErr == nil {
		if !record.Managed {
			return fmt.Errorf("Java %s is imported; unregister it before installing a managed copy", identity)
		}
		if customDir != "" && !sameInstallPath(record.Path, target) {
			return fmt.Errorf("Java %s is already registered at %s; changing --dir does not move installations", identity, record.Path)
		}
		target = record.Path
		if !force {
			if !manager.IsInstalled(identity) {
				return fmt.Errorf("Java %s is registered at %s but missing or invalid; use --force to repair", identity, target)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Java %s is already installed at %s\n", identity, target)
			return nil
		}
	} else if _, statErr := os.Lstat(target); statErr == nil {
		return fmt.Errorf("destination exists but is not a registered JDK: %s; refusing to overwrite", target)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	// 相同目标的安装互斥；遗留锁保留供用户检查，不自动抢占。
	lockPath := target + ".install-lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("cannot acquire installation lock %s: %w", lockPath, err)
	}
	lock.Close()
	defer os.Remove(lockPath)
	cache, err := config.GetDownloadsDir()
	if err != nil {
		return err
	}
	archive, err := d.DownloadJava(release, cache)
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".jvm-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	installer := install.NewInstaller()
	if err := installer.InstallJava(archive, stage, identity); err != nil {
		return err
	}
	if err := installer.NormalizeInstallation(stage); err != nil {
		return err
	}
	if err := installer.ValidateRelease(stage, identity); err != nil {
		return err
	}
	if err := commitInstallation(stage, target, func() error {
		return manager.RegisterRelease(identity, target, release.Version, release.Source, release.Vendor)
	}); err != nil {
		return err
	}
	if !keepDownload {
		if err := os.Remove(archive); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: cannot remove download %s: %v\n", archive, err)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Installed Java %s at %s\nUse 'jvm use %s' to select it.\n", identity, target, identity)
	return nil
}

func sameInstallPath(a, b string) bool {
	a, errA := filepath.Abs(a)
	b, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// commitInstallation 在同一文件系统内替换安装，登记失败时恢复旧目录。
func commitInstallation(stage, target string, register func() error) error {
	backup := ""
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace non-directory or linked destination: %s", target)
		}
		canonicalTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return err
		}
		if err := version.ValidateRemovalPath(canonicalTarget); err != nil {
			return err
		}
		backup, err = os.MkdirTemp(filepath.Dir(target), ".jvm-backup-")
		if err != nil {
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("cannot replace JDK (close programs using it): %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	restore := func(cause error) error {
		if backup != "" {
			if err := os.Rename(backup, target); err != nil {
				return errors.Join(cause, fmt.Errorf("restore failed; previous JDK remains at %s: %w", backup, err))
			}
		}
		return cause
	}
	if err := os.Rename(stage, target); err != nil {
		return restore(err)
	}
	if err := register(); err != nil {
		// 移回暂存目录，避免清理失败破坏旧安装的恢复。
		if moveErr := os.Rename(target, stage); moveErr != nil {
			return errors.Join(err, fmt.Errorf("rollback failed; new JDK at %s, previous JDK at %s: %w", target, backup, moveErr))
		}
		return restore(err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("installation registered successfully, but old backup could not be removed at %s: %w", backup, err)
		}
	}
	return nil
}

func init() {
	installCmd.Flags().StringP("dir", "d", "", "安装仓库目录（支持其他磁盘）")
	installCmd.Flags().BoolP("force", "f", false, "验证新安装后替换已有版本")
	installCmd.Flags().Bool("keep-download", false, "保留已验证的安装包")
	installCmd.Flags().StringP("source", "s", "", "指定下载源（必须支持自动下载且已启用）")
}
