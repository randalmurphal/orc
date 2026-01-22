package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// newConstitutionCmd creates the constitution command with subcommands.
func newConstitutionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "constitution",
		Short: "Manage project constitution (principles that guide all tasks)",
		Long: `Manage the project's constitution - a set of principles that guide all task execution.

The constitution is injected into every phase prompt as {{CONSTITUTION_CONTENT}}.
Use it to encode project-specific rules, coding standards, architectural decisions,
and invariants that should always be followed.

Subcommands:
  set         Set the constitution from file or stdin
  show        Display the current constitution
  delete      Remove the constitution

Examples:
  # Set from file
  orc constitution set --file INVARIANTS.md

  # Set interactively (Ctrl+D to finish)
  orc constitution set

  # Set from stdin
  echo "# Rules" | orc constitution set

  # Show current constitution
  orc constitution show

  # Remove constitution
  orc constitution delete`,
	}

	cmd.AddCommand(newConstitutionSetCmd())
	cmd.AddCommand(newConstitutionShowCmd())
	cmd.AddCommand(newConstitutionDeleteCmd())

	return cmd
}

// newConstitutionSetCmd creates the 'constitution set' subcommand.
func newConstitutionSetCmd() *cobra.Command {
	var file string
	var version string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the project constitution",
		Long: `Set the project constitution from a file or stdin.

The constitution should contain markdown-formatted principles and rules
that apply to all task execution. Common sections include:

  - Coding standards
  - Architectural invariants
  - Testing requirements
  - Error handling patterns

Examples:
  # From file
  orc constitution set --file INVARIANTS.md

  # From stdin (pipe)
  cat INVARIANTS.md | orc constitution set

  # Interactive (Ctrl+D or Ctrl+C to finish)
  orc constitution set`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			// Load storage backend
			backend, err := storage.NewBackend(".", &config.StorageConfig{})
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer backend.Close()

			// Read content
			var content string
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				content = string(data)
			} else {
				// Check if stdin has data (pipe or redirect)
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					// Stdin is a pipe/file
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("read stdin: %w", err)
					}
					content = string(data)
				} else {
					// Interactive mode
					fmt.Fprintln(cmd.ErrOrStderr(), "Enter constitution content (Ctrl+D when done):")
					scanner := bufio.NewScanner(os.Stdin)
					var lines []string
					for scanner.Scan() {
						lines = append(lines, scanner.Text())
					}
					if err := scanner.Err(); err != nil {
						return fmt.Errorf("read input: %w", err)
					}
					content = strings.Join(lines, "\n")
				}
			}

			content = strings.TrimSpace(content)
			if content == "" {
				return fmt.Errorf("constitution content cannot be empty")
			}

			// Default version
			if version == "" {
				version = "1.0.0"
			}

			// Save
			if err := backend.SaveConstitution(content, version); err != nil {
				return fmt.Errorf("save constitution: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Constitution saved (version %s, %d bytes)\n", version, len(content))
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Read constitution from file")
	cmd.Flags().StringVarP(&version, "version", "V", "", "Version string (default: 1.0.0)")

	return cmd
}

// newConstitutionShowCmd creates the 'constitution show' subcommand.
func newConstitutionShowCmd() *cobra.Command {
	var showMeta bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display the current constitution",
		Long: `Display the current project constitution.

Use --meta to see version and hash information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := storage.NewBackend(".", &config.StorageConfig{})
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer backend.Close()

			content, version, err := backend.LoadConstitution()
			if err != nil {
				return fmt.Errorf("load constitution: %w", err)
			}

			out := cmd.OutOrStdout()
			if showMeta {
				fmt.Fprintf(out, "# Constitution (version: %s)\n\n", version)
			}
			fmt.Fprintln(out, content)
			return nil
		},
	}

	cmd.Flags().BoolVar(&showMeta, "meta", false, "Show version and metadata")

	return cmd
}

// newConstitutionDeleteCmd creates the 'constitution delete' subcommand.
func newConstitutionDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove the project constitution",
		Long:  `Remove the project constitution. Use --force to skip confirmation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := storage.NewBackend(".", &config.StorageConfig{})
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer backend.Close()

			exists, err := backend.ConstitutionExists()
			if err != nil {
				return fmt.Errorf("check constitution: %w", err)
			}
			if !exists {
				fmt.Fprintln(cmd.OutOrStdout(), "No constitution configured")
				return nil
			}

			if !force {
				fmt.Fprint(cmd.ErrOrStderr(), "Delete constitution? [y/N] ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
					return nil
				}
			}

			if err := backend.DeleteConstitution(); err != nil {
				return fmt.Errorf("delete constitution: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Constitution deleted")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")

	return cmd
}
