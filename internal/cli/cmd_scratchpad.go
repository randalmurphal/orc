package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

func newScratchpadCmd() *cobra.Command {
	var phase string

	cmd := &cobra.Command{
		Use:   "scratchpad <task-id>",
		Short: "View scratchpad entries for a task",
		Long: `View structured notes (observations, decisions, blockers) that agents
recorded during phase execution.

Examples:
  orc scratchpad TASK-001              # All entries
  orc scratchpad TASK-001 --phase spec # Only spec phase entries`,
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
			return runScratchpadCmd(backend, taskID, phase)
		},
	}

	cmd.Flags().StringVar(&phase, "phase", "", "filter entries by phase")

	return cmd
}

func runScratchpadCmd(backend storage.Backend, taskID, phase string) error {
	var entries []storage.ScratchpadEntry
	var err error

	if phase != "" {
		entries, err = backend.GetScratchpadEntriesByPhase(taskID, phase)
	} else {
		entries, err = backend.GetScratchpadEntries(taskID)
	}
	if err != nil {
		return fmt.Errorf("load scratchpad entries: %w", err)
	}

	if len(entries) == 0 {
		if phase != "" {
			fmt.Printf("no scratchpad entries found for phase: %s\n", phase)
		} else {
			fmt.Println("no scratchpad entries found")
		}
		return nil
	}

	// Group entries by phase for display
	type phaseGroup struct {
		phaseID string
		entries []storage.ScratchpadEntry
	}
	var groups []phaseGroup
	seen := map[string]int{}

	for _, e := range entries {
		idx, ok := seen[e.PhaseID]
		if !ok {
			idx = len(groups)
			seen[e.PhaseID] = idx
			groups = append(groups, phaseGroup{phaseID: e.PhaseID})
		}
		groups[idx].entries = append(groups[idx].entries, e)
	}

	for _, g := range groups {
		fmt.Printf("=== %s ===\n", g.phaseID)
		for _, e := range g.entries {
			fmt.Printf("  [%s] %s\n", e.Category, e.Content)
		}
		fmt.Println()
	}

	return nil
}
