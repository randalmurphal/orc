// provider_adapter.go provides the ProviderAdapter abstraction that isolates
// provider-specific behavior from the shared orchestration loop in executeWithProvider().
// Each adapter handles session management, executor config wiring, and post-turn
// persistence while the shared loop handles retry, quality checks, and token accumulation.
package executor

import (
	"github.com/google/uuid"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// ProviderAdapter encapsulates provider-specific behavior around the shared
// orchestration loop. Package-internal — adapters access WorkflowExecutor directly.
type ProviderAdapter interface {
	// Name returns the provider name for logging and error messages.
	Name() string
	// PrepareExecution handles provider-specific setup: session management,
	// model normalization, env vars, phase settings. May modify cfg.
	PrepareExecution(cfg *PhaseExecutionConfig, we *WorkflowExecutor) (*ProviderExecContext, error)
	// BuildTurnExecutorConfig creates the TurnExecutorConfig for this provider.
	BuildTurnExecutorConfig(cfg *PhaseExecutionConfig, pctx *ProviderExecContext, we *WorkflowExecutor) TurnExecutorConfig
	// PostTurn handles provider-specific work after each turn (e.g., session persistence).
	PostTurn(turnResult *TurnResult, pctx *ProviderExecContext, cfg *PhaseExecutionConfig, we *WorkflowExecutor) error
}

// ProviderExecContext carries state between PrepareExecution and the orchestration loop.
type ProviderExecContext struct {
	SessionID    string // Pre-assigned UUID (Claude) or empty (Codex, captured from response)
	ShouldResume bool   // Whether to resume an existing session
	Prompt       string // Initial prompt (original or "Continue where you left off.")
}

// providerAdapterFor returns the appropriate adapter for the given provider.
func providerAdapterFor(provider string) ProviderAdapter {
	if isCodexFamilyProvider(provider) {
		return &codexAdapter{}
	}
	return &claudeAdapter{}
}

// checkResumeSession checks if we should resume an interrupted session.
// Shared logic for both Claude and Codex — resumes only PENDING phases
// with a stored session ID. Does NOT resume FAILED phases.
func checkResumeSession(we *WorkflowExecutor, phaseID string) (sessionID string, shouldResume bool) {
	if we.task == nil || !we.isResuming {
		return "", false
	}
	if shouldStartFreshRetryPhase(we.task, phaseID) {
		return "", false
	}
	if shouldStartFreshBlockedReviewPhase(we.task, phaseID) {
		return "", false
	}
	if we.task.Execution == nil || we.task.Execution.Phases == nil {
		return "", false
	}
	ps, ok := we.task.Execution.Phases[phaseID]
	if !ok {
		return "", false
	}
	storedSessionID := ""
	if ps.SessionId != nil {
		storedSessionID = *ps.SessionId
	}
	if storedSessionID != "" && ps.Status == orcv1.PhaseStatus_PHASE_STATUS_PENDING {
		return storedSessionID, true
	}
	return "", false
}

func shouldStartFreshBlockedReviewPhase(t *orcv1.Task, phaseID string) bool {
	if t == nil {
		return false
	}
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return false
	}
	return phaseID == "review" || phaseID == "review_cross"
}

func shouldStartFreshRetryPhase(t *orcv1.Task, phaseID string) bool {
	rs := task.GetRetryState(t)
	return rs != nil && rs.ToPhase == phaseID
}

// --- claudeAdapter ---

type claudeAdapter struct{}

func (a *claudeAdapter) Name() string { return "claude" }

