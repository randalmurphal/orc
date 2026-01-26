// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newResetCmd creates the reset command
func newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset <task-id>",
		Short: "Reset task to initial state for retry",
		Long: `Reset a task to its initial state, clearing all execution progress.

This allows you to start the task fresh from the beginning. All phase progress,
execution state, and error information is cleared.

Unlike 'rewind', which goes back to a specific checkpoint, reset clears everything
and starts from scratch.

Use cases:
  - Retry a failed task from the beginning
  - Clear a blocked task and try again
  - Restart a paused task from scratch

Examples:
  orc reset TASK-001           # Reset with confirmation
  orc reset TASK-001 --force   # Skip confirmation (for scripts/automation)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Load task to verify it exists and check status
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Don't reset running tasks unless forced
			if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING && !force {
				return fmt.Errorf("task is currently running; use --force to reset anyway or 'orc stop %s' first", id)
			}

			// Don't reset already-planned tasks (nothing to reset)
			if t.Status == orcv1.TaskStatus_TASK_STATUS_PLANNED || t.Status == orcv1.TaskStatus_TASK_STATUS_CREATED {
				fmt.Printf("Task %s is already in %s state, nothing to reset\n", id, t.Status)
				return nil
			}

			// Confirmation prompt
			if !force && !quiet {
				fmt.Printf("⚠️  Reset task %s?\n", id)
				fmt.Println("   All execution progress will be cleared.")
				fmt.Println("   The task will return to 'planned' status.")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				_, _ = fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Reset execution state (task.Execution contains all execution-related state)
			task.EnsureExecutionProto(t)
			task.ResetExecutionStateProto(t.Execution)
			task.SetCurrentPhaseProto(t, "")

			// Update task status
			t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("🔄 Task %s reset to initial state\n", id)
			fmt.Printf("   Run: orc run %s to start fresh\n", id)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation and safety checks")
	return cmd
}
