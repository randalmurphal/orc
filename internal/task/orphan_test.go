package task

import (
	"os"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// SC-2: Primary orphan detection uses PID, not heartbeat
// ============================================================================

// TestCheckOrphanedProto_LivePID_FreshHeartbeat verifies that a running task
// with a live PID and fresh heartbeat is NOT considered orphaned.
// Covers: SC-2 (testing requirement #1)
func TestCheckOrphanedProto_LivePID_FreshHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = int32(os.Getpid()) // Current process is alive
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-30 * time.Second))

	isOrphaned, reason := CheckOrphanedProto(tk)
	if isOrphaned {
		t.Errorf("expected live PID + fresh heartbeat to NOT be orphaned, got orphaned: %s", reason)
	}
}

// TestCheckOrphanedProto_LivePID_StaleHeartbeat verifies that a running task
// with a live PID but stale heartbeat is NOT orphaned. PID check takes priority.
// Covers: SC-2 (testing requirement #2 - PID wins over heartbeat)
func TestCheckOrphanedProto_LivePID_StaleHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = int32(os.Getpid()) // Current process is alive
	// Heartbeat is very stale (1 hour old)
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-1 * time.Hour))

	isOrphaned, reason := CheckOrphanedProto(tk)
	if isOrphaned {
		t.Errorf("expected live PID + stale heartbeat to NOT be orphaned (PID wins), got orphaned: %s", reason)
	}
}

// TestCheckOrphanedProto_DeadPID_NoHeartbeat verifies that a running task
// with a dead PID and no heartbeat is orphaned.
// Covers: SC-2 (testing requirement #3)
func TestCheckOrphanedProto_DeadPID_NoHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 999999 // Very large PID that almost certainly doesn't exist
	tk.LastHeartbeat = nil

	isOrphaned, _ := CheckOrphanedProto(tk)
	if !isOrphaned {
		t.Error("expected dead PID + no heartbeat to be orphaned")
	}
}

// TestCheckOrphanedProto_DeadPID_StaleHeartbeat verifies that a running task
// with a dead PID and stale heartbeat is orphaned with enhanced message.
// Covers: SC-2 (testing requirement #4)
func TestCheckOrphanedProto_DeadPID_StaleHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 999999 // Dead PID
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-30 * time.Minute))

	isOrphaned, reason := CheckOrphanedProto(tk)
	if !isOrphaned {
		t.Error("expected dead PID + stale heartbeat to be orphaned")
	}
	// Enhanced message should mention heartbeat staleness
	if !containsSubstring(reason, "heartbeat stale") {
		t.Errorf("expected reason to mention 'heartbeat stale', got: %q", reason)
	}
}

// TestCheckOrphanedProto_NoPID verifies that a running task with PID 0 (no
// executor info) is considered orphaned.
// Covers: SC-2 (testing requirement #5), EC-4
func TestCheckOrphanedProto_NoPID(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 0

	isOrphaned, reason := CheckOrphanedProto(tk)
	if !isOrphaned {
		t.Error("expected running task with no PID to be orphaned")
	}
	if !containsSubstring(reason, "no execution info") {
		t.Errorf("expected reason to mention 'no execution info', got: %q", reason)
	}
}

// TestCheckOrphanedProto_NonRunningStatus verifies that non-running tasks
// are never considered orphaned, regardless of PID state.
// Covers: SC-2 (testing requirement #6)
func TestCheckOrphanedProto_NonRunningStatus(t *testing.T) {
	t.Parallel()

	statuses := []orcv1.TaskStatus{
		orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		orcv1.TaskStatus_TASK_STATUS_FAILED,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
	}

	for _, status := range statuses {
		tk := NewProtoTask("TASK-001", "Test task")
		tk.Status = status
		tk.ExecutorPid = 999999 // Dead PID - would be orphaned if running

		isOrphaned, reason := CheckOrphanedProto(tk)
		if isOrphaned {
			t.Errorf("expected non-running task (status=%v) to NOT be orphaned, got orphaned: %s", status, reason)
		}
	}
}

// TestCheckOrphanedProto_NilTask verifies nil task is handled gracefully.
// Covers: SC-2 (edge case)
func TestCheckOrphanedProto_NilTask(t *testing.T) {
	t.Parallel()

	isOrphaned, _ := CheckOrphanedProto(nil)
	if isOrphaned {
		t.Error("expected nil task to NOT be orphaned")
	}
}

// ============================================================================
// Edge Cases from Spec
// ============================================================================

// TestCheckOrphanedProto_LivePID_NoHeartbeat verifies that a live PID with
// no heartbeat is NOT orphaned (PID check is authoritative).
// Covers: EC-2 (heartbeat stops but process alive)
func TestCheckOrphanedProto_LivePID_NoHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = int32(os.Getpid()) // Current process is alive
	tk.LastHeartbeat = nil               // No heartbeat at all

	isOrphaned, reason := CheckOrphanedProto(tk)
	if isOrphaned {
		t.Errorf("expected live PID + no heartbeat to NOT be orphaned (PID wins), got: %s", reason)
	}
}
