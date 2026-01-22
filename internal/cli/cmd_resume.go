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
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ResumeValidationResult contains the result of validating a task for resume.
type ResumeValidationResult struct {
	// IsOrphaned indicates the task was running but its executor died.
	IsOrphaned bool
	// OrphanReason explains why the task was detected as orphaned.
	OrphanReason string
	// RequiresStateUpdate indicates the task/state need updating before execution.
	RequiresStateUpdate bool
}

// ValidateTaskResumable checks if a task can be resumed and returns validation details.
// Returns an error if the task cannot be resumed.
func ValidateTaskResumable(t *task.Task, s *state.State, forceResume bool) (*ResumeValidationResult, error) {
	result := &ResumeValidationResult{}

	switch t.Status {
	case task.StatusPaused, task.StatusBlocked:
		// These are always resumable
		return result, nil
	case task.StatusRunning:
		// Check if it's orphaned
		isOrphaned, reason := s.CheckOrphaned()
		if isOrphaned {
			result.IsOrphaned = true
			result.OrphanReason = reason
			result.RequiresStateUpdate = true
			return result, nil
		} else if forceResume {
			result.RequiresStateUpdate = true
			return result, nil
		}
		return nil, fmt.Errorf("task is currently running (PID %d). Use --force to resume anyway", s.GetExecutorPID())
	case task.StatusFailed:
		// Allow resuming failed tasks
		return result, nil
	default:
		return nil, fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
	}
}

// ApplyResumeStateUpdates applies necessary state updates for orphaned or force-resumed tasks.
func ApplyResumeStateUpdates(t *task.Task, s *state.State, result *ResumeValidationResult, forceResume bool, backend storage.Backend) error {
	if !result.RequiresStateUpdate {
		return nil
	}

	if result.IsOrphaned {
		s.InterruptPhase(s.CurrentPhase)
	} else if forceResume {
		s.ClearExecution()
		s.InterruptPhase(s.CurrentPhase)
	}

	if err := backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	t.Status = task.StatusBlocked
	if err := backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	return nil
}

func newResumeCmd() *cobra.Command {
	var forceResume bool

	cmd := &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume a paused, blocked, interrupted, orphaned, or failed task",
		Long: `Resume a task that was paused, blocked, interrupted, failed, or became orphaned.

For tasks marked as "running" but whose executor process has died (orphaned),
this command will automatically mark them as interrupted and resume execution.

For failed tasks, this command will resume from the last incomplete phase,
allowing you to retry after fixing any issues.

Use --force to resume a task even if it appears to still be running.`,
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

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			s, err := backend.LoadState(id)
			if err != nil {
				// State might not exist, create new one
				s = state.New(id)
			}

			// Validate task is resumable
			validationResult, err := ValidateTaskResumable(t, s, forceResume)
			if err != nil {
				return err
			}

			// Print appropriate messages based on validation result
			if validationResult.IsOrphaned {
				fmt.Printf("Task %s appears orphaned (%s)\n", id, validationResult.OrphanReason)
				fmt.Println("Marking as interrupted and resuming...")
			} else if forceResume && validationResult.RequiresStateUpdate {
				fmt.Printf("Warning: Task %s may still be running (PID %d)\n", id, s.GetExecutorPID())
				fmt.Println("Force-resuming as requested...")
			} else if t.Status == task.StatusFailed {
				fmt.Printf("Task %s failed previously, resuming from last phase...\n", id)
			}

			// Apply state updates if needed
			if err := ApplyResumeStateUpdates(t, s, validationResult, forceResume, backend); err != nil {
				return err
			}

			// Create plan dynamically from task weight
			p := createResumePlanForWeight(id, t.Weight)

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Set up signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Resuming task %s", id))

			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)
			exec.SetBackend(backend)

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

			// Find resume phase with smart retry handling
			resumePhase := s.GetResumePhase()

			// If no interrupted/running phase, check retry context
			if resumePhase == "" {
				if rc := s.GetRetryContext(); rc != nil && rc.ToPhase != "" {
					resumePhase = rc.ToPhase
					fmt.Printf("Resuming from retry target: %s (failed at %s)\n", rc.ToPhase, rc.FromPhase)
				}
			}

			// For failed phases (e.g., review, test), use retry map to go back to earlier phase
			// This prevents loops where failed phases keep restarting from the failed phase
			if resumePhase == "" && s.CurrentPhase != "" {
				if ps, ok := s.Phases[s.CurrentPhase]; ok && ps.Status == state.StatusFailed {
					if retryFrom := cfg.ShouldRetryFrom(s.CurrentPhase); retryFrom != "" {
						resumePhase = retryFrom
						fmt.Printf("Using retry map: %s -> %s (retrying from earlier phase)\n", s.CurrentPhase, retryFrom)
					}
				}
			}

			// Final fallback to current phase
			if resumePhase == "" {
				resumePhase = s.CurrentPhase
			}

			if resumePhase == "" {
				return fmt.Errorf("no phase to resume from")
			}

			// Check for phase-specific session ID and pass to executor for --resume flag
			// Session IDs are tracked per-phase to ensure correct Claude context on resume
			if sessionID := s.GetPhaseSessionID(resumePhase); sessionID != "" {
				// Check if session is from this machine
				currentHost, _ := os.Hostname()
				if s.Execution != nil && s.Execution.Hostname != "" && s.Execution.Hostname != currentHost {
					disp.Warning(fmt.Sprintf("Task ran on different machine (%s), starting fresh session", s.Execution.Hostname))
				} else {
					disp.Info(fmt.Sprintf("Resuming Claude session for %s: %s", resumePhase, sessionID))
					exec.SetResumeSessionID(sessionID)
				}
			}

			err = exec.ResumeFromPhase(ctx, t, p, s, resumePhase)
			if err != nil {
				if ctx.Err() != nil {
					disp.TaskInterrupted()
					return nil
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
				fileStats = getResumeFileChangeStats(ctx, projectRoot, t.Branch, cfg)
			}

			disp.TaskComplete(s.Tokens.TotalTokens, s.Elapsed(), fileStats)
			return nil
		},
	}
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")
	cmd.Flags().BoolVarP(&forceResume, "force", "f", false, "force resume even if task appears to be running")
	return cmd
}

// getResumeFileChangeStats computes diff statistics for the task branch vs target branch.
// Returns nil if stats cannot be computed (not an error - just no stats to display).
func getResumeFileChangeStats(ctx context.Context, projectRoot, taskBranch string, cfg *config.Config) *progress.FileChangeStats {
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

// createResumePlanForWeight creates an execution plan based on task weight.
// Plans are created dynamically for execution, not stored.
func createResumePlanForWeight(taskID string, weight task.Weight) *executor.Plan {
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
