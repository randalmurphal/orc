// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// orphanedWorktree holds information about an orphaned worktree.
type orphanedWorktree struct {
	TaskID string
	Path   string
	Status task.Status // Empty if task doesn't exist
	Reason string      // Why it's considered orphaned
}

// newCleanupCmd creates the cleanup command
func newCleanupCmd() *cobra.Command {
	var dryRun bool
	var all bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up orphaned worktrees",
		Long: `Clean up worktrees for tasks that are in terminal states.

By default, cleans worktrees for tasks that are completed.

Use --all to also clean worktrees for failed tasks.

Examples:
  orc cleanup              # Clean orphaned worktrees
  orc cleanup --dry-run    # Show what would be cleaned
  orc cleanup --all        # Also clean failed task worktrees`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find the project root (handles worktrees)
			projectRoot, err := config.FindProjectRoot()
			if err != nil {
				return err
			}

			if err := config.RequireInitAt(projectRoot); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Load config for git settings
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			// Initialize git operations
			gitCfg := git.Config{
				BranchPrefix:   cfg.BranchPrefix,
				CommitPrefix:   cfg.CommitPrefix,
				WorktreeDir:    cfg.Worktree.Dir,
				ExecutorPrefix: cfg.ExecutorPrefix(),
			}
			gitOps, err := git.New(projectRoot, gitCfg)
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Find orphaned worktrees
			orphans, err := findOrphanedWorktrees(gitOps, backend, all)
			if err != nil {
				return fmt.Errorf("find orphaned worktrees: %w", err)
			}

			if len(orphans) == 0 {
				if !quiet {
					fmt.Println("No orphaned worktrees found.")
				}
				return nil
			}

			// Display what will be cleaned
			if !quiet {
				if dryRun {
					fmt.Println("Would clean the following worktrees:")
				} else {
					fmt.Println("Cleaning worktrees:")
				}
				fmt.Println()
			}

			cleanedCount := 0
			failedCount := 0
			for _, o := range orphans {
				statusStr := string(o.Status)
				if statusStr == "" {
					statusStr = "unknown"
				}

				if !quiet {
					fmt.Printf("  %s (%s) - %s\n", o.TaskID, statusStr, o.Reason)
					fmt.Printf("    Path: %s\n", o.Path)
				}

				if dryRun {
					cleanedCount++
					continue
				}

				// Clean up Playwright user data directory first
				if err := executor.CleanupPlaywrightUserData(o.TaskID); err != nil {
					if !quiet {
						fmt.Printf("    ⚠️  Failed to cleanup playwright data: %v\n", err)
					}
				}

				// Remove the worktree
				if err := gitOps.CleanupWorktree(o.TaskID); err != nil {
					if !quiet {
						fmt.Printf("    ❌ Failed: %v\n", err)
					}
					failedCount++
				} else {
					if !quiet {
						fmt.Printf("    ✓ Cleaned\n")
					}
					cleanedCount++
				}
			}

			// Summary
			if !quiet {
				fmt.Println()
				if dryRun {
					fmt.Printf("Would clean %d worktree(s)\n", cleanedCount)
				} else {
					fmt.Printf("Cleaned %d worktree(s)", cleanedCount)
					if failedCount > 0 {
						fmt.Printf(", %d failed", failedCount)
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be cleaned without actually cleaning")
	cmd.Flags().BoolVar(&all, "all", false, "also clean worktrees for failed tasks")

	return cmd
}

// findOrphanedWorktrees finds worktrees that should be cleaned up.
func findOrphanedWorktrees(gitOps *git.Git, backend interface {
	LoadTask(id string) (*task.Task, error)
}, includeFailed bool) ([]orphanedWorktree, error) {
	var orphans []orphanedWorktree

	// Get list of worktrees from git
	ctx := gitOps.Context()
	output, err := ctx.RunGit("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	// Parse worktree list output
	// Format:
	// worktree /path/to/worktree
	// HEAD <sha>
	// branch refs/heads/<branch>
	//
	// worktree /path/to/next
	// ...

	// Extract task IDs from worktree paths
	// Pattern: orc-TASK-XXX or <prefix>-TASK-XXX
	taskIDPattern := regexp.MustCompile(`(?:orc-|^)(TASK-\d+)$`)

	worktrees := parseWorktreeList(output)
	for _, wt := range worktrees {
		// Skip the main worktree (no orc- prefix)
		if !strings.Contains(wt.path, "orc-TASK-") && !strings.Contains(wt.path, "/TASK-") {
			continue
		}

		// Extract task ID from path
		baseName := strings.TrimSuffix(wt.path, "/")
		if idx := strings.LastIndex(baseName, "/"); idx != -1 {
			baseName = baseName[idx+1:]
		}

		matches := taskIDPattern.FindStringSubmatch(baseName)
		if len(matches) < 2 {
			continue
		}
		taskID := matches[1]

		// Check if worktree path still exists
		if _, statErr := os.Stat(wt.path); os.IsNotExist(statErr) {
			orphans = append(orphans, orphanedWorktree{
				TaskID: taskID,
				Path:   wt.path,
				Reason: "directory no longer exists",
			})
			continue
		}

		// Load task to check status
		t, err := backend.LoadTask(taskID)
		if err != nil {
			orphans = append(orphans, orphanedWorktree{
				TaskID: taskID,
				Path:   wt.path,
				Reason: "task not found in database",
			})
			continue
		}

		// Check if task is in a terminal state
		switch t.Status {
		case task.StatusCompleted:
			orphans = append(orphans, orphanedWorktree{
				TaskID: taskID,
				Path:   wt.path,
				Status: t.Status,
				Reason: "task completed",
			})
		case task.StatusFailed:
			if includeFailed {
				orphans = append(orphans, orphanedWorktree{
					TaskID: taskID,
					Path:   wt.path,
					Status: t.Status,
					Reason: "task failed",
				})
			}
		}
	}

	return orphans, nil
}

// worktreeInfo holds parsed info from git worktree list --porcelain
type worktreeInfo struct {
	path   string
	head   string
	branch string
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) []worktreeInfo {
	var worktrees []worktreeInfo
	var current worktreeInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.path != "" {
				worktrees = append(worktrees, current)
				current = worktreeInfo{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.head = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			current.branch = strings.TrimPrefix(line, "branch ")
		}
	}

	// Don't forget the last worktree
	if current.path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}
