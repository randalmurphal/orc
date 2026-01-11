// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/template"
)

// newNewCmd creates the new task command
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new task",
		Long: `Create a new task to be orchestrated by orc.

The task will be classified by weight (trivial, small, medium, large, greenfield)
either automatically by AI or manually via --weight flag.

Use --template to create a task from a predefined template:
  orc new -t bugfix "Fix authentication timeout bug"
  orc new -t feature "Add dark mode" -v FEATURE_SCOPE="UI only"

Available templates: bugfix, feature, refactor, migration, spike
Use 'orc template list' to see all templates.

Example:
  orc new "Fix authentication timeout bug"
  orc new "Implement user dashboard" --weight large
  orc new "Create new microservice" --weight greenfield
  orc new -t bugfix "Fix memory leak"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			title := args[0]
			weight, _ := cmd.Flags().GetString("weight")
			description, _ := cmd.Flags().GetString("description")
			templateName, _ := cmd.Flags().GetString("template")
			varsFlag, _ := cmd.Flags().GetStringSlice("var")

			// Parse variable flags
			vars := make(map[string]string)
			for _, v := range varsFlag {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Generate next task ID
			id, err := task.NextID()
			if err != nil {
				return fmt.Errorf("generate task ID: %w", err)
			}

			// Create task
			t := task.New(id, title)
			if description != "" {
				t.Description = description
			}

			// If using template, get weight and phases from template
			var tpl *template.Template
			if templateName != "" {
				tpl, err = template.Load(templateName)
				if err != nil {
					return fmt.Errorf("template %q not found", templateName)
				}

				// Validate required variables
				if err := tpl.ValidateVariables(vars); err != nil {
					return err
				}

				// Use template weight unless overridden
				if weight == "" {
					weight = tpl.Weight
				}

				// Render title and description with variables
				vars["TASK_TITLE"] = title
				t.Description = template.Render(t.Description, vars)

				if !quiet {
					fmt.Printf("Using template: %s\n", tpl.Name)
				}
			}

			// Set weight
			if weight != "" {
				t.Weight = task.Weight(weight)
			} else {
				// Default to medium if not specified
				// TODO: Add AI classification
				t.Weight = task.WeightMedium
			}

			// Save task
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Create plan
			var p *plan.Plan
			if tpl != nil {
				// Create plan from task template
				p = &plan.Plan{
					Version:     1,
					TaskID:      id,
					Weight:      t.Weight,
					Description: fmt.Sprintf("From template: %s", tpl.Name),
					Phases:      make([]plan.Phase, 0, len(tpl.Phases)),
				}
				for _, phaseID := range tpl.Phases {
					p.Phases = append(p.Phases, plan.Phase{
						ID:     phaseID,
						Name:   phaseID,
						Gate:   plan.Gate{Type: plan.GateAuto},
						Status: plan.PhasePending,
					})
				}
			} else {
				// Create plan from weight template
				p, err = plan.CreateFromTemplate(t)
				if err != nil {
					// If template not found, use default plan
					fmt.Printf("Warning: No template for weight %s, using default plan\n", t.Weight)
					p = &plan.Plan{
						Version:     1,
						TaskID:      id,
						Weight:      t.Weight,
						Description: "Default plan",
						Phases: []plan.Phase{
							{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
						},
					}
				}
			}

			// Save plan
			if err := p.Save(id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}

			// Update task status
			t.Status = task.StatusPlanned
			if err := t.Save(); err != nil {
				return fmt.Errorf("update task: %w", err)
			}

			fmt.Printf("Task created: %s\n", id)
			fmt.Printf("   Title:  %s\n", title)
			fmt.Printf("   Weight: %s\n", t.Weight)
			fmt.Printf("   Phases: %d\n", len(p.Phases))
			if tpl != nil {
				fmt.Printf("   Template: %s\n", tpl.Name)
			}
			fmt.Println("\nNext steps:")
			fmt.Printf("  orc run %s    - Execute the task\n", id)
			fmt.Printf("  orc show %s   - View task details\n", id)

			return nil
		},
	}
	cmd.Flags().StringP("weight", "w", "", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().StringP("description", "d", "", "task description")
	cmd.Flags().StringP("template", "t", "", "use template (bugfix, feature, refactor, migration, spike)")
	cmd.Flags().StringSlice("var", nil, "template variable (KEY=VALUE)")
	return cmd
}
