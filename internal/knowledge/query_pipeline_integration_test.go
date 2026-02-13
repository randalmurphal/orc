// Tests for TASK-003: Service → retrieve.Pipeline integration.
//
// These tests verify that Service.Query() correctly wires to the retrieve
// package's Pipeline via preset routing. They complement the unit tests in
// service_query_test.go by verifying STRUCTURAL correctness (which stages
// execute, which signals appear) rather than behavioral correctness (sorting,
// scoring, budget filtering).
//
// The unit tests use only the "fast" preset. These integration tests exercise
// "standard", "deep", "graph_first", and "recency" presets to prove the Service
// routes to the correct retrieve.XxxPreset() builder.
//
// Wiring verified:
//   Service.Query() → retrieve.XxxPreset(deps) → Pipeline.Execute() → staged results
//
// Deletion test: If the Service's import of retrieve or the call to
// retrieve.StandardPreset() is removed, these tests fail because results
// won't contain stage-specific signals.
package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

// =============================================================================
// Service.Query() → Pipeline stage execution integration
//
// Each pipeline stage adds a named signal (e.g., "semantic", "graph",
// "temporal", "pagerank", "rerank") to documents. By checking which signals
// appear in the results, we prove which stages actually executed.
// =============================================================================

// TestServiceQuery_StandardPreset_AllStageSignals verifies that Service.Query()
// with "standard" preset routes to retrieve.StandardPreset() and executes ALL
// pipeline stages. The standard pipeline includes semantic, hydrate,
// graph_expansion, temporal_decay, pagerank, and rerank stages.
//
// Integration: calls Service.Query() (production entry point) → internally
// builds PresetDeps → calls retrieve.StandardPreset() → executes Pipeline.
// If the Service doesn't wire to the retrieve package, no stage-specific
// signals will appear in the results.
func TestServiceQuery_StandardPreset_AllStageSignals(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "how does task execution work?", retrieve.QueryOpts{
		Preset:    "standard",
		MaxTokens: 8000,
	})
	if err != nil {
		t.Fatalf("Query(standard): %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("standard preset should return documents")
	}

	// Standard preset runs: semantic → hydrate → graph_expansion → temporal_decay → pagerank → rerank
	// Each stage adds its signal. Verify core signals are present.
	firstDoc := result.Documents[0]
	coreSignals := []string{"semantic", "temporal", "pagerank"}
	for _, signal := range coreSignals {
		if _, ok := firstDoc.Signals[signal]; !ok {
			t.Errorf("standard preset result missing %q signal — %s stage not wired into pipeline", signal, signal)
		}
	}

	// Graph expansion adds "graph" signal to expanded documents (may appear
	// on related docs, not necessarily the first doc).
	hasGraphSignal := false
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["graph"]; ok {
			hasGraphSignal = true
			break
		}
	}
	if !hasGraphSignal {
		t.Error("standard preset should include graph_expansion stage — no document has 'graph' signal")
	}

	// Rerank adds "rerank" signal to top-K documents with content.
	hasRerankSignal := false
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["rerank"]; ok {
			hasRerankSignal = true
			break
		}
	}
	if !hasRerankSignal {
		t.Error("standard preset should include rerank stage — no document has 'rerank' signal")
	}
}

// TestServiceQuery_FastPreset_MinimalSignals verifies that "fast" preset routes
// to retrieve.FastPreset() which builds a lightweight pipeline (semantic +
// hydrate only). Documents should have "semantic" signal but NOT graph/temporal/
// pagerank/rerank signals.
//
// Proves: preset routing differentiates between pipeline configurations.
func TestServiceQuery_FastPreset_MinimalSignals(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{
		Preset:    "fast",
		MaxTokens: 8000,
	})
	if err != nil {
		t.Fatalf("Query(fast): %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("fast preset should return documents")
	}

	firstDoc := result.Documents[0]

	// Fast preset MUST have semantic signal (from SemanticStage).
	if _, ok := firstDoc.Signals["semantic"]; !ok {
		t.Error("fast preset result should have 'semantic' signal")
	}

	// Fast preset MUST NOT have signals from stages it doesn't include.
	unexpectedSignals := []string{"graph", "temporal", "pagerank", "rerank"}
	for _, signal := range unexpectedSignals {
		if _, ok := firstDoc.Signals[signal]; ok {
			t.Errorf("fast preset result should NOT have %q signal — wrong preset routed", signal)
		}
	}
}

// TestServiceQuery_PresetRoutingProducesDifferentSignalSets verifies that
// different preset names route to structurally different pipelines. If preset
// routing is broken (e.g., all presets map to the same pipeline), this test
// catches it by comparing signal counts.
func TestServiceQuery_PresetRoutingProducesDifferentSignalSets(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	fastResult, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{Preset: "fast"})
	if err != nil {
		t.Fatalf("Query(fast): %v", err)
	}

	stdResult, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{Preset: "standard"})
	if err != nil {
		t.Fatalf("Query(standard): %v", err)
	}

	if len(fastResult.Documents) == 0 || len(stdResult.Documents) == 0 {
		t.Skip("need documents from both presets to compare")
	}

	fastSignalCount := len(fastResult.Documents[0].Signals)
	stdSignalCount := len(stdResult.Documents[0].Signals)

	if stdSignalCount <= fastSignalCount {
		t.Errorf("standard preset should produce more signals (%d) than fast (%d) — preset routing may be broken",
			stdSignalCount, fastSignalCount)
	}
}

