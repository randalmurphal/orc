// Package lock provides task locking for coordinated execution.
// Solo mode uses NoOpLocker (zero overhead), P2P mode uses FileLocker.
package lock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Mode represents the coordination mode.
type Mode string

const (
	ModeSolo Mode = "solo"
	ModeP2P  Mode = "p2p"
	ModeTeam Mode = "team"
)

// LockFileName is the name of the lock file in the task directory.
const LockFileName = "lock.yaml"

// DefaultTTL is the default time-to-live for locks.
const DefaultTTL = 60 * time.Second

// DefaultHeartbeatInterval is the default interval for heartbeat updates.
const DefaultHeartbeatInterval = 10 * time.Second

// Lock represents task execution lock state.
type Lock struct {
	Owner     string    `yaml:"owner"`     // user@machine identifier
	Acquired  time.Time `yaml:"acquired"`  // when lock was acquired
	Heartbeat time.Time `yaml:"heartbeat"` // last heartbeat update
	TTL       string    `yaml:"ttl"`       // time-to-live as duration string
	PID       int       `yaml:"pid"`       // process ID of lock holder
}

// TTLDuration parses the TTL string and returns a time.Duration.
func (l *Lock) TTLDuration() time.Duration {
	d, err := time.ParseDuration(l.TTL)
	if err != nil {
		return DefaultTTL
	}
	return d
}

// IsStale returns true if the lock heartbeat is older than TTL.
func (l *Lock) IsStale() bool {
	return time.Since(l.Heartbeat) > l.TTLDuration()
}

// LockInfo provides information about a lock holder.
type LockInfo struct {
	Owner     string
	Acquired  time.Time
	Heartbeat time.Time
	PID       int
}

// Locker defines the interface for task locking.
type Locker interface {
	// Acquire attempts to acquire a lock for the task.
	// Returns nil on success, error if lock is held or on failure.
	Acquire(taskID string) error

	// Release releases the lock for the task.
	Release(taskID string) error

	// Heartbeat updates the heartbeat timestamp for the lock.
	Heartbeat(taskID string) error

	// IsLocked checks if a task is locked.
	// Returns (locked, lockInfo, error).
	IsLocked(taskID string) (bool, *LockInfo, error)
}

// NewLocker creates a Locker appropriate for the given mode.
func NewLocker(mode Mode, tasksDir, owner string) Locker {
	switch mode {
	case ModeSolo:
		return NewNoOpLocker()
	case ModeP2P, ModeTeam:
		return NewFileLocker(tasksDir, owner)
	default:
		return NewNoOpLocker()
	}
}

// NoOpLocker is a no-op locker for solo mode.
// All operations succeed immediately with zero overhead.
type NoOpLocker struct{}

// NewNoOpLocker creates a new NoOpLocker.
func NewNoOpLocker() *NoOpLocker {
	return &NoOpLocker{}
}

// Acquire always succeeds for NoOpLocker.
func (l *NoOpLocker) Acquire(taskID string) error {
	return nil
}

// Release always succeeds for NoOpLocker.
func (l *NoOpLocker) Release(taskID string) error {
	return nil
}

// Heartbeat always succeeds for NoOpLocker.
func (l *NoOpLocker) Heartbeat(taskID string) error {
	return nil
}

// IsLocked always returns false for NoOpLocker.
func (l *NoOpLocker) IsLocked(taskID string) (bool, *LockInfo, error) {
	return false, nil, nil
}

// FileLocker implements file-based locking for P2P mode.
// Lock files are stored as lock.yaml in each task directory.
type FileLocker struct {
	tasksDir string // base directory containing task directories
	owner    string // owner identifier (user@machine)
	mu       sync.Mutex
}

// NewFileLocker creates a new FileLocker.
func NewFileLocker(tasksDir, owner string) *FileLocker {
	return &FileLocker{
		tasksDir: tasksDir,
		owner:    owner,
	}
}

// lockPath returns the path to the lock file for a task.
func (l *FileLocker) lockPath(taskID string) string {
	return filepath.Join(l.tasksDir, taskID, LockFileName)
}

// readLock reads and parses a lock file.
func (l *FileLocker) readLock(taskID string) (*Lock, error) {
	path := l.lockPath(taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock Lock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}

	return &lock, nil
}

