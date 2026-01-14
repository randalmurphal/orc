// Package state provides execution state tracking for orc tasks.
package state

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
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

// CommitTaskState commits all task files (task.yaml, state.yaml, plan.yaml) to git.
// This should be called after significant state changes like phase transitions.
// The action parameter describes what changed (e.g., "implement phase started",
// "test phase completed").
func CommitTaskState(taskID, action string, cfg CommitConfig) error {
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

	// Stage the entire task directory (includes task.yaml, state.yaml, plan.yaml, etc.)
	taskPath := filepath.Join(projectRoot, task.OrcDir, task.TasksDir, taskID)
	if err := gitAdd(projectRoot, taskPath); err != nil {
		logger.Warn("failed to stage task files", "id", taskID, "error", err)
		return nil // Non-blocking - files are saved
	}

	// Create commit message
	msg := fmt.Sprintf("%s task %s: %s", commitPrefix, taskID, action)
	if err := gitCommit(projectRoot, msg); err != nil {
		logger.Warn("failed to commit task state", "id", taskID, "error", err)
	} else {
		logger.Debug("committed task state", "id", taskID, "action", action)
	}

	return nil
}

// CommitPhaseTransition commits state after a phase transition.
// This is a convenience wrapper for common phase actions.
func CommitPhaseTransition(taskID, phase, transition string, cfg CommitConfig) error {
	action := fmt.Sprintf("%s phase %s", phase, transition)
	return CommitTaskState(taskID, action, cfg)
}

// CommitExecutionState commits current execution state with a descriptive action.
// Use this for significant execution events like retries, errors, or completions.
func CommitExecutionState(taskID, description string, cfg CommitConfig) error {
	return CommitTaskState(taskID, description, cfg)
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
