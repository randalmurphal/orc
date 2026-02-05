package task

import (
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// SC-1: Heartbeat-based stale detection
// ============================================================================

// TestIsClaimStale_FreshHeartbeat verifies that a running task with a recent
// heartbeat is NOT considered stale.
// Covers: SC-1
func TestIsClaimStale_FreshHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-1 * time.Minute))

	isStale, detail := IsClaimStale(tk)
	if isStale {
		t.Errorf("expected task with fresh heartbeat (1 min ago) to NOT be stale, got stale: %s", detail)
	}
}

// TestIsClaimStale_StaleHeartbeat verifies that a running task whose heartbeat
// exceeds StaleHeartbeatThreshold is considered stale.
// Covers: SC-1
func TestIsClaimStale_StaleHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-20 * time.Minute))

	isStale, detail := IsClaimStale(tk)
	if !isStale {
		t.Error("expected task with stale heartbeat (20 min ago) to be stale")
	}
	if detail == "" {
		t.Error("expected non-empty detail string for stale heartbeat")
	}
}

// TestIsClaimStale_NoHeartbeat verifies that a running task with no heartbeat
// recorded is considered stale.
// Covers: SC-1
func TestIsClaimStale_NoHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = nil

	isStale, detail := IsClaimStale(tk)
	if !isStale {
		t.Error("expected task with no heartbeat to be stale")
	}
	if detail == "" {
		t.Error("expected non-empty detail string when no heartbeat recorded")
	}
}

// TestIsClaimStale_NonRunningTask verifies that non-running tasks are never
// considered stale, regardless of heartbeat age.
// Covers: SC-1
func TestIsClaimStale_NonRunningTask(t *testing.T) {
	t.Parallel()

	statuses := []orcv1.TaskStatus{
		orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		orcv1.TaskStatus_TASK_STATUS_FAILED,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
	}

	for _, status := range statuses {
		tk := NewProtoTask("TASK-001", "Test task")
		tk.Status = status
		tk.ExecutorPid = 12345
		// Old heartbeat that would be stale if task were running
		tk.LastHeartbeat = timestamppb.New(time.Now().Add(-1 * time.Hour))

		isStale, detail := IsClaimStale(tk)
		if isStale {
			t.Errorf("expected non-running task (status=%v) to NOT be stale, got stale: %s", status, detail)
		}
	}
}

// TestIsClaimStale_NilTask verifies that nil task is handled gracefully.
// Covers: SC-1
func TestIsClaimStale_NilTask(t *testing.T) {
	t.Parallel()

	isStale, _ := IsClaimStale(nil)
	if isStale {
		t.Error("expected nil task to NOT be stale")
	}
}

// TestIsClaimStale_ThresholdBoundary verifies exact boundary behavior at the
// stale heartbeat threshold (15 minutes).
// Covers: SC-1
func TestIsClaimStale_ThresholdBoundary(t *testing.T) {
	t.Parallel()

	// At exactly the threshold (14m59s) → NOT stale
	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-(StaleHeartbeatThreshold - time.Second)))

	isStale, _ := IsClaimStale(tk)
	if isStale {
		t.Error("expected task with heartbeat just inside threshold to NOT be stale")
	}

	// Just past threshold (15m1s) → stale
	tk2 := NewProtoTask("TASK-002", "Test task 2")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk2.ExecutorPid = 12345
	tk2.LastHeartbeat = timestamppb.New(time.Now().Add(-(StaleHeartbeatThreshold + time.Second)))

	isStale2, _ := IsClaimStale(tk2)
	if !isStale2 {
		t.Error("expected task with heartbeat just past threshold to be stale")
	}
}

// TestStaleHeartbeatThreshold_Value verifies the threshold constant is 15 minutes.
// Covers: SC-1
func TestStaleHeartbeatThreshold_Value(t *testing.T) {
	t.Parallel()

	if StaleHeartbeatThreshold != 15*time.Minute {
		t.Errorf("expected StaleHeartbeatThreshold = 15m, got %v", StaleHeartbeatThreshold)
	}
}

// ============================================================================
// SC-3: Status output shows stale indicators
// ============================================================================

