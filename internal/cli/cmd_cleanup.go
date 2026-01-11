// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newCleanupCmd creates the cleanup command
func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove completed task branches and data",
		Long: `Remove completed task branches and worktrees.

By default, only removes completed tasks. Use --all to remove all tasks.

Example:
  orc cleanup                    # Remove completed task branches
  orc cleanup --all              # Remove all task branches
  orc cleanup --older-than 7d    # Remove branches older than 7 days
  orc cleanup --dry-run          # Show what would be removed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			all, _ := cmd.Flags().GetBool("all")
			olderThan, _ := cmd.Flags().GetString("older-than")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			var toClean []*task.Task
			for _, t := range tasks {
				// Filter by status
				if !all && t.Status != task.StatusCompleted {
					continue
				}

				// Filter by age
				if olderThan != "" {
					duration, err := time.ParseDuration(olderThan)
					if err != nil {
						return fmt.Errorf("invalid duration: %w", err)
					}
					if time.Since(t.CreatedAt) < duration {
						continue
					}
				}

				toClean = append(toClean, t)
			}

			if len(toClean) == 0 {
				fmt.Println("No tasks to clean up")
				return nil
			}

			if dryRun {
				fmt.Println("Would remove the following tasks:")
				for _, t := range toClean {
					fmt.Printf("  %s - %s (%s)\n", t.ID, t.Title, t.Status)
				}
				return nil
			}

			fmt.Printf("Cleaning up %d task(s)...\n", len(toClean))
			for _, t := range toClean {
				// Remove task directory
				taskDir := task.TaskDir(t.ID)
				if err := os.RemoveAll(taskDir); err != nil {
					fmt.Printf("  Warning: Failed to remove %s: %v\n", t.ID, err)
					continue
				}
				fmt.Printf("  Removed %s\n", t.ID)
			}

			return nil
		},
	}
	cmd.Flags().BoolP("all", "a", false, "remove all task branches")
	cmd.Flags().String("older-than", "", "remove tasks older than duration (e.g., 7d)")
	cmd.Flags().Bool("dry-run", false, "show what would be removed")
	return cmd
}
