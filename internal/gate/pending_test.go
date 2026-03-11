package gate

import (
	"testing"
	"time"
)

func TestPendingDecisionStore(t *testing.T) {
	store := NewPendingDecisionStore()

	// Test Add and Get
	decision := &PendingDecision{
		ProjectID:   "proj-001",
		DecisionID:  "gate_TASK-001_review",
		TaskID:      "TASK-001",
		TaskTitle:   "Test Task",
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve?",
		Context:     "Test context",
		RequestedAt: time.Now(),
	}

	if err := store.Add(decision); err != nil {
		t.Fatalf("add decision: %v", err)
	}

	retrieved, ok := store.Get("proj-001", "gate_TASK-001_review")
	if !ok {
		t.Fatal("expected decision to be found")
	}

	if retrieved.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", retrieved.TaskID)
	}

	// Test Remove
	store.Remove("proj-001", "gate_TASK-001_review")
	_, ok = store.Get("proj-001", "gate_TASK-001_review")
	if ok {
		t.Fatal("expected decision to be removed")
	}
}

func TestPendingDecisionStore_List(t *testing.T) {
	store := NewPendingDecisionStore()

	// Add multiple decisions
	decision1 := &PendingDecision{
		ProjectID:  "proj-001",
		DecisionID: "gate_TASK-001_review",
		TaskID:     "TASK-001",
		Phase:      "review",
	}
	decision2 := &PendingDecision{
		ProjectID:  "proj-002",
		DecisionID: "gate_TASK-002_implement",
		TaskID:     "TASK-002",
		Phase:      "implement",
	}

	if err := store.Add(decision1); err != nil {
		t.Fatalf("add decision1: %v", err)
	}
	if err := store.Add(decision2); err != nil {
		t.Fatalf("add decision2: %v", err)
	}

	decisions := store.List("proj-001")
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(decisions))
	}

	unscoped := store.List("")
	if len(unscoped) != 0 {
		t.Errorf("expected 0 unscoped decisions, got %d", len(unscoped))
	}
}

func TestPendingDecisionStore_IsolatesProjectsWithSameDecisionID(t *testing.T) {
	t.Parallel()

	store := NewPendingDecisionStore()
	alpha := &PendingDecision{
		ProjectID:  "proj-alpha",
		DecisionID: "gate_TASK-001_review",
		TaskID:     "TASK-001",
	}
	beta := &PendingDecision{
		ProjectID:  "proj-beta",
		DecisionID: "gate_TASK-001_review",
		TaskID:     "TASK-999",
	}

	if err := store.Add(alpha); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if err := store.Add(beta); err != nil {
		t.Fatalf("add beta: %v", err)
	}

	alphaDecision, ok := store.Get("proj-alpha", "gate_TASK-001_review")
	if !ok {
		t.Fatal("expected alpha decision")
	}
	if alphaDecision.TaskID != "TASK-001" {
		t.Fatalf("alpha task id = %s, want TASK-001", alphaDecision.TaskID)
	}

	betaDecision, ok := store.Get("proj-beta", "gate_TASK-001_review")
	if !ok {
		t.Fatal("expected beta decision")
	}
	if betaDecision.TaskID != "TASK-999" {
		t.Fatalf("beta task id = %s, want TASK-999", betaDecision.TaskID)
	}

	store.Remove("proj-alpha", "gate_TASK-001_review")
	if _, ok := store.Get("proj-alpha", "gate_TASK-001_review"); ok {
		t.Fatal("expected alpha decision removed")
	}
	if _, ok := store.Get("proj-beta", "gate_TASK-001_review"); !ok {
		t.Fatal("expected beta decision to remain")
	}
}

func TestPendingDecisionStore_Concurrent(t *testing.T) {
	store := NewPendingDecisionStore()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			decision := &PendingDecision{
				ProjectID:  "proj-001",
				DecisionID: string(rune('a' + idx)),
				TaskID:     string(rune('a' + idx)),
			}
			if err := store.Add(decision); err != nil {
				t.Errorf("add decision: %v", err)
			}
			_, _ = store.Get("proj-001", string(rune('a'+idx)))
			store.Remove("proj-001", string(rune('a'+idx)))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPendingDecisionStoreAddRejectsInvalidDecisions(t *testing.T) {
	t.Parallel()

	store := NewPendingDecisionStore()

	testCases := []struct {
		name     string
		decision *PendingDecision
	}{
		{
			name:     "nil decision",
			decision: nil,
		},
		{
			name: "missing decision id",
			decision: &PendingDecision{
				ProjectID: "proj-001",
			},
		},
		{
			name: "missing project id",
			decision: &PendingDecision{
				DecisionID: "gate_TASK-001_review",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := store.Add(testCase.decision); err == nil {
				t.Fatal("expected add to fail")
			}
		})
	}
}
