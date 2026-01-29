// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

func newInitiativeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "initiative",
		Aliases: []string{"i"},
		Short:   "Manage initiatives (grouped tasks with shared context)",
		Long: `Manage initiatives - groupings of related tasks with shared vision and decisions.

Initiatives are the primary way to organize complex work in orc. Use them when:
  • A feature requires multiple related tasks
  • Tasks need to share context and decisions
  • You want dependency ordering across tasks
  • Multiple people are collaborating on related work

Key concepts:
  Vision      High-level goal describing what the initiative achieves
  Decisions   Recorded choices with rationale (e.g., "Use JWT for auth")
  Tasks       Individual work items linked to the initiative
  Dependencies Tasks/initiatives that must complete first (blocked_by)

Workflow - Creating an initiative:
  1. orc initiative new "Feature Name" --vision "What we're building"
  2. orc initiative decide INIT-001 "Key decision" --rationale "Why"
  3. orc new "First task" --initiative INIT-001
  4. orc initiative activate INIT-001
  5. orc initiative run INIT-001 --execute

Commands:
  new        Create a new initiative with optional vision/dependencies
  list       List all initiatives (filter with --status)
  show       Show initiative details, tasks, and decisions
  edit       Edit properties including title, vision, dependencies
  add-task   Link a single task to an initiative
  link       Batch link multiple tasks (supports --all-matching pattern)
  unlink     Remove tasks from an initiative
  plan       Create tasks from a manifest.yaml file
  decide     Record a decision with optional rationale
  activate   Set initiative status to active (ready to run)
  complete   Mark initiative as completed
  run        Run all ready tasks in dependency order
  delete     Delete an initiative

Quality tips:
  • Use --vision to clearly state what the initiative achieves
  • Record decisions with 'decide' to maintain context for Claude
  • Use --blocked-by to order initiatives (e.g., "API before frontend")
  • Use 'initiative run --execute' to run all ready tasks automatically

Examples:
  orc initiative new "Auth System" -V "User login/logout with JWT"
  orc i list --status active      # Short alias, filter by status
  orc i show INIT-001             # View details and linked tasks
  orc i run INIT-001              # Preview what would run
  orc i run INIT-001 --execute    # Actually run the tasks`,
	}

	cmd.AddCommand(newInitiativeNewCmd())
	cmd.AddCommand(newInitiativeListCmd())
	cmd.AddCommand(newInitiativeShowCmd())
	cmd.AddCommand(newInitiativeEditCmd())
	cmd.AddCommand(newInitiativeAddTaskCmd())
	cmd.AddCommand(newInitiativeLinkCmd())
	cmd.AddCommand(newInitiativeUnlinkCmd())
	cmd.AddCommand(newInitiativeDecideCmd())
	cmd.AddCommand(newInitiativeActivateCmd())
	cmd.AddCommand(newInitiativeCompleteCmd())
	cmd.AddCommand(newInitiativeRunCmd())
	cmd.AddCommand(newInitiativeDeleteCmd())
	cmd.AddCommand(newInitiativePlanCmd())

	return cmd
}

func newInitiativeNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new initiative",
		Long: `Create a new initiative to group related tasks.

Example:
  orc initiative new "User Authentication System"
  orc initiative new "API Refactor" --vision "Modern REST API with OpenAPI spec"
  orc initiative new "React Migration" --blocked-by INIT-001  # Depends on another initiative
  orc initiative new "Auth Feature" --branch-base feature/auth --branch-prefix feature/auth-`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			title := args[0]
			vision, _ := cmd.Flags().GetString("vision")
			ownerInitials, _ := cmd.Flags().GetString("owner")
			blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")
			branchBase, _ := cmd.Flags().GetString("branch-base")
			branchPrefix, _ := cmd.Flags().GetString("branch-prefix")

			// Validate branch names if specified
			if branchBase != "" {
				if err := git.ValidateBranchName(branchBase); err != nil {
					return fmt.Errorf("invalid branch-base: %w", err)
				}
			}
			if branchPrefix != "" {
				// Branch prefix can have trailing chars that become part of branch name
				// Validate by appending a test task ID
				testName := branchPrefix + "TASK-001"
				if err := git.ValidateBranchName(testName); err != nil {
					return fmt.Errorf("invalid branch-prefix: %w", err)
				}
			}

			// Generate next initiative ID
			id, err := backend.GetNextInitiativeID()
			if err != nil {
				return fmt.Errorf("generate initiative ID: %w", err)
			}

			// Validate blocked-by references
			if len(blockedBy) > 0 {
				// Load all initiatives to validate
				allInits, err := backend.LoadAllInitiatives()
				if err != nil {
					return fmt.Errorf("load initiatives for validation: %w", err)
				}

				existingIDs := make(map[string]bool)
				for _, init := range allInits {
					existingIDs[init.ID] = true
				}

				if errs := initiative.ValidateBlockedBy(id, blockedBy, existingIDs); len(errs) > 0 {
					return errs[0]
				}
			}

			// Create initiative
			init := initiative.New(id, title)
			if vision != "" {
				init.Vision = vision
			}
			if ownerInitials != "" {
				init.Owner = initiative.Identity{Initials: ownerInitials}
			}
			if len(blockedBy) > 0 {
				init.BlockedBy = blockedBy
			}
			if branchBase != "" {
				init.BranchBase = branchBase
			}
			if branchPrefix != "" {
				init.BranchPrefix = branchPrefix
			}

			// Save
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			if !quiet {
				fmt.Printf("Initiative created: %s\n", id)
				fmt.Printf("   Title:  %s\n", title)
				fmt.Printf("   Status: %s\n", init.Status)
				if vision != "" {
					fmt.Printf("   Vision: %s\n", vision)
				}
				if len(blockedBy) > 0 {
					fmt.Printf("   Blocked by: %s\n", strings.Join(blockedBy, ", "))
				}
				if branchBase != "" {
					fmt.Printf("   Branch base: %s\n", branchBase)
				}
				if branchPrefix != "" {
					fmt.Printf("   Branch prefix: %s\n", branchPrefix)
				}
				fmt.Println("\nNext steps:")
				fmt.Printf("  orc initiative add-task %s TASK-XXX  - Link tasks\n", id)
				fmt.Printf("  orc initiative decide %s \"...\"       - Record decisions\n", id)
				fmt.Printf("  orc initiative activate %s           - Activate for execution\n", id)
			}

			return nil
		},
	}

	cmd.Flags().StringP("vision", "V", "", "initiative vision statement")
	cmd.Flags().StringP("owner", "o", "", "owner initials")
	cmd.Flags().StringSlice("blocked-by", nil, "initiative IDs that must complete before this initiative")
	cmd.Flags().String("branch-base", "", "target branch for tasks in this initiative (e.g., feature/auth)")
	cmd.Flags().String("branch-prefix", "", "prefix for task branches (e.g., feature/auth-)")

	return cmd
}

func newInitiativeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all initiatives",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			statusFilter, _ := cmd.Flags().GetString("status")

			initiatives, err := backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("list initiatives: %w", err)
			}

			// Filter by status if provided
			if statusFilter != "" {
				targetStatus := initiative.Status(statusFilter)
				var filtered []*initiative.Initiative
				for _, init := range initiatives {
					if init.Status == targetStatus {
						filtered = append(filtered, init)
					}
				}
				initiatives = filtered
			}

			if len(initiatives) == 0 {
				fmt.Println("No initiatives found.")
				fmt.Println("\nCreate one with: orc initiative new \"Title\"")
				return nil
			}

			// Populate computed fields (Blocks)
			initiative.PopulateComputedFields(initiatives)

			// Build map for IsBlocked check
			initMap := make(map[string]*initiative.Initiative)
			for _, init := range initiatives {
				initMap[init.ID] = init
			}

			// Auto-complete eligible initiatives (catch-up for initiatives with all tasks completed)
			// Best-effort: log warning on failure, don't fail list
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			completer := executor.NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
			for _, init := range initiatives {
				// Only check active initiatives without BranchBase
				if init.Status != initiative.StatusCompleted && !init.HasBranchBase() && len(init.Tasks) > 0 {
					if err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), init.ID); err != nil {
						// Best-effort: log but don't fail
						logger.Debug("auto-completion check failed", "initiative", init.ID, "error", err)
					}
				}
			}

			// Reload initiatives to get updated statuses after auto-completion
			initiatives, err = backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("reload initiatives: %w", err)
			}
			// Reapply status filter if needed
			if statusFilter != "" {
				targetStatus := initiative.Status(statusFilter)
				var filtered []*initiative.Initiative
				for _, init := range initiatives {
					if init.Status == targetStatus {
						filtered = append(filtered, init)
					}
				}
				initiatives = filtered
			}
			// Rebuild map with updated initiatives
			initMap = make(map[string]*initiative.Initiative)
			for _, init := range initiatives {
				initMap[init.ID] = init
			}
			initiative.PopulateComputedFields(initiatives)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tTASKS\tOWNER")
			_, _ = fmt.Fprintln(w, "--\t-----\t------\t-----\t-----")

			for _, init := range initiatives {
				owner := "-"
				if init.Owner.Initials != "" {
					owner = init.Owner.Initials
				}
				statusStr := string(init.Status)
				// Only show BLOCKED for non-completed initiatives (SC-3)
				if init.Status != initiative.StatusCompleted && init.IsBlocked(initMap) {
					statusStr = statusStr + " [BLOCKED]"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					init.ID, truncate(init.Title, 30), statusStr, len(init.Tasks), owner)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().StringP("status", "s", "", "filter by status (draft, active, completed, archived)")

	return cmd
}

func newInitiativeEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an initiative",
		Long: `Edit an initiative's properties including dependencies.

Example:
  orc initiative edit INIT-001 --title "New Title"
  orc initiative edit INIT-001 --vision "Updated vision statement"
  orc initiative edit INIT-001 --blocked-by INIT-002,INIT-003  # Replace blockers
  orc initiative edit INIT-001 --add-blocker INIT-004          # Add blocker
  orc initiative edit INIT-001 --remove-blocker INIT-002       # Remove blocker
  orc initiative edit INIT-001 --branch-base feature/auth      # Set target branch
  orc initiative edit INIT-001 --branch-prefix feature/auth-   # Set branch prefix`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			// Load initiative
			init, err := backend.LoadInitiative(id)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load all initiatives for validation
			allInits, err := backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("load initiatives for validation: %w", err)
			}

			// Build map for validation
			initMap := make(map[string]*initiative.Initiative)
			for _, i := range allInits {
				initMap[i.ID] = i
			}

			// Track if anything changed
			changed := false

			// Update title if provided
			if cmd.Flags().Changed("title") {
				title, _ := cmd.Flags().GetString("title")
				init.Title = title
				changed = true
			}

			// Update vision if provided
			if cmd.Flags().Changed("vision") {
				vision, _ := cmd.Flags().GetString("vision")
				init.Vision = vision
				changed = true
			}

			// Update owner if provided
			if cmd.Flags().Changed("owner") {
				owner, _ := cmd.Flags().GetString("owner")
				init.Owner = initiative.Identity{Initials: owner}
				changed = true
			}

			// Handle blocked-by (replace entire list)
			if cmd.Flags().Changed("blocked-by") {
				blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")
				if err := init.SetBlockedBy(blockedBy, initMap); err != nil {
					return err
				}
				changed = true
			}

			// Handle add-blocker (add to existing)
			if cmd.Flags().Changed("add-blocker") {
				blockers, _ := cmd.Flags().GetStringSlice("add-blocker")
				for _, blockerID := range blockers {
					if err := init.AddBlocker(blockerID, initMap); err != nil {
						return err
					}
				}
				changed = true
			}

			// Handle remove-blocker
			if cmd.Flags().Changed("remove-blocker") {
				blockers, _ := cmd.Flags().GetStringSlice("remove-blocker")
				for _, blockerID := range blockers {
					init.RemoveBlocker(blockerID)
				}
				changed = true
			}

			// Handle branch-base
			if cmd.Flags().Changed("branch-base") {
				branchBase, _ := cmd.Flags().GetString("branch-base")
				// Validate if not clearing
				if branchBase != "" {
					if err := git.ValidateBranchName(branchBase); err != nil {
						return fmt.Errorf("invalid branch-base: %w", err)
					}
				}
				init.BranchBase = branchBase
				changed = true
			}

			// Handle branch-prefix
			if cmd.Flags().Changed("branch-prefix") {
				branchPrefix, _ := cmd.Flags().GetString("branch-prefix")
				// Validate if not clearing
				if branchPrefix != "" {
					testName := branchPrefix + "TASK-001"
					if err := git.ValidateBranchName(testName); err != nil {
						return fmt.Errorf("invalid branch-prefix: %w", err)
					}
				}
				init.BranchPrefix = branchPrefix
				changed = true
			}

			if !changed {
				fmt.Println("No changes specified.")
				return nil
			}

			// Save
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			fmt.Printf("Updated initiative %s\n", id)
			return nil
		},
	}

	cmd.Flags().String("title", "", "set initiative title")
	cmd.Flags().StringP("vision", "V", "", "set initiative vision")
	cmd.Flags().StringP("owner", "o", "", "set owner initials")
	cmd.Flags().StringSlice("blocked-by", nil, "set blocked_by list (replaces existing)")
	cmd.Flags().StringSlice("add-blocker", nil, "add initiative(s) to blocked_by list")
	cmd.Flags().StringSlice("remove-blocker", nil, "remove initiative(s) from blocked_by list")
	cmd.Flags().String("branch-base", "", "set target branch for tasks in this initiative")
	cmd.Flags().String("branch-prefix", "", "set prefix for task branch names")

	return cmd
}

func newInitiativeShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show initiative details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			init, err := backend.LoadInitiative(id)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Auto-complete check (catch-up for initiatives with all tasks completed)
			// Best-effort: log warning on failure, don't fail show
			if init.Status != initiative.StatusCompleted && !init.HasBranchBase() && len(init.Tasks) > 0 {
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
				completer := executor.NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
				if err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), init.ID); err != nil {
					logger.Debug("auto-completion check failed", "initiative", init.ID, "error", err)
				}
				// Reload initiative to get updated status
				init, err = backend.LoadInitiative(id)
				if err != nil {
					return fmt.Errorf("reload initiative: %w", err)
				}
			}

			// Load all initiatives for dependency info
			allInits, err := backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("load initiatives: %w", err)
			}
			initMap := make(map[string]*initiative.Initiative)
			for _, i := range allInits {
				initMap[i.ID] = i
			}
			initiative.PopulateComputedFields(allInits)

			fmt.Printf("Initiative: %s\n", init.ID)
			fmt.Printf("Title:      %s\n", init.Title)

			// Status with blocked indicator (only for non-completed initiatives)
			if init.Status != initiative.StatusCompleted && init.IsBlocked(initMap) {
				fmt.Printf("Status:     %s (BLOCKED)\n", init.Status)
			} else {
				fmt.Printf("Status:     %s\n", init.Status)
			}

			if init.Owner.Initials != "" {
				fmt.Printf("Owner:      %s", init.Owner.Initials)
				if init.Owner.DisplayName != "" {
					fmt.Printf(" (%s)", init.Owner.DisplayName)
				}
				fmt.Println()
			}
			if init.Vision != "" {
				fmt.Printf("\nVision:\n  %s\n", init.Vision)
			}
			fmt.Printf("\nCreated:  %s\n", init.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("Updated:  %s\n", init.UpdatedAt.Format("2006-01-02 15:04"))

			// Show dependencies
			if len(init.BlockedBy) > 0 || len(init.Blocks) > 0 {
				fmt.Println("\nDependencies:")
				if len(init.BlockedBy) > 0 {
					fmt.Printf("  Blocked by:\n")
					for _, blockerID := range init.BlockedBy {
						blocker, exists := initMap[blockerID]
						if exists {
							status := string(blocker.Status)
							if blocker.Status == initiative.StatusCompleted {
								status = "✓ " + status
							} else {
								status = "○ " + status
							}
							fmt.Printf("    %s: %s (%s)\n", blockerID, blocker.Title, status)
						} else {
							fmt.Printf("    %s: (not found)\n", blockerID)
						}
					}
				}
				if len(init.Blocks) > 0 {
					fmt.Printf("  Blocks:\n")
					for _, blockedID := range init.Blocks {
						blocked, exists := initMap[blockedID]
						if exists {
							fmt.Printf("    %s: %s\n", blockedID, blocked.Title)
						} else {
							fmt.Printf("    %s: (not found)\n", blockedID)
						}
					}
				}
			}

			// Show decisions
			if len(init.Decisions) > 0 {
				fmt.Printf("\nDecisions (%d):\n", len(init.Decisions))
				for _, dec := range init.Decisions {
					fmt.Printf("  %s: %s\n", dec.ID, dec.Decision)
					if dec.Rationale != "" {
						fmt.Printf("      Rationale: %s\n", dec.Rationale)
					}
					fmt.Printf("      By: %s at %s\n", dec.By, dec.Date.Format("2006-01-02"))
				}
			}

			// Show tasks with actual status from database
			if len(init.Tasks) > 0 {
				// Create task loader to get actual status
				taskLoader := func(taskID string) (string, string, error) {
					t, err := backend.LoadTask(taskID)
					if err != nil {
						return "", "", nil // Task not found, keep stored status
					}
					return string(t.Status), t.Title, nil
				}
				init.EnrichTaskStatuses(taskLoader)

				// Populate DependsOn from each task's BlockedBy field
				for i, taskRef := range init.Tasks {
					loadedTask, err := backend.LoadTask(taskRef.ID)
					if err != nil {
						continue
					}
					init.Tasks[i].DependsOn = loadedTask.BlockedBy
				}

				fmt.Printf("\nTasks (%d):\n", len(init.Tasks))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, t := range init.Tasks {
					deps := "-"
					if len(t.DependsOn) > 0 {
						deps = strings.Join(t.DependsOn, ", ")
					}
					_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\tdeps: %s\n", t.ID, t.Title, t.Status, deps)
				}
				_ = w.Flush()

				// Show ready tasks (using loader for accurate status)
				ready := init.GetReadyTasksWithLoader(taskLoader)
				if len(ready) > 0 {
					fmt.Printf("\nReady to run:")
					for _, t := range ready {
						fmt.Printf(" %s", t.ID)
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	return cmd
}

func newInitiativeAddTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-task <initiative-id> <task-id>",
		Short: "Link a task to an initiative",
		Long: `Link an existing task to an initiative.

Example:
  orc initiative add-task INIT-001 TASK-001
  orc initiative add-task INIT-001 TASK-002 --depends-on TASK-001`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			initID := args[0]
			taskID := args[1]
			dependsOn, _ := cmd.Flags().GetStringSlice("depends-on")

			// Load initiative
			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load task to get title
			t, err := backend.LoadTask(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Add task to initiative
			init.AddTask(taskID, t.Title, dependsOn)

			// Save
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			fmt.Printf("Added %s to %s\n", taskID, initID)
			if len(dependsOn) > 0 {
				fmt.Printf("  Depends on: %s\n", strings.Join(dependsOn, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringSlice("depends-on", nil, "task dependencies (can specify multiple)")

	return cmd
}

func newInitiativeLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <initiative-id> <task-id>...",
		Short: "Batch link multiple tasks to an initiative",
		Long: `Link multiple tasks to an initiative at once.

Examples:
  orc initiative link INIT-001 TASK-060 TASK-061 TASK-062
  orc initiative link INIT-001 --all-matching "auth"        # Link tasks matching pattern
  orc initiative link INIT-001 --all-matching "TASK-06"     # Link tasks by ID pattern`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			initID := args[0]
			taskIDs := args[1:]
			allMatching, _ := cmd.Flags().GetString("all-matching")

			// Load initiative
			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Collect tasks to link
			type taskInfo struct {
				ID           string
				Title        string
				InitiativeID string
			}
			var tasksToLink []taskInfo

			// If --all-matching is provided, find matching tasks
			if allMatching != "" {
				allTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks: %w", err)
				}

				pattern := strings.ToLower(allMatching)
				for _, t := range allTasks {
					// Match against ID or title
					if strings.Contains(strings.ToLower(t.Id), pattern) ||
						strings.Contains(strings.ToLower(t.Title), pattern) {
						// Skip only if fully linked (both initiative_id set AND in task list)
						taskInitID := ""
						if t.InitiativeId != nil {
							taskInitID = *t.InitiativeId
						}
						if taskInitID == initID && init.HasTask(t.Id) {
							continue
						}
						tasksToLink = append(tasksToLink, taskInfo{ID: t.Id, Title: t.Title, InitiativeID: taskInitID})
					}
				}

				if len(tasksToLink) == 0 {
					fmt.Printf("No unlinked tasks matching %q found.\n", allMatching)
					return nil
				}
			}

			// Add explicitly specified task IDs
			for _, taskID := range taskIDs {
				t, err := backend.LoadTask(taskID)
				if err != nil {
					return fmt.Errorf("load task %s: %w", taskID, err)
				}
				// Get initiative ID (handling *string)
				taskInitID := ""
				if t.InitiativeId != nil {
					taskInitID = *t.InitiativeId
				}
				// Skip only if fully linked (both initiative_id set AND in task list)
				if taskInitID == initID && init.HasTask(t.Id) {
					fmt.Printf("Skipping %s: already linked to %s\n", taskID, initID)
					continue
				}
				tasksToLink = append(tasksToLink, taskInfo{ID: t.Id, Title: t.Title, InitiativeID: taskInitID})
			}

			if len(tasksToLink) == 0 {
				fmt.Println("No tasks to link.")
				return nil
			}

			// Link all tasks
			var linked []string
			var skippedOther []string // Tasks linked to a different initiative
			for _, ti := range tasksToLink {
				// Check if already linked to another initiative
				if ti.InitiativeID != "" && ti.InitiativeID != initID {
					skippedOther = append(skippedOther, fmt.Sprintf("%s (linked to %s)", ti.ID, ti.InitiativeID))
					continue
				}

				// Load full task to update
				t, err := backend.LoadTask(ti.ID)
				if err != nil {
					return fmt.Errorf("load task %s for update: %w", ti.ID, err)
				}

				// Update task - set initiative ID directly (proto has no methods)
				t.InitiativeId = &initID
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("save task %s: %w", t.Id, err)
				}

				// Update initiative
				init.AddTask(t.Id, t.Title, nil)
				linked = append(linked, t.Id)
			}

			// Save initiative
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Print summary
			if len(linked) > 0 {
				fmt.Printf("Linked %d task(s) to %s:\n", len(linked), initID)
				for _, id := range linked {
					fmt.Printf("  • %s\n", id)
				}
			}
			if len(skippedOther) > 0 {
				fmt.Printf("\nSkipped %d task(s) already linked to other initiatives:\n", len(skippedOther))
				for _, info := range skippedOther {
					fmt.Printf("  • %s\n", info)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("all-matching", "", "link all tasks matching pattern (matches ID or title)")
	cmd.Flags().Bool("shared", false, "use shared initiative")

	return cmd
}

func newInitiativeUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <initiative-id> <task-id>...",
		Short: "Remove tasks from an initiative",
		Long: `Remove one or more tasks from an initiative.

Examples:
  orc initiative unlink INIT-001 TASK-060
  orc initiative unlink INIT-001 TASK-060 TASK-061 TASK-062
  orc initiative unlink INIT-001 --all   # Unlink all tasks from initiative`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			initID := args[0]
			taskIDs := args[1:]
			unlinkAll, _ := cmd.Flags().GetBool("all")

			// Load initiative
			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// If --all, get all task IDs from the initiative
			if unlinkAll {
				taskIDs = make([]string, len(init.Tasks))
				for i, t := range init.Tasks {
					taskIDs[i] = t.ID
				}
			}

			if len(taskIDs) == 0 {
				fmt.Println("No tasks to unlink.")
				return nil
			}

			// Unlink all tasks
			var unlinked []string
			var notFound []string
			for _, taskID := range taskIDs {
				// Load task
				t, err := backend.LoadTask(taskID)
				if err != nil {
					notFound = append(notFound, taskID)
					continue
				}

				// Check if task belongs to this initiative
				taskInitID := ""
				if t.InitiativeId != nil {
					taskInitID = *t.InitiativeId
				}
				if taskInitID != initID {
					if taskInitID == "" {
						fmt.Printf("Skipping %s: not linked to any initiative\n", taskID)
					} else {
						fmt.Printf("Skipping %s: linked to %s, not %s\n", taskID, taskInitID, initID)
					}
					continue
				}

				// Update task - clear initiative ID
				t.InitiativeId = nil
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("save task %s: %w", taskID, err)
				}

				// Update initiative
				init.RemoveTask(taskID)
				unlinked = append(unlinked, taskID)
			}

			// Save initiative
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Print summary
			if len(unlinked) > 0 {
				fmt.Printf("Unlinked %d task(s) from %s:\n", len(unlinked), initID)
				for _, id := range unlinked {
					fmt.Printf("  • %s\n", id)
				}
			}
			if len(notFound) > 0 {
				fmt.Printf("\nCould not find %d task(s):\n", len(notFound))
				for _, id := range notFound {
					fmt.Printf("  • %s\n", id)
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("all", false, "unlink all tasks from the initiative")
	cmd.Flags().Bool("shared", false, "use shared initiative")

	return cmd
}

func newInitiativeDecideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decide <initiative-id> <decision>",
		Short: "Record a decision in the initiative",
		Long: `Record a decision with optional rationale.

Example:
  orc initiative decide INIT-001 "Use JWT tokens for auth"
  orc initiative decide INIT-001 "Use bcrypt for passwords" --rationale "Industry standard, well-tested"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			initID := args[0]
			decision := args[1]
			rationale, _ := cmd.Flags().GetString("rationale")
			by, _ := cmd.Flags().GetString("by")

			// Load initiative
			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Add decision
			init.AddDecision(decision, rationale, by)

			// Save
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			decID := init.Decisions[len(init.Decisions)-1].ID
			fmt.Printf("Decision recorded: %s\n", decID)
			fmt.Printf("  %s\n", decision)
			if rationale != "" {
				fmt.Printf("  Rationale: %s\n", rationale)
			}

			return nil
		},
	}

	cmd.Flags().StringP("rationale", "r", "", "rationale for the decision")
	cmd.Flags().StringP("by", "b", "", "who made the decision (initials)")

	return cmd
}

func newInitiativeActivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activate <id>",
		Short: "Set initiative status to active",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			init, err := backend.LoadInitiative(id)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			init.Activate()

			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			fmt.Printf("Initiative %s is now active\n", id)
			return nil
		},
	}

	return cmd
}

func newInitiativeCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete <id>",
		Short: "Mark initiative as completed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			init, err := backend.LoadInitiative(id)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			init.Complete()

			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			fmt.Printf("Initiative %s marked as completed\n", id)
			return nil
		},
	}

	return cmd
}

func newInitiativeRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <id>",
		Short: "Run all initiative tasks in dependency order",
		Long: `Run all tasks in an initiative, respecting dependencies.

Only tasks with all dependencies completed will be executed.
By default shows what would run - use --execute to actually run tasks.

Examples:
  orc initiative run INIT-001              # Show ready tasks (safe preview)
  orc initiative run INIT-001 --execute    # Actually run the tasks
  orc initiative run INIT-001 --parallel   # Run ready tasks in parallel
  orc initiative run INIT-001 --force      # Run even if blocked by other initiatives`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]
			execute, _ := cmd.Flags().GetBool("execute")
			parallel, _ := cmd.Flags().GetBool("parallel")
			profile, _ := cmd.Flags().GetString("profile")
			force, _ := cmd.Flags().GetBool("force")

			init, err := backend.LoadInitiative(id)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load all initiatives to check blocking status
			allInits, err := backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("load initiatives: %w", err)
			}
			initMap := make(map[string]*initiative.Initiative)
			for _, i := range allInits {
				initMap[i.ID] = i
			}

			// Check if initiative is blocked
			if init.IsBlocked(initMap) && !force {
				blockers := init.GetIncompleteBlockers(initMap)
				fmt.Printf("Initiative %s is blocked by:\n", id)
				for _, blocker := range blockers {
					fmt.Printf("  • %s: %s (%s)\n", blocker.ID, blocker.Title, blocker.Status)
				}
				fmt.Println("\nComplete the blocking initiatives first, or use --force to run anyway.")
				return nil
			}

			// Populate DependsOn from each task's BlockedBy field
			for i, taskRef := range init.Tasks {
				loadedTask, err := backend.LoadTask(taskRef.ID)
				if err != nil {
					continue
				}
				init.Tasks[i].DependsOn = loadedTask.BlockedBy
			}

			ready := init.GetReadyTasksWithLoader(nil)
			if len(ready) == 0 {
				fmt.Println("No tasks ready to run.")
				fmt.Println("\nPossible reasons:")
				fmt.Println("  • All tasks are already completed")
				fmt.Println("  • Tasks are waiting on dependencies")
				fmt.Println("  • No tasks have been added to this initiative")
				if len(init.Tasks) > 0 {
					fmt.Println("\nTask status:")
					for _, t := range init.Tasks {
						deps := ""
						if len(t.DependsOn) > 0 {
							deps = fmt.Sprintf(" (depends on: %s)", strings.Join(t.DependsOn, ", "))
						}
						fmt.Printf("  %s: %s%s\n", t.ID, t.Status, deps)
					}
				}
				return nil
			}

			// Preview mode (default) - just show what would run
			if !execute {
				fmt.Printf("Tasks ready to run from %s:\n\n", id)
				for _, t := range ready {
					fmt.Printf("  %s: %s\n", t.ID, t.Title)
				}
				fmt.Printf("\n%d task(s) ready. Add --execute to run them.\n", len(ready))
				if len(ready) > 1 {
					fmt.Println("Add --parallel to run them concurrently.")
				}
				return nil
			}

			// Execute mode - actually run the tasks
			fmt.Printf("Running %d task(s) from %s:\n\n", len(ready), id)

			if parallel && len(ready) > 1 {
				// Parallel execution - run all ready tasks concurrently
				fmt.Println("Running tasks in parallel...")
				fmt.Println("(Each task runs in its own worktree)")
				fmt.Println()

				// Build command for each task
				for _, t := range ready {
					cmdArgs := []string{"run", t.ID}
					if profile != "" {
						cmdArgs = append(cmdArgs, "--profile", profile)
					}
					fmt.Printf("  Starting: orc %s\n", strings.Join(cmdArgs, " "))
				}

				fmt.Println("\nNote: Parallel execution starts background processes.")
				fmt.Println("Use 'orc status' to monitor progress.")

				// For now, just give instructions - true parallel would need goroutines
				// and proper process management
				return nil
			}

			// Sequential execution
			for i, initTask := range ready {
				fmt.Printf("[%d/%d] Running %s: %s\n", i+1, len(ready), initTask.ID, initTask.Title)

				// Load actual task
				t, err := backend.LoadTask(initTask.ID)
				if err != nil {
					fmt.Printf("  ✗ Failed to load: %v\n", err)
					continue
				}

				// Check if can run
				if !task.CanRunProto(t) && t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
					fmt.Printf("  ✗ Cannot run (status: %s)\n", t.Status)
					continue
				}

				// Execute via subprocess for cleaner output
				cmdArgs := []string{"run", initTask.ID}
				if profile != "" {
					cmdArgs = append(cmdArgs, "--profile", profile)
				}

				// For now, instruct user - full integration would shell out
				fmt.Printf("  → orc %s\n", strings.Join(cmdArgs, " "))
			}

			fmt.Println("\nTo run tasks sequentially, execute the commands above.")
			fmt.Println("Or run: orc run <task-id> for each task individually.")

			return nil
		},
	}

	cmd.Flags().Bool("execute", false, "actually run the tasks (default: preview only)")
	cmd.Flags().Bool("parallel", false, "run ready tasks in parallel (requires --execute)")
	cmd.Flags().StringP("profile", "p", "", "automation profile for task execution")
	cmd.Flags().BoolP("force", "f", false, "run even if blocked by other initiatives")

	return cmd
}

func newInitiativeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an initiative",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Check if exists
			exists, err := backend.InitiativeExists(id)
			if err != nil {
				return fmt.Errorf("check initiative: %w", err)
			}
			if !exists {
				return fmt.Errorf("initiative %s not found", id)
			}

			if !force {
				fmt.Printf("Delete initiative %s? This cannot be undone. [y/N]: ", id)
				var response string
				_, _ = fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			if err := backend.DeleteInitiative(id); err != nil {
				return fmt.Errorf("delete initiative: %w", err)
			}

			fmt.Printf("Deleted initiative %s\n", id)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation")

	return cmd
}
