// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newRunCmd creates the run command
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Execute a task",
		Long: `Execute a task through its phases.

The task will be executed according to its plan (based on weight).
Each phase creates a git checkpoint for rewindability.

Automation profiles control gate behavior:
  auto   - Fully automated, no human intervention (default)
  fast   - Minimal gates, speed over safety
  safe   - AI reviews, human approval only for merge
  strict - Human gates on spec/review/merge

Artifact detection:
  When artifacts from previous runs exist (e.g., spec.md), orc will prompt
  whether to skip that phase. Use --auto-skip to skip automatically.

Example:
  orc run TASK-001
  orc run TASK-001 --profile safe
  orc run TASK-001 --auto-skip         # skip phases with existing artifacts
  orc run TASK-001 --phase implement   # run specific phase`,
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
			profile, _ := cmd.Flags().GetString("profile")
			force, _ := cmd.Flags().GetBool("force")

			// Load task
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check for incomplete blockers
			if len(t.BlockedBy) > 0 {
				// Load all tasks to check blocker status
				allTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks for dependency check: %w", err)
				}

				// Build task map
				taskMap := make(map[string]*task.Task)
				for _, tsk := range allTasks {
					taskMap[tsk.ID] = tsk
				}

				// Get incomplete blockers
				blockers := t.GetIncompleteBlockers(taskMap)
				if len(blockers) > 0 {
					if !force {
						fmt.Printf("\n⚠️  This task is blocked by incomplete tasks:\n")
						for _, b := range blockers {
							fmt.Printf("    - %s: %s (%s)\n", b.ID, b.Title, b.Status)
						}
						fmt.Println()

						if quiet {
							// In quiet mode, refuse to run blocked tasks without --force
							return fmt.Errorf("task is blocked by incomplete dependencies (use --force to override)")
						}

						// Prompt for confirmation
						fmt.Print("Run anyway? [y/N]: ")
						reader := bufio.NewReader(os.Stdin)
						input, err := reader.ReadString('\n')
						if err != nil {
							return fmt.Errorf("read input: %w", err)
						}
						input = strings.TrimSpace(strings.ToLower(input))
						if input != "y" && input != "yes" {
							fmt.Println("Aborted.")
							return nil
						}
						fmt.Println()
					}
				}
			}

			// Check if task can run
			if !t.CanRun() && t.Status != task.StatusRunning {
				// Provide helpful error message based on status
				switch t.Status {
				case task.StatusPaused:
					fmt.Printf("Task %s is paused.\n\n", id)
					fmt.Printf("To resume:  orc resume %s\n", id)
					fmt.Printf("To restart: orc rewind %s --to <phase>\n", id)
					return nil
				case task.StatusBlocked:
					fmt.Printf("Task %s is blocked and needs user input.\n\n", id)
					fmt.Println("Check the task for pending questions or approvals.")
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusCompleted:
					fmt.Printf("Task %s is already completed.\n\n", id)
					fmt.Printf("To rerun:   orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusFailed:
					fmt.Printf("Task %s has failed.\n\n", id)
					fmt.Printf("To resume:  orc resume %s\n", id)
					fmt.Printf("To restart: orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc log %s\n", id)
					return nil
				default:
					return fmt.Errorf("task cannot be run (status: %s)", t.Status)
				}
			}

			// Load plan
			p, err := backend.LoadPlan(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			// Load or create state
			s, err := backend.LoadState(id)
			if err != nil {
				// State might not exist, create new one
				s = state.New(id)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Apply profile if specified
			if profile != "" {
				cfg.ApplyProfile(config.AutomationProfile(profile))
			}

			// Handle artifact detection and skip prompting
			autoSkip, _ := cmd.Flags().GetBool("auto-skip")
			if cfg.ArtifactSkip.Enabled {
				// Override config with flag if specified
				if autoSkip {
					cfg.ArtifactSkip.AutoSkip = true
				}

				// Detect artifacts and prompt/skip as appropriate
				taskDir := task.TaskDirIn(projectRoot, id)
				detector := executor.NewArtifactDetectorWithDir(taskDir, id, t.Weight)

				// Get phase IDs from plan
				phaseIDs := make([]string, len(p.Phases))
				for i, phase := range p.Phases {
					phaseIDs[i] = phase.ID
				}

				// Check each configured phase for artifacts
				for _, phaseID := range cfg.ArtifactSkip.Phases {
					// Skip if phase not in plan or already completed
					if !containsPhase(phaseIDs, phaseID) || s.IsPhaseCompleted(phaseID) {
						continue
					}

					status := detector.DetectPhaseArtifacts(phaseID)
					if status.HasArtifacts && status.CanAutoSkip {
						shouldSkip := cfg.ArtifactSkip.AutoSkip

						if !shouldSkip && !quiet {
							// Prompt user
							fmt.Printf("\n📄 %s already exists. Skip %s phase? [Y/n]: ",
								strings.Join(status.Artifacts, ", "), phaseID)
							reader := bufio.NewReader(os.Stdin)
							input, err := reader.ReadString('\n')
							if err != nil {
								// On EOF or error, don't skip - safer to run the phase
								shouldSkip = false
							} else {
								input = strings.TrimSpace(strings.ToLower(input))
								shouldSkip = input == "" || input == "y" || input == "yes"
							}
						}

						if shouldSkip {
							reason := fmt.Sprintf("artifact exists: %s", status.Description)
							s.SkipPhase(phaseID, reason)

							// Also update plan status
							if phase := p.GetPhase(phaseID); phase != nil {
								phase.Status = plan.PhaseSkipped
							}

							if !quiet {
								fmt.Printf("⊘ Skipping %s phase: %s\n", phaseID, reason)
							}
						}
					}
				}

				// Save state and plan if any phases were skipped
				if err := backend.SaveState(s); err != nil {
					return fmt.Errorf("save state after artifact skip: %w", err)
				}
				if err := backend.SavePlan(p, id); err != nil {
					return fmt.Errorf("save plan after artifact skip: %w", err)
				}
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
			disp.Info(fmt.Sprintf("Starting task %s (%s) [profile: %s]", id, t.Weight, cfg.Profile))

			// Create executor with config
			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)
			exec.SetBackend(backend)

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

			// Execute task
			err = exec.ExecuteTask(ctx, t, p, s)
			if err != nil {
				if ctx.Err() != nil {
					// Update task and state status for clean interrupt
					s.InterruptPhase(s.CurrentPhase)
					if saveErr := backend.SaveState(s); saveErr != nil {
						// Log but continue - we're in cleanup mode
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
				fileStats = getFileChangeStats(ctx, projectRoot, t.Branch, cfg)
			}

			disp.TaskComplete(s.Tokens.TotalTokens, s.Elapsed(), fileStats)
			return nil
		},
	}
	cmd.Flags().String("phase", "", "run specific phase only")
	cmd.Flags().StringP("profile", "p", "", "automation profile (auto, fast, safe, strict)")
	cmd.Flags().Bool("continue", false, "continue from last checkpoint")
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")
	cmd.Flags().Bool("auto-skip", false, "automatically skip phases with existing artifacts")
	cmd.Flags().BoolP("force", "f", false, "run even if task has incomplete blockers")
	return cmd
}

// containsPhase checks if a phase ID is in the list.
func containsPhase(phases []string, phaseID string) bool {
	for _, p := range phases {
		if p == phaseID {
			return true
		}
	}
	return false
}

// getFileChangeStats computes diff statistics for the task branch vs target branch.
// Returns nil if stats cannot be computed (not an error - just no stats to display).
func getFileChangeStats(ctx context.Context, projectRoot, taskBranch string, cfg *config.Config) *progress.FileChangeStats {
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

// buildBlockedContext builds a BlockedContext from the task and config for enhanced
// conflict resolution guidance. It extracts worktree path, conflict files from task
// metadata, and sync strategy from config.
func buildBlockedContext(t *task.Task, cfg *config.Config) *progress.BlockedContext {
	ctx := &progress.BlockedContext{}

	// Get worktree path from task ID and config
	if cfg != nil && cfg.Worktree.Enabled {
		// Construct worktree path using config's worktree directory
		worktreeDir := cfg.Worktree.Dir
		if worktreeDir == "" {
			worktreeDir = ".orc/worktrees"
		}
		ctx.WorktreePath = worktreeDir + "/orc-" + t.ID
	}

	// Extract conflict files from task metadata if available
	if t.Metadata != nil {
		if errStr, ok := t.Metadata["blocked_error"]; ok {
			ctx.ConflictFiles = parseConflictFilesFromError(errStr)
		}
	}

	// Set sync strategy based on config
	if cfg != nil {
		if cfg.Completion.Finalize.Sync.Strategy == config.FinalizeSyncMerge {
			ctx.SyncStrategy = progress.SyncStrategyMerge
		} else {
			ctx.SyncStrategy = progress.SyncStrategyRebase
		}

		// Set target branch
		ctx.TargetBranch = cfg.Completion.TargetBranch
		if ctx.TargetBranch == "" {
			ctx.TargetBranch = "main"
		}
	}

	return ctx
}

// parseConflictFilesFromError extracts conflict file paths from an error message.
// Error messages contain conflict files in format: "[file1 file2 ...]" or similar patterns.
func parseConflictFilesFromError(errStr string) []string {
	var files []string

	// Look for file list in brackets: [file1 file2 file3]
	// This matches the format from ErrSyncConflict error messages
	startBracket := strings.Index(errStr, "[")
	endBracket := strings.LastIndex(errStr, "]")

	if startBracket >= 0 && endBracket > startBracket {
		fileListStr := errStr[startBracket+1 : endBracket]
		// Split by space, handling empty strings
		for _, f := range strings.Fields(fileListStr) {
			// Clean up any extra punctuation
			f = strings.Trim(f, ",")
			if f != "" {
				files = append(files, f)
			}
		}
	}

	return files
}
