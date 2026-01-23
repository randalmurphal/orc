// Package executor provides the execution engine for orc.
// This file contains the WorkflowExecutor, its configuration, and the main Run() function.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/tokenpool"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/internal/workflow"
)

// ContextType determines how the workflow is executed.
type ContextType string

const (
	// ContextDefault creates a new task with worktree.
	ContextDefault ContextType = "default"

	// ContextTask attaches to an existing task.
	ContextTask ContextType = "task"

	// ContextBranch operates on an existing branch without a task.
	ContextBranch ContextType = "branch"

	// ContextPR operates on a pull request branch.
	ContextPR ContextType = "pr"

	// ContextStandalone runs without task or special git setup.
	ContextStandalone ContextType = "standalone"
)

// WorkflowRunOptions configures a workflow run.
type WorkflowRunOptions struct {
	// ContextType determines how the workflow executes.
	ContextType ContextType

	// Prompt is the user-provided task description.
	Prompt string

	// Instructions are additional guidance for this run.
	Instructions string

	// TaskID is set when ContextType is ContextTask.
	TaskID string

	// Branch is set when ContextType is ContextBranch.
	Branch string

	// PRID is set when ContextType is ContextPR.
	PRID int

	// Category helps Claude understand the type of work.
	Category task.Category

	// Variables are additional variables to inject.
	Variables map[string]string

	// Stream enables real-time output streaming.
	Stream bool
}

// WorkflowExecutor runs workflows using the new database-first workflow system.
type WorkflowExecutor struct {
	backend       storage.Backend
	projectDB     *db.ProjectDB
	globalDB      *db.GlobalDB // Global database for cost tracking
	orcConfig     *config.Config
	resolver      *variable.Resolver
	gateEvaluator *gate.Evaluator
	logger        *slog.Logger
	workingDir    string
	claudePath    string

	// Optional components
	gitOps             *git.Git
	publisher          *PublishHelper
	tokenPool          *tokenpool.Pool          // For automatic account switching on rate limits
	automationSvc      *automation.Service      // For automation event triggers
	sessionBroadcaster *SessionBroadcaster      // For real-time session metrics
	resourceTracker    *ResourceTracker         // For orphan process detection

	// Per-run state (set during Run)
	worktreePath string        // Path to worktree (if created)
	worktreeGit  *git.Git      // Git ops scoped to worktree
	execState    *state.State  // Execution state (for task-based contexts)
	heartbeat    *HeartbeatRunner
	fileWatcher  *FileWatcher

	// turnExecutor is injected for testing to avoid spawning real Claude CLI.
	turnExecutor TurnExecutor
}

// WorkflowExecutorOption configures a WorkflowExecutor.
type WorkflowExecutorOption func(*WorkflowExecutor)

// WithWorkflowGitOps sets the git operations handler.
func WithWorkflowGitOps(g *git.Git) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.gitOps = g
	}
}

// WithWorkflowPublisher sets the event publisher.
func WithWorkflowPublisher(p events.Publisher) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.publisher = NewPublishHelper(p)
	}
}

// WithWorkflowLogger sets the logger.
func WithWorkflowLogger(l *slog.Logger) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.logger = l
	}
}

// WithWorkflowClaudePath sets the path to the Claude CLI executable.
func WithWorkflowClaudePath(path string) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.claudePath = path
	}
}

// WithWorkflowGlobalDB sets the global database for cost tracking.
func WithWorkflowGlobalDB(gdb *db.GlobalDB) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.globalDB = gdb
	}
}

// WithWorkflowTurnExecutor sets a TurnExecutor for testing.
// When set, executeWithClaude uses this instead of creating a real ClaudeExecutor.
func WithWorkflowTurnExecutor(te TurnExecutor) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.turnExecutor = te
	}
}

// WithWorkflowTokenPool sets the token pool for automatic account switching on rate limits.
func WithWorkflowTokenPool(pool *tokenpool.Pool) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.tokenPool = pool
	}
}

// WithWorkflowAutomationService sets the automation service for event triggers.
func WithWorkflowAutomationService(svc *automation.Service) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.automationSvc = svc
	}
}

// WithWorkflowSessionBroadcaster sets the session broadcaster for real-time metrics.
func WithWorkflowSessionBroadcaster(sb *SessionBroadcaster) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.sessionBroadcaster = sb
	}
}

// WithWorkflowResourceTracker sets the resource tracker for orphan process detection.
func WithWorkflowResourceTracker(rt *ResourceTracker) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.resourceTracker = rt
	}
}

