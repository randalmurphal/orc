// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"sort"
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
	tasks, err := task.LoadAll()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		fmt.Println("\nGet started:")
		fmt.Println("  orc new \"Your task description\"")
		return nil
	}

	// Categorize tasks
	var blocked, running, paused, recent, other []*task.Task
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	for _, t := range tasks {
		switch t.Status {
		case task.StatusBlocked:
			blocked = append(blocked, t)
		case task.StatusRunning:
			running = append(running, t)
		case task.StatusPaused:
			paused = append(paused, t)
		case task.StatusCompleted, task.StatusFailed:
			if t.UpdatedAt.After(dayAgo) {
				recent = append(recent, t)
			} else if showAll {
				other = append(other, t)
			}
		default:
			other = append(other, t)
		}
	}

	// Sort recent by update time (newest first)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].UpdatedAt.After(recent[j].UpdatedAt)
	})

	// Print sections with priority ordering
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Attention needed (blocked)
	if len(blocked) > 0 {
		fmt.Println("⚠️  ATTENTION NEEDED")
		fmt.Println()
		for _, t := range blocked {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", t.ID, truncate(t.Title, 40), "(blocked - needs input)")
		}
		w.Flush()
		fmt.Println()
	}

	// In progress (running)
	if len(running) > 0 {
		if plain {
			fmt.Println("RUNNING")
		} else {
			fmt.Println("⏳ RUNNING")
		}
		fmt.Println()
		for _, t := range running {
			phase := t.CurrentPhase
			if phase == "" {
				phase = "starting"
			}
			fmt.Fprintf(w, "  %s\t%s\t[%s]\n", t.ID, truncate(t.Title, 40), phase)
		}
		w.Flush()
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
			fmt.Fprintf(w, "  %s\t%s\t→ orc resume %s\n", t.ID, truncate(t.Title, 40), t.ID)
		}
		w.Flush()
		fmt.Println()
	}

	// Recent activity (completed/failed in last 24h)
	if len(recent) > 0 {
		fmt.Println("RECENT (24h)")
		fmt.Println()
		for _, t := range recent {
			icon := statusIcon(t.Status)
			ago := formatTimeAgo(t.UpdatedAt)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", icon, t.ID, truncate(t.Title, 35), ago)
		}
		w.Flush()
		fmt.Println()
	}

	// Other tasks (if --all)
	if showAll && len(other) > 0 {
		fmt.Println("OTHER")
		fmt.Println()
		for _, t := range other {
			icon := statusIcon(t.Status)
			fmt.Fprintf(w, "  %s\t%s\t%s\n", icon, t.ID, truncate(t.Title, 40))
		}
		w.Flush()
		fmt.Println()
	}

	// Quick stats summary
	total := len(tasks)
	completed := 0
	for _, t := range tasks {
		if t.Status == task.StatusCompleted {
			completed++
		}
	}

	fmt.Printf("─── %d tasks (%d running, %d blocked, %d completed) ───\n",
		total, len(running), len(blocked), completed)

	return nil
}

func watchStatus(showAll bool) error {
	fmt.Println("Watching status (Ctrl+C to stop)...\n")
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
