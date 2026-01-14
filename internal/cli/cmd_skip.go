// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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

			id := args[0]
			phaseID, _ := cmd.Flags().GetString("phase")
			reason, _ := cmd.Flags().GetString("reason")

			if phaseID == "" {
				return fmt.Errorf("--phase flag is required")
			}

			// Load plan
			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			// Find and skip the phase
			phase := p.GetPhase(phaseID)
			if phase == nil {
				return fmt.Errorf("phase %s not found", phaseID)
			}

			if phase.Status == plan.PhaseCompleted {
				return fmt.Errorf("phase %s is already completed", phaseID)
			}

			phase.Status = plan.PhaseSkipped

			// Save plan
			if err := p.Save(id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}

			// Load and update state
			s, err := state.Load(id)
			if err == nil && s != nil {
				s.SkipPhase(phaseID, reason)
				s.Save()
			}

			// Auto-commit the phase skip
			cfg, _ := config.Load()
			if cfg != nil && !cfg.Tasks.DisableAutoCommit {
				if projectDir, err := config.FindProjectRoot(); err == nil {
					t, _ := task.Load(id)
					if t != nil {
						commitCfg := task.CommitConfig{
							ProjectRoot:  projectDir,
							CommitPrefix: cfg.CommitPrefix,
						}
						task.CommitAndSync(t, fmt.Sprintf("phase %s skipped", phaseID), commitCfg)
					}
				}
			}

			fmt.Printf("⊘ Phase %s skipped", phaseID)
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
	cmd.MarkFlagRequired("phase")
	return cmd
}
