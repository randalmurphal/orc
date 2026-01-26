// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func newAutomationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "automation",
		Aliases: []string{"auto"},
		Short:   "Manage automation triggers and tasks",
		Long: `Manage automation triggers that fire based on configurable conditions.

Automation enables automatic execution of maintenance and workflow tasks:
  • Count-based: Fire after N tasks/phases complete
  • Initiative-based: Fire on initiative events
  • Event-based: Fire on specific events (pr_merged, task_completed)
  • Threshold-based: Fire when metrics cross values
  • Schedule-based: Fire on cron expressions (team mode only)

Commands:
  list       List all triggers with status
  show       Show trigger details and history
  enable     Enable a trigger
  disable    Disable a trigger
  run        Manually run a trigger
  history    Show execution history
  reset      Reset trigger counter`,
	}

	cmd.AddCommand(newAutomationListCmd())
	cmd.AddCommand(newAutomationShowCmd())
	cmd.AddCommand(newAutomationEnableCmd())
	cmd.AddCommand(newAutomationDisableCmd())
	cmd.AddCommand(newAutomationRunCmd())
	cmd.AddCommand(newAutomationHistoryCmd())
	cmd.AddCommand(newAutomationResetCmd())
	cmd.AddCommand(newAutomationTasksCmd())

	return cmd
}

