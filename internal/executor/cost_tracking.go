package executor

import (
	"log/slog"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// CostMetadata holds additional metadata for cost tracking.
type CostMetadata struct {
	Model       string        // Model used (e.g., "opus", "sonnet", "haiku")
	Iteration   int           // Iteration count for the phase
	Duration    time.Duration // Phase execution duration
	ProjectPath string        // Normalized project path
}

// RecordCostEntry records a cost entry to the global database.
// Used by both WorkflowExecutor and Executor for cross-project analytics.
// Failures are logged but don't interrupt execution.
func RecordCostEntry(globalDB *db.GlobalDB, entry db.CostEntry, logger *slog.Logger) {
	if globalDB == nil {
		return // Global DB not available, skip silently
	}

	if err := globalDB.RecordCostExtended(entry); err != nil {
		logger.Warn("failed to record cost to global database",
			"task", entry.TaskID,
			"phase", entry.Phase,
			"error", err,
		)
	} else {
		logger.Debug("recorded cost to global database",
			"task", entry.TaskID,
			"phase", entry.Phase,
			"cost_usd", entry.CostUSD,
			"model", entry.Model,
			"input_tokens", entry.InputTokens,
			"output_tokens", entry.OutputTokens,
		)
	}
}

