package retrieve

import (
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// SC-3: Semantic stage queries vector store and returns candidates
// =============================================================================

func TestSemanticStage_ReturnsCandidatesWithSemanticSignal(t *testing.T) {
	embedder := &mockEmbedder{
		vectors: map[string][]float32{
			"how does gate evaluation work?": {0.1, 0.2, 0.3},
		},
	}
	vectorStore := &mockVectorSearcher{
		results: []vectorSearchResult{
			{id: "doc-1", score: 0.95, payload: map[string]interface{}{"file_path": "gate.go"}},
			{id: "doc-2", score: 0.80, payload: map[string]interface{}{"file_path": "eval.go"}},
		},
	}

	stage := NewSemanticStage(embedder, vectorStore, "documents", 10)

	results, err := stage.Execute(context.Background(), "how does gate evaluation work?", nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(results))
	}

	// Verify semantic signal is set to similarity score
	for _, doc := range results {
		semScore, ok := doc.Signals["semantic"]
		if !ok {
			t.Errorf("document %s missing 'semantic' signal", doc.ID)
			continue
		}
		if semScore < 0 || semScore > 1 {
			t.Errorf("document %s semantic score %f outside [0,1]", doc.ID, semScore)
		}
	}

	// Verify first result has highest score
	if results[0].Signals["semantic"] < results[1].Signals["semantic"] {
		t.Error("results should be ordered by semantic score descending")
	}
}

func TestSemanticStage_EmbeddingFailureReturnsError(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("API rate limit exceeded")}
	vectorStore := &mockVectorSearcher{}

	stage := NewSemanticStage(embedder, vectorStore, "documents", 10)

	_, err := stage.Execute(context.Background(), "test query", nil)
	if err == nil {
		t.Fatal("semantic stage should return error when embedding fails")
	}
}

func TestSemanticStage_VectorSearchFailureReturnsError(t *testing.T) {
	embedder := &mockEmbedder{
		vectors: map[string][]float32{"test query": {0.1, 0.2}},
	}
	vectorStore := &mockVectorSearcher{err: errors.New("connection refused")}

	stage := NewSemanticStage(embedder, vectorStore, "documents", 10)

	_, err := stage.Execute(context.Background(), "test query", nil)
	if err == nil {
		t.Fatal("semantic stage should return error when vector search fails")
	}
}

func TestSemanticStage_NoMatchesReturnsEmpty(t *testing.T) {
	embedder := &mockEmbedder{
		vectors: map[string][]float32{"test query": {0.1, 0.2}},
	}
	vectorStore := &mockVectorSearcher{results: nil}

	stage := NewSemanticStage(embedder, vectorStore, "documents", 10)

	results, err := stage.Execute(context.Background(), "test query", nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for no matches, got %d", len(results))
	}
}

func TestSemanticStage_Name(t *testing.T) {
	stage := NewSemanticStage(nil, nil, "documents", 10)
	if stage.Name() != "semantic" {
		t.Errorf("name = %q, want %q", stage.Name(), "semantic")
	}
}

// =============================================================================
// SC-4: Hydrate stage loads full document content from graph store
// =============================================================================

func TestHydrateStage_LoadsFullContent(t *testing.T) {
	graphStore := &mockGraphQuerier{
		documents: map[string]Document{
			"doc-1": {
				ID:       "doc-1",
				Content:  "Full content of doc-1 about gate evaluation",
				Summary:  "Gate evaluation overview",
				FilePath: "internal/gate/gate.go",
			},
			"doc-2": {
				ID:       "doc-2",
				Content:  "Full content of doc-2 about execution",
				FilePath: "internal/executor/executor.go",
			},
		},
	}

	stage := NewHydrateStage(graphStore)

	// Input: candidates with only IDs (from semantic stage)
	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify content was loaded
	if results[0].Content == "" {
		t.Error("doc-1 should have content loaded")
	}
	if results[0].FilePath != "internal/gate/gate.go" {
		t.Errorf("doc-1 file path = %q, want %q", results[0].FilePath, "internal/gate/gate.go")
	}

	// Verify signals preserved from prior stage
	if results[0].Signals["semantic"] != 0.9 {
		t.Errorf("semantic signal should be preserved, got %f", results[0].Signals["semantic"])
	}
}

