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
	"strings"
	"syscall"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/tokenpool"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/internal/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	Category orcv1.TaskCategory

	// Variables are additional variables to inject.
	Variables map[string]string

	// Stream enables real-time output streaming.
	Stream bool

	// IsResume indicates this is resuming an interrupted/paused task.
	// Set this when the original task status was paused/failed/blocked before TryClaimTaskExecution changed it to running.
	IsResume bool
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
	publisher          *events.PublishHelper
	tokenPool          *tokenpool.Pool          // For automatic account switching on rate limits
	automationSvc      *automation.Service      // For automation event triggers
	sessionBroadcaster *SessionBroadcaster      // For real-time session metrics
	resourceTracker    *ResourceTracker         // For orphan process detection

	// Per-run state (set during Run)
	worktreePath string            // Path to worktree (if created)
	worktreeGit  *git.Git          // Git ops scoped to worktree
	task         *orcv1.Task       // Task being executed (for task-based contexts)
	heartbeat    *HeartbeatRunner
	fileWatcher  *FileWatcher
	isResuming   bool              // True if resuming a paused/failed/blocked task

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
		we.publisher = events.NewPublishHelper(p)
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
		gateEvaluator: gate.New(),
		workingDir:    workingDir,
		logger:        slog.Default(),
		claudePath:    "claude",
		publisher:     events.NewPublishHelper(nil), // Initialize with nil-safe wrapper
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

	// Sort phases by dependency graph (DependsOn) with Sequence as tiebreaker
	phases, err = topologicalSort(phases)
	if err != nil {
		return nil, fmt.Errorf("resolve phase execution order: %w", err)
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

	// Handle task creation/loading based on context type
	var t *orcv1.Task
	switch opts.ContextType {
	case ContextDefault:
		t, err = we.createTaskForRunProto(opts, workflowID)
		if err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
		run.TaskID = &t.Id

	case ContextTask:
		// Load existing task
		t, err = we.backend.LoadTask(opts.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %s: %w", opts.TaskID, err)
		}
		run.TaskID = &t.Id

		// Set workflow_id on task if not already set (enables `orc show` to display correct phases)
		if t.WorkflowId == nil || *t.WorkflowId != workflowID {
			t.WorkflowId = &workflowID
			if err := we.backend.SaveTask(t); err != nil {
				we.logger.Error("failed to save workflow_id to task", "task_id", t.Id, "error", err)
			}
		}

	}

	// Initialize task and execution state for task-based contexts
	if t != nil {
		// Store task reference - execution state is in t.Execution
		we.task = t

		// Set execution info (PID, hostname, heartbeat) on task
		hostname, _ := os.Hostname()
		if err := we.backend.SetTaskExecutor(t.Id, os.Getpid(), hostname); err != nil {
			we.logger.Error("failed to set task executor", "task_id", t.Id, "error", err)
		}

		// Start heartbeat runner for orphan detection
		we.heartbeat = NewHeartbeatRunner(we.backend, t.Id, we.logger)
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
			taskID := ""
			if t != nil {
				taskID = t.Id
			}
			we.logger.Info("SIGUSR1 received, initiating graceful pause", "task", taskID)
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
			we.fileWatcher = NewFileWatcher(detector, we.publisher, t.Id, we.worktreePath, baseRef, we.logger)
			we.fileWatcher.Start(execCtx)
			defer we.fileWatcher.Stop()
		}
	}

	// Sync with target branch before execution starts
	if t != nil && we.orcConfig.ShouldSyncOnStart() && we.orcConfig.ShouldSyncForWeight(t.Weight.String()) {
		if err := we.syncOnTaskStart(execCtx, t); err != nil {
			we.logger.Error("sync-on-start failed", "task", t.Id, "error", err)

			// Unconditionally cleanup worktree and branch on sync failure.
			// Since no phases ran, there's no user work to preserve — cleanup is always correct.
			// This bypasses the config-gated deferred cleanup.
			we.cleanupSyncFailure(t)

			we.failSetup(run, t, err)
			return nil, fmt.Errorf("sync on start: %w", err)
		}
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
	vars, err := we.resolver.ResolveAll(execCtx, varDefs, rctx)
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
	// Note: Use opts.IsResume since TryClaimTaskExecution already changed status to running
	if t != nil {
		we.isResuming = opts.IsResume
		if we.isResuming {
			we.logger.Info("detected resume from interrupted state", "task", t.Id)
		}
		t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
		if err := we.backend.SaveTask(t); err != nil {
			we.logger.Error("failed to save task status running", "task_id", t.Id, "error", err)
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
		if we.task != nil {
			if ps, ok := we.task.Execution.Phases[phase.PhaseTemplateID]; ok {
				if ps.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
					we.logger.Info("skipping completed phase", "phase", phase.PhaseTemplateID)
					// Load content from completed phase for variable chaining
					// Phase outputs are stored in unified phase_outputs table keyed by run ID
					if output, err := we.backend.GetPhaseOutput(run.ID, phase.PhaseTemplateID); err == nil && output != nil {
						applyPhaseContentToVars(vars, rctx, phase.PhaseTemplateID, output.Content, output.OutputVarName)
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

		// Check for context cancellation (from SIGUSR1 pause signal)
		if execCtx.Err() != nil {
			we.interruptRun(run, t, phase.PhaseTemplateID, execCtx.Err())
			return result, execCtx.Err()
		}

		// Create run phase record
		runPhase := &db.WorkflowRunPhase{
			WorkflowRunID:   runID,
			PhaseTemplateID: phase.PhaseTemplateID,
			Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
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
		we.enrichContextForPhase(rctx, tmpl.ID, t)

		// Re-resolve variables with updated context
		vars, err = we.resolver.ResolveAll(execCtx, varDefs, rctx)
		if err != nil {
			we.failRun(run, t, fmt.Errorf("resolve variables for phase %s: %w", tmpl.ID, err))
			return result, err
		}

		// Execute the phase with timeout support (PhaseMax config)
		phaseResult, err := we.executePhaseWithTimeout(execCtx, tmpl, phase, vars, rctx, run, runPhase, t)
		result.PhaseResults = append(result.PhaseResults, phaseResult)

		if err != nil {
			// Check for blocked status - proceed to gate evaluation, not failure
			var blockedErr *PhaseBlockedError
			if errors.As(err, &blockedErr) {
				we.logger.Info("phase blocked, proceeding to gate evaluation",
					"phase", tmpl.ID,
					"reason", blockedErr.Reason,
				)

				// Mark result as blocked for gate evaluation
				// Note: Review findings are stored in RetryContext.FailureOutput when
				// SetRetryContextProto is called by gate rejection handler below
				phaseResult.BlockedReason = blockedErr.Reason
				// Fall through to gate evaluation (don't return)
			} else {
				// Check if this was triggered by pause signal (execCtx cancelled)
				// Note: The error may be "signal: killed" from subprocess, not context.Canceled
				if execCtx.Err() != nil {
					we.interruptRun(run, t, phase.PhaseTemplateID, execCtx.Err())
					return result, err
				}
				we.failRun(run, t, err)
				return result, err
			}
		}

		// Update variables with phase output content
		if phaseResult.Content != "" {
			applyPhaseContentToVars(vars, rctx, phaseResult.PhaseID, phaseResult.Content, tmpl.OutputVarName)
		}

		// Check for loop configuration and handle iterative loops
		if phase.LoopConfig != "" {
			loopCfg, loopErr := db.ParseLoopConfig(phase.LoopConfig)
			if loopErr != nil {
				we.logger.Warn("invalid loop config", "phase", tmpl.ID, "error", loopErr)
			} else if loopCfg != nil {
				// Initialize loop iteration tracking in resolution context
				if rctx.QAIteration == 0 {
					rctx.QAIteration = 1
				}
				if rctx.QAMaxIterations == 0 && loopCfg.MaxIterations > 0 {
					rctx.QAMaxIterations = loopCfg.MaxIterations
				}

				// Evaluate loop condition based on prior phase output
				shouldLoop := we.evaluateLoopCondition(loopCfg.Condition, loopCfg.LoopToPhase, vars, rctx)

				if shouldLoop && rctx.QAIteration < rctx.QAMaxIterations {
					we.logger.Info("loop condition met, looping back",
						"phase", tmpl.ID,
						"loop_to", loopCfg.LoopToPhase,
						"iteration", rctx.QAIteration,
						"max_iterations", rctx.QAMaxIterations,
					)

					// Find loop target phase index
					loopIdx := -1
					for j := 0; j < i; j++ {
						if phases[j].PhaseTemplateID == loopCfg.LoopToPhase {
							loopIdx = j
							break
						}
					}

					if loopIdx >= 0 {
						rctx.QAIteration++
						// Store previous findings for verification in next test iteration
						if findingsContent, ok := rctx.PriorOutputs[loopCfg.LoopToPhase]; ok {
							rctx.PreviousFindings = findingsContent
						}

						// Reset phase completion status for loop-back phases
						if we.task != nil {
							for k := loopIdx; k <= i; k++ {
								phaseID := phases[k].PhaseTemplateID
								if ps, exists := we.task.Execution.Phases[phaseID]; exists {
									ps.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
									ps.Iterations++
								}
							}
							if err := we.backend.SaveTask(we.task); err != nil {
								we.logger.Warn("failed to save loop state", "error", err)
							}
						}

						// Jump back to loop target phase
						i = loopIdx - 1 // Will be incremented by loop
						continue
					}
				} else if shouldLoop {
					we.logger.Info("max loop iterations reached",
						"phase", tmpl.ID,
						"iteration", rctx.QAIteration,
						"max_iterations", rctx.QAMaxIterations,
					)
				}
			}
		}

		// Evaluate phase gate
		// For blocked phases, bypass gate evaluation and force rejection
		gateResult, gateErr := we.evaluatePhaseGate(ctx, tmpl, phase, phaseResult.Content, t)
		if gateErr != nil {
			we.logger.Warn("gate evaluation failed", "phase", tmpl.ID, "error", gateErr)
			// Continue on gate error - don't block automation
		}

		// Handle blocked phases: force gate rejection to trigger retry
		if phaseResult.BlockedReason != "" {
			we.logger.Info("phase blocked, forcing gate rejection for retry",
				"phase", tmpl.ID,
				"reason", phaseResult.BlockedReason,
			)
			// Create or override gate result to trigger retry
			if gateResult == nil {
				gateResult = &GateEvaluationResult{}
			}
			gateResult.Approved = false
			gateResult.Reason = phaseResult.BlockedReason
			// Check retry map for this phase
			if tmpl.RetryFromPhase != "" {
				gateResult.RetryPhase = tmpl.RetryFromPhase
			} else if we.orcConfig != nil {
				gateResult.RetryPhase = we.orcConfig.ShouldRetryFrom(tmpl.ID)
			}
		}

		if gateResult != nil {
			// Handle gate decision
			if gateResult.Pending {
				// Task is blocked waiting for human decision
				we.logger.Info("gate decision pending", "phase", tmpl.ID)
				if t != nil {
					t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
					if err := we.backend.SaveTask(t); err != nil {
						we.logger.Error("failed to save blocked task", "error", err)
					}
				}
				if we.task != nil {
					errMsg := fmt.Sprintf("blocked at gate: %s (phase %s)", gateResult.Reason, tmpl.ID)
					task.SetErrorProto(we.task.Execution, errMsg)
					if err := we.backend.SaveTask(we.task); err != nil {
						we.logger.Warn("failed to save blocked state", "error", err)
					}
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
							task.RecordPhaseRetryProto(t, tmpl.ID)
							if tmpl.ID == "review" {
								task.RecordReviewRejectionProto(t)
							}
							if err := we.backend.SaveTask(t); err != nil {
								we.logger.Warn("failed to save task after retry", "task", t.Id, "error", err)
							}
						}

						// Save retry context
						if we.task != nil {
							reason := fmt.Sprintf("Gate rejected for phase %s: %s", tmpl.ID, gateResult.Reason)
							task.SetRetryContextProto(we.task.Execution, tmpl.ID, gateResult.RetryPhase, reason, phaseResult.Content, int32(retryCounts[tmpl.ID]))
							if err := we.backend.SaveTask(we.task); err != nil {
								we.logger.Warn("failed to save retry state", "error", err)
							}
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
			if we.task != nil {
				task.RecordGateDecisionProto(we.task.Execution, tmpl.ID, tmpl.GateType, gateResult.Approved, gateResult.Reason)

				// Clear retry context after successful review round 2
				// This prevents stale context from affecting future runs
				if gateResult.Approved && tmpl.ID == "review" && rctx.ReviewRound > 1 {
					we.task.Execution.RetryContext = nil
					we.logger.Info("cleared retry context after successful review round 2",
						"task", we.task.Id,
						"round", rctx.ReviewRound,
					)
				}

				if err := we.backend.SaveTask(we.task); err != nil {
					we.logger.Warn("failed to save gate decision state", "error", err)
				}
			}
		}
	}

	// Run completion action (sync, PR/merge) for task-based contexts
	if t != nil && we.gitOps != nil {
		completionErr := we.runCompletion(execCtx, t)
		if completionErr != nil {
			// Check if it's a conflict or merge error
			if errors.Is(completionErr, ErrSyncConflict) || errors.Is(completionErr, ErrMergeFailed) {
				we.logger.Error("completion failed",
					"task", t.Id,
					"error", completionErr)

				// Mark task as blocked, not completed
				t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
				task.EnsureMetadataProto(t)
				if errors.Is(completionErr, ErrSyncConflict) {
					t.Metadata["blocked_reason"] = "sync_conflict"
				} else {
					t.Metadata["blocked_reason"] = "merge_failed"
				}
				t.Metadata["blocked_error"] = completionErr.Error()

				// Clear executor claim
				if err := we.backend.ClearTaskExecutor(t.Id); err != nil {
					we.logger.Warn("failed to clear task executor", "error", err)
				}
				if err := we.backend.SaveTask(t); err != nil {
					we.logger.Warn("failed to save blocked task", "task", t.Id, "error", err)
				}

				// Run is completed but task is blocked
				run.Status = string(workflow.RunStatusCompleted)
				run.CompletedAt = timePtr(time.Now())
				if err := we.backend.SaveWorkflowRun(run); err != nil {
					we.logger.Warn("failed to save workflow run", "run", run.ID, "error", err)
				}

				result.CompletedAt = run.CompletedAt
				result.Success = false
				result.Error = completionErr.Error()
				return result, fmt.Errorf("%w: %v", ErrTaskBlocked, completionErr)
			}

			// Other completion errors - fail the task properly
			we.logger.Error("completion action failed", "task", t.Id, "error", completionErr)
			t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
			task.EnsureMetadataProto(t)
			t.Metadata["failed_reason"] = "completion_failed"
			t.Metadata["failed_error"] = completionErr.Error()
			if err := we.backend.SaveTask(t); err != nil {
				we.logger.Warn("failed to save failed task", "task", t.Id, "error", err)
			}
			result.Success = false
			result.Error = completionErr.Error()
			return result, fmt.Errorf("completion failed: %w", completionErr)
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
		t.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		t.CompletedAt = timestamppb.Now()
		if err := we.backend.SaveTask(t); err != nil {
			we.logger.Error("failed to save task status completed", "task_id", t.Id, "error", err)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
		// Trigger automation event for task completion
		we.triggerAutomationEvent(execCtx, automation.EventTaskCompleted, t, "")

		// Check if task's initiative should be auto-completed (for no-branch initiatives)
		initiativeID := task.GetInitiativeIDProto(t)
		if initiativeID != "" {
			completer := NewInitiativeCompleter(we.gitOps, nil, we.backend, we.orcConfig, we.logger, we.workingDir)
			if err := completer.CheckAndCompleteInitiativeNoBranch(execCtx, initiativeID); err != nil {
				// Best-effort: log error but don't fail task completion
				we.logger.Warn("failed to check initiative completion",
					"task", t.Id,
					"initiative", initiativeID,
					"error", err)
			}
		}
	}

	// Clear execution state and release executor claim
	if t != nil {
		if err := we.backend.ClearTaskExecutor(t.Id); err != nil {
			we.logger.Warn("failed to clear task executor", "error", err)
		}
	}
	// Note: Task completion (t.CompletedAt, t.Status = StatusCompleted) is handled
	// by runCompletion or other completion paths, not here. This is just for
	// cleaning up execution state if needed.

	// Populate result fields from run
	if t != nil {
		result.TaskID = t.Id
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
	Content             string
	Error               string
	BlockedReason       string // Set when phase outputs blocked status (for gate evaluation)
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	CostUSD             float64
}

// Helper functions

// applyPhaseContentToVars updates variable maps with phase output content.
// Called both when resuming from completed phases and after phase completion.
// For phases with structured JSON output, this formats the content appropriately
// for injection into subsequent phase prompts.
//
// outputVarName is the variable name from the phase template (e.g., "SPEC_CONTENT").
// If empty, falls back to inferOutputVarName() for backward compatibility.
func applyPhaseContentToVars(vars map[string]string, rctx *variable.ResolutionContext, phaseID, content, outputVarName string) {
	// Store raw output for OUTPUT_* variable (used by loop condition evaluation)
	vars["OUTPUT_"+phaseID] = content

	// Determine the output variable name
	varName := outputVarName
	if varName == "" {
		varName = inferOutputVarName(phaseID)
	}

	// Set the named variable (e.g., SPEC_CONTENT, TDD_TESTS_CONTENT)
	vars[varName] = content

	// Special case: QA findings need additional formatting for the fix phase
	if phaseID == "qa_e2e_test" {
		result, err := ParseQAE2ETestResult(content)
		if err == nil {
			vars["QA_FINDINGS"] = result.FormatFindingsForFix()
		} else {
			// Fallback to raw if parse fails
			vars["QA_FINDINGS"] = content
		}
		// Persist to rctx so QA_FINDINGS survives ResolveAll() on next loop iteration
		rctx.QAFindings = vars["QA_FINDINGS"]
	}

	// Store raw content in priorOutputs for loop condition evaluation
	if rctx.PriorOutputs != nil {
		rctx.PriorOutputs[phaseID] = content
	}
}

// evaluateLoopCondition checks if a loop condition is met based on phase output.
// Supported conditions:
// - "has_findings": checks if the target phase output contains any findings
// - "not_empty": checks if the target phase output is not empty
// - "status_needs_fix": checks if the output status indicates fixes needed
func (we *WorkflowExecutor) evaluateLoopCondition(condition, targetPhase string, vars map[string]string, rctx *variable.ResolutionContext) bool {
	// Get the target phase output
	output := ""
	if o, ok := rctx.PriorOutputs[targetPhase]; ok {
		output = o
	} else if o, ok := vars["OUTPUT_"+targetPhase]; ok {
		output = o
	}

	if output == "" {
		return false
	}

	switch condition {
	case "has_findings":
		// Parse the QA findings and check if there are any
		var result QAE2ETestResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			we.logger.Warn("failed to parse QA findings for loop condition", "error", err)
			return false
		}
		hasFindingsToFix := len(result.Findings) > 0
		we.logger.Debug("loop condition has_findings evaluated",
			"findings_count", len(result.Findings),
			"should_loop", hasFindingsToFix,
		)
		return hasFindingsToFix

	case "not_empty":
		return output != "" && output != "{}" && output != "[]"

	case "status_needs_fix":
		// Check if the output JSON has a status indicating fixes needed
		var statusCheck struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(output), &statusCheck); err != nil {
			return false
		}
		return statusCheck.Status == "needs_fix" || statusCheck.Status == "findings"

	default:
		we.logger.Warn("unknown loop condition", "condition", condition)
		return false
	}
}

// createTaskForRunProto creates a proto task for a default context run.
func (we *WorkflowExecutor) createTaskForRunProto(opts WorkflowRunOptions, workflowID string) (*orcv1.Task, error) {
	taskID, err := we.backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("get next task ID: %w", err)
	}

	t := task.NewProtoTask(taskID, truncateTitle(opts.Prompt))
	task.SetDescriptionProto(t, opts.Prompt)

	// Set workflow_id so task knows what workflow it's running
	t.WorkflowId = &workflowID

	// Set category from options or default to feature
	if opts.Category != orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
		t.Category = opts.Category
	}

	if err := we.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}

	return t, nil
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

// extractPhaseOutput extracts the phase output content from JSON.
// Checks for "content" field, and if it doesn't exist, returns the entire JSON as the output.
func extractPhaseOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	// Verify it's valid JSON
	var generic map[string]any
	if err := json.Unmarshal([]byte(output), &generic); err != nil {
		return ""
	}

	// Check for "content" field (content-producing phases)
	if content, ok := generic["content"].(string); ok && content != "" {
		return content
	}

	// No content field - the entire JSON IS the output
	// This handles qa_e2e_test (findings), qa_e2e_fix (fixes_applied), review, etc.
	return output
}

