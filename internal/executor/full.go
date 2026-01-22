package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events" // events.Publisher for option func
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// FullExecutor executes phases using ClaudeCLI with per-iteration
// checkpointing. Best for large tasks that need robust recovery.
//
// Execution Strategy: ClaudeCLI with --json-schema, supports resume
// Checkpointing: Every iteration (saves to disk)
// Iteration Limit: 30-50 (high, for complex tasks)
type FullExecutor struct {
	claudePath   string // Path to claude binary
	gitSvc       *git.Git
	publisher    *PublishHelper
	logger       *slog.Logger
	config       ExecutorConfig
	workingDir   string
	taskDir      string             // Directory for task-specific files
	stateUpdater func(*state.State) // Callback to persist state changes
	backend      storage.Backend    // Storage backend for loading initiatives

	// MCP config path (generated for worktree)
	mcpConfigPath string

	// Resume support: if set, use Claude's --resume flag instead of starting fresh
	resumeSessionID string

	// Validation components (optional)
	backpressure *BackpressureRunner // Deterministic quality checks
	orcConfig    *config.Config      // Config for validation settings

	// turnExecutor allows injection of a mock for testing
	turnExecutor TurnExecutor
}

// FullExecutorOption configures a FullExecutor.
type FullExecutorOption func(*FullExecutor)

// WithFullGitSvc sets the git service.
func WithFullGitSvc(svc *git.Git) FullExecutorOption {
	return func(e *FullExecutor) { e.gitSvc = svc }
}

