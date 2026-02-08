package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
)

func newBenchReportCmd() *cobra.Command {
	var (
		phaseFilter string
		full        bool
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "View benchmark results and recommendations",
		Long: `Display benchmark results as phase leaderboards with pass rates, costs, and timing.

Shows a summary table per variant with pass/fail/error counts and average cost.
Use --phase to see detailed results for a specific phase.
Use --full for the complete analysis with leaderboards, recommendations, and
statistical comparisons.

Examples:
  orc bench report
  orc bench report --full
  orc bench report --phase implement
  orc bench report --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()

			if full {
				return reportFull(cmd, store, ctx)
			}
			if phaseFilter != "" {
				return reportPhaseDetail(cmd, store, ctx, phaseFilter)
			}
			return reportSummary(cmd, store, ctx)
		},
	}

	cmd.Flags().StringVar(&phaseFilter, "phase", "", "Show detailed results for a specific phase")
	cmd.Flags().BoolVar(&full, "full", false, "Full analysis with leaderboards, recommendations, and stats")

	return cmd
}

// reportSummary shows a high-level overview of all variants.
func reportSummary(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	variants, err := store.ListVariants(ctx)
	if err != nil {
		return err
	}

	if len(variants) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No variants defined. Run 'orc bench curate import' first.")
		return nil
	}

	type variantSummary struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Baseline   bool    `json:"baseline"`
		Workflow   string  `json:"workflow"`
		Pass       int     `json:"pass"`
		Fail       int     `json:"fail"`
		Error      int     `json:"error"`
		Total      int     `json:"total"`
		PassRate   float64 `json:"pass_rate"`
		TotalCost  float64 `json:"total_cost"`
		AvgCostUSD float64 `json:"avg_cost_usd"`
	}

	var summaries []variantSummary

	for _, v := range variants {
		pass, fail, errCount, err := store.CountRunsByStatus(ctx, v.ID)
		if err != nil {
			return fmt.Errorf("count runs for %s: %w", v.ID, err)
		}

		total := pass + fail + errCount
		var passRate float64
		if total > 0 {
			passRate = float64(pass) / float64(total) * 100
		}

		// Compute cost from phase results
		runs, err := store.ListRuns(ctx, v.ID, "", "")
		if err != nil {
			return fmt.Errorf("list runs for %s: %w", v.ID, err)
		}

		var totalCost float64
		for _, run := range runs {
			phases, err := store.GetPhaseResults(ctx, run.ID)
			if err != nil {
				continue
			}
			for _, pr := range phases {
				if !pr.WasFrozen {
					totalCost += pr.CostUSD
				}
			}
		}

		var avgCost float64
		if total > 0 {
			avgCost = totalCost / float64(total)
		}

		summaries = append(summaries, variantSummary{
			ID:         v.ID,
			Name:       v.Name,
			Baseline:   v.IsBaseline,
			Workflow:   v.BaseWorkflow,
			Pass:       pass,
			Fail:       fail,
			Error:      errCount,
			Total:      total,
			PassRate:   passRate,
			TotalCost:  totalCost,
			AvgCostUSD: avgCost,
		})
	}

	if jsonOut {
		return outputJSON(cmd, summaries)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "VARIANT\tWORKFLOW\tPASS\tFAIL\tERR\tRATE\tAVG COST\tTOTAL COST")

	for _, s := range summaries {
		name := s.ID
		if s.Baseline {
			name += " *"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%.0f%%\t$%.4f\t$%.4f\n",
			name, s.Workflow, s.Pass, s.Fail, s.Error, s.PassRate, s.AvgCostUSD, s.TotalCost)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// Check if any data exists
	hasData := false
	for _, s := range summaries {
		if s.Total > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo runs yet. Execute 'orc bench run --baseline' to start.")
	}

	return nil
}

// reportPhaseDetail shows detailed results for a specific phase across variants.
func reportPhaseDetail(cmd *cobra.Command, store *bench.Store, ctx context.Context, phaseID string) error {
	variants, err := store.ListVariants(ctx)
	if err != nil {
		return err
	}

	type phaseStats struct {
		Variant         string  `json:"variant"`
		Provider        string  `json:"provider"`
		Model           string  `json:"model"`
		Reasoning       string  `json:"reasoning,omitempty"`
		Executions      int     `json:"executions"`
		FrozenCount     int     `json:"frozen_count"`
		AvgDurationMs   int     `json:"avg_duration_ms"`
		AvgInputTokens  int     `json:"avg_input_tokens"`
		AvgOutputTokens int     `json:"avg_output_tokens"`
		TotalCost       float64 `json:"total_cost"`
	}

	var stats []phaseStats

	for _, v := range variants {
		runs, err := store.ListRuns(ctx, v.ID, "", "")
		if err != nil {
			continue
		}

		var (
			totalDur    int
			totalInput  int
			totalOutput int
			totalCost   float64
			execCount   int
			frozenCount int
			provider    string
			model       string
			reasoning   string
		)

		for _, run := range runs {
			phases, err := store.GetPhaseResults(ctx, run.ID)
			if err != nil {
				continue
			}
			for _, pr := range phases {
				if pr.PhaseID != phaseID {
					continue
				}
				if pr.WasFrozen {
					frozenCount++
					continue
				}
				execCount++
				totalDur += pr.DurationMs
				totalInput += pr.InputTokens
				totalOutput += pr.OutputTokens
				totalCost += pr.CostUSD
				if provider == "" {
					provider = pr.Provider
					model = pr.Model
					reasoning = pr.ReasoningEffort
				}
			}
		}

		if execCount == 0 && frozenCount == 0 {
			continue
		}

		var avgDur, avgInput, avgOutput int
		if execCount > 0 {
			avgDur = totalDur / execCount
			avgInput = totalInput / execCount
			avgOutput = totalOutput / execCount
		}

		stats = append(stats, phaseStats{
			Variant:         v.ID,
			Provider:        provider,
			Model:           model,
			Reasoning:       reasoning,
			Executions:      execCount,
			FrozenCount:     frozenCount,
			AvgDurationMs:   avgDur,
			AvgInputTokens:  avgInput,
			AvgOutputTokens: avgOutput,
			TotalCost:       totalCost,
		})
	}

	if jsonOut {
		return outputJSON(cmd, stats)
	}

	if len(stats) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No results for phase %q\n", phaseID)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Phase: %s\n\n", phaseID)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "VARIANT\tPROVIDER\tMODEL\tEXEC\tFROZEN\tAVG DUR\tAVG IN\tAVG OUT\tCOST")

	for _, s := range stats {
		dur := fmt.Sprintf("%.1fs", float64(s.AvgDurationMs)/1000)
		if s.Executions == 0 {
			dur = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\t%d\t%d\t$%.4f\n",
			s.Variant, s.Provider, s.Model, s.Executions, s.FrozenCount,
			dur, s.AvgInputTokens, s.AvgOutputTokens, s.TotalCost)
	}
	return w.Flush()
}

// reportFull generates the complete analysis using ReportGenerator.
func reportFull(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	rg := bench.NewReportGenerator(store)
	report, err := rg.GenerateFullReport(ctx)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}

	if jsonOut {
		return outputJSON(cmd, report)
	}

	fmt.Fprint(cmd.OutOrStdout(), bench.FormatReport(report))
	return nil
}