// writeLock writes a lock file atomically.
func (l *FileLocker) writeLock(taskID string, lock *Lock) error {
	path := l.lockPath(taskID)

	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}

	// Write to temp file first for atomic operation
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}

	// Rename for atomic update
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("rename lock file: %w", err)
	}

	return nil
}

// Acquire attempts to acquire the lock for a task.
func (l *FileLocker) Acquire(taskID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check existing lock
	existing, err := l.readLock(taskID)
	if err == nil {
		// Lock file exists - check if stale
		if !existing.IsStale() {
			// Lock is active and held by someone else
			if existing.Owner != l.owner {
				return &LockError{
					TaskID:  taskID,
					Owner:   existing.Owner,
					Reason:  "task is locked",
					Stale:   false,
					Claimed: false,
				}
			}
			// We already hold the lock, refresh it
		}
		// Lock is stale, we can claim it
	} else if !os.IsNotExist(err) {
		// Some other error reading lock file
		return fmt.Errorf("read lock: %w", err)
	}

	// Create or claim the lock
	lock := &Lock{
		Owner:     l.owner,
		Acquired:  time.Now().UTC(),
		Heartbeat: time.Now().UTC(),
		TTL:       DefaultTTL.String(),
		PID:       os.Getpid(),
	}

	if err := l.writeLock(taskID, lock); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}

	return nil
}

// Release releases the lock for a task.
func (l *FileLocker) Release(taskID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	path := l.lockPath(taskID)

	// Check if we own the lock
	existing, err := l.readLock(taskID)
	if os.IsNotExist(err) {
		// No lock file, nothing to release
		return nil
	}
	if err != nil {
		return fmt.Errorf("read lock: %w", err)
	}

	// Only release if we own the lock
	if existing.Owner != l.owner {
		return &LockError{
			TaskID:  taskID,
			Owner:   existing.Owner,
			Reason:  "cannot release lock owned by another",
			Stale:   false,
			Claimed: false,
		}
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove lock file: %w", err)
	}

	return nil
}

// Heartbeat updates the heartbeat timestamp for a lock.
func (l *FileLocker) Heartbeat(taskID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	existing, err := l.readLock(taskID)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("lock not found for task %s", taskID)
		}
		return fmt.Errorf("read lock: %w", err)
	}

	// Only update if we own the lock
	if existing.Owner != l.owner {
		return &LockError{
			TaskID:  taskID,
			Owner:   existing.Owner,
			Reason:  "cannot heartbeat lock owned by another",
			Stale:   false,
			Claimed: false,
		}
	}

	existing.Heartbeat = time.Now().UTC()
	if err := l.writeLock(taskID, existing); err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}

	return nil
}

// IsLocked checks if a task is currently locked.
func (l *FileLocker) IsLocked(taskID string) (bool, *LockInfo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock, err := l.readLock(taskID)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("read lock: %w", err)
	}

	// Check if lock is stale
	if lock.IsStale() {
		return false, nil, nil
	}

	return true, &LockInfo{
		Owner:     lock.Owner,
		Acquired:  lock.Acquired,
		Heartbeat: lock.Heartbeat,
		PID:       lock.PID,
	}, nil
}

// LockError represents a lock acquisition failure.
type LockError struct {
	TaskID  string
	Owner   string
	Reason  string
	Stale   bool // true if lock was stale
	Claimed bool // true if stale lock was claimed
}

func (e *LockError) Error() string {
	if e.Stale && e.Claimed {
		return fmt.Sprintf("task %s: claimed stale lock from %s", e.TaskID, e.Owner)
	}
	return fmt.Sprintf("task %s: %s (owner: %s)", e.TaskID, e.Reason, e.Owner)
}

// HeartbeatRunner runs periodic heartbeat updates for a lock.
type HeartbeatRunner struct {
	locker   Locker
	taskID   string
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewHeartbeatRunner creates a new heartbeat runner.
func NewHeartbeatRunner(locker Locker, taskID string, interval time.Duration) *HeartbeatRunner {
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	return &HeartbeatRunner{
		locker:   locker,
		taskID:   taskID,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a goroutine.
func (h *HeartbeatRunner) Start(ctx context.Context) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-h.stopCh:
				return
			case <-ticker.C:
				// Ignore heartbeat errors - lock will become stale if they persist
				_ = h.locker.Heartbeat(h.taskID)
			}
		}
	}()
}

// Stop stops the heartbeat loop and waits for it to finish.
func (h *HeartbeatRunner) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}
