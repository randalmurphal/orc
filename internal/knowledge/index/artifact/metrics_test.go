package artifact

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// --- SC-5: Metrics indexing updates :File nodes with aggregated metrics ---

// SC-5: Modified files get metrics updates via MERGE queries.
func TestIndexMetrics_UpdatesFileNodes(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	changedFiles := []string{
		"internal/executor/workflow_executor.go",
		"internal/knowledge/knowledge.go",
		"internal/api/server.go",
	}

	err := idx.IndexMetrics(context.Background(), "TASK-001", changedFiles, 0)
	if err != nil {
		t.Fatalf("IndexMetrics: %v", err)
	}

	// Verify one operation per changed file (either Cypher MERGE or node create/update).
	calls := mock.allCypherCalls()
	if len(calls) < 3 {
		t.Errorf("expected at least 3 Cypher operations (one per file), got %d", len(calls))
	}
}

// SC-5: total_tasks_touching is included in the update.
func TestIndexMetrics_IncrementsTotalTasks(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexMetrics(context.Background(), "TASK-001", []string{"internal/foo.go"}, 0)
	if err != nil {
		t.Fatalf("IndexMetrics: %v", err)
	}

	// Verify Cypher query mentions total_tasks_touching increment.
	calls := mock.allCypherCalls()
	found := false
	for _, c := range calls {
		if strings.Contains(c.query, "total_tasks_touching") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Cypher query to reference total_tasks_touching")
	}
}

// SC-5: avg_retry_rate is included in the update.
func TestIndexMetrics_UpdatesRetryRate(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	// Task had 2 retries.
	err := idx.IndexMetrics(context.Background(), "TASK-001", []string{"internal/foo.go"}, 2)
	if err != nil {
		t.Fatalf("IndexMetrics: %v", err)
	}

	// Verify Cypher params include retry rate information.
	calls := mock.allCypherCalls()
	found := false
	for _, c := range calls {
		if strings.Contains(c.query, "avg_retry_rate") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Cypher query to reference avg_retry_rate")
	}
}

// SC-5 edge case: File node doesn't exist → MERGE creates with initial values.
func TestIndexMetrics_NewFile(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	// No pre-existing File nodes in mock — all files are new.
	err := idx.IndexMetrics(context.Background(), "TASK-001", []string{"brand_new_file.go"}, 0)
	if err != nil {
		t.Fatalf("IndexMetrics: %v", err)
	}

	// Verify Cypher uses MERGE (which handles create-or-update).
	calls := mock.allCypherCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one Cypher call for metrics update")
	}
	foundMerge := false
	for _, c := range calls {
		if strings.Contains(strings.ToUpper(c.query), "MERGE") {
			foundMerge = true
			break
		}
	}
	if !foundMerge {
		t.Error("expected MERGE query for handling new file nodes")
	}
}

// Edge case: No changed files → nothing to update.
func TestIndexMetrics_NoChangedFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexMetrics(context.Background(), "TASK-001", nil, 0)
	if err != nil {
		t.Fatalf("expected no error for nil changed files, got: %v", err)
	}
	if len(mock.allCypherCalls()) != 0 {
		t.Error("expected no Cypher calls for nil changed files")
	}

	// Also test empty slice.
	err = idx.IndexMetrics(context.Background(), "TASK-002", []string{}, 0)
	if err != nil {
		t.Fatalf("expected no error for empty changed files, got: %v", err)
	}
}

// Failure mode: Cypher execution failure propagates error.
func TestIndexMetrics_CypherFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.cypherErr = fmt.Errorf("cypher execution failed")
	idx := NewIndexer(mock)

	err := idx.IndexMetrics(context.Background(), "TASK-001", []string{"internal/foo.go"}, 0)
	if err == nil {
		t.Fatal("expected error when Cypher execution fails")
	}
}
