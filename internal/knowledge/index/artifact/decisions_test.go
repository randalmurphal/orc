package artifact

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/initiative"
)

// --- SC-3: Initiative decision indexing creates :Decision nodes ---

// SC-3: Each decision creates a :Decision node with content, rationale, decision_id.
func TestIndexDecisions_CreatesNodes(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	decisions := []initiative.Decision{
		{
			ID:        "DEC-001",
			Date:      time.Now(),
			By:        "user",
			Decision:  "Use bcrypt for passwords",
			Rationale: "Industry standard, resistant to brute force",
		},
		{
			ID:        "DEC-002",
			Date:      time.Now(),
			By:        "user",
			Decision:  "JWT tokens with 1h expiry",
			Rationale: "Balance security with UX",
		},
	}
	changedFiles := []string{"internal/auth/bcrypt.go", "internal/auth/jwt.go"}

	err := idx.IndexDecisions(context.Background(), "TASK-001", "INIT-001", decisions, changedFiles)
	if err != nil {
		t.Fatalf("IndexDecisions: %v", err)
	}

	decNodes := mock.nodesWithLabel("Decision")
	if len(decNodes) != 2 {
		t.Fatalf("expected 2 Decision nodes, got %d", len(decNodes))
	}

	// Verify properties.
	node := decNodes[0]
	if node.Properties["decision_id"] == nil {
		t.Error("Decision node missing decision_id property")
	}
	if node.Properties["content"] == nil {
		t.Error("Decision node missing content property")
	}
	if node.Properties["rationale"] == nil {
		t.Error("Decision node missing rationale property")
	}
}

// SC-3: FROM_INITIATIVE relationship links decision to initiative.
func TestIndexDecisions_FromInitiative(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	decisions := []initiative.Decision{
		{
			ID:       "DEC-001",
			Decision: "Use bcrypt",
		},
	}

	err := idx.IndexDecisions(context.Background(), "TASK-001", "INIT-001", decisions, nil)
	if err != nil {
		t.Fatalf("IndexDecisions: %v", err)
	}

	fromInitRels := mock.relsOfType("FROM_INITIATIVE")
	if len(fromInitRels) == 0 {
		t.Fatal("expected FROM_INITIATIVE relationship")
	}
}

// SC-3: AFFECTS relationships link decisions to changed files.
func TestIndexDecisions_AffectsFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	decisions := []initiative.Decision{
		{
			ID:       "DEC-001",
			Decision: "Use bcrypt",
		},
	}
	changedFiles := []string{
		"internal/auth/bcrypt.go",
		"internal/auth/handler.go",
	}

	err := idx.IndexDecisions(context.Background(), "TASK-001", "INIT-001", decisions, changedFiles)
	if err != nil {
		t.Fatalf("IndexDecisions: %v", err)
	}

	affectsRels := mock.relsOfType("AFFECTS")
	if len(affectsRels) < 2 {
		t.Errorf("expected at least 2 AFFECTS relationships, got %d", len(affectsRels))
	}
}

// SC-3 error path: No initiative → skip with no error.
func TestIndexDecisions_NoInitiative(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	decisions := []initiative.Decision{
		{ID: "DEC-001", Decision: "Use bcrypt"},
	}

	// Empty initiative ID means task has no initiative — skip.
	err := idx.IndexDecisions(context.Background(), "TASK-001", "", decisions, nil)
	if err != nil {
		t.Fatalf("expected no error for empty initiative, got: %v", err)
	}
	if len(mock.nodesWithLabel("Decision")) != 0 {
		t.Error("expected no Decision nodes when initiative is empty")
	}
}

// SC-3 error path: No decisions → skip with no error.
func TestIndexDecisions_NoDecisions(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexDecisions(context.Background(), "TASK-001", "INIT-001", nil, nil)
	if err != nil {
		t.Fatalf("expected no error for nil decisions, got: %v", err)
	}
	if len(mock.nodesWithLabel("Decision")) != 0 {
		t.Error("expected no Decision nodes for nil decisions")
	}
}

// Failure mode: Graph store failure propagates error.
func TestIndexDecisions_GraphStoreFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("graph unavailable")
	idx := NewIndexer(mock)

	decisions := []initiative.Decision{
		{ID: "DEC-001", Decision: "Use bcrypt"},
	}

	err := idx.IndexDecisions(context.Background(), "TASK-001", "INIT-001", decisions, nil)
	if err == nil {
		t.Fatal("expected error when graph store fails")
	}
}
