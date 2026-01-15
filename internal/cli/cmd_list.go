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
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks",
		Long: `List all tasks in the current project.

Filter by initiative:
  --initiative INIT-001    Show tasks in that initiative
  --initiative unassigned  Show tasks not in any initiative
  --initiative ""          Same as unassigned

Example:
  orc list
  orc list --status running
  orc list --weight large
  orc list --initiative INIT-001
  orc list --initiative unassigned`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer backend.Close()

			// Get filter flags
			initiativeFilter, _ := cmd.Flags().GetString("initiative")
			statusFilter, _ := cmd.Flags().GetString("status")
			weightFilter, _ := cmd.Flags().GetString("weight")

			// Validate initiative filter if provided (unless it's "unassigned" or empty)
			initiativeFilterActive := cmd.Flags().Changed("initiative")
			if initiativeFilterActive && initiativeFilter != "" && initiativeFilter != "unassigned" {
				exists, err := backend.InitiativeExists(initiativeFilter)
				if err != nil {
					return fmt.Errorf("check initiative: %w", err)
				}
				if !exists {
					return fmt.Errorf("initiative %s not found", initiativeFilter)
				}
			}

			tasks, err := backend.LoadAllTasks()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			out := cmd.OutOrStdout()

			if len(tasks) == 0 {
				fmt.Fprintln(out, "No tasks found. Create one with: orc new \"Your task\"")
				return nil
			}

			// Apply filters
			var filtered []*task.Task
			for _, t := range tasks {
				// Initiative filter
				if initiativeFilterActive {
					// Empty string or "unassigned" means show tasks without initiative
					if initiativeFilter == "" || strings.ToLower(initiativeFilter) == "unassigned" {
						if t.InitiativeID != "" {
							continue
						}
					} else {
						if t.InitiativeID != initiativeFilter {
							continue
						}
					}
				}

				// Status filter
				if statusFilter != "" {
					if string(t.Status) != statusFilter {
						continue
					}
				}

				// Weight filter
				if weightFilter != "" {
					if string(t.Weight) != weightFilter {
						continue
					}
				}

				filtered = append(filtered, t)
			}

			if len(filtered) == 0 {
				var filterDesc []string
				if initiativeFilterActive {
					if initiativeFilter == "" || strings.ToLower(initiativeFilter) == "unassigned" {
						filterDesc = append(filterDesc, "unassigned initiative")
					} else {
						filterDesc = append(filterDesc, fmt.Sprintf("initiative %s", initiativeFilter))
					}
				}
				if statusFilter != "" {
					filterDesc = append(filterDesc, fmt.Sprintf("status %s", statusFilter))
				}
				if weightFilter != "" {
					filterDesc = append(filterDesc, fmt.Sprintf("weight %s", weightFilter))
				}
				fmt.Fprintf(out, "No tasks found matching: %s\n", strings.Join(filterDesc, ", "))
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

	// Add filter flags
	cmd.Flags().StringP("initiative", "i", "", "filter by initiative ID (use 'unassigned' or '' for tasks without initiative)")
	cmd.Flags().StringP("status", "s", "", "filter by status (pending, running, completed, etc.)")
	cmd.Flags().StringP("weight", "w", "", "filter by weight (trivial, small, medium, large, greenfield)")

	// Register completion function for initiative flag
	_ = cmd.RegisterFlagCompletionFunc("initiative", completeInitiativeIDs)

	return cmd
}

// completeInitiativeIDs provides tab completion for initiative IDs
func completeInitiativeIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Load initiatives via backend (ignore errors for completion)
	backend, err := getBackend()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer backend.Close()

	inits, err := backend.LoadAllInitiatives()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Build completion list
	var completions []string
	completions = append(completions, "unassigned\ttasks without an initiative")
	for _, init := range inits {
		// Filter by prefix if user started typing
		if toComplete == "" || strings.HasPrefix(init.ID, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s", init.ID, truncate(init.Title, 30)))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
