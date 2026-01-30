// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

func init() {
	rootCmd.AddCommand(runsCmd)
	runsCmd.AddCommand(runShowCmd)
	runsCmd.AddCommand(newRunCancelCmd())

	// List flags
	runsCmd.Flags().Bool("running", false, "Show only running workflows")
	runsCmd.Flags().String("workflow", "", "Filter by workflow ID")
	runsCmd.Flags().String("task", "", "Filter by task ID")
	runsCmd.Flags().Int("limit", 20, "Maximum number of runs to show")
}

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List workflow runs",
	Long: `List recent workflow runs with their status and metrics.

Workflow runs are execution instances - each time you run a workflow
(e.g., 'orc run implement "Add feature"'), a new run is created.

Examples:
  orc runs                     # List recent runs
  orc runs --running           # Show only running workflows
  orc runs --workflow medium   # Filter by workflow type
  orc runs --task TASK-001     # Show runs for a specific task`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		runningOnly, _ := cmd.Flags().GetBool("running")
		workflowFilter, _ := cmd.Flags().GetString("workflow")
		taskFilter, _ := cmd.Flags().GetString("task")
		limit, _ := cmd.Flags().GetInt("limit")

		opts := db.WorkflowRunListOpts{
			Limit: limit,
		}
		if runningOnly {
			opts.Status = string(workflow.RunStatusRunning)
		}
		if workflowFilter != "" {
			opts.WorkflowID = workflowFilter
		}
		if taskFilter != "" {
			opts.TaskID = taskFilter
		}

		runs, err := pdb.ListWorkflowRuns(opts)
		if err != nil {
			return fmt.Errorf("list runs: %w", err)
		}

		if len(runs) == 0 {
			fmt.Println("No workflow runs found.")
			return nil
		}

		// Display as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "RUN ID\tWORKFLOW\tSTATUS\tTASK\tSTARTED\tCOST")
		for _, run := range runs {
			taskID := "-"
			if run.TaskID != nil {
				taskID = *run.TaskID
			}
			started := "-"
			if run.StartedAt != nil {
				started = formatRelativeTime(*run.StartedAt)
			}
			cost := fmt.Sprintf("$%.4f", run.TotalCostUSD)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				run.ID, run.WorkflowID, run.Status, taskID, started, cost)
		}
		_ = w.Flush()

		return nil
	},
}

var runShowCmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show workflow run details",
	Long: `Display detailed information about a workflow run including
phase status, metrics, and any errors.

Examples:
  orc runs show RUN-001`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runID := args[0]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		run, err := pdb.GetWorkflowRun(runID)
		if err != nil {
			return fmt.Errorf("get workflow run: %w", err)
		}
		if run == nil {
			return fmt.Errorf("workflow run not found: %s", runID)
		}

		// Display run info
		fmt.Printf("Run ID: %s\n", run.ID)
		fmt.Printf("Workflow: %s\n", run.WorkflowID)
		fmt.Printf("Status: %s\n", run.Status)
		fmt.Printf("Context Type: %s\n", run.ContextType)
		if run.TaskID != nil {
			fmt.Printf("Task: %s\n", *run.TaskID)
		}
		if run.CurrentPhase != "" {
			fmt.Printf("Current Phase: %s\n", run.CurrentPhase)
		}
		if run.StartedAt != nil {
			fmt.Printf("Started: %s\n", run.StartedAt.Format(time.RFC3339))
		}
		if run.CompletedAt != nil {
			fmt.Printf("Completed: %s\n", run.CompletedAt.Format(time.RFC3339))
		}
		if run.Error != "" {
			fmt.Printf("Error: %s\n", run.Error)
		}

		// Metrics
		fmt.Println("\nMetrics:")
		fmt.Printf("  Total Cost: $%.4f\n", run.TotalCostUSD)
		fmt.Printf("  Input Tokens: %d\n", run.TotalInputTokens)
		fmt.Printf("  Output Tokens: %d\n", run.TotalOutputTokens)

		// Show prompt
		if run.Prompt != "" {
			fmt.Println("\nPrompt:")
			if len(run.Prompt) > 200 {
				fmt.Printf("  %s...\n", run.Prompt[:200])
			} else {
				fmt.Printf("  %s\n", run.Prompt)
			}
		}
		if run.Instructions != "" {
			fmt.Println("\nInstructions:")
			if len(run.Instructions) > 200 {
				fmt.Printf("  %s...\n", run.Instructions[:200])
			} else {
				fmt.Printf("  %s\n", run.Instructions)
			}
		}

		// Display phase status
		phases, err := pdb.GetWorkflowRunPhases(runID)
		if err != nil {
			return fmt.Errorf("get run phases: %w", err)
		}

		if len(phases) > 0 {
			fmt.Println("\nPhases:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "  PHASE\tSTATUS\tITER\tCOST\tDURATION")
			for _, p := range phases {
				duration := "-"
				if p.StartedAt != nil && p.CompletedAt != nil {
					duration = p.CompletedAt.Sub(*p.StartedAt).Round(time.Second).String()
				}
				cost := fmt.Sprintf("$%.4f", p.CostUSD)
				_, _ = fmt.Fprintf(w, "  %s\t%s\t%d\t%s\t%s\n",
					p.PhaseTemplateID, p.Status, p.Iterations, cost, duration)
			}
			_ = w.Flush()
		}

		return nil
	},
}

