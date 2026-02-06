// Tests for TASK-003: Service.EnrichBrief() → brief package integration.
//
// These tests verify that Service.EnrichBrief() produces output compatible with
// the existing brief package's formatting pipeline. The brief package is used by
// the executor (workflow_context.go:populateProjectBrief) to inject context into
// phase prompts via {{PROJECT_BRIEF}}.
//
// Wiring verified:
//   Service.EnrichBrief() → brief.Brief with brief.CategoryXxx constants
//   → brief.FormatBrief() → markdown with correct section headers
//
// Deletion test: If EnrichBrief uses hardcoded category strings instead of
// brief.CategoryXxx constants, FormatBrief() won't produce recognized section
// headers (FormatBrief uses categoryDisplayNames which maps from those constants).
package knowledge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

// =============================================================================
// Service.EnrichBrief() → brief.FormatBrief() integration
//
// EnrichBrief adds graph-derived sections to a brief.Brief. These sections must
// use the brief package's category constants so that FormatBrief() renders them
// with correct display names.
// =============================================================================

// TestEnrichBrief_SectionsFormattableByBriefPackage verifies that the sections
// produced by EnrichBrief() are renderable by brief.FormatBrief(). This is a
// cross-package integration test: knowledge.Service produces brief.Brief output
// that the brief package can format.
//
// If EnrichBrief uses category strings that aren't in brief.categoryDisplayNames,
// FormatBrief() falls back to raw category IDs instead of display names.
func TestEnrichBrief_SectionsFormattableByBriefPackage(t *testing.T) {
	t.Parallel()

	comps := newEnrichmentComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		Sections: []brief.Section{
			{
				Category: brief.CategoryDecisions,
				Entries: []brief.Entry{
					{Content: "Use JWT for auth", Source: "INIT-001", Impact: 0.9},
				},
			},
		},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief: %v", err)
	}

	// The enriched brief should be formattable by brief.FormatBrief().
	formatted := brief.FormatBrief(enriched)
	if formatted == "" {
		t.Fatal("brief.FormatBrief(enriched) returned empty — enriched brief has no entries")
	}

	// Verify graph-derived sections appear with their display names.
	// brief.FormatBrief uses categoryDisplayNames to map constants to headers.
	expectedHeaders := []string{"Patterns", "Hot Files", "Known Issues"}
	for _, header := range expectedHeaders {
		if !strings.Contains(formatted, "### "+header) {
			t.Errorf("formatted brief should contain '### %s' section header — "+
				"EnrichBrief may not use brief.Category constants", header)
		}
	}

	// Original sections must be preserved.
	if !strings.Contains(formatted, "### Decisions") {
		t.Error("formatted brief should preserve original '### Decisions' section")
	}
	if !strings.Contains(formatted, "JWT") {
		t.Error("formatted brief should preserve original decision content")
	}
}

// TestEnrichBrief_SectionCategoriesAreValidBriefConstants verifies that every
// section added by EnrichBrief uses a category constant from the brief package.
// The brief.FormatBrief() function has a categoryDisplayNames map that maps
// these constants to display names. Unknown categories fall through to raw IDs.
func TestEnrichBrief_SectionCategoriesAreValidBriefConstants(t *testing.T) {
	t.Parallel()

	comps := newEnrichmentComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		Sections:    []brief.Section{},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief: %v", err)
	}

	// Valid brief category constants.
	validCategories := map[string]bool{
		brief.CategoryDecisions:      true,
		brief.CategoryRecentFindings: true,
		brief.CategoryHotFiles:       true,
		brief.CategoryPatterns:       true,
		brief.CategoryKnownIssues:    true,
	}

	for _, sec := range enriched.Sections {
		if !validCategories[sec.Category] {
			t.Errorf("enriched section has category %q which is not a valid brief.Category constant — "+
				"FormatBrief() won't render it with a display name", sec.Category)
		}
	}
}

// TestEnrichBrief_GraphSectionsHaveSourceAndContent verifies that graph-derived
// entries have populated Content and Source fields. These are required by
// brief.FormatBrief() which renders entries as "- {Content} [{Source}]".
func TestEnrichBrief_GraphSectionsHaveSourceAndContent(t *testing.T) {
	t.Parallel()

	comps := newEnrichmentComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{GeneratedAt: time.Now()}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief: %v", err)
	}

	graphCategories := map[string]bool{
		brief.CategoryPatterns:    true,
		brief.CategoryHotFiles:    true,
		brief.CategoryKnownIssues: true,
	}

	for _, sec := range enriched.Sections {
		if !graphCategories[sec.Category] {
			continue
		}
		for _, entry := range sec.Entries {
			if entry.Content == "" {
				t.Errorf("section %q entry has empty Content — FormatBrief will render blank line", sec.Category)
			}
			if entry.Source == "" {
				t.Errorf("section %q entry has empty Source — FormatBrief will render empty brackets", sec.Category)
			}
		}
	}
}

