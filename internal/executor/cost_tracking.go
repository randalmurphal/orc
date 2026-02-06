package executor

import (
	"fmt"
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

// checkBudget checks budget status and returns an error if over budget.
// Returns nil if budget check passes or should be skipped.
// Budget enforcement is best-effort: DB errors log a warning and allow execution.
func (we *WorkflowExecutor) checkBudget(ignoreBudget bool) error {
	if we.globalDB == nil {
		return nil // No global DB → no budget check
	}

	status, err := we.globalDB.GetBudgetStatus(we.workingDir)
	if err != nil {
		we.logger.Warn("budget check failed, proceeding anyway",
			"error", err,
		)
		return nil // Best-effort: don't block on DB errors
	}
	if status == nil {
		return nil // No budget configured for this project
	}
	if status.MonthlyLimitUSD == 0 {
		return nil // Limit=0 means enforcement is disabled
	}

	if status.OverBudget {
		if ignoreBudget {
			we.logger.Warn("budget exceeded, proceeding with --ignore-budget",
				"spent", status.CurrentMonthSpent,
				"limit", status.MonthlyLimitUSD,
			)
			return nil
		}
		return fmt.Errorf(
			"budget exceeded: $%.0f spent of $%.0f limit for %s — use --ignore-budget to proceed",
			status.CurrentMonthSpent, status.MonthlyLimitUSD, status.CurrentMonth,
		)
	}

	if status.AtAlertThreshold {
		we.logger.Warn("budget alert: approaching monthly limit",
			"spent", status.CurrentMonthSpent,
			"limit", status.MonthlyLimitUSD,
			"percent_used", status.PercentUsed,
			"threshold", status.AlertThreshold,
		)
	}

	return nil
}

