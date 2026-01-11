package lock

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIDGuard_Check_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	guard := NewPIDGuard(tmpDir)

	// No PID file exists, should succeed
	err := guard.Check()
	assert.NoError(t, err)
}

func TestPIDGuard_Check_StaleProcess(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a PID file with a non-existent PID
	// Using a very high PID that's unlikely to exist
	pidFile := filepath.Join(tmpDir, PIDFileName)
	err := os.WriteFile(pidFile, []byte("999999"), 0644)
	require.NoError(t, err)

	guard := NewPIDGuard(tmpDir)
	err = guard.Check()

	// Should succeed because process doesn't exist (stale PID)
	assert.NoError(t, err)

	// Stale PID file should be cleaned up
	_, err = os.Stat(pidFile)
	assert.True(t, os.IsNotExist(err), "stale PID file should be removed")
}

func TestPIDGuard_Check_InvalidPID(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid PID
	pidFile := filepath.Join(tmpDir, PIDFileName)
	err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
	require.NoError(t, err)

	guard := NewPIDGuard(tmpDir)
	err = guard.Check()

	// Should succeed (invalid PID file is cleaned up)
	assert.NoError(t, err)

	// Invalid PID file should be cleaned up
	_, err = os.Stat(pidFile)
	assert.True(t, os.IsNotExist(err), "invalid PID file should be removed")
}

func TestPIDGuard_Check_CurrentProcess(t *testing.T) {
	tmpDir := t.TempDir()

	// Write current process PID
	pidFile := filepath.Join(tmpDir, PIDFileName)
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	require.NoError(t, err)

	guard := NewPIDGuard(tmpDir)
	err = guard.Check()

	// Should fail because current process exists
	assert.Error(t, err)
	alreadyRunning, ok := err.(*AlreadyRunningError)
	require.True(t, ok, "error should be AlreadyRunningError")
	assert.Equal(t, os.Getpid(), alreadyRunning.PID)
}

func TestPIDGuard_AcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	guard := NewPIDGuard(tmpDir)

	// Acquire should write PID file
	err := guard.Acquire()
	require.NoError(t, err)

	// Verify PID file exists with current PID
	pidFile := filepath.Join(tmpDir, PIDFileName)
	data, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(os.Getpid()), string(data))

	// Release should remove PID file
	guard.Release()
	_, err = os.Stat(pidFile)
	assert.True(t, os.IsNotExist(err), "PID file should be removed after release")
}

func TestPIDGuard_Acquire_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "worktree")
	guard := NewPIDGuard(nestedDir)

	// Acquire should create directory if it doesn't exist
	err := guard.Acquire()
	require.NoError(t, err)

	// Directory should exist
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// PID file should exist
	pidFile := filepath.Join(nestedDir, PIDFileName)
	_, err = os.Stat(pidFile)
	assert.NoError(t, err)
}

func TestPIDGuard_Release_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	guard := NewPIDGuard(tmpDir)

	// Release without acquire should not panic
	guard.Release()

	// Multiple releases should not panic
	guard.Release()
	guard.Release()
}

func TestAlreadyRunningError(t *testing.T) {
	err := &AlreadyRunningError{PID: 12345}
	assert.Equal(t, "task already running (pid 12345)", err.Error())
}

func TestPIDGuard_DifferentUsers_NoConflict(t *testing.T) {
	// This test demonstrates that different users (different worktrees) don't conflict
	// In p2p mode, Alice's worktree: .orc/worktrees/TASK-001-am
	// Bob's worktree: .orc/worktrees/TASK-001-bj
	// Each has its own PID guard, no conflict

	baseDir := t.TempDir()
	aliceWorktree := filepath.Join(baseDir, "TASK-001-am")
	bobWorktree := filepath.Join(baseDir, "TASK-001-bj")

	aliceGuard := NewPIDGuard(aliceWorktree)
	bobGuard := NewPIDGuard(bobWorktree)

	// Both can acquire their guards
	err := aliceGuard.Acquire()
	require.NoError(t, err)
	defer aliceGuard.Release()

	err = bobGuard.Acquire()
	require.NoError(t, err)
	defer bobGuard.Release()

	// Both guards are active, no conflict
	// (In real scenario, these would be different processes)
}
