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

// Note: The CheckOrphaned method was removed as part of the proto migration.
// Use CheckOrphanedProto in proto_helpers.go for orcv1.Task instead.
