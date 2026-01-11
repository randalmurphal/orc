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
		Aliases: []string{"init", "i"},
		Short:   "Manage initiatives (grouped tasks with shared context)",
		Long: `Manage initiatives - groupings of related tasks with shared vision and decisions.

Initiatives provide:
  • Shared context across related tasks
  • Decision tracking with rationale
  • Task dependency management
  • P2P/team collaboration via shared directories

Commands:
  new        Create a new initiative
  list       List all initiatives
  show       Show initiative details
  add-task   Link a task to an initiative
  decide     Record a decision
  activate   Set initiative status to active
  complete   Mark initiative as completed
  run        Run all initiative tasks in order`,
	}

	cmd.AddCommand(newInitiativeNewCmd())
	cmd.AddCommand(newInitiativeListCmd())
	cmd.AddCommand(newInitiativeShowCmd())
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
  orc initiative new "Dark Mode" --shared  # Creates in shared directory for teams`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			title := args[0]
			vision, _ := cmd.Flags().GetString("vision")
			shared, _ := cmd.Flags().GetBool("shared")
			ownerInitials, _ := cmd.Flags().GetString("owner")

			// Generate next initiative ID
			id, err := initiative.NextID(shared)
			if err != nil {
				return fmt.Errorf("generate initiative ID: %w", err)
			}

			// Create initiative
			init := initiative.New(id, title)
			if vision != "" {
				init.Vision = vision
			}
			if ownerInitials != "" {
				init.Owner = initiative.Identity{Initials: ownerInitials}
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

			if !quiet {
				fmt.Printf("Initiative created: %s\n", id)
				fmt.Printf("   Title:  %s\n", title)
				fmt.Printf("   Status: %s\n", init.Status)
				if vision != "" {
					fmt.Printf("   Vision: %s\n", vision)
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

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tTASKS\tOWNER")
			fmt.Fprintln(w, "--\t-----\t------\t-----\t-----")

			for _, init := range initiatives {
				owner := "-"
				if init.Owner.Initials != "" {
					owner = init.Owner.Initials
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					init.ID, truncate(init.Title, 30), init.Status, len(init.Tasks), owner)
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().StringP("status", "s", "", "filter by status (draft, active, completed, archived)")
	cmd.Flags().Bool("shared", false, "list shared initiatives")

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

			fmt.Printf("Initiative: %s\n", init.ID)
			fmt.Printf("Title:      %s\n", init.Title)
			fmt.Printf("Status:     %s\n", init.Status)
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
Use --dry-run to see what would be executed without running.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			shared, _ := cmd.Flags().GetBool("shared")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

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

			ready := init.GetReadyTasks()
			if len(ready) == 0 {
				fmt.Println("No tasks ready to run.")
				fmt.Println("All tasks either completed or waiting on dependencies.")
				return nil
			}

			if dryRun {
				fmt.Println("Tasks ready to run (dry-run mode):")
				for _, t := range ready {
					fmt.Printf("  %s: %s\n", t.ID, t.Title)
				}
				return nil
			}

			fmt.Printf("Running %d ready task(s) from %s:\n", len(ready), id)
			for _, t := range ready {
				fmt.Printf("  → %s: %s\n", t.ID, t.Title)
			}
			fmt.Println("\nUse 'orc run TASK-ID' to execute individual tasks.")

			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "use shared initiative")
	cmd.Flags().Bool("dry-run", false, "show what would be run without executing")

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

			fmt.Printf("Deleted initiative %s\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("shared", false, "delete from shared directory")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation")

	return cmd
}

