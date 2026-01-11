package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// FullExecutor executes phases using persistent sessions with per-iteration
// checkpointing. Best for large/greenfield tasks that need robust recovery.
//
// Session Strategy: Persistent sessions that can be resumed
// Checkpointing: Every iteration (saves to disk)
// Iteration Limit: 30-50 (high, for complex tasks)
type FullExecutor struct {
	manager      session.SessionManager
	gitSvc       *git.Git
	publisher    events.Publisher
	logger       *slog.Logger
	config       ExecutorConfig
	workingDir   string
	taskDir      string // Directory for task-specific files
	stateUpdater func(*state.State) // Callback to persist state changes
}

// FullExecutorOption configures a FullExecutor.
type FullExecutorOption func(*FullExecutor)

// WithFullGitSvc sets the git service.
func WithFullGitSvc(svc *git.Git) FullExecutorOption {
	return func(e *FullExecutor) { e.gitSvc = svc }
}

// WithFullPublisher sets the event publisher.
func WithFullPublisher(p events.Publisher) FullExecutorOption {
	return func(e *FullExecutor) { e.publisher = p }
}

// WithFullLogger sets the logger.
func WithFullLogger(l *slog.Logger) FullExecutorOption {
	return func(e *FullExecutor) { e.logger = l }
}

// WithFullConfig sets the execution config.
func WithFullConfig(cfg ExecutorConfig) FullExecutorOption {
	return func(e *FullExecutor) { e.config = cfg }
}

// WithFullWorkingDir sets the working directory.
func WithFullWorkingDir(dir string) FullExecutorOption {
	return func(e *FullExecutor) { e.workingDir = dir }
}

// WithTaskDir sets the task directory for checkpoints.
func WithTaskDir(dir string) FullExecutorOption {
	return func(e *FullExecutor) { e.taskDir = dir }
}

// WithStateUpdater sets a callback for persisting state changes.
func WithStateUpdater(fn func(*state.State)) FullExecutorOption {
	return func(e *FullExecutor) { e.stateUpdater = fn }
}

// NewFullExecutor creates a new full executor.
func NewFullExecutor(mgr session.SessionManager, opts ...FullExecutorOption) *FullExecutor {
	e := &FullExecutor{
		manager: mgr,
		logger:  slog.Default(),
		config: ExecutorConfig{
			MaxIterations:      30,
			CheckpointInterval: 1, // Checkpoint every iteration
			SessionPersistence: true,
		},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Name returns the executor type name.
func (e *FullExecutor) Name() string {
	return "full"
}

// Execute runs a phase with persistent session and per-iteration checkpointing.
func (e *FullExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Generate session ID for resumability
	sessionID := fmt.Sprintf("%s-%s", t.ID, p.ID)

	// Check for existing checkpoint to resume from
	checkpoint, err := e.loadCheckpoint(t.ID, p.ID)
	if err != nil {
		e.logger.Debug("no checkpoint found, starting fresh", "task", t.ID, "phase", p.ID)
	}

	// Create session adapter with resume capability
	adapterOpts := SessionAdapterOptions{
		SessionID:   sessionID,
		Resume:      checkpoint != nil, // Resume if we have a checkpoint
		Model:       e.config.Model,
		Workdir:     e.workingDir,
		MaxTurns:    e.config.MaxIterations,
		Persistence: e.config.SessionPersistence,
	}

	adapter, err := NewSessionAdapter(ctx, e.manager, adapterOpts)
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("create session: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}
	defer func() {
		if closeErr := adapter.Close(); closeErr != nil {
			e.logger.Error("failed to close adapter", "error", closeErr)
		}
	}()

	// Determine starting iteration (from checkpoint or 0)
	startIteration := 0
	if checkpoint != nil {
		startIteration = checkpoint.Iteration
		result.InputTokens = checkpoint.InputTokens
		result.OutputTokens = checkpoint.OutputTokens
		e.logger.Info("resuming from checkpoint",
			"task", t.ID,
			"phase", p.ID,
			"iteration", startIteration,
		)
	}

	// Load and render initial prompt
	promptText, err := e.loadAndRenderPrompt(t, p, s, startIteration, "")
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("load prompt: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Iteration loop with checkpointing
	var lastResponse string
	for iteration := startIteration + 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publishPhaseProgress(t.ID, p.ID, iteration)
		e.publishTranscript(t.ID, p.ID, iteration, "prompt", promptText)

		// Execute turn with streaming
		turnResult, err := adapter.StreamTurn(ctx, promptText, func(chunk string) {
			e.publishTranscriptChunk(t.ID, p.ID, iteration, chunk)
		})

		if err != nil {
			// Save checkpoint before failing
			if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
				Iteration:    iteration - 1,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				LastResponse: lastResponse,
			}); cpErr != nil {
				e.logger.Error("failed to save checkpoint", "error", cpErr)
			}

			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("execute turn %d: %w", iteration, err)
			result.Output = lastResponse // Preserve any previous response for debugging
			goto done
		}

		// Track tokens
		result.InputTokens += turnResult.Usage.InputTokens
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.Iterations = iteration
		lastResponse = turnResult.Content

		e.publishTranscript(t.ID, p.ID, iteration, "response", turnResult.Content)

		// Save iteration checkpoint (per-iteration checkpointing)
		if e.config.CheckpointInterval > 0 && iteration%e.config.CheckpointInterval == 0 {
			if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
				Iteration:    iteration,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				LastResponse: lastResponse,
			}); cpErr != nil {
				e.logger.Error("failed to save iteration checkpoint", "error", cpErr)
			}
		}

		// Update state with iteration progress
		if s != nil && e.stateUpdater != nil {
			s.IncrementIteration()
			s.AddTokens(turnResult.Usage.InputTokens, turnResult.Usage.OutputTokens)
			e.stateUpdater(s)
		}

		// Check for completion
		switch turnResult.Status {
		case PhaseStatusComplete:
			result.Status = plan.PhaseCompleted
			result.Output = turnResult.Content

			// Create git checkpoint on completion
			if e.gitSvc != nil {
				checkpoint, err := e.gitSvc.CreateCheckpoint(t.ID, p.ID, "completed")
				if err != nil {
					e.logger.Warn("failed to create git checkpoint", "error", err)
				} else if checkpoint != nil {
					result.CommitSHA = checkpoint.CommitSHA
				}
			}

			// Remove iteration checkpoint (phase complete)
			e.removeCheckpoint(t.ID, p.ID)

			e.logger.Info("phase complete (full)", "task", t.ID, "phase", p.ID, "iterations", iteration)
			goto done

		case PhaseStatusBlocked:
			// Save checkpoint before failing
			if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
				Iteration:    iteration,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				LastResponse: lastResponse,
				Blocked:      true,
				BlockReason:  turnResult.Reason,
			}); cpErr != nil {
				e.logger.Error("failed to save checkpoint on block", "error", cpErr)
			}

			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("phase blocked: %s", turnResult.Reason)
			e.logger.Warn("phase blocked (full)", "task", t.ID, "phase", p.ID, "reason", turnResult.Reason)
			goto done

		case PhaseStatusContinue:
			// Session maintains context, just send continuation prompt
			promptText = "Continue working on the task. Remember to output <phase_complete>true</phase_complete> when you're done."
		}

		// Check for errors
		if turnResult.IsError {
			if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
				Iteration:    iteration,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				LastResponse: lastResponse,
				Error:        turnResult.ErrorText,
			}); cpErr != nil {
				e.logger.Error("failed to save checkpoint on error", "error", cpErr)
			}

			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("LLM error: %s", turnResult.ErrorText)
			result.Output = lastResponse
			goto done
		}
	}

	if result.Status == plan.PhaseRunning {
		// Save checkpoint for max iterations case
		if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
			Iteration:    e.config.MaxIterations,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			LastResponse: lastResponse,
		}); cpErr != nil {
			e.logger.Error("failed to save checkpoint on max iterations", "error", cpErr)
		}

		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached", e.config.MaxIterations)
		result.Output = lastResponse // Preserve last response for debugging
	}

