// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/detect"
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

Specify the weight (trivial, small, medium, large, greenfield) via --weight flag.
If not specified, defaults to medium.

Specify the category (feature, bug, refactor, chore, docs, test) via --category flag.
If not specified, defaults to feature.

Use --template to create a task from a predefined template:
  orc new -t bugfix "Fix authentication timeout bug"
  orc new -t feature "Add dark mode" -v FEATURE_SCOPE="UI only"

Available templates: bugfix, feature, refactor, migration, spike
Use 'orc template list' to see all templates.

Use --attach to add screenshots or files during task creation:
  orc new "UI bug" --attach screenshot.png
  orc new "Fix layout" --attach before.png --attach after.png

Use --initiative to link the task to an initiative:
  orc new "Add auth flow" --initiative INIT-001

Use --blocked-by to specify task dependencies:
  orc new "Part 2 of feature" --blocked-by TASK-001
  orc new "Final step" --blocked-by TASK-001,TASK-002

Use --related-to to link related tasks:
  orc new "Related feature" --related-to TASK-003

Use --priority to set task priority:
  orc new "Urgent fix" --priority critical
  orc new "Important feature" -p high

Example:
  orc new "Fix authentication timeout bug"
  orc new "Implement user dashboard" --weight large
  orc new "Create new microservice" --weight greenfield
  orc new "Fix login bug" --category bug
  orc new -t bugfix "Fix memory leak"
  orc new "Button misaligned" --attach screenshot.png
  orc new "Implement login" --initiative INIT-001
  orc new "Add tests" --blocked-by TASK-005
  orc new "Critical bug" --priority critical`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer backend.Close()

			title := args[0]
			weight, _ := cmd.Flags().GetString("weight")
			category, _ := cmd.Flags().GetString("category")
			priority, _ := cmd.Flags().GetString("priority")
			description, _ := cmd.Flags().GetString("description")
			templateName, _ := cmd.Flags().GetString("template")
			varsFlag, _ := cmd.Flags().GetStringSlice("var")
			attachments, _ := cmd.Flags().GetStringSlice("attach")
			initiativeID, _ := cmd.Flags().GetString("initiative")
			blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")
			relatedTo, _ := cmd.Flags().GetStringSlice("related-to")

			// Parse variable flags
			vars := make(map[string]string)
			for _, v := range varsFlag {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Generate next task ID
			id, err := backend.GetNextTaskID()
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

			// Set weight (defaults to medium if not specified via --weight flag)
			if weight != "" {
				t.Weight = task.Weight(weight)
			} else {
				t.Weight = task.WeightMedium
			}

			// Set category (defaults to feature if not specified)
			if category != "" {
				cat := task.Category(category)
				if !task.IsValidCategory(cat) {
					return fmt.Errorf("invalid category: %s (valid: feature, bug, refactor, chore, docs, test)", category)
				}
				t.Category = cat
			}

			// Set priority (defaults to normal if not specified)
			if priority != "" {
				pri := task.Priority(priority)
				if !task.IsValidPriority(pri) {
					return fmt.Errorf("invalid priority: %s (valid: critical, high, normal, low)", priority)
				}
				t.Priority = pri
			}

			// Link to initiative if specified
			if initiativeID != "" {
				// Verify initiative exists
				exists, err := backend.InitiativeExists(initiativeID)
				if err != nil {
					return fmt.Errorf("check initiative: %w", err)
				}
				if !exists {
					return fmt.Errorf("initiative %s not found", initiativeID)
				}
				t.SetInitiative(initiativeID)
			}

			// Detect project characteristics for testing requirements
			// This is a fast operation (<10ms) so we run it on every task creation
			detection, _ := detect.Detect(".")
			hasFrontend := detection != nil && detection.HasFrontend

			// Set testing requirements based on project and task content
			t.SetTestingRequirements(hasFrontend)

			// Set dependencies if provided
			if len(blockedBy) > 0 || len(relatedTo) > 0 {
				// Load existing tasks for validation
				existingTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load existing tasks: %w", err)
				}
				existingIDs := make(map[string]bool)
				for _, existing := range existingTasks {
					existingIDs[existing.ID] = true
				}

				// Validate blocked_by references
				if errs := task.ValidateBlockedBy(id, blockedBy, existingIDs); len(errs) > 0 {
					return errs[0]
				}

				// Validate related_to references
				if errs := task.ValidateRelatedTo(id, relatedTo, existingIDs); len(errs) > 0 {
					return errs[0]
				}

				t.BlockedBy = blockedBy
				t.RelatedTo = relatedTo
			}

			// Save task
			if err := backend.SaveTask(t); err != nil {
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
			if err := backend.SavePlan(p, id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}

			// Update task status
			t.Status = task.StatusPlanned
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("update task: %w", err)
			}

			// Sync task to initiative if linked
			if t.HasInitiative() {
				init, err := backend.LoadInitiative(t.InitiativeID)
				if err != nil {
					// Log warning but don't fail task creation
					fmt.Printf("Warning: failed to load initiative %s for sync: %v\n", t.InitiativeID, err)
				} else {
					init.AddTask(t.ID, t.Title, nil)
					if err := backend.SaveInitiative(init); err != nil {
						fmt.Printf("Warning: failed to sync task to initiative: %v\n", err)
					}
				}
			}

			fmt.Printf("Task created: %s\n", id)
			fmt.Printf("   Title:    %s\n", title)
			fmt.Printf("   Weight:   %s\n", t.Weight)
			fmt.Printf("   Category: %s\n", t.GetCategory())
			fmt.Printf("   Priority: %s\n", t.GetPriority())
			fmt.Printf("   Phases:   %d\n", len(p.Phases))
			if tpl != nil {
				fmt.Printf("   Template: %s\n", tpl.Name)
			}
			if t.HasInitiative() {
				fmt.Printf("   Initiative: %s\n", t.InitiativeID)
			}
			if t.RequiresUITesting {
				fmt.Printf("   UI Testing: required (detected from task description)\n")
			}
			if t.TestingRequirements != nil {
				var reqs []string
				if t.TestingRequirements.Unit {
					reqs = append(reqs, "unit")
				}
				if t.TestingRequirements.E2E {
					reqs = append(reqs, "e2e")
				}
				if t.TestingRequirements.Visual {
					reqs = append(reqs, "visual")
				}
				if len(reqs) > 0 {
					fmt.Printf("   Testing: %s\n", strings.Join(reqs, ", "))
				}
			}
			if len(t.BlockedBy) > 0 {
				fmt.Printf("   Blocked by: %s\n", strings.Join(t.BlockedBy, ", "))
			}
			if len(t.RelatedTo) > 0 {
				fmt.Printf("   Related to: %s\n", strings.Join(t.RelatedTo, ", "))
			}

			// Upload attachments if provided
			if len(attachments) > 0 {
				var uploadedCount int
				for _, attachPath := range attachments {
					// Resolve relative paths
					if !filepath.IsAbs(attachPath) {
						cwd, err := os.Getwd()
						if err != nil {
							return fmt.Errorf("get working directory: %w", err)
						}
						attachPath = filepath.Join(cwd, attachPath)
					}

					// Read file
					data, err := os.ReadFile(attachPath)
					if err != nil {
						if os.IsNotExist(err) {
							return fmt.Errorf("attachment not found: %s", attachPath)
						}
						return fmt.Errorf("read attachment %s: %w", attachPath, err)
					}

					// Save attachment via backend
					filename := filepath.Base(attachPath)
					contentType := task.DetectContentType(filename)
					_, err = backend.SaveAttachment(id, filename, contentType, data)
					if err != nil {
						return fmt.Errorf("save attachment %s: %w", filename, err)
					}
					uploadedCount++
				}

				if uploadedCount > 0 {
					fmt.Printf("   Attachments: %d file(s) uploaded\n", uploadedCount)
				}
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("  orc run %s    - Execute the task\n", id)
			fmt.Printf("  orc show %s   - View task details\n", id)

			return nil
		},
	}
	cmd.Flags().StringP("weight", "w", "", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().StringP("category", "c", "", "task category (feature, bug, refactor, chore, docs, test)")
	cmd.Flags().StringP("priority", "p", "", "task priority (critical, high, normal, low)")
	cmd.Flags().StringP("description", "d", "", "task description")
	cmd.Flags().StringP("template", "t", "", "use template (bugfix, feature, refactor, migration, spike)")
	cmd.Flags().StringSlice("var", nil, "template variable (KEY=VALUE)")
	cmd.Flags().StringSliceP("attach", "a", nil, "file(s) to attach (screenshots, logs, etc.)")
	cmd.Flags().StringP("initiative", "i", "", "link task to initiative (e.g., INIT-001)")
	cmd.Flags().StringSlice("blocked-by", nil, "task IDs that must complete before this task")
	cmd.Flags().StringSlice("related-to", nil, "task IDs related to this task")
	return cmd
}