func (a *claudeAdapter) PrepareExecution(cfg *PhaseExecutionConfig, we *WorkflowExecutor) (*ProviderExecContext, error) {
	pctx := &ProviderExecContext{Prompt: cfg.Prompt}

	// Enable extended thinking via MAX_THINKING_TOKENS env var
	if cfg.Thinking {
		if cfg.ClaudeConfig == nil {
			cfg.ClaudeConfig = &PhaseClaudeConfig{}
		}
		if cfg.ClaudeConfig.Env == nil {
			cfg.ClaudeConfig.Env = make(map[string]string)
		}
		cfg.ClaudeConfig.Env["MAX_THINKING_TOKENS"] = "31999"
	}

	// Check for session resume
	sessionID, shouldResume := checkResumeSession(we, cfg.PhaseID)
	if shouldResume {
		pctx.SessionID = sessionID
		pctx.ShouldResume = true
		pctx.Prompt = "Continue where you left off."
		we.logger.Info("resuming paused session", "phase", cfg.PhaseID, "session_id", sessionID)
		return pctx, nil
	}

	// Fresh session: pre-assign UUID and save to task state BEFORE execution.
	// This ensures we can resume even if the process is killed mid-turn.
	sessionID = uuid.New().String()
	pctx.SessionID = sessionID
	if we.task != nil {
		task.SetPhaseSessionIDProto(we.task.Execution, cfg.PhaseID, sessionID)
		if saveErr := we.backend.SaveTask(we.task); saveErr != nil {
			we.logger.Warn("failed to save session ID", "phase", cfg.PhaseID, "error", saveErr)
		}
	}

	return pctx, nil
}

func (a *claudeAdapter) BuildTurnExecutorConfig(cfg *PhaseExecutionConfig, pctx *ProviderExecContext, we *WorkflowExecutor) TurnExecutorConfig {
	maxTurns := 0
	if we.maxTurnsOverride != nil {
		maxTurns = *we.maxTurnsOverride
	} else if we.orcConfig != nil {
		maxTurns = we.orcConfig.MaxTurns
	}

	return TurnExecutorConfig{
		Provider:         "claude",
		ClaudePath:       we.claudePath,
		Model:            cfg.Model,
		WorkingDir:       cfg.WorkingDir,
		SessionID:        pctx.SessionID,
		Resume:           pctx.ShouldResume,
		PhaseID:          cfg.PhaseID,
		TaskID:           cfg.TaskID,
		RunID:            cfg.RunID,
		ReviewRound:      cfg.ReviewRound,
		MaxTurns:         maxTurns,
		ProducesArtifact: cfg.PhaseTemplate != nil && cfg.PhaseTemplate.ProducesArtifact,
		ClaudeConfig:     cfg.ClaudeConfig,
		Backend:          we.backend,
		Logger:           we.logger,
		Publisher:        we.publisher,
	}
}

func (a *claudeAdapter) PostTurn(_ *TurnResult, _ *ProviderExecContext, _ *PhaseExecutionConfig, _ *WorkflowExecutor) error {
	// Claude pre-assigns session ID — no post-turn persistence needed.
	return nil
}

// --- codexAdapter ---

type codexAdapter struct{}

func (a *codexAdapter) Name() string { return "codex" }

func (a *codexAdapter) PrepareExecution(cfg *PhaseExecutionConfig, we *WorkflowExecutor) (*ProviderExecContext, error) {
	pctx := &ProviderExecContext{Prompt: cfg.Prompt}

	// Normalize model for Codex execution (strip provider prefix like "ollama/qwen2.5" -> "qwen2.5")
	cfg.Model = normalizeCodexExecutionModel(cfg.Provider, cfg.Model)

	// Check for session resume
	sessionID, shouldResume := checkResumeSession(we, cfg.PhaseID)
	if shouldResume {
		pctx.SessionID = sessionID
		pctx.ShouldResume = true
		pctx.Prompt = "Continue where you left off."
		we.logger.Info("resuming codex session", "phase", cfg.PhaseID, "session_id", sessionID)
		return pctx, nil
	}

	// Fresh call: DON'T pre-assign a session ID.
	// Codex assigns its own thread_id, which we capture from the response.

	// Write .codex/instruction.md if phase has custom instructions
	if we.worktreePath != "" {
		var codexCfg *PhaseCodexConfig
		if cfg.ClaudeConfig != nil {
			codexCfg = cfg.ClaudeConfig.Codex
		}
		if err := applyCodexInstructions(we.effectiveWorkingDir(), codexCfg); err != nil {
			we.logger.Warn("failed to write codex instructions", "error", err)
		}
	}

	return pctx, nil
}

