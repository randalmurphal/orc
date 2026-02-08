package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
	"github.com/randalmurphal/orc/internal/db"
)

func newBenchRunCmd() *cobra.Command {
	var (
		baseline    bool
		variantID   string
		allVariants bool
		trials      int
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

Examples:
  orc bench run --baseline --trials 2
  orc bench run --variant codex53-high-impl --trials 2
  orc bench run --all-variants --trials 2`,
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

			runner := bench.NewRunner(store, gdb, workspace, bench.WithRunnerLogger(logger))

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

	return cmd
}
