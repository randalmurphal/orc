// Package executor provides the execution engine for orc.
// This file contains the workflow execution system which replaces task-centric execution.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
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
	backpressure *BackpressureRunner // For deterministic quality checks

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
	} else if opts.ContextType == ContextTask {
		// Load existing task
		t, err = we.backend.LoadTask(opts.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %s: %w", opts.TaskID, err)
		}
		run.TaskID = &t.ID
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

	// Initialize backpressure runner (needs worktree path)
	we.initBackpressure()

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
					// Specs are stored separately via LoadSpec, artifacts via LoadArtifact
					var artifact string
					if phase.PhaseTemplateID == "spec" || phase.PhaseTemplateID == "tiny_spec" {
						artifact, _ = we.backend.LoadSpec(t.ID)
					} else {
						artifact, _ = we.backend.LoadArtifact(t.ID, phase.PhaseTemplateID)
					}
					if artifact != "" {
						vars["OUTPUT_"+phase.PhaseTemplateID] = artifact
						switch phase.PhaseTemplateID {
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
						rctx.PriorOutputs[phase.PhaseTemplateID] = artifact
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
			we.interruptRun(run, t, ctx.Err())
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

		// Execute the phase
		phaseResult, err := we.executePhase(ctx, tmpl, phase, vars, rctx, run, runPhase, t)
		result.PhaseResults = append(result.PhaseResults, phaseResult)

		if err != nil {
			we.failRun(run, t, err)
			return result, err
		}

		// Update variables with phase output if artifact was produced
		if phaseResult.Artifact != "" {
			vars["OUTPUT_"+phaseResult.PhaseID] = phaseResult.Artifact
			// Update common aliases
			switch phaseResult.PhaseID {
			case "spec", "tiny_spec":
				vars["SPEC_CONTENT"] = phaseResult.Artifact
			case "design":
				vars["DESIGN_CONTENT"] = phaseResult.Artifact
			case "tdd_write":
				vars["TDD_TESTS_CONTENT"] = phaseResult.Artifact
			case "breakdown":
				vars["BREAKDOWN_CONTENT"] = phaseResult.Artifact
			case "research":
				vars["RESEARCH_CONTENT"] = phaseResult.Artifact
			}
			rctx.PriorOutputs[phaseResult.PhaseID] = phaseResult.Artifact
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
	PhaseID      string
	Status       string
	Iterations   int
	DurationMS   int64
	Artifact     string
	Error        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// buildContextData creates the context data JSON for a run.
func (we *WorkflowExecutor) buildContextData(opts WorkflowRunOptions) string {
	data := map[string]any{
		"prompt":       opts.Prompt,
		"instructions": opts.Instructions,
	}

	switch opts.ContextType {
	case ContextTask:
		data["task_id"] = opts.TaskID
	case ContextBranch:
		data["branch"] = opts.Branch
	case ContextPR:
		data["pr_id"] = opts.PRID
	}

	j, _ := json.Marshal(data)
	return string(j)
}

// createTaskForRun creates a task for a default context run.
func (we *WorkflowExecutor) createTaskForRun(opts WorkflowRunOptions) (*task.Task, error) {
	taskID, err := we.backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("get next task ID: %w", err)
	}

	t := &task.Task{
		ID:          taskID,
		Title:       truncateTitle(opts.Prompt),
		Description: opts.Prompt,
		Category:    opts.Category,
		Status:      task.StatusCreated,
		Queue:       task.QueueActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if t.Category == "" {
		t.Category = task.CategoryFeature
	}

	if err := we.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}

	return t, nil
}

// buildResolutionContext creates the variable resolution context.
func (we *WorkflowExecutor) buildResolutionContext(
	opts WorkflowRunOptions,
	t *task.Task,
	wf *db.Workflow,
	run *db.WorkflowRun,
) *variable.ResolutionContext {
	rctx := &variable.ResolutionContext{
		WorkflowID:    wf.ID,
		WorkflowRunID: run.ID,
		Prompt:        opts.Prompt,
		Instructions:  opts.Instructions,
		WorkingDir:    we.workingDir,
		ProjectRoot:   we.workingDir,
		PriorOutputs:  make(map[string]string),
	}

	if t != nil {
		rctx.TaskID = t.ID
		rctx.TaskTitle = t.Title
		rctx.TaskDescription = t.Description
		rctx.TaskCategory = string(t.Category)
		rctx.TaskWeight = string(t.Weight)
		rctx.TaskBranch = t.Branch
		rctx.RequiresUITesting = t.RequiresUITesting

		// Resolve target branch
		rctx.TargetBranch = ResolveTargetBranchForTask(t, we.backend, we.orcConfig)

		// Load initiative context if task belongs to an initiative
		if t.InitiativeID != "" {
			we.loadInitiativeContext(rctx, t.InitiativeID)
		}

		// Set up screenshot dir for UI testing tasks
		if t.RequiresUITesting && we.workingDir != "" {
			rctx.ScreenshotDir = task.ScreenshotsPath(we.workingDir, t.ID)
			if err := os.MkdirAll(rctx.ScreenshotDir, 0755); err != nil {
				we.logger.Warn("failed to create screenshot directory", "error", err)
			}
		}
	}

	// Load constitution content (project-level principles)
	if content, _, err := we.backend.LoadConstitution(); err == nil && content != "" {
		rctx.ConstitutionContent = content
	}

	// Load project detection from database
	we.loadProjectDetectionContext(rctx)

	// Set testing configuration from orc config
	if we.orcConfig != nil {
		rctx.CoverageThreshold = we.orcConfig.Testing.CoverageThreshold
	}

	// Merge user-provided variables
	if opts.Variables != nil {
		rctx.Environment = opts.Variables
	}

	return rctx
}

// loadInitiativeContext loads initiative data into the resolution context.
func (we *WorkflowExecutor) loadInitiativeContext(rctx *variable.ResolutionContext, initiativeID string) {
	init, err := we.backend.LoadInitiative(initiativeID)
	if err != nil {
		we.logger.Debug("failed to load initiative",
			"initiative_id", initiativeID,
			"error", err,
		)
		return
	}

	rctx.InitiativeID = init.ID
	rctx.InitiativeTitle = init.Title
	rctx.InitiativeVision = init.Vision

	// Format decisions as markdown
	if len(init.Decisions) > 0 {
		var sb strings.Builder
		for _, d := range init.Decisions {
			fmt.Fprintf(&sb, "- **%s**: %s", d.ID, d.Decision)
			if d.Rationale != "" {
				fmt.Fprintf(&sb, " (%s)", d.Rationale)
			}
			sb.WriteString("\n")
		}
		rctx.InitiativeDecisions = strings.TrimSuffix(sb.String(), "\n")
	}

	we.logger.Debug("initiative context loaded",
		"initiative_id", init.ID,
		"has_vision", init.Vision != "",
		"decision_count", len(init.Decisions),
	)
}

// loadProjectDetectionContext loads project detection data into the resolution context.
func (we *WorkflowExecutor) loadProjectDetectionContext(rctx *variable.ResolutionContext) {
	dbBackend, ok := we.backend.(*storage.DatabaseBackend)
	if !ok {
		return
	}

	detection, err := dbBackend.DB().LoadDetection()
	if err != nil || detection == nil {
		return
	}

	rctx.Language = detection.Language
	rctx.HasTests = detection.HasTests
	rctx.TestCommand = detection.TestCommand
	rctx.LintCommand = detection.LintCommand
	rctx.Frameworks = detection.Frameworks

	// Determine HasFrontend from frameworks
	for _, f := range detection.Frameworks {
		switch f {
		case "react", "vue", "angular", "svelte", "nextjs", "nuxt", "gatsby", "astro":
			rctx.HasFrontend = true
		}
	}
}

// enrichContextForPhase adds phase-specific context to the resolution context.
// Call this before executing each phase to load review findings, artifacts, etc.
func (we *WorkflowExecutor) enrichContextForPhase(rctx *variable.ResolutionContext, phaseID string, t *task.Task, s *state.State) {
	// Load retry context from state
	if s != nil {
		rctx.RetryContext = LoadRetryContextForPhase(s)
	}

	// Load review context for review phases
	if phaseID == "review" && t != nil {
		we.loadReviewContext(rctx, t.ID, s)
	}

	// Load test results for validate phase
	if (phaseID == "validate" || phaseID == "review") && t != nil {
		rctx.TestResults = we.loadPriorPhaseContent(t.ID, s, "test")
	}

	// Load TDD test plan if it exists
	if t != nil {
		rctx.TDDTestPlan = we.loadPriorPhaseContent(t.ID, s, "tdd_write_plan")
	}

	// Load automation context for automation tasks
	if t != nil && t.IsAutomation {
		we.loadAutomationContext(rctx, t)
	}
}

// loadReviewContext loads review-specific context into the resolution context.
func (we *WorkflowExecutor) loadReviewContext(rctx *variable.ResolutionContext, taskID string, s *state.State) {
	// Determine review round from state
	round := 1
	if s != nil && s.Phases != nil {
		if ps, ok := s.Phases["review"]; ok && ps.Status == state.StatusCompleted {
			round = 2
		}
	}
	rctx.ReviewRound = round

	// Load previous round's findings for round 2+
	if round > 1 {
		findings, err := we.backend.LoadReviewFindings(taskID, round-1)
		if err != nil {
			we.logger.Debug("failed to load review findings",
				"task_id", taskID,
				"round", round-1,
				"error", err,
			)
			return
		}
		if findings != nil {
			rctx.ReviewFindings = formatReviewFindingsForPrompt(findings)
		}
	}
}

// loadAutomationContext loads automation task context.
func (we *WorkflowExecutor) loadAutomationContext(rctx *variable.ResolutionContext, t *task.Task) {
	// Load recent completed tasks
	tasks, err := we.backend.LoadAllTasks()
	if err == nil {
		rctx.RecentCompletedTasks = formatRecentCompletedTasksForPrompt(tasks, 20)
		rctx.RecentChangedFiles = collectRecentChangedFilesForPrompt(tasks, 10)
	}

	// Load CHANGELOG.md content
	changelogPath := filepath.Join(we.workingDir, "CHANGELOG.md")
	if content, err := os.ReadFile(changelogPath); err == nil {
		rctx.ChangelogContent = string(content)
	}

	// Load CLAUDE.md content
	claudeMDPath := filepath.Join(we.workingDir, "CLAUDE.md")
	if content, err := os.ReadFile(claudeMDPath); err == nil {
		rctx.ClaudeMDContent = string(content)
	}
}

// loadPriorPhaseContent loads content from a completed prior phase.
func (we *WorkflowExecutor) loadPriorPhaseContent(taskID string, s *state.State, phaseID string) string {
	// Check if phase is completed
	if s != nil && s.Phases != nil {
		ps, ok := s.Phases[phaseID]
		if ok && ps.Status != state.StatusCompleted {
			return ""
		}
	}

	// Try to load from artifact storage
	taskDir := task.TaskDir(taskID)
	artifactPath := filepath.Join(taskDir, "artifacts", phaseID+".md")
	if content, err := os.ReadFile(artifactPath); err == nil {
		return strings.TrimSpace(string(content))
	}

	return ""
}

// formatReviewFindingsForPrompt formats review findings for template injection.
func formatReviewFindingsForPrompt(findings *storage.ReviewFindings) string {
	if findings == nil {
		return "No findings from previous round."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Round %d Summary\n\n", findings.Round)
	sb.WriteString(findings.Summary)
	sb.WriteString("\n\n")

	// Count issues by severity
	highCount, mediumCount, lowCount := 0, 0, 0
	for _, issue := range findings.Issues {
		switch issue.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	fmt.Fprintf(&sb, "**Issues Found:** %d high, %d medium, %d low\n\n", highCount, mediumCount, lowCount)

	if len(findings.Issues) > 0 {
		sb.WriteString("### Issues to Verify\n\n")
		for i, issue := range findings.Issues {
			fmt.Fprintf(&sb, "%d. [%s] %s", i+1, strings.ToUpper(issue.Severity), issue.Description)
			if issue.File != "" {
				fmt.Fprintf(&sb, " (in %s", issue.File)
				if issue.Line > 0 {
					fmt.Fprintf(&sb, ":%d", issue.Line)
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
			if issue.Suggestion != "" {
				fmt.Fprintf(&sb, "   Suggested fix: %s\n", issue.Suggestion)
			}
		}
	}

	if len(findings.Positives) > 0 {
		sb.WriteString("\n### Positive Notes\n\n")
		for _, p := range findings.Positives {
			fmt.Fprintf(&sb, "- %s\n", p)
		}
	}

	if len(findings.Questions) > 0 {
		sb.WriteString("\n### Questions from Review\n\n")
		for _, q := range findings.Questions {
			fmt.Fprintf(&sb, "- %s\n", q)
		}
	}

	return sb.String()
}

// formatRecentCompletedTasksForPrompt formats recent completed tasks as a markdown list.
func formatRecentCompletedTasksForPrompt(tasks []*task.Task, limit int) string {
	var completed []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted {
			completed = append(completed, t)
		}
	}

	// Sort by completion time (most recent first) - already done by LoadAllTasks
	if len(completed) > limit {
		completed = completed[:limit]
	}

	var sb strings.Builder
	for _, t := range completed {
		fmt.Fprintf(&sb, "- **%s**: %s", t.ID, t.Title)
		if t.Category != "" {
			fmt.Fprintf(&sb, " [%s]", t.Category)
		}
		if t.Weight != "" {
			fmt.Fprintf(&sb, " (%s)", t.Weight)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// collectRecentChangedFilesForPrompt collects files changed in recent tasks.
func collectRecentChangedFilesForPrompt(tasks []*task.Task, limit int) string {
	var recent []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted {
			recent = append(recent, t)
		}
	}

	if len(recent) > limit {
		recent = recent[:limit]
	}

	seen := make(map[string]bool)
	var files []string

	for _, t := range recent {
		if t.Metadata == nil {
			continue
		}
		if changedFiles, ok := t.Metadata["changed_files"]; ok {
			for f := range strings.SplitSeq(changedFiles, ",") {
				f = strings.TrimSpace(f)
				if f != "" && !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}
	}

	return strings.Join(files, "\n")
}

// convertToDefinitions converts database workflow variables to variable definitions.
func (we *WorkflowExecutor) convertToDefinitions(wvs []*db.WorkflowVariable) []variable.Definition {
	defs := make([]variable.Definition, len(wvs))
	for i, wv := range wvs {
		defs[i] = variable.Definition{
			Name:         wv.Name,
			Description:  wv.Description,
			SourceType:   variable.SourceType(wv.SourceType),
			SourceConfig: json.RawMessage(wv.SourceConfig),
			Required:     wv.Required,
			DefaultValue: wv.DefaultValue,
			CacheTTL:     time.Duration(wv.CacheTTLSeconds) * time.Second,
		}
	}
	return defs
}

// executePhase runs a single phase of the workflow.
func (we *WorkflowExecutor) executePhase(
	ctx context.Context,
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
	vars variable.VariableSet,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	t *task.Task,
) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: tmpl.ID,
		Status:  string(workflow.PhaseStatusRunning),
	}

	startTime := time.Now()

	// Update phase status
	runPhase.Status = string(workflow.PhaseStatusRunning)
	runPhase.StartedAt = timePtr(startTime)
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return result, fmt.Errorf("update phase status: %w", err)
	}

	we.logger.Info("executing phase",
		"run_id", run.ID,
		"phase", tmpl.ID,
		"max_iterations", tmpl.MaxIterations,
	)

	// Publish phase start event for real-time UI updates
	if t != nil {
		we.publisher.PhaseStart(t.ID, tmpl.ID)
	}

	// Load prompt template
	promptContent, err := we.loadPhasePrompt(tmpl)
	if err != nil {
		result.Status = string(workflow.PhaseStatusFailed)
		result.Error = err.Error()
		return result, err
	}

	// Render template with variables
	renderedPrompt := variable.RenderTemplate(promptContent, vars)

	// Determine max iterations (phase override or template default)
	maxIter := tmpl.MaxIterations
	if phase.MaxIterationsOverride != nil {
		maxIter = *phase.MaxIterationsOverride
	}

	// Determine model (phase override or template default or global)
	model := we.resolvePhaseModel(tmpl, phase)

	// Build execution context for ClaudeExecutor
	// Use worktree path if available, otherwise fall back to original working dir
	execConfig := PhaseExecutionConfig{
		Prompt:        renderedPrompt,
		MaxIterations: maxIter,
		Model:         model,
		WorkingDir:    we.effectiveWorkingDir(),
		TaskID:        rctx.TaskID,
		PhaseID:       tmpl.ID,
		RunID:         run.ID,
		Thinking:      we.shouldUseThinking(tmpl, phase),
	}

	// Execute with ClaudeExecutor
	execResult, err := we.executeWithClaude(ctx, execConfig)
	if err != nil {
		result.Status = string(workflow.PhaseStatusFailed)
		result.Error = err.Error()
		runPhase.Status = string(workflow.PhaseStatusFailed)
		runPhase.Error = result.Error
		runPhase.CompletedAt = timePtr(time.Now())
		we.backend.SaveWorkflowRunPhase(runPhase)
		// Publish phase failed event for real-time UI updates
		if t != nil {
			we.publisher.PhaseFailed(t.ID, tmpl.ID, err)
		}
		return result, err
	}

	// Update result
	result.Status = string(workflow.PhaseStatusCompleted)
	result.Iterations = execResult.Iterations
	result.DurationMS = time.Since(startTime).Milliseconds()
	result.InputTokens = execResult.InputTokens
	result.OutputTokens = execResult.OutputTokens
	result.CostUSD = execResult.CostUSD

	// Extract artifact if phase produces one
	if tmpl.ProducesArtifact {
		result.Artifact = execResult.Artifact
		// Save artifact to database
		if result.Artifact != "" && t != nil {
			if err := we.backend.SaveArtifact(t.ID, tmpl.ID, result.Artifact, "workflow"); err != nil {
				we.logger.Warn("failed to save artifact",
					"task", t.ID,
					"phase", tmpl.ID,
					"error", err,
				)
			}
		}
	}

	// Update phase record
	runPhase.Status = string(workflow.PhaseStatusCompleted)
	runPhase.Iterations = result.Iterations
	runPhase.CompletedAt = timePtr(time.Now())
	runPhase.InputTokens = result.InputTokens
	runPhase.OutputTokens = result.OutputTokens
	runPhase.CostUSD = result.CostUSD
	if result.Artifact != "" {
		runPhase.Artifact = result.Artifact
	}
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		we.logger.Warn("failed to save run phase", "error", err)
	}

	// Publish phase complete event for real-time UI updates
	if t != nil {
		we.publisher.PhaseComplete(t.ID, tmpl.ID, "")
		// Trigger automation event for phase completion
		we.triggerAutomationEvent(ctx, automation.EventPhaseCompleted, t, tmpl.ID)
	}

	// Update run totals
	run.TotalCostUSD += result.CostUSD
	run.TotalInputTokens += result.InputTokens
	run.TotalOutputTokens += result.OutputTokens
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		we.logger.Warn("failed to update run totals", "error", err)
	}

	// Record cost to global database for cross-project analytics
	phaseModel := we.resolvePhaseModel(tmpl, phase)
	we.recordCostToGlobal(t, tmpl.ID, result, phaseModel, time.Since(startTime))

	// Sync transcripts to database
	if execResult.SessionID != "" && t != nil {
		we.syncTranscripts(ctx, execResult.SessionID, t.ID, tmpl.ID)
	}

	// Update execution state if available
	if we.execState != nil {
		we.execState.CompletePhase(tmpl.ID, "") // Empty commit SHA for workflow phases
		we.execState.AddCost(result.CostUSD)
		if err := we.backend.SaveState(we.execState); err != nil {
			we.logger.Warn("failed to save execution state", "error", err)
		}
	}

	return result, nil
}

// PhaseExecutionConfig holds configuration for a phase execution.
type PhaseExecutionConfig struct {
	Prompt        string
	MaxIterations int
	Model         string
	WorkingDir    string
	TaskID        string
	PhaseID       string
	RunID         string
	Thinking      bool
}

// PhaseExecutionResult holds the result of a phase execution.
type PhaseExecutionResult struct {
	Iterations   int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Artifact     string
	SessionID    string
}

// executeWithClaude runs the phase using Claude CLI.
func (we *WorkflowExecutor) executeWithClaude(ctx context.Context, cfg PhaseExecutionConfig) (*PhaseExecutionResult, error) {
	result := &PhaseExecutionResult{}

	// Inject ultrathink prefix if thinking is enabled
	prompt := cfg.Prompt
	if cfg.Thinking {
		prompt = "ultrathink\n\n" + prompt
	}

	// Generate session ID
	sessionID := fmt.Sprintf("%s-%s-%s", cfg.RunID, cfg.TaskID, cfg.PhaseID)
	result.SessionID = sessionID

	// Get schema for this phase
	schema := GetSchemaForPhase(cfg.PhaseID)

	// Use injected TurnExecutor for testing, or create real ClaudeExecutor
	var turnExec TurnExecutor
	if we.turnExecutor != nil {
		turnExec = we.turnExecutor
		turnExec.UpdateSessionID(sessionID)
	} else {
		turnExec = NewClaudeExecutor(
			WithClaudePath(we.claudePath),
			WithClaudeWorkdir(cfg.WorkingDir),
			WithClaudeModel(cfg.Model),
			WithClaudeSessionID(sessionID),
			WithClaudeMaxTurns(cfg.MaxIterations),
			WithClaudeLogger(we.logger),
			WithClaudePhaseID(cfg.PhaseID),
		)
	}

	// Set the schema
	if schema != "" {
		// Schema is set via phaseID, which GetSchemaForPhaseWithRound uses
	}

	// Execute turns until completion
	for i := 0; i < cfg.MaxIterations; i++ {
		// Check context
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		result.Iterations++

		// Execute turn
		turnResult, err := turnExec.ExecuteTurn(ctx, prompt)
		if err != nil {
			return result, fmt.Errorf("turn %d: %w", i+1, err)
		}

		// Accumulate tokens
		result.InputTokens += turnResult.Usage.InputTokens
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.CostUSD += turnResult.CostUSD

		// Check for completion
		status, reason, err := ParsePhaseSpecificResponse(cfg.PhaseID, 1, turnResult.Content)
		if err != nil {
			we.logger.Debug("parse phase response failed",
				"phase", cfg.PhaseID,
				"error", err,
			)
			// Continue iteration
			prompt = fmt.Sprintf("Continue. Previous output was not valid JSON. Iteration %d/%d.",
				i+2, cfg.MaxIterations)
			continue
		}

		switch status {
		case PhaseStatusComplete:
			// Run backpressure checks for implement phase before accepting completion
			if we.backpressure != nil && !ShouldSkipBackpressure(cfg.PhaseID) {
				bpResult := we.backpressure.Run(ctx)
				if bpResult.HasFailures() {
					we.logger.Info("backpressure failed, continuing iteration",
						"phase", cfg.PhaseID,
						"failures", bpResult.FailureSummary(),
					)
					// Continue with backpressure context
					prompt = FormatBackpressureForPrompt(bpResult)
					continue
				}
				we.logger.Info("backpressure passed", "phase", cfg.PhaseID)
			}

			// Extract artifact if present
			result.Artifact = extractArtifactFromJSON(turnResult.Content)
			return result, nil

		case PhaseStatusBlocked:
			return result, fmt.Errorf("phase blocked: %s", reason)

		case PhaseStatusContinue:
			// Continue to next iteration
			prompt = fmt.Sprintf("Continue working. Iteration %d/%d. %s",
				i+2, cfg.MaxIterations, reason)
		}
	}

	return result, fmt.Errorf("max iterations (%d) reached without completion", cfg.MaxIterations)
}

// loadPhasePrompt loads the prompt content for a phase template.
func (we *WorkflowExecutor) loadPhasePrompt(tmpl *db.PhaseTemplate) (string, error) {
	switch tmpl.PromptSource {
	case "embedded":
		// Load from embedded templates
		return we.loadEmbeddedPrompt(tmpl.PromptPath)

	case "db":
		// Use inline prompt content
		if tmpl.PromptContent == "" {
			return "", fmt.Errorf("phase %s has no prompt content", tmpl.ID)
		}
		return tmpl.PromptContent, nil

	case "file":
		// Load from file system
		return we.loadFilePrompt(tmpl.PromptPath)

	default:
		return "", fmt.Errorf("unknown prompt source: %s", tmpl.PromptSource)
	}
}

// loadEmbeddedPrompt loads a prompt from embedded templates.
func (we *WorkflowExecutor) loadEmbeddedPrompt(path string) (string, error) {
	// Import from templates package - fallback to file for now
	fullPath := filepath.Join(we.workingDir, "templates", path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		// Try embedded
		return "", fmt.Errorf("load embedded prompt %s: %w", path, err)
	}
	return string(content), nil
}

// loadFilePrompt loads a prompt from the file system.
func (we *WorkflowExecutor) loadFilePrompt(path string) (string, error) {
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(we.workingDir, ".orc", "prompts", path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("load prompt file %s: %w", fullPath, err)
	}
	return string(content), nil
}

// resolvePhaseModel determines which model to use for a phase.
func (we *WorkflowExecutor) resolvePhaseModel(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) string {
	// Phase override takes precedence
	if phase.ModelOverride != "" {
		return phase.ModelOverride
	}

	// Template override
	if tmpl.ModelOverride != "" {
		return tmpl.ModelOverride
	}

	// Default to sonnet
	return "sonnet"
}

// shouldUseThinking determines if extended thinking should be enabled.
func (we *WorkflowExecutor) shouldUseThinking(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) bool {
	// Phase override takes precedence
	if phase.ThinkingOverride != nil {
		return *phase.ThinkingOverride
	}

	// Template default
	if tmpl.ThinkingEnabled != nil {
		return *tmpl.ThinkingEnabled
	}

	// Decision phases default to thinking
	switch tmpl.ID {
	case "spec", "design", "review", "validate":
		return true
	}

	return false
}

// failRun marks a run as failed and syncs task status.
func (we *WorkflowExecutor) failRun(run *db.WorkflowRun, t *task.Task, err error) {
	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run failure", "error", saveErr)
	}

	// Sync task status to Failed
	if t != nil {
		t.Status = task.StatusFailed
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status failed", "task_id", t.ID, "error", saveErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
		// Trigger automation event for task failure
		we.triggerAutomationEvent(context.Background(), automation.EventTaskFailed, t, "")
	}
}

// interruptRun marks a run as cancelled (interrupted by context cancellation) and syncs task status.
func (we *WorkflowExecutor) interruptRun(run *db.WorkflowRun, t *task.Task, err error) {
	run.Status = string(workflow.RunStatusCancelled)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run interruption", "error", saveErr)
	}

	// Sync task status to Paused (can be resumed)
	if t != nil {
		t.Status = task.StatusPaused
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status paused", "task_id", t.ID, "error", saveErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
	}
}

// Helper functions

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

// setupWorktree creates or reuses an isolated worktree for the given task.
func (we *WorkflowExecutor) setupWorktree(t *task.Task) error {
	result, err := SetupWorktreeForTask(t, we.orcConfig, we.gitOps, we.backend)
	if err != nil {
		return fmt.Errorf("setup worktree: %w", err)
	}

	we.worktreePath = result.Path
	we.worktreeGit = we.gitOps.InWorktree(result.Path)

	logMsg := "created worktree"
	if result.Reused {
		logMsg = "reusing existing worktree"
	}
	we.logger.Info(logMsg, "task", t.ID, "path", result.Path, "target_branch", result.TargetBranch)

	// Generate per-worktree MCP config for isolated Playwright sessions
	if ShouldGenerateMCPConfig(t, we.orcConfig) {
		if err := GenerateWorktreeMCPConfig(result.Path, t.ID, t, we.orcConfig); err != nil {
			we.logger.Warn("failed to generate MCP config", "task", t.ID, "error", err)
			// Non-fatal: continue without MCP config
		} else {
			we.logger.Info("generated MCP config", "task", t.ID, "path", result.Path+"/.mcp.json")
		}
	}

	// Update the resolver to use worktree path
	we.resolver = variable.NewResolver(result.Path)

	return nil
}

// cleanupWorktree removes the worktree based on config and task status.
func (we *WorkflowExecutor) cleanupWorktree(t *task.Task) {
	if we.worktreePath == "" {
		return
	}

	// StatusResolved is treated like StatusCompleted for cleanup - both are terminal success states
	shouldCleanup := ((t.Status == task.StatusCompleted || t.Status == task.StatusResolved) && we.orcConfig.Worktree.CleanupOnComplete) ||
		(t.Status == task.StatusFailed && we.orcConfig.Worktree.CleanupOnFail)
	if !shouldCleanup {
		return
	}

	// Cleanup Playwright user data directory (task-specific browser profile)
	if err := CleanupPlaywrightUserData(t.ID); err != nil {
		we.logger.Warn("failed to cleanup playwright user data", "task", t.ID, "error", err)
	}

	// Use stored worktree path directly instead of reconstructing from task ID.
	// This handles initiative-prefixed worktrees correctly.
	if err := we.gitOps.CleanupWorktreeAtPath(we.worktreePath); err != nil {
		we.logger.Warn("failed to cleanup worktree", "path", we.worktreePath, "error", err)
	} else {
		we.logger.Info("cleaned up worktree", "task", t.ID, "path", we.worktreePath)
	}
}

// effectiveWorkingDir returns the working directory for phase execution.
// Returns worktree path if one was created, otherwise the original working dir.
func (we *WorkflowExecutor) effectiveWorkingDir() string {
	if we.worktreePath != "" {
		return we.worktreePath
	}
	return we.workingDir
}

// failSetup handles failures during setup phase (before any phase runs).
func (we *WorkflowExecutor) failSetup(run *db.WorkflowRun, t *task.Task, err error) {
	we.logger.Error("task setup failed", "task", t.ID, "error", err)

	// Clear execution tracking and set error in state
	if we.execState != nil {
		we.execState.ClearExecution()
		we.execState.Error = err.Error()
		if saveErr := we.backend.SaveState(we.execState); saveErr != nil {
			we.logger.Error("failed to save state on setup failure", "error", saveErr)
		}
	}

	// Update task status
	if t != nil {
		t.Status = task.StatusFailed
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task on setup failure", "error", saveErr)
		}
	}

	// Update run status
	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run on setup failure", "error", saveErr)
	}
}

