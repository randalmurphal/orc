// Package cli implements the orc command-line interface.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/wizard"
)

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize orc in current project",
		Long: `Initialize orc configuration in the current directory.

Creates .orc/ directory with:
  • config.yaml - project configuration
  • tasks/ - task storage directory

Detects project type and suggests appropriate settings.
Also registers the project in the global registry (~/.orc/projects.yaml).

Example:
  orc init            # Interactive initialization
  orc init --quick    # Non-interactive with defaults
  orc init --force    # Overwrite existing config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			quick, _ := cmd.Flags().GetBool("quick")
			profile, _ := cmd.Flags().GetString("profile")

			result, err := wizard.Run(wizard.Options{
				Force:   force,
				Quick:   quick,
				Profile: profile,
			})
			if err != nil {
				return err
			}

			wizard.PrintResult(result)
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	cmd.Flags().Bool("quick", false, "skip interactive prompts, use defaults")
	cmd.Flags().String("profile", "", "set automation profile (auto, fast, safe, strict)")
	return cmd
}