// NewWorkflowExecutor creates a new workflow executor.
func NewWorkflowExecutor(
	backend storage.Backend,
	projectDB *db.ProjectDB,
	orcConfig *config.Config,
	workingDir string,
	opts ...WorkflowExecutorOption,
) *WorkflowExecutor {
	// Try to open global DB for cost tracking (best-effort)
	globalDB, _ := db.OpenGlobal()

	we := &WorkflowExecutor{
		backend:       backend,
		projectDB:     projectDB,
		globalDB:      globalDB,
		orcConfig:     orcConfig,
		resolver:      variable.NewResolver(workingDir),
		gateEvaluator: gate.New(nil),
		workingDir:    workingDir,
		logger:        slog.Default(),
		claudePath:    "claude",
		publisher:     NewPublishHelper(nil), // Initialize with nil-safe wrapper
	}

	for _, opt := range opts {
		opt(we)
	}

	return we
}

// Run executes a workflow with the given options.
// This is the main entry point for workflow execution.
func (we *WorkflowExecutor) Run(ctx context.Context, workflowID string, opts WorkflowRunOptions) (*WorkflowRunResult, error) {
	// Load workflow from database
	wf, err := we.projectDB.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", workflowID, err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Load workflow phases
	phases, err := we.projectDB.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow phases: %w", err)
	}

	// Load workflow variables
	workflowVars, err := we.projectDB.GetWorkflowVariables(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow variables: %w", err)
	}

	// Create workflow run record
	runID, err := we.backend.GetNextWorkflowRunID()
	if err != nil {
		return nil, fmt.Errorf("get next run ID: %w", err)
	}

	// Build context data based on context type
	contextData := we.buildContextData(opts)

	run := &db.WorkflowRun{
		ID:           runID,
		WorkflowID:   workflowID,
		ContextType:  string(opts.ContextType),
		ContextData:  contextData,
		Prompt:       opts.Prompt,
		Instructions: opts.Instructions,
		Status:       string(workflow.RunStatusPending),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Handle task creation for default context
	var t *task.Task
	if opts.ContextType == ContextDefault {
		t, err = we.createTaskForRun(opts)
		if err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
		run.TaskID = &t.ID

		// Infer weight from workflow ID for newly created tasks
		if t.Weight == "" {
			if inferred := workflow.GetWeightForWorkflow(workflowID); inferred != "" {
				t.Weight = task.Weight(inferred)
				if err := we.backend.SaveTask(t); err != nil {
					we.logger.Warn("failed to save inferred weight", "task_id", t.ID, "error", err)
				}
			}
		}
	} else if opts.ContextType == ContextTask {
		// Load existing task
		t, err = we.backend.LoadTask(opts.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %s: %w", opts.TaskID, err)
		}
		run.TaskID = &t.ID

		// Infer weight if not set on existing task
		if t.Weight == "" {
			if inferred := workflow.GetWeightForWorkflow(workflowID); inferred != "" {
				t.Weight = task.Weight(inferred)
				if err := we.backend.SaveTask(t); err != nil {
					we.logger.Warn("failed to save inferred weight", "task_id", t.ID, "error", err)
				}
			}
		}
	}

	// Initialize execution state for task-based contexts
	if t != nil {
		// Load existing state or create new
		we.execState, err = we.backend.LoadState(t.ID)
		if err != nil || we.execState == nil {
			we.execState = state.New(t.ID)
		}

		// Set execution info (PID, hostname, heartbeat)
		hostname, _ := os.Hostname()
		we.execState.StartExecution(os.Getpid(), hostname)
		if err := we.backend.SaveState(we.execState); err != nil {
			we.logger.Error("failed to save initial state", "task_id", t.ID, "error", err)
		}

		// Start heartbeat runner for orphan detection
		we.heartbeat = NewHeartbeatRunner(we.backend, we.execState, we.logger)
		we.heartbeat.Start(ctx)
		defer we.heartbeat.Stop()

		// Take resource snapshot before execution (for orphan process detection)
		if we.resourceTracker != nil {
			if err := we.resourceTracker.SnapshotBefore(); err != nil {
				we.logger.Warn("failed to take resource snapshot", "error", err)
			}
			defer we.runResourceAnalysis()
		}

		// Notify session broadcaster that a task has started
		if we.sessionBroadcaster != nil {
			we.sessionBroadcaster.OnTaskStart(ctx)
			defer we.sessionBroadcaster.OnTaskComplete(ctx)
		}
	}

	// Set up SIGUSR1 handler for external pause requests
	pauseCh := make(chan os.Signal, 1)
	signal.Notify(pauseCh, syscall.SIGUSR1)
	defer func() {
		signal.Stop(pauseCh)
		select {
		case <-pauseCh:
		default:
		}
	}()

	// Create a cancellable context for pause handling
	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()

	go func() {
		select {
		case <-pauseCh:
			we.logger.Info("SIGUSR1 received, initiating graceful pause", "task", t.ID)
			execCancel()
		case <-execCtx.Done():
			return
		}
	}()

	// Setup worktree for task-based contexts
	if t != nil && we.orcConfig.Worktree.Enabled && we.gitOps != nil {
		if err := we.setupWorktree(t); err != nil {
			we.failSetup(run, t, err)
			return nil, fmt.Errorf("setup worktree: %w", err)
		}
		// Cleanup worktree on exit based on config and success
		defer we.cleanupWorktree(t)

		// Start file watcher for real-time diff detection
		if we.publisher != nil && we.worktreePath != "" {
			baseRef := ResolveTargetBranchForTask(t, we.backend, we.orcConfig)
			detector := NewGitDiffDetector(we.worktreePath)
			we.fileWatcher = NewFileWatcher(detector, we.publisher, t.ID, we.worktreePath, baseRef, we.logger)
			we.fileWatcher.Start(execCtx)
			defer we.fileWatcher.Stop()
		}
	}

	// Sync with target branch before execution starts
	if t != nil && we.orcConfig.ShouldSyncOnStart() && we.orcConfig.ShouldSyncForWeight(string(t.Weight)) {
		if err := we.syncOnTaskStart(execCtx, t); err != nil {
			we.logger.Error("sync-on-start failed", "task", t.ID, "error", err)
			we.failSetup(run, t, err)
			return nil, fmt.Errorf("sync on start: %w", err)
		}
	}

	// Check spec requirements for non-trivial tasks
	if err := we.checkSpecRequirements(t, phases); err != nil {
		we.failSetup(run, t, err)
		return nil, err
	}

	// Save run
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return nil, fmt.Errorf("save workflow run: %w", err)
	}

	// Build resolution context
	rctx := we.buildResolutionContext(opts, t, wf, run)

	// Convert workflow variables to definitions
	varDefs := we.convertToDefinitions(workflowVars)

	// Resolve all variables
	vars, err := we.resolver.ResolveAll(ctx, varDefs, rctx)
	if err != nil {
		we.failRun(run, t, fmt.Errorf("resolve variables: %w", err))
		return nil, err
	}

	// Store variable snapshot
	varsJSON, _ := json.Marshal(vars)
	run.VariablesSnapshot = string(varsJSON)
	run.Status = string(workflow.RunStatusRunning)
	run.StartedAt = timePtr(time.Now())
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return nil, fmt.Errorf("save workflow run: %w", err)
	}

	// Sync task status to Running
	if t != nil {
		t.Status = task.StatusRunning
		if err := we.backend.SaveTask(t); err != nil {
			we.logger.Error("failed to save task status running", "task_id", t.ID, "error", err)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
	}

	// Execute phases in order
	result := &WorkflowRunResult{
		RunID:        runID,
		WorkflowID:   workflowID,
		StartedAt:    *run.StartedAt,
		PhaseResults: make([]PhaseResult, 0, len(phases)),
	}

	// Track retry counts per phase to prevent infinite loops
	retryCounts := make(map[string]int)
	maxRetries := 3
	if we.orcConfig != nil && we.orcConfig.Retry.MaxRetries > 0 {
		maxRetries = we.orcConfig.Retry.MaxRetries
	}

	for i := 0; i < len(phases); i++ {
		phase := phases[i]

		// Resume logic: skip phases that are already completed in task state
		// This allows resuming a task from where it left off
		if we.execState != nil && t != nil {
			if ps, ok := we.execState.Phases[phase.PhaseTemplateID]; ok {
				if ps.Status == state.StatusCompleted {
					we.logger.Info("skipping completed phase", "phase", phase.PhaseTemplateID)
					// Load artifact from completed phase for variable chaining
					// Phase outputs are stored in unified phase_outputs table keyed by run ID
					if output, err := we.backend.GetPhaseOutput(run.ID, phase.PhaseTemplateID); err == nil && output != nil {
						applyArtifactToVars(vars, rctx.PriorOutputs, phase.PhaseTemplateID, output.Content)
					}
					continue
				}
			}
		}

		// Load phase template
		tmpl, err := we.projectDB.GetPhaseTemplate(phase.PhaseTemplateID)
		if err != nil {
			we.failRun(run, t, fmt.Errorf("load phase template %s: %w", phase.PhaseTemplateID, err))
			return result, err
		}
		if tmpl == nil {
			we.failRun(run, t, fmt.Errorf("phase template not found: %s", phase.PhaseTemplateID))
			return result, fmt.Errorf("phase template not found: %s", phase.PhaseTemplateID)
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			we.interruptRun(run, t, phase.PhaseTemplateID, ctx.Err())
			return result, ctx.Err()
		}

		// Create run phase record
		runPhase := &db.WorkflowRunPhase{
			WorkflowRunID:   runID,
			PhaseTemplateID: phase.PhaseTemplateID,
			Status:          string(workflow.PhaseStatusPending),
		}
		if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
			return result, fmt.Errorf("save run phase: %w", err)
		}

		// Update run with current phase
		run.CurrentPhase = phase.PhaseTemplateID
		if err := we.backend.SaveWorkflowRun(run); err != nil {
			return result, fmt.Errorf("update run phase: %w", err)
		}

		// Update phase in resolution context
		rctx.Phase = tmpl.ID

		// Enrich context with phase-specific data (review findings, test results, etc.)
		we.enrichContextForPhase(rctx, tmpl.ID, t, we.execState)

		// Re-resolve variables with updated context
		vars, err = we.resolver.ResolveAll(ctx, varDefs, rctx)
		if err != nil {
			we.failRun(run, t, fmt.Errorf("resolve variables for phase %s: %w", tmpl.ID, err))
			return result, err
		}

		// Execute the phase with timeout support (PhaseMax config)
		phaseResult, err := we.executePhaseWithTimeout(ctx, tmpl, phase, vars, rctx, run, runPhase, t)
		result.PhaseResults = append(result.PhaseResults, phaseResult)

		if err != nil {
			we.failRun(run, t, err)
			return result, err
		}

		// Update variables with phase output if artifact was produced
		if phaseResult.Artifact != "" {
			applyArtifactToVars(vars, rctx.PriorOutputs, phaseResult.PhaseID, phaseResult.Artifact)
		}

		// Evaluate phase gate
		gateResult, gateErr := we.evaluatePhaseGate(ctx, tmpl, phase, phaseResult.Artifact, t)
		if gateErr != nil {
			we.logger.Warn("gate evaluation failed", "phase", tmpl.ID, "error", gateErr)
			// Continue on gate error - don't block automation
		} else if gateResult != nil {
			// Handle gate decision
			if gateResult.Pending {
				// Task is blocked waiting for human decision
				we.logger.Info("gate decision pending", "phase", tmpl.ID)
				if t != nil {
					t.Status = task.StatusBlocked
					if err := we.backend.SaveTask(t); err != nil {
						we.logger.Error("failed to save blocked task", "error", err)
					}
				}
				if we.execState != nil {
					we.execState.Error = fmt.Sprintf("blocked at gate: %s (phase %s)", gateResult.Reason, tmpl.ID)
					we.backend.SaveState(we.execState)
				}
				return result, fmt.Errorf("blocked at gate: %s", gateResult.Reason)
			}

			if !gateResult.Approved {
				// Gate rejected - check if we should retry
				if gateResult.RetryPhase != "" && retryCounts[tmpl.ID] < maxRetries {
					retryCounts[tmpl.ID]++
					we.logger.Info("gate rejected, retrying from earlier phase",
						"failed_phase", tmpl.ID,
						"reason", gateResult.Reason,
						"retry_from", gateResult.RetryPhase,
						"retry_count", retryCounts[tmpl.ID],
					)

					// Find retry target phase index
					retryIdx := -1
					for j := 0; j <= i; j++ {
						if phases[j].PhaseTemplateID == gateResult.RetryPhase {
							retryIdx = j
							break
						}
					}

					if retryIdx >= 0 {
						// Track quality metrics
						if t != nil {
							t.RecordPhaseRetry(tmpl.ID)
							if tmpl.ID == "review" {
								t.RecordReviewRejection()
							}
							we.backend.SaveTask(t)
						}

						// Save retry context
						if we.execState != nil {
							reason := fmt.Sprintf("Gate rejected for phase %s: %s", tmpl.ID, gateResult.Reason)
							we.execState.SetRetryContext(tmpl.ID, gateResult.RetryPhase, reason, phaseResult.Artifact, retryCounts[tmpl.ID])
							we.backend.SaveState(we.execState)
						}

						// Jump back to retry phase
						i = retryIdx - 1 // Will be incremented by loop
						continue
					}
				}

				// No retry - log rejection and continue (automation-first)
				we.logger.Warn("gate rejected, continuing anyway (automation mode)",
					"phase", tmpl.ID,
					"reason", gateResult.Reason,
				)
			}

			// Record gate decision in state
			if we.execState != nil {
				we.execState.RecordGateDecision(tmpl.ID, tmpl.GateType, gateResult.Approved, gateResult.Reason)
				we.backend.SaveState(we.execState)
			}
		}
	}

	// Run completion action (sync, PR/merge) for task-based contexts
	if t != nil && we.gitOps != nil {
		completionErr := we.runCompletion(ctx, t)
		if completionErr != nil {
			// Check if it's a conflict or merge error
			if errors.Is(completionErr, ErrSyncConflict) || errors.Is(completionErr, ErrMergeFailed) {
				we.logger.Error("completion failed",
					"task", t.ID,
					"error", completionErr)

				// Mark task as blocked, not completed
				t.Status = task.StatusBlocked
				if t.Metadata == nil {
					t.Metadata = make(map[string]string)
				}
				if errors.Is(completionErr, ErrSyncConflict) {
					t.Metadata["blocked_reason"] = "sync_conflict"
				} else {
					t.Metadata["blocked_reason"] = "merge_failed"
				}
				t.Metadata["blocked_error"] = completionErr.Error()

				if we.execState != nil {
					we.execState.ClearExecution()
					we.backend.SaveState(we.execState)
				}
				we.backend.SaveTask(t)

				// Run is completed but task is blocked
				run.Status = string(workflow.RunStatusCompleted)
				run.CompletedAt = timePtr(time.Now())
				we.backend.SaveWorkflowRun(run)

				result.CompletedAt = run.CompletedAt
				result.Success = false
				result.Error = completionErr.Error()
				return result, fmt.Errorf("%w: %v", ErrTaskBlocked, completionErr)
			}

			// Other completion errors - log warning but continue
			we.logger.Warn("completion action failed", "error", completionErr)
		}
	}

	// Complete run
	run.Status = string(workflow.RunStatusCompleted)
	run.CompletedAt = timePtr(time.Now())
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return result, fmt.Errorf("complete run: %w", err)
	}

	// Sync task status to Completed
	if t != nil {
		t.Status = task.StatusCompleted
		completedAt := time.Now()
		t.CompletedAt = &completedAt
		if err := we.backend.SaveTask(t); err != nil {
			we.logger.Error("failed to save task status completed", "task_id", t.ID, "error", err)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
		// Trigger automation event for task completion
		we.triggerAutomationEvent(execCtx, automation.EventTaskCompleted, t, "")
	}

	// Clear execution state
	if we.execState != nil {
		we.execState.Complete()
		we.execState.ClearExecution()
		we.backend.SaveState(we.execState)
	}

	// Populate result fields from run
	if t != nil {
		result.TaskID = t.ID
	}
	result.TotalCostUSD = run.TotalCostUSD
	result.TotalTokens = run.TotalInputTokens + run.TotalOutputTokens
	result.CompletedAt = run.CompletedAt
	result.Success = true

	return result, nil
}

