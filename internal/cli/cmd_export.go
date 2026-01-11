// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newExportCmd creates the export command
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <task-id>",
		Short: "Export task context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Exporting task: %s\n", args[0])
			// TODO: Implement export
			return nil
		},
	}
}

// newImportCmd creates the import command
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import context into task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Importing from: %s\n", args[0])
			// TODO: Implement import
			return nil
		},
	}
}
