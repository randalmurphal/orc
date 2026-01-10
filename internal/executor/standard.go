package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
)

// StandardExecutor executes phases using session-based LLM interaction
// with completion marker detection. This is the recommended executor
// for small to medium tasks.
//
// Session Strategy: Creates a session per phase, maintains context within phase
// Checkpointing: Only on phase completion
// Iteration Limit: Configurable, defaults based on weight
type StandardExecutor struct {
	manager    session.SessionManager
	gitSvc     *git.Git
	publisher  events.Publisher
	logger     *slog.Logger
	config     ExecutorConfig
	workingDir string
}

// StandardExecutorOption configures a StandardExecutor.
type StandardExecutorOption func(*StandardExecutor)

// WithGitSvc sets the git service for commits.
func WithGitSvc(svc *git.Git) StandardExecutorOption {
	return func(e *StandardExecutor) { e.gitSvc = svc }
}

// WithPublisher sets the event publisher for real-time updates.
func WithPublisher(p events.Publisher) StandardExecutorOption {
	return func(e *StandardExecutor) { e.publisher = p }
}

// WithExecutorLogger sets the logger.
func WithExecutorLogger(l *slog.Logger) StandardExecutorOption {
	return func(e *StandardExecutor) { e.logger = l }
}

// WithExecutorConfig sets the execution configuration.
func WithExecutorConfig(cfg ExecutorConfig) StandardExecutorOption {
	return func(e *StandardExecutor) { e.config = cfg }
}

// WithWorkingDir sets the working directory for the session.
func WithWorkingDir(dir string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.workingDir = dir }
}

// NewStandardExecutor creates a new standard executor.
func NewStandardExecutor(mgr session.SessionManager, opts ...StandardExecutorOption) *StandardExecutor {
	e := &StandardExecutor{
		manager: mgr,
		logger:  slog.Default(),
		config: ExecutorConfig{
			MaxIterations:      20,
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
func (e *StandardExecutor) Name() string {
	return "standard"
}

// Execute runs a phase to completion using session-based execution.
func (e *StandardExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Generate session ID: {task_id}-{phase_id}
	sessionID := fmt.Sprintf("%s-%s", t.ID, p.ID)

	// Create session adapter
	adapter, err := NewSessionAdapter(ctx, e.manager, SessionAdapterOptions{
		SessionID:   sessionID,
		Model:       e.config.Model,
		Workdir:     e.workingDir,
		MaxTurns:    e.config.MaxIterations,
		Persistence: e.config.SessionPersistence,
	})
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("create session: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}
	defer adapter.Close()

	// Load and render initial prompt
	promptText, err := e.loadAndRenderPrompt(t, p, s, 0, "")
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("load prompt: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Iteration loop
	var lastResponse string
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publishPhaseProgress(t.ID, p.ID, iteration)

		// Publish prompt transcript
		e.publishTranscript(t.ID, p.ID, iteration, "prompt", promptText)

		// Execute turn with streaming
		turnResult, err := adapter.StreamTurn(ctx, promptText, func(chunk string) {
			// Publish chunk for real-time display
			e.publishTranscriptChunk(t.ID, p.ID, iteration, chunk)
		})

		if err != nil {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("execute turn %d: %w", iteration, err)
			break
		}

		// Track tokens
		result.InputTokens += turnResult.Usage.InputTokens
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.Iterations = iteration
		lastResponse = turnResult.Content

		// Publish response transcript
		e.publishTranscript(t.ID, p.ID, iteration, "response", turnResult.Content)

		// Check for completion
		switch turnResult.Status {
		case PhaseStatusComplete:
			result.Status = plan.PhaseCompleted
			result.Output = turnResult.Content
			e.logger.Info("phase complete", "task", t.ID, "phase", p.ID, "iterations", iteration)
			goto done

		case PhaseStatusBlocked:
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("phase blocked: %s", turnResult.Reason)
			e.logger.Warn("phase blocked", "task", t.ID, "phase", p.ID, "reason", turnResult.Reason)
			goto done

		case PhaseStatusContinue:
			// Continue with next iteration
			// For session-based execution, we don't need to re-render the full prompt
			// The session maintains context. Just send a continuation prompt.
			promptText = "Continue working on the task. Remember to output <phase_complete>true</phase_complete> when you're done."
		}

		// Check for errors
		if turnResult.IsError {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("LLM error: %s", turnResult.ErrorText)
			break
		}
	}

	// If we exhausted iterations without completion
	if result.Status == plan.PhaseRunning {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached without completion", e.config.MaxIterations)
	}

done:
	result.Output = lastResponse
	result.Duration = time.Since(start)

	// Commit on success if git service available
	if result.Status == plan.PhaseCompleted && e.gitSvc != nil {
		checkpoint, err := e.gitSvc.CreateCheckpoint(t.ID, p.ID, "completed")
		if err != nil {
			e.logger.Warn("failed to create checkpoint", "error", err)
		} else if checkpoint != nil {
			result.CommitSHA = checkpoint.CommitSHA
		}
	}

	return result, result.Error
}

// loadAndRenderPrompt loads the prompt template and renders it with variables.
func (e *StandardExecutor) loadAndRenderPrompt(t *task.Task, p *plan.Phase, s *state.State, iteration int, retryContext string) (string, error) {
	// Try inline prompt first
	if p.Prompt != "" {
		return e.renderTemplate(p.Prompt, t, p, s, iteration, retryContext), nil
	}

	// Load from embedded templates
	tmplPath := fmt.Sprintf("prompts/%s.md", p.ID)
	content, err := templates.Prompts.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("prompt not found for phase %s", p.ID)
	}

	return e.renderTemplate(string(content), t, p, s, iteration, retryContext), nil
}

// renderTemplate does simple template variable substitution.
func (e *StandardExecutor) renderTemplate(tmpl string, t *task.Task, p *plan.Phase, s *state.State, iteration int, retryContext string) string {
	replacements := map[string]string{
		"{{TASK_ID}}":          t.ID,
		"{{TASK_TITLE}}":       t.Title,
		"{{TASK_DESCRIPTION}}": t.Description,
		"{{PHASE}}":            p.ID,
		"{{WEIGHT}}":           string(t.Weight),
		"{{ITERATION}}":        fmt.Sprintf("%d", iteration),
		"{{RETRY_CONTEXT}}":    retryContext,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}

// Event publishing helpers

func (e *StandardExecutor) publish(ev events.Event) {
	if e.publisher != nil {
		e.publisher.Publish(ev)
	}
}

func (e *StandardExecutor) publishPhaseProgress(taskID, phase string, iteration int) {
	e.publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: string(plan.PhaseRunning),
	}))
}

func (e *StandardExecutor) publishTranscript(taskID, phase string, iteration int, msgType, content string) {
	e.publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}))
}

func (e *StandardExecutor) publishTranscriptChunk(taskID, phase string, iteration int, chunk string) {
	e.publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      "chunk",
		Content:   chunk,
		Timestamp: time.Now(),
	}))
}