// syncOnTaskStart syncs the task branch with target before execution starts.
// This catches conflicts from parallel tasks early.
func (we *WorkflowExecutor) syncOnTaskStart(ctx context.Context, t *task.Task) error {
	cfg := we.orcConfig.Completion
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Use worktree git if available
	gitOps := we.gitOps
	if we.worktreeGit != nil {
		gitOps = we.worktreeGit
	}

	if gitOps == nil {
		we.logger.Debug("skipping sync-on-start: git ops not available")
		return nil
	}

	// Skip sync if no remote is configured (e.g., E2E sandbox projects)
	if !gitOps.HasRemote("origin") {
		we.logger.Debug("skipping sync-on-start: no remote configured",
			"task", t.ID,
			"reason", "repository has no 'origin' remote")
		return nil
	}

	we.logger.Info("syncing with target before execution",
		"target", targetBranch,
		"task", t.ID,
		"reason", "catch stale worktree from parallel tasks")

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	target := "origin/" + targetBranch

	// Check if we're behind target
	ahead, behind, err := gitOps.GetCommitCounts(target)
	if err != nil {
		we.logger.Warn("could not determine commit counts, skipping sync", "error", err)
		return nil // Don't fail - this is best effort
	}

	if behind == 0 {
		we.logger.Info("branch already up-to-date with target",
			"target", targetBranch,
			"commits_ahead", ahead)
		return nil
	}

	we.logger.Info("task branch is behind target",
		"target", targetBranch,
		"commits_behind", behind,
		"commits_ahead", ahead)

	// Attempt rebase with conflict detection
	result, err := gitOps.RebaseWithConflictCheck(target)
	if err != nil {
		if errors.Is(err, git.ErrMergeConflict) {
			// Log conflict details
			we.logger.Warn("sync-on-start encountered conflicts",
				"task", t.ID,
				"conflict_files", result.ConflictFiles,
				"commits_behind", result.CommitsBehind)

			syncCfg := cfg.Sync
			conflictCount := len(result.ConflictFiles)

			// Check if we should fail on conflicts
			if syncCfg.MaxConflictFiles > 0 && conflictCount > syncCfg.MaxConflictFiles {
				return fmt.Errorf("sync conflict: %d conflict files exceeds max allowed (%d): %v",
					conflictCount, syncCfg.MaxConflictFiles, result.ConflictFiles)
			}

			if syncCfg.FailOnConflict {
				return fmt.Errorf("sync conflict: task branch has %d files in conflict with target",
					conflictCount)
			}

			// Continue execution - implement phase may resolve conflicts
			we.logger.Warn("continuing despite conflicts (fail_on_conflict: false)",
				"task", t.ID,
				"conflict_count", conflictCount)
			return nil
		}
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	we.logger.Info("synced task branch with target",
		"target", targetBranch,
		"commits_behind", result.CommitsBehind)

	return nil
}

// recordCostToGlobal logs cost and token usage to the global database for cross-project analytics.
// Failures are logged but don't interrupt execution.
func (we *WorkflowExecutor) recordCostToGlobal(t *task.Task, phaseID string, result PhaseResult, model string, duration time.Duration) {
	if we.globalDB == nil {
		return // Global DB not available, skip silently
	}

	projectPath := we.workingDir
	if projectPath == "" {
		projectPath = "unknown"
	}

	taskID := ""
	initiativeID := ""
	if t != nil {
		taskID = t.ID
		initiativeID = t.InitiativeID
	}

	entry := db.CostEntry{
		ProjectID:    projectPath,
		TaskID:       taskID,
		Phase:        phaseID,
		Model:        db.DetectModel(model),
		Iteration:    result.Iterations,
		CostUSD:      result.CostUSD,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		TotalTokens:  result.InputTokens + result.OutputTokens,
		InitiativeID: initiativeID,
		DurationMs:   duration.Milliseconds(),
		Timestamp:    time.Now(),
	}

	if err := we.globalDB.RecordCostExtended(entry); err != nil {
		we.logger.Warn("failed to record cost to global database",
			"task", taskID,
			"phase", phaseID,
			"error", err,
		)
	} else {
		we.logger.Debug("recorded cost to global database",
			"task", taskID,
			"phase", phaseID,
			"cost_usd", result.CostUSD,
			"model", model,
		)
	}
}

// syncTranscripts syncs Claude JSONL transcripts to the database.
func (we *WorkflowExecutor) syncTranscripts(ctx context.Context, sessionID, taskID, phaseID string) {
	if we.backend == nil || sessionID == "" {
		return
	}

	// Determine JSONL path from session ID
	// Claude Code stores sessions in ~/.claude/projects/{project-hash}/
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return
	}

	// Session files are stored under the project path hash
	projectHash := hashString(we.workingDir)
	jsonlPath := filepath.Join(homeDir, ".claude", "projects", projectHash, sessionID+".jsonl")

	// Check if file exists
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		we.logger.Debug("no JSONL file found for session", "session", sessionID, "path", jsonlPath)
		return
	}

	syncer := NewJSONLSyncer(we.backend, we.logger)
	if err := syncer.SyncFromFile(ctx, jsonlPath, SyncOptions{
		TaskID: taskID,
		Phase:  phaseID,
		Append: true, // Always append, dedup by UUID
	}); err != nil {
		we.logger.Warn("failed to sync transcripts",
			"session", sessionID,
			"task", taskID,
			"phase", phaseID,
			"error", err,
		)
	}
}

