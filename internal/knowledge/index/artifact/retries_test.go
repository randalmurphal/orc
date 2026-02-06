package artifact

import (
	"context"
	"fmt"
	"testing"
)

// --- SC-4: Retry indexing creates :Retry nodes with FROM_TASK ---

// SC-4: Each retry attempt becomes a :Retry node with correct properties.
func TestIndexRetries_CreatesNodes(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	retries := []RetryInfo{
		{Attempt: 1, Reason: "test failures in auth module", FromPhase: "implement"},
		{Attempt: 2, Reason: "linting errors", FromPhase: "implement"},
	}

	err := idx.IndexRetries(context.Background(), "TASK-001", retries)
	if err != nil {
		t.Fatalf("IndexRetries: %v", err)
	}

	retryNodes := mock.nodesWithLabel("Retry")
	if len(retryNodes) != 2 {
		t.Fatalf("expected 2 Retry nodes, got %d", len(retryNodes))
	}
}

// SC-4: Retry nodes have attempt, reason, from_phase properties.
func TestIndexRetries_CorrectProperties(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	retries := []RetryInfo{
		{Attempt: 1, Reason: "test failures", FromPhase: "implement"},
	}

	err := idx.IndexRetries(context.Background(), "TASK-001", retries)
	if err != nil {
		t.Fatalf("IndexRetries: %v", err)
	}

	retryNodes := mock.nodesWithLabel("Retry")
	if len(retryNodes) != 1 {
		t.Fatalf("expected 1 Retry node, got %d", len(retryNodes))
	}

	node := retryNodes[0]
	if node.Properties["attempt"] != 1 {
		t.Errorf("expected attempt=1, got %v", node.Properties["attempt"])
	}
	if node.Properties["reason"] != "test failures" {
		t.Errorf("expected reason='test failures', got %v", node.Properties["reason"])
	}
	if node.Properties["from_phase"] != "implement" {
		t.Errorf("expected from_phase='implement', got %v", node.Properties["from_phase"])
	}
}

// SC-4: FROM_TASK relationship created from :Retry node.
func TestIndexRetries_FromTask(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	retries := []RetryInfo{
		{Attempt: 1, Reason: "failures", FromPhase: "review"},
	}

	err := idx.IndexRetries(context.Background(), "TASK-001", retries)
	if err != nil {
		t.Fatalf("IndexRetries: %v", err)
	}

	fromTaskRels := mock.relsOfType("FROM_TASK")
	if len(fromTaskRels) == 0 {
		t.Fatal("expected FROM_TASK relationship from Retry node")
	}
}

// SC-4 error path: No retries → skip with no error.
func TestIndexRetries_NoRetries(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexRetries(context.Background(), "TASK-001", nil)
	if err != nil {
		t.Fatalf("expected no error for nil retries, got: %v", err)
	}
	if len(mock.nodesWithLabel("Retry")) != 0 {
		t.Error("expected no Retry nodes for nil retries")
	}

	// Also test empty slice.
	err = idx.IndexRetries(context.Background(), "TASK-002", []RetryInfo{})
	if err != nil {
		t.Fatalf("expected no error for empty retries, got: %v", err)
	}
}

// Edge case: Multiple retry attempts across different phases.
func TestIndexRetries_MultiplePhases(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	retries := []RetryInfo{
		{Attempt: 1, Reason: "spec incomplete", FromPhase: "spec"},
		{Attempt: 2, Reason: "test failures", FromPhase: "implement"},
		{Attempt: 3, Reason: "review rejected", FromPhase: "review"},
	}

	err := idx.IndexRetries(context.Background(), "TASK-001", retries)
	if err != nil {
		t.Fatalf("IndexRetries: %v", err)
	}

	retryNodes := mock.nodesWithLabel("Retry")
	if len(retryNodes) != 3 {
		t.Fatalf("expected 3 Retry nodes, got %d", len(retryNodes))
	}

	// Verify each has a distinct from_phase.
	phases := make(map[interface{}]bool)
	for _, n := range retryNodes {
		phases[n.Properties["from_phase"]] = true
	}
	if len(phases) != 3 {
		t.Errorf("expected 3 distinct from_phase values, got %d", len(phases))
	}
}

// Failure mode: Graph store failure propagates error.
func TestIndexRetries_GraphStoreFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("graph unavailable")
	idx := NewIndexer(mock)

	retries := []RetryInfo{
		{Attempt: 1, Reason: "failure", FromPhase: "implement"},
	}

	err := idx.IndexRetries(context.Background(), "TASK-001", retries)
	if err == nil {
		t.Fatal("expected error when graph store fails")
	}
}