// TestServiceQuery_RecencyPreset_HasTemporalSignal verifies that "recency"
// preset routes to retrieve.RecencyPreset() which includes temporal_decay stage.
func TestServiceQuery_RecencyPreset_HasTemporalSignal(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "recent changes", retrieve.QueryOpts{
		Preset: "recency",
	})
	if err != nil {
		t.Fatalf("Query(recency): %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("recency preset should return documents")
	}

	// Recency preset includes temporal_decay stage.
	hasTemporalSignal := false
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["temporal"]; ok {
			hasTemporalSignal = true
			break
		}
	}
	if !hasTemporalSignal {
		t.Error("recency preset should include temporal_decay stage — no document has 'temporal' signal")
	}

	// Recency preset should NOT have graph_expansion or pagerank.
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["graph"]; ok {
			t.Error("recency preset should NOT include graph_expansion stage")
			break
		}
	}
}

// TestServiceQuery_GraphFirstPreset_HasGraphSignal verifies that "graph_first"
// preset routes to retrieve.GraphFirstPreset() which includes graph_expansion
// and pagerank stages but NOT temporal_decay or rerank.
func TestServiceQuery_GraphFirstPreset_HasGraphSignal(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "code dependencies", retrieve.QueryOpts{
		Preset: "graph_first",
	})
	if err != nil {
		t.Fatalf("Query(graph_first): %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("graph_first preset should return documents")
	}

	// Should have pagerank signal (from PageRankStage).
	hasPagerankSignal := false
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["pagerank"]; ok {
			hasPagerankSignal = true
			break
		}
	}
	if !hasPagerankSignal {
		t.Error("graph_first preset should include pagerank stage")
	}

	// Should NOT have temporal or rerank signals.
	for _, doc := range result.Documents {
		if _, ok := doc.Signals["temporal"]; ok {
			t.Error("graph_first preset should NOT include temporal_decay stage")
			break
		}
		if _, ok := doc.Signals["rerank"]; ok {
			t.Error("graph_first preset should NOT include rerank stage")
			break
		}
	}
}

