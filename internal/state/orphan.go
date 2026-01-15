// Package state provides execution state tracking for orc tasks.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package state

import (
	"os"
	"syscall"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

// OrphanInfo contains information about an orphaned task.
type OrphanInfo struct {
	TaskID       string
	State        *State
	Task         *task.Task
	LastPID      int
	LastHostname string
	OrphanedAt   time.Time
	Reason       string
}

// CheckOrphaned checks if a state represents an orphaned task.
// A task is orphaned if:
// 1. Its status is "running" but no executor PID is tracked
// 2. Its status is "running" with a PID that no longer exists
// 3. Its status is "running" but the heartbeat is stale (>5 minutes)
//
// Returns (isOrphaned, reason) where reason explains why.
func (s *State) CheckOrphaned() (bool, string) {
	// Only running tasks can be orphaned
	if s.Status != StatusRunning {
		return false, ""
	}

	// No execution info means potentially orphaned (legacy or incomplete state)
	if s.Execution == nil {
		return true, "no execution info (legacy state or incomplete)"
	}

	// Check if the PID is still alive
	if !IsPIDAlive(s.Execution.PID) {
		return true, "executor process not running"
	}

	// Check for stale heartbeat (>5 minutes without update)
	const staleThreshold = 5 * time.Minute
	if time.Since(s.Execution.LastHeartbeat) > staleThreshold {
		return true, "heartbeat stale (>5 minutes)"
	}

	return false, ""
}

// IsPIDAlive checks if a process with the given PID exists.
// On Unix-like systems, this sends signal 0 to check existence.
func IsPIDAlive(pid int) bool {
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

// MarkAsInterrupted marks this state as interrupted due to orphan detection.
// Clears execution info and marks the current phase as interrupted.
func (s *State) MarkAsInterrupted() {
	// Mark the current phase as interrupted
	if s.CurrentPhase != "" {
		s.InterruptPhase(s.CurrentPhase)
	} else {
		s.Status = StatusInterrupted
	}

	// Clear the stale execution info
	s.ClearExecution()
}