func (a *codexAdapter) BuildTurnExecutorConfig(cfg *PhaseExecutionConfig, pctx *ProviderExecContext, we *WorkflowExecutor) TurnExecutorConfig {
	teCfg := TurnExecutorConfig{
		Provider:                  cfg.Provider,
		CodexPath:                 we.resolveCodexPath(),
		Model:                     cfg.Model,
		WorkingDir:                cfg.WorkingDir,
		SessionID:                 pctx.SessionID,
		Resume:                    pctx.ShouldResume,
		PhaseID:                   cfg.PhaseID,
		TaskID:                    cfg.TaskID,
		RunID:                     cfg.RunID,
		ReviewRound:               cfg.ReviewRound,
		ProducesArtifact:          cfg.PhaseTemplate != nil && cfg.PhaseTemplate.ProducesArtifact,
		LocalProvider:             localCodexProvider(cfg.Provider),
		Backend:                   we.backend,
		Logger:                    we.logger,
		Publisher:                 we.publisher,
		BypassApprovalsAndSandbox: true,
	}

	// Apply config-level Codex defaults first (lowest precedence)
	if we.orcConfig != nil {
		pc := we.orcConfig.Providers.Codex
		if pc.ReasoningEffort != "" {
			teCfg.ReasoningEffort = pc.ReasoningEffort
		}
	}

	// Wire Codex-specific settings from phase config (overrides config defaults)
	if cfg.ClaudeConfig != nil && cfg.ClaudeConfig.Codex != nil {
		cc := cfg.ClaudeConfig.Codex
		if cc.ReasoningEffort != "" {
			teCfg.ReasoningEffort = cc.ReasoningEffort
		}
	}

	// Apply bench phase-model override for reasoning effort (highest precedence)
	if override, ok := we.phaseModelOverrides[cfg.PhaseID]; ok && override.ReasoningEffort != "" {
		teCfg.ReasoningEffort = override.ReasoningEffort
	}

	// (continued from above — wire remaining Codex-specific settings)
	if cfg.ClaudeConfig != nil && cfg.ClaudeConfig.Codex != nil {
		cc := cfg.ClaudeConfig.Codex
		if cc.WebSearchMode != "" {
			teCfg.WebSearchMode = cc.WebSearchMode
		}
		if len(cc.Env) > 0 {
			teCfg.Env = cc.Env
		}
		if len(cc.AddDirs) > 0 {
			teCfg.AddDirs = cc.AddDirs
		}
		if cc.LocalProvider != "" {
			teCfg.LocalProvider = cc.LocalProvider
		}
	}

	// Wire Ollama base_url as OLLAMA_HOST env var when using local provider
	if we.orcConfig != nil && we.orcConfig.Providers.Ollama.BaseURL != "" {
		if teCfg.LocalProvider == "ollama" || cfg.Provider == ProviderOllama {
			if teCfg.Env == nil {
				teCfg.Env = make(map[string]string)
			}
			// Only set if not already overridden by phase config
			if _, ok := teCfg.Env["OLLAMA_HOST"]; !ok {
				teCfg.Env["OLLAMA_HOST"] = we.orcConfig.Providers.Ollama.BaseURL
			}
		}
	}

	return teCfg
}

func (a *codexAdapter) PostTurn(turnResult *TurnResult, _ *ProviderExecContext, cfg *PhaseExecutionConfig, we *WorkflowExecutor) error {
	// Persist codex thread_id to task state for cross-process resume.
	// Codex assigns its own thread_id (unlike Claude where we pre-assign UUID).
	if turnResult.SessionID != "" && we.task != nil {
		task.SetPhaseSessionIDProto(we.task.Execution, cfg.PhaseID, turnResult.SessionID)
		if saveErr := we.backend.SaveTask(we.task); saveErr != nil {
			we.logger.Warn("failed to save codex session ID", "phase", cfg.PhaseID, "error", saveErr)
		}
	}
	return nil
}
