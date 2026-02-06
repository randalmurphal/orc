package retrieve

import (
	"context"
	"testing"
)

// =============================================================================
// SC-10: Each preset builds correct stage sequence and weight configuration
// =============================================================================

func TestPreset_Standard_StageSequence(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  &mockGraphQuerier{},
		Embedder:    &mockEmbedder{},
		Reranker:    &mockReranker{},
		Collection:  "documents",
	}

	p, err := StandardPreset(deps)
	if err != nil {
		t.Fatalf("StandardPreset: %v", err)
	}

	stages := p.StageNames()
	expected := []string{"semantic", "hydrate", "graph_expansion", "temporal_decay", "pagerank", "rerank"}
	if len(stages) != len(expected) {
		t.Fatalf("Standard stages = %v, want %v", stages, expected)
	}
	for i, want := range expected {
		if stages[i] != want {
			t.Errorf("stage[%d] = %q, want %q", i, stages[i], want)
		}
	}
}

func TestPreset_Standard_WithoutReranker_SkipsRerank(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  &mockGraphQuerier{},
		Embedder:    &mockEmbedder{},
		Reranker:    nil, // no reranker
		Collection:  "documents",
	}

	p, err := StandardPreset(deps)
	if err != nil {
		t.Fatalf("StandardPreset: %v", err)
	}

	stages := p.StageNames()
	for _, name := range stages {
		if name == "rerank" {
			t.Error("Standard preset without reranker should skip rerank stage")
		}
	}
}

func TestPreset_Fast_StageSequence(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	p, err := FastPreset(deps)
	if err != nil {
		t.Fatalf("FastPreset: %v", err)
	}

	stages := p.StageNames()
	expected := []string{"semantic", "hydrate"}
	if len(stages) != len(expected) {
		t.Fatalf("Fast stages = %v, want %v", stages, expected)
	}
	for i, want := range expected {
		if stages[i] != want {
			t.Errorf("stage[%d] = %q, want %q", i, stages[i], want)
		}
	}
}

func TestPreset_Deep_StageSequence(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  &mockGraphQuerier{},
		Embedder:    &mockEmbedder{},
		Reranker:    &mockReranker{},
		Collection:  "documents",
	}

	p, err := DeepPreset(deps)
	if err != nil {
		t.Fatalf("DeepPreset: %v", err)
	}

	// Deep should have same stages as Standard but with higher limits
	stages := p.StageNames()
	expected := []string{"semantic", "hydrate", "graph_expansion", "temporal_decay", "pagerank", "rerank"}
	if len(stages) != len(expected) {
		t.Fatalf("Deep stages = %v, want %v", stages, expected)
	}
	for i, want := range expected {
		if stages[i] != want {
			t.Errorf("stage[%d] = %q, want %q", i, stages[i], want)
		}
	}
}

func TestPreset_GraphFirst_StageSequence(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  &mockGraphQuerier{},
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	p, err := GraphFirstPreset(deps)
	if err != nil {
		t.Fatalf("GraphFirstPreset: %v", err)
	}

	stages := p.StageNames()
	expected := []string{"semantic", "hydrate", "graph_expansion", "pagerank"}
	if len(stages) != len(expected) {
		t.Fatalf("GraphFirst stages = %v, want %v", stages, expected)
	}
	for i, want := range expected {
		if stages[i] != want {
			t.Errorf("stage[%d] = %q, want %q", i, stages[i], want)
		}
	}
}

func TestPreset_Recency_StageSequence(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	p, err := RecencyPreset(deps)
	if err != nil {
		t.Fatalf("RecencyPreset: %v", err)
	}

	stages := p.StageNames()
	expected := []string{"semantic", "hydrate", "temporal_decay"}
	if len(stages) != len(expected) {
		t.Fatalf("Recency stages = %v, want %v", stages, expected)
	}
	for i, want := range expected {
		if stages[i] != want {
			t.Errorf("stage[%d] = %q, want %q", i, stages[i], want)
		}
	}
}

func TestPreset_NilVectorStoreReturnsError(t *testing.T) {
	deps := PresetDeps{
		VectorStore: nil, // required
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	_, err := StandardPreset(deps)
	if err == nil {
		t.Fatal("preset with nil VectorStore should return error")
	}

	_, err = FastPreset(deps)
	if err == nil {
		t.Fatal("preset with nil VectorStore should return error")
	}
}

func TestPreset_NilEmbedderReturnsError(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		Embedder:    nil, // required
		Collection:  "documents",
	}

	_, err := StandardPreset(deps)
	if err == nil {
		t.Fatal("preset with nil Embedder should return error")
	}
}

func TestPreset_NilGraphStoreReturnsError_WhenRequired(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  nil, // required for Standard
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	_, err := StandardPreset(deps)
	if err == nil {
		t.Fatal("Standard preset with nil GraphStore should return error")
	}

	_, err = GraphFirstPreset(deps)
	if err == nil {
		t.Fatal("GraphFirst preset with nil GraphStore should return error")
	}
}

func TestPreset_Fast_DoesNotRequireGraphStore(t *testing.T) {
	deps := PresetDeps{
		VectorStore: &mockVectorSearcher{},
		GraphStore:  nil, // not required for Fast
		Embedder:    &mockEmbedder{},
		Collection:  "documents",
	}

	_, err := FastPreset(deps)
	if err != nil {
		t.Fatalf("Fast preset should not require GraphStore: %v", err)
	}
}

