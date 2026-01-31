// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
  - A task is stuck in 'running' status but its PR was already merged

The task will be marked as completed with metadata indicating it was resolved
rather than executed to completion. This preserves the failure context in the
execution history.

Unlike 'reset' which clears progress and allows retry, 'resolve' closes the
task without clearing its execution state.

Force resolving non-failed tasks:
  By default, resolve only works on failed tasks. Use --force to resolve tasks
  in any status (running, paused, blocked, created, etc.). This is useful when
  a task is stuck but the work is already complete (e.g., PR merged but executor
  crashed before marking task complete).

  When force-resolving, the command will:
  - Check if the task has a merged PR and report it
  - Warn if no PR exists or the PR is not merged
  - Record the original status and force_resolved flag in metadata

Worktree handling:
  If the task has an associated worktree with uncommitted changes, in-progress
  git operations (rebase/merge), or unresolved conflicts, a warning will be
  displayed with suggested actions:

  --cleanup   Abort in-progress git operations and discard uncommitted changes
  -f/--force  Skip confirmation and status checks (resolve any status)

Note: --cleanup cleans the worktree state but preserves the worktree itself.
Use 'orc cleanup TASK-XXX' to fully remove a worktree after resolving.

Skipping confirmation:
  By default, resolve asks for confirmation before proceeding. Use --yes/-y to
  skip the confirmation prompt (useful in scripts and automated pipelines).
  Note: --yes only skips the prompt; it does NOT allow resolving non-failed tasks.
  Use --force to resolve tasks in any status.

Examples:
  orc resolve TASK-001                          # Mark failed task as resolved
  orc resolve TASK-001 --yes                    # Skip confirmation prompt
  orc resolve TASK-001 -y -m "Fixed manually"   # Skip prompt with message
  orc resolve TASK-001 --cleanup                # Clean up worktree state first
  orc resolve TASK-001 --force                  # Resolve any status (skip checks)
  orc resolve TASK-001 --force -m "PR merged"   # Force resolve with message`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find the project root (handles worktrees)
			projectRoot, err := ResolveProjectPath()
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
			yes, _ := cmd.Flags().GetBool("yes")

			// Load task to verify it exists and check status
			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Track if we're force-resolving a non-failed task
			originalStatus := t.Status
			forceResolving := false

			// Only allow resolving failed tasks without --force.
			// With --force, allow any status (useful for stuck running tasks with merged PRs).
			// Blocked tasks get special guidance since users often confuse "resolve" with "resume".
			if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
				if !force {
					if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
						// Provide actionable guidance with task ID included for copy-paste
						return fmt.Errorf(`task %s is blocked (status: blocked), not failed

For blocked tasks, use one of these commands instead:
  orc approve %s   Approve a gate and mark task ready to run
  orc resume %s    Resume execution (for paused/blocked/failed tasks)

The 'resolve' command is for marking failed tasks as complete without re-running.
Use --force to resolve anyway (e.g., if work is already complete)`, id, id, id)
					}
					return fmt.Errorf("task %s is %s, not failed; resolve is only for failed tasks (use --force to override)", id, task.StatusFromProto(t.Status))
				}
				forceResolving = true
			}

			// Check PR merge status when force-resolving non-failed tasks
			prWasMerged := false
			if forceResolving {
				prStatus := task.GetPRStatusProto(t)
				prNumber := int32(0)
				if t.Pr != nil && t.Pr.Number != nil {
					prNumber = *t.Pr.Number
				}
				hasPR := task.HasPRProto(t)
				if hasPR {
					prMerged := prStatus == orcv1.PRStatus_PR_STATUS_MERGED
					if prMerged {
						prWasMerged = true
						if !quiet {
							if prNumber > 0 {
								fmt.Printf("PR merged (PR #%d)\n", prNumber)
							} else {
								fmt.Println("PR merged")
							}
						}
					} else if !quiet {
						// PR exists but not merged - warn user
						statusStr := prStatus.String()
						if statusStr == "" || statusStr == "PR_STATUS_UNSPECIFIED" {
							statusStr = "unknown"
						}
						if prNumber > 0 {
							fmt.Printf("Warning: PR #%d is not merged (status: %s). Work may be incomplete.\n", prNumber, statusStr)
						} else {
							fmt.Printf("Warning: PR is not merged (status: %s). Work may be incomplete.\n", statusStr)
						}
					}
				} else if !quiet {
					// No PR - warn user
					fmt.Println("Warning: No PR found for this task. Work may be incomplete.")
				}
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
					WorktreeDir:    config.ResolveWorktreeDir(cfg.Worktree.Dir, projectRoot),
					ExecutorPrefix: cfg.ExecutorPrefix(),
				}
				gitOps, err = git.New(projectRoot, gitCfg)
				if err != nil {
					// Non-fatal: warn but continue
					if !quiet {
						fmt.Fprintf(os.Stderr, "⚠️  Warning: Could not initialize git: %v\n", err)
					}
				} else {
					var wtErr error
					wtStatus, wtErr = checkWorktreeStatus(id, gitOps)
					if wtErr != nil && !quiet {
						fmt.Fprintf(os.Stderr, "⚠️  Warning: Could not check worktree status: %v\n", wtErr)
					}
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
			if !force && !quiet && !yes {
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

			// Update task status to resolved (distinct from completed to indicate no actual work done)
			t.Status = orcv1.TaskStatus_TASK_STATUS_RESOLVED
			now := time.Now()

			// Track manual intervention in quality metrics
			reason := "Resolved manually via 'orc resolve'"
			if message != "" {
				reason = message
			}
			task.RecordManualInterventionProto(t, reason)

			// Add resolution metadata
			task.EnsureMetadataProto(t)
			t.Metadata["resolved"] = "true"
			t.Metadata["resolved_at"] = now.Format(time.RFC3339)
			if message != "" {
				t.Metadata["resolution_message"] = message
			}
			// Track force-resolve metadata for non-failed tasks
			if forceResolving {
				t.Metadata["force_resolved"] = "true"
				t.Metadata["original_status"] = task.StatusFromProto(originalStatus)
				if prWasMerged {
					t.Metadata["pr_was_merged"] = "true"
				}
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
				if forceResolving {
					fmt.Printf("Task %s resolved (was: %s)\n", id, originalStatus)
				} else {
					fmt.Printf("Task %s resolved\n", id)
				}
			} else {
				if forceResolving {
					fmt.Printf("✓ Task %s marked as resolved (was: %s)\n", id, originalStatus)
				} else {
					fmt.Printf("✓ Task %s marked as resolved\n", id)
				}
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

	cmd.Flags().BoolP("force", "f", false, "skip confirmation and allow resolving non-failed tasks")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt (does not imply --force)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "resolution message explaining why task was resolved")
	cmd.Flags().BoolVar(&cleanup, "cleanup", false, "abort in-progress git operations and discard uncommitted changes")
	return cmd
}
