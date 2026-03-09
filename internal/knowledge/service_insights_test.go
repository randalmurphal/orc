package knowledge

import (
	"context"
	"testing"

	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

func TestServiceInsights_ReturnsEmptyWhenUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewService(ServiceConfig{Enabled: false})
	insights, err := svc.Insights(context.Background())
	if err != nil {
		t.Fatalf("Insights() error: %v", err)
	}

	if len(insights.Patterns) != 0 {
		t.Errorf("Patterns length = %d, want 0", len(insights.Patterns))
	}
	if len(insights.HotFiles) != 0 {
		t.Errorf("HotFiles length = %d, want 0", len(insights.HotFiles))
	}
	if len(insights.ConstitutionUpdates) != 0 {
		t.Errorf("ConstitutionUpdates length = %d, want 0", len(insights.ConstitutionUpdates))
	}
}

func TestServiceInsights_ReturnsEnrichmentData(t *testing.T) {
	t.Parallel()

	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		patterns: []retrieve.Document{
			{ID: "pattern-1", Content: "Shared transaction pattern"},
		},
		hotFiles: []retrieve.Document{
			{ID: "file-1", FilePath: "internal/executor/workflow_executor.go", Content: "hot file"},
		},
		knownIssues: []retrieve.Document{
			{ID: "constitution-1", Content: "Updated merge policy"},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))
	insights, err := svc.Insights(context.Background())
	if err != nil {
		t.Fatalf("Insights() error: %v", err)
	}

	if len(insights.Patterns) != 1 {
		t.Errorf("Patterns length = %d, want 1", len(insights.Patterns))
	}
	if len(insights.HotFiles) != 1 {
		t.Errorf("HotFiles length = %d, want 1", len(insights.HotFiles))
	}
	if len(insights.ConstitutionUpdates) != 1 {
		t.Errorf("ConstitutionUpdates length = %d, want 1", len(insights.ConstitutionUpdates))
	}
}

func TestServiceInsights_PropagatesProviderError(t *testing.T) {
	t.Parallel()

	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		enrichErr:     true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))
	_, err := svc.Insights(context.Background())
	if err == nil {
		t.Fatal("Insights() should return error when provider fails")
	}
}
