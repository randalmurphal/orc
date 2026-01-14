// Package task provides task management functionality.
package task

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
)

// CommitConfig configures the auto-commit behavior.
type CommitConfig struct {
	ProjectRoot  string
	CommitPrefix string
	Logger       *slog.Logger
}

// DefaultCommitConfig returns sensible defaults.
func DefaultCommitConfig() CommitConfig {
	return CommitConfig{
		CommitPrefix: "[orc]",
	}
}

// CommitAndSync commits a task file to git.
// This should be called after any task modification via CLI.
func CommitAndSync(t *Task, action string, cfg CommitConfig) error {
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		var err error
		projectRoot, err = config.FindProjectRoot()
		if err != nil {
			return fmt.Errorf("find project root: %w", err)
		}
	}

	commitPrefix := cfg.CommitPrefix
	if commitPrefix == "" {
		commitPrefix = "[orc]"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Stage the task directory (includes task.yaml, plan.yaml, state.yaml, etc.)
	taskPath := filepath.Join(projectRoot, OrcDir, TasksDir, t.ID)
	if err := gitAdd(projectRoot, taskPath); err != nil {
		logger.Warn("failed to stage task files", "id", t.ID, "error", err)
		return nil // Non-blocking - file is saved
	}

	// Create commit message
	msg := fmt.Sprintf("%s task %s: %s - %s", commitPrefix, t.ID, action, t.Title)
	if err := gitCommit(projectRoot, msg); err != nil {
		logger.Warn("failed to commit task", "id", t.ID, "error", err)
	} else {
		logger.Debug("committed task", "id", t.ID, "action", action)
	}

	return nil
}

// CommitDeletion commits a task deletion to git.
func CommitDeletion(id string, cfg CommitConfig) error {
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		var err error
		projectRoot, err = config.FindProjectRoot()
		if err != nil {
			return fmt.Errorf("find project root: %w", err)
		}
	}

	commitPrefix := cfg.CommitPrefix
	if commitPrefix == "" {
		commitPrefix = "[orc]"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Stage the deletion
	taskPath := filepath.Join(projectRoot, OrcDir, TasksDir, id)
	if err := gitAdd(projectRoot, taskPath); err != nil {
		// Directory already deleted, try staging all changes
		if err := gitAddAll(projectRoot); err != nil {
			logger.Warn("failed to stage task deletion", "id", id, "error", err)
		}
	}

	msg := fmt.Sprintf("%s task %s: deleted", commitPrefix, id)
	if err := gitCommit(projectRoot, msg); err != nil {
		logger.Warn("failed to commit task deletion", "id", id, "error", err)
	} else {
		logger.Debug("committed task deletion", "id", id)
	}

	return nil
}

// CommitStatusChange commits a task status change to git.
func CommitStatusChange(t *Task, status string, cfg CommitConfig) error {
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		var err error
		projectRoot, err = config.FindProjectRoot()
		if err != nil {
			return fmt.Errorf("find project root: %w", err)
		}
	}

	commitPrefix := cfg.CommitPrefix
	if commitPrefix == "" {
		commitPrefix = "[orc]"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Stage the task directory
	taskPath := filepath.Join(projectRoot, OrcDir, TasksDir, t.ID)
	if err := gitAdd(projectRoot, taskPath); err != nil {
		logger.Warn("failed to stage task files", "id", t.ID, "error", err)
		return nil
	}

	msg := fmt.Sprintf("%s task %s: status %s - %s", commitPrefix, t.ID, status, t.Title)
	if err := gitCommit(projectRoot, msg); err != nil {
		logger.Warn("failed to commit task status change", "id", t.ID, "error", err)
	} else {
		logger.Debug("committed task status change", "id", t.ID, "status", status)
	}

	return nil
}

// gitAdd stages a file or directory.
func gitAdd(projectRoot, path string) error {
	cmd := exec.Command("git", "-C", projectRoot, "add", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add %s: %s: %w", path, string(output), err)
	}
	return nil
}

// gitAddAll stages all changes.
func gitAddAll(projectRoot string) error {
	cmd := exec.Command("git", "-C", projectRoot, "add", "-A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add -A: %s: %w", string(output), err)
	}
	return nil
}

// gitCommit creates a commit.
func gitCommit(projectRoot, msg string) error {
	cmd := exec.Command("git", "-C", projectRoot, "commit", "-m", msg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if nothing to commit
		if strings.Contains(string(output), "nothing to commit") ||
			strings.Contains(string(output), "no changes added") {
			return nil
		}
		return fmt.Errorf("git commit: %s: %w", string(output), err)
	}
	return nil
}