// WithFullPublisher sets the event publisher.
func WithFullPublisher(p events.Publisher) FullExecutorOption {
	return func(e *FullExecutor) { e.publisher = NewPublishHelper(p) }
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

// WithFullBackend sets the storage backend for loading initiatives.
func WithFullBackend(b storage.Backend) FullExecutorOption {
	return func(e *FullExecutor) { e.backend = b }
}

// WithFullBackpressure sets the backpressure runner for quality checks.
func WithFullBackpressure(bp *BackpressureRunner) FullExecutorOption {
	return func(e *FullExecutor) { e.backpressure = bp }
}

// WithFullOrcConfig sets the orc config for validation settings.
func WithFullOrcConfig(cfg *config.Config) FullExecutorOption {
	return func(e *FullExecutor) { e.orcConfig = cfg }
}

// WithFullResumeSessionID sets the session ID to resume from.
// When set, uses Claude's --resume flag instead of starting a fresh session.
func WithFullResumeSessionID(id string) FullExecutorOption {
	return func(e *FullExecutor) { e.resumeSessionID = id }
}

// WithFullClaudePath sets the path to the claude binary.
func WithFullClaudePath(path string) FullExecutorOption {
	return func(e *FullExecutor) { e.claudePath = path }
}

// WithFullMCPConfig sets the MCP config path.
func WithFullMCPConfig(path string) FullExecutorOption {
	return func(e *FullExecutor) { e.mcpConfigPath = path }
}

// WithFullTurnExecutor sets a mock TurnExecutor for testing.
func WithFullTurnExecutor(te TurnExecutor) FullExecutorOption {
	return func(e *FullExecutor) { e.turnExecutor = te }
}

// NewFullExecutor creates a new full executor.
func NewFullExecutor(opts ...FullExecutorOption) *FullExecutor {
	e := &FullExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
		publisher:  NewPublishHelper(nil), // Initialize with nil-safe wrapper
		config: ExecutorConfig{
			MaxIterations:      30,
			CheckpointInterval: 1, // Checkpoint every iteration
			SessionPersistence: true,
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}

	return e
}

// Name returns the executor type name.
func (e *FullExecutor) Name() string {
	return "full"
}

// Execute runs a phase with persistent session and per-iteration checkpointing.
func (e *FullExecutor) Execute(ctx context.Context, t *task.Task, p *Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: PhaseRunning,
	}

	// Transcript streamer for real-time DB sync (started when JSONL path is known)
	var transcriptStreamer *TranscriptStreamer

	// Check for existing checkpoint to resume from (fallback if no explicit resume)
	checkpoint, err := e.loadCheckpoint(t.ID, p.ID)
	if err != nil {
		e.logger.Debug("no checkpoint found, starting fresh", "task", t.ID, "phase", p.ID)
	}

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

	// Build execution context using centralized builder
	execCtx, err := BuildExecutionContext(ExecutionContextConfig{
		Task:            t,
		Phase:           p,
		State:           s,
		Backend:         e.backend,
		WorkingDir:      e.workingDir,
		MCPConfigPath:   e.mcpConfigPath,
		TaskDir:         e.taskDir,
		ExecutorConfig:  e.config,
		OrcConfig:       e.orcConfig,
		ResumeSessionID: e.resumeSessionID,
		Logger:          e.logger,
	})
	if err != nil {
		result.Status = PhaseFailed
		result.Error = fmt.Errorf("build execution context: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	result.Model = execCtx.ModelSetting.Model
	promptText := execCtx.PromptText

	// Use injected TurnExecutor if available (for testing), otherwise create ClaudeExecutor
	var turnExec TurnExecutor
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		turnExec = NewClaudeExecutorFromContext(execCtx, e.claudePath, e.config.MaxIterations, e.logger)
	}

	// Update state with session info
	if s != nil {
		s.SetSession(execCtx.SessionID, execCtx.ModelSetting.Model, "running", 0)
		if e.stateUpdater != nil {
			e.stateUpdater(s)
		}
	}

	// Iteration loop with checkpointing
	var lastResponse string
	for iteration := startIteration + 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.PhaseStart(t.ID, p.ID)
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Publish activity state
		e.publisher.Activity(t.ID, p.ID, string(ActivityWaitingAPI))

		// Execute turn using TurnExecutor with JSON schema
		turnResult, err := turnExec.ExecuteTurn(ctx, promptText)

		// Update session ID from response for subsequent calls
		if turnResult != nil && turnResult.SessionID != "" {
			turnExec.UpdateSessionID(turnResult.SessionID)
			// Store session ID per-phase for correct resume behavior
			if s != nil {
				s.SetPhaseSessionID(p.ID, turnResult.SessionID)
			}
			// Compute and store JSONL path for transcript sync and --follow mode
			// Update for every phase since each phase has its own JSONL file
			if s != nil {
				if jsonlPath, pathErr := ComputeJSONLPath(e.workingDir, turnResult.SessionID); pathErr == nil {
					s.JSONLPath = jsonlPath
					// Persist immediately so --follow can find it
					if e.stateUpdater != nil {
						e.stateUpdater(s)
					}
					// Start real-time transcript streaming to DB
					if transcriptStreamer == nil && e.backend != nil {
						syncer := NewJSONLSyncer(e.backend, e.logger)
						streamer, streamErr := syncer.StartStreaming(jsonlPath, SyncOptions{
							TaskID: t.ID,
							Phase:  p.ID,
							Append: true,
						})
						if streamErr != nil {
							e.logger.Warn("failed to start transcript streaming", "error", streamErr)
						} else {
							transcriptStreamer = streamer
							e.logger.Debug("started real-time transcript streaming", "path", jsonlPath)
						}
					}
				}
			}
		}

		if err != nil {
			// Save checkpoint before failing (with partial content if available)
			checkpointContent := lastResponse
			if turnResult != nil && turnResult.Content != "" {
				checkpointContent = turnResult.Content
				e.logger.Info("partial response received before error",
					"task", t.ID,
					"phase", p.ID,
					"content_len", len(turnResult.Content),
				)
			}

			if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
				Iteration:    iteration - 1,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
				LastResponse: checkpointContent,
			}); cpErr != nil {
				e.logger.Error("failed to save checkpoint", "error", cpErr)
			}

			result.Status = PhaseFailed
			result.Error = fmt.Errorf("execute turn %d: %w", iteration, err)
			result.Output = lastResponse // Preserve any previous response for debugging
			goto done
		}

		// Track tokens and cost - use effective input to include cached context
		effectiveInput := turnResult.Usage.EffectiveInputTokens()
		result.InputTokens += effectiveInput
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.CacheCreationTokens += turnResult.Usage.CacheCreationInputTokens
		result.CacheReadTokens += turnResult.Usage.CacheReadInputTokens
		result.CostUSD += turnResult.CostUSD
		result.Iterations = iteration
		lastResponse = turnResult.Content

		// Update session turn count for resume tracking
		if s != nil && s.Session != nil {
			s.Session.TurnCount = iteration
			s.Session.LastActivity = time.Now()
		}

		e.publisher.Transcript(t.ID, p.ID, iteration, "response", turnResult.Content)

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
		// Note: Cost is NOT added here - it's accumulated in result.CostUSD and transferred
		// to state after phase completion in task_execution.go for consistency across all executors.
		if s != nil && e.stateUpdater != nil {
			s.IncrementIteration()
			s.AddTokens(effectiveInput, turnResult.Usage.OutputTokens,
				turnResult.Usage.CacheCreationInputTokens, turnResult.Usage.CacheReadInputTokens)
			e.stateUpdater(s)
		}

		// Check for completion
		switch turnResult.Status {
		case PhaseStatusComplete:
			// Run backpressure checks before accepting completion (implement phase only)
			if e.backpressure != nil && !ShouldSkipBackpressure(p.ID) {
				bpResult := e.backpressure.Run(ctx)
				if !bpResult.AllPassed {
					// Reject completion, inject failure context
					e.logger.Info("backpressure failed, continuing iteration",
						"task", t.ID,
						"phase", p.ID,
						"tests", bpResult.TestsPassed,
						"lint", bpResult.LintPassed,
						"summary", bpResult.FailureSummary(),
					)
					e.publisher.Warning(t.ID, p.ID, "Backpressure check failed: "+bpResult.FailureSummary())

					// Inject failure context into next prompt
					promptText = FormatBackpressureForPrompt(bpResult)
					continue // Don't accept completion, iterate again
				}
				e.logger.Info("backpressure passed",
					"task", t.ID,
					"phase", p.ID,
					"duration", bpResult.Duration,
				)
			}

			result.Status = PhaseCompleted
			result.Output = turnResult.Content

			// Save artifact on success (spec is saved centrally in task_execution.go with fail-fast logic)
			if saved, err := SaveArtifactToDatabase(e.backend, t.ID, p.ID, result.Output); err != nil {
				e.logger.Warn("failed to save phase artifact to database", "error", err)
			} else if saved {
				e.logger.Info("saved phase artifact to database", "phase", p.ID)
			}

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

			result.Status = PhaseFailed
			result.Output = lastResponse // Preserve output for retry context
			result.Error = fmt.Errorf("phase blocked: %s", turnResult.Reason)
			e.logger.Warn("phase blocked (full)", "task", t.ID, "phase", p.ID, "reason", turnResult.Reason)
			goto done

		case PhaseStatusContinue:
			// Session maintains context, just send continuation prompt
			promptText = "Continue working on the task."
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

			result.Status = PhaseFailed
			result.Error = fmt.Errorf("LLM error: %s", turnResult.ErrorText)
			result.Output = lastResponse
			goto done
		}
	}

	if result.Status == PhaseRunning {
		// Save checkpoint for max iterations case
		if cpErr := e.saveCheckpoint(t.ID, p.ID, &iterationCheckpoint{
			Iteration:    e.config.MaxIterations,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			LastResponse: lastResponse,
		}); cpErr != nil {
			e.logger.Error("failed to save checkpoint on max iterations", "error", cpErr)
		}

		result.Status = PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached", e.config.MaxIterations)
		result.Output = lastResponse // Preserve last response for debugging
	}

done:
	result.Duration = time.Since(start)

	// Stop transcript streaming (flushes any remaining messages to DB)
	if transcriptStreamer != nil {
		transcriptStreamer.Stop()
		e.logger.Debug("stopped transcript streaming")
	}

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