// TestServiceQuery_PipelineResultHasTokensUsed verifies that Service.Query()
// returns a PipelineResult with TokensUsed > 0, proving the retrieve.Pipeline
// executed (not a stub return).
func TestServiceQuery_PipelineResultHasTokensUsed(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	result, err := svc.Query(context.Background(), "test", retrieve.QueryOpts{
		Preset:    "fast",
		MaxTokens: 8000,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Documents) == 0 {
		t.Fatal("expected documents in result")
	}

	if result.TokensUsed <= 0 {
		t.Error("PipelineResult.TokensUsed should be positive — pipeline may not have executed")
	}
}

// TestServiceQuery_ComponentsAdaptedToPresetDeps verifies that the Service
// correctly adapts its Components (or additional deps) into retrieve.PresetDeps.
// The pipeline stages require specific interfaces (Embedder, VectorSearcher,
// GraphQuerier, Reranker). If the adaptation is wrong, stages fail to execute
// and results will be empty or missing signals.
//
// This test uses the recording mock pattern: the mock records which methods
// were called, and we verify the expected methods were invoked.
func TestServiceQuery_ComponentsAdaptedToPresetDeps(t *testing.T) {
	t.Parallel()

	comps := newFullPipelineComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(comps))

	_, err := svc.Query(context.Background(), "test query", retrieve.QueryOpts{
		Preset: "fast",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	// Fast preset uses SemanticStage which requires Embed + Search.
	// If the Service didn't adapt components correctly, these wouldn't be called.
	if !comps.embedCalled {
		t.Error("Service.Query() should call Embed() on the components — adapter wiring missing")
	}
	if !comps.searchCalled {
		t.Error("Service.Query() should call Search() on the components — adapter wiring missing")
	}
}

// =============================================================================
// Test double: fullPipelineComponents
//
// Implements Components (lifecycle + health) AND the retrieve package interfaces
// (Embedder, VectorSearcher, GraphQuerier, Reranker) needed for full pipeline
// execution. Records which methods were called for wiring verification.
// =============================================================================

type fullPipelineComponents struct {
	// Data for pipeline stages
	embeddings   map[string][]float32
	documents    map[string]retrieve.Document
	related      map[string][]fullRelatedDoc
	centrality   map[string]float64
	rerankScores map[string]float64

	// Recording flags
	embedCalled       bool
	searchCalled      bool
	fetchCalled       bool
	findRelatedCalled bool
	centralityCalled  bool
	rerankCalled      bool
}

type fullRelatedDoc struct {
	id    string
	depth int
}

func newFullPipelineComponents() *fullPipelineComponents {
	now := time.Now()
	return &fullPipelineComponents{
		embeddings: map[string][]float32{},
		documents: map[string]retrieve.Document{
			"doc-exec": {
				ID: "doc-exec", Content: "Executor runs workflow phases sequentially",
				Summary: "Executor overview", FilePath: "internal/executor/executor.go",
				Tokens: 50, UpdatedAt: now.Add(-24 * time.Hour),
			},
			"doc-phase": {
				ID: "doc-phase", Content: "Phase model defines execution stages",
				Summary: "Phase model", FilePath: "internal/executor/workflow_phase.go",
				Tokens: 45, UpdatedAt: now.Add(-48 * time.Hour),
			},
			"doc-gate": {
				ID: "doc-gate", Content: "Gate evaluation controls quality checkpoints",
				Summary: "Gate evaluation", FilePath: "internal/gate/gate.go",
				Tokens: 40, UpdatedAt: now.Add(-72 * time.Hour),
			},
			"doc-related": {
				ID: "doc-related", Content: "Task model defines task state machine",
				Summary: "Task model", FilePath: "internal/task/task.go",
				Tokens: 35, UpdatedAt: now.Add(-96 * time.Hour),
			},
		},
		related: map[string][]fullRelatedDoc{
			"doc-exec": {
				{id: "doc-related", depth: 1},
			},
		},
		centrality: map[string]float64{
			"doc-exec":    0.85,
			"doc-phase":   0.65,
			"doc-gate":    0.50,
			"doc-related": 0.40,
		},
		rerankScores: map[string]float64{
			"doc-exec":  0.95,
			"doc-phase": 0.80,
			"doc-gate":  0.60,
		},
	}
}

// --- Components interface ---

func (f *fullPipelineComponents) InfraStart(_ context.Context) error        { return nil }
func (f *fullPipelineComponents) InfraStop(_ context.Context) error         { return nil }
func (f *fullPipelineComponents) GraphConnect(_ context.Context) error      { return nil }
func (f *fullPipelineComponents) GraphClose() error                         { return nil }
func (f *fullPipelineComponents) VectorConnect(_ context.Context) error     { return nil }
func (f *fullPipelineComponents) VectorClose() error                        { return nil }
func (f *fullPipelineComponents) CacheConnect(_ context.Context) error      { return nil }
func (f *fullPipelineComponents) CacheClose() error                         { return nil }
func (f *fullPipelineComponents) IsHealthy() (neo4j, qdrant, redis bool)   { return true, true, true }

// --- retrieve.Embedder interface ---

func (f *fullPipelineComponents) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.embedCalled = true
	result := make([][]float32, len(texts))
	for i, text := range texts {
		if vec, ok := f.embeddings[text]; ok {
			result[i] = vec
		} else {
			result[i] = []float32{0.1, 0.2, 0.3} // default embedding
		}
	}
	return result, nil
}

func (f *fullPipelineComponents) Type() string { return "mock" }

// --- retrieve.VectorSearcher interface ---

func (f *fullPipelineComponents) Search(_ context.Context, _ string, _ []float32, limit int) ([]retrieve.VectorSearchResult, error) {
	f.searchCalled = true
	var results []retrieve.VectorSearchResult
	scores := []float32{0.92, 0.85, 0.78, 0.60}
	i := 0
	for id := range f.documents {
		if limit > 0 && i >= limit {
			break
		}
		score := float32(0.5)
		if i < len(scores) {
			score = scores[i]
		}
		results = append(results, retrieve.VectorSearchResult{
			ID:    id,
			Score: score,
		})
		i++
	}
	return results, nil
}

// --- retrieve.GraphQuerier interface ---

func (f *fullPipelineComponents) FetchDocument(_ context.Context, id string) (*retrieve.Document, error) {
	f.fetchCalled = true
	doc, ok := f.documents[id]
	if !ok {
		return nil, nil
	}
	return &doc, nil
}

func (f *fullPipelineComponents) FindRelated(_ context.Context, id string, _ int) ([]retrieve.RelatedDoc, error) {
	f.findRelatedCalled = true
	rels := f.related[id]
	var out []retrieve.RelatedDoc
	for _, r := range rels {
		doc := f.documents[r.id]
		out = append(out, retrieve.RelatedDoc{Doc: doc, Depth: r.depth})
	}
	return out, nil
}

func (f *fullPipelineComponents) GetCentrality(_ context.Context, ids []string) (map[string]float64, error) {
	f.centralityCalled = true
	result := make(map[string]float64)
	for _, id := range ids {
		if score, ok := f.centrality[id]; ok {
			result[id] = score
		}
	}
	return result, nil
}

// --- retrieve.Reranker interface ---

func (f *fullPipelineComponents) Rerank(_ context.Context, _ string, docs []retrieve.ScoredDocument) ([]retrieve.ScoredDocument, error) {
	f.rerankCalled = true
	for i := range docs {
		if score, ok := f.rerankScores[docs[i].ID]; ok {
			if docs[i].Signals == nil {
				docs[i].Signals = make(map[string]float64)
			}
			docs[i].Signals["rerank"] = score
		}
	}
	return docs, nil
}
