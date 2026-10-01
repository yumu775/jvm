//go:build windows

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
)

func init() {
	// 双击由安装引导处理，避免 Cobra 显示英文提示后以错误码退出。
	cobra.MousetrapHelpText = ""
}

func executeDesktopLaunch() (bool, error) {
	if len(os.Args) != 1 || !mousetrap.StartedByExplorer() {
		return false, nil
	}
	reader := bufio.NewReader(os.Stdin)
	executable, err := os.Executable()
	if err != nil {
		return true, err
	}
	fmt.Printf("JVM %s — Java 版本管理工具\n\n", Version)
	fmt.Println("这是命令行工具。双击可以配置工具 PATH，之后在 CMD 或 PowerShell 中使用 jvm 命令。")
	fmt.Printf("工具位置：%s\n", executable)
	fmt.Println("配置仅将此目录加入当前用户 PATH，不会安装或切换 Java。")
	fmt.Println("如需 PowerShell 自动激活集成，请运行发行包中的 install-jvm.bat。")
	fmt.Print("\n现在配置？输入 Y 或 是 确认，直接回车取消：")
	answer, _ := reader.ReadString('\n')
	if desktopSetupConfirmed(answer) {
		rootCmd.SetArgs([]string{"setup"})
		err = rootCmd.Execute()
		rootCmd.SetArgs(nil)
		if err != nil {
			fmt.Printf("\n配置失败：%v\n", err)
		} else {
			fmt.Printf("\n配置成功。请保留工具目录 %s，并打开新终端。\n", filepath.Dir(executable))
			fmt.Println("常用命令：jvm --version / jvm list available / jvm install 17 / jvm use 17")
		}
	} else {
		fmt.Println("未修改环境。可以在终端执行 jvm --help 查看帮助。")
	}
	fmt.Print("\n按回车关闭窗口……")
	reader.ReadString('\n')
	return true, err
}

func desktopSetupConfirmed(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "是", "确认":
		return true
	default:
		return false
	}
}
