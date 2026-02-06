package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	addCmd(newIndexCmd(), groupAdvanced)
}

func newIndexCmd() *cobra.Command {
	var (
		incremental bool
		status      bool
	)

	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Index project code for knowledge layer",
		Long: `Index project source code into the knowledge layer (graph + vector stores).

Walks the project tree, parses source files (Go, Python, JavaScript, TypeScript),
extracts symbols and relationships, detects patterns, and stores embeddings.

Use --incremental to only re-index files that have changed since the last run.
Use --status to show indexing status without running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if status {
				return fmt.Errorf("index status: not yet implemented")
			}
			return fmt.Errorf("index: not yet implemented (incremental=%v)", incremental)
		},
	}

	cmd.Flags().BoolVar(&incremental, "incremental", false, "only re-index changed files")
	cmd.Flags().BoolVar(&status, "status", false, "show indexing status")

	return cmd
}
