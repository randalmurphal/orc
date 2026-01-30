// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
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
  --title         Update the task title
  --description   Update the task description (or -d)
  --weight        Change task weight (triggers plan regeneration)
  --workflow      Change task workflow (e.g., qa-e2e, implement)
  --priority      Change task priority (critical, high, normal, low)
  --status        Change task status (for administrative corrections)
  --initiative    Link/unlink task to initiative (use "" to unlink)

Branch & PR control:
  --branch-name   Custom branch name (overrides auto-generated from task ID)
  --target-branch Override PR target branch for this task
  --pr-draft      PR draft mode (true/false/unset to clear)
  --pr-labels     PR labels (comma-separated, replaces existing)
  --pr-reviewers  PR reviewers (comma-separated, replaces existing)

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
  completed, failed. Note: running tasks must be paused first.

Example:
  orc edit TASK-001 --title "New title"
  orc edit TASK-001 --weight large
  orc edit TASK-001 --workflow qa-e2e       # use QA E2E workflow
  orc edit TASK-001 --priority critical
  orc edit TASK-001 --status completed      # mark task as done
  orc edit TASK-001 -d "Updated description" --title "Better title"
  orc edit TASK-001 --initiative INIT-001   # link to initiative
  orc edit TASK-001 --initiative ""         # unlink from initiative
  orc edit TASK-001 --blocked-by TASK-002,TASK-003
  orc edit TASK-001 --add-blocker TASK-004
  orc edit TASK-001 --remove-blocker TASK-002
  orc edit TASK-001 --target-branch hotfix/v2.1
  orc edit TASK-001 --branch-name feature/custom-name
  orc edit TASK-001 --pr-draft true         # create as draft PR
  orc edit TASK-001 --pr-labels bug,urgent  # set PR labels
  orc edit TASK-001 --pr-reviewers alice,bob`,
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

			taskID := args[0]
			newTitle, _ := cmd.Flags().GetString("title")
			newDescription, _ := cmd.Flags().GetString("description")
			newWeight, _ := cmd.Flags().GetString("weight")
			newWorkflow, _ := cmd.Flags().GetString("workflow")
			workflowChanged := cmd.Flags().Changed("workflow")
			newPriority, _ := cmd.Flags().GetString("priority")
			newStatus, _ := cmd.Flags().GetString("status")
			newInitiative, _ := cmd.Flags().GetString("initiative")
			initiativeChanged := cmd.Flags().Changed("initiative")
			newTargetBranch, _ := cmd.Flags().GetString("target-branch")
			targetBranchChanged := cmd.Flags().Changed("target-branch")

			// Branch control flags
			newBranchName, _ := cmd.Flags().GetString("branch-name")
			branchNameChanged := cmd.Flags().Changed("branch-name")
			newPrDraft, _ := cmd.Flags().GetString("pr-draft")
			prDraftChanged := cmd.Flags().Changed("pr-draft")
			newPrLabels, _ := cmd.Flags().GetStringSlice("pr-labels")
			prLabelsChanged := cmd.Flags().Changed("pr-labels")
			newPrReviewers, _ := cmd.Flags().GetStringSlice("pr-reviewers")
			prReviewersChanged := cmd.Flags().Changed("pr-reviewers")

			// Validate target branch if specified (empty string clears it)
			if targetBranchChanged && newTargetBranch != "" {
				if err := git.ValidateBranchName(newTargetBranch); err != nil {
					return fmt.Errorf("invalid target branch: %w", err)
				}
			}

			// Validate branch name if specified (empty string clears it)
			if branchNameChanged && newBranchName != "" {
				if err := git.ValidateBranchName(newBranchName); err != nil {
					return fmt.Errorf("invalid branch name: %w", err)
				}
			}

			// Validate pr-draft value
			if prDraftChanged && newPrDraft != "" && newPrDraft != "true" && newPrDraft != "false" {
				return fmt.Errorf("invalid pr-draft value %q - valid options: true, false, \"\" (to clear)", newPrDraft)
			}

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
			if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
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
				currentDesc := ""
				if t.Description != nil {
					currentDesc = *t.Description
				}
				if currentDesc != newDescription {
					t.Description = &newDescription
					changes = append(changes, "description")
				}
			}

			// Update weight if provided
			if newWeight != "" {
				w, valid := task.ParseWeightProto(newWeight)
				if !valid {
					return fmt.Errorf("invalid weight %q - valid options: trivial, small, medium, large", newWeight)
				}
				if t.Weight != w {
					t.Weight = w
					changes = append(changes, "weight")
					weightChanged = true
				}
			}

			// Update workflow if flag was provided (even if empty, to allow clearing)
			oldWorkflow := ""
			if t.WorkflowId != nil {
				oldWorkflow = *t.WorkflowId
			}
			if workflowChanged {
				if newWorkflow != "" {
					// Verify workflow exists
					dbBackend, ok := backend.(*storage.DatabaseBackend)
					if !ok {
						return fmt.Errorf("workflow validation requires database backend")
					}
					wf, err := dbBackend.DB().GetWorkflow(newWorkflow)
					if err != nil {
						return fmt.Errorf("check workflow: %w", err)
					}
					if wf == nil {
						return fmt.Errorf("workflow not found: %s\n\nRun 'orc workflows' to see available workflows", newWorkflow)
					}
				}
				currentWorkflow := ""
				if t.WorkflowId != nil {
					currentWorkflow = *t.WorkflowId
				}
				if currentWorkflow != newWorkflow {
					if newWorkflow == "" {
						t.WorkflowId = nil
					} else {
						t.WorkflowId = &newWorkflow
					}
					changes = append(changes, "workflow")
				}
			}

			// Update priority if provided
			oldPriority := task.GetPriorityProto(t)
			if newPriority != "" {
				p, valid := task.ParsePriorityProto(newPriority)
				if !valid {
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
				s, ok := task.ParseStatusProto(newStatus)
				if !ok {
					return fmt.Errorf("invalid status %q - valid options: created, classifying, planned, running, paused, blocked, finalizing, completed, failed, resolved", newStatus)
				}
				if t.Status != s {
					t.Status = s
					changes = append(changes, "status")
				}
			}

			// Update initiative if flag was provided (even if empty, to allow unlinking)
			oldInitiative := ""
			if t.InitiativeId != nil {
				oldInitiative = *t.InitiativeId
			}
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
				currentInit := ""
				if t.InitiativeId != nil {
					currentInit = *t.InitiativeId
				}
				if currentInit != newInitiative {
					task.SetInitiativeProto(t, newInitiative)
					changes = append(changes, "initiative")
				}
			}

			// Update target branch if flag was provided (even if empty, to allow clearing)
			oldTargetBranch := task.GetTargetBranchProto(t)
			if targetBranchChanged {
				currentBranch := task.GetTargetBranchProto(t)
				if currentBranch != newTargetBranch {
					task.SetTargetBranchProto(t, newTargetBranch)
					changes = append(changes, "target_branch")
				}
			}

			// Update branch name if flag was provided (even if empty, to allow clearing)
			oldBranchName := ""
			if t.BranchName != nil {
				oldBranchName = *t.BranchName
			}
			if branchNameChanged {
				currentName := ""
				if t.BranchName != nil {
					currentName = *t.BranchName
				}
				if currentName != newBranchName {
					if newBranchName == "" {
						t.BranchName = nil
					} else {
						t.BranchName = &newBranchName
					}
					changes = append(changes, "branch_name")
				}
			}

			// Update PR draft mode if flag was provided
			oldPrDraft := t.PrDraft
			if prDraftChanged {
				var newDraftVal *bool
				switch newPrDraft {
				case "true":
					v := true
					newDraftVal = &v
				case "false":
					v := false
					newDraftVal = &v
				}
				// Compare: both nil, or both non-nil with same value
				changed := (t.PrDraft == nil) != (newDraftVal == nil)
				if !changed && t.PrDraft != nil && newDraftVal != nil {
					changed = *t.PrDraft != *newDraftVal
				}
				if changed {
					t.PrDraft = newDraftVal
					changes = append(changes, "pr_draft")
				}
			}

			// Update PR labels if flag was provided
			oldPrLabels := t.PrLabels
			if prLabelsChanged {
				// Empty slice clears labels
				if len(newPrLabels) == 0 {
					if len(t.PrLabels) > 0 {
						t.PrLabels = nil
						t.PrLabelsSet = false
						changes = append(changes, "pr_labels")
					}
				} else {
					t.PrLabels = newPrLabels
					t.PrLabelsSet = true
					changes = append(changes, "pr_labels")
				}
			}

			// Update PR reviewers if flag was provided
			oldPrReviewers := t.PrReviewers
			if prReviewersChanged {
				// Empty slice clears reviewers
				if len(newPrReviewers) == 0 {
					if len(t.PrReviewers) > 0 {
						t.PrReviewers = nil
						t.PrReviewersSet = false
						changes = append(changes, "pr_reviewers")
					}
				} else {
					t.PrReviewers = newPrReviewers
					t.PrReviewersSet = true
					changes = append(changes, "pr_reviewers")
				}
			}

			// Handle dependency updates
			// Use cmd.Flags().Changed() to detect explicit flag usage (including empty strings to clear)
			blockedByChanged := cmd.Flags().Changed("blocked-by")
			relatedToChanged := cmd.Flags().Changed("related-to")
			hasDepChanges := blockedByChanged || len(addBlockers) > 0 || len(removeBlockers) > 0 ||
				relatedToChanged || len(addRelated) > 0 || len(removeRelated) > 0

			if hasDepChanges {
				// Load all tasks for validation
				allTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks for validation: %w", err)
				}

				existingIDs := make(map[string]bool)
				taskMap := make(map[string]*orcv1.Task)
				for _, existing := range allTasks {
					existingIDs[existing.Id] = true
					taskMap[existing.Id] = existing
				}

				// Handle blocked_by changes
				if blockedByChanged {
					// Replace entire list (can be empty to clear all blockers)
					if len(blockedBy) > 0 {
						if errs := task.ValidateBlockedBy(taskID, blockedBy, existingIDs); len(errs) > 0 {
							return errs[0]
						}
						// Check for circular dependencies with all new blockers at once
						if cycle := task.DetectCircularDependencyWithAllProto(taskID, blockedBy, taskMap); cycle != nil {
							return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
						}
					}
					t.BlockedBy = blockedBy
					changes = append(changes, "blocked_by")
				} else if len(addBlockers) > 0 || len(removeBlockers) > 0 {
					// Handle add/remove
					if len(addBlockers) > 0 {
						if errs := task.ValidateBlockedBy(taskID, addBlockers, existingIDs); len(errs) > 0 {
							return errs[0]
						}
						for _, newBlocker := range addBlockers {
							if cycle := task.DetectCircularDependencyWithAllProto(taskID, []string{newBlocker}, taskMap); cycle != nil {
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
				if relatedToChanged {
					// Replace entire list (can be empty to clear all related tasks)
					if len(relatedTo) > 0 {
						if errs := task.ValidateRelatedTo(taskID, relatedTo, existingIDs); len(errs) > 0 {
							return errs[0]
						}
					}
					t.RelatedTo = relatedTo
					changes = append(changes, "related_to")
				} else if len(addRelated) > 0 || len(removeRelated) > 0 {
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
				if err := regeneratePlanForWeightProto(backend, t); err != nil {
					return fmt.Errorf("regenerate plan: %w", err)
				}
			}

			// Handle initiative change - sync bidirectionally
			currentInit := task.GetInitiativeIDProto(t)
			if initiativeChanged && oldInitiative != currentInit {
				// Remove from old initiative if it was linked
				if oldInitiative != "" {
					if oldInit, err := backend.LoadInitiative(oldInitiative); err == nil {
						oldInit.RemoveTask(t.Id)
						if err := backend.SaveInitiative(oldInit); err != nil {
							fmt.Printf("Warning: failed to remove task from old initiative: %v\n", err)
						}
					}
				}
				// Add to new initiative if linking
				if task.HasInitiativeProto(t) {
					if newInit, err := backend.LoadInitiative(currentInit); err == nil {
						newInit.AddTask(t.Id, t.Title, nil)
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
						desc := ""
						if t.Description != nil {
							desc = *t.Description
						}
						if len(desc) > 60 {
							desc = desc[:57] + "..."
						}
						fmt.Printf("   Description: %s\n", desc)
					case "weight":
						fmt.Printf("   Weight: %s -> %s (plan regenerated)\n", task.WeightFromProto(oldWeight), task.WeightFromProto(t.Weight))
					case "workflow":
						currentWorkflow := ""
						if t.WorkflowId != nil {
							currentWorkflow = *t.WorkflowId
						}
						if currentWorkflow != "" {
							if oldWorkflow == "" {
								fmt.Printf("   Workflow: set to %s\n", currentWorkflow)
							} else {
								fmt.Printf("   Workflow: %s -> %s\n", oldWorkflow, currentWorkflow)
							}
						} else {
							fmt.Printf("   Workflow: cleared (was %s, task cannot run without workflow_id)\n", oldWorkflow)
						}
					case "priority":
						fmt.Printf("   Priority: %s -> %s\n", task.PriorityFromProto(oldPriority), task.PriorityFromProto(t.Priority))
					case "status":
						fmt.Printf("   Status: %s -> %s\n", task.StatusFromProto(oldStatus), task.StatusFromProto(t.Status))
					case "initiative":
						if task.HasInitiativeProto(t) {
							if oldInitiative == "" {
								fmt.Printf("   Initiative: linked to %s\n", task.GetInitiativeIDProto(t))
							} else {
								fmt.Printf("   Initiative: %s -> %s\n", oldInitiative, task.GetInitiativeIDProto(t))
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
					case "target_branch":
						currentTargetBranch := task.GetTargetBranchProto(t)
						if currentTargetBranch != "" {
							if oldTargetBranch == "" {
								fmt.Printf("   Target Branch: set to %s\n", currentTargetBranch)
							} else {
								fmt.Printf("   Target Branch: %s -> %s\n", oldTargetBranch, currentTargetBranch)
							}
						} else {
							fmt.Printf("   Target Branch: cleared (was %s)\n", oldTargetBranch)
						}
					case "branch_name":
						currentBranchName := ""
						if t.BranchName != nil {
							currentBranchName = *t.BranchName
						}
						if currentBranchName != "" {
							if oldBranchName == "" {
								fmt.Printf("   Branch Name: set to %s\n", currentBranchName)
							} else {
								fmt.Printf("   Branch Name: %s -> %s\n", oldBranchName, currentBranchName)
							}
						} else {
							fmt.Printf("   Branch Name: cleared (was %s)\n", oldBranchName)
						}
					case "pr_draft":
						if t.PrDraft != nil {
							newVal := "false"
							if *t.PrDraft {
								newVal = "true"
							}
							if oldPrDraft == nil {
								fmt.Printf("   PR Draft: set to %s\n", newVal)
							} else {
								oldVal := "false"
								if *oldPrDraft {
									oldVal = "true"
								}
								fmt.Printf("   PR Draft: %s -> %s\n", oldVal, newVal)
							}
						} else {
							fmt.Printf("   PR Draft: cleared (will use default)\n")
						}
					case "pr_labels":
						if len(t.PrLabels) > 0 {
							if len(oldPrLabels) == 0 {
								fmt.Printf("   PR Labels: set to [%s]\n", strings.Join(t.PrLabels, ", "))
							} else {
								fmt.Printf("   PR Labels: [%s] -> [%s]\n", strings.Join(oldPrLabels, ", "), strings.Join(t.PrLabels, ", "))
							}
						} else {
							fmt.Printf("   PR Labels: cleared (was [%s])\n", strings.Join(oldPrLabels, ", "))
						}
					case "pr_reviewers":
						if len(t.PrReviewers) > 0 {
							if len(oldPrReviewers) == 0 {
								fmt.Printf("   PR Reviewers: set to [%s]\n", strings.Join(t.PrReviewers, ", "))
							} else {
								fmt.Printf("   PR Reviewers: [%s] -> [%s]\n", strings.Join(oldPrReviewers, ", "), strings.Join(t.PrReviewers, ", "))
							}
						} else {
							fmt.Printf("   PR Reviewers: cleared (was [%s])\n", strings.Join(oldPrReviewers, ", "))
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
	cmd.Flags().String("workflow", "", "new task workflow (e.g., qa-e2e, implement)")
	cmd.Flags().StringP("priority", "p", "", "new task priority (critical, high, normal, low)")
	cmd.Flags().StringP("status", "s", "", "new task status (created, classifying, planned, paused, blocked, completed, failed)")
	cmd.Flags().StringP("initiative", "i", "", "link/unlink task to initiative (use \"\" to unlink)")
	cmd.Flags().String("target-branch", "", "override PR target branch for this task (use \"\" to clear)")

	// Branch control flags
	cmd.Flags().String("branch-name", "", "custom branch name (overrides auto-generated from task ID)")
	cmd.Flags().String("pr-draft", "", "PR draft mode (true/false, \"\" to clear)")
	cmd.Flags().StringSlice("pr-labels", nil, "PR labels (comma-separated, replaces existing)")
	cmd.Flags().StringSlice("pr-reviewers", nil, "PR reviewers (comma-separated, replaces existing)")

	// Dependency flags
	cmd.Flags().StringSlice("blocked-by", nil, "set blocked_by list (replaces existing)")
	cmd.Flags().StringSlice("add-blocker", nil, "add task(s) to blocked_by list")
	cmd.Flags().StringSlice("remove-blocker", nil, "remove task(s) from blocked_by list")
	cmd.Flags().StringSlice("related-to", nil, "set related_to list (replaces existing)")
	cmd.Flags().StringSlice("add-related", nil, "add task(s) to related_to list")
	cmd.Flags().StringSlice("remove-related", nil, "remove task(s) from related_to list")

	return cmd
}

// regeneratePlanForWeightProto resets the execution state when task weight changes.
// Plans are created dynamically at execution time from task weight,
// so we only need to reset the state for re-execution.
func regeneratePlanForWeightProto(backend storage.Backend, t *orcv1.Task) error {
	// Reset execution state for fresh execution
	// Note: task.Status is the source of truth (updated below)
	t.CurrentPhase = nil
	task.EnsureExecutionProto(t)
	t.Execution.CurrentIteration = 0
	t.Execution.Error = nil
	t.Execution.RetryContext = nil
	t.Execution.Phases = make(map[string]*orcv1.PhaseState)

	// Update task status to planned
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	return nil
}
