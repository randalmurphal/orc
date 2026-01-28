// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
)

// newDocsCmd creates the docs command
func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage orc documentation in CLAUDE.md",
		Long: `Manage orc documentation sections in your project's CLAUDE.md file.

This command group provides control over orc-specific sections that can be added
to your CLAUDE.md file. These sections are NOT automatically injected during
'orc init' - you choose whether and when to add them.

Subcommands:
  inject     Add the orc workflow documentation section
  knowledge  Add the knowledge capture section
  status     Check which sections are present

The injected sections are marked with HTML comments (<!-- orc:begin --> etc.)
so they can be identified and updated.`,
	}

	cmd.AddCommand(newDocsInjectCmd())
	cmd.AddCommand(newDocsKnowledgeCmd())
	cmd.AddCommand(newDocsStatusCmd())

	return cmd
}

// newDocsInjectCmd creates the docs inject subcommand
func newDocsInjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inject",
		Short: "Add orc workflow section to CLAUDE.md",
		Long: `Inject the orc workflow documentation section into CLAUDE.md.

This adds a section documenting:
- When to use orc
- Available slash commands
- Key CLI commands

The section is wrapped in <!-- orc:begin --> and <!-- orc:end --> markers.
If the section already exists, it will be updated with the latest content.

Example:
  orc docs inject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			projectRoot, err := config.FindProjectRoot()
			if err != nil {
				return err
			}

			if err := bootstrap.InjectOrcSection(projectRoot); err != nil {
				return fmt.Errorf("inject orc section: %w", err)
			}

			fmt.Println("Added orc workflow section to CLAUDE.md")
			return nil
		},
	}

	return cmd
}

// newDocsKnowledgeCmd creates the docs knowledge subcommand
func newDocsKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Add knowledge capture section to CLAUDE.md",
		Long: `Inject the knowledge capture section into CLAUDE.md.

This adds a section for capturing:
- Patterns learned during development
- Known gotchas and resolutions
- Architectural decisions

The section is wrapped in <!-- orc:knowledge:begin --> and <!-- orc:knowledge:end -->
markers. If the section already exists, it will be preserved.

Example:
  orc docs knowledge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			projectRoot, err := config.FindProjectRoot()
			if err != nil {
				return err
			}

			if err := bootstrap.InjectKnowledgeSection(projectRoot); err != nil {
				return fmt.Errorf("inject knowledge section: %w", err)
			}

			fmt.Println("Added knowledge capture section to CLAUDE.md")
			return nil
		},
	}

	return cmd
}

// newDocsStatusCmd creates the docs status subcommand
func newDocsStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check orc section status in CLAUDE.md",
		Long: `Check which orc-related sections are present in CLAUDE.md.

Example:
  orc docs status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			projectRoot, err := config.FindProjectRoot()
			if err != nil {
				return err
			}

			hasOrc := bootstrap.HasOrcSection(projectRoot)
			hasKnowledge := bootstrap.HasKnowledgeSection(projectRoot)

			fmt.Println("CLAUDE.md sections:")
			if hasOrc {
				fmt.Println("  orc workflow:     present")
			} else {
				fmt.Println("  orc workflow:     not present (run: orc docs inject)")
			}
			if hasKnowledge {
				fmt.Println("  knowledge:        present")
			} else {
				fmt.Println("  knowledge:        not present (run: orc docs knowledge)")
			}

			return nil
		},
	}

	return cmd
}
