// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/spec"
)

func newFeatureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature <name>",
		Short: "Create a feature (initiative + spec + tasks)",
		Long: `Create a complete feature with initiative, spec, and tasks.

This is a combined workflow that:
1. Creates an initiative for the feature
2. Starts an interactive spec session
3. Creates tasks from the spec
4. Links everything together

Example:
  orc feature "Real-time notifications"
  orc feature "User dashboard" --shared`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			name := args[0]
			model, _ := cmd.Flags().GetString("model")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			shared, _ := cmd.Flags().GetBool("shared")
			owner, _ := cmd.Flags().GetString("owner")

			ctx := context.Background()

			// Step 1: Create initiative
			initID, err := initiative.NextID(shared)
			if err != nil {
				return fmt.Errorf("generate initiative ID: %w", err)
			}

			init := initiative.New(initID, name)
			if owner != "" {
				init.Owner = initiative.Identity{Initials: owner}
			}

			if dryRun {
				fmt.Printf("Would create initiative: %s - %s\n", initID, name)
				fmt.Println("\n=== Spec Session Prompt Preview ===")
			} else {
				var saveErr error
				if shared {
					saveErr = init.SaveShared()
				} else {
					saveErr = init.Save()
				}
				if saveErr != nil {
					return fmt.Errorf("save initiative: %w", saveErr)
				}

				fmt.Printf("Initiative created: %s\n", initID)
				fmt.Printf("   Title: %s\n", name)
				fmt.Printf("   Status: %s\n", init.Status)
				fmt.Println()
			}

			// Step 2: Start spec session linked to the initiative
			result, err := spec.Run(ctx, name, spec.Options{
				WorkDir:      ".",
				Model:        model,
				InitiativeID: initID,
				DryRun:       dryRun,
				CreateTasks:  true,
				Shared:       shared,
			})
			if err != nil {
				return fmt.Errorf("spec session failed: %w", err)
			}

			if dryRun {
				return nil
			}

			// Activate the initiative now that spec is done
			init.Activate()
			if shared {
				init.SaveShared()
			} else {
				init.Save()
			}

			fmt.Printf("\nFeature workflow complete!\n")
			fmt.Printf("   Initiative: %s (status: active)\n", initID)
			if result.SpecPath != "" {
				fmt.Printf("   Spec: %s\n", result.SpecPath)
			}
			if len(result.TaskIDs) > 0 {
				fmt.Printf("   Tasks: %v\n", result.TaskIDs)
			}

			fmt.Println("\nNext steps:")
			fmt.Printf("  orc initiative show %s    - View initiative details\n", initID)
			fmt.Printf("  orc initiative run %s     - Run all tasks\n", initID)

			return nil
		},
	}

	cmd.Flags().String("model", "", "Claude model to use")
	cmd.Flags().Bool("dry-run", false, "show what would happen without executing")
	cmd.Flags().Bool("shared", false, "create in shared directory for team access")
	cmd.Flags().StringP("owner", "o", "", "owner initials")

	return cmd
}
