// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// DepsOutput represents the JSON output structure for dependencies.
type DepsOutput struct {
	TaskID       string           `json:"task_id"`
	Title        string           `json:"title"`
	Status       task.Status      `json:"status"`
	BlockedBy    []DepsTaskInfo   `json:"blocked_by,omitempty"`
	Blocks       []DepsTaskInfo   `json:"blocks,omitempty"`
	RelatedTo    []DepsTaskInfo   `json:"related_to,omitempty"`
	ReferencedBy []DepsTaskInfo   `json:"referenced_by,omitempty"`
	Summary      DepsSummary      `json:"summary"`
}

// DepsTaskInfo contains information about a related task.
type DepsTaskInfo struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Status task.Status `json:"status"`
}

// DepsSummary provides a status summary.
type DepsSummary struct {
	IsBlocked        bool `json:"is_blocked"`
	UnmetBlockers    int  `json:"unmet_blockers"`
	TotalBlockers    int  `json:"total_blockers"`
	TasksBlocking    int  `json:"tasks_blocking"`
	RelatedCount     int  `json:"related_count"`
	ReferencedCount  int  `json:"referenced_count"`
}

func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps [task-id]",
		Short: "Show task dependencies",
		Long: `Show dependencies for a task, including blocked_by, blocks, related_to, and referenced_by.

Without arguments, shows the dependency overview for all tasks.

Examples:
  orc deps TASK-062            # Show dependencies for TASK-062
  orc deps TASK-062 --tree     # Show full dependency tree
  orc deps --graph INIT-001    # ASCII graph for initiative
  orc deps --graph             # ASCII graph for all tasks
  orc deps TASK-062 --json     # JSON output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			treeView, _ := cmd.Flags().GetBool("tree")
			graphView, _ := cmd.Flags().GetBool("graph")
			initFilter, _ := cmd.Flags().GetString("initiative")

			// Load all tasks for dependency computation
			allTasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			if len(allTasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			// Populate computed fields (blocks, referenced_by)
			task.PopulateComputedFields(allTasks)

			// Build task map
			taskMap := make(map[string]*task.Task)
			for _, t := range allTasks {
				taskMap[t.ID] = t
			}

			// If graph view requested
			if graphView {
				return showDependencyGraph(allTasks, taskMap, initFilter)
			}

			// If no task ID provided, show overview
			if len(args) == 0 {
				return showDependencyOverview(allTasks, taskMap)
			}

			taskID := args[0]
			t, exists := taskMap[taskID]
			if !exists {
				return fmt.Errorf("task %s not found", taskID)
			}

			// Tree view
			if treeView {
				return showDependencyTree(t, taskMap)
			}

			// Standard view
			if jsonOut {
				return showDepsJSON(t, taskMap)
			}
			return showDepsHuman(t, taskMap)
		},
	}

	cmd.Flags().Bool("tree", false, "show full dependency tree")
	cmd.Flags().Bool("graph", false, "show ASCII dependency graph")
	cmd.Flags().StringP("initiative", "i", "", "filter graph by initiative ID")

	return cmd
}

func showDepsHuman(t *task.Task, taskMap map[string]*task.Task) error {
	fmt.Printf("\n%s: %s\n", t.ID, t.Title)
	fmt.Println(strings.Repeat("─", 50))

	// Blocked by
	if len(t.BlockedBy) > 0 {
		fmt.Printf("\nBlocked by (%d):\n", len(t.BlockedBy))
		for _, blockerID := range t.BlockedBy {
			blocker, exists := taskMap[blockerID]
			icon := "○" // pending
			title := "(not found)"
			status := ""
			if exists {
				title = truncate(blocker.Title, 35)
				status = string(blocker.Status)
				if blocker.Status == task.StatusCompleted || blocker.Status == task.StatusFinished {
					icon = "●" // completed
				}
			}
			fmt.Printf("  %s %s  %-35s  %s\n", icon, blockerID, title, status)
		}
	}

	// Blocks
	if len(t.Blocks) > 0 {
		fmt.Printf("\nBlocks (%d):\n", len(t.Blocks))
		for _, blockedID := range t.Blocks {
			blocked, exists := taskMap[blockedID]
			icon := "○"
			title := "(not found)"
			status := ""
			if exists {
				title = truncate(blocked.Title, 35)
				status = string(blocked.Status)
				if blocked.Status == task.StatusCompleted || blocked.Status == task.StatusFinished {
					icon = "●"
				}
			}
			fmt.Printf("  %s %s  %-35s  %s\n", icon, blockedID, title, status)
		}
	}

	// Related to
	if len(t.RelatedTo) > 0 {
		fmt.Printf("\nRelated (%d):\n", len(t.RelatedTo))
		for _, relatedID := range t.RelatedTo {
			related, exists := taskMap[relatedID]
			title := "(not found)"
			if exists {
				title = truncate(related.Title, 40)
			}
			fmt.Printf("  %s  %s\n", relatedID, title)
		}
	}

	// Referenced by
	if len(t.ReferencedBy) > 0 {
		fmt.Printf("\nReferenced by (%d):\n", len(t.ReferencedBy))
		for _, refID := range t.ReferencedBy {
			ref, exists := taskMap[refID]
			title := "(not found)"
			if exists {
				title = truncate(ref.Title, 40)
			}
			fmt.Printf("  %s  %s\n", refID, title)
		}
	}

	// Status summary
	fmt.Println()
	unmet := t.GetUnmetDependencies(taskMap)
	if len(unmet) > 0 {
		if plain {
			fmt.Printf("Status: BLOCKED (waiting on %d task(s): %s)\n",
				len(unmet), strings.Join(unmet, ", "))
		} else {
			fmt.Printf("Status: 🚫 BLOCKED (waiting on %d task(s): %s)\n",
				len(unmet), strings.Join(unmet, ", "))
		}
	} else if len(t.BlockedBy) > 0 {
		if plain {
			fmt.Println("Status: READY (all blockers completed)")
		} else {
			fmt.Println("Status: ✅ READY (all blockers completed)")
		}
	} else {
		if plain {
			fmt.Println("Status: NO BLOCKERS")
		} else {
			fmt.Println("Status: ○ NO BLOCKERS")
		}
	}

	return nil
}

func showDepsJSON(t *task.Task, taskMap map[string]*task.Task) error {
	output := DepsOutput{
		TaskID: t.ID,
		Title:  t.Title,
		Status: t.Status,
	}

	// Blocked by
	for _, blockerID := range t.BlockedBy {
		info := DepsTaskInfo{ID: blockerID}
		if blocker, exists := taskMap[blockerID]; exists {
			info.Title = blocker.Title
			info.Status = blocker.Status
		}
		output.BlockedBy = append(output.BlockedBy, info)
	}

	// Blocks
	for _, blockedID := range t.Blocks {
		info := DepsTaskInfo{ID: blockedID}
		if blocked, exists := taskMap[blockedID]; exists {
			info.Title = blocked.Title
			info.Status = blocked.Status
		}
		output.Blocks = append(output.Blocks, info)
	}

	// Related to
	for _, relatedID := range t.RelatedTo {
		info := DepsTaskInfo{ID: relatedID}
		if related, exists := taskMap[relatedID]; exists {
			info.Title = related.Title
			info.Status = related.Status
		}
		output.RelatedTo = append(output.RelatedTo, info)
	}

	// Referenced by
	for _, refID := range t.ReferencedBy {
		info := DepsTaskInfo{ID: refID}
		if ref, exists := taskMap[refID]; exists {
			info.Title = ref.Title
			info.Status = ref.Status
		}
		output.ReferencedBy = append(output.ReferencedBy, info)
	}

	// Summary
	unmet := t.GetUnmetDependencies(taskMap)
	output.Summary = DepsSummary{
		IsBlocked:       len(unmet) > 0,
		UnmetBlockers:   len(unmet),
		TotalBlockers:   len(t.BlockedBy),
		TasksBlocking:   len(t.Blocks),
		RelatedCount:    len(t.RelatedTo),
		ReferencedCount: len(t.ReferencedBy),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func showDependencyTree(t *task.Task, taskMap map[string]*task.Task) error {
	fmt.Printf("\n%s: %s\n", t.ID, truncate(t.Title, 40))

	seen := make(map[string]bool)
	seen[t.ID] = true

	printTree(t, taskMap, "", true, seen)
	return nil
}

func printTree(t *task.Task, taskMap map[string]*task.Task, prefix string, isLast bool, seen map[string]bool) {
	if len(t.BlockedBy) == 0 {
		return
	}

	for i, blockerID := range t.BlockedBy {
		isLastBlocker := i == len(t.BlockedBy)-1

		// Build the connector
		connector := "├── "
		if isLastBlocker {
			connector = "└── "
		}

		// Check if already seen (circular reference indicator)
		alreadySeen := seen[blockerID]

		blocker, exists := taskMap[blockerID]
		title := "(not found)"
		suffix := ""
		if exists {
			title = truncate(blocker.Title, 35)
		}
		if alreadySeen {
			suffix = " ← already shown"
		} else if exists && (blocker.Status == task.StatusCompleted || blocker.Status == task.StatusFinished) {
			suffix = " ✓"
		} else if exists && len(blocker.BlockedBy) == 0 {
			suffix = " ← start here"
		}

		fmt.Printf("%s%s%s: %s%s\n", prefix, connector, blockerID, title, suffix)

		// Recurse if not seen
		if !alreadySeen && exists {
			seen[blockerID] = true
			newPrefix := prefix
			if isLastBlocker {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			printTree(blocker, taskMap, newPrefix, isLastBlocker, seen)
		}
	}
}

func showDependencyGraph(allTasks []*task.Task, taskMap map[string]*task.Task, initFilter string) error {
	// Filter tasks by initiative if specified
	var filteredTasks []*task.Task
	if initFilter != "" {
		// Verify initiative exists
		if !initiative.Exists(initFilter, false) && !initiative.Exists(initFilter, true) {
			return fmt.Errorf("initiative %s not found", initFilter)
		}
		for _, t := range allTasks {
			if t.InitiativeID == initFilter {
				filteredTasks = append(filteredTasks, t)
			}
		}
		if len(filteredTasks) == 0 {
			fmt.Printf("No tasks found in initiative %s\n", initFilter)
			return nil
		}
	} else {
		filteredTasks = allTasks
	}

	// Build filtered ID set
	filteredIDs := make(map[string]bool)
	for _, t := range filteredTasks {
		filteredIDs[t.ID] = true
	}

	// Find root tasks (no dependencies or all deps outside filter)
	var roots []*task.Task
	var dependent []*task.Task

	for _, t := range filteredTasks {
		hasInternalDep := false
		for _, depID := range t.BlockedBy {
			if filteredIDs[depID] {
				hasInternalDep = true
				break
			}
		}
		if !hasInternalDep {
			roots = append(roots, t)
		} else {
			dependent = append(dependent, t)
		}
	}

	// Sort roots by ID
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].ID < roots[j].ID
	})

	if initFilter != "" {
		fmt.Printf("\nDependency graph for %s:\n\n", initFilter)
	} else {
		fmt.Println("\nDependency graph:")
		fmt.Println()
	}

	// Print each root and its downstream dependencies
	printed := make(map[string]bool)
	for _, root := range roots {
		if printed[root.ID] {
			continue
		}
		printGraphNode(root, taskMap, filteredIDs, printed, "")
		fmt.Println()
	}

	// Print any orphaned tasks (shouldn't happen but just in case)
	for _, t := range filteredTasks {
		if !printed[t.ID] {
			fmt.Printf("%s (orphaned)\n", t.ID)
		}
	}

	return nil
}

func printGraphNode(t *task.Task, taskMap map[string]*task.Task, filteredIDs map[string]bool, printed map[string]bool, indent string) {
	if printed[t.ID] {
		return
	}
	printed[t.ID] = true

	// Find downstream tasks (tasks blocked by this one) within the filter
	var downstream []*task.Task
	for _, other := range taskMap {
		if !filteredIDs[other.ID] {
			continue
		}
		for _, depID := range other.BlockedBy {
			if depID == t.ID {
				downstream = append(downstream, other)
				break
			}
		}
	}

	// Sort downstream by ID
	sort.Slice(downstream, func(i, j int) bool {
		return downstream[i].ID < downstream[j].ID
	})

	// Print this node
	status := ""
	if t.Status == task.StatusCompleted || t.Status == task.StatusFinished {
		status = " ✓"
	} else if t.Status == task.StatusRunning {
		status = " ◐"
	}

	if len(downstream) == 0 {
		fmt.Printf("%s%s%s\n", indent, t.ID, status)
		return
	}

	// Print with children
	if indent == "" {
		fmt.Printf("%s%s\n", t.ID, status)
	} else {
		fmt.Printf("%s%s%s\n", indent, t.ID, status)
	}

	for i, child := range downstream {
		isLast := i == len(downstream)-1
		childPrefix := "├─> "
		nextIndent := indent + "│   "
		if isLast {
			childPrefix = "└─> "
			nextIndent = indent + "    "
		}

		if printed[child.ID] {
			fmt.Printf("%s%s%s (see above)\n", indent, childPrefix, child.ID)
			continue
		}

		// Print inline if single chain
		chain := getChain(child, taskMap, filteredIDs, printed)
		if len(chain) > 0 {
			var chainStr []string
			for _, c := range chain {
				s := c.ID
				if c.Status == task.StatusCompleted || c.Status == task.StatusFinished {
					s += " ✓"
				} else if c.Status == task.StatusRunning {
					s += " ◐"
				}
				chainStr = append(chainStr, s)
				printed[c.ID] = true
			}
			fmt.Printf("%s%s%s\n", indent, childPrefix, strings.Join(chainStr, " ─> "))
		} else {
			fmt.Printf("%s%s", indent, childPrefix)
			printGraphNode(child, taskMap, filteredIDs, printed, nextIndent)
		}
	}
}

// getChain follows a single path of dependencies (no forks)
func getChain(t *task.Task, taskMap map[string]*task.Task, filteredIDs map[string]bool, printed map[string]bool) []*task.Task {
	var chain []*task.Task
	current := t

	for {
		if printed[current.ID] {
			break
		}
		chain = append(chain, current)

		// Find downstream tasks
		var downstream []*task.Task
		for _, other := range taskMap {
			if !filteredIDs[other.ID] || printed[other.ID] {
				continue
			}
			for _, depID := range other.BlockedBy {
				if depID == current.ID {
					downstream = append(downstream, other)
					break
				}
			}
		}

		// Only continue chain if exactly one downstream
		if len(downstream) != 1 {
			break
		}
		current = downstream[0]
	}

	// Return nil if just single node (not a chain)
	if len(chain) <= 1 {
		return nil
	}
	return chain
}

func showDependencyOverview(allTasks []*task.Task, taskMap map[string]*task.Task) error {
	// Categorize tasks by blocking status
	var blocked, blocking, independent []*task.Task

	for _, t := range allTasks {
		unmet := t.GetUnmetDependencies(taskMap)
		if len(unmet) > 0 {
			blocked = append(blocked, t)
		} else if len(t.Blocks) > 0 {
			blocking = append(blocking, t)
		} else {
			independent = append(independent, t)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Show blocking tasks first (high priority to complete)
	if len(blocking) > 0 {
		if plain {
			fmt.Println("BLOCKING OTHER TASKS")
		} else {
			fmt.Println("⚡ BLOCKING OTHER TASKS")
		}
		fmt.Println()
		for _, t := range blocking {
			blocksStr := strings.Join(t.Blocks[:min(3, len(t.Blocks))], ", ")
			if len(t.Blocks) > 3 {
				blocksStr += fmt.Sprintf(" +%d more", len(t.Blocks)-3)
			}
			fmt.Fprintf(w, "  %s\t%s\t→ blocks: %s\n", t.ID, truncate(t.Title, 30), blocksStr)
		}
		w.Flush()
		fmt.Println()
	}

	// Show blocked tasks
	if len(blocked) > 0 {
		if plain {
			fmt.Println("BLOCKED")
		} else {
			fmt.Println("🚫 BLOCKED")
		}
		fmt.Println()
		for _, t := range blocked {
			unmet := t.GetUnmetDependencies(taskMap)
			waitingStr := strings.Join(unmet[:min(3, len(unmet))], ", ")
			if len(unmet) > 3 {
				waitingStr += fmt.Sprintf(" +%d more", len(unmet)-3)
			}
			fmt.Fprintf(w, "  %s\t%s\t← waiting on: %s\n", t.ID, truncate(t.Title, 30), waitingStr)
		}
		w.Flush()
		fmt.Println()
	}

	// Summary
	fmt.Printf("─── %d tasks: %d blocking, %d blocked, %d independent ───\n",
		len(allTasks), len(blocking), len(blocked), len(independent))

	return nil
}
