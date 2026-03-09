package knowledge

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

// Insights bundles enrichment data for exploration UIs.
type Insights struct {
	Patterns            []retrieve.Document
	HotFiles            []retrieve.Document
	ConstitutionUpdates []retrieve.Document
}

// Insights returns enrichment data from the underlying provider.
func (s *Service) Insights(ctx context.Context) (*Insights, error) {
	insights := &Insights{}
	if !s.IsAvailable() {
		return insights, nil
	}

	enricher, ok := s.comps.(retrieve.EnrichmentProvider)
	if !ok {
		return insights, nil
	}

	patterns, err := enricher.GetPatterns(ctx)
	if err != nil {
		return nil, fmt.Errorf("get patterns: %w", err)
	}

	hotFiles, err := enricher.GetHotFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("get hot files: %w", err)
	}

	knownIssues, err := enricher.GetKnownIssues(ctx)
	if err != nil {
		return nil, fmt.Errorf("get constitution updates: %w", err)
	}

	insights.Patterns = patterns
	insights.HotFiles = hotFiles
	insights.ConstitutionUpdates = knownIssues
	return insights, nil
}
