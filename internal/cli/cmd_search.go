// Package cli implements the orc command-line interface.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// newSearchCmd creates the search command for full-text search across transcripts.
func newSearchCmd() *cobra.Command {
	var limit int
	var taskID string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search transcripts using full-text search",
		Long: `Search across all task transcripts using full-text search.

Uses FTS5 (SQLite) or ILIKE (PostgreSQL) to find matching content.
Returns up to 50 results by default, sorted by relevance.

Examples:
  orc search "error handling"
  orc search "authentication" --limit 10
  orc search "API" --task TASK-001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			query := args[0]

			// Open project database
			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(projectRoot)
			if err != nil {
				// Provide user-friendly message for common error cases
				if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") ||
					strings.Contains(err.Error(), "database") {
					return errors.New("no transcripts indexed yet - run a task first to enable search")
				}
				return fmt.Errorf("open project database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			// Perform search
			matches, err := pdb.SearchTranscripts(query)
			if err != nil {
				return fmt.Errorf("search transcripts: %w", err)
			}

			if len(matches) == 0 {
				fmt.Printf("No matches found for: %s\n", query)
				return nil
			}

			// Filter by task ID if specified
			if taskID != "" {
				filtered := make([]db.TranscriptMatch, 0)
				for _, m := range matches {
					if m.TaskID == taskID {
						filtered = append(filtered, m)
					}
				}
				matches = filtered
				if len(matches) == 0 {
					fmt.Printf("No matches found for \"%s\" in task %s\n", query, taskID)
					return nil
				}
			}

			// Apply limit
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}

			// Print results
			fmt.Printf("Found %d match(es) for: %s\n\n", len(matches), query)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TASK\tPHASE\tSNIPPET")
			_, _ = fmt.Fprintln(w, "────\t─────\t───────")

			for _, m := range matches {
				// Clean up snippet - remove newlines and truncate
				snippet := cleanSnippet(m.Snippet, 60)
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", m.TaskID, m.Phase, snippet)
			}

			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "Maximum number of results")
	cmd.Flags().StringVarP(&taskID, "task", "t", "", "Filter results to a specific task")

	return cmd
}

// cleanSnippet removes newlines and truncates the snippet for display.
func cleanSnippet(s string, maxLen int) string {
	// Replace newlines with spaces
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			result = append(result, ' ')
		} else {
			result = append(result, s[i])
		}
	}
	s = string(result)

	// Truncate if needed
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
