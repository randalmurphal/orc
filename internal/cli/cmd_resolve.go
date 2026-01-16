// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// worktreeStatus holds information about a task's worktree state.
type worktreeStatus struct {
	exists         bool
	path           string
	isDirty        bool
	hasConflicts   bool
	conflictFiles  []string
	rebaseInProg   bool
	mergeInProg    bool
	uncommittedMsg string
}

// checkWorktreeStatus checks the state of a task's worktree.
func checkWorktreeStatus(taskID string, gitOps *git.Git) (*worktreeStatus, error) {
	status := &worktreeStatus{}

	if gitOps == nil {
		return status, nil
	}

	// Get worktree path
	status.path = gitOps.WorktreePath(taskID)

	// Check if worktree exists
	if _, err := os.Stat(status.path); os.IsNotExist(err) {
		return status, nil
	}
	status.exists = true

	// Create a git context for the worktree
	worktreeGit := gitOps.InWorktree(status.path)
	ctx := worktreeGit.Context()

	// Check if working directory is clean
	clean, err := worktreeGit.IsClean()
	if err == nil && !clean {
		status.isDirty = true
		// Get details about uncommitted changes
		output, _ := ctx.RunGit("status", "--porcelain")
		if output != "" {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			status.uncommittedMsg = fmt.Sprintf("%d uncommitted file(s)", len(lines))
		}
	}

	// Check for rebase in progress using the git package method
	status.rebaseInProg, _ = worktreeGit.IsRebaseInProgress()

	// Check for merge in progress using the git package method
	status.mergeInProg, _ = worktreeGit.IsMergeInProgress()

	// Check for unmerged files (conflicts)
	output, err := ctx.RunGit("diff", "--name-only", "--diff-filter=U")
	if err == nil && strings.TrimSpace(output) != "" {
		status.hasConflicts = true
		status.conflictFiles = strings.Split(strings.TrimSpace(output), "\n")
	}

	return status, nil
}

// hasWorktreeIssues returns true if the worktree has any issues that need attention.
func (s *worktreeStatus) hasWorktreeIssues() bool {
	if s == nil || !s.exists {
		return false
	}
	return s.isDirty || s.hasConflicts || s.rebaseInProg || s.mergeInProg
}

