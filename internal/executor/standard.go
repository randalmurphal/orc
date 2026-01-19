package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events" // events.Publisher for option func
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
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
	publisher  *EventPublisher
	logger     *slog.Logger
	config     ExecutorConfig
	workingDir string
	backend    storage.Backend // Storage backend for loading initiatives

	// Resume support: if set, use Claude's --resume flag instead of starting fresh
	resumeSessionID string

	// Validation components (optional)
	backpressure *BackpressureRunner // Deterministic quality checks
	haikuClient  claude.Client       // Haiku client for progress validation
	orcConfig    *config.Config      // Config for validation settings
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

// WithStandardHaikuClient sets the Haiku client for progress validation.
func WithStandardHaikuClient(c claude.Client) StandardExecutorOption {
	return func(e *StandardExecutor) { e.haikuClient = c }
}

// WithStandardOrcConfig sets the orc config for validation settings.
func WithStandardOrcConfig(cfg *config.Config) StandardExecutorOption {
	return func(e *StandardExecutor) { e.orcConfig = cfg }
}

// WithStandardResumeSessionID sets the session ID to resume from.
// When set, uses Claude's --resume flag instead of starting a fresh session.
func WithStandardResumeSessionID(id string) StandardExecutorOption {
	return func(e *StandardExecutor) { e.resumeSessionID = id }
}

// NewStandardExecutor creates a new standard executor.
func NewStandardExecutor(mgr session.SessionManager, opts ...StandardExecutorOption) *StandardExecutor {
	e := &StandardExecutor{
		manager:   mgr,
		logger:    slog.Default(),
		publisher: NewEventPublisher(nil), // Initialize with nil-safe wrapper
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

	// Transcript persistence is handled via JSONL sync from Claude Code's session files
	// (see jsonl_sync.go), not through the publisher buffer

	// Generate session ID: {task_id}-{phase_id}
	// If resuming, use the stored session ID instead
	sessionID := fmt.Sprintf("%s-%s", t.ID, p.ID)
	isResume := e.resumeSessionID != ""
	if isResume {
		sessionID = e.resumeSessionID
		e.logger.Info("resuming from previous session", "session_id", sessionID)
	}

	// Resolve model settings for this phase and weight
	modelSetting := e.config.ResolveModelSetting(string(t.Weight), p.ID)
	result.Model = modelSetting.Model

	// Create session adapter
	adapter, err := NewSessionAdapter(ctx, e.manager, SessionAdapterOptions{
		SessionID:   sessionID,
		Resume:      isResume,
		Model:       modelSetting.Model,
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
	defer func() {
		if closeErr := adapter.Close(); closeErr != nil {
			e.logger.Error("failed to close adapter", "error", closeErr)
		}
	}()

	// Update state with session info and JSONL path for live tailing (orc log --follow)
	// Persist immediately so `orc log --follow` can find the file and resume can use session ID.
	if s != nil {
		s.SetSession(adapter.SessionID(), modelSetting.Model, "running", 0)
		if adapter.JSONLPath() != "" {
			s.JSONLPath = adapter.JSONLPath()
		}
		if e.backend != nil {
			if err := e.backend.SaveState(s); err != nil {
				e.logger.Warn("failed to persist session info to state", "error", err)
			}
		}
	}

	// Load and render initial prompt using shared template module
	tmpl, err := LoadPromptTemplate(p)
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("load prompt: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}
	vars := BuildTemplateVars(t, p, s, 0, LoadRetryContextForPhase(s))

	// Load spec content from database (specs are not stored as file artifacts)
	vars = vars.WithSpecFromDatabase(e.backend, t.ID)

	// Load review context for review phases (round 2+ needs prior findings)
	if p.ID == "review" {
		// Determine review round from config (default 1 for first review)
		round := 1
		if e.config.OrcConfig != nil {
			// Check if this is a subsequent review round based on state
			if s != nil && s.Phases != nil {
				if ps, ok := s.Phases["review"]; ok && ps.Status == "completed" {
					round = 2 // Re-running review means it's round 2
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

			e.logger.Info("UI testing enabled",
				"task", t.ID,
				"phase", p.ID,
				"screenshot_dir", screenshotDir,
			)
		}
	}

	// Add initiative context if task belongs to an initiative
	if initCtx := LoadInitiativeContext(t, e.backend); initCtx != nil {
		vars = vars.WithInitiativeContext(*initCtx)
		e.logger.Info("initiative context injected",
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
			e.logger.Info("automation context injected",
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

	// Iteration loop
	var lastResponse string
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		e.publisher.PhaseStart(t.ID, p.ID)

		// Publish prompt transcript
		e.publisher.Transcript(t.ID, p.ID, iteration, "prompt", promptText)

		// Execute turn with streaming and progress tracking
		isSpecPhase := p.ID == "spec"
		progressOpts := StreamProgressOptions{
			TurnTimeout:       e.config.TurnTimeout,
			HeartbeatInterval: e.config.HeartbeatInterval,
			IdleTimeout:       e.config.IdleTimeout,
			OnChunk: func(chunk string) {
				// Publish chunk for real-time display
				e.publisher.TranscriptChunk(t.ID, p.ID, iteration, chunk)
			},
			OnActivityChange: func(state ActivityState) {
				// For spec phase, translate generic states to spec-specific states
				if isSpecPhase {
					switch state {
					case ActivityWaitingAPI:
						state = ActivitySpecAnalyzing
					case ActivityStreaming:
						state = ActivitySpecWriting
					}
				}
				e.publisher.Activity(t.ID, p.ID, string(state))
			},
			OnHeartbeat: func() {
				e.publisher.Heartbeat(t.ID, p.ID, iteration)
			},
			OnIdleWarning: func(idleDuration time.Duration) {
				e.logger.Warn("API idle warning",
					"task", t.ID,
					"phase", p.ID,
					"idle_duration", idleDuration,
				)
				e.publisher.Warning(t.ID, p.ID, fmt.Sprintf("No activity for %s - API may be slow", idleDuration.Round(time.Second)))
			},
			OnTurnTimeout: func(turnDuration time.Duration) {
				e.logger.Warn("turn timeout",
					"task", t.ID,
					"phase", p.ID,
					"duration", turnDuration,
				)
				e.publisher.Warning(t.ID, p.ID, fmt.Sprintf("Turn timeout after %s", turnDuration.Round(time.Second)))
			},
		}

		turnResult, err := adapter.StreamTurnWithProgress(ctx, promptText, progressOpts)

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
			// Continue with next iteration
			// For session-based execution, we don't need to re-render the full prompt
			// The session maintains context. Just send a continuation prompt.
			promptText = "Continue working on the task. Remember to output <phase_complete>true</phase_complete> when you're done."
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

	// Sync JSONL transcripts to database
	if jsonlPath := adapter.JSONLPath(); jsonlPath != "" && e.backend != nil {
		syncer := NewJSONLSyncer(e.backend, e.logger)
		if err := syncer.SyncFromFile(ctx, jsonlPath, SyncOptions{
			TaskID: t.ID,
			Phase:  p.ID,
			Append: true, // Append mode to avoid duplicates on re-execution
		}); err != nil {
			e.logger.Warn("failed to sync JSONL transcripts", "error", err, "path", jsonlPath)
		} else {
			e.logger.Debug("synced JSONL transcripts", "task", t.ID, "phase", p.ID, "path", jsonlPath)
		}
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
