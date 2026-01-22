package gate

import (
	"testing"
	"time"
)

func TestPendingDecisionStore(t *testing.T) {
	store := NewPendingDecisionStore()

	// Test Add and Get
	decision := &PendingDecision{
		DecisionID:  "gate_TASK-001_review",
		TaskID:      "TASK-001",
		TaskTitle:   "Test Task",
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve?",
		Context:     "Test context",
		RequestedAt: time.Now(),
	}

	store.Add(decision)

	retrieved, ok := store.Get("gate_TASK-001_review")
	if !ok {
		t.Fatal("expected decision to be found")
	}

	if retrieved.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", retrieved.TaskID)
	}

	// Test Remove
	store.Remove("gate_TASK-001_review")
	_, ok = store.Get("gate_TASK-001_review")
	if ok {
		t.Fatal("expected decision to be removed")
	}
}

func TestPendingDecisionStore_List(t *testing.T) {
	store := NewPendingDecisionStore()

	// Add multiple decisions
	decision1 := &PendingDecision{
		DecisionID: "gate_TASK-001_review",
		TaskID:     "TASK-001",
		Phase:      "review",
	}
	decision2 := &PendingDecision{
		DecisionID: "gate_TASK-002_implement",
		TaskID:     "TASK-002",
		Phase:      "implement",
	}

	store.Add(decision1)
	store.Add(decision2)

	decisions := store.List()
	if len(decisions) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(decisions))
	}
}

func TestPendingDecisionStore_Concurrent(t *testing.T) {
	store := NewPendingDecisionStore()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			decision := &PendingDecision{
				DecisionID: string(rune(idx)),
				TaskID:     string(rune(idx)),
			}
			store.Add(decision)
			_, _ = store.Get(string(rune(idx)))
			store.Remove(string(rune(idx)))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
