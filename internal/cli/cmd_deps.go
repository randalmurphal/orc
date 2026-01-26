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

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// DepsOutput represents the JSON output structure for dependencies.
type DepsOutput struct {
	TaskID       string           `json:"task_id"`
	Title        string           `json:"title"`
	Status       orcv1.TaskStatus `json:"status"`
	BlockedBy    []DepsTaskInfo   `json:"blocked_by,omitempty"`
	Blocks       []DepsTaskInfo   `json:"blocks,omitempty"`
	RelatedTo    []DepsTaskInfo   `json:"related_to,omitempty"`
	ReferencedBy []DepsTaskInfo   `json:"referenced_by,omitempty"`
	Summary      DepsSummary      `json:"summary"`
}

// DepsTaskInfo contains information about a related task.
type DepsTaskInfo struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Status orcv1.TaskStatus `json:"status"`
}

// DepsSummary provides a status summary.
type DepsSummary struct {
	IsBlocked       bool `json:"is_blocked"`
	UnmetBlockers   int  `json:"unmet_blockers"`
	TotalBlockers   int  `json:"total_blockers"`
	TasksBlocking   int  `json:"tasks_blocking"`
	RelatedCount    int  `json:"related_count"`
	ReferencedCount int  `json:"referenced_count"`
}

