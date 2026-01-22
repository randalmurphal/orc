package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events" // events.Publisher for option func
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TrivialExecutor executes phases using ClaudeExecutor with minimal overhead.
// Best for trivial tasks that can be completed in a single prompt.
//
// Session Strategy: No session persistence, each iteration is stateless
// Checkpointing: None
// Iteration Limit: 5 (low, expects quick completion)
type TrivialExecutor struct {
	claudePath string // Path to claude binary
	workingDir string // Working directory for execution
	publisher  *EventPublisher
	logger     *slog.Logger
	config     ExecutorConfig
	backend    storage.Backend // Storage backend for loading initiatives
	orcConfig  *config.Config  // Orc config for model resolution

	// MCP config path (generated for worktree)
	mcpConfigPath string

	// turnExecutor allows injection of a mock for testing
	turnExecutor TurnExecutor
}

// TrivialExecutorOption configures a TrivialExecutor.
type TrivialExecutorOption func(*TrivialExecutor)

// WithTrivialClaudePath sets the path to the claude binary.
func WithTrivialClaudePath(path string) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.claudePath = path }
}

// WithTrivialWorkingDir sets the working directory.
func WithTrivialWorkingDir(dir string) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.workingDir = dir }
}

// WithTrivialPublisher sets the event publisher.
func WithTrivialPublisher(p events.Publisher) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.publisher = NewEventPublisher(p) }
}

// WithTrivialLogger sets the logger.
func WithTrivialLogger(l *slog.Logger) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.logger = l }
}

// WithTrivialConfig sets the execution config.
func WithTrivialConfig(cfg ExecutorConfig) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.config = cfg }
}

// WithTrivialBackend sets the storage backend for loading initiatives.
func WithTrivialBackend(b storage.Backend) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.backend = b }
}

// WithTrivialOrcConfig sets the orc config for model resolution.
func WithTrivialOrcConfig(cfg *config.Config) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.orcConfig = cfg }
}

// WithTrivialMCPConfig sets the MCP config path.
func WithTrivialMCPConfig(path string) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.mcpConfigPath = path }
}

// WithTrivialTurnExecutor sets a mock TurnExecutor for testing.
func WithTrivialTurnExecutor(te TurnExecutor) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.turnExecutor = te }
}

// NewTrivialExecutor creates a new trivial executor.
func NewTrivialExecutor(opts ...TrivialExecutorOption) *TrivialExecutor {
	e := &TrivialExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
		publisher:  NewEventPublisher(nil), // Initialize with nil-safe wrapper
		config: ExecutorConfig{
			MaxIterations:      5,
			CheckpointInterval: 0,
			SessionPersistence: false,
		},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Name returns the executor type name.
func (e *TrivialExecutor) Name() string {
	return "trivial"
}

// Execute runs a phase using ClaudeExecutor without session management.
func (e *TrivialExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Build execution context using centralized builder
	execCtx, err := BuildExecutionContext(ExecutionContextConfig{
		Task:           t,
		Phase:          p,
		State:          s,
		Backend:        e.backend,
		WorkingDir:     e.workingDir,
		MCPConfigPath:  e.mcpConfigPath,
		ExecutorConfig: e.config,
		OrcConfig:      e.orcConfig,
		Logger:         e.logger,
	})
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("build execution context: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	result.Model = execCtx.ModelSetting.Model

	// Use injected TurnExecutor if available (for testing), otherwise create ClaudeExecutor
	var turnExec TurnExecutor
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		turnExec = NewClaudeExecutorFromContext(execCtx, e.claudePath, e.config.MaxIterations, e.logger)
	}

	// Simple iteration loop - no session persistence, each turn is standalone
	promptText := execCtx.PromptText
	var lastResponse string

	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Execute turn using TurnExecutor with JSON schema
		turnResult, err := turnExec.ExecuteTurn(ctx, promptText)

		if err != nil {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("execute turn %d: %w", iteration, err)
			break
		}

		// Track tokens using effective input (includes cache)
		result.InputTokens += turnResult.Usage.EffectiveInputTokens()
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.CacheCreationTokens += turnResult.Usage.CacheCreationInputTokens
		result.CacheReadTokens += turnResult.Usage.CacheReadInputTokens
		result.CostUSD += turnResult.CostUSD
		result.Iterations = iteration
		lastResponse = turnResult.Content

		e.publisher.Transcript(t.ID, p.ID, iteration, "response", turnResult.Content)

		// Handle completion status
		switch turnResult.Status {
		case PhaseStatusComplete:
			result.Status = plan.PhaseCompleted
			result.Output = turnResult.Content
			e.logger.Info("phase complete (trivial)", "task", t.ID, "phase", p.ID, "iterations", iteration)
			goto done

		case PhaseStatusBlocked:
			result.Status = plan.PhaseFailed
			result.Output = turnResult.Content // Preserve output for retry context
			result.Error = fmt.Errorf("phase blocked: %s", turnResult.Reason)
			goto done

		case PhaseStatusContinue:
			// For trivial executor, add response to prompt for next iteration
			// (stateless, so we concatenate prior context)
			promptText = fmt.Sprintf("%s\n\nAssistant's previous response:\n%s\n\nContinue working on the task.",
				promptText, turnResult.Content)
		}

		// Check for errors
		if turnResult.IsError {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("LLM error: %s", turnResult.ErrorText)
			result.Output = lastResponse
			goto done
		}
	}

	if result.Status == plan.PhaseRunning {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached", e.config.MaxIterations)
	}

done:
	if result.Output == "" {
		result.Output = lastResponse
	}
	result.Duration = time.Since(start)

	// Save artifact on success (spec is saved centrally in task_execution.go with fail-fast logic)
	if result.Status == plan.PhaseCompleted && result.Output != "" {
		if saved, err := SaveArtifactToDatabase(e.backend, t.ID, p.ID, result.Output); err != nil {
			e.logger.Warn("failed to save phase artifact to database", "error", err)
		} else if saved {
			e.logger.Info("saved phase artifact to database", "phase", p.ID)
		}
	}

	return result, result.Error
}
