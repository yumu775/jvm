package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"jvm/internal/version"
)

var currentCmd = &cobra.Command{
	Use: "current", Short: "显示默认选择与当前终端实际 Java",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager, err := version.NewManager()
		if err != nil {
			return err
		}
		selected, err := manager.GetCurrent()
		if err != nil {
			fmt.Printf("Managed default: unavailable (%v)\n", err)
		} else {
			home, err := manager.GetVersionPath(selected)
			if err != nil {
				return err
			}
			fmt.Printf("Managed default: %s\nManaged Java home: %s\n", selected, home)
		}
		fmt.Printf("Inherited JAVA_HOME: %s\n", os.Getenv("JAVA_HOME"))
		if actual, err := exec.LookPath("java"); err == nil {
			fmt.Printf("Resolved executable: %s\n", actual)
		}
		return showActiveJavaInfo()
	},
}
