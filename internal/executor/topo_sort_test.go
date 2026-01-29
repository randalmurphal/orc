package executor

import (
	"encoding/json"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

// helper to build a WorkflowPhase with minimal boilerplate
func makePhase(templateID string, seq int, dependsOn []string) *db.WorkflowPhase {
	deps := "[]"
	if len(dependsOn) > 0 {
		b, _ := json.Marshal(dependsOn)
		deps = string(b)
	}
	return &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: templateID,
		Sequence:        seq,
		DependsOn:       deps,
	}
}

// helper to build a WorkflowPhase with loop config
func makePhaseWithLoop(templateID string, seq int, dependsOn []string, loopToPhase string) *db.WorkflowPhase {
	p := makePhase(templateID, seq, dependsOn)
	if loopToPhase != "" {
		cfg := db.LoopConfig{
			Condition:     "has_findings",
			LoopToPhase:   loopToPhase,
			MaxIterations: 3,
		}
		b, _ := json.Marshal(cfg)
		p.LoopConfig = string(b)
	}
	return p
}

// extractIDs returns the PhaseTemplateIDs in order from the sorted result.
func extractIDs(phases []*db.WorkflowPhase) []string {
	ids := make([]string, len(phases))
	for i, p := range phases {
		ids[i] = p.PhaseTemplateID
	}
	return ids
}

