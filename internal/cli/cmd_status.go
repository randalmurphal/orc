// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show task status overview (prioritized by need for attention)",
		Long: `Show current task status organized by priority.

This is your dashboard for understanding what needs attention. Tasks are
organized by priority so you always know what to work on next.

PRIORITY ORDER:
  1. 🔴 Orphaned    Executor died mid-run (use 'orc resume' to continue)
  2. 🟡 Attention   Failed tasks, blocked gates needing approval
  3. 🔵 Running     Currently executing
  4. ⚫ Blocked     Waiting on task dependencies (blocked_by)
  5. 🟢 Ready       Can be run (dependencies complete)
  6. ⏸  Paused      Manually paused (use 'orc resume' to continue)
  7. 📋 Recent      Completed in last 24h

DEPENDENCY AWARENESS:
  The status command shows dependency state alongside task state:
  • BLOCKED (waiting on deps) - has incomplete blocked_by tasks
  • READY (deps complete) - all blocked_by tasks finished

COMMON WORKFLOWS:
  After checking status, typical next steps:
  • Orphaned task  → orc resume TASK-XXX
  • Attention task → orc show TASK-XXX to diagnose, then fix/retry
  • Ready task     → orc run TASK-XXX
  • Blocked task   → orc deps TASK-XXX to see what's blocking

Examples:
  orc status          # Quick overview (most common)
  orc st              # Short alias
  orc status --all    # Include completed tasks
  orc status --watch  # Auto-refresh every 5 seconds

See also:
  orc deps     - View dependency relationships
  orc list     - List all tasks with filters
  orc show     - Detailed view of specific task`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			all, _ := cmd.Flags().GetBool("all")
			watch, _ := cmd.Flags().GetBool("watch")

			if watch {
				return watchStatus(cmd, all)
			}

			return showStatus(cmd, all)
		},
	}

	cmd.Flags().BoolP("all", "a", false, "show all tasks including completed")
	cmd.Flags().BoolP("watch", "w", false, "refresh status every 5 seconds")
	cmd.Flags().StringP("initiative", "i", "", "filter by initiative ID (use 'unassigned' or '' for tasks without initiative)")

	// Register completion function for initiative flag
	_ = cmd.RegisterFlagCompletionFunc("initiative", completeInitiativeIDs)

	return cmd
}

