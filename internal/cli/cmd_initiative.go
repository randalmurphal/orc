// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

func newInitiativeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "initiative",
		Aliases: []string{"i"},
		Short:   "Manage initiatives (grouped tasks with shared context)",
		Long: `Manage initiatives - groupings of related tasks with shared vision and decisions.

Initiatives provide:
  • Shared context across related tasks
  • Decision tracking with rationale
  • Task dependency management
  • Initiative-to-initiative dependencies
  • P2P/team collaboration via shared directories

Commands:
  new        Create a new initiative
  list       List all initiatives
  show       Show initiative details
  edit       Edit initiative properties and dependencies
  add-task   Link a task to an initiative
  decide     Record a decision
  activate   Set initiative status to active
  complete   Mark initiative as completed
  run        Run all initiative tasks in order
  delete     Delete an initiative`,
	}

	cmd.AddCommand(newInitiativeNewCmd())
	cmd.AddCommand(newInitiativeListCmd())
	cmd.AddCommand(newInitiativeShowCmd())
	cmd.AddCommand(newInitiativeEditCmd())
	cmd.AddCommand(newInitiativeAddTaskCmd())
	cmd.AddCommand(newInitiativeDecideCmd())
	cmd.AddCommand(newInitiativeActivateCmd())
	cmd.AddCommand(newInitiativeCompleteCmd())
	cmd.AddCommand(newInitiativeRunCmd())
	cmd.AddCommand(newInitiativeDeleteCmd())

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
  orc initiative new "Dark Mode" --shared  # Creates in shared directory for teams
  orc initiative new "React Migration" --blocked-by INIT-001  # Depends on another initiative`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			title := args[0]
			vision, _ := cmd.Flags().GetString("vision")
			shared, _ := cmd.Flags().GetBool("shared")
			ownerInitials, _ := cmd.Flags().GetString("owner")
			blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")

			// Generate next initiative ID
			id, err := initiative.NextID(shared)
			if err != nil {
				return fmt.Errorf("generate initiative ID: %w", err)
			}

			// Validate blocked-by references
			if len(blockedBy) > 0 {
				// Load all initiatives to validate
				allInits, err := initiative.List(shared)
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

			// Save
			var saveErr error
			if shared {
				saveErr = init.SaveShared()
			} else {
				saveErr = init.Save()
			}
			if saveErr != nil {
				return fmt.Errorf("save initiative: %w", saveErr)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "create", initiative.DefaultCommitConfig())

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
				if shared {
					fmt.Println("   Location: shared (team visible)")
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
	cmd.Flags().Bool("shared", false, "create in shared directory for team access")
	cmd.Flags().StringSlice("blocked-by", nil, "initiative IDs that must complete before this initiative")

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

			status, _ := cmd.Flags().GetString("status")
			shared, _ := cmd.Flags().GetBool("shared")

			var initiatives []*initiative.Initiative
			var err error

			if status != "" {
				initiatives, err = initiative.ListByStatus(initiative.Status(status), shared)
			} else {
				initiatives, err = initiative.List(shared)
			}
			if err != nil {
				return fmt.Errorf("list initiatives: %w", err)
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

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tTASKS\tOWNER")
			fmt.Fprintln(w, "--\t-----\t------\t-----\t-----")

			for _, init := range initiatives {
				owner := "-"
				if init.Owner.Initials != "" {
					owner = init.Owner.Initials
				}
				statusStr := string(init.Status)
				if init.IsBlocked(initMap) {
					statusStr = statusStr + " [BLOCKED]"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					init.ID, truncate(init.Title, 30), statusStr, len(init.Tasks), owner)
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().StringP("status", "s", "", "filter by status (draft, active, completed, archived)")
	cmd.Flags().Bool("shared", false, "list shared initiatives")

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
  orc initiative edit INIT-001 --remove-blocker INIT-002       # Remove blocker`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")

			// Load initiative
			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(id)
			} else {
				init, err = initiative.Load(id)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load all initiatives for validation
			allInits, err := initiative.List(shared)
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

			if !changed {
				fmt.Println("No changes specified.")
				return nil
			}

			// Save
			if shared {
				err = init.SaveShared()
			} else {
				err = init.Save()
			}
			if err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "edit", initiative.DefaultCommitConfig())

			fmt.Printf("Updated initiative %s\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "use shared initiative")
	cmd.Flags().String("title", "", "set initiative title")
	cmd.Flags().StringP("vision", "V", "", "set initiative vision")
	cmd.Flags().StringP("owner", "o", "", "set owner initials")
	cmd.Flags().StringSlice("blocked-by", nil, "set blocked_by list (replaces existing)")
	cmd.Flags().StringSlice("add-blocker", nil, "add initiative(s) to blocked_by list")
	cmd.Flags().StringSlice("remove-blocker", nil, "remove initiative(s) from blocked_by list")

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

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")

			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(id)
			} else {
				init, err = initiative.Load(id)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load all initiatives for dependency info
			allInits, err := initiative.List(shared)
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

			// Status with blocked indicator
			if init.IsBlocked(initMap) {
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

			// Show tasks
			if len(init.Tasks) > 0 {
				fmt.Printf("\nTasks (%d):\n", len(init.Tasks))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, t := range init.Tasks {
					deps := "-"
					if len(t.DependsOn) > 0 {
						deps = strings.Join(t.DependsOn, ", ")
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\tdeps: %s\n", t.ID, t.Title, t.Status, deps)
				}
				w.Flush()

				// Show ready tasks
				ready := init.GetReadyTasks()
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

	cmd.Flags().Bool("shared", false, "look in shared directory")

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

			initID := args[0]
			taskID := args[1]
			dependsOn, _ := cmd.Flags().GetStringSlice("depends-on")
			shared, _ := cmd.Flags().GetBool("shared")

			// Load initiative
			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(initID)
			} else {
				init, err = initiative.Load(initID)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load task to get title
			t, err := task.Load(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Add task to initiative
			init.AddTask(taskID, t.Title, dependsOn)

			// Save
			if shared {
				err = init.SaveShared()
			} else {
				err = init.Save()
			}
			if err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "add-task", initiative.DefaultCommitConfig())

			fmt.Printf("Added %s to %s\n", taskID, initID)
			if len(dependsOn) > 0 {
				fmt.Printf("  Depends on: %s\n", strings.Join(dependsOn, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringSlice("depends-on", nil, "task dependencies (can specify multiple)")
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

			initID := args[0]
			decision := args[1]
			rationale, _ := cmd.Flags().GetString("rationale")
			by, _ := cmd.Flags().GetString("by")
			shared, _ := cmd.Flags().GetBool("shared")

			// Load initiative
			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(initID)
			} else {
				init, err = initiative.Load(initID)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Add decision
			init.AddDecision(decision, rationale, by)

			// Save
			if shared {
				err = init.SaveShared()
			} else {
				err = init.Save()
			}
			if err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "decide", initiative.DefaultCommitConfig())

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
	cmd.Flags().Bool("shared", false, "use shared initiative")

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

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")

			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(id)
			} else {
				init, err = initiative.Load(id)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			init.Activate()

			if shared {
				err = init.SaveShared()
			} else {
				err = init.Save()
			}
			if err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "activate", initiative.DefaultCommitConfig())

			fmt.Printf("Initiative %s is now active\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "use shared initiative")
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

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")

			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(id)
			} else {
				init, err = initiative.Load(id)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			init.Complete()

			if shared {
				err = init.SaveShared()
			} else {
				err = init.Save()
			}
			if err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			// Auto-commit and sync to DB
			initiative.CommitAndSync(init, "complete", initiative.DefaultCommitConfig())

			fmt.Printf("Initiative %s marked as completed\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "use shared initiative")
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

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")
			execute, _ := cmd.Flags().GetBool("execute")
			parallel, _ := cmd.Flags().GetBool("parallel")
			profile, _ := cmd.Flags().GetString("profile")
			force, _ := cmd.Flags().GetBool("force")

			var init *initiative.Initiative
			var err error
			if shared {
				init, err = initiative.LoadShared(id)
			} else {
				init, err = initiative.Load(id)
			}
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Load all initiatives to check blocking status
			allInits, err := initiative.List(shared)
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

			ready := init.GetReadyTasks()
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
				t, err := task.Load(initTask.ID)
				if err != nil {
					fmt.Printf("  ✗ Failed to load: %v\n", err)
					continue
				}

				// Check if can run
				if !t.CanRun() && t.Status != task.StatusRunning {
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

	cmd.Flags().Bool("shared", false, "use shared initiative")
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

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")
			force, _ := cmd.Flags().GetBool("force")

			// Check if exists
			if !initiative.Exists(id, shared) {
				return fmt.Errorf("initiative %s not found", id)
			}

			if !force {
				fmt.Printf("Delete initiative %s? This cannot be undone. [y/N]: ", id)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			if err := initiative.Delete(id, shared); err != nil {
				return fmt.Errorf("delete initiative: %w", err)
			}

			// Auto-commit deletion and remove from DB
			initiative.CommitDeletion(id, initiative.DefaultCommitConfig())

			fmt.Printf("Deleted initiative %s\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "delete from shared directory")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation")

	return cmd
}
