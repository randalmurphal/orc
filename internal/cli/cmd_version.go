// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show orc version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("orc version 0.1.0-dev")
		},
	}
}
