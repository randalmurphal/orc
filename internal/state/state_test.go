package state

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	s := New("TASK-001")

	if s.TaskID != "TASK-001" {
		t.Errorf("TaskID = %s, want TASK-001", s.TaskID)
	}

	if s.Status != StatusPending {
		t.Errorf("Status = %s, want %s", s.Status, StatusPending)
	}

	if s.Phases == nil {
		t.Error("Phases map is nil")
	}

	if s.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
}

func TestStartPhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")

	if s.CurrentPhase != "implement" {
		t.Errorf("CurrentPhase = %s, want implement", s.CurrentPhase)
	}

	if s.Status != StatusRunning {
		t.Errorf("Status = %s, want %s", s.Status, StatusRunning)
	}

	ps := s.Phases["implement"]
	if ps == nil {
		t.Fatal("Phase state is nil")
	}

	if ps.Status != StatusRunning {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusRunning)
	}
}

func TestCompletePhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")
	s.CompletePhase("implement", "abc123")

	ps := s.Phases["implement"]
	if ps.Status != StatusCompleted {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusCompleted)
	}

	if ps.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", ps.CommitSHA)
	}

	if ps.CompletedAt == nil {
		t.Error("CompletedAt is nil")
	}
}

func TestFailPhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")

	testErr := &testError{"test error"}
	s.FailPhase("implement", testErr)

	if s.Status != StatusFailed {
		t.Errorf("State status = %s, want %s", s.Status, StatusFailed)
	}

	ps := s.Phases["implement"]
	if ps.Status != StatusFailed {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusFailed)
	}

	if s.Error != "test error" {
		t.Errorf("Error = %s, want 'test error'", s.Error)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestIncrementIteration(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")

	s.IncrementIteration()
	if s.CurrentIteration != 1 {
		t.Errorf("CurrentIteration = %d, want 1", s.CurrentIteration)
	}

	s.IncrementIteration()
	if s.CurrentIteration != 2 {
		t.Errorf("CurrentIteration = %d, want 2", s.CurrentIteration)
	}

	ps := s.Phases["implement"]
	if ps.Iterations != 2 {
		t.Errorf("Phase iterations = %d, want 2", ps.Iterations)
	}
}

func TestAddTokens(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")

	s.AddTokens(100, 50)
	if s.Tokens.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", s.Tokens.InputTokens)
	}
	if s.Tokens.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", s.Tokens.OutputTokens)
	}
	if s.Tokens.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", s.Tokens.TotalTokens)
	}

	s.AddTokens(200, 100)
	if s.Tokens.TotalTokens != 450 {
		t.Errorf("TotalTokens = %d, want 450", s.Tokens.TotalTokens)
	}
}

func TestRecordGateDecision(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("spec")

	s.RecordGateDecision("spec", "human", true, "looks good")

	if len(s.Gates) != 1 {
		t.Fatalf("len(Gates) = %d, want 1", len(s.Gates))
	}

	gate := s.Gates[0]
	if gate.Phase != "spec" {
		t.Errorf("Phase = %s, want spec", gate.Phase)
	}
	if gate.GateType != "human" {
		t.Errorf("GateType = %s, want human", gate.GateType)
	}
	if !gate.Approved {
		t.Error("Approved = false, want true")
	}
	if gate.Reason != "looks good" {
		t.Errorf("Reason = %s, want 'looks good'", gate.Reason)
	}
}

func TestComplete(t *testing.T) {
	s := New("TASK-001")
	s.Complete()

	if s.Status != StatusCompleted {
		t.Errorf("Status = %s, want %s", s.Status, StatusCompleted)
	}

	if s.CompletedAt == nil {
		t.Error("CompletedAt is nil")
	}
}

func TestGetResumePhase(t *testing.T) {
	s := New("TASK-001")

	// No phases - return empty
	if resume := s.GetResumePhase(); resume != "" {
		t.Errorf("GetResumePhase() = %s, want empty", resume)
	}

	// Add completed and interrupted phases
	s.Phases["spec"] = &PhaseState{Status: StatusCompleted}
	s.Phases["implement"] = &PhaseState{Status: StatusInterrupted}

	resume := s.GetResumePhase()
	if resume != "implement" {
		t.Errorf("GetResumePhase() = %s, want implement", resume)
	}
}

func TestIsPhaseCompleted(t *testing.T) {
	s := New("TASK-001")

	// Not started - false
	if s.IsPhaseCompleted("spec") {
		t.Error("IsPhaseCompleted() = true for missing phase")
	}

	// Running - false
	s.Phases["spec"] = &PhaseState{Status: StatusRunning}
	if s.IsPhaseCompleted("spec") {
		t.Error("IsPhaseCompleted() = true for running phase")
	}

	// Completed - true
	s.Phases["spec"].Status = StatusCompleted
	if !s.IsPhaseCompleted("spec") {
		t.Error("IsPhaseCompleted() = false for completed phase")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks/TASK-001", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create and save state
	s := New("TASK-001")
	s.StartPhase("implement")
	s.AddTokens(100, 50)

	err = s.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load state
	loaded, err := Load("TASK-001")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.TaskID != s.TaskID {
		t.Errorf("loaded TaskID = %s, want %s", loaded.TaskID, s.TaskID)
	}

	if loaded.CurrentPhase != s.CurrentPhase {
		t.Errorf("loaded CurrentPhase = %s, want %s", loaded.CurrentPhase, s.CurrentPhase)
	}

	if loaded.Tokens.TotalTokens != s.Tokens.TotalTokens {
		t.Errorf("loaded TotalTokens = %d, want %d", loaded.Tokens.TotalTokens, s.Tokens.TotalTokens)
	}
}
