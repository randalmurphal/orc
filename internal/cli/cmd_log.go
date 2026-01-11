// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
)

// newLogCmd creates the log command
func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log <task-id>",
		Short: "Show task transcripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			transcriptsDir := fmt.Sprintf(".orc/tasks/%s/transcripts", id)
			entries, err := os.ReadDir(transcriptsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No transcripts found for this task")
					return nil
				}
				return fmt.Errorf("read transcripts: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No transcripts found for this task")
				return nil
			}

			fmt.Printf("Transcripts for %s:\n", id)
			for _, entry := range entries {
				fmt.Printf("  %s/%s\n", transcriptsDir, entry.Name())
			}

			return nil
		},
	}
}
