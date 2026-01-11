// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show orc status",
		Long: `Show current orc status including:
  * Active tasks and their phases
  * Pending approvals
  * Recent completions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			// Count by status
			var running, paused, blocked, completed int
			for _, t := range tasks {
				switch t.Status {
				case task.StatusRunning:
					running++
				case task.StatusPaused:
					paused++
				case task.StatusBlocked:
					blocked++
				case task.StatusCompleted:
					completed++
				}
			}

			fmt.Println("orc status")
			fmt.Println("----------")
			fmt.Printf("Running:   %d\n", running)
			fmt.Printf("Paused:    %d\n", paused)
			fmt.Printf("Blocked:   %d\n", blocked)
			fmt.Printf("Completed: %d\n", completed)
			fmt.Printf("Total:     %d\n", len(tasks))

			// Show running/blocked tasks
			if running > 0 || blocked > 0 {
				fmt.Println("\nActive tasks:")
				for _, t := range tasks {
					if t.Status == task.StatusRunning || t.Status == task.StatusBlocked {
						fmt.Printf("  %s - %s [%s] %s\n", statusIcon(t.Status), t.ID, t.CurrentPhase, t.Title)
					}
				}
			}

			return nil
		},
	}
}
