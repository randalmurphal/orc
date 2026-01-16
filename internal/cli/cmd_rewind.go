// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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

			// Load plan to get commit SHA for the phase
			p, err := backend.LoadPlan(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			phase := p.GetPhase(toPhase)
			if phase == nil {
				// Show available phases
				fmt.Printf("Phase '%s' not found.\n\nAvailable phases:\n", toPhase)
				for _, ph := range p.Phases {
					checkpoint := ""
					if ph.CommitSHA != "" {
						checkpoint = fmt.Sprintf(" (checkpoint: %s)", ph.CommitSHA[:7])
					}
					fmt.Printf("  %s%s\n", ph.ID, checkpoint)
				}
				return fmt.Errorf("phase %s not found", toPhase)
			}

			if phase.CommitSHA == "" {
				return fmt.Errorf("phase %s has no checkpoint (has it completed?)", toPhase)
			}

			if !force {
				fmt.Printf("⚠️  This will reset to commit %s\n", phase.CommitSHA[:7])
				fmt.Println("   All changes after this point will be lost!")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				_, _ = fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted")
					return nil
				}
			}

			// Load state and reset phases after this one
			s, err := backend.LoadState(id)
			if err != nil {
				// State might not exist, create new one
				s = state.New(id)
			}

			// Mark later phases as pending
			foundTarget := false
			for i := range p.Phases {
				if p.Phases[i].ID == toPhase {
					foundTarget = true
					p.Phases[i].Status = plan.PhasePending
					p.Phases[i].CommitSHA = ""
					continue
				}
				if foundTarget {
					p.Phases[i].Status = plan.PhasePending
					p.Phases[i].CommitSHA = ""
					if s.Phases[p.Phases[i].ID] != nil {
						s.Phases[p.Phases[i].ID].Status = state.StatusPending
					}
				}
			}

			// Save updated state
			if err := backend.SavePlan(p, id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}
			if err := backend.SaveState(s); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			fmt.Printf("✅ Rewound to phase: %s\n", toPhase)
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
	cmd.Flags().String("to", "", "phase to rewind to (required)")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt (for scripts/automation)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}
