package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

func newBriefCmd() *cobra.Command {
	var (
		regenerate bool
		jsonOutput bool
		stats      bool
	)

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Display the auto-generated project brief",
		Long: `Display the project brief — auto-generated context from task history.

The brief summarizes decisions, findings, and patterns accumulated across
completed tasks. It is injected into phase prompts via {{PROJECT_BRIEF}}.

Examples:
  orc brief                # Show current brief
  orc brief --regenerate   # Force regeneration
  orc brief --json         # Output as JSON
  orc brief --stats        # Show metadata only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// Check if project is initialized
			if err := config.RequireInitAt(cwd); err != nil {
				return fmt.Errorf("project not initialized — run 'orc init' first: %w", err)
			}

			dbPath := filepath.Join(cwd, ".orc", "orc.db")
			backend, err := storage.OpenDatabaseBackend(dbPath)
			if err != nil {
				return fmt.Errorf("open project database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			cfg := brief.DefaultConfig()

			// Load config to get brief settings
			if orcCfg, cfgErr := config.LoadFrom(cwd); cfgErr == nil {
				cfg.MaxTokens = orcCfg.Brief.MaxTokens
				cfg.StaleThreshold = orcCfg.Brief.StaleThreshold
			}

			cachePath := filepath.Join(cwd, ".orc", "brief-cache.json")
			cfg.CachePath = cachePath

			gen := brief.NewGenerator(backend, cfg)

			// Handle --regenerate
			if regenerate {
				cache := brief.NewCache(cachePath)
				_ = cache.Invalidate()

				b, err := gen.Generate(context.Background())
				if err != nil {
					return fmt.Errorf("generate brief: %w", err)
				}

				formatted := brief.FormatBrief(b)
				if formatted == "" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Brief regenerated\n\nNo brief data available")
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Brief regenerated\n\n%s", formatted)
				}
				return nil
			}

			b, err := gen.Generate(context.Background())
			if err != nil {
				return fmt.Errorf("generate brief: %w", err)
			}

			// Handle --json
			if jsonOutput {
				data, err := json.MarshalIndent(b, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal brief: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Handle --stats
			if stats {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "task count: %d\ntoken count: %d\ngenerated at: %s\nsections: %d\n",
					b.TaskCount, b.TokenCount, b.GeneratedAt.Format("2006-01-02 15:04:05"), len(b.Sections))
				return nil
			}

			// Default: show formatted brief
			formatted := brief.FormatBrief(b)
			if formatted == "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No brief data available")
				return nil
			}

			_, _ = fmt.Fprint(cmd.OutOrStdout(), formatted)
			return nil
		},
	}

	cmd.Flags().BoolVar(&regenerate, "regenerate", false, "Force cache invalidation and regeneration")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output brief as JSON")
	cmd.Flags().BoolVar(&stats, "stats", false, "Show brief metadata only")

	return cmd
}
