// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newHelloCmd creates the hello command
func newHelloCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hello",
		Short: "Output hello",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hello")
		},
	}
}
