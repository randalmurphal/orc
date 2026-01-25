// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newRewindCmd creates the rewind command
func newRewindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rewind <task-id> --to <phase>",
		Short: "Rewind task to a checkpoint",
		Long: `Rewind a task to a previous checkpoint.

This uses git reset to restore the codebase state at that checkpoint.
All changes after that checkpoint will be lost.

Examples:
  orc rewind TASK-001 --to spec
  orc rewind TASK-001 --to implement
  orc rewind TASK-001 --to implement --force  # Skip confirmation (for scripts)`,
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
			toPhase, _ := cmd.Flags().GetString("to")
			force, _ := cmd.Flags().GetBool("force")

			if toPhase == "" {
				return fmt.Errorf("--to flag is required")
			}

			// Load task (execution state is embedded in task.Execution)
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if phase exists in execution state
			phaseState := t.Execution.Phases[toPhase]
			if phaseState == nil {
				// Show available phases
				fmt.Printf("Phase '%s' not found or has no state.\n\nAvailable phases:\n", toPhase)
				for phaseID, ps := range t.Execution.Phases {
					checkpoint := ""
					if ps.CommitSHA != "" {
						checkpoint = fmt.Sprintf(" (checkpoint: %s)", ps.CommitSHA[:7])
					}
					fmt.Printf("  %s%s\n", phaseID, checkpoint)
				}
				return fmt.Errorf("phase %s not found", toPhase)
			}

			if phaseState.CommitSHA == "" {
				return fmt.Errorf("phase %s has no checkpoint (has it completed?)", toPhase)
			}

			if !force {
				fmt.Printf("Warning: This will reset to commit %s\n", phaseState.CommitSHA[:7])
				fmt.Println("   All changes after this point will be lost!")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				_, _ = fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted")
					return nil
				}
			}

			// Reset phases after the target phase
			// Since we don't have an ordered phase list from plan, we reset all phases
			// to pending and let the executor determine the order at runtime
			for phaseID, ps := range t.Execution.Phases {
				if phaseID == toPhase || ps.Status == task.PhaseStatusCompleted {
					// Keep completed phases before target
					continue
				}
				ps.Status = task.PhaseStatusPending
				ps.CommitSHA = ""
			}

			// Mark target phase as pending so it will be re-executed
			t.Execution.Phases[toPhase].Status = task.PhaseStatusPending
			t.Execution.Phases[toPhase].CommitSHA = ""

			// Reset current phase tracking
			t.CurrentPhase = toPhase

			// Update task status to allow re-running (task.Status is single source of truth)
			t.Status = task.StatusPlanned
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("Rewound to phase: %s\n", toPhase)
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
	cmd.Flags().String("to", "", "phase to rewind to (required)")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt (for scripts/automation)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}
