package retrieve

import (
	"context"
	"errors"
	"testing"
)

// =============================================================================
// SC-1: Pipeline executes stages sequentially, each receiving prior output
// =============================================================================

func TestPipeline_StagesExecuteInOrder(t *testing.T) {
	var order []string

	stageA := &recordingStage{name: "A", order: &order, produce: []ScoredDocument{
		{Document: Document{ID: "doc-1"}, Signals: map[string]float64{"a": 0.9}},
	}}
	stageB := &recordingStage{name: "B", order: &order}
	stageC := &recordingStage{name: "C", order: &order}

	p, err := NewPipelineBuilder().
		AddStage(stageA).
		AddStage(stageB).
		AddStage(stageC).
		WithScorer(NewWeightedScorer(map[string]float64{"a": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	result, err := p.Execute(context.Background(), "test query")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify stage execution order
	if len(order) != 3 {
		t.Fatalf("expected 3 stages executed, got %d: %v", len(order), order)
	}
	if order[0] != "A" || order[1] != "B" || order[2] != "C" {
		t.Errorf("stages executed in wrong order: %v", order)
	}

	// Verify results came through
	if len(result.Documents) == 0 {
		t.Error("pipeline should produce results")
	}
}

func TestPipeline_StageReceivesPriorOutput(t *testing.T) {
	// Stage A produces documents, Stage B should receive them
	stageA := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-from-a"}, Signals: map[string]float64{"sem": 0.8}},
		{Document: Document{ID: "doc-from-a-2"}, Signals: map[string]float64{"sem": 0.6}},
	}}
	stageB := &captureInputStage{}

	p, err := NewPipelineBuilder().
		AddStage(stageA).
		AddStage(stageB).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err = p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Stage B should have received A's output as input candidates
	if len(stageB.receivedCandidates) != 2 {
		t.Errorf("stage B should receive 2 candidates from A, got %d", len(stageB.receivedCandidates))
	}
	if stageB.receivedCandidates[0].ID != "doc-from-a" {
		t.Errorf("stage B should receive doc-from-a, got %s", stageB.receivedCandidates[0].ID)
	}
}

func TestPipeline_FirstStageReceivesNoCandidates(t *testing.T) {
	stageA := &captureInputStage{}

	p, err := NewPipelineBuilder().
		AddStage(stageA).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err = p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(stageA.receivedCandidates) != 0 {
		t.Errorf("first stage should receive 0 candidates, got %d", len(stageA.receivedCandidates))
	}
}

func TestPipeline_StageErrorStopsPipeline(t *testing.T) {
	stageA := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "doc-1"}},
	}}
	stageB := &errorStage{err: errors.New("stage B failed")}
	stageC := &captureInputStage{}

	p, err := NewPipelineBuilder().
		AddStage(stageA).
		AddStage(stageB).
		AddStage(stageC).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err = p.Execute(context.Background(), "test")
	if err == nil {
		t.Fatal("pipeline should return error when stage fails")
	}

	// Stage C should NOT have been called
	if len(stageC.receivedCandidates) > 0 || stageC.called {
		t.Error("stages after failed stage should not execute")
	}
}

// =============================================================================
// SC-2: Scorer computes normalized weighted sum of all signals
// =============================================================================

func TestWeightedScorer_NormalizedWeightedSum(t *testing.T) {
	scorer := NewWeightedScorer(map[string]float64{
		"semantic": 0.5,
		"graph":    0.5,
	})

	// Given signals {semantic: 0.8, graph: 0.6} with weights {semantic: 0.5, graph: 0.5}
	// score = (0.8*0.5 + 0.6*0.5) / (0.5+0.5) = 0.7
	score := scorer.Score(map[string]float64{
		"semantic": 0.8,
		"graph":    0.6,
	})

	if !approxEqual(score, 0.7, 0.001) {
		t.Errorf("score = %f, want 0.7", score)
	}
}

func TestWeightedScorer_MissingSignalContributesZero(t *testing.T) {
	scorer := NewWeightedScorer(map[string]float64{
		"semantic": 0.5,
		"graph":    0.5,
	})

	// Only semantic signal present; graph contributes 0
	// score = (0.8*0.5 + 0*0.5) / (0.5+0.5) = 0.4
	score := scorer.Score(map[string]float64{
		"semantic": 0.8,
	})

	if !approxEqual(score, 0.4, 0.001) {
		t.Errorf("score = %f, want 0.4", score)
	}
}

func TestWeightedScorer_ZeroTotalWeightReturnsZero(t *testing.T) {
	scorer := NewWeightedScorer(map[string]float64{})

	score := scorer.Score(map[string]float64{"semantic": 0.9})

	if score != 0.0 {
		t.Errorf("score with zero total weight = %f, want 0.0", score)
	}
}

func TestWeightedScorer_ScoreInZeroOneRange(t *testing.T) {
	scorer := NewWeightedScorer(map[string]float64{
		"semantic": 0.3,
		"graph":    0.2,
		"temporal": 0.3,
		"pagerank": 0.2,
	})

	tests := []struct {
		name    string
		signals map[string]float64
	}{
		{"all max", map[string]float64{"semantic": 1.0, "graph": 1.0, "temporal": 1.0, "pagerank": 1.0}},
		{"all zero", map[string]float64{"semantic": 0.0, "graph": 0.0, "temporal": 0.0, "pagerank": 0.0}},
		{"mixed", map[string]float64{"semantic": 0.8, "graph": 0.3, "temporal": 0.5}},
		{"empty signals", map[string]float64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.Score(tt.signals)
			if score < 0.0 || score > 1.0 {
				t.Errorf("score %f outside [0, 1] range", score)
			}
		})
	}
}

