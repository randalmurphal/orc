// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// getBackend is imported from commands.go

// newPauseCmd creates the pause command
func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause task execution (can resume later)",
		Long: `Pause a running task, saving its current state.

The task can be resumed later with 'orc resume'. All progress is preserved.

Use 'orc stop' instead if you want to abort the task permanently.

Examples:
  orc pause TASK-001
  orc resume TASK-001  # Continue later`,
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

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusRunning {
				return fmt.Errorf("task is not running (status: %s)", t.Status)
			}

			t.Status = task.StatusPaused
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("⏸️  Task %s paused\n", id)
			fmt.Printf("   Resume with: orc resume %s\n", id)
			return nil
		},
	}
}

// newStopCmd creates the stop command
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <task-id>",
		Short: "Stop task execution permanently (marks as failed)",
		Long: `Stop a task and mark it as failed. This is permanent.

Unlike 'pause', a stopped task cannot be resumed. Use this when you want
to abandon a task entirely.

Use 'orc pause' instead if you want to continue the task later.

Examples:
  orc stop TASK-001           # Prompts for confirmation
  orc stop TASK-001 --force   # Skip confirmation`,
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

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status == task.StatusCompleted {
				return fmt.Errorf("task is already completed")
			}

			if t.Status == task.StatusFailed {
				fmt.Printf("Task %s is already stopped/failed\n", id)
				return nil
			}

			if !force && !quiet {
				fmt.Printf("⚠️  Stop task %s?\n", id)
				fmt.Println("   This marks the task as failed and cannot be resumed.")
				fmt.Println("   Use 'orc pause' instead to preserve progress.")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted. Task still running.")
					fmt.Printf("To pause instead: orc pause %s\n", id)
					return nil
				}
			}

			t.Status = task.StatusFailed
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("🛑 Task %s stopped (marked as failed)\n", id)
			fmt.Println("\nTo start fresh: orc rewind " + id + " --to <phase>")
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")
	return cmd
}
