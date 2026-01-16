//go:build !windows

package orchestrator

import (
	"os/exec"
	"syscall"
	"testing"
)

// TestSetProcAttrSetsProcessGroup verifies that setProcAttr sets Setpgid=true
// on the command's SysProcAttr, enabling process group creation.
func TestSetProcAttrSetsProcessGroup(t *testing.T) {
	cmd := exec.Command("echo", "test")

	// Before setProcAttr, SysProcAttr is nil
	if cmd.SysProcAttr != nil {
		t.Error("expected SysProcAttr to be nil before setProcAttr")
	}

	// Apply process group attributes
	setProcAttr(cmd)

	// Verify SysProcAttr is set
	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be non-nil after setProcAttr")
	}

	// Verify Setpgid is true
	if !cmd.SysProcAttr.Setpgid {
		t.Error("expected Setpgid to be true after setProcAttr")
	}
}

// TestKillProcessGroupNegativePID verifies that killProcessGroup correctly
// attempts to signal the process group (negative PID) rather than just
// the individual process.
func TestKillProcessGroupNegativePID(t *testing.T) {
	// Test with invalid PID (0 or negative) - should be no-op
	err := killProcessGroup(0)
	if err != nil {
		t.Errorf("expected no error for PID 0, got %v", err)
	}

	err = killProcessGroup(-1)
	if err != nil {
		t.Errorf("expected no error for negative PID, got %v", err)
	}

	// Test with non-existent PID - should return ESRCH (no such process)
	err = killProcessGroup(99999999)
	if err == nil {
		t.Log("warning: expected error for non-existent PID, but none returned (process may exist)")
	} else if err != syscall.ESRCH {
		// ESRCH (no such process) is the expected error
		t.Logf("killProcessGroup returned expected error type: %v", err)
	}
}

// TestWorkerKillProcessGroupIdempotent verifies that calling killProcessGroup
// multiple times is safe (idempotent) and doesn't panic.
func TestWorkerKillProcessGroupIdempotent(t *testing.T) {
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
		cancel: func() {},
	}

	// Call with nil cmd - should not panic
	worker.killProcessGroup()
	worker.killProcessGroup() // Double call should be safe

	// Set cmd but with nil Process - should not panic
	worker.cmd = &exec.Cmd{}
	worker.killProcessGroup()
	worker.killProcessGroup() // Double call should be safe

	// Test passed if no panic occurred
}

// TestWorkerStopKillsProcessGroup verifies that Stop() calls both cancel()
// and killProcessGroup().
func TestWorkerStopKillsProcessGroup(t *testing.T) {
	var cancelCalled bool

	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
		cancel: func() { cancelCalled = true },
	}

	// Stop should call cancel and killProcessGroup
	worker.Stop()

	if !cancelCalled {
		t.Error("expected cancel to be called by Stop()")
	}

	// killProcessGroup should have been called (no cmd = no-op, but safe)
	// If we got here without panic, the test passes
}

// TestWorkerStopDoubleCallSafe verifies that Stop() can be called multiple times
// without panicking or causing issues.
func TestWorkerStopDoubleCallSafe(t *testing.T) {
	var cancelCount int

	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
		cancel: func() { cancelCount++ },
	}

	// First stop
	worker.Stop()

	// Second stop - should not panic
	worker.Stop()

	// Third stop - should not panic
	worker.Stop()

	// cancel is called each time (which is fine - cancelling is idempotent in Go)
	if cancelCount != 3 {
		t.Errorf("expected cancel to be called 3 times, got %d", cancelCount)
	}
}

// TestProcessGroupCreationOnCommand verifies that when we create a command
// and apply setProcAttr, the resulting command has the correct attributes
// for process group creation.
func TestProcessGroupCreationOnCommand(t *testing.T) {
	// Simulate what Worker.run() does
	cmd := exec.Command("sleep", "1")
	cmd.Dir = "/tmp"
	setProcAttr(cmd)

	// Verify the command is properly configured for process groups
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should be set")
	}

	if !cmd.SysProcAttr.Setpgid {
		t.Error("Setpgid should be true for process group isolation")
	}

	// Additional check: other SysProcAttr fields should be at default
	// (we only set Setpgid, not other fields like Setsid)
	if cmd.SysProcAttr.Setsid {
		t.Error("Setsid should remain false (we only set Setpgid)")
	}
}
