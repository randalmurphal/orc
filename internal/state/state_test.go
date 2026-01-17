package state

import (
	"testing"
	"time"
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

	s.AddTokens(100, 50, 10, 20)
	if s.Tokens.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", s.Tokens.InputTokens)
	}
	if s.Tokens.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", s.Tokens.OutputTokens)
	}
	if s.Tokens.CacheCreationInputTokens != 10 {
		t.Errorf("CacheCreationInputTokens = %d, want 10", s.Tokens.CacheCreationInputTokens)
	}
	if s.Tokens.CacheReadInputTokens != 20 {
		t.Errorf("CacheReadInputTokens = %d, want 20", s.Tokens.CacheReadInputTokens)
	}
	if s.Tokens.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", s.Tokens.TotalTokens)
	}

	s.AddTokens(200, 100, 5, 15)
	if s.Tokens.TotalTokens != 450 {
		t.Errorf("TotalTokens = %d, want 450", s.Tokens.TotalTokens)
	}
	if s.Tokens.CacheCreationInputTokens != 15 {
		t.Errorf("CacheCreationInputTokens = %d, want 15", s.Tokens.CacheCreationInputTokens)
	}
	if s.Tokens.CacheReadInputTokens != 35 {
		t.Errorf("CacheReadInputTokens = %d, want 35", s.Tokens.CacheReadInputTokens)
	}

	// Check phase tokens are tracked too
	if s.Phases["implement"].Tokens.CacheReadInputTokens != 35 {
		t.Errorf("Phase CacheReadInputTokens = %d, want 35", s.Phases["implement"].Tokens.CacheReadInputTokens)
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

func TestReset(t *testing.T) {
	s := New("TASK-001")

	// Set up a task with various state
	s.StartPhase("spec")
	s.CompletePhase("spec", "abc123")
	s.StartPhase("implement")
	s.FailPhase("implement", &testError{"implementation failed"})
	s.AddTokens(1000, 500, 0, 0)
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

func TestIsPhaseCompleted_IncludesSkipped(t *testing.T) {
	s := New("TASK-001")

	// Set up phases
	s.StartPhase("spec")
	s.CompletePhase("spec", "abc123")
	s.StartPhase("research")
	s.SkipPhase("research", "already have research")
	s.Phases["implement"] = &PhaseState{Status: StatusPending}

	tests := []struct {
		phaseID string
		want    bool
	}{
		{"spec", true},       // Completed
		{"research", true},   // Skipped - should also be considered "completed" (done)
		{"implement", false}, // Pending
		{"unknown", false},   // Not in map
	}

	for _, tt := range tests {
		t.Run(tt.phaseID, func(t *testing.T) {
			got := s.IsPhaseCompleted(tt.phaseID)
			if got != tt.want {
				t.Errorf("IsPhaseCompleted(%s) = %v, want %v", tt.phaseID, got, tt.want)
			}
		})
	}
}

func TestIsPhaseSkipped(t *testing.T) {
	s := New("TASK-001")

	s.StartPhase("spec")
	s.CompletePhase("spec", "abc123")
	s.SkipPhase("research", "artifact exists")

	if s.IsPhaseSkipped("spec") {
		t.Error("IsPhaseSkipped(spec) should be false for completed phase")
	}
	if !s.IsPhaseSkipped("research") {
		t.Error("IsPhaseSkipped(research) should be true for skipped phase")
	}
	if s.IsPhaseSkipped("unknown") {
		t.Error("IsPhaseSkipped(unknown) should be false for unknown phase")
	}
}

func TestGetSkipReason(t *testing.T) {
	s := New("TASK-001")

	// Phase with skip reason
	s.SkipPhase("spec", "artifact exists: spec.md")

	reason := s.GetSkipReason("spec")
	if reason != "artifact exists: spec.md" {
		t.Errorf("GetSkipReason(spec) = %q, want %q", reason, "artifact exists: spec.md")
	}

	// Completed phase has no skip reason
	s.StartPhase("implement")
	s.CompletePhase("implement", "abc123")
	if s.GetSkipReason("implement") != "" {
		t.Error("GetSkipReason(implement) should be empty for completed phase")
	}

	// Unknown phase
	if s.GetSkipReason("unknown") != "" {
		t.Error("GetSkipReason(unknown) should be empty for unknown phase")
	}
}

func TestElapsed(t *testing.T) {
	// Test 1: Zero StartedAt should return 0 duration
	// This was the original bug - time.Since(zero time) returned ~292 years
	s := &State{}
	if s.StartedAt.IsZero() == false {
		t.Error("StartedAt should be zero for empty State")
	}
	elapsed := s.Elapsed()
	if elapsed != 0 {
		t.Errorf("Elapsed() with zero StartedAt = %v, want 0", elapsed)
	}

	// Test 2: Valid StartedAt should return positive duration
	s2 := New("TASK-001")
	// New() sets StartedAt to now, so Elapsed should be small but positive
	elapsed2 := s2.Elapsed()
	if elapsed2 < 0 {
		t.Errorf("Elapsed() = %v, want non-negative", elapsed2)
	}
	// Should be less than 1 second for freshly created state
	if elapsed2 > time.Second {
		t.Errorf("Elapsed() = %v, unexpectedly large for fresh state", elapsed2)
	}

	// Test 3: Manually set StartedAt in the past
	s3 := &State{
		StartedAt: time.Now().Add(-5 * time.Minute),
	}
	elapsed3 := s3.Elapsed()
	// Should be approximately 5 minutes (allow some tolerance)
	if elapsed3 < 4*time.Minute || elapsed3 > 6*time.Minute {
		t.Errorf("Elapsed() = %v, want ~5m", elapsed3)
	}
}

func TestResetPhasesFrom(t *testing.T) {
	// Create a state with multiple phases in various states
	s := New("TASK-001")

	// Set up phases: spec (completed), implement (completed), review (failed), test (pending), docs (pending)
	s.Phases["spec"] = &PhaseState{
		Status:    StatusCompleted,
		CommitSHA: "abc123",
	}
	s.Phases["implement"] = &PhaseState{
		Status:    StatusCompleted,
		CommitSHA: "def456",
	}
	now := time.Now()
	s.Phases["review"] = &PhaseState{
		Status:      StatusFailed,
		CompletedAt: &now,
		Error:       "review failed",
	}
	s.Phases["test"] = &PhaseState{Status: StatusPending}
	s.Phases["docs"] = &PhaseState{Status: StatusPending}

	// Set error and retry context
	s.Status = StatusFailed
	s.Error = "review phase failed"
	s.SetRetryContext("review", "implement", "retry from implement", "output", 1)

	// Define phase order
	allPhases := []string{"spec", "implement", "review", "test", "docs"}

	// Reset from "implement" onward
	s.ResetPhasesFrom("implement", allPhases)

	// Verify spec is still completed (before reset point)
	if s.Phases["spec"].Status != StatusCompleted {
		t.Errorf("spec status = %s, want %s (should be preserved)", s.Phases["spec"].Status, StatusCompleted)
	}
	if s.Phases["spec"].CommitSHA != "abc123" {
		t.Errorf("spec CommitSHA = %s, want abc123 (should be preserved)", s.Phases["spec"].CommitSHA)
	}

	// Verify implement and later phases are reset to pending
	for _, phaseID := range []string{"implement", "review", "test", "docs"} {
		if s.Phases[phaseID].Status != StatusPending {
			t.Errorf("%s status = %s, want %s", phaseID, s.Phases[phaseID].Status, StatusPending)
		}
		if s.Phases[phaseID].Error != "" {
			t.Errorf("%s Error = %s, want empty", phaseID, s.Phases[phaseID].Error)
		}
	}

	// Verify task-level error is cleared
	if s.Error != "" {
		t.Errorf("Error = %s, want empty", s.Error)
	}

	// Verify status is reset to pending
	if s.Status != StatusPending {
		t.Errorf("Status = %s, want %s", s.Status, StatusPending)
	}

	// Verify retry context is cleared
	if s.HasRetryContext() {
		t.Error("RetryContext should be cleared")
	}
}

func TestResetPhasesFrom_FirstPhase(t *testing.T) {
	s := New("TASK-001")

	// Set up all phases as completed
	s.Phases["spec"] = &PhaseState{Status: StatusCompleted}
	s.Phases["implement"] = &PhaseState{Status: StatusCompleted}
	s.Phases["test"] = &PhaseState{Status: StatusCompleted}

	allPhases := []string{"spec", "implement", "test"}

	// Reset from first phase
	s.ResetPhasesFrom("spec", allPhases)

	// All phases should be reset
	for _, phaseID := range allPhases {
		if s.Phases[phaseID].Status != StatusPending {
			t.Errorf("%s status = %s, want %s", phaseID, s.Phases[phaseID].Status, StatusPending)
		}
	}
}

func TestResetPhasesFrom_LastPhase(t *testing.T) {
	s := New("TASK-001")

	// Set up phases
	s.Phases["spec"] = &PhaseState{Status: StatusCompleted}
	s.Phases["implement"] = &PhaseState{Status: StatusCompleted}
	s.Phases["test"] = &PhaseState{Status: StatusFailed, Error: "tests failed"}

	allPhases := []string{"spec", "implement", "test"}

	// Reset only from last phase
	s.ResetPhasesFrom("test", allPhases)

	// Earlier phases should be preserved
	if s.Phases["spec"].Status != StatusCompleted {
		t.Errorf("spec status = %s, want %s", s.Phases["spec"].Status, StatusCompleted)
	}
	if s.Phases["implement"].Status != StatusCompleted {
		t.Errorf("implement status = %s, want %s", s.Phases["implement"].Status, StatusCompleted)
	}

	// Only test phase should be reset
	if s.Phases["test"].Status != StatusPending {
		t.Errorf("test status = %s, want %s", s.Phases["test"].Status, StatusPending)
	}
}

func TestStartExecution_SetsStartedAt(t *testing.T) {
	// Test 1: StartExecution on fresh state (zero StartedAt) should set StartedAt
	// This is the key bug fix - loaded states have zero StartedAt
	s := &State{
		TaskID: "TASK-001",
		Phases: make(map[string]*PhaseState),
	}
	if !s.StartedAt.IsZero() {
		t.Error("Pre-condition: StartedAt should be zero for loaded state simulation")
	}

	beforeExec := time.Now()
	s.StartExecution(12345, "testhost")
	afterExec := time.Now()

	// Verify StartedAt was set
	if s.StartedAt.IsZero() {
		t.Fatal("StartedAt should not be zero after StartExecution")
	}

	// Verify StartedAt is within expected range
	if s.StartedAt.Before(beforeExec) || s.StartedAt.After(afterExec) {
		t.Errorf("StartedAt = %v, want between %v and %v", s.StartedAt, beforeExec, afterExec)
	}

	// Verify Elapsed() now returns a sensible value
	elapsed := s.Elapsed()
	if elapsed < 0 || elapsed > time.Second {
		t.Errorf("Elapsed() = %v, want small positive duration", elapsed)
	}

	// Verify Execution info was also set
	if s.Execution == nil {
		t.Fatal("Execution should not be nil after StartExecution")
	}
	if s.Execution.PID != 12345 {
		t.Errorf("Execution.PID = %d, want 12345", s.Execution.PID)
	}
	if s.Execution.Hostname != "testhost" {
		t.Errorf("Execution.Hostname = %s, want testhost", s.Execution.Hostname)
	}

	// Test 2: StartExecution on resumed state (existing StartedAt) should preserve original
	// This ensures we don't reset StartedAt when resuming a paused task
	originalStartTime := time.Now().Add(-10 * time.Minute)
	s2 := &State{
		TaskID:    "TASK-002",
		StartedAt: originalStartTime,
		Phases:    make(map[string]*PhaseState),
	}

	s2.StartExecution(67890, "otherhost")

	// Verify original StartedAt was preserved
	if !s2.StartedAt.Equal(originalStartTime) {
		t.Errorf("StartedAt = %v, want %v (should preserve original for resumed tasks)", s2.StartedAt, originalStartTime)
	}

	// Verify Elapsed() returns the full time since original start
	elapsed2 := s2.Elapsed()
	if elapsed2 < 9*time.Minute || elapsed2 > 11*time.Minute {
		t.Errorf("Elapsed() = %v, want ~10m (should reflect original start time)", elapsed2)
	}
}
