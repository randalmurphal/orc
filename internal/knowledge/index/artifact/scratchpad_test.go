package artifact

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
)

// --- SC-7: Scratchpad "decision" entries create :Decision nodes ---

// SC-7: Decision entries become :Decision nodes with source="scratchpad".
func TestIndexScratchpad_DecisionEntries(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:        1,
			TaskID:    "TASK-001",
			PhaseID:   "implement",
			Category:  "decision",
			Content:   "Decided to use table-driven tests for the validator",
			Attempt:   1,
			CreatedAt: time.Now(),
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	decNodes := mock.nodesWithLabel("Decision")
	if len(decNodes) != 1 {
		t.Fatalf("expected 1 Decision node, got %d", len(decNodes))
	}

	node := decNodes[0]
	if node.Properties["source"] != "scratchpad" {
		t.Errorf("expected source='scratchpad', got %v", node.Properties["source"])
	}
	if node.Properties["task_id"] != "TASK-001" {
		t.Errorf("expected task_id=TASK-001, got %v", node.Properties["task_id"])
	}
}

// SC-7: Decision entries linked via FROM_TASK.
func TestIndexScratchpad_DecisionLinkedToTask(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:       1,
			TaskID:   "TASK-001",
			PhaseID:  "implement",
			Category: "decision",
			Content:  "Decided to use bcrypt",
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	fromTaskRels := mock.relsOfType("FROM_TASK")
	if len(fromTaskRels) == 0 {
		t.Fatal("expected FROM_TASK relationship from scratchpad Decision")
	}
}

// --- SC-8: Scratchpad "observation" entries create :Observation nodes ---

// SC-8: Observation entries become :Observation nodes with content, phase_id, task_id.
func TestIndexScratchpad_ObservationEntries(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:        1,
			TaskID:    "TASK-001",
			PhaseID:   "review",
			Category:  "observation",
			Content:   "The auth module in internal/auth/handler.go has high cyclomatic complexity",
			Attempt:   1,
			CreatedAt: time.Now(),
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	obsNodes := mock.nodesWithLabel("Observation")
	if len(obsNodes) != 1 {
		t.Fatalf("expected 1 Observation node, got %d", len(obsNodes))
	}

	node := obsNodes[0]
	if node.Properties["content"] == nil {
		t.Error("Observation node missing content property")
	}
	if node.Properties["phase_id"] != "review" {
		t.Errorf("expected phase_id='review', got %v", node.Properties["phase_id"])
	}
	if node.Properties["task_id"] != "TASK-001" {
		t.Errorf("expected task_id=TASK-001, got %v", node.Properties["task_id"])
	}
}

// SC-8: Observations linked to mentioned files via ABOUT.
func TestIndexScratchpad_ObservationAboutFiles(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:       1,
			TaskID:   "TASK-001",
			PhaseID:  "review",
			Category: "observation",
			Content:  "Found complexity issues in internal/auth/handler.go and internal/api/server.go",
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	aboutRels := mock.relsOfType("ABOUT")
	if len(aboutRels) < 2 {
		t.Errorf("expected at least 2 ABOUT relationships (one per mentioned file), got %d", len(aboutRels))
	}
}

// --- SC-9: Scratchpad warning/blocker/todo handling ---

// SC-9: Warning entries create :Warning nodes linked via ABOUT.
func TestIndexScratchpad_WarningEntries(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:       1,
			TaskID:   "TASK-001",
			PhaseID:  "implement",
			Category: "warning",
			Content:  "internal/db/migrations.go has deprecated API usage",
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	warnNodes := mock.nodesWithLabel("Warning")
	if len(warnNodes) != 1 {
		t.Fatalf("expected 1 Warning node, got %d", len(warnNodes))
	}

	aboutRels := mock.relsOfType("ABOUT")
	if len(aboutRels) == 0 {
		t.Error("expected ABOUT relationship from Warning to File")
	}
}

// SC-9: Blocker entries update :File difficulty_score.
func TestIndexScratchpad_BlockerUpdatesDifficulty(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:       1,
			TaskID:   "TASK-001",
			PhaseID:  "implement",
			Category: "blocker",
			Content:  "internal/db/migrations.go is extremely complex and hard to modify",
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	// Blocker should update File difficulty_score via Cypher MERGE.
	calls := mock.allCypherCalls()
	found := false
	for _, c := range calls {
		if strings.Contains(c.query, "difficulty_score") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Cypher query updating difficulty_score for blocker entry")
	}
}

