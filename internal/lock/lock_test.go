package lock

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpLocker_AlwaysSucceeds(t *testing.T) {
	locker := NewNoOpLocker()

	// Acquire should succeed
	err := locker.Acquire("TASK-001")
	assert.NoError(t, err)

	// Release should succeed
	err = locker.Release("TASK-001")
	assert.NoError(t, err)

	// Heartbeat should succeed
	err = locker.Heartbeat("TASK-001")
	assert.NoError(t, err)

	// IsLocked should always return false
	locked, info, err := locker.IsLocked("TASK-001")
	assert.NoError(t, err)
	assert.False(t, locked)
	assert.Nil(t, info)
}

func TestFileLocker_AcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Acquire lock
	err := locker.Acquire(taskID)
	require.NoError(t, err)

	// Verify lock file exists
	lockPath := filepath.Join(taskDir, LockFileName)
	_, err = os.Stat(lockPath)
	assert.NoError(t, err, "lock file should exist")

	// Verify lock is held
	locked, info, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, "alice@laptop", info.Owner)
	assert.Equal(t, os.Getpid(), info.PID)

	// Release lock
	err = locker.Release(taskID)
	require.NoError(t, err)

	// Verify lock file removed
	_, err = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err), "lock file should be removed")

	// Verify lock is no longer held
	locked, info, err = locker.IsLocked(taskID)
	require.NoError(t, err)
	assert.False(t, locked)
	assert.Nil(t, info)
}

func TestFileLocker_ConcurrentAcquisitionFails(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	alice := NewFileLocker(tmpDir, "alice@laptop")
	bob := NewFileLocker(tmpDir, "bob@desktop")

	// Alice acquires
	err := alice.Acquire(taskID)
	require.NoError(t, err)

	// Bob cannot acquire
	err = bob.Acquire(taskID)
	assert.Error(t, err)

	lockErr, ok := err.(*LockError)
	require.True(t, ok, "error should be LockError")
	assert.Equal(t, "alice@laptop", lockErr.Owner)
	assert.Equal(t, taskID, lockErr.TaskID)

	// Alice releases
	err = alice.Release(taskID)
	require.NoError(t, err)

	// Bob can now acquire
	err = bob.Acquire(taskID)
	require.NoError(t, err)

	// Verify Bob is the owner
	locked, info, err := bob.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, "bob@desktop", info.Owner)
}

func TestFileLocker_StaleLockCanBeClaimed(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	// Create a stale lock (heartbeat in the past)
	staleLock := &Lock{
		Owner:     "zombie@ghost",
		Acquired:  time.Now().Add(-2 * time.Hour).UTC(),
		Heartbeat: time.Now().Add(-2 * time.Hour).UTC(), // Stale - 2 hours old
		TTL:       DefaultTTL.String(),
		PID:       99999,
	}

	lockPath := filepath.Join(taskDir, LockFileName)
	data, err := staleLock.marshalYAML()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lockPath, data, 0o644))

	// Verify the lock appears as not locked (stale)
	alice := NewFileLocker(tmpDir, "alice@laptop")
	locked, _, err := alice.IsLocked(taskID)
	require.NoError(t, err)
	assert.False(t, locked, "stale lock should not appear locked")

	// Alice can claim the stale lock
	err = alice.Acquire(taskID)
	require.NoError(t, err)

	// Verify Alice now owns the lock
	locked, info, err := alice.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, "alice@laptop", info.Owner)
}

func (l *Lock) marshalYAML() ([]byte, error) {
	return []byte("owner: " + l.Owner + "\n" +
		"acquired: " + l.Acquired.Format(time.RFC3339) + "\n" +
		"heartbeat: " + l.Heartbeat.Format(time.RFC3339) + "\n" +
		"ttl: " + l.TTL + "\n" +
		"pid: " + strconv.Itoa(l.PID) + "\n"), nil
}

func TestFileLocker_HeartbeatUpdatesTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Acquire lock
	err := locker.Acquire(taskID)
	require.NoError(t, err)

	// Get initial heartbeat
	_, info1, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	initialHeartbeat := info1.Heartbeat

	// Wait a bit and update heartbeat
	time.Sleep(10 * time.Millisecond)
	err = locker.Heartbeat(taskID)
	require.NoError(t, err)

	// Verify heartbeat was updated
	_, info2, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, info2.Heartbeat.After(initialHeartbeat),
		"heartbeat should be updated: was %v, now %v", initialHeartbeat, info2.Heartbeat)
}

func TestFileLocker_HeartbeatFailsForNonOwner(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	alice := NewFileLocker(tmpDir, "alice@laptop")
	bob := NewFileLocker(tmpDir, "bob@desktop")

	// Alice acquires
	err := alice.Acquire(taskID)
	require.NoError(t, err)

	// Bob cannot heartbeat Alice's lock
	err = bob.Heartbeat(taskID)
	assert.Error(t, err)

	lockErr, ok := err.(*LockError)
	require.True(t, ok, "error should be LockError")
	assert.Equal(t, "alice@laptop", lockErr.Owner)
}

