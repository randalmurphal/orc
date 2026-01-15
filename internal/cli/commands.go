// Package cli implements the orc command-line interface.
package cli

import (
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// getBackend creates a storage backend for the current project.
// Most CLI commands that need to access task data should use this.
func getBackend() (storage.Backend, error) {
	// Find project root (works from worktrees too)
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		// If not in a project, use current directory
		projectRoot = "."
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
	case task.StatusFinished:
		return "📦"
	case task.StatusFailed:
		return "❌"
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
	case task.StatusFinished:
		return "[END]"
	case task.StatusFailed:
		return "[ERR]"
	default:
		return "[???]"
	}
}

// phaseStatusIcon returns an icon for phase status
func phaseStatusIcon(status plan.PhaseStatus) string {
	// Phase status icons are already ASCII-safe, no plain mode needed
	switch status {
	case plan.PhasePending:
		return "○"
	case plan.PhaseRunning:
		return "◐"
	case plan.PhaseCompleted:
		return "●"
	case plan.PhaseFailed:
		return "✗"
	case plan.PhaseSkipped:
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