// TestFormatHeartbeatStatus_FreshHeartbeat verifies that a running task with
// a recent heartbeat shows "healthy" status.
// Covers: SC-3
func TestFormatHeartbeatStatus_FreshHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-30 * time.Second))

	status := FormatHeartbeatStatus(tk)
	if status == "" {
		t.Error("expected non-empty status for running task with fresh heartbeat")
	}
	// Must contain "healthy" keyword
	if !containsSubstring(status, "healthy") {
		t.Errorf("expected status to contain 'healthy', got: %q", status)
	}
	// Must contain time info
	if !containsSubstring(status, "heartbeat") {
		t.Errorf("expected status to contain 'heartbeat', got: %q", status)
	}
}

// TestFormatHeartbeatStatus_StaleHeartbeat verifies that a running task with
// a stale heartbeat shows "stale" status with age.
// Covers: SC-3
func TestFormatHeartbeatStatus_StaleHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-23 * time.Minute))

	status := FormatHeartbeatStatus(tk)
	if status == "" {
		t.Error("expected non-empty status for running task with stale heartbeat")
	}
	// Must contain "stale" keyword
	if !containsSubstring(status, "stale") {
		t.Errorf("expected status to contain 'stale', got: %q", status)
	}
}

// TestFormatHeartbeatStatus_NoHeartbeat verifies that a running task with
// no heartbeat shows appropriate stale message.
// Covers: SC-3
func TestFormatHeartbeatStatus_NoHeartbeat(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = nil

	status := FormatHeartbeatStatus(tk)
	if status == "" {
		t.Error("expected non-empty status for running task with no heartbeat")
	}
	if !containsSubstring(status, "stale") {
		t.Errorf("expected status to contain 'stale', got: %q", status)
	}
}

// TestFormatHeartbeatStatus_NonRunningTask verifies that non-running tasks
// return empty status (heartbeat is irrelevant).
// Covers: SC-3
func TestFormatHeartbeatStatus_NonRunningTask(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-1 * time.Hour))

	status := FormatHeartbeatStatus(tk)
	if status != "" {
		t.Errorf("expected empty status for non-running task, got: %q", status)
	}
}

// TestFormatHeartbeatStatus_NilTask verifies nil task returns empty string.
// Covers: SC-3
func TestFormatHeartbeatStatus_NilTask(t *testing.T) {
	t.Parallel()

	status := FormatHeartbeatStatus(nil)
	if status != "" {
		t.Errorf("expected empty status for nil task, got: %q", status)
	}
}

// ============================================================================
// SC-4: Stale claims warn but don't auto-release
// ============================================================================

// TestIsClaimStale_StaleButNotAutoReleased verifies that IsClaimStale only
// reports staleness but does NOT modify the task (no auto-release).
// Covers: SC-4
func TestIsClaimStale_StaleButNotAutoReleased(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 12345
	tk.LastHeartbeat = timestamppb.New(time.Now().Add(-30 * time.Minute))

	// Call IsClaimStale
	isStale, _ := IsClaimStale(tk)
	if !isStale {
		t.Fatal("expected task to be stale")
	}

	// Verify task was NOT modified (no auto-release behavior)
	if tk.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("IsClaimStale should NOT modify task status, got: %v", tk.Status)
	}
	if tk.ExecutorPid != 12345 {
		t.Errorf("IsClaimStale should NOT modify executor PID, got: %d", tk.ExecutorPid)
	}
	if tk.LastHeartbeat == nil {
		t.Error("IsClaimStale should NOT clear heartbeat")
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestIsClaimStale_ZeroPID verifies behavior when PID is 0 (legacy/no executor).
// Covers: EC-4
func TestIsClaimStale_ZeroPID(t *testing.T) {
	t.Parallel()

	tk := NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.ExecutorPid = 0
	tk.LastHeartbeat = nil

	// A running task with no PID and no heartbeat should be stale
	isStale, _ := IsClaimStale(tk)
	if !isStale {
		t.Error("expected running task with no PID and no heartbeat to be stale")
	}
}

// ============================================================================
// Helpers
// ============================================================================

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