func TestFileLocker_ReleaseFailsForNonOwner(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	alice := NewFileLocker(tmpDir, "alice@laptop")
	bob := NewFileLocker(tmpDir, "bob@desktop")

	// Alice acquires
	err := alice.Acquire(taskID)
	require.NoError(t, err)

	// Bob cannot release Alice's lock
	err = bob.Release(taskID)
	assert.Error(t, err)

	lockErr, ok := err.(*LockError)
	require.True(t, ok, "error should be LockError")
	assert.Equal(t, "alice@laptop", lockErr.Owner)

	// Lock should still be held by Alice
	locked, info, err := alice.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, "alice@laptop", info.Owner)
}

func TestFileLocker_ReacquireByOwner(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Acquire lock
	err := locker.Acquire(taskID)
	require.NoError(t, err)

	// Reacquire (should succeed - refreshes the lock)
	err = locker.Acquire(taskID)
	require.NoError(t, err)

	// Still locked by alice
	locked, info, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, locked)
	assert.Equal(t, "alice@laptop", info.Owner)
}

func TestNewLocker_Factory(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		mode     Mode
		wantType string
	}{
		{ModeSolo, "*lock.NoOpLocker"},
		{ModeP2P, "*lock.FileLocker"},
		{ModeTeam, "*lock.FileLocker"},
		{"unknown", "*lock.NoOpLocker"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			locker := NewLocker(tt.mode, tmpDir, "test@machine")
			gotType := typeString(locker)
			assert.Equal(t, tt.wantType, gotType)
		})
	}
}

func typeString(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return "*lock." + typeName(v)
}

func typeName(v interface{}) string {
	switch v.(type) {
	case *NoOpLocker:
		return "NoOpLocker"
	case *FileLocker:
		return "FileLocker"
	default:
		return "unknown"
	}
}

func TestLock_IsStale(t *testing.T) {
	tests := []struct {
		name      string
		heartbeat time.Time
		ttl       string
		wantStale bool
	}{
		{
			name:      "fresh lock",
			heartbeat: time.Now(),
			ttl:       "60s",
			wantStale: false,
		},
		{
			name:      "stale lock",
			heartbeat: time.Now().Add(-2 * time.Minute),
			ttl:       "60s",
			wantStale: true,
		},
		{
			name:      "just before stale",
			heartbeat: time.Now().Add(-59 * time.Second),
			ttl:       "60s",
			wantStale: false,
		},
		{
			name:      "invalid ttl uses default",
			heartbeat: time.Now().Add(-2 * time.Minute),
			ttl:       "invalid",
			wantStale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock := &Lock{
				Heartbeat: tt.heartbeat,
				TTL:       tt.ttl,
			}
			assert.Equal(t, tt.wantStale, lock.IsStale())
		})
	}
}

func TestHeartbeatRunner(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Acquire lock
	err := locker.Acquire(taskID)
	require.NoError(t, err)

	// Get initial heartbeat
	_, info1, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	initialHeartbeat := info1.Heartbeat

	// Start heartbeat runner with short interval
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := NewHeartbeatRunner(locker, taskID, 50*time.Millisecond)
	runner.Start(ctx)

	// Wait for at least one heartbeat
	time.Sleep(100 * time.Millisecond)

	// Stop runner
	runner.Stop()

	// Verify heartbeat was updated
	_, info2, err := locker.IsLocked(taskID)
	require.NoError(t, err)
	assert.True(t, info2.Heartbeat.After(initialHeartbeat),
		"heartbeat should be updated by runner")
}

func TestHeartbeatRunner_StopsOnContextCancel(t *testing.T) {
	locker := NewNoOpLocker()

	ctx, cancel := context.WithCancel(context.Background())
	runner := NewHeartbeatRunner(locker, "TASK-001", 10*time.Millisecond)
	runner.Start(ctx)

	// Cancel context
	cancel()

	// Should complete quickly without blocking
	done := make(chan struct{})
	go func() {
		runner.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Stop should complete quickly after context cancel")
	}
}

func TestFileLocker_HeartbeatNonexistentLock(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Heartbeat without acquiring should fail
	err := locker.Heartbeat(taskID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not found")
}

func TestFileLocker_ReleaseNonexistentLock(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"
	taskDir := filepath.Join(tmpDir, taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	locker := NewFileLocker(tmpDir, "alice@laptop")

	// Release without lock should succeed (idempotent)
	err := locker.Release(taskID)
	assert.NoError(t, err)
}

func TestLockError_ErrorMessage(t *testing.T) {
	tests := []struct {
		name    string
		err     *LockError
		wantMsg string
	}{
		{
			name: "regular lock error",
			err: &LockError{
				TaskID: "TASK-001",
				Owner:  "bob@desktop",
				Reason: "task is locked",
				Stale:  false,
			},
			wantMsg: "task TASK-001: task is locked (owner: bob@desktop)",
		},
		{
			name: "stale lock claimed",
			err: &LockError{
				TaskID:  "TASK-001",
				Owner:   "zombie@ghost",
				Reason:  "claimed stale lock",
				Stale:   true,
				Claimed: true,
			},
			wantMsg: "task TASK-001: claimed stale lock from zombie@ghost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}
