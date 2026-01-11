package cli

import (
	"encoding/json"
	"fmt"

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
	cmd := &cobra.Command{
		Use:   "cost [task-id]",
		Short: "Show cost information",
		Long: `Show token usage and cost information.

Without arguments, shows aggregate cost across all tasks.
With a task ID, shows detailed cost breakdown for that task.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return wrapNotInitialized()
			}

			if len(args) > 0 {
				// Show cost for specific task
				id := args[0]
				return showTaskCost(id)
			}

			// Show aggregate cost
			return showAggregateCost()
		},
	}
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
func showAggregateCost() error {
	// Get all task IDs
	entries, err := state.LoadAllStates()
	if err != nil {
		fmt.Println("No tasks found.")
		return nil
	}

	var totalCost float64
	var totalInputTokens, totalOutputTokens int
	taskCount := 0

	for _, s := range entries {
		totalCost += s.Cost.TotalCostUSD
		totalInputTokens += s.Tokens.InputTokens
		totalOutputTokens += s.Tokens.OutputTokens
		taskCount++
	}

	if jsonOut {
		data, _ := json.MarshalIndent(map[string]any{
			"task_count":     taskCount,
			"total_cost_usd": totalCost,
			"tokens": map[string]int{
				"input":  totalInputTokens,
				"output": totalOutputTokens,
				"total":  totalInputTokens + totalOutputTokens,
			},
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("Cost Summary")
	fmt.Println("─────────────────────────")
	fmt.Printf("Tasks:       %d\n", taskCount)
	fmt.Printf("Total Cost:  $%.4f\n", totalCost)
	fmt.Println()
	fmt.Println("Token Usage:")
	fmt.Printf("  Input:     %d tokens\n", totalInputTokens)
	fmt.Printf("  Output:    %d tokens\n", totalOutputTokens)
	fmt.Printf("  Total:     %d tokens\n", totalInputTokens+totalOutputTokens)

	return nil
}
