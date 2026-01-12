// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/spec"
)

func newSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "spec <title>",
		Deprecated: "Use 'orc plan' instead. Example: orc plan \"feature title\"",
		Short:      "Start interactive spec session with Claude",
		Long: `Start an interactive Claude session to collaboratively create a specification.

The session will:
1. Research your codebase
2. Ask clarifying questions
3. Propose implementation approaches
4. Create a structured spec document
5. Optionally generate tasks from the spec

Example:
  orc spec "Add user authentication"
  orc spec "Refactor payment processing" --initiative INIT-001
  orc spec "Add dark mode" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			title := args[0]
			initID, _ := cmd.Flags().GetString("initiative")
			model, _ := cmd.Flags().GetString("model")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			createTasks, _ := cmd.Flags().GetBool("create-tasks")
			shared, _ := cmd.Flags().GetBool("shared")

			ctx := context.Background()

			result, err := spec.Run(ctx, title, spec.Options{
				WorkDir:      ".",
				Model:        model,
				InitiativeID: initID,
				DryRun:       dryRun,
				CreateTasks:  createTasks,
				Shared:       shared,
			})
			if err != nil {
				return fmt.Errorf("spec session failed: %w", err)
			}

			if dryRun {
				return nil
			}

			if result.SpecPath != "" {
				fmt.Printf("\nSpec created: %s\n", result.SpecPath)
			}

			if len(result.TaskIDs) > 0 {
				fmt.Printf("Tasks created: %v\n", result.TaskIDs)
			}

			return nil
		},
	}

	cmd.Flags().String("initiative", "", "link to existing initiative")
	cmd.Flags().String("model", "", "Claude model to use")
	cmd.Flags().Bool("dry-run", false, "show prompt without running")
	cmd.Flags().Bool("create-tasks", false, "create tasks from spec output")
	cmd.Flags().Bool("shared", false, "use shared initiative")

	return cmd
}
