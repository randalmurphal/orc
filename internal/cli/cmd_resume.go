// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
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

			// Atomically claim task execution to prevent race conditions
			hostname, _ := os.Hostname()
			ctx := context.Background()
			if err := backend.TryClaimTaskExecution(ctx, id, os.Getpid(), hostname); err != nil {
				// Check if this is a "already claimed" error
				if strings.Contains(err.Error(), "already claimed") {
					return fmt.Errorf("task is already being executed by another process")
				}
				return fmt.Errorf("claim task execution: %w", err)
			}

			// Get workflow ID from task weight
			workflowID := workflow.GetWorkflowForWeight(string(t.Weight))

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Open project database for workflows
			pdb, err := db.OpenProject(projectRoot)
			if err != nil {
				return fmt.Errorf("open project database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			// Seed built-in workflows if not already seeded
			if _, err := workflow.SeedBuiltins(pdb); err != nil {
				return fmt.Errorf("seed workflows: %w", err)
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

			// Create WorkflowExecutor
			gitOps, err := git.New(projectRoot, git.DefaultConfig())
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Build executor options
			execOpts := []executor.WorkflowExecutorOption{
				executor.WithWorkflowGitOps(gitOps),
			}

			// Create persistent publisher for database event logging
			persistentPub := events.NewPersistentPublisher(backend, "cli", nil)
			defer persistentPub.Close()

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				cliPub := events.NewCLIPublisher(os.Stdout,
					events.WithStreamMode(true),
					events.WithInnerPublisher(persistentPub),
				)
				execOpts = append(execOpts, executor.WithWorkflowPublisher(cliPub))
				defer cliPub.Close()
			} else {
				execOpts = append(execOpts, executor.WithWorkflowPublisher(persistentPub))
			}

			we := executor.NewWorkflowExecutor(
				backend,
				pdb,
				cfg,
				projectRoot,
				execOpts...,
			)

			// Build run options
			opts := executor.WorkflowRunOptions{
				ContextType: executor.ContextTask,
				TaskID:      id,
				Prompt:      t.Description,
				Category:    t.Category,
			}

			// Execute workflow (WorkflowExecutor handles resume internally via state)
			result, err := we.Run(ctx, workflowID, opts)
			if err != nil {
				if ctx.Err() != nil {
					disp.TaskInterrupted()
					return nil
				}

				// Check if task is blocked (phases succeeded but completion failed)
				if errors.Is(err, executor.ErrTaskBlocked) {
					// Reload task and state for summary
					t, _ = backend.LoadTask(id)
					s, _ = backend.LoadState(id)
					blockedCtx := buildBlockedContext(t, cfg)
					var tokens int
					if s != nil {
						tokens = s.Tokens.TotalTokens
					}
					disp.TaskBlockedWithContext(tokens, s.Elapsed(), "sync conflict", blockedCtx)
					return nil // Not a fatal error - task execution succeeded
				}

				disp.TaskFailed(err)
				return err
			}

			// Reload state for summary
			s, _ = backend.LoadState(id)

			// Compute file change stats for completion summary
			var fileStats *progress.FileChangeStats
			if t.Branch != "" {
				fileStats = getResumeFileChangeStats(ctx, projectRoot, t.Branch, cfg)
			}

			var tokens int
			if s != nil {
				tokens = s.Tokens.TotalTokens
			}
			_ = result // Result contains run details but we use state for tokens
			disp.TaskComplete(tokens, s.Elapsed(), fileStats)
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

