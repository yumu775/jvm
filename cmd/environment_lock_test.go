package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestEnvironmentWrapperSkipsReadOnlyActivationAndPreview(t *testing.T) {
	for _, flag := range []string{"temp", "dry-run"} {
		t.Run(flag, func(t *testing.T) {
			ran := false
			command := &cobra.Command{Use: "fixture", RunE: func(*cobra.Command, []string) error { ran = true; return nil }}
			command.Flags().Bool(flag, true, "")
			wrapEnvironmentCommand(command, func() (func() error, error) { t.Fatal("read-only command acquired mutation lock"); return nil, nil })
			if err := command.RunE(command, nil); err != nil || !ran {
				t.Fatalf("read-only operation failed: %v", err)
			}
		})
	}
}
