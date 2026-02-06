package artifact

import (
	"context"
	"fmt"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// --- Orchestrator: IndexAll coordinates all artifact indexers ---

// SC-6 (unit aspect): IndexAll calls all sub-indexers with correct data.
func TestIndexAll_CallsAllIndexers(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID: "TASK-001",
		Spec:   "## Spec\nModifies internal/foo.go",
		Findings: []*orcv1.ReviewRoundFindings{
			{
				TaskId: "TASK-001",
				Round:  1,
				Issues: []*orcv1.ReviewFinding{
					{
						Severity:    "high",
						File:        strPtr("internal/handler.go"),
						Description: "Issue found",
						AgentId:     strPtr("reviewer"),
					},
				},
			},
		},
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Use bcrypt", Rationale: "Standard"},
		},
		InitiativeID: "INIT-001",
		Retries: []RetryInfo{
			{Attempt: 1, Reason: "test failure", FromPhase: "implement"},
		},
		ChangedFiles: []string{"internal/foo.go", "internal/bar.go"},
		ScratchpadEntries: []storage.ScratchpadEntry{
			{
				ID:        1,
				TaskID:    "TASK-001",
				PhaseID:   "implement",
				Category:  "observation",
				Content:   "Noticed complexity in internal/foo.go",
				CreatedAt: time.Now(),
			},
		},
	}

	err := idx.IndexAll(context.Background(), params)
	if err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Verify nodes were created for each artifact type.
	if len(mock.nodesWithLabel("Spec")) == 0 {
		t.Error("expected Spec node from IndexAll")
	}
	if len(mock.nodesWithLabel("Finding")) == 0 {
		t.Error("expected Finding node from IndexAll")
	}
	if len(mock.nodesWithLabel("Decision")) == 0 {
		t.Error("expected Decision node from IndexAll")
	}
	if len(mock.nodesWithLabel("Retry")) == 0 {
		t.Error("expected Retry node from IndexAll")
	}
	if len(mock.nodesWithLabel("Observation")) == 0 {
		t.Error("expected Observation node from IndexAll")
	}

	// Verify metrics Cypher calls were made for changed files.
	calls := mock.allCypherCalls()
	if len(calls) == 0 {
		t.Error("expected Cypher calls for metrics updates")
	}
}

// BDD-4: Task with no data — all sub-indexers skip gracefully.
func TestIndexAll_SkipsGracefully(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID:            "TASK-001",
		Spec:              "",
		Findings:          nil,
		Decisions:         nil,
		InitiativeID:      "",
		Retries:           nil,
		ChangedFiles:      nil,
		ScratchpadEntries: nil,
	}

	err := idx.IndexAll(context.Background(), params)
	if err != nil {
		t.Fatalf("expected no error for empty params, got: %v", err)
	}

	// No nodes should be created.
	allNodeCount := len(mock.nodesWithLabel("Spec")) +
		len(mock.nodesWithLabel("Finding")) +
		len(mock.nodesWithLabel("Decision")) +
		len(mock.nodesWithLabel("Retry")) +
		len(mock.nodesWithLabel("Observation")) +
		len(mock.nodesWithLabel("Warning"))
	if allNodeCount != 0 {
		t.Errorf("expected 0 nodes for empty params, got %d", allNodeCount)
	}
}

// SC-11 (unit aspect): Individual indexer errors don't stop other indexers.
func TestIndexAll_ContinuesOnIndividualError(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	// Make Spec node creation fail, but others should still work.
	mock.createNodeErrForLabel = map[string]error{
		"Spec": fmt.Errorf("spec indexing failed"),
	}
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID: "TASK-001",
		Spec:   "Some spec about internal/foo.go",
		Findings: []*orcv1.ReviewRoundFindings{
			{
				TaskId: "TASK-001",
				Round:  1,
				Issues: []*orcv1.ReviewFinding{
					{
						Severity:    "medium",
						File:        strPtr("internal/bar.go"),
						Description: "Minor issue",
						AgentId:     strPtr("reviewer"),
					},
				},
			},
		},
		Retries: []RetryInfo{
			{Attempt: 1, Reason: "failure", FromPhase: "implement"},
		},
	}

	err := idx.IndexAll(context.Background(), params)
	// Should return error (spec failed), but other indexers should have run.
	if err == nil {
		t.Fatal("expected error when spec indexing fails")
	}

	// Findings should still be indexed despite spec failure.
	if len(mock.nodesWithLabel("Finding")) == 0 {
		t.Error("Finding nodes should be indexed even when Spec indexing fails")
	}

	// Retries should still be indexed.
	if len(mock.nodesWithLabel("Retry")) == 0 {
		t.Error("Retry nodes should be indexed even when Spec indexing fails")
	}
}

// Edge case: Same task indexed twice (idempotent).
func TestIndexAll_Idempotent(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID: "TASK-001",
		Spec:   "Some spec about internal/foo.go",
		Retries: []RetryInfo{
			{Attempt: 1, Reason: "failure", FromPhase: "implement"},
		},
		ChangedFiles: []string{"internal/foo.go"},
	}

	// Index twice.
	if err := idx.IndexAll(context.Background(), params); err != nil {
		t.Fatalf("first IndexAll: %v", err)
	}
	if err := idx.IndexAll(context.Background(), params); err != nil {
		t.Fatalf("second IndexAll: %v", err)
	}

	// Spec should be exactly 1 (idempotent).
	specNodes := mock.nodesWithLabel("Spec")
	if len(specNodes) != 1 {
		t.Errorf("expected 1 Spec node after double indexing, got %d", len(specNodes))
	}

	// Retry should be exactly 1 (idempotent).
	retryNodes := mock.nodesWithLabel("Retry")
	if len(retryNodes) != 1 {
		t.Errorf("expected 1 Retry node after double indexing, got %d", len(retryNodes))
	}
}

// Edge case: Task completed with zero changed files.
func TestIndexAll_NoChangedFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID:       "TASK-001",
		Spec:         "Some spec about internal/foo.go",
		ChangedFiles: nil,
	}

	err := idx.IndexAll(context.Background(), params)
	if err != nil {
		t.Fatalf("IndexAll: %v", err)
	}

	// Spec should still be indexed.
	if len(mock.nodesWithLabel("Spec")) == 0 {
		t.Error("Spec should be indexed even with no changed files")
	}

	// No metrics Cypher calls since no files changed.
	// (Metrics calls happen only for changed files.)
}

// Failure mode: All graph operations fail — all errors collected.
func TestIndexAll_AllGraphOperationsFail(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("graph completely unavailable")
	mock.cypherErr = fmt.Errorf("cypher unavailable")
	idx := NewIndexer(mock)

	params := IndexParams{
		TaskID:       "TASK-001",
		Spec:         "Some spec about internal/foo.go",
		ChangedFiles: []string{"internal/foo.go"},
		Retries: []RetryInfo{
			{Attempt: 1, Reason: "failure", FromPhase: "implement"},
		},
		ScratchpadEntries: []storage.ScratchpadEntry{
			{ID: 1, TaskID: "TASK-001", PhaseID: "impl", Category: "decision", Content: "A decision"},
		},
	}

	err := idx.IndexAll(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when all graph operations fail")
	}
}

// Failure mode: Backend load error (simulated by invalid data reaching the indexer).
func TestIndexAll_NilTaskID(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	// TaskID is required — empty should either error or handle gracefully.
	params := IndexParams{
		TaskID: "",
		Spec:   "Some spec",
	}

	// The indexer should either reject empty task ID or handle gracefully.
	// Either behavior is acceptable as long as it doesn't panic.
	_ = idx.IndexAll(context.Background(), params)
}