func showStatus(cmd *cobra.Command, showAll bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	out := cmd.OutOrStdout()

	// Extract initiative filter
	initiativeFilter, _ := cmd.Flags().GetString("initiative")
	initiativeFilterActive := cmd.Flags().Changed("initiative")

	// Validate initiative filter if provided (unless it's "unassigned" or empty)
	if initiativeFilterActive && initiativeFilter != "" && strings.ToLower(initiativeFilter) != "unassigned" {
		exists, err := backend.InitiativeExists(initiativeFilter)
		if err != nil {
			return fmt.Errorf("check initiative: %w", err)
		}
		if !exists {
			return fmt.Errorf("initiative %s not found", initiativeFilter)
		}
	}

	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	// Task.CurrentPhase is now authoritative — set by the executor at phase start.
	// No need to enrich from workflow_runs (SC-5).

	if len(allTasks) == 0 {
		_, _ = fmt.Fprintln(out, "No tasks found.")
		_, _ = fmt.Fprintln(out, "\nGet started:")
		_, _ = fmt.Fprintln(out, "  orc new \"Your task description\"")
		return nil
	}

	// Populate computed fields for dependency tracking (on ALL tasks)
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map for dependency checks (using ALL tasks, not filtered)
	// This ensures dependencies work correctly even when blocker is outside initiative
	taskMap := make(map[string]*orcv1.Task)
	for _, t := range allTasks {
		taskMap[t.Id] = t
	}

	// Apply initiative filter after loading but before categorization
	var tasks []*orcv1.Task
	for _, t := range allTasks {
		// Initiative filter
		if initiativeFilterActive {
			initID := task.GetInitiativeIDProto(t)
			// Empty string or "unassigned" means show tasks without initiative
			if initiativeFilter == "" || strings.ToLower(initiativeFilter) == "unassigned" {
				if initID != "" {
					continue
				}
			} else {
				if initID != initiativeFilter {
					continue
				}
			}
		}
		tasks = append(tasks, t)
	}

	if len(tasks) == 0 {
		var filterDesc []string
		if initiativeFilterActive {
			if initiativeFilter == "" || strings.ToLower(initiativeFilter) == "unassigned" {
				filterDesc = append(filterDesc, "unassigned to any initiative")
			} else {
				filterDesc = append(filterDesc, fmt.Sprintf("in initiative %s", initiativeFilter))
			}
		}
		if len(filterDesc) > 0 {
			_, _ = fmt.Fprintf(out, "No tasks found %s.\n", strings.Join(filterDesc, " "))
		} else {
			_, _ = fmt.Fprintln(out, "No tasks found.")
		}
		_, _ = fmt.Fprintln(out, "\nGet started:")
		_, _ = fmt.Fprintln(out, "  orc new \"Your task description\"")
		return nil
	}

	// Check for orphaned tasks directly from task executor fields
	// This is much more efficient than loading full states (no N+1 queries)
	type orphanInfo struct {
		TaskID string
		Reason string
	}
	orphanedIDs := make(map[string]orphanInfo)
	for _, t := range tasks {
		if isOrphaned, reason := task.CheckOrphanedProto(t); isOrphaned {
			orphanedIDs[t.Id] = orphanInfo{TaskID: t.Id, Reason: reason}
		}
	}

	// Categorize tasks
	var systemBlocked, depBlocked, running, orphaned, paused, ready, recent, other []*orcv1.Task
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	for _, t := range tasks {
		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
			// System-level blocked (needs human input)
			systemBlocked = append(systemBlocked, t)
		case orcv1.TaskStatus_TASK_STATUS_RUNNING:
			// Check if this running task is actually orphaned
			if _, isOrphaned := orphanedIDs[t.Id]; isOrphaned {
				orphaned = append(orphaned, t)
			} else {
				running = append(running, t)
			}
		case orcv1.TaskStatus_TASK_STATUS_PAUSED:
			paused = append(paused, t)
		case orcv1.TaskStatus_TASK_STATUS_FINALIZING, orcv1.TaskStatus_TASK_STATUS_COMPLETED, orcv1.TaskStatus_TASK_STATUS_FAILED:
			if t.UpdatedAt != nil && t.UpdatedAt.AsTime().After(dayAgo) {
				recent = append(recent, t)
			} else if showAll {
				other = append(other, t)
			}
		case orcv1.TaskStatus_TASK_STATUS_CREATED, orcv1.TaskStatus_TASK_STATUS_PLANNED:
			// Check dependency status for created/planned tasks
			if len(t.BlockedBy) > 0 {
				unmet := task.GetUnmetDependenciesProto(t, taskMap)
				if len(unmet) > 0 {
					depBlocked = append(depBlocked, t)
				} else {
					ready = append(ready, t)
				}
			} else {
				ready = append(ready, t)
			}
		default:
			other = append(other, t)
		}
	}

	// Sort recent by update time (newest first)
	sort.Slice(recent, func(i, j int) bool {
		ti := time.Time{}
		tj := time.Time{}
		if recent[i].UpdatedAt != nil {
			ti = recent[i].UpdatedAt.AsTime()
		}
		if recent[j].UpdatedAt != nil {
			tj = recent[j].UpdatedAt.AsTime()
		}
		return ti.After(tj)
	})

	// Sort ready by priority
	sort.Slice(ready, func(i, j int) bool {
		return task.PriorityOrderFromProto(task.GetPriorityProto(ready[i])) < task.PriorityOrderFromProto(task.GetPriorityProto(ready[j]))
	})

	// Print sections with priority ordering
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Orphaned tasks (highest priority - executor died)
	if len(orphaned) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "ORPHANED (executor died)")
		} else {
			_, _ = fmt.Fprintln(out, "\u26a0\ufe0f  ORPHANED (executor died)")
		}
		_, _ = fmt.Fprintln(out)
		for _, t := range orphaned {
			info := orphanedIDs[t.Id]
			reason := info.Reason
			if reason == "" {
				reason = "unknown"
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t(%s)\n", t.Id, truncate(t.Title, 35), reason)
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out, "  Use 'orc resume <task-id>' to continue these tasks")
		_, _ = fmt.Fprintln(out)
	}

	// Attention needed (system blocked - needs human input)
	if len(systemBlocked) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "ATTENTION NEEDED")
		} else {
			_, _ = fmt.Fprintln(out, "\u26a0\ufe0f  ATTENTION NEEDED")
		}
		_, _ = fmt.Fprintln(out)
		for _, t := range systemBlocked {
			// Show detailed info for blocked tasks with conflict metadata
			blockedReason := "(blocked - needs input)"
			worktreePath := ""
			if t.Metadata != nil {
				if reason, ok := t.Metadata["blocked_reason"]; ok && reason == "sync_conflict" {
					blockedReason = "(sync conflict)"
					// Construct worktree path
					cfg, _ := config.Load()
					if cfg != nil && cfg.Worktree.Enabled {
						cwd, _ := os.Getwd()
						resolvedDir := config.ResolveWorktreeDir(cfg.Worktree.Dir, cwd)
						worktreePath = resolvedDir + "/orc-" + t.Id
					}
				}
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", t.Id, truncate(t.Title, 40), blockedReason)
			if worktreePath != "" {
				_, _ = fmt.Fprintf(out, "      Worktree: %s\n", worktreePath)
				_, _ = fmt.Fprintf(out, "      → orc resume %s (after resolving conflicts)\n", t.Id)
			}
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// In progress (running)
	if len(running) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "RUNNING")
		} else {
			_, _ = fmt.Fprintln(out, "\u23f3 RUNNING")
		}
		_, _ = fmt.Fprintln(out)
		cfg, _ := config.Load()
		for _, t := range running {
			phase := task.GetCurrentPhaseProto(t)
			if phase == "" {
				phase = "starting"
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t[%s]\n", t.Id, truncate(t.Title, 40), phase)
			_ = w.Flush()
			if cfg != nil && cfg.Worktree.Enabled {
				cwd, _ := os.Getwd()
				resolvedDir := config.ResolveWorktreeDir(cfg.Worktree.Dir, cwd)
				worktreePath := resolvedDir + "/orc-" + t.Id
				if _, statErr := os.Stat(worktreePath); statErr == nil {
					_, _ = fmt.Fprintf(out, "      Worktree: %s\n", worktreePath)
				}
			}
		}
		_, _ = fmt.Fprintln(out)
	}

	// Dependency blocked (waiting on other tasks)
	if len(depBlocked) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "BLOCKED")
		} else {
			_, _ = fmt.Fprintln(out, "🚫 BLOCKED")
		}
		_, _ = fmt.Fprintln(out)

		// Build set of task IDs that are being displayed (for filtering blocker IDs)
		displayedTaskIDs := make(map[string]bool)
		for _, t := range tasks {
			displayedTaskIDs[t.Id] = true
		}

		for _, t := range depBlocked {
			unmet := task.GetUnmetDependenciesProto(t, taskMap)
			// Filter unmet dependencies to only show blockers that are in the displayed task list
			var filteredUnmet []string
			for _, blockerID := range unmet {
				if displayedTaskIDs[blockerID] {
					filteredUnmet = append(filteredUnmet, blockerID)
				}
			}
			// Only show "(by ...)" if there are displayed blockers
			if len(filteredUnmet) > 0 {
				blockerStr := formatBlockerList(filteredUnmet)
				_, _ = fmt.Fprintf(w, "  %s\t%s\t(by %s)\n", t.Id, truncate(t.Title, 35), blockerStr)
			} else {
				// No displayed blockers - just show the task without blocker info
				_, _ = fmt.Fprintf(w, "  %s\t%s\n", t.Id, truncate(t.Title, 35))
			}
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Ready (can run now - dependencies satisfied)
	if len(ready) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "READY")
		} else {
			_, _ = fmt.Fprintln(out, "📋 READY")
		}
		_, _ = fmt.Fprintln(out)
		for _, t := range ready {
			_, _ = fmt.Fprintf(w, "  %s\t%s\n", t.Id, truncate(t.Title, 45))
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Paused (can resume)
	if len(paused) > 0 {
		if plain {
			_, _ = fmt.Fprintln(out, "PAUSED")
		} else {
			_, _ = fmt.Fprintln(out, "⏸️  PAUSED")
		}
		_, _ = fmt.Fprintln(out)
		for _, t := range paused {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t→ orc resume %s\n", t.Id, truncate(t.Title, 40), t.Id)
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Recent activity (completed/failed in last 24h)
	if len(recent) > 0 {
		_, _ = fmt.Fprintln(out, "RECENT (24h)")
		_, _ = fmt.Fprintln(out)
		for _, t := range recent {
			icon := statusIcon(t.Status)
			ago := ""
			if t.UpdatedAt != nil {
				ago = formatTimeAgo(t.UpdatedAt.AsTime())
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", icon, t.Id, truncate(t.Title, 35), ago)
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Other tasks (if --all)
	if showAll && len(other) > 0 {
		_, _ = fmt.Fprintln(out, "OTHER")
		_, _ = fmt.Fprintln(out)
		for _, t := range other {
			icon := statusIcon(t.Status)
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", icon, t.Id, truncate(t.Title, 40))
		}
		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Quick stats summary
	total := len(tasks)
	completed := 0
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			completed++
		}
	}

	// Build summary line
	summaryParts := []string{fmt.Sprintf("%d running", len(running))}
	if len(orphaned) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d orphaned", len(orphaned)))
	}
	if len(depBlocked) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d blocked", len(depBlocked)))
	}
	if len(ready) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d ready", len(ready)))
	}
	summaryParts = append(summaryParts, fmt.Sprintf("%d completed", completed))

	_, _ = fmt.Fprintf(out, "─── %d tasks (%s) ───\n", total, strings.Join(summaryParts, ", "))

	return nil
}

// formatBlockerList formats a list of blocker IDs for display
func formatBlockerList(blockerIDs []string) string {
	if len(blockerIDs) == 0 {
		return ""
	}
	if len(blockerIDs) <= 3 {
		return strings.Join(blockerIDs, ", ")
	}
	return strings.Join(blockerIDs[:3], ", ") + fmt.Sprintf(" +%d more", len(blockerIDs)-3)
}

func watchStatus(cmd *cobra.Command, showAll bool) error {
	fmt.Println("Watching status (Ctrl+C to stop)...")
	for {
		// Clear screen
		fmt.Print("\033[H\033[2J")
		fmt.Printf("orc status (updated %s)\n\n", time.Now().Format("15:04:05"))
		if err := showStatus(cmd, showAll); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		time.Sleep(5 * time.Second)
	}
}

// formatTimeAgo returns a human-readable relative time
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
