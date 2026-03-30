package executor

import (
	"log/slog"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

// TurnExecutorConfig holds parameters for constructing a TurnExecutor.
type TurnExecutorConfig struct {
	Provider    string // "claude" or "codex"
	Model       string
	WorkingDir  string
	SessionID   string
	Resume      bool
	PhaseID     string
	TaskID      string
	RunID       string
	ReviewRound int
	MaxTurns    int

	// Executor paths
	ClaudePath string
	CodexPath  string

	// Claude-specific
	ProducesArtifact bool
	RuntimeConfig    *PhaseRuntimeConfig

	// Codex-specific
	BypassApprovalsAndSandbox bool              // Always true for orc execution
	ReasoningEffort           string            // Codex model_reasoning_effort
	WebSearchMode             string            // Codex web_search mode
	Env                       map[string]string // Additional env vars for codex process
	AddDirs                   []string          // Additional accessible directories

	// Shared
	Backend   storage.Backend
	Logger    *slog.Logger
	Publisher *events.PublishHelper
}

// NewTurnExecutor creates the appropriate TurnExecutor based on provider.
// Claude-family providers use ClaudeExecutor, Codex-family use CodexExecutor.
func NewTurnExecutor(cfg TurnExecutorConfig) TurnExecutor {
	if isCodexFamilyProvider(cfg.Provider) {
		return newCodexTurnExecutor(cfg)
	}
	return newClaudeTurnExecutor(cfg)
}

func newClaudeTurnExecutor(cfg TurnExecutorConfig) TurnExecutor {
	opts := []ClaudeExecutorOption{
		WithClaudePath(cfg.ClaudePath),
		WithClaudeWorkdir(cfg.WorkingDir),
		WithClaudeModel(cfg.Model),
		WithClaudeSessionID(cfg.SessionID),
		WithClaudeMaxTurns(cfg.MaxTurns),
		WithClaudeLogger(cfg.Logger),
		WithClaudePhaseID(cfg.PhaseID),
		WithClaudeProducesArtifact(cfg.ProducesArtifact),
		WithClaudeBackend(cfg.Backend),
		WithClaudeTaskID(cfg.TaskID),
		WithClaudeRunID(cfg.RunID),
	}

	if cfg.ReviewRound > 0 {
		opts = append(opts, WithClaudeReviewRound(cfg.ReviewRound))
	}
	if cfg.RuntimeConfig != nil {
		opts = append(opts, WithPhaseRuntimeConfig(cfg.RuntimeConfig))
	}
	if cfg.Resume {
		opts = append(opts, WithClaudeResume(true))
	}

	return NewClaudeExecutor(opts...)
}

func newCodexTurnExecutor(cfg TurnExecutorConfig) TurnExecutor {
	opts := []CodexExecutorOption{
		WithCodexWorkdir(cfg.WorkingDir),
		WithCodexModel(cfg.Model),
		WithCodexSessionID(cfg.SessionID),
		WithCodexLogger(cfg.Logger),
		WithCodexPhaseID(cfg.PhaseID),
		WithCodexProducesArtifact(cfg.ProducesArtifact),
		WithCodexBackend(cfg.Backend),
		WithCodexTaskID(cfg.TaskID),
		WithCodexRunID(cfg.RunID),
		WithCodexPublisher(cfg.Publisher),
	}

	if cfg.CodexPath != "" {
		opts = append(opts, WithCodexPath(cfg.CodexPath))
	}
	if cfg.ReviewRound > 0 {
		opts = append(opts, WithCodexReviewRound(cfg.ReviewRound))
	}
	if cfg.BypassApprovalsAndSandbox {
		opts = append(opts, WithCodexBypassApprovalsAndSandbox(true))
	}
	if cfg.Resume {
		opts = append(opts, WithCodexResume(true))
	}
	if cfg.ReasoningEffort != "" {
		opts = append(opts, WithCodexReasoningEffort(cfg.ReasoningEffort))
	}
	if cfg.WebSearchMode != "" {
		opts = append(opts, WithCodexWebSearchMode(cfg.WebSearchMode))
	}
	if len(cfg.Env) > 0 {
		opts = append(opts, WithCodexEnv(cfg.Env))
	}
	if len(cfg.AddDirs) > 0 {
		opts = append(opts, WithCodexAddDirs(cfg.AddDirs))
	}

	return NewCodexExecutor(opts...)
}
