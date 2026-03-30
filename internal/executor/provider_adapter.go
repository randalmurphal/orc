// provider_adapter.go provides the ProviderAdapter abstraction that isolates
// provider-specific behavior from the shared orchestration loop in executeWithProvider().
// Each adapter handles session management, executor config wiring, and post-turn
// persistence while the shared loop handles retry, quality checks, and token accumulation.
package executor

import (
	"fmt"

	"github.com/google/uuid"
	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// ProviderAdapter encapsulates provider-specific behavior around the shared
// orchestration loop. Package-internal — adapters access WorkflowExecutor directly.
type ProviderAdapter interface {
	// Name returns the provider name for logging and error messages.
	Name() string
	// PrepareExecution handles provider-specific setup: session management,
	// model normalization, env vars, runtime preparation inputs. May modify cfg.
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
// with valid stored session metadata. Does NOT resume FAILED phases.
func checkResumeSession(we *WorkflowExecutor, phaseID string) (sessionID string, shouldResume bool, err error) {
	if we.task == nil || !we.isResuming {
		return "", false, nil
	}
	if shouldStartFreshRetryPhase(we.task, phaseID) {
		return "", false, nil
	}
	if shouldStartFreshReviewPhase(phaseID) {
		return "", false, nil
	}
	if we.task.Execution == nil || we.task.Execution.Phases == nil {
		return "", false, nil
	}
	ps, ok := we.task.Execution.Phases[phaseID]
	if !ok {
		return "", false, nil
	}
	storedSessionID := ""
	if ps.SessionMetadata != nil {
		session, err := llmkit.ParseSessionMetadata(*ps.SessionMetadata)
		if err != nil {
			return "", false, fmt.Errorf("parse stored session metadata for phase %s: %w", phaseID, err)
		}
		storedSessionID = llmkit.SessionID(session)
	}
	if storedSessionID != "" && ps.Status == orcv1.PhaseStatus_PHASE_STATUS_PENDING {
		return storedSessionID, true, nil
	}
	return "", false, nil
}

func shouldStartFreshReviewPhase(phaseID string) bool {
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
		if cfg.RuntimeConfig == nil {
			cfg.RuntimeConfig = &PhaseRuntimeConfig{}
		}
		if cfg.RuntimeConfig.Shared.Env == nil {
			cfg.RuntimeConfig.Shared.Env = make(map[string]string)
		}
		cfg.RuntimeConfig.Shared.Env["MAX_THINKING_TOKENS"] = "31999"
	}

	// Check for session resume
	sessionID, shouldResume, err := checkResumeSession(we, cfg.PhaseID)
	if err != nil {
		return nil, err
	}
	if shouldResume {
		pctx.SessionID = sessionID
		pctx.ShouldResume = true
		pctx.Prompt = "Continue where you left off."
		we.logger.Info("resuming paused session", "phase", cfg.PhaseID, "session_id", sessionID)
		return pctx, nil
	}

	// Fresh session: pre-assign UUID and save to task state before execution.
	sessionID = uuid.New().String()
	pctx.SessionID = sessionID
	if we.task != nil {
		sessionMetadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(ProviderClaude, sessionID))
		if err != nil {
			return nil, fmt.Errorf("marshal claude session metadata: %w", err)
		}
		task.SetPhaseSessionMetadataProto(we.task.Execution, cfg.PhaseID, sessionMetadata)
		if saveErr := we.saveTaskStrict(we.task, fmt.Sprintf("persist claude session metadata for phase %s", cfg.PhaseID)); saveErr != nil {
			return nil, saveErr
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
		RuntimeConfig:    cfg.RuntimeConfig,
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

	// Check for session resume
	sessionID, shouldResume, err := checkResumeSession(we, cfg.PhaseID)
	if err != nil {
		return nil, err
	}
	if shouldResume {
		pctx.SessionID = sessionID
		pctx.ShouldResume = true
		pctx.Prompt = "Continue where you left off."
		we.logger.Info("resuming codex session", "phase", cfg.PhaseID, "session_id", sessionID)
		return pctx, nil
	}

	// Fresh call: don't pre-assign a session ID. Codex assigns its own thread_id,
	// which we capture from the response and persist as opaque llmkit session metadata.

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
	if cfg.RuntimeConfig != nil && cfg.RuntimeConfig.Providers.Codex != nil {
		cc := cfg.RuntimeConfig.Providers.Codex
		if cc.ReasoningEffort != "" {
			teCfg.ReasoningEffort = cc.ReasoningEffort
		}
	}

	// Apply bench phase-model override for reasoning effort (highest precedence)
	if override, ok := we.phaseModelOverrides[cfg.PhaseID]; ok && override.ReasoningEffort != "" {
		teCfg.ReasoningEffort = override.ReasoningEffort
	}

	// (continued from above — wire remaining Codex-specific settings)
	if cfg.RuntimeConfig != nil && cfg.RuntimeConfig.Providers.Codex != nil {
		cc := cfg.RuntimeConfig.Providers.Codex
		if cc.WebSearchMode != "" {
			teCfg.WebSearchMode = cc.WebSearchMode
		}
	}

	if cfg.RuntimeConfig != nil {
		if len(cfg.RuntimeConfig.Shared.Env) > 0 {
			teCfg.Env = cfg.RuntimeConfig.Shared.Env
		}
		if len(cfg.RuntimeConfig.Shared.AddDirs) > 0 {
			teCfg.AddDirs = cfg.RuntimeConfig.Shared.AddDirs
		}
	}

	return teCfg
}

func (a *codexAdapter) PostTurn(turnResult *TurnResult, _ *ProviderExecContext, cfg *PhaseExecutionConfig, we *WorkflowExecutor) error {
	// Persist codex thread_id to task state for cross-process resume.
	// Codex assigns its own thread_id (unlike Claude where we pre-assign UUID).
	if turnResult.SessionID != "" && we.task != nil {
		sessionMetadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(ProviderCodex, turnResult.SessionID))
		if err != nil {
			return fmt.Errorf("marshal codex session metadata: %w", err)
		}
		task.SetPhaseSessionMetadataProto(we.task.Execution, cfg.PhaseID, sessionMetadata)
		if saveErr := we.saveTaskStrict(we.task, fmt.Sprintf("persist codex session metadata for phase %s", cfg.PhaseID)); saveErr != nil {
			return saveErr
		}
	}
	return nil
}
