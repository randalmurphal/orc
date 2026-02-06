package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newKnowledgeQueryCmd() *cobra.Command {
	var (
		preset  string
		limit   int
		summary bool
	)

	cmd := &cobra.Command{
		Use:   "query <search-query>",
		Short: "Search the knowledge graph",
		Long: `Search the knowledge graph using semantic search with configurable presets.

Presets control which pipeline stages execute:
  standard  - Full pipeline: semantic + graph + temporal + pagerank + rerank
  fast      - Lightweight: semantic + hydrate only
  deep      - Full pipeline with higher limits
  graph_first - Graph-heavy: semantic + graph + pagerank
  recency   - Recent focus: semantic + temporal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			// In production, this would use the knowledge service.
			// For now, print a message when knowledge is unavailable.
			fmt.Fprintf(cmd.OutOrStdout(), "Knowledge layer is not available. Start it with: orc knowledge start\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Query: %s (preset=%s, limit=%d, summary=%v)\n", query, preset, limit, summary)
			return nil
		},
	}

	cmd.Flags().StringVar(&preset, "preset", "standard", "Search preset (standard, fast, deep, graph_first, recency)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	cmd.Flags().BoolVar(&summary, "summary", false, "Return summaries instead of full content")

	return cmd
}
