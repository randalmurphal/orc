// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// newInitiativePlanCmdWithTriggerRunner creates the initiative plan command with a trigger runner for testing.
func newInitiativePlanCmdWithTriggerRunner(runner CLIInitiativeTriggerRunner) *cobra.Command {
	return newInitiativePlanCmdInternal(runner)
}

func newInitiativePlanCmd() *cobra.Command {
	return newInitiativePlanCmdInternal(nil)
}

func newInitiativePlanCmdInternal(triggerRunner CLIInitiativeTriggerRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan <manifest.yaml>",
		Short: "Create multiple tasks from a manifest file",
		Long: `Create multiple tasks from a YAML manifest file.

The manifest format allows defining multiple tasks with their specifications,
weights, categories, priorities, and dependencies in a single file.

Tasks are created in dependency order, and inline specs are stored in the
database. Tasks with specs will skip the spec phase during execution.

Manifest Format:
  version: 1
  initiative: INIT-001           # OR create_initiative below
  create_initiative:
    title: "Initiative Title"
    vision: "Optional vision statement"
  tasks:
    - id: 1                      # Local ID for dependency refs
      title: "Task title"
      weight: medium             # trivial/small/medium/large/greenfield
      category: feature          # feature/bug/refactor/chore/docs/test
      priority: normal           # critical/high/normal/low
      description: |
        Optional detailed description
      spec: |
        # Inline Specification
        Markdown spec content...
      depends_on: [1, 2]         # Local IDs of prerequisite tasks

Examples:
  orc initiative plan tasks.yaml              # Create tasks, prompt for confirm
  orc initiative plan tasks.yaml --dry-run    # Preview without creating
  orc initiative plan tasks.yaml --yes        # Skip confirmation prompt
  orc initiative plan tasks.yaml --create-initiative  # Create initiative if missing`,
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

			manifestPath := args[0]
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			skipConfirm, _ := cmd.Flags().GetBool("yes")
			createInitiative, _ := cmd.Flags().GetBool("create-initiative")

			// Parse manifest
			manifest, err := initiative.ParseManifest(manifestPath)
			if err != nil {
				return err
			}

			// Validate manifest
			if errs := initiative.ValidateManifest(manifest); len(errs) > 0 {
				fmt.Println("Manifest validation errors:")
				for _, e := range errs {
					fmt.Printf("  - %v\n", e)
				}
				return fmt.Errorf("manifest validation failed with %d error(s)", len(errs))
			}

			// Determine target initiative
			var targetInitiativeID string
			var initToCreate *initiative.Initiative

			if manifest.CreateInitiative != nil {
				// Generate initiative ID
				id, err := backend.GetNextInitiativeID()
				if err != nil {
					return fmt.Errorf("generate initiative ID: %w", err)
				}
				targetInitiativeID = id
				initToCreate = initiative.New(id, manifest.CreateInitiative.Title)
				if manifest.CreateInitiative.Vision != "" {
					initToCreate.Vision = manifest.CreateInitiative.Vision
				}
			} else {
				// Check if initiative exists
				exists, err := backend.InitiativeExists(manifest.Initiative)
				if err != nil {
					return fmt.Errorf("check initiative: %w", err)
				}
				if !exists {
					if createInitiative {
						// Create a new initiative with the ID
						initToCreate = initiative.New(manifest.Initiative, manifest.Initiative)
					} else {
						return fmt.Errorf("initiative %s not found (use --create-initiative to create it)", manifest.Initiative)
					}
				}
				targetInitiativeID = manifest.Initiative
			}

			// Get topological order
			order, err := initiative.TopologicalSort(manifest.Tasks)
			if err != nil {
				return fmt.Errorf("sort tasks: %w", err)
			}

			// Count tasks with specs
			specsCount := 0
			for _, t := range manifest.Tasks {
				if t.Spec != "" {
					specsCount++
				}
			}

			// Preview mode
			fmt.Println("Manifest Summary:")
			if initToCreate != nil {
				fmt.Printf("  Initiative: %s (will be created)\n", targetInitiativeID)
				if initToCreate.Vision != "" {
					fmt.Printf("  Vision: %s\n", initToCreate.Vision)
				}
			} else {
				fmt.Printf("  Initiative: %s (existing)\n", targetInitiativeID)
			}
			fmt.Printf("  Tasks: %d\n", len(manifest.Tasks))
			fmt.Printf("  With specs: %d (will skip spec phase)\n", specsCount)
			fmt.Println()

			fmt.Println("Tasks to create (in dependency order):")
			for _, idx := range order {
				t := manifest.Tasks[idx]
				weight := t.Weight
				if weight == "" {
					weight = "medium"
				}
				hasSpec := ""
				if t.Spec != "" {
					hasSpec = " [spec provided]"
				}
				deps := ""
				if len(t.DependsOn) > 0 {
					depStrs := make([]string, len(t.DependsOn))
					for i, d := range t.DependsOn {
						depStrs[i] = fmt.Sprintf("%d", d)
					}
					deps = fmt.Sprintf(" (depends on: %s)", strings.Join(depStrs, ", "))
				}
				fmt.Printf("  %d. [%s] %s%s%s\n", t.ID, weight, t.Title, deps, hasSpec)
			}
			fmt.Println()

			if dryRun {
				fmt.Println("Dry run - no tasks created.")
				return nil
			}

			// Confirm unless --yes
			if !skipConfirm {
				fmt.Print("Create these tasks? [y/N]: ")
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("read response: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Create initiative if needed
			if initToCreate != nil {
				if err := backend.SaveInitiative(initToCreate); err != nil {
					return fmt.Errorf("create initiative: %w", err)
				}
				fmt.Printf("Created initiative: %s\n", targetInitiativeID)
			}

			// Load initiative for updating
			init, err := backend.LoadInitiative(targetInitiativeID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			// Detect project characteristics for testing requirements
			detection, _ := detect.Detect(".")
			hasFrontend := detection != nil && detection.HasFrontend

			// Map local IDs to real task IDs as we create them
			localToTaskID := make(map[int]string)
			var createdTasks []string

			for _, idx := range order {
				mt := manifest.Tasks[idx]

				// Generate task ID
				taskID, err := backend.GetNextTaskID()
				if err != nil {
					return fmt.Errorf("generate task ID: %w", err)
				}
				localToTaskID[mt.ID] = taskID

				// Create task
				t := task.NewProtoTask(taskID, mt.Title)
				if mt.Description != "" {
					t.Description = &mt.Description
				}

				// Set weight (default medium)
				if mt.Weight != "" {
					t.Weight = task.WeightToProto(mt.Weight)
				} else {
					t.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
				}

				// Auto-assign workflow based on weight
				wfID := workflow.WeightToWorkflowID(t.Weight)
				if wfID != "" {
					t.WorkflowId = &wfID
				}

				// Set category (default feature)
				if mt.Category != "" {
					t.Category = task.CategoryToProto(mt.Category)
				}

				// Set priority (default normal)
				if mt.Priority != "" {
					t.Priority = task.PriorityToProto(mt.Priority)
				}

				// Link to initiative
				t.InitiativeId = &targetInitiativeID

				// Map dependencies
				var blockedBy []string
				for _, dep := range mt.DependsOn {
					if realID, ok := localToTaskID[dep]; ok {
						blockedBy = append(blockedBy, realID)
					}
				}
				t.BlockedBy = blockedBy

				// Set testing requirements
				task.SetTestingRequirementsProto(t, hasFrontend)

				// Save task
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("save task %s: %w", taskID, err)
				}

				// If spec is provided, save it to database
				// The executor will create the plan dynamically at runtime
				// and skip the spec phase since a spec already exists
				if mt.Spec != "" {
					if err := backend.SaveSpecForTask(taskID, mt.Spec, "manifest"); err != nil {
						return fmt.Errorf("save spec for %s: %w", taskID, err)
					}
				}

				// Update task status to planned
				t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("update task %s: %w", taskID, err)
				}

				// Add to initiative
				init.AddTask(taskID, t.Title, blockedBy)
				createdTasks = append(createdTasks, taskID)

				// Output
				specNote := ""
				if mt.Spec != "" {
					specNote = " (spec stored)"
				}
				fmt.Printf("Created task: %s - %s [%s]%s\n", taskID, t.Title, task.WeightFromProto(t.Weight), specNote)
			}

			// Save initiative with updated task list
			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("update initiative: %w", err)
			}

			// Fire on_initiative_planned lifecycle trigger
			if triggerRunner != nil && len(createdTasks) > 0 {
				if err := triggerRunner.RunInitiativePlannedTrigger(
					context.Background(),
					nil, // Triggers resolved by the runner
					targetInitiativeID,
					createdTasks,
				); err != nil {
					// Log but don't fail the plan command
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: initiative planned trigger failed: %v\n", err)
				}
			}

			fmt.Printf("\nSummary: %d task(s) created in %s\n", len(createdTasks), targetInitiativeID)

			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "preview tasks without creating them")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
	cmd.Flags().Bool("create-initiative", false, "create initiative if it doesn't exist")

	return cmd
}
