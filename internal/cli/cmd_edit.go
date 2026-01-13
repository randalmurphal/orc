// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newEditCmd creates the edit command for modifying task properties.
func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <task-id>",
		Short: "Edit task properties",
		Long: `Edit task properties after creation.

Modifiable properties:
  --title       Update the task title
  --description Update the task description (or -d)
  --weight      Change task weight (triggers plan regeneration)

Weight changes will regenerate the task plan with phases appropriate
for the new weight. This requires the task to not be running.

Example:
  orc edit TASK-001 --title "New title"
  orc edit TASK-001 --weight large
  orc edit TASK-001 -d "Updated description" --title "Better title"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			taskID := args[0]
			newTitle, _ := cmd.Flags().GetString("title")
			newDescription, _ := cmd.Flags().GetString("description")
			newWeight, _ := cmd.Flags().GetString("weight")

			// Load task to verify it exists
			t, err := task.Load(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if task is running (cannot edit running tasks)
			if t.Status == task.StatusRunning {
				return fmt.Errorf("cannot edit running task %s - pause it first", taskID)
			}

			// Track what changed
			var changes []string
			weightChanged := false
			oldWeight := t.Weight

			// Update title if provided
			if newTitle != "" {
				if t.Title != newTitle {
					t.Title = newTitle
					changes = append(changes, "title")
				}
			}

			// Update description if provided
			if newDescription != "" {
				if t.Description != newDescription {
					t.Description = newDescription
					changes = append(changes, "description")
				}
			}

			// Update weight if provided
			if newWeight != "" {
				w := task.Weight(newWeight)
				if !task.IsValidWeight(w) {
					return fmt.Errorf("invalid weight %q - valid options: trivial, small, medium, large, greenfield", newWeight)
				}
				if t.Weight != w {
					t.Weight = w
					changes = append(changes, "weight")
					weightChanged = true
				}
			}

			// No changes requested
			if len(changes) == 0 {
				if !quiet {
					fmt.Printf("No changes to apply to task %s\n", taskID)
				}
				return nil
			}

			// Save task
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Handle weight change - regenerate plan and reset state
			if weightChanged {
				if err := regeneratePlanForWeight(t, oldWeight); err != nil {
					return fmt.Errorf("regenerate plan: %w", err)
				}
			}

			if !quiet {
				fmt.Printf("Updated task %s\n", taskID)
				for _, change := range changes {
					switch change {
					case "title":
						fmt.Printf("   Title: %s\n", t.Title)
					case "description":
						desc := t.Description
						if len(desc) > 60 {
							desc = desc[:57] + "..."
						}
						fmt.Printf("   Description: %s\n", desc)
					case "weight":
						fmt.Printf("   Weight: %s -> %s (plan regenerated)\n", oldWeight, t.Weight)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "new task title")
	cmd.Flags().StringP("description", "d", "", "new task description")
	cmd.Flags().StringP("weight", "w", "", "new task weight (trivial, small, medium, large, greenfield)")

	return cmd
}

// regeneratePlanForWeight creates a new plan based on the task's current weight
// and resets the state to pending.
func regeneratePlanForWeight(t *task.Task, oldWeight task.Weight) error {
	// Create new plan from weight template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
		// If template not found, create default plan
		p = &plan.Plan{
			Version:     1,
			TaskID:      t.ID,
			Weight:      t.Weight,
			Description: "Default plan",
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
			},
		}
	}

	// Save new plan
	if err := p.Save(t.ID); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}

	// Reset state - clear all phase progress
	s, err := state.Load(t.ID)
	if err != nil {
		// State doesn't exist, create new one
		s = state.New(t.ID)
	} else {
		// Reset existing state
		s.Status = state.StatusPending
		s.CurrentPhase = ""
		s.CurrentIteration = 0
		s.Phases = make(map[string]*state.PhaseState)
		s.Error = ""
		s.RetryContext = nil
	}

	if err := s.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Update task status to planned
	t.Status = task.StatusPlanned
	if err := t.Save(); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	return nil
}