// TestEnrichBrief_PreservesAllOriginalSections verifies that EnrichBrief is
// additive: it adds new sections without removing or modifying existing ones.
// The executor's populateProjectBrief() may pass a Generator-produced brief
// with decisions and findings. EnrichBrief must not discard these.
func TestEnrichBrief_PreservesAllOriginalSections(t *testing.T) {
	t.Parallel()

	comps := newEnrichmentComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		TaskCount:   10,
		Sections: []brief.Section{
			{
				Category: brief.CategoryDecisions,
				Entries: []brief.Entry{
					{Content: "Use bcrypt for passwords", Source: "INIT-002", Impact: 0.8},
					{Content: "REST over gRPC for external API", Source: "INIT-003", Impact: 0.7},
				},
			},
			{
				Category: brief.CategoryRecentFindings,
				Entries: []brief.Entry{
					{Content: "SQL injection in login handler", Source: "TASK-042", Impact: 0.9},
				},
			},
		},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief: %v", err)
	}

	// Count original entries — they must all be present in the enriched brief.
	originalEntries := map[string]bool{
		"Use bcrypt for passwords":            false,
		"REST over gRPC for external API":     false,
		"SQL injection in login handler":      false,
	}

	for _, sec := range enriched.Sections {
		for _, entry := range sec.Entries {
			if _, ok := originalEntries[entry.Content]; ok {
				originalEntries[entry.Content] = true
			}
		}
	}

	for content, found := range originalEntries {
		if !found {
			t.Errorf("original entry %q was not preserved after enrichment", content)
		}
	}

	// Enriched brief should have MORE sections than the base.
	if len(enriched.Sections) <= len(baseBrief.Sections) {
		t.Errorf("enriched brief should have more sections (%d) than base (%d) — enrichment may not be working",
			len(enriched.Sections), len(baseBrief.Sections))
	}
}

// =============================================================================
// Test double: enrichmentComponents
//
// Provides graph-derived data for EnrichBrief: patterns, hot files, known
// issues. Implements Components + enough to support EnrichBrief's graph queries.
// =============================================================================

type enrichmentComponents struct {
	patterns    []retrieve.Document
	hotFiles    []retrieve.Document
	knownIssues []retrieve.Document
}

func newEnrichmentComponents() *enrichmentComponents {
	return &enrichmentComponents{
		patterns: []retrieve.Document{
			{
				ID:       "pattern-ctx",
				Content:  "Always pass context.Context as first parameter",
				FilePath: "internal/",
			},
			{
				ID:       "pattern-err",
				Content:  "Wrap errors with fmt.Errorf and %w",
				FilePath: "internal/",
			},
		},
		hotFiles: []retrieve.Document{
			{
				ID:       "hot-executor",
				Content:  "executor.go",
				FilePath: "internal/executor/executor.go",
				Metadata: map[string]interface{}{"difficulty": 0.8},
			},
		},
		knownIssues: []retrieve.Document{
			{
				ID:      "issue-coupling",
				Content: "Tight coupling between gate and executor packages",
			},
		},
	}
}

// --- Components interface ---

func (e *enrichmentComponents) InfraStart(_ context.Context) error        { return nil }
func (e *enrichmentComponents) InfraStop(_ context.Context) error         { return nil }
func (e *enrichmentComponents) GraphConnect(_ context.Context) error      { return nil }
func (e *enrichmentComponents) GraphClose() error                         { return nil }
func (e *enrichmentComponents) VectorConnect(_ context.Context) error     { return nil }
func (e *enrichmentComponents) VectorClose() error                        { return nil }
func (e *enrichmentComponents) CacheConnect(_ context.Context) error      { return nil }
func (e *enrichmentComponents) CacheClose() error                         { return nil }
func (e *enrichmentComponents) IsHealthy() (neo4j, qdrant, redis bool)   { return true, true, true }

// --- Enrichment data access ---
// The Service will need to extract these from the components (via type assertion
// or additional interface). These methods provide the graph-derived data.

func (e *enrichmentComponents) GetPatterns(_ context.Context) ([]retrieve.Document, error) {
	return e.patterns, nil
}

func (e *enrichmentComponents) GetHotFiles(_ context.Context) ([]retrieve.Document, error) {
	return e.hotFiles, nil
}

func (e *enrichmentComponents) GetKnownIssues(_ context.Context) ([]retrieve.Document, error) {
	return e.knownIssues, nil
}
