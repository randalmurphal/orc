package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events" // events.Publisher for option func
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// StandardExecutor executes phases using ClaudeCLI with headless mode (-p)
// and JSON schema for structured completion output. This is the recommended
// executor for small to medium tasks.
//
// Execution Strategy: ClaudeCLI per phase with --json-schema for completion detection
// Checkpointing: Only on phase completion
// Iteration Limit: Configurable, defaults based on weight
type StandardExecutor struct {
	claudePath string // Path to claude binary
	gitSvc     *git.Git
	publisher  *EventPublisher
	logger     *slog.Logger
	config     ExecutorConfig
	workingDir string
	backend    storage.Backend // Storage backend for loading initiatives

	// MCP config path (generated for worktree)
	mcpConfigPath string

	// Resume support: if set, use Claude's --resume flag instead of starting fresh
	resumeSessionID string

	// Validation components (optional)
	backpressure    *BackpressureRunner // Deterministic quality checks
	validationModel string              // Model for validation (empty = disabled)
	orcConfig       *config.Config      // Config for validation settings

	// turnExecutor allows injection of a mock for testing.
	// If nil, a real ClaudeExecutor is created during Execute.
	turnExecutor TurnExecutor
}

// StandardExecutorOption configures a StandardExecutor.
type StandardExecutorOption func(*StandardExecutor)

// WithGitSvc sets the git service for commits.
func WithGitSvc(svc *git.Git) StandardExecutorOption {
	return func(e *StandardExecutor) { e.gitSvc = svc }
}

// WithPublisher sets the event publisher for real-time updates.
func WithPublisher(p events.Publisher) StandardExecutorOption {
	return func(e *StandardExecutor) { e.publisher = NewEventPublisher(p) }
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

// WithStandardBackend sets the storage backend for loading initiatives.
func WithStandardBackend(b storage.Backend) StandardExecutorOption {
	return func(e *StandardExecutor) { e.backend = b }
}

// WithStandardBackpressure sets the backpressure runner for quality checks.
func WithStandardBackpressure(bp *BackpressureRunner) StandardExecutorOption {
	return func(e *StandardExecutor) { e.backpressure = bp }
}

// WithStandardValidationModel sets the model for progress validation.
// The validation client is created dynamically with the correct workdir.
func WithStandardValidationModel(model string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.validationModel = model }
}

// WithStandardOrcConfig sets the orc config for validation settings.
func WithStandardOrcConfig(cfg *config.Config) StandardExecutorOption {
	return func(e *StandardExecutor) { e.orcConfig = cfg }
}

// WithStandardTurnExecutor sets a custom TurnExecutor (for testing).
// When set, the executor uses this instead of creating a real ClaudeExecutor.
func WithStandardTurnExecutor(te TurnExecutor) StandardExecutorOption {
	return func(e *StandardExecutor) { e.turnExecutor = te }
}

// WithStandardResumeSessionID sets the session ID to resume from.
// When set, uses Claude's --resume flag instead of starting a fresh session.
func WithStandardResumeSessionID(id string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.resumeSessionID = id }
}

// WithStandardClaudePath sets the path to the claude binary.
func WithStandardClaudePath(path string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.claudePath = path }
}

// WithStandardMCPConfig sets the MCP config path.
func WithStandardMCPConfig(path string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.mcpConfigPath = path }
}