done:
	result.Duration = time.Since(start)
	return result, result.Error
}

// iterationCheckpoint holds state for resuming mid-phase.
type iterationCheckpoint struct {
	Iteration    int    `json:"iteration"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	LastResponse string `json:"last_response"`
	Blocked      bool   `json:"blocked,omitempty"`
	BlockReason  string `json:"block_reason,omitempty"`
	Error        string `json:"error,omitempty"`
}

// checkpointPath returns the path for a phase checkpoint file.
func (e *FullExecutor) checkpointPath(taskID, phaseID string) string {
	if e.taskDir != "" {
		return filepath.Join(e.taskDir, fmt.Sprintf("checkpoint-%s.json", phaseID))
	}
	return filepath.Join(".orc", "tasks", taskID, fmt.Sprintf("checkpoint-%s.json", phaseID))
}

// loadCheckpoint loads an existing checkpoint for resume.
func (e *FullExecutor) loadCheckpoint(taskID, phaseID string) (*iterationCheckpoint, error) {
	path := e.checkpointPath(taskID, phaseID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cp iterationCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}

	return &cp, nil
}

// saveCheckpoint saves an iteration checkpoint.
func (e *FullExecutor) saveCheckpoint(taskID, phaseID string, cp *iterationCheckpoint) error {
	path := e.checkpointPath(taskID, phaseID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// removeCheckpoint removes a checkpoint file after successful completion.
func (e *FullExecutor) removeCheckpoint(taskID, phaseID string) {
	path := e.checkpointPath(taskID, phaseID)
	_ = os.Remove(path) // Intentionally ignore - file may not exist
}

// loadAndRenderPrompt loads and renders the prompt template.
func (e *FullExecutor) loadAndRenderPrompt(t *task.Task, p *plan.Phase, s *state.State, iteration int, retryContext string) (string, error) {
	if p.Prompt != "" {
		return e.renderTemplate(p.Prompt, t, p, iteration, retryContext), nil
	}

	tmplPath := fmt.Sprintf("prompts/%s.md", p.ID)
	content, err := templates.Prompts.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("prompt not found for phase %s", p.ID)
	}

	return e.renderTemplate(string(content), t, p, iteration, retryContext), nil
}

// renderTemplate does simple variable substitution.
func (e *FullExecutor) renderTemplate(tmpl string, t *task.Task, p *plan.Phase, iteration int, retryContext string) string {
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

// Event publishing

func (e *FullExecutor) publish(ev events.Event) {
	if e.publisher != nil {
		e.publisher.Publish(ev)
	}
}

func (e *FullExecutor) publishPhaseProgress(taskID, phase string, iteration int) {
	e.publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: string(plan.PhaseRunning),
	}))
}

func (e *FullExecutor) publishTranscript(taskID, phase string, iteration int, msgType, content string) {
	e.publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}))
}

func (e *FullExecutor) publishTranscriptChunk(taskID, phase string, iteration int, chunk string) {
	e.publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      "chunk",
		Content:   chunk,
		Timestamp: time.Now(),
	}))
}
