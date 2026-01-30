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

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// newShowCmd creates the show command
func newShowCmd() *cobra.Command {
	var showSession bool
	var showCost bool
	var showFull bool
	var showSpec bool
	var showReview bool
	var period string

	cmd := &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Long: `Show task details including status, phases, and execution state.

Optional flags to include additional information:
  --session    Include Claude session info (session ID, model, turn count)
  --cost       Include cost breakdown (tokens, per-phase costs)
  --spec       Show the task specification content
  --review     Show review findings (issues, positives, constitution violations)
  --full       Include everything (session + cost + review)

Examples:
  orc show TASK-001              # Basic task info
  orc show TASK-001 --session    # Include session info
  orc show TASK-001 --cost       # Include cost breakdown
  orc show TASK-001 --spec       # View the spec content
  orc show TASK-001 --review     # View review findings
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

			// Create plan from actual workflow if a workflow run exists, otherwise from task weight
			var p *executor.Plan
			workflowID := t.GetWorkflowId()
			if workflowID == "" {
				// Check if there's a workflow run for this task
				runs, _ := backend.ListWorkflowRuns(db.WorkflowRunListOpts{TaskID: id, Limit: 1})
				if len(runs) > 0 {
					workflowID = runs[0].WorkflowID
				}
			}
			if workflowID != "" {
				p, err = createShowPlanForWorkflow(id, workflowID, backend)
				if err != nil {
					// Fall back to weight-based display if workflow not found
					p = createShowPlanForWeightProto(id, t.Weight)
				}
			} else {
				p = createShowPlanForWeightProto(id, t.Weight)
			}

			// Merge phase states from task's execution state into the plan
			mergePhaseStatesProto(p, t)

			// --full enables everything
			if showFull {
				showSession = true
				showCost = true
				showReview = true
			}

			// Load spec if requested
			var spec *storage.PhaseOutputInfo
			if showSpec {
				spec, _ = backend.GetFullSpecForTask(id)
			}

			// Load review findings if requested
			var reviewFindings []*orcv1.ReviewRoundFindings
			if showReview {
				reviewFindings, _ = backend.LoadAllReviewFindings(id)
			}

			// JSON output
			if jsonOut {
				result := map[string]any{
					"task":      t,
					"plan":      p,
					"status":    t.Status,
					"execution": t.Execution,
				}
				if showSession && t.Execution != nil && t.Execution.Session != nil {
					result["session"] = t.Execution.Session
				}
				if showCost && t.Execution != nil {
					result["cost"] = map[string]any{
						"tokens": t.Execution.Tokens,
						"cost":   t.Execution.Cost,
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
				if showReview {
					result["review_findings"] = reviewFindings
				}
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Print task details
			fmt.Printf("\n%s - %s\n", t.Id, t.Title)
			fmt.Printf("────────────────────────────────────────────\n")
			fmt.Printf("Status:    %s\n", task.StatusFromProto(t.Status))
			fmt.Printf("Weight:    %s\n", task.WeightFromProto(t.Weight))
			fmt.Printf("Branch:    %s\n", t.Branch)
			if t.CreatedAt != nil {
				fmt.Printf("Created:   %s\n", t.CreatedAt.AsTime().Format(time.RFC3339))
			}

			if t.StartedAt != nil {
				fmt.Printf("Started:   %s\n", t.StartedAt.AsTime().Format(time.RFC3339))
			}
			if t.CompletedAt != nil {
				fmt.Printf("Completed: %s\n", t.CompletedAt.AsTime().Format(time.RFC3339))
			}

			if t.Description != nil && *t.Description != "" {
				fmt.Printf("\nDescription:\n%s\n", *t.Description)
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
			if t.Execution != nil && t.Execution.Tokens != nil && t.Execution.Tokens.TotalTokens > 0 {
				fmt.Printf("\nTokens Used: %d\n", t.Execution.Tokens.TotalTokens)
			}

			// Print session info if requested
			if showSession {
				printSessionInfoProto(t, id)
			}

			// Print cost info if requested
			if showCost {
				printCostInfoProto(t, id, period)
			}

			// Print spec info if requested
			if showSpec {
				printSpecInfo(spec)
			}

			// Print review findings if requested
			if showReview {
				printReviewFindings(reviewFindings)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showSession, "session", false, "include session information")
	cmd.Flags().BoolVar(&showCost, "cost", false, "include cost breakdown")
	cmd.Flags().BoolVar(&showSpec, "spec", false, "show specification content")
	cmd.Flags().BoolVar(&showReview, "review", false, "show review findings")
	cmd.Flags().BoolVar(&showFull, "full", false, "include all details (session + cost + review)")
	cmd.Flags().StringVarP(&period, "period", "p", "", "cost period filter (day, week, month) - only with --cost")

	return cmd
}

// printSessionInfoProto displays session information for a task (proto version).
func printSessionInfoProto(t *orcv1.Task, id string) {
	fmt.Printf("\nSession\n")
	fmt.Printf("─────────────────────────\n")

	if t.Execution == nil || t.Execution.Session == nil {
		fmt.Printf("No session information recorded.\n")
		fmt.Println("Session info is recorded after the task starts running.")
		return
	}

	session := t.Execution.Session
	fmt.Printf("Session ID:    %s\n", session.Id)
	fmt.Printf("Model:         %s\n", session.Model)
	fmt.Printf("Status:        %s\n", session.Status)
	fmt.Printf("Turn Count:    %d\n", session.TurnCount)
	if session.CreatedAt != nil {
		fmt.Printf("Created:       %s\n", session.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if session.LastActivity != nil {
		fmt.Printf("Last Activity: %s\n", session.LastActivity.AsTime().Format("2006-01-02 15:04:05"))
	}

	// Show resume hint if task is paused or blocked (task.Status is single source of truth)
	if t.Status == orcv1.TaskStatus_TASK_STATUS_PAUSED || t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		fmt.Println()
		fmt.Printf("To resume: orc resume %s\n", id)
	}
}

// printCostInfoProto displays cost information for a task (proto version).
func printCostInfoProto(t *orcv1.Task, _ string, _ string) {
	fmt.Printf("\nCost\n")
	fmt.Printf("─────────────────────────\n")

	if t.Execution == nil || t.Execution.Cost == nil {
		fmt.Printf("No cost information recorded.\n")
		return
	}

	fmt.Printf("Total Cost:    $%.4f\n", t.Execution.Cost.TotalCostUsd)
	fmt.Println()
	fmt.Println("Token Usage:")
	if t.Execution.Tokens != nil {
		fmt.Printf("  Input:       %d tokens\n", t.Execution.Tokens.InputTokens)
		fmt.Printf("  Output:      %d tokens\n", t.Execution.Tokens.OutputTokens)
		fmt.Printf("  Total:       %d tokens\n", t.Execution.Tokens.TotalTokens)
	}

	if len(t.Execution.Cost.PhaseCosts) > 0 {
		fmt.Println()
		fmt.Println("Cost by Phase:")
		for phase, cost := range t.Execution.Cost.PhaseCosts {
			fmt.Printf("  %-12s $%.4f\n", phase+":", cost)
		}
	}
}

// printSpecInfo displays specification content for a task.
func printSpecInfo(spec *storage.PhaseOutputInfo) {
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

// printReviewFindings displays review findings for a task.
func printReviewFindings(findings []*orcv1.ReviewRoundFindings) {
	fmt.Printf("\nReview Findings\n")
	fmt.Printf("─────────────────────────\n")

	if len(findings) == 0 {
		fmt.Printf("No review findings recorded.\n")
		fmt.Println("Review findings are generated during the 'review' phase.")
		return
	}

	for _, f := range findings {
		fmt.Printf("\nRound %d: %s\n", f.Round, f.Summary)

		if len(f.Issues) > 0 {
			fmt.Printf("\nIssues:\n")
			for _, issue := range f.Issues {
				icon := severityIcon(issue.Severity)
				fmt.Printf("  %s [%s] %s\n", icon, issue.Severity, issue.Description)
				if issue.File != nil && *issue.File != "" {
					if issue.Line != nil && *issue.Line > 0 {
						fmt.Printf("      %s:%d\n", *issue.File, *issue.Line)
					} else {
						fmt.Printf("      %s\n", *issue.File)
					}
				}
				if issue.Suggestion != nil && *issue.Suggestion != "" {
					fmt.Printf("      💡 %s\n", *issue.Suggestion)
				}
				if issue.ConstitutionViolation != nil && *issue.ConstitutionViolation != "" {
					if *issue.ConstitutionViolation == "invariant" {
						fmt.Printf("      ⚠️  Constitution: INVARIANT (must fix)\n")
					} else {
						fmt.Printf("      ⚠️  Constitution: %s\n", *issue.ConstitutionViolation)
					}
				}
			}
		}

		if len(f.Positives) > 0 {
			fmt.Printf("\nPositives:\n")
			for _, p := range f.Positives {
				fmt.Printf("  ✓ %s\n", p)
			}
		}

		if len(f.Questions) > 0 {
			fmt.Printf("\nQuestions:\n")
			for _, q := range f.Questions {
				fmt.Printf("  ? %s\n", q)
			}
		}
	}
}

// severityIcon returns an emoji for the severity level.
func severityIcon(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	case "low":
		return "🔵"
	default:
		return "⚪"
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

// createShowPlanForWorkflow creates an execution plan from actual workflow phases in the database.
func createShowPlanForWorkflow(taskID, workflowID string, backend storage.Backend) (*executor.Plan, error) {
	dbPhases, err := backend.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, err
	}
	if len(dbPhases) == 0 {
		return nil, fmt.Errorf("no phases found for workflow %s", workflowID)
	}

	phases := make([]executor.PhaseDisplay, 0, len(dbPhases))
	for _, wp := range dbPhases {
		// The phase template ID is the display ID (e.g., "implement", "review", "my-custom-phase")
		phases = append(phases, executor.PhaseDisplay{
			ID:     wp.PhaseTemplateID,
			Name:   wp.PhaseTemplateID, // Use template ID as name too
			Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
			Gate:   gate.Gate{Type: gate.GateAuto},
		})
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}, nil
}

// createShowPlanForWeightProto creates an execution plan based on task weight (proto version).
// Plans are created dynamically for display, not stored.
func createShowPlanForWeightProto(taskID string, weight orcv1.TaskWeight) *executor.Plan {
	var phases []executor.PhaseDisplay

	switch weight {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "breakdown", Name: "Breakdown", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	default:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}
}

// mergePhaseStatesProto updates plan phase statuses from the execution state (proto version).
func mergePhaseStatesProto(p *executor.Plan, t *orcv1.Task) {
	if t.Execution == nil || t.Execution.Phases == nil {
		return
	}
	for i := range p.Phases {
		ps, ok := t.Execution.Phases[p.Phases[i].ID]
		if !ok {
			continue
		}
		// Phase status is completion-only: PENDING, COMPLETED, SKIPPED
		switch ps.Status {
		case orcv1.PhaseStatus_PHASE_STATUS_COMPLETED:
			p.Phases[i].Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
			p.Phases[i].CommitSHA = ps.GetCommitSha()
		case orcv1.PhaseStatus_PHASE_STATUS_SKIPPED:
			p.Phases[i].Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
		default:
			// PENDING or any legacy value stays as PENDING
			p.Phases[i].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
		}
	}
}
