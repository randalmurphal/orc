// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/task"
)

// finalizeElapsedProto calculates elapsed time since task execution started.
func finalizeElapsedProto(t *orcv1.Task) time.Duration {
	return task.ElapsedProto(t)
}

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
			projectRoot, err := ResolveProjectPath()
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
			if err := validateFinalizeStateProto(t); err != nil {
				return err
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

			// Create git operations
			gitOps, err := NewGitOpsFromConfig(projectRoot, cfg)
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Build executor config (use task weight for appropriate settings)
			execCfg := executor.DefaultConfigForWeight(t.Weight)

			// Create finalize phase
			finalizePhase := &executor.PhaseDisplay{
				ID:     "finalize",
				Name:   "Finalize",
				Prompt: "Sync with target branch, resolve conflicts, run tests, and assess risk",
				Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
			}

			// Build FinalizeExecutor options
			opts := []executor.FinalizeExecutorOption{
				executor.WithFinalizeGitSvc(gitOps),
				executor.WithFinalizeLogger(slog.Default()),
				executor.WithFinalizeConfig(execCfg),
				executor.WithFinalizeOrcConfig(cfg),
				executor.WithFinalizeWorkingDir(projectRoot),
				executor.WithFinalizeBackend(backend),
				executor.WithFinalizeClaudePath(executor.ResolveClaudePath("claude")),
				executor.WithFinalizeExecutionUpdater(func(exec *orcv1.ExecutionState) {
					// Persist execution state updates during finalization
					t.Execution = exec
					if saveErr := backend.SaveTask(t); saveErr != nil {
						slog.Warn("failed to save task update", "error", saveErr)
					}
				}),
			}

			// Set up streaming publisher if verbose or --stream flag is set
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				opts = append(opts, executor.WithFinalizePublisher(publisher))
				defer publisher.Close()
			}

			// Create FinalizeExecutor
			finalizeExec := executor.NewFinalizeExecutor(opts...)

			// Execute finalize phase
			_, err = finalizeExec.Execute(ctx, t, finalizePhase, t.Execution)
			if err != nil {
				if ctx.Err() != nil {
					// Update task and execution state for clean interrupt
					task.EnsureExecutionProto(t)
					task.InterruptPhaseProto(t.Execution, "finalize")
					t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
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
					blockedCtx := buildBlockedContextProto(t, cfg, projectRoot)
					disp.TaskBlockedWithContext(task.GetTotalTokensProto(t), finalizeElapsedProto(t), "sync conflict", blockedCtx)
					return nil // Not a fatal error - task execution succeeded
				}

				disp.TaskFailed(err)
				return err
			}

			// Reload task for final display (execution state is in task.Execution)
			t, _ = backend.LoadTask(id)

			// Compute file change stats for completion summary
			var fileStats *progress.FileChangeStats
			if t.Branch != "" {
				fileStats = getFinalizeFileChangeStats(ctx, projectRoot, t.Branch, cfg)
			}

			disp.TaskComplete(task.GetTotalTokensProto(t), finalizeElapsedProto(t), fileStats)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip risk assessment and proceed even if risk is high")
	cmd.Flags().BoolVar(&skipRisk, "skip-risk", false, "skip the risk assessment step entirely")
	cmd.Flags().StringVar(&gateType, "gate", "", "override gate configuration (human, ai, none, auto)")
	cmd.Flags().BoolVar(&stream, "stream", false, "stream Claude transcript to stdout")

	return cmd
}

// validateFinalizeStateProto checks if the task is in a valid state for finalize.
func validateFinalizeStateProto(t *orcv1.Task) error {
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return fmt.Errorf("task %s is already completed", t.Id)
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return fmt.Errorf("task %s is currently running - pause it first if you want to run finalize manually", t.Id)
	case orcv1.TaskStatus_TASK_STATUS_CREATED, orcv1.TaskStatus_TASK_STATUS_PLANNED:
		// Allow finalize on created/planned tasks (e.g., after manual implementation)
		return nil
	case orcv1.TaskStatus_TASK_STATUS_PAUSED, orcv1.TaskStatus_TASK_STATUS_BLOCKED, orcv1.TaskStatus_TASK_STATUS_FAILED:
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