func TestWeightedScorer_UnequalWeights(t *testing.T) {
	scorer := NewWeightedScorer(map[string]float64{
		"semantic": 0.8,
		"temporal": 0.2,
	})

	// score = (1.0*0.8 + 0.0*0.2) / (0.8+0.2) = 0.8
	score := scorer.Score(map[string]float64{
		"semantic": 1.0,
		"temporal": 0.0,
	})

	if !approxEqual(score, 0.8, 0.001) {
		t.Errorf("score = %f, want 0.8", score)
	}
}

// =============================================================================
// SC-9: Pipeline applies scorer, sorts descending, respects limit and MinScore
// =============================================================================

func TestPipeline_ResultsSortedByScoreDescending(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "low"}, Signals: map[string]float64{"sem": 0.2}},
		{Document: Document{ID: "high"}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "mid"}, Signals: map[string]float64{"sem": 0.5}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Documents) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Documents))
	}
	if result.Documents[0].ID != "high" {
		t.Errorf("first result should be 'high', got %s", result.Documents[0].ID)
	}
	if result.Documents[1].ID != "mid" {
		t.Errorf("second result should be 'mid', got %s", result.Documents[1].ID)
	}
	if result.Documents[2].ID != "low" {
		t.Errorf("third result should be 'low', got %s", result.Documents[2].ID)
	}
}

func TestPipeline_RespectsLimit(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "1"}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "2"}, Signals: map[string]float64{"sem": 0.8}},
		{Document: Document{ID: "3"}, Signals: map[string]float64{"sem": 0.7}},
		{Document: Document{ID: "4"}, Signals: map[string]float64{"sem": 0.6}},
		{Document: Document{ID: "5"}, Signals: map[string]float64{"sem": 0.5}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithLimit(3).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Documents) != 3 {
		t.Errorf("expected 3 results with limit=3, got %d", len(result.Documents))
	}
}

func TestPipeline_MinScoreFiltersLowResults(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "high"}, Signals: map[string]float64{"sem": 0.9}},
		{Document: Document{ID: "mid"}, Signals: map[string]float64{"sem": 0.5}},
		{Document: Document{ID: "low"}, Signals: map[string]float64{"sem": 0.1}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMinScore(0.4).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Only "high" (0.9) and "mid" (0.5) should remain
	if len(result.Documents) != 2 {
		t.Errorf("expected 2 results above MinScore=0.4, got %d", len(result.Documents))
	}
	for _, doc := range result.Documents {
		if doc.FinalScore < 0.4 {
			t.Errorf("document %s has score %f below MinScore 0.4", doc.ID, doc.FinalScore)
		}
	}
}

func TestPipeline_AllBelowMinScoreReturnsEmpty(t *testing.T) {
	stage := &producerStage{docs: []ScoredDocument{
		{Document: Document{ID: "low"}, Signals: map[string]float64{"sem": 0.1}},
	}}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		WithMinScore(0.9).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := p.Execute(context.Background(), "test")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.Documents) != 0 {
		t.Errorf("expected 0 results when all below MinScore, got %d", len(result.Documents))
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestPipelineBuilder_ZeroStagesReturnsError(t *testing.T) {
	_, err := NewPipelineBuilder().
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()

	if err == nil {
		t.Fatal("building pipeline with zero stages should return error")
	}
}

func TestPipeline_EmptyQueryReturnsError(t *testing.T) {
	stage := &producerStage{}

	p, err := NewPipelineBuilder().
		AddStage(stage).
		WithScorer(NewWeightedScorer(map[string]float64{"sem": 1.0})).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err = p.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("pipeline should return error for empty query")
	}
}

// =============================================================================
// Test doubles
// =============================================================================

// recordingStage records execution order and optionally produces documents.
type recordingStage struct {
	name    string
	order   *[]string
	produce []ScoredDocument
}

func (s *recordingStage) Name() string { return s.name }
func (s *recordingStage) Execute(_ context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	*s.order = append(*s.order, s.name)
	if len(s.produce) > 0 {
		return append(candidates, s.produce...), nil
	}
	return candidates, nil
}

// producerStage always returns its configured documents.
type producerStage struct {
	docs []ScoredDocument
}

func (s *producerStage) Name() string { return "producer" }
func (s *producerStage) Execute(_ context.Context, _ string, _ []ScoredDocument) ([]ScoredDocument, error) {
	return s.docs, nil
}

// captureInputStage records what candidates it received.
type captureInputStage struct {
	receivedCandidates []ScoredDocument
	called             bool
}

func (s *captureInputStage) Name() string { return "capture" }
func (s *captureInputStage) Execute(_ context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	s.called = true
	s.receivedCandidates = candidates
	return candidates, nil
}

// errorStage always returns an error.
type errorStage struct {
	err error
}

func (s *errorStage) Name() string { return "error" }
func (s *errorStage) Execute(_ context.Context, _ string, _ []ScoredDocument) ([]ScoredDocument, error) {
	return nil, s.err
}

// approxEqual checks if two floats are approximately equal.
func approxEqual(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}