func newAutomationListCmd() *cobra.Command {
	var showDisabled bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all automation triggers",
		Long: `List all configured automation triggers with their status.

Example:
  orc automation list
  orc automation list --all  # Include disabled triggers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if !cfg.Automation.Enabled {
				fmt.Println("Automation is disabled in config (automation.enabled: false)")
				return nil
			}

			triggers := cfg.Automation.Triggers
			if len(triggers) == 0 {
				fmt.Println("No triggers configured.")
				fmt.Println("\nAdd triggers to .orc/config.yaml under automation.triggers:")
				fmt.Println("  automation:")
				fmt.Println("    triggers:")
				fmt.Println("      - id: style-normalization")
				fmt.Println("        type: count")
				fmt.Println("        condition:")
				fmt.Println("          metric: tasks_completed")
				fmt.Println("          threshold: 5")
				fmt.Println("        action:")
				fmt.Println("          template: style-normalization")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTYPE\tENABLED\tMODE\tDESCRIPTION")
			_, _ = fmt.Fprintln(w, "──\t────\t───────\t────\t───────────")

			count := 0
			for _, t := range triggers {
				if !showDisabled && !t.Enabled {
					continue
				}

				enabledStr := "yes"
				if !t.Enabled {
					enabledStr = "no"
				}

				mode := string(cfg.GetTriggerMode(t))
				desc := t.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					t.ID,
					string(t.Type),
					enabledStr,
					mode,
					desc,
				)
				count++
			}

			_ = w.Flush()

			if count == 0 && !showDisabled {
				fmt.Println("\nNo enabled triggers. Use --all to show disabled triggers.")
			}

			// Show summary
			enabled := 0
			for _, t := range triggers {
				if t.Enabled {
					enabled++
				}
			}
			fmt.Printf("\n%d trigger(s), %d enabled\n", len(triggers), enabled)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showDisabled, "all", "a", false, "show disabled triggers")

	return cmd
}

func newAutomationShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <trigger-id>",
		Short: "Show trigger details",
		Long: `Show detailed information about a trigger.

Example:
  orc automation show style-normalization`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			triggerID := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Find trigger in config
			var trigger *config.TriggerConfig
			for _, t := range cfg.Automation.Triggers {
				if t.ID == triggerID {
					trigger = &t
					break
				}
			}

			if trigger == nil {
				return fmt.Errorf("trigger %q not found", triggerID)
			}

			// Display trigger details
			fmt.Printf("Trigger: %s\n", trigger.ID)
			fmt.Printf("  Type:        %s\n", trigger.Type)
			fmt.Printf("  Enabled:     %v\n", trigger.Enabled)
			fmt.Printf("  Mode:        %s\n", cfg.GetTriggerMode(*trigger))
			if trigger.Description != "" {
				fmt.Printf("  Description: %s\n", trigger.Description)
			}

			// Condition details
			fmt.Println("\n  Condition:")
			switch trigger.Type {
			case config.TriggerTypeCount:
				fmt.Printf("    Metric:    %s\n", trigger.Condition.Metric)
				fmt.Printf("    Threshold: %d\n", trigger.Condition.Threshold)
				if len(trigger.Condition.Weights) > 0 {
					fmt.Printf("    Weights:   %s\n", strings.Join(trigger.Condition.Weights, ", "))
				}
				if len(trigger.Condition.Categories) > 0 {
					fmt.Printf("    Categories: %s\n", strings.Join(trigger.Condition.Categories, ", "))
				}
			case config.TriggerTypeInitiative, config.TriggerTypeEvent:
				fmt.Printf("    Event: %s\n", trigger.Condition.Event)
				if len(trigger.Condition.Filter) > 0 {
					fmt.Printf("    Filter: %v\n", trigger.Condition.Filter)
				}
			case config.TriggerTypeThreshold:
				fmt.Printf("    Metric:   %s\n", trigger.Condition.Metric)
				fmt.Printf("    Operator: %s\n", trigger.Condition.Operator)
				fmt.Printf("    Value:    %.2f\n", trigger.Condition.Value)
			case config.TriggerTypeSchedule:
				fmt.Printf("    Schedule: %s\n", trigger.Condition.Schedule)
				if !cfg.IsTeamMode() {
					fmt.Println("    Note: Schedule triggers require team mode (shared database)")
				}
			}

			// Action details
			fmt.Println("\n  Action:")
			fmt.Printf("    Template: %s\n", trigger.Action.Template)
			if trigger.Action.Priority != "" {
				fmt.Printf("    Priority: %s\n", trigger.Action.Priority)
			}
			if trigger.Action.Queue != "" {
				fmt.Printf("    Queue:    %s\n", trigger.Action.Queue)
			}

			// Cooldown
			if trigger.Cooldown.Tasks > 0 || trigger.Cooldown.Duration > 0 {
				fmt.Println("\n  Cooldown:")
				if trigger.Cooldown.Tasks > 0 {
					fmt.Printf("    Tasks: %d\n", trigger.Cooldown.Tasks)
				}
				if trigger.Cooldown.Duration > 0 {
					fmt.Printf("    Duration: %s\n", trigger.Cooldown.Duration)
				}
			}

			// Template info
			tmpl := cfg.GetAutomationTemplate(trigger.Action.Template)
			if tmpl != nil {
				fmt.Println("\n  Template:")
				fmt.Printf("    Title:    %s\n", tmpl.Title)
				fmt.Printf("    Weight:   %s\n", tmpl.Weight)
				fmt.Printf("    Category: %s\n", tmpl.Category)
				fmt.Printf("    Phases:   %s\n", strings.Join(tmpl.Phases, " → "))
			}

			// TODO: Show execution history from database when service is implemented
			fmt.Println("\n  History: (not yet implemented)")

			return nil
		},
	}

	return cmd
}

func newAutomationEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <trigger-id>",
		Short: "Enable a trigger",
		Long: `Enable an automation trigger.

Note: This modifies .orc/config.yaml

Example:
  orc automation enable style-normalization`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setTriggerEnabled(args[0], true)
		},
	}

	return cmd
}

func newAutomationDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <trigger-id>",
		Short: "Disable a trigger",
		Long: `Disable an automation trigger without removing it.

Note: This modifies .orc/config.yaml

Example:
  orc automation disable style-normalization`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setTriggerEnabled(args[0], false)
		},
	}

	return cmd
}

func setTriggerEnabled(triggerID string, enabled bool) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Find and update trigger
	found := false
	for i := range cfg.Automation.Triggers {
		if cfg.Automation.Triggers[i].ID == triggerID {
			if cfg.Automation.Triggers[i].Enabled == enabled {
				action := "enabled"
				if !enabled {
					action = "disabled"
				}
				fmt.Printf("Trigger %q is already %s\n", triggerID, action)
				return nil
			}
			cfg.Automation.Triggers[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("trigger %q not found", triggerID)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	if !quiet {
		fmt.Printf("Trigger %q %s\n", triggerID, action)
	}

	return nil
}

func newAutomationRunCmd() *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "run <trigger-id>",
		Short: "Manually run a trigger",
		Long: `Manually fire an automation trigger.

This creates an automation task from the trigger's template,
regardless of whether the trigger condition is met.

Example:
  orc automation run style-normalization
  orc automation run style-normalization --branch main
  orc automation run style-normalization --branch feature/foo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			triggerID := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if !cfg.Automation.Enabled {
				return fmt.Errorf("automation is disabled in config (automation.enabled: false)")
			}

			// Find trigger in config
			var trigger *config.TriggerConfig
			for _, t := range cfg.Automation.Triggers {
				if t.ID == triggerID {
					trigger = &t
					break
				}
			}

			if trigger == nil {
				return fmt.Errorf("trigger %q not found", triggerID)
			}

			if !trigger.Enabled && !quiet {
				fmt.Printf("Warning: Trigger %q is disabled, running anyway\n", triggerID)
			}

			// Get template info
			tmpl := cfg.GetAutomationTemplate(trigger.Action.Template)
			if tmpl == nil {
				return fmt.Errorf("template %q not found", trigger.Action.Template)
			}

			if !quiet {
				fmt.Printf("Running trigger: %s\n", triggerID)
				fmt.Printf("  Template: %s\n", trigger.Action.Template)
				fmt.Printf("  Title:    %s\n", tmpl.Title)
				if branch != "" {
					fmt.Printf("  Branch:   %s\n", branch)
				}
			}

			// Get backend and create automation service
			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			dbBackend, ok := backend.(*storage.DatabaseBackend)
			if !ok {
				return fmt.Errorf("database backend required for automation")
			}

			// Create automation service
			adapter := automation.NewProjectDBAdapter(dbBackend.DB())
			svc := automation.NewService(cfg, adapter, nil)

			// Create task creator with efficient DB adapter
			taskCreator := automation.NewAutoTaskCreator(cfg, backend, nil,
				automation.WithDBAdapter(adapter))
			svc.SetTaskCreator(taskCreator)

			// Run the trigger
			if err := svc.RunTrigger(cmd.Context(), triggerID); err != nil {
				return fmt.Errorf("run trigger: %w", err)
			}

			if !quiet {
				fmt.Println("\nTrigger executed successfully.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "target branch for automation task")

	return cmd
}

func newAutomationHistoryCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history [trigger-id]",
		Short: "Show trigger execution history",
		Long: `Show execution history for automation triggers.

Without arguments, shows recent executions across all triggers.
With a trigger ID, shows history for that specific trigger.

Example:
  orc automation history
  orc automation history --limit 20
  orc automation history style-normalization`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			var triggerID string
			if len(args) > 0 {
				triggerID = args[0]
			}

			// Get database connection
			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Load executions from database
			dbBackend, ok := backend.(*storage.DatabaseBackend)
			if !ok {
				return fmt.Errorf("database backend required")
			}
			pdb := dbBackend.DB()

			// Query executions
			query := `
				SELECT id, trigger_id, task_id, triggered_at, trigger_reason, status, completed_at, error_message
				FROM trigger_executions
			`
			queryArgs := []any{}
			if triggerID != "" {
				query += ` WHERE trigger_id = ?`
				queryArgs = append(queryArgs, triggerID)
			}
			query += ` ORDER BY triggered_at DESC LIMIT ?`
			queryArgs = append(queryArgs, limit)

			rows, err := pdb.Query(query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query executions: %w", err)
			}
			defer func() { _ = rows.Close() }()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TRIGGER\tSTATUS\tTASK\tTRIGGERED\tREASON")
			_, _ = fmt.Fprintln(w, "───────\t──────\t────\t─────────\t──────")

			count := 0
			for rows.Next() {
				var id int64
				var trigger, taskID, triggeredAt, reason, status string
				var completedAt, errMsg *string

				if err := rows.Scan(&id, &trigger, &taskID, &triggeredAt, &reason, &status, &completedAt, &errMsg); err != nil {
					continue
				}

				// Format timestamp
				if t, parseErr := time.Parse(time.RFC3339, triggeredAt); parseErr == nil {
					triggeredAt = formatTimeAgo(t)
				}

				// Truncate reason
				if len(reason) > 40 {
					reason = reason[:37] + "..."
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					trigger,
					status,
					taskID,
					triggeredAt,
					reason,
				)
				count++
			}

			_ = w.Flush()

			if count == 0 {
				if triggerID != "" {
					fmt.Printf("\nNo executions found for trigger %q\n", triggerID)
				} else {
					fmt.Println("\nNo executions found.")
				}
			} else {
				fmt.Printf("\n%d execution(s)\n", count)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "maximum number of executions to show")

	return cmd
}

func newAutomationResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset <trigger-id>",
		Short: "Reset trigger counter",
		Long: `Reset the cooldown counter for a trigger.

This allows the trigger to fire again immediately, even if
the cooldown period has not elapsed.

Example:
  orc automation reset style-normalization`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			triggerID := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Verify trigger exists
			found := false
			for _, t := range cfg.Automation.Triggers {
				if t.ID == triggerID {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("trigger %q not found", triggerID)
			}

			// Get database connection
			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			dbBackend, ok := backend.(*storage.DatabaseBackend)
			if !ok {
				return fmt.Errorf("database backend required")
			}
			pdb := dbBackend.DB()

			// Reset counter in database
			_, err = pdb.Exec(`
				UPDATE trigger_counters
				SET count = 0, last_reset_at = datetime('now')
				WHERE trigger_id = ?
			`, triggerID)
			if err != nil {
				return fmt.Errorf("reset counter: %w", err)
			}

			if !quiet {
				fmt.Printf("Reset counter for trigger %q\n", triggerID)
			}

			return nil
		},
	}

	return cmd
}

