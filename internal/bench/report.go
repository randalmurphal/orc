package bench

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// PhaseLeaderboard holds ranked results for a single phase.
type PhaseLeaderboard struct {
	PhaseID  string             `json:"phase_id"`
	Entries  []LeaderboardEntry `json:"entries"`
	Winner   string             `json:"winner,omitempty"`    // Best variant ID
	Uncertain bool             `json:"uncertain,omitempty"` // True if no statistically significant winner
}

// LeaderboardEntry is one variant's results for a phase.
type LeaderboardEntry struct {
	VariantID       string  `json:"variant_id"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Reasoning       string  `json:"reasoning_effort,omitempty"`
	PassRate        float64 `json:"pass_rate"`
	AvgCostUSD      float64 `json:"avg_cost_usd"`
	AvgDurationMs   int     `json:"avg_duration_ms"`
	AvgJudgeScore   float64 `json:"avg_judge_score,omitempty"`
	SampleSize      int     `json:"sample_size"`
	CostPerSuccess  float64 `json:"cost_per_success"` // North star metric
}

// OptimalConfig recommends the best model for each phase.
type OptimalConfig struct {
	PhaseID       string  `json:"phase_id"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	Reasoning     string  `json:"reasoning_effort,omitempty"`
	PassRate      float64 `json:"pass_rate"`
	CostPerSuccess float64 `json:"cost_per_success"`
	Confidence    string  `json:"confidence"` // "high", "medium", "low"
	Rationale     string  `json:"rationale"`
}

// FullReport contains the complete benchmark analysis.
type FullReport struct {
	Summary         ReportSummary      `json:"summary"`
	Leaderboards    []PhaseLeaderboard `json:"leaderboards"`
	Recommendations []OptimalConfig    `json:"recommendations"`
	Comparisons     []PairedComparison `json:"comparisons,omitempty"`
}

