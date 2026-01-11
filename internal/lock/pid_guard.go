// Package lock provides same-user execution protection via PID guard.
//
// Design Philosophy:
// - NO cross-user locking - anyone can run any task
// - PID guard only prevents same user from accidentally running same task twice
// - Each user gets their own worktree/branch (no conflicts)
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PIDFileName is the name of the PID file in the worktree directory.
const PIDFileName = ".orc.pid"

// PIDGuard prevents the same user from running the same task twice.
// It uses a simple PID file in the worktree directory.
type PIDGuard struct {
	worktreePath string
}

// NewPIDGuard creates a new PID guard for the given worktree path.
func NewPIDGuard(worktreePath string) *PIDGuard {
	return &PIDGuard{worktreePath: worktreePath}
}

// pidFilePath returns the path to the PID file.
func (g *PIDGuard) pidFilePath() string {
	return filepath.Join(g.worktreePath, PIDFileName)
}

// Check verifies no other process from this user is running the task.
// If a stale PID file exists (process no longer running), it's cleaned up.
// Returns nil if safe to proceed, error if task is already running.
func (g *PIDGuard) Check() error {
	pidFile := g.pidFilePath()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No PID file, safe to proceed
		}
		return fmt.Errorf("read pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		// Invalid PID file, remove it
		os.Remove(pidFile)
		return nil
	}

	if processExists(pid) {
		return &AlreadyRunningError{PID: pid}
	}

	// Stale PID file, clean it up
	os.Remove(pidFile)
	return nil
}

// Acquire writes the current process PID to the guard file.
// Call Check() before Acquire() to ensure no conflict.
func (g *PIDGuard) Acquire() error {
	// Ensure worktree directory exists
	if err := os.MkdirAll(g.worktreePath, 0755); err != nil {
		return fmt.Errorf("create worktree dir: %w", err)
	}

	pidFile := g.pidFilePath()
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	return nil
}

// Release removes the PID file.
// Safe to call even if file doesn't exist.
func (g *PIDGuard) Release() {
	os.Remove(g.pidFilePath())
}

// AlreadyRunningError indicates the task is already running.
type AlreadyRunningError struct {
	PID int
}

func (e *AlreadyRunningError) Error() string {
	return fmt.Sprintf("task already running (pid %d)", e.PID)
}

// processExists checks if a process with the given PID exists.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. We need to send signal 0 to check.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
