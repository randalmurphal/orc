// Package task provides task management for orc.
package task

import (
	"os"
	"syscall"
	"time"
)

// StaleHeartbeatThreshold is the duration after which a heartbeat is considered stale.
// This threshold is only used as a fallback when PID check is inconclusive.
// A live PID always indicates a healthy task, regardless of heartbeat staleness.
const StaleHeartbeatThreshold = 15 * time.Minute

// isPIDAlive checks if a process with the given PID exists.
// On Unix-like systems, this sends signal 0 to check existence.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 checks if process exists without actually signaling it
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CheckOrphaned checks if a task is orphaned (executor process died mid-run).
// A task is orphaned if:
// 1. Its status is "running" but no executor PID is tracked
// 2. Its status is "running" with a PID that no longer exists
//
// Note: Heartbeat staleness is only used for additional context when the PID is dead.
// A live PID always indicates a healthy task - this prevents false positives during
// long-running phases where heartbeats may not be updated frequently.
//
// Returns (isOrphaned, reason) where reason explains why.
func (t *Task) CheckOrphaned() (bool, string) {
	// Only running tasks can be orphaned
	if t.Status != StatusRunning {
		return false, ""
	}

	// No execution info means potentially orphaned (legacy or incomplete state)
	if t.ExecutorPID == 0 {
		return true, "no execution info (legacy state or incomplete)"
	}

	// Primary check: Is the executor process alive?
	if !isPIDAlive(t.ExecutorPID) {
		// PID is dead - task is definitely orphaned
		// Use heartbeat to provide additional context in the reason
		if t.LastHeartbeat != nil && time.Since(*t.LastHeartbeat) > StaleHeartbeatThreshold {
			return true, "executor process not running (heartbeat stale)"
		}
		return true, "executor process not running"
	}

	// PID is alive - task is NOT orphaned, regardless of heartbeat
	return false, ""
}
