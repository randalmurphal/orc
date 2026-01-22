// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// newShowCmd creates the show command
func newShowCmd() *cobra.Command {
	var showSession bool
	var showCost bool
	var showFull bool
	var showSpec bool
	var period string

	cmd := &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Long: `Show task details including status, phases, and execution state.

Optional flags to include additional information:
  --session    Include Claude session info (session ID, model, turn count)
  --cost       Include cost breakdown (tokens, per-phase costs)
  --spec       Show the task specification content
  --full       Include everything (session + cost)

Examples:
  orc show TASK-001              # Basic task info
  orc show TASK-001 --session    # Include session info
  orc show TASK-001 --cost       # Include cost breakdown
  orc show TASK-001 --spec       # View the spec content
  orc show TASK-001 --full       # Everything`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Create plan dynamically from task weight
			p := createShowPlanForWeight(id, t.Weight)
			s, _ := backend.LoadState(id)

			// --full enables everything
			if showFull {
				showSession = true
				showCost = true
			}

			// Load spec if requested
			var spec *storage.SpecInfo
			if showSpec {
				spec, _ = backend.LoadFullSpec(id)
			}

			// JSON output
			if jsonOut {
				result := map[string]any{
					"task":   t,
					"plan":   p,
					"status": t.Status,
				}
				if s != nil {
					result["state"] = s
				}
				if showSession && s != nil && s.Session != nil {
					result["session"] = s.Session
				}
				if showCost && s != nil {
					result["cost"] = map[string]any{
						"tokens": s.Tokens,
						"cost":   s.Cost,
					}
				}
				if showSpec {
					if spec != nil {
						result["spec"] = map[string]any{
							"source":       spec.Source,
							"content":      spec.Content,
							"content_hash": spec.ContentHash,
							"created_at":   spec.CreatedAt,
							"updated_at":   spec.UpdatedAt,
						}
					} else {
						result["spec"] = nil
					}
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Print task details
			fmt.Printf("\n%s - %s\n", t.ID, t.Title)
			fmt.Printf("────────────────────────────────────────────\n")
			fmt.Printf("Status:    %s\n", t.Status)
			fmt.Printf("Weight:    %s\n", t.Weight)
			fmt.Printf("Branch:    %s\n", t.Branch)
			fmt.Printf("Created:   %s\n", t.CreatedAt.Format(time.RFC3339))

			if t.StartedAt != nil {
				fmt.Printf("Started:   %s\n", t.StartedAt.Format(time.RFC3339))
			}
			if t.CompletedAt != nil {
				fmt.Printf("Completed: %s\n", t.CompletedAt.Format(time.RFC3339))
			}

			if t.Description != "" {
				fmt.Printf("\nDescription:\n%s\n", t.Description)
			}

			// Print phases
			if p != nil && len(p.Phases) > 0 {
				fmt.Printf("\nPhases:\n")
				for _, phase := range p.Phases {
					status := phaseStatusIcon(phase.Status)
					fmt.Printf("  %s %s", status, phase.ID)
					if phase.CommitSHA != "" {
						fmt.Printf(" (commit: %s)", phase.CommitSHA[:7])
					}
					fmt.Println()
				}
			}

			// Print execution state (tokens summary - always shown)
			if s != nil && s.Tokens.TotalTokens > 0 {
				fmt.Printf("\nTokens Used: %d\n", s.Tokens.TotalTokens)
			}

			// Print session info if requested
			if showSession {
				printSessionInfo(s, id)
			}

			// Print cost info if requested
			if showCost {
				printCostInfo(s, id, period)
			}

			// Print spec info if requested
			if showSpec {
				printSpecInfo(spec)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showSession, "session", false, "include session information")
	cmd.Flags().BoolVar(&showCost, "cost", false, "include cost breakdown")
	cmd.Flags().BoolVar(&showSpec, "spec", false, "show specification content")
	cmd.Flags().BoolVar(&showFull, "full", false, "include all details (session + cost)")
	cmd.Flags().StringVarP(&period, "period", "p", "", "cost period filter (day, week, month) - only with --cost")

	return cmd
}

// printSessionInfo displays session information for a task.
func printSessionInfo(s *state.State, id string) {
	fmt.Printf("\nSession\n")
	fmt.Printf("─────────────────────────\n")

	if s == nil || s.Session == nil {
		fmt.Printf("No session information recorded.\n")
		fmt.Println("Session info is recorded after the task starts running.")
		return
	}

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
}

// printCostInfo displays cost information for a task.
func printCostInfo(s *state.State, id string, _ string) {
	fmt.Printf("\nCost\n")
	fmt.Printf("─────────────────────────\n")

	if s == nil {
		fmt.Printf("No cost information recorded.\n")
		return
	}

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
}

// printSpecInfo displays specification content for a task.
func printSpecInfo(spec *storage.SpecInfo) {
	fmt.Printf("\nSpecification\n")
	fmt.Printf("─────────────────────────\n")

	if spec == nil {
		fmt.Printf("No specification found.\n")
		fmt.Println("Specs are generated during the 'spec' phase for medium/large/greenfield tasks.")
		return
	}

	// Show metadata
	source := spec.Source
	if source == "" {
		source = "unknown"
	}
	fmt.Printf("Source:   %s\n", source)
	fmt.Printf("Length:   %d bytes\n", len(spec.Content))
	lineCount := strings.Count(spec.Content, "\n") + 1
	if spec.Content == "" {
		lineCount = 0
	}
	fmt.Printf("Lines:    %d\n", lineCount)
	fmt.Printf("Created:  %s\n", spec.CreatedAt.Format("2006-01-02 15:04:05"))
	if !spec.UpdatedAt.IsZero() && spec.UpdatedAt != spec.CreatedAt {
		fmt.Printf("Updated:  %s\n", spec.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\n")

	// For long content (>50 lines), try to use a pager if we're in a terminal
	const pagerThreshold = 50
	if lineCount > pagerThreshold && isatty.IsTerminal(os.Stdout.Fd()) {
		// Try to use less, fall back to direct output
		if showWithPager(spec.Content) {
			return
		}
	}

	// Direct output
	fmt.Printf("Content:\n")
	fmt.Printf("────────────────────────────────────────────\n")
	fmt.Print(spec.Content)
	if !strings.HasSuffix(spec.Content, "\n") {
		fmt.Println()
	}
}

// showWithPager attempts to display content using the system pager (less).
// Returns true if successful, false if pager is not available.
func showWithPager(content string) bool {
	// Look for less first, then more
	pagerPath, err := exec.LookPath("less")
	if err != nil {
		pagerPath, err = exec.LookPath("more")
		if err != nil {
			return false
		}
	}

	cmd := exec.Command(pagerPath)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// createShowPlanForWeight creates an execution plan based on task weight.
// Plans are created dynamically for display, not stored.
func createShowPlanForWeight(taskID string, weight task.Weight) *executor.Plan {
	var phases []executor.Phase

	switch weight {
	case task.WeightTrivial:
		phases = []executor.Phase{
			{ID: "tiny_spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightSmall:
		phases = []executor.Phase{
			{ID: "tiny_spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightMedium:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightLarge:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "breakdown", Name: "Breakdown", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "validate", Name: "Validation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	default:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}
}
