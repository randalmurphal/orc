// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newGoodbyeCmd creates the goodbye command
func newGoodbyeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "goodbye",
		Short: "Say goodbye",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Goodbye!")
		},
	}
}
