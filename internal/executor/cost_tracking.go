package executor

import (
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

	if err := e.globalDB.RecordCostExtended(entry); err != nil {
		e.logger.Warn("failed to record cost to global database",
			"task", t.ID,
			"phase", phaseID,
			"error", err,
		)
	} else {
		e.logger.Debug("recorded cost to global database",
			"task", t.ID,
			"phase", phaseID,
			"cost_usd", result.CostUSD,
			"model", meta.Model,
			"input_tokens", result.InputTokens,
			"output_tokens", result.OutputTokens,
		)
	}
}