func TestHydrateStage_SkipsAlreadyHydratedDocuments(t *testing.T) {
	graphStore := &mockGraphQuerier{
		documents: map[string]Document{
			"doc-1": {ID: "doc-1", Content: "fresh content"},
		},
	}

	stage := NewHydrateStage(graphStore)

	// doc-2 already has content (already hydrated)
	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2", Content: "existing content"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// doc-2 should keep its existing content
	for _, doc := range results {
		if doc.ID == "doc-2" && doc.Content != "existing content" {
			t.Errorf("already-hydrated doc should keep existing content, got %q", doc.Content)
		}
	}

	// Verify doc-1 was fetched
	if graphStore.fetchedIDs == nil || !containsStr(graphStore.fetchedIDs, "doc-1") {
		t.Error("doc-1 should have been fetched from graph")
	}
}

func TestHydrateStage_MissingDocumentsSkipped(t *testing.T) {
	graphStore := &mockGraphQuerier{
		documents: map[string]Document{
			"doc-1": {ID: "doc-1", Content: "content"},
		},
		// doc-2 not in graph (may have been deleted)
	}

	stage := NewHydrateStage(graphStore)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v, should not error for missing documents", err)
	}

	// doc-2 should be skipped (not error)
	if len(results) != 1 {
		t.Errorf("expected 1 result (doc-2 skipped), got %d", len(results))
	}
}

func TestHydrateStage_Name(t *testing.T) {
	stage := NewHydrateStage(nil)
	if stage.Name() != "hydrate" {
		t.Errorf("name = %q, want %q", stage.Name(), "hydrate")
	}
}

// =============================================================================
// SC-5: GraphExpansion stage adds graph-connected documents with decay
// =============================================================================

func TestGraphExpansionStage_AddsRelatedDocuments(t *testing.T) {
	graphStore := &mockGraphQuerier{
		related: map[string][]relatedDoc{
			"doc-1": {
				{doc: Document{ID: "related-1"}, depth: 1},
				{doc: Document{ID: "related-2"}, depth: 2},
			},
		},
	}

	stage := NewGraphExpansionStage(graphStore, 2) // maxDepth = 2

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Should have original + 2 related
	if len(results) != 3 {
		t.Fatalf("expected 3 results (1 original + 2 related), got %d", len(results))
	}

	// Verify graph signal decays by depth
	for _, doc := range results {
		if doc.ID == "related-1" {
			graphSignal := doc.Signals["graph"]
			// depth 1: 1.0 / (1 + 1) = 0.5
			if !approxEqual(graphSignal, 0.5, 0.01) {
				t.Errorf("depth-1 graph signal = %f, want ~0.5", graphSignal)
			}
		}
		if doc.ID == "related-2" {
			graphSignal := doc.Signals["graph"]
			// depth 2: 1.0 / (1 + 2) ≈ 0.333
			if !approxEqual(graphSignal, 1.0/3.0, 0.01) {
				t.Errorf("depth-2 graph signal = %f, want ~0.333", graphSignal)
			}
		}
	}
}

func TestGraphExpansionStage_DeduplicatesExistingCandidates(t *testing.T) {
	graphStore := &mockGraphQuerier{
		related: map[string][]relatedDoc{
			"doc-1": {
				{doc: Document{ID: "doc-2"}, depth: 1}, // doc-2 already exists as candidate
			},
		},
	}

	stage := NewGraphExpansionStage(graphStore, 2)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Count doc-2 occurrences - should be exactly 1
	count := 0
	for _, doc := range results {
		if doc.ID == "doc-2" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("doc-2 should appear once (deduplicated), appeared %d times", count)
	}
}

func TestGraphExpansionStage_GraphQueryFailureReturnsError(t *testing.T) {
	graphStore := &mockGraphQuerier{err: errors.New("graph connection lost")}

	stage := NewGraphExpansionStage(graphStore, 2)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
	}

	_, err := stage.Execute(context.Background(), "test", candidates)
	if err == nil {
		t.Fatal("graph expansion should return error when graph query fails")
	}
}

func TestGraphExpansionStage_OriginalCandidateGetsDepthZeroSignal(t *testing.T) {
	graphStore := &mockGraphQuerier{related: map[string][]relatedDoc{}}

	stage := NewGraphExpansionStage(graphStore, 2)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Original candidate at depth 0 should have graph signal = 1.0
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	graphSignal := results[0].Signals["graph"]
	if !approxEqual(graphSignal, 1.0, 0.01) {
		t.Errorf("depth-0 graph signal = %f, want 1.0", graphSignal)
	}
}

