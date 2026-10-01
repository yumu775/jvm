package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"jvm/internal/env"
	"jvm/internal/version"
)

var tempOnly, persistent bool
var useShell string
var useCmd = &cobra.Command{
	Use: "use <version>", Short: "选择默认 Java 版本；已有终端需执行激活脚本",
	Long: `选择默认 Java 并持久化环境。已有终端和 IDE 不会被子进程直接修改。
PowerShell 激活：jvm env --shell powershell | Out-String | Invoke-Expression
Bash/Zsh 激活：eval "$(jvm env --shell bash)"
临时选择：jvm use 17 --temp --shell powershell | Out-String | Invoke-Expression
CMD：jvm env --shell cmd > "%TEMP%\jvm-activate.cmd" && call "%TEMP%\jvm-activate.cmd"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if useShell != "" && !tempOnly {
			return fmt.Errorf("--shell requires --temp; use jvm env --shell <shell> for activation")
		}
		if tempOnly {
			return printVersionActivation(cmd, args[0], useShell)
		}
		return selectJavaVersion(args[0])
	},
}

// shellInitCmd 输出由用户显式加载的 shell 集成，不修改启动脚本。
var shellInitCmd = &cobra.Command{
	Use: "init powershell", Short: "输出 PowerShell 集成函数，使普通 use 自动应用到当前终端",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != "powershell" {
			return fmt.Errorf("automatic integration currently supports powershell; other shells can execute jvm env --shell output")
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), env.PowerShellIntegration(executable))
		return err
	},
}

func printVersionActivation(cmd *cobra.Command, target, shell string) error {
	if shell == "" {
		return fmt.Errorf("--temp requires --shell; execute the emitted script in your shell")
	}
	manager, err := version.NewManager()
	if err != nil {
		return err
	}
	home, err := manager.GetVersionPath(target)
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

func selectJavaVersion(target string) error {
	manager, err := version.NewManager()
	if err != nil {
		return err
	}
	home, err := manager.GetVersionPath(target)
	if err != nil {
		return err
	}
	if err := env.NewManager().SetJavaEnvironment(home, false); err != nil {
		return err
	}
	if err := manager.SetCurrent(target); err != nil {
		return fmt.Errorf("environment was saved, but selected-version metadata could not be saved: %w", err)
	}
	fmt.Printf("Default Java: %s\nJAVA_HOME: %s\n", target, home)
	printActivationInstructions()
	return nil
}

func printActivationInstructions() {
	fmt.Println("Persistent environment saved. Existing terminals and IDEs keep their inherited environment.")
	fmt.Println("PowerShell: jvm env --shell powershell | Out-String | Invoke-Expression")
	cmdHint := `CMD: jvm env --shell cmd > "%TEMP%\jvm-activate.cmd" && call "%TEMP%\jvm-activate.cmd"`
	fmt.Println(cmdHint)
	fmt.Println(`Bash/Zsh: eval "$(jvm env --shell bash)"`)
	fmt.Println("Restart the IDE from an activated terminal, or explicitly select JAVA_HOME in its Gradle JDK settings.")
	fmt.Println("A Java shim in Windows system PATH can still override user PATH; jvm env diagnoses this, and shell activation prepends the selected JDK.")
}

func init() {
	useCmd.Flags().BoolVarP(&tempOnly, "temp", "t", false, "输出临时激活脚本，不保存默认版本")
	useCmd.Flags().BoolVarP(&persistent, "persistent", "p", false, "持久化默认版本（默认行为）")
	useCmd.Flags().StringVar(&useShell, "shell", "", "临时激活脚本格式")
	useCmd.MarkFlagsMutuallyExclusive("temp", "persistent")
}
