// Package artifact provides indexing of task artifacts into the knowledge graph.
package artifact

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/knowledge/index"
	"github.com/randalmurphal/orc/internal/storage"
)

// IndexParams contains all data needed to index a task's artifacts.
type IndexParams struct {
	TaskID            string
	Spec              string
	Findings          []*orcv1.ReviewRoundFindings
	Decisions         []initiative.Decision
	InitiativeID      string
	Retries           []RetryInfo
	ChangedFiles      []string
	ScratchpadEntries []storage.ScratchpadEntry
}

// RetryInfo contains metadata about a retry attempt.
type RetryInfo struct {
	Attempt   int
	Reason    string
	FromPhase string
}

// Indexer orchestrates artifact indexing into the knowledge graph.
type Indexer struct {
	graph index.GraphStorer
}

// NewIndexer creates a new artifact indexer with the given graph store.
func NewIndexer(graph index.GraphStorer) *Indexer {
	return &Indexer{graph: graph}
}

// IndexAll runs all artifact indexers, collecting errors.
// Individual indexer failures don't stop other indexers from running.
func (idx *Indexer) IndexAll(ctx context.Context, params IndexParams) error {
	var errs []error

	if err := idx.IndexSpec(ctx, params.TaskID, params.Spec); err != nil {
		errs = append(errs, fmt.Errorf("spec: %w", err))
	}

	if err := idx.IndexFindings(ctx, params.TaskID, params.Findings); err != nil {
		errs = append(errs, fmt.Errorf("findings: %w", err))
	}

	if err := idx.IndexDecisions(ctx, params.TaskID, params.InitiativeID, params.Decisions, params.ChangedFiles); err != nil {
		errs = append(errs, fmt.Errorf("decisions: %w", err))
	}

	if err := idx.IndexRetries(ctx, params.TaskID, params.Retries); err != nil {
		errs = append(errs, fmt.Errorf("retries: %w", err))
	}

	if err := idx.IndexMetrics(ctx, params.TaskID, params.ChangedFiles, len(params.Retries)); err != nil {
		errs = append(errs, fmt.Errorf("metrics: %w", err))
	}

	if err := idx.IndexScratchpad(ctx, params.TaskID, params.ScratchpadEntries); err != nil {
		errs = append(errs, fmt.Errorf("scratchpad: %w", err))
	}

	return errors.Join(errs...)
}

// filePathRe matches file paths containing at least one directory separator
// and a file extension (e.g., "internal/foo.go", "web/src/App.tsx").
var filePathRe = regexp.MustCompile(`[\w][\w./-]*/[\w][\w./-]*\.\w{1,5}`)

// extractFilePaths finds file paths mentioned in text content.
func extractFilePaths(text string) []string {
	matches := filePathRe.FindAllString(text, -1)
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// isValidContent checks if content is non-empty and valid UTF-8.
func isValidContent(s string) bool {
	return s != "" && utf8.ValidString(s)
}
