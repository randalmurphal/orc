package state

import (
	"os"
	"testing"
	"time"
)

func TestCheckOrphaned_NotRunning(t *testing.T) {
	s := &State{
		Status: StatusCompleted,
	}

	isOrphaned, reason := s.CheckOrphaned()
	if isOrphaned {
		t.Errorf("expected non-running task to not be orphaned, got orphaned with reason: %s", reason)
	}
}

func TestCheckOrphaned_NoExecutionInfo(t *testing.T) {
	s := &State{
		Status:    StatusRunning,
		Execution: nil,
	}

	isOrphaned, reason := s.CheckOrphaned()
	if !isOrphaned {
		t.Error("expected running task without execution info to be orphaned")
	}
	if reason != "no execution info (legacy state or incomplete)" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_DeadPID(t *testing.T) {
	s := &State{
		Status: StatusRunning,
		Execution: &ExecutionInfo{
			PID:           999999999, // Very high PID unlikely to exist
			Hostname:      "test-host",
			StartedAt:     time.Now().Add(-time.Hour),
			LastHeartbeat: time.Now(), // Recent heartbeat
		},
	}

	isOrphaned, reason := s.CheckOrphaned()
	if !isOrphaned {
		t.Error("expected task with dead PID to be orphaned")
	}
	if reason != "executor process not running" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_AlivePIDStaleHeartbeat(t *testing.T) {
	// Use current PID which is definitely alive
	// Even with a stale heartbeat, a live PID means the task is NOT orphaned
	s := &State{
		Status: StatusRunning,
		Execution: &ExecutionInfo{
			PID:           os.Getpid(),
			Hostname:      "test-host",
			StartedAt:     time.Now().Add(-time.Hour),
			LastHeartbeat: time.Now().Add(-30 * time.Minute), // 30 minutes ago (way past threshold)
		},
	}

	isOrphaned, reason := s.CheckOrphaned()
	if isOrphaned {
		t.Errorf("expected task with alive PID and stale heartbeat to NOT be orphaned, got reason: %s", reason)
	}
}

func TestCheckOrphaned_DeadPIDStaleHeartbeat(t *testing.T) {
	// Dead PID with stale heartbeat should be orphaned with context about staleness
	s := &State{
		Status: StatusRunning,
		Execution: &ExecutionInfo{
			PID:           999999999, // Very high PID unlikely to exist
			Hostname:      "test-host",
			StartedAt:     time.Now().Add(-time.Hour),
			LastHeartbeat: time.Now().Add(-30 * time.Minute), // 30 minutes ago (>15 min threshold)
		},
	}

	isOrphaned, reason := s.CheckOrphaned()
	if !isOrphaned {
		t.Error("expected task with dead PID and stale heartbeat to be orphaned")
	}
	if reason != "executor process not running (heartbeat stale)" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_DeadPIDRecentHeartbeat(t *testing.T) {
	// Dead PID with recent heartbeat should still be orphaned (PID takes precedence)
	s := &State{
		Status: StatusRunning,
		Execution: &ExecutionInfo{
			PID:           999999999, // Very high PID unlikely to exist
			Hostname:      "test-host",
			StartedAt:     time.Now().Add(-time.Hour),
			LastHeartbeat: time.Now().Add(-1 * time.Minute), // Very recent heartbeat
		},
	}

	isOrphaned, reason := s.CheckOrphaned()
	if !isOrphaned {
		t.Error("expected task with dead PID to be orphaned even with recent heartbeat")
	}
	if reason != "executor process not running" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_Healthy(t *testing.T) {
	// Use current PID which is definitely alive
	s := &State{
		Status: StatusRunning,
		Execution: &ExecutionInfo{
			PID:           os.Getpid(),
			Hostname:      "test-host",
			StartedAt:     time.Now().Add(-time.Minute),
			LastHeartbeat: time.Now(), // Recent heartbeat
		},
	}

	isOrphaned, reason := s.CheckOrphaned()
	if isOrphaned {
		t.Errorf("expected healthy running task to not be orphaned, got reason: %s", reason)
	}
}

func TestIsPIDAlive_CurrentProcess(t *testing.T) {
	if !IsPIDAlive(os.Getpid()) {
		t.Error("expected current process PID to be alive")
	}
}

func TestIsPIDAlive_ZeroPID(t *testing.T) {
	if IsPIDAlive(0) {
		t.Error("expected PID 0 to not be considered alive")
	}
}

func TestIsPIDAlive_NegativePID(t *testing.T) {
	if IsPIDAlive(-1) {
		t.Error("expected negative PID to not be considered alive")
	}
}

func TestIsPIDAlive_HighPID(t *testing.T) {
	// Very high PID is unlikely to exist
	if IsPIDAlive(999999999) {
		t.Error("expected very high PID to not be alive (unless system is unusual)")
	}
}

func TestStartExecution(t *testing.T) {
	s := New("test-task")

	s.StartExecution(12345, "test-host")

	if s.Execution == nil {
		t.Fatal("expected execution info to be set")
	}
	if s.Execution.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", s.Execution.PID)
	}
	if s.Execution.Hostname != "test-host" {
		t.Errorf("expected hostname test-host, got %s", s.Execution.Hostname)
	}
	if time.Since(s.Execution.StartedAt) > time.Second {
		t.Error("expected StartedAt to be recent")
	}
	if time.Since(s.Execution.LastHeartbeat) > time.Second {
		t.Error("expected LastHeartbeat to be recent")
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	s := New("test-task")
	s.StartExecution(12345, "test-host")

	// Make heartbeat stale
	s.Execution.LastHeartbeat = time.Now().Add(-time.Hour)

	// Update heartbeat
	s.UpdateHeartbeat()

	if time.Since(s.Execution.LastHeartbeat) > time.Second {
		t.Error("expected LastHeartbeat to be updated to recent time")
	}
}

func TestClearExecution(t *testing.T) {
	s := New("test-task")
	s.StartExecution(12345, "test-host")

	s.ClearExecution()

	if s.Execution != nil {
		t.Error("expected execution info to be cleared")
	}
}

func TestGetExecutorPID(t *testing.T) {
	s := New("test-task")

	// No execution info
	if s.GetExecutorPID() != 0 {
		t.Error("expected 0 PID when no execution info")
	}

	// With execution info
	s.StartExecution(12345, "test-host")
	if s.GetExecutorPID() != 12345 {
		t.Errorf("expected PID 12345, got %d", s.GetExecutorPID())
	}
}
