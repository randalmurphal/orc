// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// getBackend creates a storage backend for the current project.
// Most CLI commands that need to access task data should use this.
// Returns an error if not in an orc project (do not fall back to "." as
// that could create a database in the wrong location, e.g., inside a worktree).
func getBackend() (storage.Backend, error) {
	// Find project root (works from worktrees too)
	projectRoot, err := ResolveProjectPath()
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
func statusIcon(status orcv1.TaskStatus) string {
	if plain {
		return statusText(status)
	}
	switch status {
	case orcv1.TaskStatus_TASK_STATUS_CREATED:
		return "📝"
	case orcv1.TaskStatus_TASK_STATUS_CLASSIFYING:
		return "🔍"
	case orcv1.TaskStatus_TASK_STATUS_PLANNED:
		return "📋"
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return "⏳"
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return "⏸️"
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return "🚫"
	case orcv1.TaskStatus_TASK_STATUS_FINALIZING:
		return "🏁"
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return "✅"
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return "❌"
	case orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return "🔧"
	default:
		return "❓"
	}
}

// statusText returns a plain text status indicator
func statusText(status orcv1.TaskStatus) string {
	switch status {
	case orcv1.TaskStatus_TASK_STATUS_CREATED:
		return "[NEW]"
	case orcv1.TaskStatus_TASK_STATUS_CLASSIFYING:
		return "[...]"
	case orcv1.TaskStatus_TASK_STATUS_PLANNED:
		return "[RDY]"
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return "[RUN]"
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return "[PSE]"
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return "[BLK]"
	case orcv1.TaskStatus_TASK_STATUS_FINALIZING:
		return "[FIN]"
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return "[OK]"
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return "[ERR]"
	case orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return "[RSV]"
	default:
		return "[???]"
	}
}

// phaseStatusIcon returns an icon for phase status
func phaseStatusIcon(status orcv1.PhaseStatus) string {
	// Phase status is completion-only: PENDING, COMPLETED, SKIPPED
	// Running/failed state is derived from task status + current_phase
	switch status {
	case orcv1.PhaseStatus_PHASE_STATUS_PENDING:
		return "○"
	case orcv1.PhaseStatus_PHASE_STATUS_COMPLETED:
		return "●"
	case orcv1.PhaseStatus_PHASE_STATUS_SKIPPED:
		return "⊘"
	default:
		return "○" // Treat unknown as pending
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// weightStringProto returns a string representation of the weight enum
func weightStringProto(w orcv1.TaskWeight) string {
	switch w {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return "trivial"
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return "small"
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return "medium"
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return "large"
	default:
		return "unknown"
	}
}

// matchStatusProto returns true if the proto status matches the filter string
func matchStatusProto(status orcv1.TaskStatus, filter string) bool {
	// Map common filter strings to proto enum values
	switch filter {
	case "created", "new":
		return status == orcv1.TaskStatus_TASK_STATUS_CREATED
	case "classifying":
		return status == orcv1.TaskStatus_TASK_STATUS_CLASSIFYING
	case "planned", "ready":
		return status == orcv1.TaskStatus_TASK_STATUS_PLANNED
	case "running":
		return status == orcv1.TaskStatus_TASK_STATUS_RUNNING
	case "paused":
		return status == orcv1.TaskStatus_TASK_STATUS_PAUSED
	case "blocked":
		return status == orcv1.TaskStatus_TASK_STATUS_BLOCKED
	case "finalizing":
		return status == orcv1.TaskStatus_TASK_STATUS_FINALIZING
	case "completed", "done":
		return status == orcv1.TaskStatus_TASK_STATUS_COMPLETED
	case "failed":
		return status == orcv1.TaskStatus_TASK_STATUS_FAILED
	case "resolved":
		return status == orcv1.TaskStatus_TASK_STATUS_RESOLVED
	case "pending": // "pending" is a meta-filter for created+planned
		return status == orcv1.TaskStatus_TASK_STATUS_CREATED ||
			status == orcv1.TaskStatus_TASK_STATUS_PLANNED
	default:
		return false
	}
}

// matchWeightProto returns true if the proto weight matches the filter string
func matchWeightProto(weight orcv1.TaskWeight, filter string) bool {
	switch filter {
	case "trivial":
		return weight == orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	case "small":
		return weight == orcv1.TaskWeight_TASK_WEIGHT_SMALL
	case "medium":
		return weight == orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	case "large":
		return weight == orcv1.TaskWeight_TASK_WEIGHT_LARGE
	default:
		return false
	}
}