// newRunCancelCmd creates the runs cancel command.
func newRunCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running workflow",
		Long: `Cancel a workflow that is currently running.

This will stop the current phase execution and mark the run as cancelled.

Examples:
  orc runs cancel RUN-001`,
		Args: cobra.ExactArgs(1),
		RunE: runCancelRunE,
	}
}

func runCancelRunE(cmd *cobra.Command, args []string) error {
	runID := args[0]

	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return err
	}

	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	run, err := pdb.GetWorkflowRun(runID)
	if err != nil {
		return fmt.Errorf("get workflow run: %w", err)
	}
	if run == nil {
		return fmt.Errorf("workflow run not found: %s", runID)
	}

	if run.Status != string(workflow.RunStatusRunning) &&
		run.Status != string(workflow.RunStatusPending) {
		return fmt.Errorf("cannot cancel run with status: %s", run.Status)
	}

	// Update status
	run.Status = string(workflow.RunStatusCancelled)
	run.Error = "cancelled by user"
	now := time.Now()
	run.CompletedAt = &now

	if err := pdb.SaveWorkflowRun(run); err != nil {
		return fmt.Errorf("save workflow run: %w", err)
	}

	// Signal the executor process if there's a linked task with a live PID
	processSignaled := false
	if run.TaskID != nil {
		backend, backendErr := getBackend()
		if backendErr == nil {
			defer func() { _ = backend.Close() }()

			t, loadErr := backend.LoadTask(*run.TaskID)
			if loadErr != nil {
				// Log warning but don't fail - task may have been deleted
				cmd.Printf("Warning: Could not load linked task %s: %v\n", *run.TaskID, loadErr)
			} else if t != nil && t.ExecutorPid > 0 {
				if task.IsPIDAlive(int(t.ExecutorPid)) {
					proc, procErr := os.FindProcess(int(t.ExecutorPid))
					if procErr == nil {
						if sigErr := proc.Signal(syscall.SIGTERM); sigErr != nil {
							// Log warning but don't fail - process may have just exited
							cmd.Printf("Warning: Could not signal executor (PID %d): %v\n", t.ExecutorPid, sigErr)
						} else {
							processSignaled = true
						}
					}
				}
			}
		}
	}

	cmd.Printf("Cancelled workflow run '%s'\n", runID)
	if processSignaled {
		cmd.Println("Executor process has been signaled to terminate.")
	} else {
		cmd.Println("Note: The running Claude session may still need to be terminated manually.")
	}
	return nil
}

// formatRelativeTime formats a time as relative (e.g., "2 hours ago")
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
