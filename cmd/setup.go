package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"jvm/internal/env"
)

var setupForce, setupUninstall, setupSystemWide bool
var setupCustomPath, setupPowerShellProfile string

const shellIntegrationMarker = "JVM Shell Integration"

var setupCmd = &cobra.Command{
	Use: "setup", Short: "配置当前用户的 jvm 工具 PATH",
	Long: "配置当前用户的工具 PATH；--path 复制工具到指定目录。Java 存储目录由 config 单独管理。",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		options := setupOptions{force: setupForce, uninstall: setupUninstall, systemWide: setupSystemWide, customPath: setupCustomPath, profile: setupPowerShellProfile, profileSet: cmd.Flags().Changed("powershell-profile")}
		dir, err := performSetup(options, executable, runtime.GOOS, env.NewManager().ConfigureToolPath)
		if err != nil {
			return err
		}
		if options.uninstall {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed tool directory from persistent PATH: %s\n", dir)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Configured tool directory in persistent PATH: %s\n", dir)
		}
		if options.profile != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Updated PowerShell integration in: %s\n", options.profile)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Restart the terminal from a refreshed desktop environment. Existing terminals retain their PATH.")
		return nil
	},
}

type setupOptions struct {
	force, uninstall, systemWide, profileSet bool
	customPath, profile                      string
}

// performSetup 在复制和修改 PATH 前校验配置文件，便于隔离测试所有副作用。
func performSetup(options setupOptions, executable, platform string, configurePath func(string, bool) error) (string, error) {
	if options.systemWide {
		return "", fmt.Errorf("system-wide setup is not supported; use user-level setup (no elevation required)")
	}
	if options.uninstall && options.force {
		return "", fmt.Errorf("--force cannot be combined with --uninstall")
	}
	if options.profileSet || options.profile != "" {
		if platform != "windows" {
			return "", fmt.Errorf("--powershell-profile is supported only on Windows")
		}
		if options.profile == "" || !filepath.IsAbs(options.profile) || strings.ContainsAny(options.profile, "\r\n\x00") {
			return "", fmt.Errorf("--powershell-profile requires an absolute file path")
		}
		if err := env.ValidateProfileBlock(options.profile, shellIntegrationMarker); err != nil {
			return "", err
		}
	}
	dir := filepath.Dir(executable)
	target := executable
	if options.customPath != "" {
		var err error
		dir, err = filepath.Abs(options.customPath)
		if err != nil {
			return "", err
		}
		name := "jvm"
		if platform == "windows" {
			name += ".exe"
		}
		target = filepath.Join(dir, name)
	}
	separator := ":"
	if platform == "windows" {
		separator = ";"
	}
	if strings.ContainsAny(dir, separator+"\r\n\x00") {
		return "", fmt.Errorf("tool directory contains characters unsupported in PATH")
	}
	if options.profile != "" {
		same := filepath.Clean(options.profile) == filepath.Clean(target)
		if platform == "windows" {
			same = strings.EqualFold(filepath.Clean(options.profile), filepath.Clean(target))
		}
		if same {
			return "", fmt.Errorf("PowerShell profile must not be the executable destination")
		}
	}
	if !options.uninstall && options.customPath != "" {
		if err := copyExecutable(executable, target, options.force); err != nil {
			return "", err
		}
	}
	if err := configurePath(dir, options.uninstall); err != nil {
		return "", err
	}
	if options.profile != "" {
		body := ""
		if !options.uninstall {
			body = "& '" + strings.ReplaceAll(target, "'", "''") + "' init powershell | Out-String | Invoke-Expression"
		}
		if err := env.WriteProfileBlock(options.profile, shellIntegrationMarker, body); err != nil {
			return "", fmt.Errorf("tool PATH updated, but PowerShell integration failed: %w", err)
		}
	}
	return dir, nil
}

func copyExecutable(src, dst string, overwrite bool) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source executable is not a regular file: %s", src)
	}
	if existing, err := os.Lstat(dst); err == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace linked or non-regular executable destination: %s", dst)
		}
		if os.SameFile(info, existing) {
			return nil
		}
		if !overwrite {
			return fmt.Errorf("destination exists: %s (use --force to replace)", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dst), ".jvm-install-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(info.Mode()); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, dst)
}

func init() {
	setupCmd.Flags().BoolVarP(&setupForce, "force", "f", false, "覆盖目标工具文件")
	setupCmd.Flags().BoolVar(&setupUninstall, "uninstall", false, "从持久 PATH 移除工具目录")
	setupCmd.Flags().BoolVar(&setupSystemWide, "system-wide", false, "系统级配置（暂不支持）")
	setupCmd.Flags().StringVar(&setupPowerShellProfile, "powershell-profile", "", "PowerShell 配置文件绝对路径（仅 Windows）")
	setupCmd.Flags().StringVar(&setupCustomPath, "path", "", "工具安装目录（不是 Java 存储目录）")
}
