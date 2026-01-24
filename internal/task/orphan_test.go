package task

import (
	"os"
	"testing"
	"time"
)

func TestCheckOrphaned_NotRunning(t *testing.T) {
	task := &Task{
		ID:     "TASK-001",
		Status: StatusCompleted,
	}

	orphaned, reason := task.CheckOrphaned()
	if orphaned {
		t.Errorf("expected non-running task to not be orphaned, got reason: %s", reason)
	}
}

func TestCheckOrphaned_NoExecutorPID(t *testing.T) {
	task := &Task{
		ID:          "TASK-001",
		Status:      StatusRunning,
		ExecutorPID: 0, // No PID
	}

	orphaned, reason := task.CheckOrphaned()
	if !orphaned {
		t.Error("expected running task with no executor PID to be orphaned")
	}
	if reason != "no execution info (legacy state or incomplete)" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_CurrentProcessAlive(t *testing.T) {
	// Use current process PID - it's definitely alive
	task := &Task{
		ID:          "TASK-001",
		Status:      StatusRunning,
		ExecutorPID: os.Getpid(),
	}

	orphaned, reason := task.CheckOrphaned()
	if orphaned {
		t.Errorf("expected task with live PID to not be orphaned, got reason: %s", reason)
	}
}

func TestCheckOrphaned_DeadPID(t *testing.T) {
	// Use a PID that definitely doesn't exist
	task := &Task{
		ID:          "TASK-001",
		Status:      StatusRunning,
		ExecutorPID: 999999999, // Very high PID, unlikely to exist
	}

	orphaned, reason := task.CheckOrphaned()
	if !orphaned {
		t.Error("expected task with dead PID to be orphaned")
	}
	if reason != "executor process not running" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCheckOrphaned_DeadPIDWithStaleHeartbeat(t *testing.T) {
	staleTime := time.Now().Add(-30 * time.Minute) // 30 minutes ago
	task := &Task{
		ID:            "TASK-001",
		Status:        StatusRunning,
		ExecutorPID:   999999999, // Very high PID, unlikely to exist
		LastHeartbeat: &staleTime,
	}

	orphaned, reason := task.CheckOrphaned()
	if !orphaned {
		t.Error("expected task with dead PID and stale heartbeat to be orphaned")
	}
	if reason != "executor process not running (heartbeat stale)" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestIsPIDAlive(t *testing.T) {
	tests := []struct {
		name     string
		pid      int
		expected bool
	}{
		{"zero PID", 0, false},
		{"negative PID", -1, false},
		{"current process", os.Getpid(), true},
		{"nonexistent PID", 999999999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPIDAlive(tt.pid)
			if result != tt.expected {
				t.Errorf("isPIDAlive(%d) = %v, want %v", tt.pid, result, tt.expected)
			}
		})
	}
}
