// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newListCmd creates the list command
func newListCmd() *cobra.Command {
	var statusFilter string
	var weightFilter string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks",
		Long: `List all tasks in the current project.

Example:
  orc list
  orc list --status running
  orc list --weight large`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			// Validate status filter if provided
			if statusFilter != "" {
				if !task.IsValidStatus(task.Status(statusFilter)) {
					validStatuses := make([]string, len(task.ValidStatuses()))
					for i, s := range task.ValidStatuses() {
						validStatuses[i] = string(s)
					}
					return fmt.Errorf("invalid status %q, valid values: %s", statusFilter, strings.Join(validStatuses, ", "))
				}
			}

			// Validate weight filter if provided
			if weightFilter != "" {
				if !task.IsValidWeight(task.Weight(weightFilter)) {
					validWeights := make([]string, len(task.ValidWeights()))
					for i, w := range task.ValidWeights() {
						validWeights[i] = string(w)
					}
					return fmt.Errorf("invalid weight %q, valid values: %s", weightFilter, strings.Join(validWeights, ", "))
				}
			}

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			// Apply filters
			var filtered []*task.Task
			for _, t := range tasks {
				if statusFilter != "" && string(t.Status) != statusFilter {
					continue
				}
				if weightFilter != "" && string(t.Weight) != weightFilter {
					continue
				}
				filtered = append(filtered, t)
			}

			out := cmd.OutOrStdout()

			if len(filtered) == 0 {
				if statusFilter != "" || weightFilter != "" {
					fmt.Fprintln(out, "No tasks match the specified filters.")
				} else {
					fmt.Fprintln(out, "No tasks found. Create one with: orc new \"Your task\"")
				}
				return nil
			}

			// Print tasks in table format
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tWEIGHT\tPHASE\tTITLE")
			fmt.Fprintln(w, "──\t──────\t──────\t─────\t─────")

			for _, t := range filtered {
				status := statusIcon(t.Status)
				phase := t.CurrentPhase
				if phase == "" {
					phase = "-"
				}
				title := truncate(t.Title, 40)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, status, t.Weight, phase, title)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&statusFilter, "status", "s", "", "filter by status (created, planned, running, paused, blocked, completed, finished, failed)")
	cmd.Flags().StringVarP(&weightFilter, "weight", "w", "", "filter by weight (trivial, small, medium, large, greenfield)")

	return cmd
}