// NewStandardExecutor creates a new standard executor.
func NewStandardExecutor(opts ...StandardExecutorOption) *StandardExecutor {
	e := &StandardExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
		publisher:  NewEventPublisher(nil), // Initialize with nil-safe wrapper
		config: ExecutorConfig{
			MaxIterations:      20,
			CheckpointInterval: 0,
			SessionPersistence: false,
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
func (e *StandardExecutor) Name() string {
	return "standard"
}

// createValidationClient creates a validation client for the current working directory.
// Returns nil if validation is disabled.
func (e *StandardExecutor) createValidationClient() claude.Client {
	if e.validationModel == "" {
		return nil
	}
	opts := []claude.ClaudeOption{
		claude.WithModel(e.validationModel),
		claude.WithWorkdir(e.workingDir),
	}
	if e.claudePath != "" {
		opts = append(opts, claude.WithClaudePath(e.claudePath))
	}
	return claude.NewClaudeCLI(opts...)
}

// Execute runs a phase to completion using ClaudeCLI with JSON schema.
func (e *StandardExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Transcript streamer for real-time DB sync (started when JSONL path is known)
	var transcriptStreamer *TranscriptStreamer

	// Build execution context using centralized builder
	execCtx, err := BuildExecutionContext(ExecutionContextConfig{
		Task:            t,
		Phase:           p,
		State:           s,
		Backend:         e.backend,
		WorkingDir:      e.workingDir,
		MCPConfigPath:   e.mcpConfigPath,
		ExecutorConfig:  e.config,
		OrcConfig:       e.orcConfig,
		ResumeSessionID: e.resumeSessionID,
		Logger:          e.logger,
	})
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("build execution context: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	result.Model = execCtx.ModelSetting.Model
	promptText := execCtx.PromptText
	specContent := execCtx.SpecContent

	// Use injected TurnExecutor if available (for testing), otherwise create real ClaudeExecutor
	var turnExec TurnExecutor
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		turnExec = NewClaudeExecutorFromContext(execCtx, e.claudePath, e.config.MaxIterations, e.logger)
	}

	// Update state with session info
	if s != nil {
		s.SetSession(execCtx.SessionID, execCtx.ModelSetting.Model, "running", 0)
		if e.backend != nil {
			if err := e.backend.SaveState(s); err != nil {
				e.logger.Warn("failed to persist session info to state", "error", err)
			}
		}
	}

	// Iteration loop
	var lastResponse string
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.PhaseStart(t.ID, p.ID)

		// Publish prompt transcript
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Publish activity state
		e.publisher.Activity(t.ID, p.ID, string(ActivityWaitingAPI))

		// Execute turn using ClaudeCLI with JSON schema
		turnResult, err := turnExec.ExecuteTurn(ctx, promptText)

		// Update session ID from response for subsequent calls
		if turnResult != nil && turnResult.SessionID != "" {
			turnExec.UpdateSessionID(turnResult.SessionID)
			// Compute and store JSONL path for transcript sync and --follow mode
			if s != nil && s.JSONLPath == "" {
				if jsonlPath, pathErr := ComputeJSONLPath(e.workingDir, turnResult.SessionID); pathErr == nil {
					s.JSONLPath = jsonlPath
					// Persist immediately so --follow can find it
					if e.backend != nil {
						if saveErr := e.backend.SaveState(s); saveErr != nil {
							e.logger.Warn("failed to persist JSONL path", "error", saveErr)
						}
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
			// Check if this is a recoverable timeout
			if turnResult != nil && turnResult.Content != "" {
				e.logger.Info("partial response received before timeout",
					"task", t.ID,
					"phase", p.ID,
					"content_len", len(turnResult.Content),
				)
				// For timeout errors, we can try to continue if we got partial content
				lastResponse = turnResult.Content
			}
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("execute turn %d: %w", iteration, err)
			result.Output = lastResponse // Preserve any previous response for debugging
			goto done
		}

		// Track tokens - use effective input to include cached context
		result.InputTokens += turnResult.Usage.EffectiveInputTokens()
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

		// Publish response transcript
		e.publisher.Transcript(t.ID, p.ID, iteration, "response", turnResult.Content)

		// Progress validation: check if iteration is on track (if enabled)
		if e.validationModel != "" && e.orcConfig != nil && specContent != "" &&
			e.orcConfig.ShouldValidateProgress(string(t.Weight)) {
			valClient := e.createValidationClient()
			decision, reason, valErr := ValidateIterationProgress(ctx, valClient, specContent, turnResult.Content)
			if valErr != nil {
				if e.orcConfig.Validation.FailOnAPIError {
					// Fail properly - task is resumable from this phase
					e.logger.Error("progress validation API error - failing task",
						"task", t.ID,
						"phase", p.ID,
						"error", valErr,
						"hint", "Task can be resumed with 'orc resume'",
					)
					result.Status = plan.PhaseFailed
					result.Error = fmt.Errorf("progress validation API error (resumable): %w", valErr)
					result.Output = lastResponse
					result.Duration = time.Since(start)
					if transcriptStreamer != nil {
						transcriptStreamer.Stop()
					}
					return result, result.Error
				}
				// Fail open (legacy behavior for fast profile)
				e.logger.Warn("progress validation error (continuing)",
					"task", t.ID,
					"phase", p.ID,
					"error", valErr,
				)
			} else {
				// Record validation result to state for resume/retry tracking
				if s != nil {
					s.RecordValidation(p.ID, state.ValidationEntry{
						Iteration: iteration,
						Type:      "progress",
						Decision:  decision.String(),
						Reason:    reason,
						Timestamp: time.Now(),
					})
					// Persist immediately so validation survives crashes/interrupts
					if err := e.backend.SaveState(s); err != nil {
						e.logger.Error("failed to persist validation result", "error", err)
					}
				}

				switch decision {
				case ValidationRetry:
					e.logger.Info("progress validation: redirect needed",
						"task", t.ID,
						"phase", p.ID,
						"reason", reason,
					)
					e.publisher.Warning(t.ID, p.ID, "Progress validation: "+reason)
					// Inject redirect prompt for next iteration
					promptText = fmt.Sprintf("## Progress Validation Feedback\n\n"+
						"External review indicates your approach may be off track:\n%s\n\n"+
						"Please review the specification and adjust your approach. "+
						"Continue working on the task.", reason)
					continue // Skip completion check, iterate with feedback
				case ValidationStop:
					e.logger.Warn("progress validation: blocked",
						"task", t.ID,
						"phase", p.ID,
						"reason", reason,
					)
					result.Status = plan.PhaseFailed
					result.Output = turnResult.Content
					result.Error = fmt.Errorf("progress validation blocked: %s", reason)
					goto done
				}
				// ValidationContinue - proceed normally
			}
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

			// Criteria validation: check if success criteria from spec are met
			// This runs after backpressure (tests pass) but before accepting completion
			if e.validationModel != "" && e.orcConfig != nil && specContent != "" && p.ID == "implement" &&
				e.orcConfig.ShouldValidateCriteria(string(t.Weight)) {
				valClient := e.createValidationClient()
				criteriaResult, valErr := ValidateSuccessCriteria(ctx, valClient, specContent, turnResult.Content)
				if valErr != nil {
					if e.orcConfig.Validation.FailOnAPIError {
						// Fail properly - task is resumable from this phase
						e.logger.Error("criteria validation API error - failing task",
							"task", t.ID,
							"phase", p.ID,
							"error", valErr,
							"hint", "Task can be resumed with 'orc resume'",
						)
						result.Status = plan.PhaseFailed
						result.Error = fmt.Errorf("criteria validation API error (resumable): %w", valErr)
						result.Output = turnResult.Content
						result.Duration = time.Since(start)
						if transcriptStreamer != nil {
							transcriptStreamer.Stop()
						}
						return result, result.Error
					}
					// Fail open (legacy behavior for fast profile)
					e.logger.Warn("criteria validation error (continuing)",
						"task", t.ID,
						"phase", p.ID,
						"error", valErr,
					)
				} else if !criteriaResult.AllMet {
					// Not all criteria met - inject feedback and continue iteration
					e.logger.Info("criteria validation failed, continuing iteration",
						"task", t.ID,
						"phase", p.ID,
						"missing", criteriaResult.MissingSummary,
					)
					e.publisher.Warning(t.ID, p.ID, "Criteria check: "+criteriaResult.MissingSummary)

					// Inject criteria feedback into next prompt
					promptText = FormatCriteriaFeedback(criteriaResult)
					continue // Don't accept completion, iterate again
				}
				e.logger.Info("criteria validation passed",
					"task", t.ID,
					"phase", p.ID,
				)
			}

			result.Status = plan.PhaseCompleted
			result.Output = turnResult.Content
			e.logger.Info("phase complete", "task", t.ID, "phase", p.ID, "iterations", iteration)
			goto done

		case PhaseStatusBlocked:
			result.Status = plan.PhaseFailed
			result.Output = turnResult.Content // Preserve output for retry context
			result.Error = fmt.Errorf("phase blocked: %s", turnResult.Reason)
			e.logger.Warn("phase blocked", "task", t.ID, "phase", p.ID, "reason", turnResult.Reason)
			goto done

		case PhaseStatusContinue:
			// Continue with next iteration using continuation prompt
			promptText = "Continue working on the task."
		}

		// Check for errors
		if turnResult.IsError {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("LLM error: %s", turnResult.ErrorText)
			result.Output = lastResponse
			goto done
		}
	}

	// If we exhausted iterations without completion
	if result.Status == plan.PhaseRunning {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached without completion", e.config.MaxIterations)
		result.Output = lastResponse // Preserve last response for debugging
	}

done:
	result.Duration = time.Since(start)

	// Stop transcript streaming (flushes any remaining messages to DB)
	if transcriptStreamer != nil {
		transcriptStreamer.Stop()
		e.logger.Debug("stopped transcript streaming")
	}

	// Save artifact on success (spec is saved centrally in task_execution.go with fail-fast logic)
	if result.Status == plan.PhaseCompleted && result.Output != "" {
		if artifactPath, err := SavePhaseArtifact(t.ID, p.ID, result.Output); err != nil {
			e.logger.Warn("failed to save phase artifact", "error", err)
		} else if artifactPath != "" {
			result.Artifacts = append(result.Artifacts, artifactPath)
			e.logger.Info("saved phase artifact", "path", artifactPath)
		}
	}

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
