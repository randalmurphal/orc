package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

// =============================================================================
// SC-13: Service.Query() returns ranked results via pipeline
// =============================================================================

func TestServiceQuery_ReturnsRankedResults(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		documents: map[string]retrieve.Document{
			"doc-1": {
				ID:       "doc-1",
				Content:  "Gate evaluation logic for quality gates",
				Summary:  "Gate evaluation overview",
				FilePath: "internal/gate/gate.go",
				Tokens:   50,
			},
			"doc-2": {
				ID:       "doc-2",
				Content:  "Executor runs phases sequentially",
				Summary:  "Executor overview",
				FilePath: "internal/executor/executor.go",
				Tokens:   40,
			},
		},
		vectorResults: []mockVectorResult{
			{id: "doc-1", score: 0.95},
			{id: "doc-2", score: 0.80},
		},
		embedResult: []float32{0.1, 0.2, 0.3},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "how does gate evaluation work?", retrieve.QueryOpts{
		Preset:    "fast",
		MaxTokens: 4000,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("Query should return documents")
	}

	// Results should be scored and sorted descending
	for i := 1; i < len(result.Documents); i++ {
		if result.Documents[i].FinalScore > result.Documents[i-1].FinalScore {
			t.Errorf("results not sorted descending: doc[%d].score=%f > doc[%d].score=%f",
				i, result.Documents[i].FinalScore, i-1, result.Documents[i-1].FinalScore)
		}
	}

	// Each document should have signals
	for _, doc := range result.Documents {
		if len(doc.Signals) == 0 {
			t.Errorf("document %s should have signals", doc.ID)
		}
	}
}

func TestServiceQuery_RespectsPresetSelection(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{{id: "doc-1", score: 0.9}},
		documents: map[string]retrieve.Document{
			"doc-1": {ID: "doc-1", Content: "content", Tokens: 10},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	// Fast preset should work without graph store
	_, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{
		Preset: "fast",
	})
	if err != nil {
		t.Fatalf("Query with fast preset: %v", err)
	}
}

func TestServiceQuery_RespectsMaxTokens(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{
			{id: "doc-1", score: 0.9},
			{id: "doc-2", score: 0.8},
		},
		documents: map[string]retrieve.Document{
			"doc-1": {ID: "doc-1", Content: makeContent(2000), Tokens: 500},
			"doc-2": {ID: "doc-2", Content: makeContent(2000), Tokens: 500},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{
		Preset:    "fast",
		MaxTokens: 600,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if result.TokensUsed > 600 {
		t.Errorf("tokens used %d exceeds max 600", result.TokensUsed)
	}
}

func TestServiceQuery_RespectsMinScore(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{
			{id: "doc-1", score: 0.9},
			{id: "doc-2", score: 0.1},
		},
		documents: map[string]retrieve.Document{
			"doc-1": {ID: "doc-1", Content: "relevant", Tokens: 10},
			"doc-2": {ID: "doc-2", Content: "irrelevant", Tokens: 10},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{
		Preset:   "fast",
		MinScore: 0.5,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	for _, doc := range result.Documents {
		if doc.FinalScore < 0.5 {
			t.Errorf("document %s score %f below MinScore 0.5", doc.ID, doc.FinalScore)
		}
	}
}

func TestServiceQuery_RespectsSummaryOnly(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{{id: "doc-1", score: 0.9}},
		documents: map[string]retrieve.Document{
			"doc-1": {
				ID:      "doc-1",
				Content: "Very long full content",
				Summary: "Short summary",
				Tokens:  100,
			},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{
		Preset:      "fast",
		SummaryOnly: true,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("should return results")
	}
	if result.Documents[0].Content != "Short summary" {
		t.Errorf("content = %q, want %q (summary)", result.Documents[0].Content, "Short summary")
	}
}

func TestServiceQuery_EmptyQueryReturnsError(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	_, err := svc.Query(context.Background(), "", retrieve.QueryOpts{Preset: "fast"})
	if err == nil {
		t.Fatal("Query with empty query should return error")
	}
}

// =============================================================================
// SC-14: Service.Query() returns empty results when knowledge unavailable
// =============================================================================

func TestServiceQuery_Unavailable_ReturnsEmpty(t *testing.T) {
	// Disabled service
	svc := NewService(ServiceConfig{Enabled: false})

	result, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{
		Preset: "fast",
	})
	if err != nil {
		t.Fatalf("Query should not return error when unavailable, got: %v", err)
	}

	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents when unavailable, got %d", len(result.Documents))
	}
}

func TestServiceQuery_UnhealthyComponents_ReturnsEmpty(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: false, // unhealthy
		redisHealthy:  true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{
		Preset: "fast",
	})
	if err != nil {
		t.Fatalf("Query should not return error when unhealthy, got: %v", err)
	}

	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents when unhealthy, got %d", len(result.Documents))
	}
}

func TestServiceQuery_NilComponents_ReturnsEmpty(t *testing.T) {
	svc := NewService(ServiceConfig{Enabled: true}) // no components

	result, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{})
	if err != nil {
		t.Fatalf("Query should not error with nil components, got: %v", err)
	}

	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents with nil components, got %d", len(result.Documents))
	}
}

