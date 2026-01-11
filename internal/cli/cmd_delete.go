// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newDeleteCmd creates the delete command
func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task",
		Long: `Delete a task and its associated files.

Running tasks cannot be deleted - pause them first.

Example:
  orc delete TASK-001           # Delete task TASK-001
  orc delete TASK-001 --force   # Delete even if paused/running`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			taskID := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Load task to verify it exists and check status
			t, err := task.Load(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if task is running
			if t.Status == task.StatusRunning && !force {
				return fmt.Errorf("task %s is running - use --force to delete anyway", taskID)
			}

			// Confirm deletion
			if !quiet {
				fmt.Printf("Deleting task %s (%s)...\n", t.ID, t.Title)
			}

			// Remove task directory
			taskDir := task.TaskDir(taskID)
			if err := os.RemoveAll(taskDir); err != nil {
				return fmt.Errorf("remove task directory: %w", err)
			}

			if !quiet {
				fmt.Printf("Deleted task %s\n", taskID)
			}

			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "force delete even if running")
	return cmd
}
