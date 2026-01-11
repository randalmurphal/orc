// Package cli implements the orc command-line interface.
package cli

import (
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// Helper functions

func statusIcon(status task.Status) string {
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
	case task.StatusCompleted:
		return "✅"
	case task.StatusFailed:
		return "❌"
	default:
		return "❓"
	}
}

func phaseStatusIcon(status plan.PhaseStatus) string {
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