// newResolveCmd creates the resolve command
func newResolveCmd() *cobra.Command {
	var message string
	var cleanup bool

	cmd := &cobra.Command{
		Use:   "resolve <task-id>",
		Short: "Mark failed task as resolved without re-running",
		Long: `Mark a failed task as resolved/acknowledged without re-running it.

This is useful when:
  - The issue was fixed manually outside of orc
  - The failure is no longer relevant (e.g., requirements changed)
  - You want to acknowledge and close out a failed task

The task will be marked as completed with metadata indicating it was resolved
rather than executed to completion. This preserves the failure context in the
execution history.

Unlike 'reset' which clears progress and allows retry, 'resolve' closes the
task without clearing its execution state.

Worktree handling:
  If the task has an associated worktree with uncommitted changes, in-progress
  git operations (rebase/merge), or unresolved conflicts, a warning will be
  displayed with suggested actions:

  --cleanup   Abort in-progress git operations and discard uncommitted changes
  --force     Skip worktree state checks entirely (resolve without cleanup)

Note: --cleanup cleans the worktree state but preserves the worktree itself.
Use 'orc cleanup TASK-XXX' to fully remove a worktree after resolving.

Examples:
  orc resolve TASK-001                          # Mark as resolved
  orc resolve TASK-001 -m "Fixed manually"      # With resolution message
  orc resolve TASK-001 --cleanup                # Clean up worktree state first
  orc resolve TASK-001 --force                  # Skip all checks`,
		Args: cobra.ExactArgs(1),
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

			id := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Load task to verify it exists and check status
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Only allow resolving failed tasks. Blocked tasks get special guidance
			// since users often confuse "resolve" with "resume" for blocked tasks.
			// See TASK-288 for rationale.
			if t.Status != task.StatusFailed {
				if t.Status == task.StatusBlocked {
					// Provide actionable guidance with task ID included for copy-paste
					return fmt.Errorf(`task %s is blocked (status: blocked), not failed

For blocked tasks, use one of these commands instead:
  orc approve %s   Approve a gate and mark task ready to run
  orc resume %s    Resume execution (for paused/blocked/failed tasks)

The 'resolve' command is for marking failed tasks as complete without re-running`, id, id, id)
				}
				return fmt.Errorf("task %s is %s, not failed; resolve is only for failed tasks", id, t.Status)
			}

			// Load config for git settings
			cfg, err := config.Load()
			if err != nil {
				// Non-fatal: use defaults if config can't be loaded
				cfg = config.Default()
			}

			// Initialize git operations to check worktree status
			var gitOps *git.Git
			var wtStatus *worktreeStatus
			if cfg.Worktree.Enabled {
				gitCfg := git.Config{
					BranchPrefix:   cfg.BranchPrefix,
					CommitPrefix:   cfg.CommitPrefix,
					WorktreeDir:    cfg.Worktree.Dir,
					ExecutorPrefix: cfg.ExecutorPrefix(),
				}
				gitOps, err = git.New(projectRoot, gitCfg)
				if err != nil {
					// Non-fatal: warn but continue
					if !quiet {
						fmt.Fprintf(os.Stderr, "⚠️  Warning: Could not initialize git: %v\n", err)
					}
				} else {
					wtStatus, _ = checkWorktreeStatus(id, gitOps)
				}
			}

			// Display worktree warnings if applicable
			if wtStatus != nil && wtStatus.exists && !quiet {
				fmt.Printf("📁 Worktree: %s\n", wtStatus.path)

				if wtStatus.rebaseInProg {
					fmt.Println("   ⚠️  Rebase in progress - worktree is in an incomplete state")
				}
				if wtStatus.mergeInProg {
					fmt.Println("   ⚠️  Merge in progress - worktree is in an incomplete state")
				}
				if wtStatus.hasConflicts {
					fmt.Printf("   ⚠️  %d file(s) have unresolved conflicts:\n", len(wtStatus.conflictFiles))
					for _, f := range wtStatus.conflictFiles {
						fmt.Printf("      - %s\n", f)
					}
				}
				if wtStatus.isDirty && !wtStatus.hasConflicts && !wtStatus.rebaseInProg && !wtStatus.mergeInProg {
					fmt.Printf("   ⚠️  %s\n", wtStatus.uncommittedMsg)
				}
				fmt.Println()
			}

			// Perform cleanup if requested (before confirmation so user sees what was cleaned)
			var cleanupPerformed bool
			var cleanupErr error
			if cleanup && wtStatus != nil && wtStatus.hasWorktreeIssues() && gitOps != nil {
				if !quiet {
					fmt.Println("🧹 Cleaning up worktree state...")
				}
				worktreeGit := gitOps.InWorktree(wtStatus.path)
				ctx := worktreeGit.Context()

				// Abort rebase if in progress
				if wtStatus.rebaseInProg {
					if _, err := ctx.RunGit("rebase", "--abort"); err == nil {
						if !quiet {
							fmt.Println("   Aborted rebase-in-progress")
						}
					}
				}
				// Abort merge if in progress
				if wtStatus.mergeInProg {
					if _, err := ctx.RunGit("merge", "--abort"); err == nil {
						if !quiet {
							fmt.Println("   Aborted merge-in-progress")
						}
					}
				}
				// Discard uncommitted changes
				if wtStatus.isDirty || wtStatus.hasConflicts {
					cleanupErr = worktreeGit.DiscardChanges()
					if cleanupErr == nil {
						if !quiet {
							fmt.Println("   Discarded uncommitted changes")
						}
					}
				}
				cleanupPerformed = true
				if !quiet {
					fmt.Println()
				}
			}

			// Confirmation prompt
			if !force && !quiet {
				fmt.Printf("⚠️  Resolve task %s as completed?\n", id)
				fmt.Println("   The task will be marked as completed (resolved).")
				fmt.Println("   Execution state will be preserved for reference.")
				if wtStatus != nil && wtStatus.exists && !cleanupPerformed {
					fmt.Println("   The worktree will be preserved.")
				}
				fmt.Print("   Continue? [y/N]: ")

				var input string
				_, _ = fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Update task status to completed
			t.Status = task.StatusCompleted
			now := time.Now()
			t.CompletedAt = &now

			// Add resolution metadata
			if t.Metadata == nil {
				t.Metadata = make(map[string]string)
			}
			t.Metadata["resolved"] = "true"
			t.Metadata["resolved_at"] = now.Format(time.RFC3339)
			if message != "" {
				t.Metadata["resolution_message"] = message
			}
			// Track worktree state at resolution time
			if wtStatus != nil && wtStatus.exists {
				if wtStatus.isDirty {
					t.Metadata["worktree_was_dirty"] = "true"
				}
				if wtStatus.hasConflicts {
					t.Metadata["worktree_had_conflicts"] = "true"
				}
				if wtStatus.rebaseInProg || wtStatus.mergeInProg {
					t.Metadata["worktree_had_incomplete_operation"] = "true"
				}
			}

			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Output results
			if plain {
				fmt.Printf("Task %s resolved\n", id)
			} else {
				fmt.Printf("✓ Task %s marked as resolved\n", id)
			}
			if message != "" {
				fmt.Printf("   Message: %s\n", message)
			}
			if cleanupPerformed && cleanupErr != nil {
				fmt.Printf("   ⚠️  Worktree cleanup had errors: %v\n", cleanupErr)
			}
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation")
	cmd.Flags().StringVarP(&message, "message", "m", "", "resolution message explaining why task was resolved")
	cmd.Flags().BoolVar(&cleanup, "cleanup", false, "abort in-progress git operations and discard uncommitted changes")
	return cmd
}