// SC-9: Todo entries are filtered out — produce no nodes.
func TestIndexScratchpad_TodoFiltered(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{
			ID:       1,
			TaskID:   "TASK-001",
			PhaseID:  "implement",
			Category: "todo",
			Content:  "Remember to add integration tests later",
		},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("expected no error for todo entries, got: %v", err)
	}

	// No nodes of any type should be created for todo entries.
	allNodes := mock.nodesWithLabel("Decision")
	allNodes = append(allNodes, mock.nodesWithLabel("Observation")...)
	allNodes = append(allNodes, mock.nodesWithLabel("Warning")...)
	if len(allNodes) != 0 {
		t.Errorf("expected 0 nodes for todo entries, got %d", len(allNodes))
	}
}

// SC-9: Mixed categories — each handled correctly.
func TestIndexScratchpad_MixedCategories(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{ID: 1, TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Use table-driven tests"},
		{ID: 2, TaskID: "TASK-001", PhaseID: "implement", Category: "observation", Content: "internal/foo.go has complex logic"},
		{ID: 3, TaskID: "TASK-001", PhaseID: "implement", Category: "warning", Content: "internal/bar.go uses deprecated API"},
		{ID: 4, TaskID: "TASK-001", PhaseID: "implement", Category: "blocker", Content: "internal/baz.go is hard to modify"},
		{ID: 5, TaskID: "TASK-001", PhaseID: "implement", Category: "todo", Content: "Add more tests later"},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("IndexScratchpad: %v", err)
	}

	// Decision: 1 node
	if len(mock.nodesWithLabel("Decision")) != 1 {
		t.Errorf("expected 1 Decision node, got %d", len(mock.nodesWithLabel("Decision")))
	}
	// Observation: 1 node
	if len(mock.nodesWithLabel("Observation")) != 1 {
		t.Errorf("expected 1 Observation node, got %d", len(mock.nodesWithLabel("Observation")))
	}
	// Warning: 1 node
	if len(mock.nodesWithLabel("Warning")) != 1 {
		t.Errorf("expected 1 Warning node, got %d", len(mock.nodesWithLabel("Warning")))
	}
	// Blocker: no separate node, but Cypher call for difficulty update.
	calls := mock.allCypherCalls()
	hasDifficultyUpdate := false
	for _, c := range calls {
		if strings.Contains(c.query, "difficulty_score") {
			hasDifficultyUpdate = true
			break
		}
	}
	if !hasDifficultyUpdate {
		t.Error("expected difficulty_score update from blocker entry")
	}
	// Todo: no nodes (already verified — Decision+Observation+Warning = 3 total).
}

// Edge case: Empty scratchpad content → entry skipped.
func TestIndexScratchpad_EmptyContent(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{ID: 1, TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: ""},
		{ID: 2, TaskID: "TASK-001", PhaseID: "implement", Category: "observation", Content: ""},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("expected no error for empty content, got: %v", err)
	}

	totalNodes := len(mock.nodesWithLabel("Decision")) + len(mock.nodesWithLabel("Observation"))
	if totalNodes != 0 {
		t.Errorf("expected 0 nodes for empty content entries, got %d", totalNodes)
	}
}

// Edge case: No relevant category entries → skip.
func TestIndexScratchpad_NoEntries(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	err := idx.IndexScratchpad(context.Background(), "TASK-001", nil)
	if err != nil {
		t.Fatalf("expected no error for nil entries, got: %v", err)
	}
}

// Failure mode: Graph store failure propagates error.
func TestIndexScratchpad_GraphStoreFailure(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	mock.createNodeErr = fmt.Errorf("graph unavailable")
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{ID: 1, TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Some decision"},
	}

	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err == nil {
		t.Fatal("expected error when graph store fails")
	}
}

// Failure mode: Malformed scratchpad entry content.
func TestIndexScratchpad_MalformedEntry(t *testing.T) {
	t.Parallel()
	mock := newMockGraphStore()
	idx := NewIndexer(mock)

	entries := []storage.ScratchpadEntry{
		{ID: 1, TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Valid decision"},
		{ID: 2, TaskID: "TASK-001", PhaseID: "implement", Category: "observation", Content: string([]byte{0xFF, 0xFE})}, // Invalid UTF-8
		{ID: 3, TaskID: "TASK-001", PhaseID: "implement", Category: "warning", Content: "Valid warning about internal/foo.go"},
	}

	// Should handle malformed entry gracefully — skip it, index others.
	err := idx.IndexScratchpad(context.Background(), "TASK-001", entries)
	if err != nil {
		t.Fatalf("expected graceful handling of malformed entry, got: %v", err)
	}

	// Valid entries should still be indexed.
	decisionNodes := mock.nodesWithLabel("Decision")
	warningNodes := mock.nodesWithLabel("Warning")
	if len(decisionNodes) != 1 {
		t.Errorf("expected 1 Decision node (from valid entry), got %d", len(decisionNodes))
	}
	if len(warningNodes) != 1 {
		t.Errorf("expected 1 Warning node (from valid entry), got %d", len(warningNodes))
	}
}