func TestGraphExpansionStage_Name(t *testing.T) {
	stage := NewGraphExpansionStage(nil, 2)
	if stage.Name() != "graph_expansion" {
		t.Errorf("name = %q, want %q", stage.Name(), "graph_expansion")
	}
}

// =============================================================================
// SC-6: TemporalDecay stage scores by recency with exponential half-life
// =============================================================================

func TestTemporalDecayStage_RecentDocumentScoresHigh(t *testing.T) {
	now := time.Now()
	stage := NewTemporalDecayStage(7*24*time.Hour, now) // 7-day half-life

	candidates := []ScoredDocument{
		{
			Document: Document{ID: "today", UpdatedAt: now},
			Signals:  map[string]float64{"semantic": 0.8},
		},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	temporal := results[0].Signals["temporal"]
	// Document from today should score ~1.0
	if temporal < 0.95 {
		t.Errorf("today's document temporal score = %f, want ~1.0", temporal)
	}
}

func TestTemporalDecayStage_HalfLifeDecay(t *testing.T) {
	now := time.Now()
	halfLife := 7 * 24 * time.Hour
	stage := NewTemporalDecayStage(halfLife, now)

	candidates := []ScoredDocument{
		{
			Document: Document{ID: "week-old", UpdatedAt: now.Add(-halfLife)},
			Signals:  map[string]float64{},
		},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	temporal := results[0].Signals["temporal"]
	// 7 days ago with 7-day half-life should score ~0.5
	if !approxEqual(temporal, 0.5, 0.05) {
		t.Errorf("7-day-old document temporal score = %f, want ~0.5", temporal)
	}
}

func TestTemporalDecayStage_TwoHalfLivesDecay(t *testing.T) {
	now := time.Now()
	halfLife := 7 * 24 * time.Hour
	stage := NewTemporalDecayStage(halfLife, now)

	candidates := []ScoredDocument{
		{
			Document: Document{ID: "2weeks-old", UpdatedAt: now.Add(-2 * halfLife)},
			Signals:  map[string]float64{},
		},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	temporal := results[0].Signals["temporal"]
	// 14 days ago with 7-day half-life should score ~0.25
	if !approxEqual(temporal, 0.25, 0.05) {
		t.Errorf("14-day-old document temporal score = %f, want ~0.25", temporal)
	}
}

func TestTemporalDecayStage_DocumentWithoutTimestampGetsDefault(t *testing.T) {
	now := time.Now()
	stage := NewTemporalDecayStage(7*24*time.Hour, now)

	candidates := []ScoredDocument{
		{
			Document: Document{ID: "no-timestamp"}, // zero-value time
			Signals:  map[string]float64{},
		},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	temporal := results[0].Signals["temporal"]
	// Should get a default temporal score (not 0, not error)
	if temporal <= 0 {
		t.Errorf("document without timestamp should get default temporal score > 0, got %f", temporal)
	}
	if temporal > 1.0 {
		t.Errorf("temporal score %f should not exceed 1.0", temporal)
	}
}

func TestTemporalDecayStage_MinimumScoreFloor(t *testing.T) {
	now := time.Now()
	stage := NewTemporalDecayStage(7*24*time.Hour, now)

	// Very old document (1 year ago)
	candidates := []ScoredDocument{
		{
			Document: Document{ID: "ancient", UpdatedAt: now.Add(-365 * 24 * time.Hour)},
			Signals:  map[string]float64{},
		},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	temporal := results[0].Signals["temporal"]
	// Should have a minimum floor, not score 0
	if temporal <= 0 {
		t.Errorf("very old document should have minimum score floor > 0, got %f", temporal)
	}
}

func TestTemporalDecayStage_Name(t *testing.T) {
	stage := NewTemporalDecayStage(7*24*time.Hour, time.Now())
	if stage.Name() != "temporal_decay" {
		t.Errorf("name = %q, want %q", stage.Name(), "temporal_decay")
	}
}

// =============================================================================
// SC-7: PageRank stage scores by graph centrality
// =============================================================================

func TestPageRankStage_AssignsPageRankSignal(t *testing.T) {
	graphStore := &mockGraphQuerier{
		centrality: map[string]float64{
			"doc-1": 0.85,
			"doc-2": 0.42,
		},
	}

	stage := NewPageRankStage(graphStore)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if results[0].Signals["pagerank"] != 0.85 {
		t.Errorf("doc-1 pagerank = %f, want 0.85", results[0].Signals["pagerank"])
	}
	if results[1].Signals["pagerank"] != 0.42 {
		t.Errorf("doc-2 pagerank = %f, want 0.42", results[1].Signals["pagerank"])
	}
}

func TestPageRankStage_MissingDocumentsGetDefault(t *testing.T) {
	graphStore := &mockGraphQuerier{
		centrality: map[string]float64{
			"doc-1": 0.85,
			// doc-2 not in graph
		},
	}

	stage := NewPageRankStage(graphStore)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// doc-2 should get a default centrality score
	pr := results[1].Signals["pagerank"]
	if pr <= 0 {
		t.Errorf("doc-2 should get default pagerank > 0, got %f", pr)
	}
}

func TestPageRankStage_GraphQueryFailureContinuesWithDefaults(t *testing.T) {
	// PageRank is a soft signal - graph failure should NOT propagate as error
	graphStore := &mockGraphQuerier{err: errors.New("graph unavailable")}

	stage := NewPageRankStage(graphStore)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("PageRank should not return error on graph failure (soft signal), got: %v", err)
	}

	// Should still have results with default scores
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	pr := results[0].Signals["pagerank"]
	if pr <= 0 {
		t.Errorf("should assign default pagerank on graph failure, got %f", pr)
	}
}

func TestPageRankStage_Name(t *testing.T) {
	stage := NewPageRankStage(nil)
	if stage.Name() != "pagerank" {
		t.Errorf("name = %q, want %q", stage.Name(), "pagerank")
	}
}

// =============================================================================
// SC-8: Rerank stage reorders top-K candidates using Reranker
// =============================================================================

func TestRerankStage_ReordersByRelevance(t *testing.T) {
	reranker := &mockReranker{
		scores: map[string]float64{
			"doc-1": 0.3, // was high by semantic, but low by reranker
			"doc-2": 0.9, // was low by semantic, but high by reranker
			"doc-3": 0.6,
		},
	}

	stage := NewRerankStage(reranker, 10)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1", Content: "content 1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2", Content: "content 2"}, Signals: map[string]float64{"semantic": 0.5}},
		{Document: Document{ID: "doc-3", Content: "content 3"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test query", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify rerank signal is set
	for _, doc := range results {
		if _, ok := doc.Signals["rerank"]; !ok {
			t.Errorf("document %s missing 'rerank' signal", doc.ID)
		}
	}

	// doc-2 should have highest rerank signal
	for _, doc := range results {
		if doc.ID == "doc-2" {
			if doc.Signals["rerank"] != 0.9 {
				t.Errorf("doc-2 rerank = %f, want 0.9", doc.Signals["rerank"])
			}
		}
	}
}

func TestRerankStage_OnlyReranksTopK(t *testing.T) {
	reranker := &mockReranker{
		scores: map[string]float64{
			"doc-1": 0.9,
			"doc-2": 0.8,
		},
	}

	stage := NewRerankStage(reranker, 2) // only rerank top-2

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1", Content: "c1"}, Signals: map[string]float64{"semantic": 0.9}},
		{Document: Document{ID: "doc-2", Content: "c2"}, Signals: map[string]float64{"semantic": 0.8}},
		{Document: Document{ID: "doc-3", Content: "c3"}, Signals: map[string]float64{"semantic": 0.7}},
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// doc-3 (beyond top-K) should not have rerank signal
	for _, doc := range results {
		if doc.ID == "doc-3" {
			if _, ok := doc.Signals["rerank"]; ok {
				t.Error("doc-3 (beyond top-K) should not have rerank signal")
			}
		}
	}

	// Reranker should have only been called with top-2
	if reranker.calledWithN != 2 {
		t.Errorf("reranker should be called with %d docs, got %d", 2, reranker.calledWithN)
	}
}

func TestRerankStage_SkipsDocumentsWithoutContent(t *testing.T) {
	reranker := &mockReranker{
		scores: map[string]float64{"doc-2": 0.9},
	}

	stage := NewRerankStage(reranker, 10)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},             // no content
		{Document: Document{ID: "doc-2", Content: "content"}, Signals: map[string]float64{"semantic": 0.8}}, // has content
	}

	results, err := stage.Execute(context.Background(), "test", candidates)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// doc-1 (no content) should not have rerank signal
	for _, doc := range results {
		if doc.ID == "doc-1" {
			if _, ok := doc.Signals["rerank"]; ok {
				t.Error("doc-1 (no content) should not have rerank signal")
			}
		}
	}
}

func TestRerankStage_RerankerFailureReturnsError(t *testing.T) {
	reranker := &mockReranker{err: errors.New("reranker service unavailable")}

	stage := NewRerankStage(reranker, 10)

	candidates := []ScoredDocument{
		{Document: Document{ID: "doc-1", Content: "content"}, Signals: map[string]float64{"semantic": 0.9}},
	}

	_, err := stage.Execute(context.Background(), "test", candidates)
	if err == nil {
		t.Fatal("rerank stage should return error when reranker fails")
	}
}

func TestRerankStage_Name(t *testing.T) {
	stage := NewRerankStage(nil, 10)
	if stage.Name() != "rerank" {
		t.Errorf("name = %q, want %q", stage.Name(), "rerank")
	}
}

// =============================================================================
// Edge cases: Deduplication
// =============================================================================

func TestDeduplication_MergesSignals(t *testing.T) {
	// When the same document appears from multiple stages, signals should merge
	stageA := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"semantic": 0.9}},
	}}
	stageB := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"graph": 0.7}},
		{Document: Document{ID: "doc-2"}, Signals: map[string]float64{"graph": 0.5}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stageA).
		AddStage(stageB).
		WithScorer(NewWeightedScorer(map[string]float64{"semantic": 0.5, "graph": 0.5})).
		WithDeduplication(true).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// doc-1 should appear once with both signals
	doc1Count := 0
	for _, doc := range result.Documents {
		if doc.ID == "doc-1" {
			doc1Count++
			if doc.Signals["semantic"] != 0.9 {
				t.Errorf("doc-1 semantic signal = %f, want 0.9", doc.Signals["semantic"])
			}
			if doc.Signals["graph"] != 0.7 {
				t.Errorf("doc-1 graph signal = %f, want 0.7", doc.Signals["graph"])
			}
		}
	}
	if doc1Count != 1 {
		t.Errorf("doc-1 should appear once (deduplicated), appeared %d times", doc1Count)
	}
}

