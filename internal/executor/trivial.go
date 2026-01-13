package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/events" // events.Publisher for option func
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// TrivialExecutor executes phases using fire-and-forget LLM calls.
// No session continuity, minimal overhead. Best for trivial tasks
// that can be completed in a single prompt.
//
// Session Strategy: No session, just a single completion call
// Checkpointing: None
// Iteration Limit: 5 (low, expects quick completion)
type TrivialExecutor struct {
	client    claude.Client
	publisher *EventPublisher
	logger    *slog.Logger
	config    ExecutorConfig
}

// TrivialExecutorOption configures a TrivialExecutor.
type TrivialExecutorOption func(*TrivialExecutor)

// WithTrivialClient sets the LLM client.
func WithTrivialClient(client claude.Client) TrivialExecutorOption {
	return func(e *TrivialExecutor) { e.client = client }
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

// NewTrivialExecutor creates a new trivial executor.
func NewTrivialExecutor(opts ...TrivialExecutorOption) *TrivialExecutor {
	e := &TrivialExecutor{
		logger:    slog.Default(),
		publisher: NewEventPublisher(nil), // Initialize with nil-safe wrapper
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

// Execute runs a phase using simple LLM calls without session management.
func (e *TrivialExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	if e.client == nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("no LLM client configured")
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Load and render prompt using shared template module
	tmpl, err := LoadPromptTemplate(p)
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("load prompt: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}
	vars := BuildTemplateVars(t, p, s, 0, "")
	promptText := RenderTemplate(tmpl, vars)

	// Simple iteration loop - no session, just repeated completions
	var lastResponse string
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Execute single completion
		resp, err := e.client.Complete(ctx, claude.CompletionRequest{
			Messages: []claude.Message{
				{Role: claude.RoleUser, Content: promptText},
			},
			Model: e.config.Model,
		})

		if err != nil {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("completion failed: %w", err)
			break
		}

		// Use effective input tokens (includes cache) to show actual context size
		// Note: claude.TokenUsage doesn't have EffectiveInputTokens method, so compute directly
		result.InputTokens += resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens
		result.OutputTokens += resp.Usage.OutputTokens
		result.Iterations = iteration
		lastResponse = resp.Content

		e.publisher.Transcript(t.ID, p.ID, iteration, "response", resp.Content)

		// Check completion markers
		status, reason := CheckPhaseCompletion(resp.Content)
		switch status {
		case PhaseStatusComplete:
			result.Status = plan.PhaseCompleted
			result.Output = resp.Content
			e.logger.Info("phase complete (trivial)", "task", t.ID, "phase", p.ID, "iterations", iteration)
			goto done

		case PhaseStatusBlocked:
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("phase blocked: %s", reason)
			goto done

		case PhaseStatusContinue:
			// For trivial executor, add response to prompt for next iteration
			// (stateless, so we concatenate)
			promptText = fmt.Sprintf("%s\n\nAssistant's previous response:\n%s\n\nContinue working. Output <phase_complete>true</phase_complete> when done.",
				promptText, resp.Content)
		}
	}

	if result.Status == plan.PhaseRunning {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached", e.config.MaxIterations)
	}

done:
	result.Output = lastResponse
	result.Duration = time.Since(start)

	// Save artifact on success
	if result.Status == plan.PhaseCompleted && result.Output != "" {
		if artifactPath, err := SavePhaseArtifact(t.ID, p.ID, result.Output); err != nil {
			e.logger.Warn("failed to save phase artifact", "error", err)
		} else if artifactPath != "" {
			result.Artifacts = append(result.Artifacts, artifactPath)
			e.logger.Info("saved phase artifact", "path", artifactPath)
		}
	}

	return result, result.Error
}
