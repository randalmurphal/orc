package executor

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// CostMetadata holds additional metadata for cost tracking.
type CostMetadata struct {
	Model       string        // Model used (e.g., "opus", "sonnet", "haiku")
	Iteration   int           // Iteration count for the phase
	Duration    time.Duration // Phase execution duration
	ProjectPath string        // Normalized project path
}

// TokenRate defines per-1M-token pricing in USD.
type TokenRate struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// providerRates are best-effort defaults used when providers do not return
// explicit cost values. Rates are in USD per 1M tokens.
var providerRates = map[string]map[string]TokenRate{
	"claude": {
		"opus":   {Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
		"sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
		"haiku":  {Input: 0.25, Output: 1.25, CacheRead: 0.03, CacheWrite: 0.3},
	},
	"codex": {
		"gpt-5":   {Input: 2.0, Output: 8.0, CacheRead: 0.0, CacheWrite: 0.0},
		"gpt-4.1": {Input: 2.0, Output: 8.0, CacheRead: 0.0, CacheWrite: 0.0},
	},
	"ollama": {
		"*": {Input: 0.0, Output: 0.0, CacheRead: 0.0, CacheWrite: 0.0},
	},
	"lmstudio": {
		"*": {Input: 0.0, Output: 0.0, CacheRead: 0.0, CacheWrite: 0.0},
	},
}

func cloneProviderRates(src map[string]map[string]TokenRate) map[string]map[string]TokenRate {
	cloned := make(map[string]map[string]TokenRate, len(src))
	for provider, models := range src {
		modelCopy := make(map[string]TokenRate, len(models))
		for model, rate := range models {
			modelCopy[model] = rate
		}
		cloned[provider] = modelCopy
	}
	return cloned
}

// ProviderRatesForConfig merges default rates with config overrides.
// Config rates override defaults per provider+model pair.
func ProviderRatesForConfig(cfg *config.Config) map[string]map[string]TokenRate {
	merged := cloneProviderRates(providerRates)
	if cfg == nil {
		return merged
	}
	for provider, models := range cfg.Providers.Rates {
		p := normalizeProvider(provider)
		if p == "" {
			continue
		}
		if merged[p] == nil {
			merged[p] = make(map[string]TokenRate)
		}
		for model, rateCfg := range models {
			m := strings.ToLower(strings.TrimSpace(model))
			if m == "" {
				continue
			}
			merged[p][m] = TokenRate{
				Input:      rateCfg.Input,
				Output:     rateCfg.Output,
				CacheRead:  rateCfg.CacheRead,
				CacheWrite: rateCfg.CacheWrite,
			}
		}
	}
	return merged
}

// EstimateTokenCostUSD estimates cost from token usage when provider-native
// cost accounting is unavailable. Returns 0 when no rate is known.
func EstimateTokenCostUSD(provider, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64) float64 {
	return EstimateTokenCostUSDWithRates(providerRates, provider, model, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
}

// EstimateTokenCostUSDWithRates estimates cost from token usage using a caller-provided
// rate table. Returns 0 when no rate is known.
func EstimateTokenCostUSDWithRates(rates map[string]map[string]TokenRate, provider, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int64) float64 {
	p := normalizeProvider(strings.TrimSpace(provider))
	if p == "" {
		p = "claude"
	}

	m := strings.ToLower(strings.TrimSpace(model))
	if p == "claude" {
		m = strings.ToLower(db.DetectModel(m))
	}

	providerRateMap, ok := rates[p]
	if !ok {
		return 0
	}

	rate, ok := providerRateMap[m]
	if !ok {
		rate, ok = providerRateMap["*"]
		if !ok {
			return 0
		}
	}

	const perMillion = 1_000_000.0
	cost := (float64(inputTokens) / perMillion * rate.Input) +
		(float64(outputTokens) / perMillion * rate.Output) +
		(float64(cacheReadTokens) / perMillion * rate.CacheRead) +
		(float64(cacheWriteTokens) / perMillion * rate.CacheWrite)
	if cost < 0 {
		return 0
	}
	return cost
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