// ReportSummary provides high-level stats.
type ReportSummary struct {
	TotalRuns       int     `json:"total_runs"`
	TotalVariants   int     `json:"total_variants"`
	TotalTasks      int     `json:"total_tasks"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	BaselinePassRate float64 `json:"baseline_pass_rate"`
}

// ReportGenerator builds reports from benchmark data.
type ReportGenerator struct {
	store *Store
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator(store *Store) *ReportGenerator {
	return &ReportGenerator{store: store}
}

// GenerateFullReport produces a complete benchmark analysis.
func (rg *ReportGenerator) GenerateFullReport(ctx context.Context) (*FullReport, error) {
	report := &FullReport{}

	// Summary
	summary, err := rg.buildSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("build summary: %w", err)
	}
	report.Summary = *summary

	// Phase leaderboards
	leaderboards, err := rg.buildLeaderboards(ctx)
	if err != nil {
		return nil, fmt.Errorf("build leaderboards: %w", err)
	}
	report.Leaderboards = leaderboards

	// Recommendations
	report.Recommendations = rg.buildRecommendations(leaderboards)

	// Statistical comparisons
	comparisons, err := rg.buildComparisons(ctx)
	if err != nil {
		slog.Warn("statistical comparisons unavailable", "error", err)
		comparisons = nil
	}
	report.Comparisons = comparisons

	return report, nil
}

// buildSummary generates the high-level summary.
func (rg *ReportGenerator) buildSummary(ctx context.Context) (*ReportSummary, error) {
	variants, err := rg.store.ListVariants(ctx)
	if err != nil {
		return nil, err
	}

	tasks, err := rg.store.ListTasks(ctx, "", "")
	if err != nil {
		return nil, err
	}

	allRuns, err := rg.store.ListRuns(ctx, "", "", "")
	if err != nil {
		return nil, err
	}

	var totalCost float64
	for _, run := range allRuns {
		phases, err := rg.store.GetPhaseResults(ctx, run.ID)
		if err != nil {
			continue
		}
		for _, pr := range phases {
			if !pr.WasFrozen {
				totalCost += pr.CostUSD
			}
		}
	}

	// Baseline pass rate
	var baselinePassRate float64
	for _, v := range variants {
		if v.IsBaseline {
			pass, fail, errCount, _ := rg.store.CountRunsByStatus(ctx, v.ID)
			total := pass + fail + errCount
			if total > 0 {
				baselinePassRate = float64(pass) / float64(total) * 100
			}
			break
		}
	}

	return &ReportSummary{
		TotalRuns:        len(allRuns),
		TotalVariants:    len(variants),
		TotalTasks:       len(tasks),
		TotalCostUSD:     totalCost,
		BaselinePassRate: baselinePassRate,
	}, nil
}

// buildLeaderboards creates phase-level rankings.
func (rg *ReportGenerator) buildLeaderboards(ctx context.Context) ([]PhaseLeaderboard, error) {
	variants, err := rg.store.ListVariants(ctx)
	if err != nil {
		return nil, err
	}

	// Collect phase IDs from all phase results
	phaseIDs := make(map[string]bool)
	for _, v := range variants {
		runs, err := rg.store.ListRuns(ctx, v.ID, "", "")
		if err != nil {
			continue
		}
		for _, run := range runs {
			phases, err := rg.store.GetPhaseResults(ctx, run.ID)
			if err != nil {
				continue
			}
			for _, pr := range phases {
				if !pr.WasFrozen {
					phaseIDs[pr.PhaseID] = true
				}
			}
		}
	}

	// Build leaderboard for each phase
	var sortedPhases []string
	for pid := range phaseIDs {
		sortedPhases = append(sortedPhases, pid)
	}
	sort.Strings(sortedPhases)

	var leaderboards []PhaseLeaderboard

	for _, phaseID := range sortedPhases {
		lb, err := rg.buildPhaseLeaderboard(ctx, phaseID, variants)
		if err != nil {
			continue
		}
		leaderboards = append(leaderboards, *lb)
	}

	return leaderboards, nil
}

// buildPhaseLeaderboard ranks variants for a single phase.
func (rg *ReportGenerator) buildPhaseLeaderboard(ctx context.Context, phaseID string, variants []*Variant) (*PhaseLeaderboard, error) {
	lb := &PhaseLeaderboard{PhaseID: phaseID}

	for _, v := range variants {
		runs, err := rg.store.ListRuns(ctx, v.ID, "", "")
		if err != nil {
			continue
		}

		var (
			totalCost      float64
			totalDur       int
			passCount      int
			completedCount int // Only terminal statuses (pass/fail/error)
			totalCount     int
			provider       string
			model          string
			reasoning      string
			judgeTotal     float64
			judgeCount     int
		)

		for _, run := range runs {
			// Only count terminal runs for pass rate calculation
			isTerminal := run.Status == RunStatusPass || run.Status == RunStatusFail || run.Status == RunStatusError
			if !isTerminal {
				continue
			}
			completedCount++

			phases, err := rg.store.GetPhaseResults(ctx, run.ID)
			if err != nil {
				continue
			}

			for _, pr := range phases {
				if pr.PhaseID != phaseID || pr.WasFrozen {
					continue
				}

				totalCount++
				totalCost += pr.CostUSD
				totalDur += pr.DurationMs

				if provider == "" {
					provider = pr.Provider
					model = pr.Model
					reasoning = pr.ReasoningEffort
				}
			}

			// Count pass/fail at the run level
			if run.Status == RunStatusPass {
				passCount++
			}

			// Aggregate judge scores for this phase
			judgments, err := rg.store.GetJudgments(ctx, run.ID)
			if err == nil {
				for _, j := range judgments {
					if j.PhaseID == phaseID {
						for _, score := range j.Scores {
							judgeTotal += float64(score)
							judgeCount++
						}
					}
				}
			}
		}

		if totalCount == 0 {
			continue
		}

		var passRate, avgCost, costPerSuccess, avgJudgeScore float64
		avgDur := totalDur / totalCount
		avgCost = totalCost / float64(totalCount)

		if completedCount > 0 {
			passRate = float64(passCount) / float64(completedCount) * 100
		}
		if passCount > 0 {
			costPerSuccess = totalCost / float64(passCount)
		} else {
			costPerSuccess = totalCost // No successes, entire cost is "wasted"
		}

		if judgeCount > 0 {
			avgJudgeScore = judgeTotal / float64(judgeCount)
		}

		lb.Entries = append(lb.Entries, LeaderboardEntry{
			VariantID:      v.ID,
			Provider:       provider,
			Model:          model,
			Reasoning:      reasoning,
			PassRate:       passRate,
			AvgCostUSD:     avgCost,
			AvgDurationMs:  avgDur,
			AvgJudgeScore:  avgJudgeScore,
			SampleSize:     totalCount,
			CostPerSuccess: costPerSuccess,
		})
	}

	// Sort by cost-per-success (lower is better), with pass rate as tiebreaker
	sort.Slice(lb.Entries, func(i, j int) bool {
		if lb.Entries[i].CostPerSuccess == lb.Entries[j].CostPerSuccess {
			return lb.Entries[i].PassRate > lb.Entries[j].PassRate
		}
		return lb.Entries[i].CostPerSuccess < lb.Entries[j].CostPerSuccess
	})

	if len(lb.Entries) > 0 {
		lb.Winner = lb.Entries[0].VariantID
	}

	return lb, nil
}

// buildRecommendations generates per-phase recommendations from leaderboards.
func (rg *ReportGenerator) buildRecommendations(leaderboards []PhaseLeaderboard) []OptimalConfig {
	var recs []OptimalConfig

	for _, lb := range leaderboards {
		if len(lb.Entries) == 0 {
			continue
		}

		winner := lb.Entries[0]

		confidence := "low"
		rationale := "insufficient data"

		if winner.SampleSize >= 4 {
			confidence = "medium"
			rationale = fmt.Sprintf("%.0f%% pass rate at $%.4f/success across %d samples",
				winner.PassRate, winner.CostPerSuccess, winner.SampleSize)
		}
		if winner.SampleSize >= 8 && winner.PassRate >= 80 {
			confidence = "high"
		}

		// Check if the winner is clearly better than the runner-up
		if len(lb.Entries) > 1 {
			runnerUp := lb.Entries[1]
			if winner.CostPerSuccess > 0 && runnerUp.CostPerSuccess > 0 {
				improvement := (runnerUp.CostPerSuccess - winner.CostPerSuccess) / runnerUp.CostPerSuccess * 100
				if improvement > 20 {
					rationale += fmt.Sprintf("; %.0f%% cheaper than %s", improvement, runnerUp.VariantID)
				} else if improvement < 5 {
					confidence = "low"
					rationale += fmt.Sprintf("; marginal improvement over %s", runnerUp.VariantID)
				}
			}
		}

		recs = append(recs, OptimalConfig{
			PhaseID:        lb.PhaseID,
			Provider:       winner.Provider,
			Model:          winner.Model,
			Reasoning:      winner.Reasoning,
			PassRate:       winner.PassRate,
			CostPerSuccess: winner.CostPerSuccess,
			Confidence:     confidence,
			Rationale:      rationale,
		})
	}

	return recs
}

// buildComparisons performs statistical comparisons between baseline and each variant.
func (rg *ReportGenerator) buildComparisons(ctx context.Context) ([]PairedComparison, error) {
	variants, err := rg.store.ListVariants(ctx)
	if err != nil {
		return nil, err
	}

	// Find baseline
	var baseline *Variant
	for _, v := range variants {
		if v.IsBaseline {
			baseline = v
			break
		}
	}
	if baseline == nil {
		return nil, fmt.Errorf("no baseline variant found")
	}

	tasks, err := rg.store.ListTasks(ctx, "", "")
	if err != nil {
		return nil, err
	}

	var comparisons []PairedComparison

	for _, v := range variants {
		if v.IsBaseline {
			continue
		}

		// Collect paired outcomes per task
		var baselinePass, variantPass []int
		var baselineCost, variantCost []float64

		for _, task := range tasks {
			// Get baseline run for this task
			bRuns, err := rg.store.ListRuns(ctx, baseline.ID, task.ID, "")
			if err != nil || len(bRuns) == 0 {
				continue
			}

			// Get variant run for this task
			vRuns, err := rg.store.ListRuns(ctx, v.ID, task.ID, "")
			if err != nil || len(vRuns) == 0 {
				continue
			}

			// Use first trial for paired comparison
			bRun := bRuns[0]
			vRun := vRuns[0]

			// Binary outcome
			bp := 0
			if bRun.Status == RunStatusPass {
				bp = 1
			}
			vp := 0
			if vRun.Status == RunStatusPass {
				vp = 1
			}
			baselinePass = append(baselinePass, bp)
			variantPass = append(variantPass, vp)

			// Cost
			bCost := rg.runCost(ctx, bRun.ID)
			vCost := rg.runCost(ctx, vRun.ID)
			baselineCost = append(baselineCost, bCost)
			variantCost = append(variantCost, vCost)
		}

		if len(baselinePass) == 0 {
			continue
		}

		comparison := ComparePaired(
			baseline.ID, v.ID,
			baselineCost, variantCost,
			baselinePass, variantPass,
			0.05,
		)
		comparisons = append(comparisons, comparison)
	}

	return comparisons, nil
}

// runCost sums the non-frozen phase costs for a run.
func (rg *ReportGenerator) runCost(ctx context.Context, runID string) float64 {
	phases, err := rg.store.GetPhaseResults(ctx, runID)
	if err != nil {
		return 0
	}

	var total float64
	for _, pr := range phases {
		if !pr.WasFrozen {
			total += pr.CostUSD
		}
	}
	return total
}

// FormatReport renders a FullReport as human-readable text.
func FormatReport(report *FullReport) string {
	var sb strings.Builder

	// Summary
	sb.WriteString("=== Benchmark Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Runs: %d  |  Variants: %d  |  Tasks: %d  |  Total cost: $%.2f\n",
		report.Summary.TotalRuns, report.Summary.TotalVariants,
		report.Summary.TotalTasks, report.Summary.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("Baseline pass rate: %.0f%%\n\n", report.Summary.BaselinePassRate))

	// Phase leaderboards
	for _, lb := range report.Leaderboards {
		sb.WriteString(fmt.Sprintf("--- Phase: %s ---\n", lb.PhaseID))
		if lb.Uncertain {
			sb.WriteString("  (no statistically significant winner)\n")
		}
		for i, e := range lb.Entries {
			rank := i + 1
			winner := ""
			if i == 0 {
				winner = " <-- best"
			}
			sb.WriteString(fmt.Sprintf("  #%d  %-30s  %s/%-15s  pass: %5.1f%%  cost/success: $%.4f  samples: %d%s\n",
				rank, e.VariantID, e.Provider, e.Model, e.PassRate, e.CostPerSuccess, e.SampleSize, winner))
		}
		sb.WriteString("\n")
	}

	// Recommendations
	if len(report.Recommendations) > 0 {
		sb.WriteString("=== Recommended Configuration ===\n\n")
		for _, rec := range report.Recommendations {
			sb.WriteString(fmt.Sprintf("  %-15s  %s/%s  (confidence: %s)\n",
				rec.PhaseID, rec.Provider, rec.Model, rec.Confidence))
			sb.WriteString(fmt.Sprintf("                   %s\n", rec.Rationale))
		}
		sb.WriteString("\n")
	}

	// Statistical comparisons
	if len(report.Comparisons) > 0 {
		sb.WriteString("=== Statistical Comparisons (vs baseline) ===\n\n")
		for _, c := range report.Comparisons {
			sig := ""
			if c.Significant {
				sig = " *"
			}
			sb.WriteString(fmt.Sprintf("  %s vs %s: p=%.4f%s (test: %s, n=%d)\n",
				c.VariantA, c.VariantB, c.PValue, sig, c.TestUsed, c.SampleSize))
		}
	}

	return sb.String()
}
