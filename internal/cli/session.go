package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/state"
)

// newSessionCmd creates the session command.
func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session <task-id>",
		Short: "Show session information for a task",
		Long: `Show Claude session information for a task, including:
  • Session ID
  • Model being used
  • Session status
  • Turn count
  • Last activity time

This information is useful for debugging or resuming tasks.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return wrapNotInitialized()
			}

			id := args[0]
			s, err := state.Load(id)
			if err != nil {
				return wrapTaskNotFound(id)
			}

			if s.Session == nil {
				fmt.Printf("No session information for task %s\n", id)
				fmt.Println("\nSession info is recorded after the task starts running.")
				return nil
			}

			if jsonOut {
				data, _ := json.MarshalIndent(s.Session, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Session for %s\n", id)
			fmt.Println("─────────────────────────")
			fmt.Printf("Session ID:    %s\n", s.Session.ID)
			fmt.Printf("Model:         %s\n", s.Session.Model)
			fmt.Printf("Status:        %s\n", s.Session.Status)
			fmt.Printf("Turn Count:    %d\n", s.Session.TurnCount)
			fmt.Printf("Created:       %s\n", s.Session.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Last Activity: %s\n", s.Session.LastActivity.Format("2006-01-02 15:04:05"))

			// Show resume hint if session is paused
			if s.Status == state.StatusPaused || s.Status == state.StatusInterrupted {
				fmt.Println()
				fmt.Printf("💡 To resume: orc resume %s\n", id)
			}

			return nil
		},
	}
	return cmd
}

// newCostCmd creates the cost command.
func newCostCmd() *cobra.Command {
	var period string

	cmd := &cobra.Command{
		Use:   "cost [task-id]",
		Short: "Show cost information",
		Long: `Show token usage and cost information.

Without arguments, shows aggregate cost across all tasks.
With a task ID, shows detailed cost breakdown for that task.

Use --period to filter by time range:
  day   - last 24 hours
  week  - last 7 days
  month - last 30 days
  all   - all time (default)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return wrapNotInitialized()
			}

			if len(args) > 0 {
				// Show cost for specific task
				id := args[0]
				return showTaskCost(id)
			}

			// Show aggregate cost with period filter
			return showAggregateCost(period)
		},
	}

	cmd.Flags().StringVarP(&period, "period", "p", "all", "Time period: day, week, month, all")
	return cmd
}

// showTaskCost displays cost breakdown for a single task.
func showTaskCost(id string) error {
	s, err := state.Load(id)
	if err != nil {
		return wrapTaskNotFound(id)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(map[string]any{
			"task_id": id,
			"tokens":  s.Tokens,
			"cost":    s.Cost,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Cost for %s\n", id)
	fmt.Println("─────────────────────────")
	fmt.Printf("Total Cost:    $%.4f\n", s.Cost.TotalCostUSD)
	fmt.Println()
	fmt.Println("Token Usage:")
	fmt.Printf("  Input:       %d tokens\n", s.Tokens.InputTokens)
	fmt.Printf("  Output:      %d tokens\n", s.Tokens.OutputTokens)
	fmt.Printf("  Total:       %d tokens\n", s.Tokens.TotalTokens)

	if len(s.Cost.PhaseCosts) > 0 {
		fmt.Println()
		fmt.Println("Cost by Phase:")
		for phase, cost := range s.Cost.PhaseCosts {
			fmt.Printf("  %-12s $%.4f\n", phase+":", cost)
		}
	}

	return nil
}

// showAggregateCost displays cost summary across all tasks.
func showAggregateCost(period string) error {
	// Get all task IDs
	entries, err := state.LoadAllStates()
	if err != nil {
		fmt.Println("No tasks found.")
		return nil
	}

	// Calculate time filter based on period
	now := time.Now()
	var since time.Time
	switch period {
	case "day":
		since = now.AddDate(0, 0, -1)
	case "week":
		since = now.AddDate(0, 0, -7)
	case "month":
		since = now.AddDate(0, -1, 0)
	case "all", "":
		since = time.Time{} // No filter
	default:
		return fmt.Errorf("invalid period: %s (use day, week, month, or all)", period)
	}

	var totalCost float64
	var totalInputTokens, totalOutputTokens int
	taskCount := 0
	phaseCosts := make(map[string]float64)

	for _, s := range entries {
		// Filter by time if period is specified
		if !since.IsZero() && s.StartedAt.Before(since) {
			continue
		}

		totalCost += s.Cost.TotalCostUSD
		totalInputTokens += s.Tokens.InputTokens
		totalOutputTokens += s.Tokens.OutputTokens
		taskCount++

		// Aggregate phase costs
		for phase, cost := range s.Cost.PhaseCosts {
			phaseCosts[phase] += cost
		}
	}

	// Check budget threshold
	cfg, _ := config.Load()
	var budgetWarning string
	if cfg != nil && cfg.Budget.ThresholdUSD > 0 && totalCost >= cfg.Budget.ThresholdUSD {
		budgetWarning = fmt.Sprintf("Budget threshold of $%.2f reached!", cfg.Budget.ThresholdUSD)
	}

	if jsonOut {
		result := map[string]any{
			"period":         period,
			"task_count":     taskCount,
			"total_cost_usd": totalCost,
			"tokens": map[string]int{
				"input":  totalInputTokens,
				"output": totalOutputTokens,
				"total":  totalInputTokens + totalOutputTokens,
			},
			"by_phase": phaseCosts,
		}
		if budgetWarning != "" {
			result["budget_warning"] = budgetWarning
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	periodLabel := "All Time"
	switch period {
	case "day":
		periodLabel = "Last 24 Hours"
	case "week":
		periodLabel = "Last 7 Days"
	case "month":
		periodLabel = "Last 30 Days"
	}

	fmt.Printf("Cost Summary (%s)\n", periodLabel)
	fmt.Println("─────────────────────────")
	fmt.Printf("Tasks:       %d\n", taskCount)
	fmt.Printf("Total Cost:  $%.4f\n", totalCost)
	fmt.Println()
	fmt.Println("Token Usage:")
	fmt.Printf("  Input:     %d tokens\n", totalInputTokens)
	fmt.Printf("  Output:    %d tokens\n", totalOutputTokens)
	fmt.Printf("  Total:     %d tokens\n", totalInputTokens+totalOutputTokens)

	if len(phaseCosts) > 0 {
		fmt.Println()
		fmt.Println("Cost by Phase:")
		for phase, cost := range phaseCosts {
			fmt.Printf("  %-12s $%.4f\n", phase+":", cost)
		}
	}

	if budgetWarning != "" {
		fmt.Println()
		fmt.Printf("⚠️  %s\n", budgetWarning)
	}

	return nil
}
