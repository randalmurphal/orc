package executor

import (
	"log/slog"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
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

// recordCostToGlobal logs cost and token usage to the global database for cross-project analytics.
// This enables cost aggregation by model, project, time period, etc.
// Failures are logged but don't interrupt execution.
func (e *Executor) recordCostToGlobal(t *task.Task, phaseID string, result *Result, meta CostMetadata) {
	if e.globalDB == nil {
		return // Global DB not available, skip silently
	}

	// Build project ID from path (use normalized path as identifier)
	projectPath := e.config.WorkDir
	if projectPath == "" {
		projectPath = "unknown"
	}

	entry := db.CostEntry{
		ProjectID:           projectPath,
		TaskID:              t.ID,
		Phase:               phaseID,
		Model:               meta.Model,
		Iteration:           meta.Iteration,
		CostUSD:             result.CostUSD,
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		CacheCreationTokens: result.CacheCreationTokens,
		CacheReadTokens:     result.CacheReadTokens,
		TotalTokens:         result.InputTokens + result.OutputTokens,
		InitiativeID:        t.InitiativeID,
		DurationMs:          meta.Duration.Milliseconds(),
		Timestamp:           time.Now(),
	}

	RecordCostEntry(e.globalDB, entry, e.logger)
}