// hashString creates a simple hash of a string for directory naming.
func hashString(s string) string {
	// Use a simple approach matching Claude Code's behavior
	var hash uint32
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return fmt.Sprintf("%x", hash)
}

// initBackpressure initializes the backpressure runner if configured.
func (we *WorkflowExecutor) initBackpressure() {
	if we.orcConfig == nil || !we.orcConfig.Validation.Enabled {
		return
	}

	workDir := we.effectiveWorkingDir()
	we.backpressure = NewBackpressureRunner(
		workDir,
		&we.orcConfig.Validation,
		&we.orcConfig.Testing,
		we.logger,
	)
}

// runCompletion executes the completion action (sync, PR/merge) for a task.
func (we *WorkflowExecutor) runCompletion(ctx context.Context, t *task.Task) error {
	if we.orcConfig == nil {
		return nil
	}

	// Resolve action based on task weight
	action := we.orcConfig.ResolveCompletionAction(string(t.Weight))
	if action == "" || action == "none" {
		we.logger.Info("skipping completion action", "weight", t.Weight, "action", action)
		return nil
	}

	// Get effective git operations (worktree or main)
	gitOps := we.gitOps
	if we.worktreeGit != nil {
		gitOps = we.worktreeGit
	}

	if gitOps == nil {
		return fmt.Errorf("git operations not available")
	}

	// Skip if no remote is configured
	if !gitOps.HasRemote("origin") {
		we.logger.Debug("skipping completion: no remote configured")
		return nil
	}

	// Sync with target branch before completion
	targetBranch := we.orcConfig.Completion.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	we.logger.Info("syncing with target branch before completion",
		"target", targetBranch,
		"action", action)

	// Fetch latest
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	target := "origin/" + targetBranch

	// Check divergence
	ahead, behind, err := gitOps.GetCommitCounts(target)
	if err != nil {
		we.logger.Warn("could not determine divergence", "error", err)
	} else if behind > 0 {
		// Attempt rebase
		result, err := gitOps.RebaseWithConflictCheck(target)
		if err != nil {
			if errors.Is(err, git.ErrMergeConflict) {
				we.logger.Warn("sync encountered conflicts",
					"task", t.ID,
					"conflict_files", result.ConflictFiles)
				if we.orcConfig.Completion.Sync.FailOnConflict {
					return fmt.Errorf("%w: %d conflict files", ErrSyncConflict, len(result.ConflictFiles))
				}
			} else {
				return fmt.Errorf("rebase failed: %w", err)
			}
		} else {
			we.logger.Info("synced with target branch",
				"commits_behind", behind,
				"commits_ahead", ahead)
		}
	}

	// Execute completion action
	switch action {
	case "merge":
		return we.directMerge(ctx, t, gitOps, targetBranch)
	case "pr":
		return we.createPR(ctx, t, gitOps, targetBranch)
	default:
		we.logger.Warn("unknown completion action", "action", action)
		return nil
	}
}

