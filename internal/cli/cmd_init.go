// Package cli implements the orc command-line interface.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
)

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize orc in current project",
		Long: `Initialize orc in the current directory.

This is a fast, instant initialization (< 500ms) that:
  • Creates .orc/ directory with config.yaml
  • Creates SQLite database for task tracking
  • Detects project type and stores results
  • Registers project in global registry
  • Updates .gitignore

For AI-powered project configuration, run 'orc setup' after init.

Example:
  orc init                    # Instant initialization
  orc init --force            # Reinitialize existing project
  orc init --profile strict   # Initialize with strict profile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			profile, _ := cmd.Flags().GetString("profile")

			opts := bootstrap.Options{
				Force: force,
			}

			if profile != "" {
				opts.Profile = config.AutomationProfile(profile)
			}

			result, err := bootstrap.Run(opts)
			if err != nil {
				return err
			}

			bootstrap.PrintResult(result)
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	cmd.Flags().String("profile", "", "set automation profile (auto, fast, safe, strict)")
	return cmd
}