// =============================================================================
// SC-15: Service.QueryForTask() returns structured TaskContext
// =============================================================================

func TestServiceQueryForTask_ReturnsTaskContext(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{
			{id: "doc-1", score: 0.9},
			{id: "doc-2", score: 0.8},
		},
		documents: map[string]retrieve.Document{
			"doc-1": {
				ID:       "doc-1",
				Content:  "Authentication pattern using JWT",
				Summary:  "JWT auth pattern",
				FilePath: "internal/auth/jwt.go",
				Tokens:   30,
				Metadata: map[string]interface{}{"type": "pattern"},
			},
			"doc-2": {
				ID:       "doc-2",
				Content:  "Warning: race condition in session handling",
				Summary:  "Session race condition",
				FilePath: "internal/session/session.go",
				Tokens:   30,
				Metadata: map[string]interface{}{"type": "warning"},
			},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	tc, err := svc.QueryForTask(context.Background(), "Implement user authentication with JWT")
	if err != nil {
		t.Fatalf("QueryForTask: %v", err)
	}

	// TaskContext should have populated categories
	if tc == nil {
		t.Fatal("TaskContext should not be nil")
	}

	// At least some categories should be populated
	hasContent := len(tc.FileHistory) > 0 ||
		len(tc.RelatedWork) > 0 ||
		len(tc.Decisions) > 0 ||
		len(tc.Patterns) > 0 ||
		len(tc.Warnings) > 0

	if !hasContent {
		t.Error("TaskContext should have content in at least one category")
	}
}

func TestServiceQueryForTask_Unavailable_ReturnsEmptyContext(t *testing.T) {
	svc := NewService(ServiceConfig{Enabled: false})

	tc, err := svc.QueryForTask(context.Background(), "test task")
	if err != nil {
		t.Fatalf("QueryForTask should not error when unavailable, got: %v", err)
	}

	if tc == nil {
		t.Fatal("should return empty TaskContext, not nil")
	}

	if len(tc.FileHistory) != 0 || len(tc.RelatedWork) != 0 ||
		len(tc.Decisions) != 0 || len(tc.Patterns) != 0 || len(tc.Warnings) != 0 {
		t.Error("all TaskContext fields should be empty when unavailable")
	}
}

func TestServiceQueryForTask_EmptyDescription_ReturnsEmptyContext(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	tc, err := svc.QueryForTask(context.Background(), "")
	if err != nil {
		t.Fatalf("QueryForTask: %v", err)
	}

	if tc == nil {
		t.Fatal("should return empty TaskContext, not nil")
	}

	if len(tc.FileHistory) != 0 || len(tc.RelatedWork) != 0 ||
		len(tc.Decisions) != 0 || len(tc.Patterns) != 0 || len(tc.Warnings) != 0 {
		t.Error("all TaskContext fields should be empty for empty description")
	}
}

func TestServiceQueryForTask_UsesSummaryOnly(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		embedResult:   []float32{0.1, 0.2},
		vectorResults: []mockVectorResult{{id: "doc-1", score: 0.9}},
		documents: map[string]retrieve.Document{
			"doc-1": {
				ID:      "doc-1",
				Content: "Very long detailed content about auth patterns",
				Summary: "Auth pattern summary",
				Tokens:  100,
			},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	tc, err := svc.QueryForTask(context.Background(), "Implement auth")
	if err != nil {
		t.Fatalf("QueryForTask: %v", err)
	}

	// QueryForTask uses summary-only mode by default
	// Check any populated field uses summaries
	allDocs := append(tc.FileHistory, tc.RelatedWork...)
	allDocs = append(allDocs, tc.Decisions...)
	allDocs = append(allDocs, tc.Patterns...)
	allDocs = append(allDocs, tc.Warnings...)

	for _, doc := range allDocs {
		if doc.Content == "Very long detailed content about auth patterns" {
			t.Error("QueryForTask should use summary-only mode, but got full content")
		}
	}
}

// =============================================================================
// SC-17: Service.EnrichBrief() adds graph-derived entries
// =============================================================================

func TestServiceEnrichBrief_AddsGraphEntries(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		patterns: []retrieve.Document{
			{ID: "pattern-1", Content: "Always use context.Context", FilePath: "internal/"},
		},
		hotFiles: []retrieve.Document{
			{ID: "hot-1", Content: "executor.go", FilePath: "internal/executor/executor.go",
				Metadata: map[string]interface{}{"difficulty": 0.8}},
		},
		knownIssues: []retrieve.Document{
			{ID: "issue-1", Content: "Cross-file coupling between gate and executor"},
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		Sections: []brief.Section{
			{
				Category: brief.CategoryDecisions,
				Entries: []brief.Entry{
					{Content: "Use JWT tokens", Source: "INIT-001", Impact: 0.9},
				},
			},
			{
				Category: brief.CategoryRecentFindings,
				Entries: []brief.Entry{
					{Content: "Found memory leak in parser", Source: "TASK-042", Impact: 0.7},
				},
			},
		},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief: %v", err)
	}

	// Original entries must be preserved
	hasDecision := false
	hasFinding := false
	for _, sec := range enriched.Sections {
		for _, entry := range sec.Entries {
			if entry.Content == "Use JWT tokens" {
				hasDecision = true
			}
			if entry.Content == "Found memory leak in parser" {
				hasFinding = true
			}
		}
	}
	if !hasDecision {
		t.Error("original decision entry should be preserved")
	}
	if !hasFinding {
		t.Error("original finding entry should be preserved")
	}

	// Should have new sections from graph enrichment
	hasPatterns := false
	hasHotFiles := false
	hasKnownIssues := false
	for _, sec := range enriched.Sections {
		switch sec.Category {
		case brief.CategoryPatterns:
			if len(sec.Entries) > 0 {
				hasPatterns = true
			}
		case brief.CategoryHotFiles:
			if len(sec.Entries) > 0 {
				hasHotFiles = true
			}
		case brief.CategoryKnownIssues:
			if len(sec.Entries) > 0 {
				hasKnownIssues = true
			}
		}
	}
	if !hasPatterns {
		t.Error("enriched brief should have patterns section from graph")
	}
	if !hasHotFiles {
		t.Error("enriched brief should have hot_files section from graph")
	}
	if !hasKnownIssues {
		t.Error("enriched brief should have known_issues section from graph")
	}
}

func TestServiceEnrichBrief_Unavailable_ReturnsBaseBrief(t *testing.T) {
	svc := NewService(ServiceConfig{Enabled: false})

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		TaskCount:   3,
		Sections: []brief.Section{
			{
				Category: brief.CategoryDecisions,
				Entries:  []brief.Entry{{Content: "original", Source: "INIT-001", Impact: 0.9}},
			},
		},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief should not error when unavailable, got: %v", err)
	}

	// Should return the base brief unchanged
	if len(enriched.Sections) != len(baseBrief.Sections) {
		t.Errorf("should return base brief unchanged, sections: got %d, want %d",
			len(enriched.Sections), len(baseBrief.Sections))
	}
}

func TestServiceEnrichBrief_NilBrief_ReturnsNil(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	enriched, err := svc.EnrichBrief(context.Background(), nil)
	if err != nil {
		t.Fatalf("EnrichBrief with nil brief should not error: %v", err)
	}

	if enriched != nil {
		t.Error("EnrichBrief with nil brief should return nil")
	}
}

func TestServiceEnrichBrief_GraphFailure_ReturnsBaseBrief(t *testing.T) {
	comps := &mockQueryComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		enrichErr:     true, // simulate graph failure during enrichment
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	baseBrief := &brief.Brief{
		GeneratedAt: time.Now(),
		Sections: []brief.Section{
			{Category: brief.CategoryDecisions, Entries: []brief.Entry{{Content: "keep me"}}},
		},
	}

	enriched, err := svc.EnrichBrief(context.Background(), baseBrief)
	if err != nil {
		t.Fatalf("EnrichBrief should return base brief on graph failure, not error: %v", err)
	}

	// Should return base brief unchanged
	if len(enriched.Sections) != 1 {
		t.Errorf("should return base brief on failure, got %d sections", len(enriched.Sections))
	}
}

// =============================================================================
// Test doubles for service query tests
// =============================================================================

type mockVectorResult struct {
	id    string
	score float32
}

// mockQueryComponents extends mockComponents with query support.
type mockQueryComponents struct {
	// Health
	neo4jHealthy  bool
	qdrantHealthy bool
	redisHealthy  bool

	// Query data
	embedResult   []float32
	vectorResults []mockVectorResult
	documents     map[string]retrieve.Document

	// EnrichBrief data
	patterns    []retrieve.Document
	hotFiles    []retrieve.Document
	knownIssues []retrieve.Document
	enrichErr   bool

	callOrder []string
}

func (m *mockQueryComponents) InfraStart(_ context.Context) error {
	m.callOrder = append(m.callOrder, "infra.Start")
	return nil
}

func (m *mockQueryComponents) InfraStop(_ context.Context) error {
	m.callOrder = append(m.callOrder, "infra.Stop")
	return nil
}

func (m *mockQueryComponents) GraphConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "graph.Connect")
	return nil
}

func (m *mockQueryComponents) GraphClose() error {
	m.callOrder = append(m.callOrder, "graph.Close")
	return nil
}

func (m *mockQueryComponents) VectorConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "vector.Connect")
	return nil
}

func (m *mockQueryComponents) VectorClose() error {
	m.callOrder = append(m.callOrder, "vector.Close")
	return nil
}

func (m *mockQueryComponents) CacheConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "cache.Connect")
	return nil
}

func (m *mockQueryComponents) CacheClose() error {
	m.callOrder = append(m.callOrder, "cache.Close")
	return nil
}

func (m *mockQueryComponents) IsHealthy() (neo4j, qdrant, redis bool) {
	return m.neo4jHealthy, m.qdrantHealthy, m.redisHealthy
}

// makeContent creates a string of approximately n characters.
func makeContent(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(i%26)
	}
	return string(b)
}
