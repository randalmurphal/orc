// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newPauseCmd creates the pause command
func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusRunning {
				return fmt.Errorf("task is not running (status: %s)", t.Status)
			}

			t.Status = task.StatusPaused
			if err := t.Save(); err != nil {
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
	return &cobra.Command{
		Use:   "stop <task-id>",
		Short: "Stop task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status == task.StatusCompleted {
				return fmt.Errorf("task is already completed")
			}

			t.Status = task.StatusFailed
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("🛑 Task %s stopped\n", id)
			return nil
		},
	}
}
