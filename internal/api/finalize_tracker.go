package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// FinalizeStatus represents the status of a finalize operation.
type FinalizeStatus string

const (
	FinalizeStatusPending   FinalizeStatus = "pending"
	FinalizeStatusRunning   FinalizeStatus = "running"
	FinalizeStatusCompleted FinalizeStatus = "completed"
	FinalizeStatusFailed    FinalizeStatus = "failed"
)

// FinalizeState tracks the status of an async finalize operation.
type FinalizeState struct {
	mu sync.RWMutex

	TaskID    string         `json:"task_id"`
	Status    FinalizeStatus `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	// Progress tracking
	Step        string `json:"step,omitempty"`
	Progress    string `json:"progress,omitempty"`
	StepPercent int    `json:"step_percent,omitempty"`

	// Result (populated on completion)
	Result *FinalizeResult `json:"result,omitempty"`

	// Error (populated on failure)
	Error string `json:"error,omitempty"`
}

// FinalizeResult contains the outcome of the finalize operation.
type FinalizeResult struct {
	Synced            bool     `json:"synced"`
	ConflictsResolved int      `json:"conflicts_resolved"`
	ConflictFiles     []string `json:"conflict_files,omitempty"`
	TestsPassed       bool     `json:"tests_passed"`
	RiskLevel         string   `json:"risk_level"`
	FilesChanged      int      `json:"files_changed"`
	LinesChanged      int      `json:"lines_changed"`
	NeedsReview       bool     `json:"needs_review"`
	CommitSHA         string   `json:"commit_sha,omitempty"`
	TargetBranch      string   `json:"target_branch"`

	// CI and merge results (populated after finalize sync)
	CIPassed    bool   `json:"ci_passed,omitempty"`
	CIDetails   string `json:"ci_details,omitempty"`
	Merged      bool   `json:"merged,omitempty"`
	MergeCommit string `json:"merge_commit,omitempty"`
	CITimedOut  bool   `json:"ci_timed_out,omitempty"`
	MergeError  string `json:"merge_error,omitempty"`
}

// FinalizeRequest is the request body for triggering finalize.
type FinalizeRequest struct {
	Force        bool `json:"force,omitempty"`
	GateOverride bool `json:"gate_override,omitempty"`
}

// finalizeTracker tracks ongoing finalize operations.
type finalizeTracker struct {
	mu      sync.RWMutex
	states  map[string]*FinalizeState
	cancels map[string]context.CancelFunc
}

var finTracker = &finalizeTracker{
	states:  make(map[string]*FinalizeState),
	cancels: make(map[string]context.CancelFunc),
}

// setCancel stores the cancel function for a task's finalize goroutine.
func (ft *finalizeTracker) setCancel(taskID string, cancel context.CancelFunc) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.cancels[taskID] = cancel
}

// cancel cancels the finalize operation for a specific task.
func (ft *finalizeTracker) cancel(taskID string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if cancel, ok := ft.cancels[taskID]; ok {
		cancel()
		delete(ft.cancels, taskID)
	}
}

// tryStart attempts to atomically start a finalize operation for a task.
func (ft *finalizeTracker) tryStart(taskID string, newState *FinalizeState) (*FinalizeState, bool) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if existing := ft.states[taskID]; existing != nil {
		existing.mu.RLock()
		status := existing.Status
		existing.mu.RUnlock()

		if status == FinalizeStatusRunning || status == FinalizeStatusPending {
			return existing, false
		}
	}

	ft.states[taskID] = newState
	return nil, true
}

// cancelAll cancels all running finalize operations.
func (ft *finalizeTracker) cancelAll() {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for taskID, cancel := range ft.cancels {
		cancel()
		delete(ft.cancels, taskID)
	}
}

// cleanupStale removes completed/failed entries older than the retention period.
func (ft *finalizeTracker) cleanupStale(retention time.Duration) int {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	now := time.Now()
	removed := 0

	for taskID, state := range ft.states {
		state.mu.RLock()
		status := state.Status
		updatedAt := state.UpdatedAt
		state.mu.RUnlock()

		if status != FinalizeStatusCompleted && status != FinalizeStatusFailed {
			continue
		}

		if now.Sub(updatedAt) > retention {
			delete(ft.states, taskID)
			removed++
		}
	}

	return removed
}

// startCleanup starts a background goroutine that periodically cleans up stale entries.
func (ft *finalizeTracker) startCleanup(ctx context.Context, interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ft.cleanupStale(retention)
			}
		}
	}()
}

// EventFinalize is the event type for finalize progress.
const EventFinalize events.EventType = "finalize"

// TriggerFinalizeOnApproval is called when a PR is approved and auto-trigger is enabled.
func (s *Server) TriggerFinalizeOnApproval(taskID string, projectID string) (bool, error) {
	if !s.orcConfig.ShouldAutoTriggerFinalizeOnApproval() {
		s.logger.Debug("auto-trigger on approval disabled", "task", taskID)
		return false, nil
	}

	backend := s.backend
	workDir := s.workDir
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			return false, fmt.Errorf("resolve project backend: %w", err)
		}
		workDir, err = s.projectCache.GetProjectPath(projectID)
		if err != nil {
			return false, fmt.Errorf("resolve project path: %w", err)
		}
	}

	t, err := backend.LoadTask(taskID)
	if err != nil {
		return false, fmt.Errorf("load task: %w", err)
	}

	if !s.orcConfig.ShouldRunFinalize(task.WeightFromProto(t.Weight)) {
		s.logger.Debug("finalize not applicable for task weight", "task", taskID, "weight", t.Weight)
		return false, nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		s.logger.Debug("task not in completed state", "task", taskID, "status", t.Status)
		return false, nil
	}

	if t.Execution != nil && t.Execution.Phases != nil {
		if phaseState, ok := t.Execution.Phases["finalize"]; ok {
			if phaseState.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
				s.logger.Debug("finalize already completed", "task", taskID)
				return false, nil
			}
		}
	}

	now := time.Now()
	finState := &FinalizeState{
		TaskID:    taskID,
		Status:    FinalizeStatusPending,
		StartedAt: now,
		UpdatedAt: now,
		Step:      "Initializing",
		Progress:  "Auto-triggered on PR approval",
	}

	if existing, ok := finTracker.tryStart(taskID, finState); !ok {
		existing.mu.RLock()
		status := existing.Status
		existing.mu.RUnlock()
		s.logger.Debug("finalize already in progress", "task", taskID, "status", status)
		return false, nil
	}

	s.publishFinalizeEvent(taskID, finState)
	s.logger.Info("auto-triggering finalize on PR approval", "task", taskID)

	ctx, cancel := context.WithCancel(s.serverCtx)
	finTracker.setCancel(taskID, cancel)

	go s.runFinalizeAsync(ctx, taskID, t, FinalizeRequest{Force: true}, finState, backend, workDir)

	return true, nil
}

// runFinalizeAsync runs the finalize operation asynchronously.
func (s *Server) runFinalizeAsync(ctx context.Context, taskID string, _ *orcv1.Task, _ FinalizeRequest, finState *FinalizeState, backend storage.Backend, workDir string) {
	defer finTracker.cancel(taskID)

	if ctx.Err() != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("cancelled before start: %w", ctx.Err()))
		return
	}

	finState.mu.Lock()
	finState.Status = FinalizeStatusRunning
	finState.UpdatedAt = time.Now()
	finState.Step = "Loading configuration"
	finState.Progress = "Preparing finalize executor"
	finState.StepPercent = 5
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	t, loadErr := backend.LoadTask(taskID)
	if loadErr != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("reload task: %w", loadErr))
		return
	}

	finalizePhase := &executor.PhaseDisplay{
		ID:     "finalize",
		Name:   "Finalize",
		Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
	}

	if ctx.Err() != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("cancelled during setup: %w", ctx.Err()))
		return
	}

	finState.mu.Lock()
	finState.Step = "Setting up git"
	finState.Progress = "Initializing git service"
	finState.StepPercent = 10
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	gitCfg := git.Config{
		BranchPrefix: s.orcConfig.BranchPrefix,
		CommitPrefix: s.orcConfig.CommitPrefix,
		WorktreeDir:  s.orcConfig.Worktree.Dir,
	}
	gitSvc, err := git.New(workDir, gitCfg)
	if err != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("create git service: %w", err))
		return
	}

	targetBranch := "main"
	if s.orcConfig != nil && s.orcConfig.Completion.TargetBranch != "" {
		targetBranch = s.orcConfig.Completion.TargetBranch
	}
	execCfg := executor.ExecutorConfig{
		MaxIterations:      10,
		CheckpointInterval: 1,
		SessionPersistence: true,
		TargetBranch:       targetBranch,
	}

	claudePath := executor.ResolveClaudePath("claude")

	finalizeExec := executor.NewFinalizeExecutor(
		executor.WithFinalizeGitSvc(gitSvc),
		executor.WithFinalizePublisher(s.publisher),
		executor.WithFinalizeLogger(s.logger),
		executor.WithFinalizeConfig(execCfg),
		executor.WithFinalizeOrcConfig(s.orcConfig),
		executor.WithFinalizeWorkingDir(workDir),
		executor.WithFinalizeTaskDir(task.TaskDirIn(workDir, taskID)),
		executor.WithFinalizeBackend(backend),
		executor.WithFinalizeClaudePath(claudePath),
		executor.WithFinalizeExecutionUpdater(func(exec *orcv1.ExecutionState) {
			finState.mu.Lock()
			if ps := exec.Phases["finalize"]; ps != nil {
				// Phase status is completion-only (PENDING, COMPLETED, SKIPPED)
				// Use startedAt and completedAt to determine progress
				if ps.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
					finState.StepPercent = 100
				} else if ps.StartedAt != nil {
					// Started but not completed = in progress
					finState.StepPercent = 50
				}
			}
			finState.mu.Unlock()
		}),
	)

	if ctx.Err() != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("cancelled before execution: %w", ctx.Err()))
		return
	}

	finState.mu.Lock()
	finState.Step = "Executing finalize"
	finState.Progress = "Syncing with target branch"
	finState.StepPercent = 20
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	task.EnsureExecutionProto(t)
	task.StartPhaseProto(t.Execution, "finalize")
	if err := backend.SaveTask(t); err != nil {
		s.logger.Warn("failed to save task", "error", err)
	}

	result, err := finalizeExec.Execute(ctx, t, finalizePhase, t.Execution)
	if err != nil {
		task.FailPhaseProto(t.Execution, "finalize", err)
		_ = backend.SaveTask(t)
		s.finalizeFailed(taskID, finState, err)
		return
	}

	// Result.Status is COMPLETED for success, PENDING otherwise
	// Check result.Error to determine if it failed
	if result.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
		task.CompletePhaseProto(t.Execution, "finalize", result.CommitSHA)
	} else if result.Error != nil {
		task.FailPhaseProto(t.Execution, "finalize", result.Error)
	}
	_ = backend.SaveTask(t)

	finResult := &FinalizeResult{
		Synced:       result.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED,
		CommitSHA:    result.CommitSHA,
		TargetBranch: targetBranch,
	}

	if result.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED && s.orcConfig.ShouldWaitForCI() {
		finState.mu.Lock()
		finState.Step = "Waiting for CI"
		finState.Progress = "Pushing changes and waiting for CI checks..."
		finState.StepPercent = 85
		finState.mu.Unlock()
		s.publishFinalizeEvent(taskID, finState)

		hostingCfg := hosting.Config{}
		if s.orcConfig != nil {
			hostingCfg = hosting.Config{
				Provider:    s.orcConfig.Hosting.Provider,
				BaseURL:     s.orcConfig.Hosting.BaseURL,
				TokenEnvVar: s.orcConfig.Hosting.TokenEnvVar,
			}
		}
		hostingProvider, providerErr := hosting.NewProvider(workDir, hostingCfg)
		if providerErr != nil {
			s.logger.Warn("failed to create hosting provider for CI merge", "error", providerErr)
		}

		ciMergerOpts := []executor.CIMergerOption{
			executor.WithCIMergerLogger(s.logger),
			executor.WithCIMergerWorkDir(workDir),
		}
		if hostingProvider != nil {
			ciMergerOpts = append(ciMergerOpts, executor.WithCIMergerHostingProvider(hostingProvider))
		}
		ciMerger := executor.NewCIMerger(s.orcConfig, ciMergerOpts...)

		ciErr := ciMerger.WaitForCIAndMerge(ctx, t)
		if ciErr != nil {
			s.logger.Warn("CI wait/merge failed", "task", taskID, "error", ciErr)
			finResult.MergeError = ciErr.Error()
		}
	}

	finState.mu.Lock()
	finState.Status = FinalizeStatusCompleted
	finState.UpdatedAt = time.Now()
	if finResult.Merged {
		finState.Step = "Merged"
		finState.Progress = "PR merged successfully"
	} else if finResult.CIPassed {
		finState.Step = "CI Passed"
		finState.Progress = "CI passed, merge skipped"
	} else if finResult.MergeError != "" {
		finState.Step = "Complete (merge pending)"
		finState.Progress = finResult.MergeError
	} else {
		finState.Step = "Complete"
		finState.Progress = "Finalize completed successfully"
	}
	finState.StepPercent = 100
	finState.Result = finResult
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	s.logger.Info("finalize completed", "task", taskID, "commit", result.CommitSHA)

	if s.orcConfig.Worktree.CleanupOnComplete {
		if err := gitSvc.CleanupWorktree(taskID); err != nil {
			s.logger.Warn("failed to cleanup worktree after finalize", "task", taskID, "error", err)
		} else {
			s.logger.Info("worktree cleaned up after finalize", "task", taskID)
		}
	}
}

// finalizeFailed updates the finalize state to failed.
func (s *Server) finalizeFailed(taskID string, finState *FinalizeState, err error) {
	finState.mu.Lock()
	finState.Status = FinalizeStatusFailed
	finState.UpdatedAt = time.Now()
	finState.Step = "Failed"
	finState.Progress = ""
	finState.Error = err.Error()
	finState.mu.Unlock()

	s.publishFinalizeEvent(taskID, finState)
	s.logger.Error("finalize failed", "task", taskID, "error", err)
}

// publishFinalizeEvent publishes a finalize progress event via WebSocket.
func (s *Server) publishFinalizeEvent(taskID string, finState *FinalizeState) {
	finState.mu.RLock()
	data := map[string]any{
		"task_id":      finState.TaskID,
		"status":       finState.Status,
		"step":         finState.Step,
		"progress":     finState.Progress,
		"step_percent": finState.StepPercent,
		"updated_at":   finState.UpdatedAt,
	}
	if finState.Error != "" {
		data["error"] = finState.Error
	}
	if finState.Result != nil {
		data["result"] = finState.Result
	}
	finState.mu.RUnlock()

	event := events.NewEvent(EventFinalize, taskID, data)
	s.publisher.Publish(event)
}
