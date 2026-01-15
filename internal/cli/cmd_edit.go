// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
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
  --priority    Change task priority (critical, high, normal, low)
  --status      Change task status (for administrative corrections)
  --initiative  Link/unlink task to initiative (use "" to unlink)

Dependency management:
  --blocked-by      Set tasks that must complete first (replaces existing)
  --add-blocker     Add task(s) to blocked_by list
  --remove-blocker  Remove task(s) from blocked_by list
  --related-to      Set related tasks (replaces existing)
  --add-related     Add task(s) to related_to list
  --remove-related  Remove task(s) from related_to list

Weight changes will regenerate the task plan with phases appropriate
for the new weight. This requires the task to not be running.

Valid status values: created, classifying, planned, paused, blocked,
  completed, finished, failed. Note: running tasks must be paused first.

Example:
  orc edit TASK-001 --title "New title"
  orc edit TASK-001 --weight large
  orc edit TASK-001 --priority critical
  orc edit TASK-001 --status completed      # mark task as done
  orc edit TASK-001 -d "Updated description" --title "Better title"
  orc edit TASK-001 --initiative INIT-001   # link to initiative
  orc edit TASK-001 --initiative ""         # unlink from initiative
  orc edit TASK-001 --blocked-by TASK-002,TASK-003
  orc edit TASK-001 --add-blocker TASK-004
  orc edit TASK-001 --remove-blocker TASK-002`,
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

			taskID := args[0]
			newTitle, _ := cmd.Flags().GetString("title")
			newDescription, _ := cmd.Flags().GetString("description")
			newWeight, _ := cmd.Flags().GetString("weight")
			newPriority, _ := cmd.Flags().GetString("priority")
			newStatus, _ := cmd.Flags().GetString("status")
			newInitiative, _ := cmd.Flags().GetString("initiative")
			initiativeChanged := cmd.Flags().Changed("initiative")

			// Dependency flags
			blockedBy, _ := cmd.Flags().GetStringSlice("blocked-by")
			addBlockers, _ := cmd.Flags().GetStringSlice("add-blocker")
			removeBlockers, _ := cmd.Flags().GetStringSlice("remove-blocker")
			relatedTo, _ := cmd.Flags().GetStringSlice("related-to")
			addRelated, _ := cmd.Flags().GetStringSlice("add-related")
			removeRelated, _ := cmd.Flags().GetStringSlice("remove-related")

			// Load task to verify it exists
			t, err := backend.LoadTask(taskID)
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

			// Update priority if provided
			oldPriority := t.GetPriority()
			if newPriority != "" {
				p := task.Priority(newPriority)
				if !task.IsValidPriority(p) {
					return fmt.Errorf("invalid priority %q - valid options: critical, high, normal, low", newPriority)
				}
				if t.Priority != p {
					t.Priority = p
					changes = append(changes, "priority")
				}
			}

			// Update status if provided
			oldStatus := t.Status
			if newStatus != "" {
				s := task.Status(newStatus)
				if !task.IsValidStatus(s) {
					validOpts := make([]string, 0, len(task.ValidStatuses()))
					for _, vs := range task.ValidStatuses() {
						validOpts = append(validOpts, string(vs))
					}
					return fmt.Errorf("invalid status %q - valid options: %s", newStatus, strings.Join(validOpts, ", "))
				}
				if t.Status != s {
					t.Status = s
					changes = append(changes, "status")
				}
			}

			// Update initiative if flag was provided (even if empty, to allow unlinking)
			oldInitiative := t.InitiativeID
			if initiativeChanged {
				if newInitiative != "" {
					// Verify initiative exists
					exists, err := backend.InitiativeExists(newInitiative)
					if err != nil {
						return fmt.Errorf("check initiative: %w", err)
					}
					if !exists {
						return fmt.Errorf("initiative %s not found", newInitiative)
					}
				}
				if t.InitiativeID != newInitiative {
					t.SetInitiative(newInitiative)
					changes = append(changes, "initiative")
				}
			}

			// Handle dependency updates
			hasDepChanges := len(blockedBy) > 0 || len(addBlockers) > 0 || len(removeBlockers) > 0 ||
				len(relatedTo) > 0 || len(addRelated) > 0 || len(removeRelated) > 0

			if hasDepChanges {
				// Load all tasks for validation
				allTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks for validation: %w", err)
				}

				existingIDs := make(map[string]bool)
				taskMap := make(map[string]*task.Task)
				for _, existing := range allTasks {
					existingIDs[existing.ID] = true
					taskMap[existing.ID] = existing
				}

				// Handle blocked_by changes
				if len(blockedBy) > 0 {
					// Replace entire list
					if errs := task.ValidateBlockedBy(taskID, blockedBy, existingIDs); len(errs) > 0 {
						return errs[0]
					}
					// Check for circular dependencies with all new blockers at once
					if cycle := task.DetectCircularDependencyWithAll(taskID, blockedBy, taskMap); cycle != nil {
						return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
					}
					t.BlockedBy = blockedBy
					changes = append(changes, "blocked_by")
				} else {
					// Handle add/remove
					if len(addBlockers) > 0 {
						if errs := task.ValidateBlockedBy(taskID, addBlockers, existingIDs); len(errs) > 0 {
							return errs[0]
						}
						for _, newBlocker := range addBlockers {
							if cycle := task.DetectCircularDependency(taskID, newBlocker, taskMap); cycle != nil {
								return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
							}
							// Add if not already present
							found := false
							for _, existing := range t.BlockedBy {
								if existing == newBlocker {
									found = true
									break
								}
							}
							if !found {
								t.BlockedBy = append(t.BlockedBy, newBlocker)
							}
						}
						changes = append(changes, "blocked_by")
					}
					if len(removeBlockers) > 0 {
						newList := make([]string, 0, len(t.BlockedBy))
						for _, existing := range t.BlockedBy {
							keep := true
							for _, toRemove := range removeBlockers {
								if existing == toRemove {
									keep = false
									break
								}
							}
							if keep {
								newList = append(newList, existing)
							}
						}
						t.BlockedBy = newList
						changes = append(changes, "blocked_by")
					}
				}

				// Handle related_to changes
				if len(relatedTo) > 0 {
					// Replace entire list
					if errs := task.ValidateRelatedTo(taskID, relatedTo, existingIDs); len(errs) > 0 {
						return errs[0]
					}
					t.RelatedTo = relatedTo
					changes = append(changes, "related_to")
				} else {
					// Handle add/remove
					if len(addRelated) > 0 {
						if errs := task.ValidateRelatedTo(taskID, addRelated, existingIDs); len(errs) > 0 {
							return errs[0]
						}
						for _, newRelated := range addRelated {
							// Add if not already present
							found := false
							for _, existing := range t.RelatedTo {
								if existing == newRelated {
									found = true
									break
								}
							}
							if !found {
								t.RelatedTo = append(t.RelatedTo, newRelated)
							}
						}
						changes = append(changes, "related_to")
					}
					if len(removeRelated) > 0 {
						newList := make([]string, 0, len(t.RelatedTo))
						for _, existing := range t.RelatedTo {
							keep := true
							for _, toRemove := range removeRelated {
								if existing == toRemove {
									keep = false
									break
								}
							}
							if keep {
								newList = append(newList, existing)
							}
						}
						t.RelatedTo = newList
						changes = append(changes, "related_to")
					}
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
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Handle weight change - regenerate plan and reset state
			if weightChanged {
				if err := regeneratePlanForWeight(backend, t, oldWeight); err != nil {
					return fmt.Errorf("regenerate plan: %w", err)
				}
			}

			// Handle initiative change - sync bidirectionally
			if initiativeChanged && oldInitiative != t.InitiativeID {
				// Remove from old initiative if it was linked
				if oldInitiative != "" {
					if oldInit, err := backend.LoadInitiative(oldInitiative); err == nil {
						oldInit.RemoveTask(t.ID)
						if err := backend.SaveInitiative(oldInit); err != nil {
							fmt.Printf("Warning: failed to remove task from old initiative: %v\n", err)
						}
					}
				}
				// Add to new initiative if linking
				if t.HasInitiative() {
					if newInit, err := backend.LoadInitiative(t.InitiativeID); err == nil {
						newInit.AddTask(t.ID, t.Title, nil)
						if err := backend.SaveInitiative(newInit); err != nil {
							fmt.Printf("Warning: failed to add task to new initiative: %v\n", err)
						}
					}
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
					case "priority":
						fmt.Printf("   Priority: %s -> %s\n", oldPriority, t.Priority)
					case "status":
						fmt.Printf("   Status: %s -> %s\n", oldStatus, t.Status)
					case "initiative":
						if t.HasInitiative() {
							if oldInitiative == "" {
								fmt.Printf("   Initiative: linked to %s\n", t.InitiativeID)
							} else {
								fmt.Printf("   Initiative: %s -> %s\n", oldInitiative, t.InitiativeID)
							}
						} else {
							fmt.Printf("   Initiative: unlinked from %s\n", oldInitiative)
						}
					case "blocked_by":
						if len(t.BlockedBy) > 0 {
							fmt.Printf("   Blocked by: %s\n", strings.Join(t.BlockedBy, ", "))
						} else {
							fmt.Printf("   Blocked by: (none)\n")
						}
					case "related_to":
						if len(t.RelatedTo) > 0 {
							fmt.Printf("   Related to: %s\n", strings.Join(t.RelatedTo, ", "))
						} else {
							fmt.Printf("   Related to: (none)\n")
						}
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "new task title")
	cmd.Flags().StringP("description", "d", "", "new task description")
	cmd.Flags().StringP("weight", "w", "", "new task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().StringP("priority", "p", "", "new task priority (critical, high, normal, low)")
	cmd.Flags().StringP("status", "s", "", "new task status (created, classifying, planned, paused, blocked, completed, finished, failed)")
	cmd.Flags().StringP("initiative", "i", "", "link/unlink task to initiative (use \"\" to unlink)")

	// Dependency flags
	cmd.Flags().StringSlice("blocked-by", nil, "set blocked_by list (replaces existing)")
	cmd.Flags().StringSlice("add-blocker", nil, "add task(s) to blocked_by list")
	cmd.Flags().StringSlice("remove-blocker", nil, "remove task(s) from blocked_by list")
	cmd.Flags().StringSlice("related-to", nil, "set related_to list (replaces existing)")
	cmd.Flags().StringSlice("add-related", nil, "add task(s) to related_to list")
	cmd.Flags().StringSlice("remove-related", nil, "remove task(s) from related_to list")

	return cmd
}

// regeneratePlanForWeight creates a new plan based on the task's current weight,
// preserving completed/skipped phase statuses, and resets the state appropriately.
func regeneratePlanForWeight(backend storage.Backend, t *task.Task, oldWeight task.Weight) error {
	// Load current plan if it exists
	oldPlan, _ := backend.LoadPlan(t.ID)

	// Use the shared plan regeneration function
	result, err := plan.RegeneratePlan(t, oldPlan)
	if err != nil {
		return err
	}

	// Save the new plan
	if err := backend.SavePlan(result.NewPlan, t.ID); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}

	// Reset state - but preserve phase states for preserved phases
	s, err := backend.LoadState(t.ID)
	if err != nil {
		// State doesn't exist, create new one
		s = state.New(t.ID)
	} else {
		// Build set of preserved phases
		preservedSet := make(map[string]bool)
		for _, phaseID := range result.PreservedPhases {
			preservedSet[phaseID] = true
		}

		// Reset state but keep completed phase states for preserved phases
		s.Status = state.StatusPending
		s.CurrentPhase = ""
		s.CurrentIteration = 0
		s.Error = ""
		s.RetryContext = nil

		// Filter phase states: keep only preserved phases with completed/skipped status
		newPhases := make(map[string]*state.PhaseState)
		for phaseID, phaseState := range s.Phases {
			if preservedSet[phaseID] && (phaseState.Status == state.StatusCompleted || phaseState.Status == state.StatusSkipped) {
				newPhases[phaseID] = phaseState
			}
		}
		s.Phases = newPhases
	}

	if err := backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Update task status to planned
	t.Status = task.StatusPlanned
	if err := backend.SaveTask(t); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	return nil
}