// =============================================================================
// Test doubles for stages
// =============================================================================

// vectorSearchResult represents a mock vector search result.
type vectorSearchResult struct {
	id      string
	score   float32
	payload map[string]interface{}
}

// mockEmbedder implements the Embedder interface for testing.
type mockEmbedder struct {
	vectors map[string][]float32
	err     error
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result [][]float32
	for _, text := range texts {
		if vec, ok := m.vectors[text]; ok {
			result = append(result, vec)
		} else {
			// Return a default vector for unknown texts
			result = append(result, []float32{0.0, 0.0, 0.0})
		}
	}
	return result, nil
}

func (m *mockEmbedder) Type() string { return "mock" }

// mockVectorSearcher provides mock vector search results.
type mockVectorSearcher struct {
	results []vectorSearchResult
	err     error
}

func (m *mockVectorSearcher) Search(_ context.Context, _ string, _ []float32, _ int) ([]vectorSearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// relatedDoc represents a graph-related document for testing.
type relatedDoc struct {
	doc   Document
	depth int
}

// mockGraphQuerier provides mock graph query results.
type mockGraphQuerier struct {
	documents  map[string]Document
	related    map[string][]relatedDoc
	centrality map[string]float64
	fetchedIDs []string
	err        error
}

func (m *mockGraphQuerier) FetchDocument(_ context.Context, id string) (*Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.fetchedIDs = append(m.fetchedIDs, id)
	doc, ok := m.documents[id]
	if !ok {
		return nil, nil // not found
	}
	return &doc, nil
}

func (m *mockGraphQuerier) FindRelated(_ context.Context, id string, _ int) ([]relatedDoc, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.related[id], nil
}

func (m *mockGraphQuerier) GetCentrality(_ context.Context, ids []string) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[string]float64)
	for _, id := range ids {
		if score, ok := m.centrality[id]; ok {
			result[id] = score
		}
	}
	return result, nil
}

// mockReranker provides mock reranking results.
type mockReranker struct {
	scores      map[string]float64
	err         error
	calledWithN int
}

func (m *mockReranker) Rerank(_ context.Context, _ string, docs []ScoredDocument) ([]ScoredDocument, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.calledWithN = len(docs)
	for i := range docs {
		if score, ok := m.scores[docs[i].ID]; ok {
			if docs[i].Signals == nil {
				docs[i].Signals = make(map[string]float64)
			}
			docs[i].Signals["rerank"] = score
		}
	}
	return docs, nil
}

// containsStr checks if a string slice contains a value.
func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