// directMerge merges the task branch directly into target.
func (we *WorkflowExecutor) directMerge(ctx context.Context, t *task.Task, gitOps *git.Git, targetBranch string) error {
	we.logger.Info("direct merge to target branch", "target", targetBranch)

	// Push task branch first
	if err := gitOps.Push("origin", t.Branch, false); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Switch to target and merge
	if err := gitOps.CheckoutSafe(targetBranch); err != nil {
		return fmt.Errorf("checkout target: %w", err)
	}

	// Fetch and rebase to get latest changes
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed", "error", err)
	}
	if err := gitOps.Rebase("origin/" + targetBranch); err != nil {
		we.logger.Warn("rebase failed", "error", err)
	}

	if err := gitOps.Merge(t.Branch, false); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	if err := gitOps.Push("origin", targetBranch, false); err != nil {
		return fmt.Errorf("push target: %w", err)
	}

	// Update task with merge info
	t.Status = task.StatusResolved
	now := time.Now()
	t.CompletedAt = &now
	we.backend.SaveTask(t)

	we.logger.Info("direct merge completed", "task", t.ID, "target", targetBranch)
	return nil
}

// createPR creates a pull request for the task branch.
func (we *WorkflowExecutor) createPR(ctx context.Context, t *task.Task, gitOps *git.Git, targetBranch string) error {
	// Check if PR already exists
	if t.PR != nil && t.PR.URL != "" {
		we.logger.Info("PR already exists", "url", t.PR.URL)
		return nil
	}

	we.logger.Info("creating PR", "branch", t.Branch, "target", targetBranch)

	// Push task branch
	if err := gitOps.Push("origin", t.Branch, true); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Build PR body
	body := fmt.Sprintf("## Task: %s\n\n%s\n\n---\nCreated by orc workflow execution.",
		t.Title, t.Description)

	// Create PR via gh cli
	prTitle := fmt.Sprintf("[orc] %s: %s", t.ID, t.Title)
	prURL, err := we.runGHCreatePR(ctx, prTitle, body, targetBranch)
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	// Update task with PR info
	t.PR = &task.PRInfo{
		URL:    prURL,
		Status: task.PRStatusPendingReview,
	}
	we.backend.SaveTask(t)

	we.logger.Info("PR created", "url", prURL)
	return nil
}

