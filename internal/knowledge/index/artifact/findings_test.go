package artifact

import (
	"context"
	"fmt"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// --- SC-2: Review findings indexing creates :Finding nodes ---

// SC-2: Each finding becomes a :Finding node with correct properties.
func TestIndexFindings_CreatesNodes(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	findings := []*orcv1.ReviewRoundFindings{
		{
			TaskId:  "TASK-001",
			Round:   1,
			Summary: "Found 2 issues",
			Issues: []*orcv1.ReviewFinding{
				{
					Severity:    "high",
					File:        strPtr("internal/handler.go"),
					Line:        int32Ptr(42),
					Description: "Buffer overflow risk in input parsing",
					AgentId:     strPtr("security-reviewer"),
				},
				{
					Severity:    "medium",
					File:        strPtr("internal/api/server.go"),
					Line:        int32Ptr(115),
					Description: "Missing input validation on user field",
					AgentId:     strPtr("code-reviewer"),
				},
			},
		},
	}

	err := idx.IndexFindings(context.Background(), "TASK-001", findings)
	if err != nil {
		t.Fatalf("IndexFindings: %v", err)
	}

	findingNodes := mock.nodesWithLabel("Finding")
	if len(findingNodes) != 2 {
		t.Fatalf("expected 2 Finding nodes, got %d", len(findingNodes))
	}

	// Verify properties on first finding.
	node := findingNodes[0]
	if node.Properties["severity"] == nil {
		t.Error("Finding node missing severity property")
	}
	if node.Properties["description"] == nil {
		t.Error("Finding node missing description property")
	}
	if node.Properties["file_path"] == nil {
		t.Error("Finding node missing file_path property")
	}
	if node.Properties["line"] == nil {
		t.Error("Finding node missing line property")
	}
}

// SC-2: ABOUT relationships link findings to :File nodes.
func TestIndexFindings_AboutRelationship(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	findings := []*orcv1.ReviewRoundFindings{
		{
			TaskId: "TASK-001",
			Round:  1,
			Issues: []*orcv1.ReviewFinding{
				{
					Severity:    "high",
					File:        strPtr("internal/handler.go"),
					Line:        int32Ptr(42),
					Description: "Issue found",
					AgentId:     strPtr("reviewer"),
				},
			},
		},
	}

	err := idx.IndexFindings(context.Background(), "TASK-001", findings)
	if err != nil {
		t.Fatalf("IndexFindings: %v", err)
	}

	aboutRels := mock.relsOfType("ABOUT")
	if len(aboutRels) == 0 {
		t.Fatal("expected ABOUT relationship from Finding to File")
	}
}

// SC-2: FOUND_BY relationships link findings to reviewer agent.
func TestIndexFindings_FoundByRelationship(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	findings := []*orcv1.ReviewRoundFindings{
		{
			TaskId: "TASK-001",
			Round:  1,
			Issues: []*orcv1.ReviewFinding{
				{
					Severity:    "high",
					File:        strPtr("internal/handler.go"),
					Description: "Issue found",
					AgentId:     strPtr("security-reviewer"),
				},
			},
		},
	}

	err := idx.IndexFindings(context.Background(), "TASK-001", findings)
	if err != nil {
		t.Fatalf("IndexFindings: %v", err)
	}

	foundByRels := mock.relsOfType("FOUND_BY")
	if len(foundByRels) == 0 {
		t.Fatal("expected FOUND_BY relationship from Finding to reviewer agent")
	}
}

// SC-2 error path: No findings → skip with no error.
func TestIndexFindings_NoFindings(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexFindings(context.Background(), "TASK-001", nil)
	if err != nil {
		t.Fatalf("expected no error for nil findings, got: %v", err)
	}
	if len(mock.nodesWithLabel("Finding")) != 0 {
		t.Error("expected no Finding nodes for nil findings")
	}

	// Also test with empty slice.
	err = idx.IndexFindings(context.Background(), "TASK-002", []*orcv1.ReviewRoundFindings{})
	if err != nil {
		t.Fatalf("expected no error for empty findings, got: %v", err)
	}
}

// Edge case: Findings referencing non-existent files auto-create :File nodes.
func TestIndexFindings_UnknownFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	findings := []*orcv1.ReviewRoundFindings{
		{
			TaskId: "TASK-001",
			Round:  1,
			Issues: []*orcv1.ReviewFinding{
				{
					Severity:    "low",
					File:        strPtr("nonexistent/path/file.go"),
					Description: "Issue in unknown file",
				},
			},
		},
	}

	err := idx.IndexFindings(context.Background(), "TASK-001", findings)
	if err != nil {
		t.Fatalf("IndexFindings: %v", err)
	}

	// Finding node should be created.
	if len(mock.nodesWithLabel("Finding")) != 1 {
		t.Error("expected Finding node even for unknown file")
	}

	// ABOUT relationship should still be created (auto-creating File node).
	aboutRels := mock.relsOfType("ABOUT")
	if len(aboutRels) == 0 {
		t.Error("expected ABOUT relationship even for unknown file reference")
	}
}

// Failure mode: Graph store failure propagates error.
func TestIndexFindings_GraphStoreFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("graph unavailable")
	idx := NewIndexer(mock)

	findings := []*orcv1.ReviewRoundFindings{
		{
			TaskId: "TASK-001",
			Round:  1,
			Issues: []*orcv1.ReviewFinding{
				{
					Severity:    "high",
					File:        strPtr("internal/foo.go"),
					Description: "Issue",
				},
			},
		},
	}

	err := idx.IndexFindings(context.Background(), "TASK-001", findings)
	if err == nil {
		t.Fatal("expected error when graph store fails")
	}
}