// =============================================================================
// SC-11: Token budget truncates ranked results at MaxTokens boundary
// =============================================================================

func TestTokenBudget_TruncatesAtBoundary(t *testing.T) {
	// Create documents with known token sizes (~4 chars per token)
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-1", Content: makeContent(1000), Tokens: 250}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "doc-2", Content: makeContent(1000), Tokens: 250}, Signals: map[string]float64{"sem": 0.8}},
		{Document: Document{ID: "doc-3", Content: makeContent(1000), Tokens: 250}, Signals: map[string]float64{"sem": 0.7}},
		{Document: Document{ID: "doc-4", Content: makeContent(1000), Tokens: 250}, Signals: map[string]float64{"sem": 0.6}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMaxTokens(600). // room for ~2.4 docs
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Should include only docs that fit in 600 tokens
	if len(result.Documents) > 2 {
		t.Errorf("expected at most 2 documents within 600 token budget, got %d", len(result.Documents))
	}
	if result.TokensUsed > 600 {
		t.Errorf("total tokens %d exceeds budget 600", result.TokensUsed)
	}
}

func TestTokenBudget_FirstResultAlwaysIncluded(t *testing.T) {
	// First result is huge but should still be included
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "big", Content: makeContent(40000), Tokens: 10000}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "small", Content: makeContent(100), Tokens: 25}, Signals: map[string]float64{"sem": 0.8}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMaxTokens(100). // smaller than first doc
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// First result always included regardless of budget
	if len(result.Documents) == 0 {
		t.Fatal("at least first result must always be included")
	}
	if result.Documents[0].ID != "big" {
		t.Errorf("first result should be 'big', got %s", result.Documents[0].ID)
	}
}

func TestTokenBudget_ZeroMaxTokensUsesDefault(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-1", Content: "short", Tokens: 2}, Signals: map[string]float64{"sem": 0.9}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMaxTokens(0). // should use default (8000)
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Documents) != 1 {
		t.Errorf("expected 1 result with default budget, got %d", len(result.Documents))
	}
}

func TestTokenBudget_NegativeMaxTokensReturnsError(t *testing.T) {
	_, err := NewPipelineBuilder().
		AddStage(&producerStage{}).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMaxTokens(-1).
		Build()

	if err == nil {
		t.Fatal("negative MaxTokens should return validation error")
	}
}

// =============================================================================
// SC-12: SummaryOnly mode returns summaries instead of full content
// =============================================================================

func TestSummaryOnly_ReturnsSummaries(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{
			Document: Document{
				ID:      "doc-1",
				Content: "Very long full content that would use many tokens",
				Summary: "Short summary",
				Tokens:  100,
			},
			Signals: map[string]float64{"sem": 0.9},
		},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithSummaryOnly(true).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Documents) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Documents))
	}

	// Content should be replaced with summary
	if result.Documents[0].Content != "Short summary" {
		t.Errorf("content = %q, want %q (summary)", result.Documents[0].Content, "Short summary")
	}
}

func TestSummaryOnly_FallsBackToTruncatedContent(t *testing.T) {
	longContent := makeContent(4000) // ~1000 tokens
	stage := &producerStage{docs: []ScoredDocument{
		{
			Document: Document{
				ID:      "doc-1",
				Content: longContent,
				Summary: "", // no summary
				Tokens:  1000,
			},
			Signals: map[string]float64{"sem": 0.9},
		},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithSummaryOnly(true).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Without summary, should use truncated content excerpt
	content := result.Documents[0].Content
	if content == longContent {
		t.Error("content should be truncated when no summary available in SummaryOnly mode")
	}
	if content == "" {
		t.Error("content should not be empty (should use truncated excerpt)")
	}
}

func TestSummaryOnly_TokenBudgetUsesSummarySizes(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{
			Document: Document{ID: "doc-1", Content: makeContent(4000), Summary: "Short 1", Tokens: 1000},
			Signals:  map[string]float64{"sem": 0.9},
		},
		{
			Document: Document{ID: "doc-2", Content: makeContent(4000), Summary: "Short 2", Tokens: 1000},
			Signals:  map[string]float64{"sem": 0.8},
		},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithSummaryOnly(true).
		WithMaxTokens(100). // More than enough for summaries, not for full content
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// With summaries, both docs should fit in 100 tokens
	if len(result.Documents) != 2 {
		t.Errorf("expected 2 results (summaries fit in budget), got %d", len(result.Documents))
	}
}

// =============================================================================
// Edge cases from spec
// =============================================================================

func TestPreset_MaxResults_Zero_NoHardCap(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "1", Content: "a", Tokens: 1}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "2", Content: "b", Tokens: 1}, Signals: map[string]float64{"sem": 0.8}},
		{Document: Document{ID: "3", Content: "c", Tokens: 1}, Signals: map[string]float64{"sem": 0.7}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMaxResults(0). // no hard cap
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// All results should be returned (token budget permitting)
	if len(result.Documents) != 3 {
		t.Errorf("expected 3 results with no max cap, got %d", len(result.Documents))
	}
}

// =============================================================================
// Helpers
// =============================================================================

// makeContent creates a string of approximately n characters.
func makeContent(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(i%26)
	}
	return string(b)
}
