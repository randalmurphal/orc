package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
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
	taskDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir, "TASK-001")

	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create a task.yaml so task.ExistsIn returns true
	tsk := task.New("TASK-001", "Test task")
	tsk.SaveTo(taskDir)

	// Create and save state
	s := New("TASK-001")
	s.StartPhase("implement")
	s.AddTokens(100, 50)

	err = s.SaveTo(taskDir)
	if err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load state
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
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

func TestInterruptPhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")
	s.InterruptPhase("implement")

	if s.Status != StatusInterrupted {
		t.Errorf("Status = %s, want %s", s.Status, StatusInterrupted)
	}

	ps := s.Phases["implement"]
	if ps.Status != StatusInterrupted {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusInterrupted)
	}

	if ps.InterruptedAt == nil {
		t.Error("InterruptedAt is nil")
	}
}

func TestResetPhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("implement")
	s.CompletePhase("implement", "abc123")

	// Reset the phase
	s.ResetPhase("implement")

	ps := s.Phases["implement"]
	if ps.Status != StatusPending {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusPending)
	}

	// CompletedAt should be cleared
	if ps.CompletedAt != nil {
		t.Error("CompletedAt should be nil after reset")
	}

	// Error should be cleared
	if ps.Error != "" {
		t.Errorf("Error = %s, want empty", ps.Error)
	}
}

func TestRetryContext(t *testing.T) {
	s := New("TASK-001")

	// Initially no retry context
	if s.HasRetryContext() {
		t.Error("HasRetryContext() = true, want false initially")
	}

	if s.GetRetryContext() != nil {
		t.Error("GetRetryContext() != nil, want nil initially")
	}

	// Set retry context
	s.SetRetryContext("test", "implement", "test failure", "output here", 1)

	if !s.HasRetryContext() {
		t.Error("HasRetryContext() = false after SetRetryContext")
	}

	rc := s.GetRetryContext()
	if rc == nil {
		t.Fatal("GetRetryContext() = nil after SetRetryContext")
	}

	if rc.FromPhase != "test" {
		t.Errorf("FromPhase = %s, want test", rc.FromPhase)
	}

	if rc.ToPhase != "implement" {
		t.Errorf("ToPhase = %s, want implement", rc.ToPhase)
	}

	if rc.Reason != "test failure" {
		t.Errorf("Reason = %s, want 'test failure'", rc.Reason)
	}

	if rc.FailureOutput != "output here" {
		t.Errorf("FailureOutput = %s, want 'output here'", rc.FailureOutput)
	}

	if rc.Attempt != 1 {
		t.Errorf("Attempt = %d, want 1", rc.Attempt)
	}

	// Set context file
	s.SetRetryContextFile("/path/to/context.md")
	rc = s.GetRetryContext()
	if rc.ContextFile != "/path/to/context.md" {
		t.Errorf("ContextFile = %s, want /path/to/context.md", rc.ContextFile)
	}

	// Clear retry context
	s.ClearRetryContext()

	if s.HasRetryContext() {
		t.Error("HasRetryContext() = true after ClearRetryContext")
	}

	if s.GetRetryContext() != nil {
		t.Error("GetRetryContext() != nil after ClearRetryContext")
	}
}

func TestSkipPhase(t *testing.T) {
	s := New("TASK-001")
	s.StartPhase("design")

	s.SkipPhase("design", "already have design")

	ps := s.Phases["design"]
	if ps.Status != StatusSkipped {
		t.Errorf("Phase status = %s, want %s", ps.Status, StatusSkipped)
	}

	// Skip reason is stored in Error field with "skipped: " prefix
	expectedReason := "skipped: already have design"
	if ps.Error != expectedReason {
		t.Errorf("Error = %s, want %s", ps.Error, expectedReason)
	}
}

func TestLoadNonExistentTask(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)

	// Create tasks directory but not the task
	os.MkdirAll(tasksDir, 0755)

	// Try to load non-existent task
	_, err := LoadFrom(tmpDir, "TASK-999")
	if err == nil {
		t.Error("LoadFrom() should return error for non-existent task")
	}
}

func TestReset(t *testing.T) {
	s := New("TASK-001")

	// Set up a task with various state
	s.StartPhase("spec")
	s.CompletePhase("spec", "abc123")
	s.StartPhase("implement")
	s.FailPhase("implement", &testError{"implementation failed"})
	s.AddTokens(1000, 500)
	s.RecordGateDecision("spec", "ai", true, "approved")
	s.SetRetryContext("implement", "spec", "retry", "output", 1)
	s.StartExecution(12345, "testhost")

	// Verify pre-conditions
	if s.Status != StatusFailed {
		t.Errorf("Pre-reset status = %s, want %s", s.Status, StatusFailed)
	}
	if s.CurrentPhase != "implement" {
		t.Errorf("Pre-reset CurrentPhase = %s, want implement", s.CurrentPhase)
	}
	if s.Execution == nil {
		t.Error("Pre-reset Execution should not be nil")
	}

	// Reset the state
	s.Reset()

	// Verify state is reset
	if s.Status != StatusPending {
		t.Errorf("Status = %s, want %s", s.Status, StatusPending)
	}

	if s.CurrentPhase != "" {
		t.Errorf("CurrentPhase = %s, want empty", s.CurrentPhase)
	}

	if s.CurrentIteration != 0 {
		t.Errorf("CurrentIteration = %d, want 0", s.CurrentIteration)
	}

	if s.CompletedAt != nil {
		t.Error("CompletedAt should be nil after reset")
	}

	if s.Error != "" {
		t.Errorf("Error = %s, want empty", s.Error)
	}

	if s.RetryContext != nil {
		t.Error("RetryContext should be nil after reset")
	}

	if s.Execution != nil {
		t.Error("Execution should be nil after reset")
	}

	if s.Session != nil {
		t.Error("Session should be nil after reset")
	}

	if s.Gates != nil {
		t.Error("Gates should be nil after reset")
	}

	// Verify all phases are reset to pending
	for phaseID, ps := range s.Phases {
		if ps.Status != StatusPending {
			t.Errorf("Phase %s status = %s, want %s", phaseID, ps.Status, StatusPending)
		}
	}

	// Token counts are preserved (historical data)
	if s.Tokens.TotalTokens != 1500 {
		t.Errorf("TotalTokens = %d, want 1500 (should preserve historical data)", s.Tokens.TotalTokens)
	}
}