// WorkflowRunResult contains the result of a workflow execution.
type WorkflowRunResult struct {
	RunID        string
	WorkflowID   string
	TaskID       string
	StartedAt    time.Time
	CompletedAt  *time.Time
	Success      bool
	Error        string
	PhaseResults []PhaseResult
	TotalCostUSD float64
	TotalTokens  int
}

// PhaseResult contains the result of a phase execution.
type PhaseResult struct {
	PhaseID             string
	Status              string
	Iterations          int
	DurationMS          int64
	Artifact            string
	Error               string
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	CostUSD             float64
}

// Helper functions

// applyArtifactToVars updates variable maps with phase output artifacts.
// Called both when resuming from completed phases and after phase completion.
func applyArtifactToVars(vars map[string]string, priorOutputs map[string]string, phaseID, artifact string) {
	vars["OUTPUT_"+phaseID] = artifact
	switch phaseID {
	case "spec", "tiny_spec":
		vars["SPEC_CONTENT"] = artifact
	case "design":
		vars["DESIGN_CONTENT"] = artifact
	case "tdd_write":
		vars["TDD_TESTS_CONTENT"] = artifact
	case "breakdown":
		vars["BREAKDOWN_CONTENT"] = artifact
	case "research":
		vars["RESEARCH_CONTENT"] = artifact
	}
	if priorOutputs != nil {
		priorOutputs[phaseID] = artifact
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func truncateTitle(s string) string {
	const maxLen = 80
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractArtifactFromJSON extracts the artifact field from phase JSON output.
func extractArtifactFromJSON(output string) string {
	var data struct {
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return ""
	}
	return data.Artifact
}