func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps [task-id]",
		Short: "Show task dependencies and blocking relationships",
		Long: `Show dependencies for a task, including what it's waiting on and what it blocks.

Dependency types:
  blocked_by     Tasks that must complete before this task can run
  blocks         Tasks waiting on this task (computed from their blocked_by)
  related_to     Informational links to other tasks
  referenced_by  Tasks that reference this task (computed from their related_to)

View modes:
  Default        Single task's dependencies with blocking status
  No args        Overview of all blocked/blocking tasks
  --tree         Recursive dependency tree (shows what to complete first)
  --graph        ASCII visualization of dependency relationships

Understanding the output:
  ● (filled)     Dependency is completed
  ○ (empty)      Dependency is not yet completed
  BLOCKED        Task has unmet dependencies (cannot run)
  READY          All dependencies completed (can run)
  "← start here" No dependencies - good place to begin work

Quality tips:
  • Use 'orc deps' (no args) to see what's blocking progress
  • Use --tree to find the "root" tasks to complete first
  • Use --graph to visualize complex dependency chains
  • Tasks shown as "BLOCKING OTHER TASKS" should be prioritized

Examples:
  orc deps                     # Overview: blocked vs blocking tasks
  orc deps TASK-062            # Single task's dependencies
  orc deps TASK-062 --tree     # Full dependency tree (recursive)
  orc deps --graph             # ASCII graph of all dependencies
  orc deps --graph -i INIT-001 # Graph for specific initiative
  orc deps TASK-062 --json     # JSON output for scripting

See also:
  orc status   - Task status overview (includes dependency info)
  orc run      - Execute a task (checks dependencies first)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			treeView, _ := cmd.Flags().GetBool("tree")
			graphView, _ := cmd.Flags().GetBool("graph")
			initFilter, _ := cmd.Flags().GetString("initiative")

			// Load all tasks for dependency computation
			allTasks, err := backend.LoadAllTasks()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			if len(allTasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			// Populate computed fields (blocks, referenced_by)
			task.PopulateComputedFieldsProto(allTasks)

			// Build task map
			taskMap := make(map[string]*orcv1.Task)
			for _, t := range allTasks {
				taskMap[t.Id] = t
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

func showDepsHuman(t *orcv1.Task, taskMap map[string]*orcv1.Task) error {
	fmt.Printf("\n%s: %s\n", t.Id, t.Title)
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
				status = blocker.Status.String()
				if blocker.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
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
				status = blocked.Status.String()
				if blocked.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
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
	unmet := task.GetUnmetDependenciesProto(t, taskMap)
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

func showDepsJSON(t *orcv1.Task, taskMap map[string]*orcv1.Task) error {
	output := DepsOutput{
		TaskID: t.Id,
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
	unmet := task.GetUnmetDependenciesProto(t, taskMap)
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

func showDependencyTree(t *orcv1.Task, taskMap map[string]*orcv1.Task) error {
	fmt.Printf("\n%s: %s\n", t.Id, truncate(t.Title, 40))

	seen := make(map[string]bool)
	seen[t.Id] = true

	printTree(t, taskMap, "", true, seen)
	return nil
}

func printTree(t *orcv1.Task, taskMap map[string]*orcv1.Task, prefix string, _ bool, seen map[string]bool) {
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
		} else if exists && blocker.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
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

func showDependencyGraph(allTasks []*orcv1.Task, taskMap map[string]*orcv1.Task, initFilter string) error {
	// Filter tasks by initiative if specified
	var filteredTasks []*orcv1.Task
	if initFilter != "" {
		// When filtering by initiative, just check if any tasks have that initiative
		// (no need to verify initiative exists separately - if no tasks, we report that)
		for _, t := range allTasks {
			if t.InitiativeId != nil && *t.InitiativeId == initFilter {
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
		filteredIDs[t.Id] = true
	}

	// Find root tasks (no dependencies or all deps outside filter)
	var roots []*orcv1.Task

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
		}
	}

	// Sort roots by ID
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Id < roots[j].Id
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
		if printed[root.Id] {
			continue
		}
		printGraphNode(root, taskMap, filteredIDs, printed, "")
		fmt.Println()
	}

	// Print any orphaned tasks (shouldn't happen but just in case)
	for _, t := range filteredTasks {
		if !printed[t.Id] {
			fmt.Printf("%s (orphaned)\n", t.Id)
		}
	}

	return nil
}

func printGraphNode(t *orcv1.Task, taskMap map[string]*orcv1.Task, filteredIDs map[string]bool, printed map[string]bool, indent string) {
	if printed[t.Id] {
		return
	}
	printed[t.Id] = true

	// Find downstream tasks (tasks blocked by this one) within the filter
	var downstream []*orcv1.Task
	for _, other := range taskMap {
		if !filteredIDs[other.Id] {
			continue
		}
		for _, depID := range other.BlockedBy {
			if depID == t.Id {
				downstream = append(downstream, other)
				break
			}
		}
	}

	// Sort downstream by ID
	sort.Slice(downstream, func(i, j int) bool {
		return downstream[i].Id < downstream[j].Id
	})

	// Print this node
	var status string
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		status = " ✓"
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		status = " ◐"
	}

	if len(downstream) == 0 {
		fmt.Printf("%s%s%s\n", indent, t.Id, status)
		return
	}

	// Print with children
	if indent == "" {
		fmt.Printf("%s%s\n", t.Id, status)
	} else {
		fmt.Printf("%s%s%s\n", indent, t.Id, status)
	}

	for i, child := range downstream {
		isLast := i == len(downstream)-1
		childPrefix := "├─> "
		nextIndent := indent + "│   "
		if isLast {
			childPrefix = "└─> "
			nextIndent = indent + "    "
		}

		if printed[child.Id] {
			fmt.Printf("%s%s%s (see above)\n", indent, childPrefix, child.Id)
			continue
		}

		// Print inline if single chain
		chain := getChain(child, taskMap, filteredIDs, printed)
		if len(chain) > 0 {
			var chainStr []string
			for _, c := range chain {
				s := c.Id
				switch c.Status {
				case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
					s += " ✓"
				case orcv1.TaskStatus_TASK_STATUS_RUNNING:
					s += " ◐"
				}
				chainStr = append(chainStr, s)
				printed[c.Id] = true
			}
			fmt.Printf("%s%s%s\n", indent, childPrefix, strings.Join(chainStr, " ─> "))
		} else {
			fmt.Printf("%s%s", indent, childPrefix)
			printGraphNode(child, taskMap, filteredIDs, printed, nextIndent)
		}
	}
}

// getChain follows a single path of dependencies (no forks)
func getChain(t *orcv1.Task, taskMap map[string]*orcv1.Task, filteredIDs map[string]bool, printed map[string]bool) []*orcv1.Task {
	var chain []*orcv1.Task
	current := t

	for !printed[current.Id] {
		chain = append(chain, current)

		// Find downstream tasks
		var downstream []*orcv1.Task
		for _, other := range taskMap {
			if !filteredIDs[other.Id] || printed[other.Id] {
				continue
			}
			for _, depID := range other.BlockedBy {
				if depID == current.Id {
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

func showDependencyOverview(allTasks []*orcv1.Task, taskMap map[string]*orcv1.Task) error {
	// Categorize tasks by blocking status
	var blocked, blocking, independent []*orcv1.Task

	for _, t := range allTasks {
		unmet := task.GetUnmetDependenciesProto(t, taskMap)
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
			_, _ = fmt.Fprintf(w, "  %s\t%s\t→ blocks: %s\n", t.Id, truncate(t.Title, 30), blocksStr)
		}
		_ = w.Flush()
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
			unmet := task.GetUnmetDependenciesProto(t, taskMap)
			waitingStr := strings.Join(unmet[:min(3, len(unmet))], ", ")
			if len(unmet) > 3 {
				waitingStr += fmt.Sprintf(" +%d more", len(unmet)-3)
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\t← waiting on: %s\n", t.Id, truncate(t.Title, 30), waitingStr)
		}
		_ = w.Flush()
		fmt.Println()
	}

	// Summary
	fmt.Printf("─── %d tasks: %d blocking, %d blocked, %d independent ───\n",
		len(allTasks), len(blocking), len(blocked), len(independent))

	return nil
}
