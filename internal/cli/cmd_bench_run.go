package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

func newBenchRunCmd() *cobra.Command {
	var (
		baseline      bool
		variantID     string
		allVariants   bool
		trials        int
		modelOverride string
		taskIDs       []string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute benchmark runs",
		Long: `Execute benchmark runs against curated tasks.

Modes:
  --baseline          Run the baseline variant (all Opus). Must run first.
                      Saves frozen outputs for variant comparison.
  --variant <id>      Run a specific variant. Uses frozen baseline outputs
                      for phases without overrides.
  --all-variants      Run all non-baseline variants.

Each run clones the project repo, checks out the pre-fix commit in a worktree,
executes the workflow phases, and evaluates the result (tests, build, lint).

Flags:
  --model-override    Force all phases to use a specific model (provider:model[:effort]).
                      Overrides variant config. Use for cheap smoke testing.
                      Optional :effort suffix sets reasoning effort (e.g. high, medium, low).
  --task              Limit to specific task ID(s). Repeatable.

Examples:
  orc bench run --baseline --trials 2
  orc bench run --variant codex53-high-impl --trials 2
  orc bench run --all-variants --trials 2
  orc bench run --baseline --trials 1 --model-override claude:claude-haiku-4-5-20251001 --task bbolt-001
  orc bench run --baseline --trials 1 --model-override codex:gpt-5.3-codex:high --task bbolt-001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !baseline && variantID == "" && !allVariants {
				return fmt.Errorf("specify --baseline, --variant <id>, or --all-variants")
			}
			if trials < 1 {
				return fmt.Errorf("--trials must be at least 1")
			}

			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			gdb, err := db.OpenGlobal()
			if err != nil {
				return fmt.Errorf("open global db: %w", err)
			}
			defer gdb.Close()

			// Seed built-in workflows + phase templates (required for phase execution)
			if _, err := workflow.SeedBuiltins(gdb); err != nil {
				return fmt.Errorf("seed workflows: %w", err)
			}

			workspace, err := bench.DefaultWorkspace()
			if err != nil {
				return err
			}

			// Setup logger
			logLevel := slog.LevelInfo
			if verbose {
				logLevel = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

			// Build runner options
			opts := []bench.RunnerOption{bench.WithRunnerLogger(logger)}

			if modelOverride != "" {
				parts := strings.SplitN(modelOverride, ":", 3)
				if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
					return fmt.Errorf("--model-override must be provider:model[:effort] (e.g. codex:gpt-5.3-codex:high)")
				}
				var effort string
				if len(parts) == 3 {
					effort = parts[2]
				}
				opts = append(opts, bench.WithModelOverride(parts[0], parts[1], effort))
				logger.Info("model override active", "provider", parts[0], "model", parts[1], "effort", effort)
			}

			if len(taskIDs) > 0 {
				opts = append(opts, bench.WithTaskFilter(taskIDs))
				logger.Info("task filter active", "tasks", taskIDs)
			}

			runner := bench.NewRunner(store, gdb, workspace, opts...)

			// Handle interrupts gracefully
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Fprintln(os.Stderr, "\nInterrupted. Finishing current run...")
				cancel()
			}()

			if baseline {
				fmt.Fprintf(cmd.OutOrStdout(), "Running baseline with %d trial(s)...\n", trials)
				if err := runner.RunBaseline(ctx, trials); err != nil {
					return fmt.Errorf("baseline run: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Baseline complete.")
				return nil
			}

			if variantID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Running variant %s with %d trial(s)...\n", variantID, trials)
				if err := runner.RunVariant(ctx, variantID, trials); err != nil {
					return fmt.Errorf("variant run: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Variant %s complete.\n", variantID)
				return nil
			}

			// --all-variants
			variants, err := store.ListVariants(ctx)
			if err != nil {
				return fmt.Errorf("list variants: %w", err)
			}

			var nonBaseline []*bench.Variant
			for _, v := range variants {
				if !v.IsBaseline {
					nonBaseline = append(nonBaseline, v)
				}
			}

			if len(nonBaseline) == 0 {
				return fmt.Errorf("no non-baseline variants found")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Running %d variant(s) with %d trial(s) each...\n", len(nonBaseline), trials)
			for _, v := range nonBaseline {
				fmt.Fprintf(cmd.OutOrStdout(), "\n--- Variant: %s ---\n", v.ID)
				if err := runner.RunVariant(ctx, v.ID, trials); err != nil {
					fmt.Fprintf(os.Stderr, "variant %s failed: %v\n", v.ID, err)
					continue
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\nAll variant runs complete.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&baseline, "baseline", false, "Run the baseline variant (must run first)")
	cmd.Flags().StringVar(&variantID, "variant", "", "Run a specific variant by ID")
	cmd.Flags().BoolVar(&allVariants, "all-variants", false, "Run all non-baseline variants")
	cmd.Flags().IntVar(&trials, "trials", 2, "Number of trials per task")
	cmd.Flags().StringVar(&modelOverride, "model-override", "", "Force all phases to provider:model (e.g. claude:claude-haiku-4-5-20251001)")
	cmd.Flags().StringSliceVar(&taskIDs, "task", nil, "Limit to specific task ID(s) (repeatable)")

	return cmd
}
