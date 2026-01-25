// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// getBackend creates a storage backend for the current project.
// Most CLI commands that need to access task data should use this.
// Returns an error if not in an orc project (do not fall back to "." as
// that could create a database in the wrong location, e.g., inside a worktree).
func getBackend() (storage.Backend, error) {
	// Find project root (works from worktrees too)
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("not in an orc project (run 'orc init' first): %w", err)
	}

	// Load config for storage settings
	var storageCfg *config.StorageConfig
	cfg, err := config.Load()
	if err == nil && cfg != nil {
		storageCfg = &cfg.Storage
	}

	return storage.NewDatabaseBackend(projectRoot, storageCfg)
}

// Helper functions

// statusIcon returns an icon for task status, respecting the --plain flag
func statusIcon(status task.Status) string {
	if plain {
		return statusText(status)
	}
	switch status {
	case task.StatusCreated:
		return "📝"
	case task.StatusClassifying:
		return "🔍"
	case task.StatusPlanned:
		return "📋"
	case task.StatusRunning:
		return "⏳"
	case task.StatusPaused:
		return "⏸️"
	case task.StatusBlocked:
		return "🚫"
	case task.StatusFinalizing:
		return "🏁"
	case task.StatusCompleted:
		return "✅"
	case task.StatusFailed:
		return "❌"
	case task.StatusResolved:
		return "🔧"
	default:
		return "❓"
	}
}

// statusText returns a plain text status indicator
func statusText(status task.Status) string {
	switch status {
	case task.StatusCreated:
		return "[NEW]"
	case task.StatusClassifying:
		return "[...]"
	case task.StatusPlanned:
		return "[RDY]"
	case task.StatusRunning:
		return "[RUN]"
	case task.StatusPaused:
		return "[PSE]"
	case task.StatusBlocked:
		return "[BLK]"
	case task.StatusFinalizing:
		return "[FIN]"
	case task.StatusCompleted:
		return "[OK]"
	case task.StatusFailed:
		return "[ERR]"
	case task.StatusResolved:
		return "[RSV]"
	default:
		return "[???]"
	}
}

// phaseStatusIcon returns an icon for phase status
func phaseStatusIcon(status task.PhaseStatus) string {
	// Phase status icons are already ASCII-safe, no plain mode needed
	switch status {
	case task.PhaseStatusPending:
		return "○"
	case task.PhaseStatusRunning:
		return "◐"
	case task.PhaseStatusCompleted:
		return "●"
	case task.PhaseStatusFailed:
		return "✗"
	case task.PhaseStatusSkipped:
		return "⊘"
	default:
		return "?"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
