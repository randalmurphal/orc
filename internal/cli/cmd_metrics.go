// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

// metricsResult holds aggregated quality metrics.
type metricsResult struct {
	TotalTasks          int
	TasksWithRetries    int
	TotalRetries        int
	ReviewRejections    int
	ManualInterventions int

	// By phase breakdown
	RetryByPhase map[string]int

	// Tasks with issues
	TasksWithIssues []*taskMetricSummary
}

// taskMetricSummary summarizes quality metrics for a single task.
type taskMetricSummary struct {
	ID                 string
	Title              string
	TotalRetries       int
	ReviewRejections   int
	ManualIntervention bool
	Reason             string
}

func init() {
	rootCmd.AddCommand(newMetricsCmd())
}

// newMetricsCmd creates the metrics command
func newMetricsCmd() *cobra.Command {
	var since string
	var showAll bool

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show quality metrics for tasks",
		Long: `Display quality metrics to track task execution quality over time.

Metrics tracked:
  - Phase retry rate: How often phases fail and retry
  - Review rejections: How often review phase rejects implementation
  - Manual interventions: How often humans had to fix things

These metrics help identify patterns in failures and measure improvement
from changes to prompts, specs, or workflow.

Examples:
  orc metrics                    # Show metrics for all tasks
  orc metrics --since 2025-01-01 # Show metrics since date
  orc metrics --all              # Include tasks without issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find the project root (handles worktrees)
			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return err
			}

			if err := config.RequireInitAt(projectRoot); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Parse since date if provided
			var sinceTime time.Time
			if since != "" {
				sinceTime, err = time.Parse("2006-01-02", since)
				if err != nil {
					return fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", err)
				}
			}

			// Load all tasks
			tasks, err := backend.LoadAllTasks()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			// Compute metrics
			result := computeMetricsProto(tasks, sinceTime)

			// Display results
			displayMetrics(result, showAll)

			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "only include tasks created after this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&showAll, "all", false, "include tasks without quality issues")

	return cmd
}

// computeMetricsProto aggregates quality metrics from proto tasks.
func computeMetricsProto(tasks []*orcv1.Task, since time.Time) *metricsResult {
	result := &metricsResult{
		RetryByPhase: make(map[string]int),
	}

	for _, t := range tasks {
		// Filter by date if specified
		if !since.IsZero() && t.CreatedAt != nil && t.CreatedAt.AsTime().Before(since) {
			continue
		}

		result.TotalTasks++

		// Check if task has quality metrics
		if t.Quality == nil {
			continue
		}

		hasIssues := false

		// Aggregate retry counts
		if t.Quality.TotalRetries > 0 {
			result.TasksWithRetries++
			result.TotalRetries += int(t.Quality.TotalRetries)
			hasIssues = true

			for phase, count := range t.Quality.PhaseRetries {
				result.RetryByPhase[phase] += int(count)
			}
		}

		// Aggregate review rejections
		if t.Quality.ReviewRejections > 0 {
			result.ReviewRejections += int(t.Quality.ReviewRejections)
			hasIssues = true
		}

		// Aggregate manual interventions
		if t.Quality.ManualIntervention {
			result.ManualInterventions++
			hasIssues = true
		}

		// Track task summary if it has issues
		if hasIssues {
			reason := ""
			if t.Quality.ManualInterventionReason != nil {
				reason = *t.Quality.ManualInterventionReason
			}
			result.TasksWithIssues = append(result.TasksWithIssues, &taskMetricSummary{
				ID:                 t.Id,
				Title:              t.Title,
				TotalRetries:       int(t.Quality.TotalRetries),
				ReviewRejections:   int(t.Quality.ReviewRejections),
				ManualIntervention: t.Quality.ManualIntervention,
				Reason:             reason,
			})
		}
	}

	// Sort tasks with issues by total issues (most problematic first)
	sort.Slice(result.TasksWithIssues, func(i, j int) bool {
		iScore := result.TasksWithIssues[i].TotalRetries + result.TasksWithIssues[i].ReviewRejections
		jScore := result.TasksWithIssues[j].TotalRetries + result.TasksWithIssues[j].ReviewRejections
		if result.TasksWithIssues[i].ManualIntervention {
			iScore += 10
		}
		if result.TasksWithIssues[j].ManualIntervention {
			jScore += 10
		}
		return iScore > jScore
	})

	return result
}

// displayMetrics prints the metrics to stdout.
func displayMetrics(result *metricsResult, showAll bool) {
	if result.TotalTasks == 0 {
		fmt.Println("No tasks found.")
		return
	}

	// Summary
	fmt.Println("## Quality Metrics Summary")
	fmt.Println()
	fmt.Printf("Total tasks analyzed: %d\n", result.TotalTasks)
	fmt.Println()

	// Retry rate
	if result.TotalRetries > 0 {
		retryRate := float64(result.TasksWithRetries) / float64(result.TotalTasks) * 100
		fmt.Printf("Tasks with retries: %d (%.1f%%)\n", result.TasksWithRetries, retryRate)
		fmt.Printf("Total phase retries: %d\n", result.TotalRetries)

		// Breakdown by phase
		if len(result.RetryByPhase) > 0 {
			fmt.Println("\nRetries by phase:")
			// Sort phases for consistent output
			phases := make([]string, 0, len(result.RetryByPhase))
			for phase := range result.RetryByPhase {
				phases = append(phases, phase)
			}
			sort.Strings(phases)

			for _, phase := range phases {
				fmt.Printf("  - %s: %d\n", phase, result.RetryByPhase[phase])
			}
		}
	} else {
		fmt.Println("No phase retries recorded.")
	}

	fmt.Println()

	// Review rejections
	if result.ReviewRejections > 0 {
		fmt.Printf("Review rejections: %d\n", result.ReviewRejections)
	} else {
		fmt.Println("No review rejections recorded.")
	}

	// Manual interventions
	if result.ManualInterventions > 0 {
		interventionRate := float64(result.ManualInterventions) / float64(result.TotalTasks) * 100
		fmt.Printf("Manual interventions: %d (%.1f%%)\n", result.ManualInterventions, interventionRate)
	} else {
		fmt.Println("No manual interventions recorded.")
	}

	// Tasks with issues
	if len(result.TasksWithIssues) > 0 {
		fmt.Println()
		fmt.Println("## Tasks with Quality Issues")
		fmt.Println()
		fmt.Println("| Task | Title | Retries | Reviews | Manual |")
		fmt.Println("|------|-------|---------|---------|--------|")

		for _, t := range result.TasksWithIssues {
			manual := ""
			if t.ManualIntervention {
				manual = "yes"
			}
			// Truncate title if too long
			title := t.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			fmt.Printf("| %s | %s | %d | %d | %s |\n",
				t.ID, title, t.TotalRetries, t.ReviewRejections, manual)
		}
	}

	// Interpretation guidance
	fmt.Println()
	fmt.Println("## Interpretation")
	fmt.Println()
	if result.TotalRetries > 0 || result.ReviewRejections > 0 || result.ManualInterventions > 0 {
		fmt.Println("High retry rates may indicate:")
		fmt.Println("  - Specs need more detail (preservation requirements, impact analysis)")
		fmt.Println("  - Review criteria too strict or not aligned with spec")
		fmt.Println("  - Implementation phase missing dependency updates")
		fmt.Println()
		fmt.Println("High manual intervention rate may indicate:")
		fmt.Println("  - Tasks too complex for current automation")
		fmt.Println("  - Missing context in prompts")
		fmt.Println("  - Edge cases not covered in specs")
	} else {
		fmt.Println("No quality issues detected. The system is performing well.")
	}
}
