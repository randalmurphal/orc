package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

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

			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			init.AddDecision(decision, rationale, by)
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

			allInits, err := backend.LoadAllInitiatives()
			if err != nil {
				return fmt.Errorf("load initiatives: %w", err)
			}
			initMap := make(map[string]*initiative.Initiative)
			for _, i := range allInits {
				initMap[i.ID] = i
			}

			if init.IsBlocked(initMap) && !force {
				blockers := init.GetIncompleteBlockers(initMap)
				fmt.Printf("Initiative %s is blocked by:\n", id)
				for _, blocker := range blockers {
					fmt.Printf("  • %s: %s (%s)\n", blocker.ID, blocker.Title, blocker.Status)
				}
				fmt.Println("\nComplete the blocking initiatives first, or use --force to run anyway.")
				return nil
			}

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

			fmt.Printf("Running %d task(s) from %s:\n\n", len(ready), id)

			if parallel && len(ready) > 1 {
				fmt.Println("Running tasks in parallel...")
				fmt.Println("(Each task runs in its own worktree)")
				fmt.Println()

				for _, t := range ready {
					cmdArgs := []string{"run", t.ID}
					if profile != "" {
						cmdArgs = append(cmdArgs, "--profile", profile)
					}
					fmt.Printf("  Starting: orc %s\n", strings.Join(cmdArgs, " "))
				}

				fmt.Println("\nNote: Parallel execution starts background processes.")
				fmt.Println("Use 'orc status' to monitor progress.")
				return nil
			}

			for i, initTask := range ready {
				fmt.Printf("[%d/%d] Running %s: %s\n", i+1, len(ready), initTask.ID, initTask.Title)

				t, err := backend.LoadTask(initTask.ID)
				if err != nil {
					fmt.Printf("  ✗ Failed to load: %v\n", err)
					continue
				}

				if !task.CanRunProto(t) && t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
					fmt.Printf("  ✗ Cannot run (status: %s)\n", t.Status)
					continue
				}

				cmdArgs := []string{"run", initTask.ID}
				if profile != "" {
					cmdArgs = append(cmdArgs, "--profile", profile)
				}

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
