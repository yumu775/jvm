package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// installEnvironmentCommandLocks 包住完整业务操作，包括环境写入后的配置提交。
func installEnvironmentCommandLocks() {
	for _, command := range []*cobra.Command{useCmd, setEnvCmd, setupCmd, uninstallCmd, uninstallCleanCmd, projectInitCmd, projectSetCmd, projectUseCmd, configSetCmd} {
		wrapEnvironmentCommand(command, acquireEnvironmentCommandLock)
	}
}

func wrapEnvironmentCommand(command *cobra.Command, acquire func() (func() error, error)) {
	run := command.RunE
	command.RunE = func(cmd *cobra.Command, args []string) (result error) {
		if temporary, err := cmd.Flags().GetBool("temp"); err == nil && temporary {
			return run(cmd, args)
		}
		if preview, err := cmd.Flags().GetBool("dry-run"); err == nil && preview {
			return run(cmd, args)
		}
		release, err := acquire()
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, release()) }()
		return run(cmd, args)
	}
}