// SC-1: Phases with empty depends_on produce identical order to sequence-based sorting
func TestTopoSort_EmptyDependsOn(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("phase-1", 1, nil),
		makePhase("phase-2", 2, nil),
		makePhase("phase-3", 3, nil),
		makePhase("phase-4", 4, nil),
		makePhase("phase-5", 5, nil),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"phase-1", "phase-2", "phase-3", "phase-4", "phase-5"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// SC-1 variant: sequence order preserved even when input is unordered
func TestTopoSort_EmptyDependsOn_UnsortedInput(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("phase-3", 3, nil),
		makePhase("phase-1", 1, nil),
		makePhase("phase-5", 5, nil),
		makePhase("phase-2", 2, nil),
		makePhase("phase-4", 4, nil),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"phase-1", "phase-2", "phase-3", "phase-4", "phase-5"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// SC-2: Linear dependency chain produces correct order regardless of Sequence values
func TestTopoSort_LinearChain(t *testing.T) {
	// A(seq=3) -> B(seq=1) -> C(seq=2)
	// Dependency order: A first, then B, then C
	// Even though B has lowest sequence, it depends on A
	phases := []*db.WorkflowPhase{
		makePhase("A", 3, nil),
		makePhase("B", 1, []string{"A"}),
		makePhase("C", 2, []string{"B"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"A", "B", "C"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// SC-3: Parallel phases ordered by Sequence tiebreaker at same depth level
func TestTopoSort_Parallel(t *testing.T) {
	// A(seq=1) and B(seq=2) are independent, C(seq=3) depends on both
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, nil),
		makePhase("C", 3, []string{"A", "B"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	// A before B (sequence tiebreaker), C after both
	expected := []string{"A", "B", "C"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// SC-3 variant: verify sequence tiebreaker when parallel phases have reversed sequence
func TestTopoSort_Parallel_ReversedSequence(t *testing.T) {
	// B(seq=1) and A(seq=2) are independent, C depends on both
	// B should come first because lower sequence
	phases := []*db.WorkflowPhase{
		makePhase("A", 2, nil),
		makePhase("B", 1, nil),
		makePhase("C", 3, []string{"A", "B"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"B", "A", "C"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// SC-5: Cycle in depends_on returns descriptive error
func TestTopoSort_Cycle(t *testing.T) {
	// A depends on B, B depends on A
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, []string{"B"}),
		makePhase("B", 2, []string{"A"}),
	}

	_, err := topologicalSort(phases)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	// Error should mention cycle and involved phases
	errMsg := err.Error()
	if !containsSubstring(errMsg, "cycle") {
		t.Errorf("error should mention 'cycle', got: %s", errMsg)
	}
}

// SC-5 variant: self-cycle
func TestTopoSort_SelfCycle(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, []string{"A"}),
	}

	_, err := topologicalSort(phases)
	if err == nil {
		t.Fatal("expected error for self-cycle, got nil")
	}
}

// SC-5 variant: longer cycle (A->B->C->A)
func TestTopoSort_LongerCycle(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, []string{"C"}),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"B"}),
	}

	_, err := topologicalSort(phases)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

// SC-6: loop_config and retry_from_phase are NOT treated as dependency edges
func TestTopoSort_IgnoresLoopRetry(t *testing.T) {
	// B has loop_config pointing back to A — this would be a cycle if treated as a dep
	// But loop_config is runtime control flow, not structural dependency
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhaseWithLoop("B", 2, []string{"A"}, "A"), // loop back to A
		makePhase("C", 3, []string{"B"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error (loop_config should be ignored): %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"A", "B", "C"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// Edge case: empty phase slice
func TestTopoSort_Empty(t *testing.T) {
	sorted, err := topologicalSort([]*db.WorkflowPhase{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("expected empty result, got %d phases", len(sorted))
	}
}

// Edge case: nil input
func TestTopoSort_Nil(t *testing.T) {
	sorted, err := topologicalSort(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("expected empty result, got %d phases", len(sorted))
	}
}

// Edge case: single phase
func TestTopoSort_SinglePhase(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("only", 1, nil),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(sorted))
	}
	if sorted[0].PhaseTemplateID != "only" {
		t.Errorf("expected 'only', got %s", sorted[0].PhaseTemplateID)
	}
}

// Edge case: depends_on references ID not in the phase list (graceful degradation)
func TestTopoSort_MissingDependency(t *testing.T) {
	// B depends on "nonexistent" — that dep should be ignored
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"nonexistent"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error for missing dep: %v", err)
	}

	ids := extractIDs(sorted)
	// Both should appear; B's missing dep is a no-op
	if len(ids) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(ids))
	}
	// A before B by sequence
	expected := []string{"A", "B"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// Edge case: mixed — some phases have depends_on, some don't
func TestTopoSort_Mixed(t *testing.T) {
	// A(seq=1) no deps, B(seq=2) no deps, C(seq=3) depends on A, D(seq=4) depends on C
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, nil),
		makePhase("C", 3, []string{"A"}),
		makePhase("D", 4, []string{"C"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)

	// A must come before C, C must come before D
	// B has no constraints except sequence tiebreaker among zero-indegree peers
	posOf := make(map[string]int)
	for i, id := range ids {
		posOf[id] = i
	}

	if posOf["A"] >= posOf["C"] {
		t.Errorf("A must come before C: got A at %d, C at %d", posOf["A"], posOf["C"])
	}
	if posOf["C"] >= posOf["D"] {
		t.Errorf("C must come before D: got C at %d, D at %d", posOf["C"], posOf["D"])
	}
	if len(ids) != 4 {
		t.Errorf("expected 4 phases, got %d", len(ids))
	}
}

// Edge case: duplicate entries in depends_on (should be idempotent)
func TestTopoSort_DuplicateDeps(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A", "A", "A"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	expected := []string{"A", "B"}
	if !slicesEqual(ids, expected) {
		t.Errorf("order mismatch\ngot:  %v\nwant: %v", ids, expected)
	}
}

// Diamond dependency pattern: A -> B, A -> C, B -> D, C -> D
func TestTopoSort_Diamond(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"A"}),
		makePhase("D", 4, []string{"B", "C"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := extractIDs(sorted)
	posOf := make(map[string]int)
	for i, id := range ids {
		posOf[id] = i
	}

	// A before B and C
	if posOf["A"] >= posOf["B"] {
		t.Errorf("A must come before B")
	}
	if posOf["A"] >= posOf["C"] {
		t.Errorf("A must come before C")
	}
	// B and C before D
	if posOf["B"] >= posOf["D"] {
		t.Errorf("B must come before D")
	}
	if posOf["C"] >= posOf["D"] {
		t.Errorf("C must come before D")
	}
	// B before C by sequence tiebreaker
	if posOf["B"] >= posOf["C"] {
		t.Errorf("B(seq=2) should come before C(seq=3) by sequence tiebreaker")
	}
}

// Verify that the sort returns the same pointer objects, not copies
func TestTopoSort_PreservesPointers(t *testing.T) {
	a := makePhase("A", 1, nil)
	b := makePhase("B", 2, []string{"A"})

	sorted, err := topologicalSort([]*db.WorkflowPhase{a, b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sorted[0] != a {
		t.Error("first element should be same pointer as input 'A'")
	}
	if sorted[1] != b {
		t.Error("second element should be same pointer as input 'B'")
	}
}

// Verify output length always matches input length for valid graphs
func TestTopoSort_OutputLengthMatchesInput(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"A"}),
		makePhase("D", 4, []string{"B", "C"}),
		makePhase("E", 5, []string{"D"}),
	}

	sorted, err := topologicalSort(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != len(phases) {
		t.Errorf("output length %d != input length %d", len(sorted), len(phases))
	}
}

// Partial cycle: some phases form a cycle, others don't
func TestTopoSort_PartialCycle(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),        // no cycle
		makePhase("B", 2, []string{"C"}), // cycle: B->C->B
		makePhase("C", 3, []string{"B"}),
	}

	_, err := topologicalSort(phases)
	if err == nil {
		t.Fatal("expected error for partial cycle, got nil")
	}
}

// Test determinism: same input always produces same output
func TestTopoSort_Deterministic(t *testing.T) {
	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, nil),
		makePhase("C", 3, nil),
		makePhase("D", 4, []string{"A", "B"}),
	}

	// Run multiple times to check determinism
	var firstResult []string
	for i := 0; i < 10; i++ {
		sorted, err := topologicalSort(phases)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		ids := extractIDs(sorted)
		if firstResult == nil {
			firstResult = ids
		} else if !slicesEqual(ids, firstResult) {
			t.Fatalf("non-deterministic output on iteration %d\nfirst: %v\ngot:   %v", i, firstResult, ids)
		}
	}
}

// Edge case: malformed JSON in DependsOn returns error instead of silent nil
func TestTopoSort_MalformedDependsOn(t *testing.T) {
	phases := []*db.WorkflowPhase{
		{
			WorkflowID:      "test-wf",
			PhaseTemplateID: "bad-phase",
			Sequence:        1,
			DependsOn:       "not valid json",
		},
	}

	_, err := topologicalSort(phases)
	if err == nil {
		t.Fatal("expected error for malformed depends_on, got nil")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "bad-phase") {
		t.Errorf("error should mention phase ID, got: %s", errMsg)
	}
}

// helpers

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
