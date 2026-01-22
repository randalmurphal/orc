// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/task"
)

// newFinalizeCmd creates the finalize command
func newFinalizeCmd() *cobra.Command {
	var (
		force    bool
		gateType string
		stream   bool
		skipRisk bool
	)

	cmd := &cobra.Command{
		Use:   "finalize <task-id>",
		Short: "Run finalize phase for a task",
		Long: `Manually trigger the finalize phase for a task.

The finalize phase syncs the task branch with the target branch,
resolves any conflicts, runs tests, and performs risk assessment.

This command is useful when:
  - You want to manually prepare a task for merge
  - You need to sync with the latest changes from the target branch
  - The finalize phase was skipped or needs to be re-run

Options:
  --force       Skip risk assessment and proceed even if risk is high
  --skip-risk   Skip the risk assessment step entirely
  --gate        Override the gate configuration (human, ai, none, auto)

Example:
  orc finalize TASK-001
  orc finalize TASK-001 --force
  orc finalize TASK-001 --gate human
  orc finalize TASK-001 --skip-risk`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find the project root (handles worktrees)
			projectRoot, err := config.FindProjectRoot()
			if err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			// Load task
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if task is in a valid state for finalize
			if err := validateFinalizeState(t); err != nil {
				return err
			}

			// Create plan dynamically from task weight
			p := createPlanForWeight(id, t.Weight)

			// Load state
			s, err := backend.LoadState(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Apply gate override if specified
			if gateType != "" {
				if !isValidGateType(gateType) {
					return fmt.Errorf("invalid gate type: %s (must be human, ai, none, or auto)", gateType)
				}
				cfg.Completion.Finalize.Gates.PreMerge = gateType
			}

			// Apply force/skip-risk overrides
			if force || skipRisk {
				cfg.Completion.Finalize.RiskAssessment.Enabled = false
			}

			// Set up signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			// Create progress display
			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Starting finalize phase for %s", id))

			// Create executor with config
			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)
			exec.SetBackend(backend)

			// Set up streaming publisher if verbose or --stream flag is set
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

			// Get or create finalize phase
			finalizePhase := getFinalizePhase(p)

			// Execute finalize phase
			err = exec.FinalizeTask(ctx, t, finalizePhase, s)
			if err != nil {
				if ctx.Err() != nil {
					// Update task and state status for clean interrupt
					s.InterruptPhase("finalize")
					if saveErr := backend.SaveState(s); saveErr != nil {
						disp.Warning(fmt.Sprintf("failed to save state on interrupt: %v", saveErr))
					}
					t.Status = task.StatusBlocked
					if saveErr := backend.SaveTask(t); saveErr != nil {
						disp.Warning(fmt.Sprintf("failed to save task on interrupt: %v", saveErr))
					}
					disp.TaskInterrupted()
					return nil // Clean interrupt
				}

				// Check if task is blocked (phases succeeded but completion failed)
				if errors.Is(err, executor.ErrTaskBlocked) {
					// Reload task to get updated metadata with conflict info
					t, _ = backend.LoadTask(id)
					blockedCtx := buildBlockedContext(t, cfg)
					disp.TaskBlockedWithContext(s.Tokens.TotalTokens, s.Elapsed(), "sync conflict", blockedCtx)
					return nil // Not a fatal error - task execution succeeded
				}

				disp.TaskFailed(err)
				return err
			}

			// Compute file change stats for completion summary
			var fileStats *progress.FileChangeStats
			if t.Branch != "" {
				fileStats = getFinalizeFileChangeStats(ctx, projectRoot, t.Branch, cfg)
			}

			disp.TaskComplete(s.Tokens.TotalTokens, s.Elapsed(), fileStats)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip risk assessment and proceed even if risk is high")
	cmd.Flags().BoolVar(&skipRisk, "skip-risk", false, "skip the risk assessment step entirely")
	cmd.Flags().StringVar(&gateType, "gate", "", "override gate configuration (human, ai, none, auto)")
	cmd.Flags().BoolVar(&stream, "stream", false, "stream Claude transcript to stdout")

	return cmd
}

// validateFinalizeState checks if the task is in a valid state for finalize.
func validateFinalizeState(t *task.Task) error {
	switch t.Status {
	case task.StatusCompleted:
		return fmt.Errorf("task %s is already completed", t.ID)
	case task.StatusRunning:
		return fmt.Errorf("task %s is currently running - pause it first if you want to run finalize manually", t.ID)
	case task.StatusCreated, task.StatusPlanned:
		// Allow finalize on created/planned tasks (e.g., after manual implementation)
		return nil
	case task.StatusPaused, task.StatusBlocked, task.StatusFailed:
		// These states are allowed for finalize
		return nil
	default:
		return nil
	}
}

// isValidGateType checks if the gate type is valid.
func isValidGateType(gt string) bool {
	validTypes := []string{"human", "ai", "none", "auto"}
	for _, v := range validTypes {
		if gt == v {
			return true
		}
	}
	return false
}

// getFinalizePhase returns the finalize phase from the plan, or creates one if not present.
func getFinalizePhase(p *executor.Plan) *executor.Phase {
	// First try to find existing finalize phase
	for i := range p.Phases {
		if p.Phases[i].ID == "finalize" {
			return &p.Phases[i]
		}
	}

	// Create a new finalize phase if not in plan
	return &executor.Phase{
		ID:     "finalize",
		Name:   "Finalize",
		Prompt: "Sync with target branch, resolve conflicts, run tests, and assess risk",
		Status: executor.PhasePending,
		Gate:   gate.Gate{Type: gate.GateAuto},
	}
}

// getFinalizeFileChangeStats computes diff statistics for the task branch vs target branch.
// Returns nil if stats cannot be computed (not an error - just no stats to display).
func getFinalizeFileChangeStats(ctx context.Context, projectRoot, taskBranch string, cfg *config.Config) *progress.FileChangeStats {
	// Determine target branch from config
	targetBranch := "main"
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		targetBranch = cfg.Completion.TargetBranch
	}

	// Create diff service to compute stats
	diffSvc := diff.NewService(projectRoot, nil)

	// Resolve target branch (handles origin/main fallback)
	resolvedBase := diffSvc.ResolveRef(ctx, targetBranch)

	// Get diff stats between target branch and task branch
	stats, err := diffSvc.GetStats(ctx, resolvedBase, taskBranch)
	if err != nil {
		// Diff stat computation is best-effort - don't fail task completion
		return nil
	}

	return &progress.FileChangeStats{
		FilesChanged: stats.FilesChanged,
		Additions:    stats.Additions,
		Deletions:    stats.Deletions,
	}
}

// createPlanForWeight creates an execution plan based on task weight.
// Plans are created dynamically for execution, not stored.
func createPlanForWeight(taskID string, weight task.Weight) *executor.Plan {
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