// runGHCreatePR creates a PR using the gh CLI.
func (we *WorkflowExecutor) runGHCreatePR(ctx context.Context, title, body, targetBranch string) (string, error) {
	workDir := we.effectiveWorkingDir()

	args := []string{
		"pr", "create",
		"--title", title,
		"--body", body,
		"--base", targetBranch,
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w: %s", err, string(out))
	}

	// Extract URL from output (gh pr create outputs the PR URL)
	prURL := strings.TrimSpace(string(out))
	return prURL, nil
}

// GateEvaluationResult contains the result of gate evaluation.
type GateEvaluationResult struct {
	Approved   bool
	Pending    bool
	Reason     string
	RetryPhase string // If not approved and has retry target
}

// evaluatePhaseGate evaluates the gate for a completed phase.
func (we *WorkflowExecutor) evaluatePhaseGate(ctx context.Context, tmpl *db.PhaseTemplate, phase *db.WorkflowPhase, output string, t *task.Task) (*GateEvaluationResult, error) {
	result := &GateEvaluationResult{}

	// Determine effective gate type
	gateType := tmpl.GateType
	if phase.GateTypeOverride != "" {
		gateType = phase.GateTypeOverride
	}

	// If no gate or auto with auto-approve, just approve
	if gateType == "" || gateType == "auto" {
		if we.orcConfig != nil && we.orcConfig.Gates.AutoApproveOnSuccess {
			result.Approved = true
			result.Reason = "auto-approved on success"
			return result, nil
		}
	}

	// Create gate struct for evaluator
	g := &gate.Gate{
		Type: gate.GateType(gateType),
	}

	// Evaluate
	decision, err := we.gateEvaluator.Evaluate(ctx, g, output)
	if err != nil {
		return nil, fmt.Errorf("gate evaluation: %w", err)
	}

	result.Approved = decision.Approved
	result.Pending = decision.Pending
	result.Reason = decision.Reason

	// If not approved, check for retry target
	if !result.Approved && !result.Pending {
		if tmpl.RetryFromPhase != "" {
			result.RetryPhase = tmpl.RetryFromPhase
		} else if we.orcConfig != nil {
			// Fall back to config-based retry map
			result.RetryPhase = we.orcConfig.ShouldRetryFrom(tmpl.ID)
		}
	}

	return result, nil
}

