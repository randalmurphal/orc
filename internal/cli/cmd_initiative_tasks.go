package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
)

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

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			initID := args[0]
			taskID := args[1]
			dependsOn, _ := cmd.Flags().GetStringSlice("depends-on")

			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			t, err := backend.LoadTask(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			init.AddTask(taskID, t.Title, dependsOn)
			if err := backend.SaveInitiative(init); err != nil {
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

	return cmd
}

func newInitiativeLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <initiative-id> <task-id>...",
		Short: "Batch link multiple tasks to an initiative",
		Long: `Link multiple tasks to an initiative at once.

Examples:
  orc initiative link INIT-001 TASK-060 TASK-061 TASK-062
  orc initiative link INIT-001 --all-matching "auth"        # Link tasks matching pattern
  orc initiative link INIT-001 --all-matching "TASK-06"     # Link tasks by ID pattern`,
		Args: cobra.MinimumNArgs(1),
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
			taskIDs := args[1:]
			allMatching, _ := cmd.Flags().GetString("all-matching")

			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			type taskInfo struct {
				ID           string
				Title        string
				InitiativeID string
			}
			var tasksToLink []taskInfo

			if allMatching != "" {
				allTasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks: %w", err)
				}

				pattern := strings.ToLower(allMatching)
				for _, t := range allTasks {
					if strings.Contains(strings.ToLower(t.Id), pattern) ||
						strings.Contains(strings.ToLower(t.Title), pattern) {
						taskInitID := ""
						if t.InitiativeId != nil {
							taskInitID = *t.InitiativeId
						}
						if taskInitID == initID && init.HasTask(t.Id) {
							continue
						}
						tasksToLink = append(tasksToLink, taskInfo{ID: t.Id, Title: t.Title, InitiativeID: taskInitID})
					}
				}

				if len(tasksToLink) == 0 {
					fmt.Printf("No unlinked tasks matching %q found.\n", allMatching)
					return nil
				}
			}

			for _, taskID := range taskIDs {
				t, err := backend.LoadTask(taskID)
				if err != nil {
					return fmt.Errorf("load task %s: %w", taskID, err)
				}
				taskInitID := ""
				if t.InitiativeId != nil {
					taskInitID = *t.InitiativeId
				}
				if taskInitID == initID && init.HasTask(t.Id) {
					fmt.Printf("Skipping %s: already linked to %s\n", taskID, initID)
					continue
				}
				tasksToLink = append(tasksToLink, taskInfo{ID: t.Id, Title: t.Title, InitiativeID: taskInitID})
			}

			if len(tasksToLink) == 0 {
				fmt.Println("No tasks to link.")
				return nil
			}

			var linked []string
			var skippedOther []string
			for _, ti := range tasksToLink {
				if ti.InitiativeID != "" && ti.InitiativeID != initID {
					skippedOther = append(skippedOther, fmt.Sprintf("%s (linked to %s)", ti.ID, ti.InitiativeID))
					continue
				}

				t, err := backend.LoadTask(ti.ID)
				if err != nil {
					return fmt.Errorf("load task %s for update: %w", ti.ID, err)
				}

				t.InitiativeId = &initID
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("save task %s: %w", t.Id, err)
				}

				init.AddTask(t.Id, t.Title, nil)
				linked = append(linked, t.Id)
			}

			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			if len(linked) > 0 {
				fmt.Printf("Linked %d task(s) to %s:\n", len(linked), initID)
				for _, id := range linked {
					fmt.Printf("  • %s\n", id)
				}
			}
			if len(skippedOther) > 0 {
				fmt.Printf("\nSkipped %d task(s) already linked to other initiatives:\n", len(skippedOther))
				for _, info := range skippedOther {
					fmt.Printf("  • %s\n", info)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("all-matching", "", "link all tasks matching pattern (matches ID or title)")

	return cmd
}

func newInitiativeUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <initiative-id> <task-id>...",
		Short: "Remove tasks from an initiative",
		Long: `Remove one or more tasks from an initiative.

Examples:
  orc initiative unlink INIT-001 TASK-060
  orc initiative unlink INIT-001 TASK-060 TASK-061 TASK-062
  orc initiative unlink INIT-001 --all   # Unlink all tasks from initiative`,
		Args: cobra.MinimumNArgs(1),
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
			taskIDs := args[1:]
			unlinkAll, _ := cmd.Flags().GetBool("all")

			init, err := backend.LoadInitiative(initID)
			if err != nil {
				return fmt.Errorf("load initiative: %w", err)
			}

			if unlinkAll {
				taskIDs = make([]string, len(init.Tasks))
				for i, t := range init.Tasks {
					taskIDs[i] = t.ID
				}
			}

			if len(taskIDs) == 0 {
				fmt.Println("No tasks to unlink.")
				return nil
			}

			var unlinked []string
			var notFound []string
			for _, taskID := range taskIDs {
				t, err := backend.LoadTask(taskID)
				if err != nil {
					notFound = append(notFound, taskID)
					continue
				}

				taskInitID := ""
				if t.InitiativeId != nil {
					taskInitID = *t.InitiativeId
				}
				if taskInitID != initID {
					if taskInitID == "" {
						fmt.Printf("Skipping %s: not linked to any initiative\n", taskID)
					} else {
						fmt.Printf("Skipping %s: linked to %s, not %s\n", taskID, taskInitID, initID)
					}
					continue
				}

				t.InitiativeId = nil
				if err := backend.SaveTask(t); err != nil {
					return fmt.Errorf("save task %s: %w", taskID, err)
				}

				init.RemoveTask(taskID)
				unlinked = append(unlinked, taskID)
			}

			if err := backend.SaveInitiative(init); err != nil {
				return fmt.Errorf("save initiative: %w", err)
			}

			if len(unlinked) > 0 {
				fmt.Printf("Unlinked %d task(s) from %s:\n", len(unlinked), initID)
				for _, id := range unlinked {
					fmt.Printf("  • %s\n", id)
				}
			}
			if len(notFound) > 0 {
				fmt.Printf("\nCould not find %d task(s):\n", len(notFound))
				for _, id := range notFound {
					fmt.Printf("  • %s\n", id)
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("all", false, "unlink all tasks from the initiative")

	return cmd
}
