// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/setup"
)

// newSetupCmd creates the setup command
func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure project with Claude assistance",
		Long: `Launch an interactive Claude session to configure your project.

This command spawns Claude to:
  • Analyze your project structure
  • Update or create CLAUDE.md with orc-specific settings
  • Optionally create skills or custom prompts

The setup adapts to project size:
  • Small projects: Quick scan, minimal configuration
  • Medium projects: Document patterns and conventions
  • Large/monorepo: Ask which areas to focus on

Prerequisites:
  • Run 'orc init' first to detect project type
  • Claude CLI must be installed and authenticated

Example:
  orc setup                   # Interactive Claude setup
  orc setup --dry-run         # Show the prompt without running Claude
  orc setup --model sonnet    # Use a specific model`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			model, _ := cmd.Flags().GetString("model")
			skipValidation, _ := cmd.Flags().GetBool("skip-validation")

			// Create cancellable context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			opts := setup.Options{
				DryRun:         dryRun,
				Model:          model,
				SkipValidation: skipValidation,
			}

			result, err := setup.Run(ctx, opts)
			if err != nil {
				return err
			}

			// Print validation results if any
			if !dryRun && !skipValidation {
				if result.Validated {
					fmt.Println("\nSetup completed successfully.")
				} else if len(result.ValidationErrors) > 0 {
					fmt.Println("\nSetup completed with warnings:")
					for _, e := range result.ValidationErrors {
						fmt.Printf("  • %s\n", e)
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "show the prompt without running Claude")
	cmd.Flags().String("model", "", "Claude model to use (default: claude-opus-4-5-20251101)")
	cmd.Flags().Bool("skip-validation", false, "skip output validation after setup")
	return cmd
}