// publishTaskUpdated publishes a task_updated event for real-time UI updates.
// Uses the EventTaskUpdated type which the frontend listens for.
func (we *WorkflowExecutor) publishTaskUpdated(t *task.Task) {
	if we.publisher == nil || t == nil {
		return
	}
	we.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
}

// runResourceAnalysis runs the resource tracker analysis after task completion.
// Called via defer in Run() to run regardless of success or failure.
func (we *WorkflowExecutor) runResourceAnalysis() {
	if we.resourceTracker == nil {
		return
	}

	if err := we.resourceTracker.SnapshotAfter(); err != nil {
		we.logger.Warn("failed to take after snapshot", "error", err)
		return
	}

	orphans := we.resourceTracker.DetectOrphans()
	if len(orphans) > 0 {
		we.logger.Warn("detected potential orphaned processes",
			"count", len(orphans),
			"processes", formatOrphanedProcesses(orphans),
		)
	}
}

// triggerAutomationEvent sends an event to the automation service if configured.
func (we *WorkflowExecutor) triggerAutomationEvent(ctx context.Context, eventType string, t *task.Task, phase string) {
	if we.automationSvc == nil || t == nil {
		return
	}

	event := &automation.Event{
		Type:     eventType,
		TaskID:   t.ID,
		Weight:   string(t.Weight),
		Category: string(t.Category),
		Phase:    phase,
	}

	if err := we.automationSvc.HandleEvent(ctx, event); err != nil {
		we.logger.Warn("automation event handling failed",
			"event", eventType,
			"task", t.ID,
			"error", err)
	}
}

// formatOrphanedProcesses formats orphaned process info for logging.
func formatOrphanedProcesses(processes []ProcessInfo) string {
	if len(processes) == 0 {
		return ""
	}
	var parts []string
	for _, p := range processes {
		parts = append(parts, fmt.Sprintf("%d:%s", p.PID, p.Command))
	}
	return strings.Join(parts, ", ")
}
