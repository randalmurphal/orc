// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newResolveCmd creates the resolve command
func newResolveCmd() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "resolve <task-id>",
		Short: "Mark failed task as resolved without re-running",
		Long: `Mark a failed task as resolved/acknowledged without re-running it.

This is useful when:
  - The issue was fixed manually outside of orc
  - The failure is no longer relevant (e.g., requirements changed)
  - You want to acknowledge and close out a failed task

The task will be marked as completed with metadata indicating it was resolved
rather than executed to completion. This preserves the failure context in the
execution history.

Unlike 'reset' which clears progress and allows retry, 'resolve' closes the
task without clearing its execution state.

Examples:
  orc resolve TASK-001                          # Mark as resolved
  orc resolve TASK-001 -m "Fixed manually"      # With resolution message
  orc resolve TASK-001 --force                  # Skip confirmation`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer backend.Close()

			id := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Load task to verify it exists and check status
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Only allow resolving failed tasks
			if t.Status != task.StatusFailed {
				return fmt.Errorf("task %s is %s, not failed; resolve is only for failed tasks", id, t.Status)
			}

			// Confirmation prompt
			if !force && !quiet {
				fmt.Printf("⚠️  Resolve task %s as completed?\n", id)
				fmt.Println("   The task will be marked as completed (resolved).")
				fmt.Println("   Execution state will be preserved for reference.")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Update task status to completed
			t.Status = task.StatusCompleted
			now := time.Now()
			t.CompletedAt = &now

			// Add resolution metadata
			if t.Metadata == nil {
				t.Metadata = make(map[string]string)
			}
			t.Metadata["resolved"] = "true"
			t.Metadata["resolved_at"] = now.Format(time.RFC3339)
			if message != "" {
				t.Metadata["resolution_message"] = message
			}

			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			if plain {
				fmt.Printf("Task %s resolved\n", id)
			} else {
				fmt.Printf("✓ Task %s marked as resolved\n", id)
			}
			if message != "" {
				fmt.Printf("   Message: %s\n", message)
			}
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation")
	cmd.Flags().StringVarP(&message, "message", "m", "", "resolution message explaining why task was resolved")
	return cmd
}
