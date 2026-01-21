package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// FullExecutor executes phases using ClaudeCLI with per-iteration
// checkpointing. Best for large/greenfield tasks that need robust recovery.
//
// Execution Strategy: ClaudeCLI with --json-schema, supports resume
// Checkpointing: Every iteration (saves to disk)
// Iteration Limit: 30-50 (high, for complex tasks)
type FullExecutor struct {
	claudePath   string // Path to claude binary
	gitSvc       *git.Git
	publisher    *EventPublisher
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
	haikuClient  claude.Client       // Haiku client for progress validation
	orcConfig    *config.Config      // Config for validation settings
}

// FullExecutorOption configures a FullExecutor.
type FullExecutorOption func(*FullExecutor)

// WithFullGitSvc sets the git service.
func WithFullGitSvc(svc *git.Git) FullExecutorOption {
	return func(e *FullExecutor) { e.gitSvc = svc }
}

// WithFullPublisher sets the event publisher.
func WithFullPublisher(p events.Publisher) FullExecutorOption {
	return func(e *FullExecutor) { e.publisher = NewEventPublisher(p) }
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

// WithFullHaikuClient sets the Haiku client for progress validation.
func WithFullHaikuClient(c claude.Client) FullExecutorOption {
	return func(e *FullExecutor) { e.haikuClient = c }
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

// NewFullExecutor creates a new full executor.
func NewFullExecutor(opts ...FullExecutorOption) *FullExecutor {
	e := &FullExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
		publisher:  NewEventPublisher(nil), // Initialize with nil-safe wrapper
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
func (e *FullExecutor) Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Transcript streamer for real-time DB sync (started when JSONL path is known)
	var transcriptStreamer *TranscriptStreamer

	// Transcript persistence is handled via JSONL sync from Claude Code's session files
	// (see jsonl_sync.go), not through the publisher buffer

	// Generate session ID for resumability
	// If resuming via orc resume, use the stored session ID
	sessionID := fmt.Sprintf("%s-%s", t.ID, p.ID)
	isResume := e.resumeSessionID != ""
	if isResume {
		sessionID = e.resumeSessionID
		e.logger.Info("resuming from previous session", "session_id", sessionID)
	}

	// Check for existing checkpoint to resume from (fallback if no explicit resume)
	checkpoint, err := e.loadCheckpoint(t.ID, p.ID)
	if err != nil {
		e.logger.Debug("no checkpoint found, starting fresh", "task", t.ID, "phase", p.ID)
	}

	// Resolve model settings for this phase and weight
	modelSetting := e.config.ResolveModelSetting(string(t.Weight), p.ID)
	result.Model = modelSetting.Model

	// Create ClaudeExecutor for this phase
	claudeOpts := []ClaudeExecutorOption{
		WithClaudePath(e.claudePath),
		WithClaudeWorkdir(e.workingDir),
		WithClaudeModel(modelSetting.Model),
		WithClaudeSessionID(sessionID),
		WithClaudeResume(isResume),
		WithClaudeMaxTurns(e.config.MaxIterations),
		WithClaudeLogger(e.logger),
	}
	if e.mcpConfigPath != "" {
		claudeOpts = append(claudeOpts, WithClaudeMCPConfig(e.mcpConfigPath))
	}
	claudeExec := NewClaudeExecutor(claudeOpts...)

	// Update state with session info
	if s != nil {
		s.SetSession(sessionID, modelSetting.Model, "running", 0)
		if e.stateUpdater != nil {
			e.stateUpdater(s)
		}
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

	// Load and render initial prompt using shared template module
	tmpl, err := LoadPromptTemplate(p)
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("load prompt: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}
	vars := BuildTemplateVars(t, p, s, startIteration, LoadRetryContextForPhase(s))

	// Load spec content from database (specs are not stored as file artifacts)
	vars = vars.WithSpecFromDatabase(e.backend, t.ID)

	// Load review context for review phases (round 2+ needs prior findings)
	if p.ID == "review" {
		round := 1
		if e.config.OrcConfig != nil {
			if s != nil && s.Phases != nil {
				if ps, ok := s.Phases["review"]; ok && ps.Status == "completed" {
					round = 2
				}
			}
		}
		vars = vars.WithReviewContext(e.backend, t.ID, round)
	}

	// Add testing configuration (coverage threshold)
	if e.config.OrcConfig != nil {
		vars.CoverageThreshold = e.config.OrcConfig.Testing.CoverageThreshold
	}

	// Add worktree context for template rendering
	if e.workingDir != "" {
		vars.WorktreePath = e.workingDir
		vars.TaskBranch = t.Branch
		vars.TargetBranch = ResolveTargetBranchForTask(t, e.backend, e.config.OrcConfig)
	}

	// Add UI testing context if task requires it
	if t.RequiresUITesting {
		if e.workingDir == "" {
			e.logger.Warn("workingDir not set for UI testing - skipping UI testing context",
				"task", t.ID, "phase", p.ID)
		} else {
			// Set up screenshot directory in task test-results
			screenshotDir := task.ScreenshotsPath(e.workingDir, t.ID)
			if err := os.MkdirAll(screenshotDir, 0755); err != nil {
				e.logger.Warn("failed to create screenshot directory", "error", err)
			}

			vars = vars.WithUITestingContext(UITestingContext{
				RequiresUITesting: true,
				ScreenshotDir:     screenshotDir,
				TestResults:       loadPriorContent(task.TaskDir(t.ID), s, "test"),
			})

			e.logger.Info("UI testing enabled (full)",
				"task", t.ID,
				"phase", p.ID,
				"screenshot_dir", screenshotDir,
			)
		}
	}

	// Add initiative context if task belongs to an initiative
	if initCtx := LoadInitiativeContext(t, e.backend); initCtx != nil {
		vars = vars.WithInitiativeContext(*initCtx)
		e.logger.Info("initiative context injected (full)",
			"task", t.ID,
			"initiative", initCtx.ID,
			"has_vision", initCtx.Vision != "",
			"decision_count", len(initCtx.Decisions),
		)
	}

	// Add automation context if this is an automation task (AUTO-XXX)
	if t.IsAutomation {
		if e.workingDir == "" {
			e.logger.Warn("workingDir not set for automation context - skipping automation context",
				"task", t.ID, "phase", p.ID)
		} else if autoCtx := LoadAutomationContext(t, e.backend, e.workingDir); autoCtx != nil {
			vars = vars.WithAutomationContext(*autoCtx)
			e.logger.Info("automation context injected (full)",
				"task", t.ID,
				"has_recent_tasks", autoCtx.RecentCompletedTasks != "",
				"has_changed_files", autoCtx.RecentChangedFiles != "",
			)
		}
	}

	// Use continuation prompt when resuming (Claude already has full context)
	// Otherwise render the full template
	var promptText string
	if isResume {
		promptText = BuildContinuationPrompt(s, p.ID)
		e.logger.Info("using continuation prompt for resume", "task", t.ID, "phase", p.ID)
	} else {
		promptText = RenderTemplate(tmpl, vars)
	}

	// Load spec content for progress validation (if enabled)
	var specContent string
	if e.haikuClient != nil && e.orcConfig != nil && e.backend != nil {
		if content, err := e.backend.LoadSpec(t.ID); err == nil {
			specContent = content
		}
	}

	// Inject "ultrathink" for extended thinking mode (skip for resume - Claude preserves thinking mode)
	// This triggers maximum thinking budget (31,999 tokens) in Claude Code
	if modelSetting.Thinking && !isResume {
		promptText = "ultrathink\n\n" + promptText
		e.logger.Debug("extended thinking enabled", "task", t.ID, "phase", p.ID)
	}

	// Iteration loop with checkpointing
	var lastResponse string
	for iteration := startIteration + 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.PhaseStart(t.ID, p.ID)
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Publish activity state
		e.publisher.Activity(t.ID, p.ID, string(ActivityWaitingAPI))

		// Execute turn using ClaudeCLI with JSON schema
		turnResult, err := claudeExec.ExecuteTurn(ctx, promptText)

		// Update session ID from response for subsequent calls
		if turnResult != nil && turnResult.SessionID != "" {
			claudeExec.UpdateSessionID(turnResult.SessionID)
			// Compute and store JSONL path for transcript sync and --follow mode
			if s != nil && s.JSONLPath == "" {
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

			result.Status = plan.PhaseFailed
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

		// Progress validation: check if iteration is on track (if enabled)
		if e.haikuClient != nil && e.orcConfig != nil && specContent != "" &&
			e.orcConfig.ShouldValidateProgress(string(t.Weight)) {
			decision, reason, valErr := ValidateIterationProgress(ctx, e.haikuClient, specContent, turnResult.Content)
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
					if e.stateUpdater != nil {
						e.stateUpdater(s)
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

			// Criteria validation: check if success criteria from spec are met
			// This runs after backpressure (tests pass) but before accepting completion
			if e.haikuClient != nil && e.orcConfig != nil && specContent != "" && p.ID == "implement" &&
				e.orcConfig.ShouldValidateCriteria(string(t.Weight)) {
				criteriaResult, valErr := ValidateSuccessCriteria(ctx, e.haikuClient, specContent, turnResult.Content)
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

			// Save artifact on success (spec is saved centrally in task_execution.go with fail-fast logic)
			if artifactPath, err := SavePhaseArtifact(t.ID, p.ID, result.Output); err != nil {
				e.logger.Warn("failed to save phase artifact", "error", err)
			} else if artifactPath != "" {
				result.Artifacts = append(result.Artifacts, artifactPath)
				e.logger.Info("saved phase artifact", "path", artifactPath)
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

			result.Status = plan.PhaseFailed
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
