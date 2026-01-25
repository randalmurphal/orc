// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newSkipCmd creates the skip command
func newSkipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skip <task-id> --phase <phase>",
		Short: "Skip a phase",
		Long: `Skip a phase without executing it.

Creates an audit entry and advances to the next phase.
Use when you know a phase is not needed for this task.

Example:
  orc skip TASK-001 --phase research --reason "already have spec"
  orc skip TASK-001 --phase test --reason "no testable changes"`,
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
			phaseID, _ := cmd.Flags().GetString("phase")
			reason, _ := cmd.Flags().GetString("reason")

			if phaseID == "" {
				return fmt.Errorf("--phase flag is required")
			}

			// Load task (execution state is in task.Execution)
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if phase is already completed
			if ps := t.Execution.Phases[phaseID]; ps != nil && ps.Status == task.PhaseStatusCompleted {
				return fmt.Errorf("phase %s is already completed", phaseID)
			}

			// Skip the phase
			t.Execution.SkipPhase(phaseID, reason)

			// Save task (includes execution state)
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("Phase %s skipped", phaseID)
			if reason != "" {
				fmt.Printf(": %s", reason)
			}
			fmt.Println()
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
	cmd.Flags().String("phase", "", "phase to skip (required)")
	cmd.Flags().StringP("reason", "r", "", "reason for skipping")
	_ = cmd.MarkFlagRequired("phase")
	return cmd
}
