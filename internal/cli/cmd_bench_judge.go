package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
)

func newBenchJudgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Run cross-model evaluation panel on implementation quality",
		Long: `Run the frontier judge panel for qualitative evaluation.

We're testing orchestrations, not models. Judges evaluate the final
implementation — the code the workflow produced. They don't see test
results or know which models ran, preventing anchoring and identity bias.

Two frontier judges with extended reasoning evaluate every run:
  - Opus 4.6 (extended thinking)
  - GPT-5.3-Codex (xhigh reasoning effort)

Both judges evaluate every run — blinding mitigates self-evaluation bias.
Judges are spawned inside a workspace with the actual code changes
committed. They explore the repo naturally (git diff, file reads, etc.)
and score on: functional_correctness, completeness, code_quality, minimal_change.

Cross-referencing judge correctness scores against automated test results
catches valid alternative solutions that the reference PR's tests miss.

Examples:
  orc bench judge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ws, err := bench.DefaultWorkspace()
			if err != nil {
				return fmt.Errorf("create workspace: %w", err)
			}

			ctx := context.Background()
			judges := bench.DefaultJudgeConfigs()
			panel := bench.NewJudgePanel(store, bench.WithJudgeWorkspace(ws))

			// Judge all completed runs
			runs, err := store.ListRuns(ctx, "", "", "")
			if err != nil {
				return fmt.Errorf("list runs: %w", err)
			}

			judged, skipped := 0, 0
			for _, run := range runs {
				if run.Status != bench.RunStatusPass && run.Status != bench.RunStatusFail {
					continue
				}

				// Skip runs that already have all judge opinions
				existing, err := store.GetJudgments(ctx, run.ID)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: check judgments for %s: %v\n", run.ID, err)
				}
				if len(existing) >= len(judges) {
					skipped++
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Judging run %s (%s / %s)...\n", run.ID, run.VariantID, run.TaskID)
				if err := panel.EvaluateRun(ctx, run.ID, judges); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: judge run %s failed: %v\n", run.ID, err)
					continue
				}
				judged++
			}

			if skipped > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d already-judged runs.\n", skipped)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Judged %d runs.\n", judged)
			return nil
		},
	}

	return cmd
}
