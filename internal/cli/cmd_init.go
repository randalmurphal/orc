// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
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

			// Offer to set constitution file if found
			if result.FoundConstitution {
				fmt.Printf("\nFound constitution at %s\n", result.ConstitutionPath)
				fmt.Print("Set as project constitution? [Y/n] ")

				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(strings.ToLower(response))

				if response == "" || response == "y" || response == "yes" {
					// Load and save as constitution
					content, err := os.ReadFile(result.ConstitutionPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not read constitution file: %v\n", err)
					} else {
						backend, err := storage.NewBackend(".", &config.StorageConfig{})
						if err != nil {
							fmt.Fprintf(os.Stderr, "Warning: could not open storage: %v\n", err)
						} else {
							defer func() { _ = backend.Close() }()
							if err := backend.SaveConstitution(string(content)); err != nil {
								fmt.Fprintf(os.Stderr, "Warning: could not save constitution: %v\n", err)
							} else {
								fmt.Printf("Constitution set from %s\n", result.ConstitutionPath)
							}
						}
					}
				} else {
					fmt.Println("Skipped. You can set it later with: orc constitution set --file your-principles.md")
				}
			}

			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	cmd.Flags().String("profile", "", "set automation profile (auto, fast, safe, strict)")
	return cmd
}
