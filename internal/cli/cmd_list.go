// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
  orc list --initiative unassigned
  orc list -n 5                      # Show 5 most recent tasks
  orc list --status pending -n 10    # Show 10 most recent pending tasks`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Get filter flags
			initiativeFilter, _ := cmd.Flags().GetString("initiative")
			statusFilter, _ := cmd.Flags().GetString("status")
			weightFilter, _ := cmd.Flags().GetString("weight")
			limit, _ := cmd.Flags().GetInt("limit")

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
				_, _ = fmt.Fprintln(out, "No tasks found. Create one with: orc new \"Your task\"")
				return nil
			}

			// Apply filters
			var filtered []*orcv1.Task
			for _, t := range tasks {
				// Initiative filter
				if initiativeFilterActive {
					// Empty string or "unassigned" means show tasks without initiative
					initID := task.GetInitiativeIDProto(t)
					if initiativeFilter == "" || strings.ToLower(initiativeFilter) == "unassigned" {
						if initID != "" {
							continue
						}
					} else {
						if initID != initiativeFilter {
							continue
						}
					}
				}

				// Status filter
				if statusFilter != "" {
					if !matchStatusProto(t.Status, statusFilter) {
						continue
					}
				}

				// Weight filter
				if weightFilter != "" {
					if !matchWeightProto(t.Weight, weightFilter) {
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
				_, _ = fmt.Fprintf(out, "No tasks found matching: %s\n", strings.Join(filterDesc, ", "))
				return nil
			}

			// Apply limit after filtering (take the last N tasks for most recent)
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[len(filtered)-limit:]
			}

			// Print tasks in table format
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATUS\tWEIGHT\tPHASE\tTITLE")
			_, _ = fmt.Fprintln(w, "──\t──────\t──────\t─────\t─────")

			for _, t := range filtered {
				status := statusIcon(t.Status)
				phase := task.GetCurrentPhaseProto(t)
				if phase == "" {
					phase = "-"
				}
				title := truncate(t.Title, 40)
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.Id, status, weightStringProto(t.Weight), phase, title)
			}

			_ = w.Flush()
			return nil
		},
	}

	// Add filter flags
	cmd.Flags().StringP("initiative", "i", "", "filter by initiative ID (use 'unassigned' or '' for tasks without initiative)")
	cmd.Flags().StringP("status", "s", "", "filter by status (pending, running, completed, etc.)")
	cmd.Flags().StringP("weight", "w", "", "filter by weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().IntP("limit", "n", 0, "limit output to N most recent tasks (0 for all)")

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
	defer func() { _ = backend.Close() }()

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
