// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
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

			// Create git operations
			gitOps, err := git.New(projectRoot, git.DefaultConfig())
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Build executor config (use task weight for appropriate settings)
			execCfg := executor.DefaultConfigForWeight(t.Weight)

			// Create finalize phase
			finalizePhase := &executor.Phase{
				ID:     "finalize",
				Name:   "Finalize",
				Prompt: "Sync with target branch, resolve conflicts, run tests, and assess risk",
				Status: executor.PhasePending,
			}

			// Build FinalizeExecutor options
			opts := []executor.FinalizeExecutorOption{
				executor.WithFinalizeGitSvc(gitOps),
				executor.WithFinalizeLogger(slog.Default()),
				executor.WithFinalizeConfig(execCfg),
				executor.WithFinalizeOrcConfig(cfg),
				executor.WithFinalizeWorkingDir(projectRoot),
				executor.WithFinalizeTaskDir(task.TaskDirIn(projectRoot, id)),
				executor.WithFinalizeBackend(backend),
				executor.WithFinalizeClaudePath(executor.ResolveClaudePath("claude")),
				executor.WithFinalizeStateUpdater(func(updatedState *state.State) {
					// Persist state updates during finalization
					if saveErr := backend.SaveState(updatedState); saveErr != nil {
						slog.Warn("failed to save state update", "error", saveErr)
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
			_, err = finalizeExec.Execute(ctx, t, finalizePhase, s)
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
		// Provide context-specific error based on PR status
		if t.HasPR() {
			prNum := t.PR.Number
			if t.PR.Merged {
				return fmt.Errorf("task %s is already completed - PR #%d was merged", t.ID, prNum)
			}
			// PR exists but not merged yet
			return fmt.Errorf("task %s is already completed - PR #%d is open. To merge: gh pr merge %d", t.ID, prNum, prNum)
		}
		// No PR exists
		return fmt.Errorf("task %s is already completed - No PR was created (task may have been completed with --no-pr flag)", t.ID)
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
