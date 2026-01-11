// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newDiffCmd creates the diff command
func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <task-id>",
		Short: "Show task changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Showing diff for task: %s\n", args[0])
			// TODO: Implement git diff
			return nil
		},
	}
}
