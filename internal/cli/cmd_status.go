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

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show orc status",
		Long: `Show current orc status at a glance.

Organized by priority:
  1. Blocked tasks (need attention)
  2. Running tasks (in progress)
  3. Paused tasks (can resume)
  4. Recent activity (last 24h)

Examples:
  orc status           # Quick overview
  orc status --all     # Include all tasks
  orc status --watch   # Refresh every 5s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			all, _ := cmd.Flags().GetBool("all")
			watch, _ := cmd.Flags().GetBool("watch")

			if watch {
				return watchStatus(all)
			}

			return showStatus(all)
		},
	}

	cmd.Flags().BoolP("all", "a", false, "show all tasks including completed")
	cmd.Flags().BoolP("watch", "w", false, "refresh status every 5 seconds")

	return cmd
}

func showStatus(showAll bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		fmt.Println("\nGet started:")
		fmt.Println("  orc new \"Your task description\"")
		return nil
	}

	// Populate computed fields for dependency tracking
	task.PopulateComputedFields(tasks)

	// Build task map for dependency checks
	taskMap := make(map[string]*task.Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Check for orphaned tasks by loading states
	type orphanInfo struct {
		TaskID string
		Reason string
	}
	var orphans []orphanInfo
	states, stateErr := backend.LoadAllStates()
	if stateErr == nil {
		for _, s := range states {
			if isOrphaned, reason := s.CheckOrphaned(); isOrphaned {
				orphans = append(orphans, orphanInfo{TaskID: s.TaskID, Reason: reason})
			}
		}
	}
	orphanedIDs := make(map[string]orphanInfo)
	for _, o := range orphans {
		orphanedIDs[o.TaskID] = o
	}

	// Categorize tasks
	var systemBlocked, depBlocked, running, orphaned, paused, ready, recent, other []*task.Task
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	for _, t := range tasks {
		switch t.Status {
		case task.StatusBlocked:
			// System-level blocked (needs human input)
			systemBlocked = append(systemBlocked, t)
		case task.StatusRunning:
			// Check if this running task is actually orphaned
			if _, isOrphaned := orphanedIDs[t.ID]; isOrphaned {
				orphaned = append(orphaned, t)
			} else {
				running = append(running, t)
			}
		case task.StatusPaused:
			paused = append(paused, t)
		case task.StatusFinalizing, task.StatusCompleted, task.StatusFinished, task.StatusFailed:
			if t.UpdatedAt.After(dayAgo) {
				recent = append(recent, t)
			} else if showAll {
				other = append(other, t)
			}
		case task.StatusCreated, task.StatusPlanned:
			// Check dependency status for created/planned tasks
			if len(t.BlockedBy) > 0 {
				unmet := t.GetUnmetDependencies(taskMap)
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
		return recent[i].UpdatedAt.After(recent[j].UpdatedAt)
	})

	// Sort ready by priority
	sort.Slice(ready, func(i, j int) bool {
		return task.PriorityOrder(ready[i].GetPriority()) < task.PriorityOrder(ready[j].GetPriority())
	})

	// Print sections with priority ordering
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Orphaned tasks (highest priority - executor died)
	if len(orphaned) > 0 {
		if plain {
			fmt.Println("ORPHANED (executor died)")
		} else {
			fmt.Println("\u26a0\ufe0f  ORPHANED (executor died)")
		}
		fmt.Println()
		for _, t := range orphaned {
			info := orphanedIDs[t.ID]
			reason := info.Reason
			if reason == "" {
				reason = "unknown"
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t(%s)\n", t.ID, truncate(t.Title, 35), reason)
		}
		_ = w.Flush()
		fmt.Println("  Use 'orc resume <task-id>' to continue these tasks")
		fmt.Println()
	}

	// Attention needed (system blocked - needs human input)
	if len(systemBlocked) > 0 {
		if plain {
			fmt.Println("ATTENTION NEEDED")
		} else {
			fmt.Println("\u26a0\ufe0f  ATTENTION NEEDED")
		}
		fmt.Println()
		for _, t := range systemBlocked {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", t.ID, truncate(t.Title, 40), "(blocked - needs input)")
		}
		_ = w.Flush()
		fmt.Println()
	}

	// In progress (running)
	if len(running) > 0 {
		if plain {
			fmt.Println("RUNNING")
		} else {
			fmt.Println("\u23f3 RUNNING")
		}
		fmt.Println()
		for _, t := range running {
			phase := t.CurrentPhase
			if phase == "" {
				phase = "starting"
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t[%s]\n", t.ID, truncate(t.Title, 40), phase)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Dependency blocked (waiting on other tasks)
	if len(depBlocked) > 0 {
		if plain {
			fmt.Println("BLOCKED")
		} else {
			fmt.Println("🚫 BLOCKED")
		}
		fmt.Println()
		for _, t := range depBlocked {
			unmet := t.GetUnmetDependencies(taskMap)
			blockerStr := formatBlockerList(unmet)
			_, _ = fmt.Fprintf(w, "  %s\t%s\t(by %s)\n", t.ID, truncate(t.Title, 35), blockerStr)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Ready (can run now - dependencies satisfied)
	if len(ready) > 0 {
		if plain {
			fmt.Println("READY")
		} else {
			fmt.Println("📋 READY")
		}
		fmt.Println()
		for _, t := range ready {
			_, _ = fmt.Fprintf(w, "  %s\t%s\n", t.ID, truncate(t.Title, 45))
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Paused (can resume)
	if len(paused) > 0 {
		if plain {
			fmt.Println("PAUSED")
		} else {
			fmt.Println("⏸️  PAUSED")
		}
		fmt.Println()
		for _, t := range paused {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t→ orc resume %s\n", t.ID, truncate(t.Title, 40), t.ID)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Recent activity (completed/failed in last 24h)
	if len(recent) > 0 {
		fmt.Println("RECENT (24h)")
		fmt.Println()
		for _, t := range recent {
			icon := statusIcon(t.Status)
			ago := formatTimeAgo(t.UpdatedAt)
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", icon, t.ID, truncate(t.Title, 35), ago)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Other tasks (if --all)
	if showAll && len(other) > 0 {
		fmt.Println("OTHER")
		fmt.Println()
		for _, t := range other {
			icon := statusIcon(t.Status)
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", icon, t.ID, truncate(t.Title, 40))
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Quick stats summary
	total := len(tasks)
	completed := 0
	for _, t := range tasks {
		if t.Status == task.StatusCompleted || t.Status == task.StatusFinished {
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

	fmt.Printf("─── %d tasks (%s) ───\n", total, strings.Join(summaryParts, ", "))

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

func watchStatus(showAll bool) error {
	fmt.Println("Watching status (Ctrl+C to stop)...")
	for {
		// Clear screen
		fmt.Print("\033[H\033[2J")
		fmt.Printf("orc status (updated %s)\n\n", time.Now().Format("15:04:05"))
		if err := showStatus(showAll); err != nil {
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
