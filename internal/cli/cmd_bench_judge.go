package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
)

func newBenchJudgeCmd() *cobra.Command {
	var (
		phaseFilter string
	)

	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Run cross-model evaluation panel",
		Long: `Run the cross-model judge panel for qualitative evaluation.

Judges evaluate phase outputs using blinded, randomized comparisons:
  - Opus judges GPT outputs only
  - GPT-5.3 judges Claude outputs only
  - Sonnet judges everything (cheap tiebreaker)

Each judge scores outputs on phase-specific criteria (1-5 scale).
Output identities are blinded to reduce evaluation bias.

Examples:
  orc bench judge
  orc bench judge --phase spec`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()
			judges := bench.DefaultJudgeConfigs()
			panel := bench.NewJudgePanel(store)

			if phaseFilter != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Judging phase %s across all completed runs...\n", phaseFilter)
				if err := panel.EvaluatePhase(ctx, phaseFilter, judges); err != nil {
					return fmt.Errorf("judge phase %s: %w", phaseFilter, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Phase %s judging complete.\n", phaseFilter)
				return nil
			}

			// Judge all completed runs
			runs, err := store.ListRuns(ctx, "", "", "")
			if err != nil {
				return fmt.Errorf("list runs: %w", err)
			}

			judged := 0
			for _, run := range runs {
				if run.Status != bench.RunStatusPass && run.Status != bench.RunStatusFail {
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Judging run %s (%s / %s)...\n", run.ID, run.VariantID, run.TaskID)
				if err := panel.EvaluateRun(ctx, run.ID, judges); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: judge run %s failed: %v\n", run.ID, err)
					continue
				}
				judged++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Judged %d runs.\n", judged)
			return nil
		},
	}

	cmd.Flags().StringVar(&phaseFilter, "phase", "", "Judge a specific phase only")

	return cmd
}
