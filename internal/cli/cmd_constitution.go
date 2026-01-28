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
	"github.com/randalmurphal/orc/templates"
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
  template    Output a structured constitution template

Examples:
  # Generate a template
  orc constitution template > principles.md

  # Set from file
  orc constitution set --file principles.md

  # Set interactively (Ctrl+D to finish)
  orc constitution set

  # Set from stdin
  echo "# Rules" | orc constitution set

  # Show current constitution (stored at .orc/CONSTITUTION.md)
  orc constitution show

  # Remove constitution
  orc constitution delete`,
	}

	cmd.AddCommand(newConstitutionSetCmd())
	cmd.AddCommand(newConstitutionShowCmd())
	cmd.AddCommand(newConstitutionDeleteCmd())
	cmd.AddCommand(newConstitutionTemplateCmd())

	return cmd
}

// newConstitutionSetCmd creates the 'constitution set' subcommand.
func newConstitutionSetCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the project constitution",
		Long: `Set the project constitution from a file or stdin.

The constitution is stored at .orc/CONSTITUTION.md and is git-tracked.
It should contain markdown-formatted principles and rules that apply
to all task execution. Common sections include:

  - Coding standards
  - Architectural invariants
  - Testing requirements
  - Error handling patterns

Examples:
  # From file
  orc constitution set --file principles.md

  # From stdin (pipe)
  cat principles.md | orc constitution set

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
			defer func() { _ = backend.Close() }()

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
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Enter constitution content (Ctrl+D when done):")
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

			// Save to .orc/CONSTITUTION.md
			if err := backend.SaveConstitution(content); err != nil {
				return fmt.Errorf("save constitution: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Constitution saved to .orc/CONSTITUTION.md (%d bytes)\n", len(content))
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Read constitution from file")

	return cmd
}

// newConstitutionShowCmd creates the 'constitution show' subcommand.
func newConstitutionShowCmd() *cobra.Command {
	var showMeta bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display the current constitution",
		Long: `Display the current project constitution.

Use --meta to see file path information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := storage.NewBackend(".", &config.StorageConfig{})
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer func() { _ = backend.Close() }()

			content, path, err := backend.LoadConstitution()
			if err != nil {
				return fmt.Errorf("load constitution: %w", err)
			}

			out := cmd.OutOrStdout()
			if showMeta {
				_, _ = fmt.Fprintf(out, "# Constitution (path: %s)\n\n", path)
			}
			_, _ = fmt.Fprintln(out, content)
			return nil
		},
	}

	cmd.Flags().BoolVar(&showMeta, "meta", false, "Show file path and metadata")

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
			defer func() { _ = backend.Close() }()

			exists, err := backend.ConstitutionExists()
			if err != nil {
				return fmt.Errorf("check constitution: %w", err)
			}
			if !exists {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No constitution configured")
				return nil
			}

			if !force {
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Delete constitution? [y/N] ")
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
					return nil
				}
			}

			if err := backend.DeleteConstitution(); err != nil {
				return fmt.Errorf("delete constitution: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Constitution deleted")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")

	return cmd
}

// newConstitutionTemplateCmd creates the 'constitution template' subcommand.
func newConstitutionTemplateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "template",
		Short: "Output a structured constitution template",
		Long: `Output a structured constitution template to stdout.

The template provides a starting point for your project's constitution,
with sections for invariants (absolute rules), defaults (flexible guidelines),
and architectural decisions.

Example:
  # Generate template and save to file
  orc constitution template > CONSTITUTION.md

  # Edit the file, then set as constitution
  orc constitution set --file CONSTITUTION.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := templates.Prompts.ReadFile("prompts/constitution_template.md")
			if err != nil {
				return fmt.Errorf("read constitution template: %w", err)
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(content))
			return nil
		},
	}
}