func newAutomationTasksCmd() *cobra.Command {
	var showPending bool

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List automation tasks",
		Long: `List all AUTO-* automation tasks.

Example:
  orc automation tasks
  orc automation tasks --pending  # Show tasks awaiting approval`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			tasks, err := backend.LoadAllTasks()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATUS\tTITLE\tCREATED")
			_, _ = fmt.Fprintln(w, "──\t──────\t─────\t───────")

			count := 0
			for _, t := range tasks {
				// Filter for AUTO-* tasks
				if len(t.Id) < 5 || t.Id[:5] != "AUTO-" {
					continue
				}

				// Filter by pending if requested
				if showPending && (t.Status != orcv1.TaskStatus_TASK_STATUS_CREATED && t.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED) {
					continue
				}

				title := t.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}

				var createdAt string
				if t.CreatedAt != nil {
					createdAt = formatTimeAgo(t.CreatedAt.AsTime())
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					t.Id,
					task.StatusFromProto(t.Status),
					title,
					createdAt,
				)
				count++
			}

			_ = w.Flush()

			if count == 0 {
				if showPending {
					fmt.Println("\nNo pending automation tasks.")
				} else {
					fmt.Println("\nNo automation tasks.")
				}
			} else {
				fmt.Printf("\n%d task(s)\n", count)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showPending, "pending", false, "show only pending tasks")

	return cmd
}
