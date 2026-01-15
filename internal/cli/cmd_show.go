// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
)

// newShowCmd creates the show command
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer backend.Close()

			id := args[0]

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			p, _ := backend.LoadPlan(id)
			s, _ := backend.LoadState(id)

			// Print task details
			fmt.Printf("\n%s - %s\n", t.ID, t.Title)
			fmt.Printf("────────────────────────────────────────────\n")
			fmt.Printf("Status:    %s\n", t.Status)
			fmt.Printf("Weight:    %s\n", t.Weight)
			fmt.Printf("Branch:    %s\n", t.Branch)
			fmt.Printf("Created:   %s\n", t.CreatedAt.Format(time.RFC3339))

			if t.StartedAt != nil {
				fmt.Printf("Started:   %s\n", t.StartedAt.Format(time.RFC3339))
			}
			if t.CompletedAt != nil {
				fmt.Printf("Completed: %s\n", t.CompletedAt.Format(time.RFC3339))
			}

			if t.Description != "" {
				fmt.Printf("\nDescription:\n%s\n", t.Description)
			}

			// Print phases
			if p != nil && len(p.Phases) > 0 {
				fmt.Printf("\nPhases:\n")
				for _, phase := range p.Phases {
					status := phaseStatusIcon(phase.Status)
					fmt.Printf("  %s %s", status, phase.ID)
					if phase.CommitSHA != "" {
						fmt.Printf(" (commit: %s)", phase.CommitSHA[:7])
					}
					fmt.Println()
				}
			}

			// Print execution state
			if s != nil && s.Tokens.TotalTokens > 0 {
				fmt.Printf("\nTokens Used: %d\n", s.Tokens.TotalTokens)
			}

			return nil
		},
	}
}
