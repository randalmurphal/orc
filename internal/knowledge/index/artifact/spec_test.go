package artifact

import (
	"context"
	"fmt"
	"testing"
)

// --- SC-1: Spec indexing creates :Spec node linked to task and target files ---

// SC-1: Spec node created with task_id and content_hash properties.
func TestIndexSpec_CreatesSpecNode(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	spec := "## Overview\nThis task modifies internal/executor/workflow_executor.go"

	err := idx.IndexSpec(context.Background(), "TASK-001", spec)
	if err != nil {
		t.Fatalf("IndexSpec: %v", err)
	}

	specNodes := mock.nodesWithLabel("Spec")
	if len(specNodes) != 1 {
		t.Fatalf("expected 1 Spec node, got %d", len(specNodes))
	}

	node := specNodes[0]
	if node.Properties["task_id"] != "TASK-001" {
		t.Errorf("expected task_id=TASK-001, got %v", node.Properties["task_id"])
	}
	if _, ok := node.Properties["content_hash"]; !ok {
		t.Error("Spec node missing content_hash property")
	}
}

// SC-1: FROM_TASK relationship created from Spec node to task node.
func TestIndexSpec_LinksToTask(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexSpec(context.Background(), "TASK-001", "Some spec content about internal/foo.go")
	if err != nil {
		t.Fatalf("IndexSpec: %v", err)
	}

	fromTaskRels := mock.relsOfType("FROM_TASK")
	if len(fromTaskRels) == 0 {
		t.Fatal("expected at least one FROM_TASK relationship from Spec node")
	}
}

// SC-1: File paths extracted from spec text create TARGETS relationships to :File nodes.
func TestIndexSpec_ExtractsTargetFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	spec := `## Changes
This feature modifies:
- internal/executor/workflow_executor.go
- internal/knowledge/knowledge.go
- web/src/components/TaskList.tsx
`

	err := idx.IndexSpec(context.Background(), "TASK-001", spec)
	if err != nil {
		t.Fatalf("IndexSpec: %v", err)
	}

	targetsRels := mock.relsOfType("TARGETS")
	if len(targetsRels) < 3 {
		t.Errorf("expected at least 3 TARGETS relationships, got %d", len(targetsRels))
	}

	// Verify File nodes created for extracted paths.
	fileNodes := mock.nodesWithLabel("File")
	if len(fileNodes) < 3 {
		t.Errorf("expected at least 3 File nodes, got %d", len(fileNodes))
	}

	// Verify at least one File node has a path property matching expected.
	found := false
	for _, n := range fileNodes {
		if n.Properties["path"] == "internal/executor/workflow_executor.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected File node for internal/executor/workflow_executor.go")
	}
}

// SC-1 error path: Empty spec is skipped with no error.
func TestIndexSpec_EmptySpec(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexSpec(context.Background(), "TASK-001", "")
	if err != nil {
		t.Fatalf("expected no error for empty spec, got: %v", err)
	}

	if len(mock.nodesWithLabel("Spec")) != 0 {
		t.Error("expected no Spec nodes for empty spec")
	}
}

// Edge case: Re-indexing same task is idempotent (updates, not duplicates).
func TestIndexSpec_Idempotent(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	spec := "## Overview\nModifies internal/foo.go"

	// Index twice.
	if err := idx.IndexSpec(context.Background(), "TASK-001", spec); err != nil {
		t.Fatalf("first IndexSpec: %v", err)
	}
	if err := idx.IndexSpec(context.Background(), "TASK-001", spec); err != nil {
		t.Fatalf("second IndexSpec: %v", err)
	}

	// After idempotent re-index, should have exactly 1 Spec node (old deleted, new created).
	specNodes := mock.nodesWithLabel("Spec")
	if len(specNodes) != 1 {
		t.Errorf("expected 1 Spec node after re-indexing, got %d", len(specNodes))
	}
}

// Failure mode: Graph store failure propagates error.
func TestIndexSpec_GraphStoreFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("neo4j connection refused")
	idx := NewIndexer(mock)

	err := idx.IndexSpec(context.Background(), "TASK-001", "Some spec content")
	if err == nil {
		t.Fatal("expected error when graph store fails")
	}
}
