// Package state provides execution state tracking for orc tasks.
package state

import (
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/randalmurphal/orc/internal/config"
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

// IsPIDAlive checks if a process with the given PID is still running.
// Returns false for PID 0 or if the process doesn't exist.
func IsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// On Unix systems, sending signal 0 checks if process exists
	// without actually sending a signal
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	// to check if the process actually exists. syscall.Signal(0) is the
	// standard way to check process existence.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// FindOrphanedTasks finds all tasks that appear to be orphaned.
func FindOrphanedTasks() ([]OrphanInfo, error) {
	return FindOrphanedTasksFrom("")
}

// FindOrphanedTasksFrom finds orphaned tasks from a specific project directory.
func FindOrphanedTasksFrom(projectDir string) ([]OrphanInfo, error) {
	tasks, err := task.LoadAllFrom(projectDir)
	if err != nil {
		return nil, err
	}

	var orphans []OrphanInfo
	for _, t := range tasks {
		// Only check running tasks
		if t.Status != task.StatusRunning {
			continue
		}

		s, err := LoadFrom(projectDir, t.ID)
		if err != nil {
			// If we can't load state but task says running, it's potentially orphaned
			orphans = append(orphans, OrphanInfo{
				TaskID:     t.ID,
				Task:       t,
				OrphanedAt: time.Now(),
				Reason:     "cannot load state file",
			})
			continue
		}

		isOrphaned, reason := s.CheckOrphaned()
		if isOrphaned {
			info := OrphanInfo{
				TaskID:     t.ID,
				State:      s,
				Task:       t,
				OrphanedAt: time.Now(),
				Reason:     reason,
			}
			if s.Execution != nil {
				info.LastPID = s.Execution.PID
				info.LastHostname = s.Execution.Hostname
			}
			orphans = append(orphans, info)
		}
	}

	return orphans, nil
}

// MarkOrphanedAsInterrupted marks an orphaned task as interrupted.
// This allows it to be resumed later.
func MarkOrphanedAsInterrupted(projectDir, taskID string) error {
	t, err := task.LoadFrom(projectDir, taskID)
	if err != nil {
		return err
	}

	s, err := LoadFrom(projectDir, taskID)
	if err != nil {
		return err
	}

	// Mark the current phase as interrupted
	if s.CurrentPhase != "" {
		s.InterruptPhase(s.CurrentPhase)
	} else {
		s.Status = StatusInterrupted
	}

	// Clear the stale execution info
	s.ClearExecution()

	// Save state
	taskDir := task.TaskDirIn(projectDir, taskID)
	if err := s.SaveTo(taskDir); err != nil {
		return err
	}

	// Update task status to blocked (resumable)
	t.Status = task.StatusBlocked
	if err := t.SaveTo(taskDir); err != nil {
		return err
	}

	// Auto-commit: orphan recovery
	commitOrphanRecovery(projectDir, t)

	return nil
}

// commitOrphanRecovery commits the orphan recovery state change if auto-commit is enabled.
func commitOrphanRecovery(projectDir string, t *task.Task) {
	// Load config to check if auto-commit is disabled
	cfg, err := config.Load()
	if err != nil {
		return // Skip commit on config error
	}
	if cfg.Tasks.DisableAutoCommit {
		return
	}

	// Find project root for git operations
	root := projectDir
	if root == "" {
		root, err = config.FindProjectRoot()
		if err != nil {
			return
		}
	}

	commitCfg := task.CommitConfig{
		ProjectRoot:  root,
		CommitPrefix: cfg.CommitPrefix,
		Logger:       slog.Default(),
	}

	if err := task.CommitStatusChange(t, "orphan-recovered", commitCfg); err != nil {
		slog.Default().Warn("failed to auto-commit orphan recovery", "task", t.ID, "error", err)
	}
}
