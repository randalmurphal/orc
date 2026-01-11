// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newListCmd creates the list command
func newListCmd() *cobra.Command {
	return &cobra.Command{
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

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found. Create one with: orc new \"Your task\"")
				return nil
			}

			// Print tasks in table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tWEIGHT\tPHASE\tTITLE")
			fmt.Fprintln(w, "──\t──────\t──────\t─────\t─────")

			for _, t := range tasks {
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
}
