package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/randalmurphal/llmkit/claude/session"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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
	CIPassed    bool   `json:"ci_passed,omitempty"`     // CI checks passed
	CIDetails   string `json:"ci_details,omitempty"`    // CI status summary
	Merged      bool   `json:"merged,omitempty"`        // PR was merged
	MergeCommit string `json:"merge_commit,omitempty"`  // SHA of merge commit
	CITimedOut  bool   `json:"ci_timed_out,omitempty"`  // CI polling timed out
	MergeError  string `json:"merge_error,omitempty"`   // Error during CI/merge
}

// FinalizeRequest is the request body for triggering finalize.
type FinalizeRequest struct {
	Force        bool `json:"force,omitempty"`         // Force finalize even if blockers exist
	GateOverride bool `json:"gate_override,omitempty"` // Override gate checks
}

// FinalizeResponse is the response for the finalize endpoint.
type FinalizeResponse struct {
	TaskID  string         `json:"task_id"`
	Status  FinalizeStatus `json:"status"`
	Message string         `json:"message,omitempty"`
}

// finalizeTracker tracks ongoing finalize operations.
type finalizeTracker struct {
	mu     sync.RWMutex
	states map[string]*FinalizeState
}

var finTracker = &finalizeTracker{
	states: make(map[string]*FinalizeState),
}

// get retrieves the finalize state for a task.
func (ft *finalizeTracker) get(taskID string) *FinalizeState {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return ft.states[taskID]
}

// set stores the finalize state for a task.
func (ft *finalizeTracker) set(taskID string, state *FinalizeState) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.states[taskID] = state
}

// delete removes the finalize state for a task.
func (ft *finalizeTracker) delete(taskID string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	delete(ft.states, taskID)
}

// handleFinalizeTask triggers the finalize phase for a task.
// POST /api/tasks/:id/finalize
func (s *Server) handleFinalizeTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req FinalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body - use defaults
		req = FinalizeRequest{}
	}

	// Check if finalize is already running
	if existing := finTracker.get(taskID); existing != nil {
		existing.mu.RLock()
		status := existing.Status
		existing.mu.RUnlock()
		if status == FinalizeStatusRunning || status == FinalizeStatusPending {
			s.jsonResponse(w, FinalizeResponse{
				TaskID:  taskID,
				Status:  status,
				Message: "Finalize already in progress",
			})
			return
		}
	}

	// Load task
	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Check task status - should be completed or in a finalizable state
	if t.Status != task.StatusCompleted && t.Status != task.StatusPlanned && t.Status != task.StatusFailed {
		// Allow finalize on completed/planned/failed tasks
		if !req.Force {
			s.jsonError(w, fmt.Sprintf("task cannot be finalized in status: %s (use force=true to override)", t.Status), http.StatusBadRequest)
			return
		}
	}

	// Initialize finalize state
	now := time.Now()
	finState := &FinalizeState{
		TaskID:    taskID,
		Status:    FinalizeStatusPending,
		StartedAt: now,
		UpdatedAt: now,
		Step:      "Initializing",
		Progress:  "Starting finalize process",
	}
	finTracker.set(taskID, finState)

	// Publish initial event
	s.publishFinalizeEvent(taskID, finState)

	// Start async finalize
	go s.runFinalizeAsync(taskID, t, req, finState)

	// Return immediate acknowledgment
	s.jsonResponse(w, FinalizeResponse{
		TaskID:  taskID,
		Status:  FinalizeStatusPending,
		Message: "Finalize started",
	})
}

// handleGetFinalizeStatus returns the status of a finalize operation.
// GET /api/tasks/:id/finalize
func (s *Server) handleGetFinalizeStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	// Check if task exists
	if !task.ExistsIn(s.workDir, taskID) {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Get finalize state
	finState := finTracker.get(taskID)
	if finState == nil {
		// No finalize in progress - check state.yaml for completed finalize
		st, err := state.LoadFrom(s.workDir, taskID)
		if err != nil {
			s.jsonResponse(w, map[string]any{
				"task_id": taskID,
				"status":  "not_started",
				"message": "No finalize operation found",
			})
			return
		}

		// Check if finalize phase is complete in state
		if phaseState, ok := st.Phases["finalize"]; ok {
			var status FinalizeStatus
			switch phaseState.Status {
			case state.StatusCompleted:
				status = FinalizeStatusCompleted
			case state.StatusFailed:
				status = FinalizeStatusFailed
			case state.StatusRunning:
				status = FinalizeStatusRunning
			default:
				status = FinalizeStatusPending
			}

			s.jsonResponse(w, map[string]any{
				"task_id":      taskID,
				"status":       status,
				"started_at":   phaseState.StartedAt,
				"completed_at": phaseState.CompletedAt,
				"commit_sha":   phaseState.CommitSHA,
				"error":        phaseState.Error,
			})
			return
		}

		// No finalize info
		s.jsonResponse(w, map[string]any{
			"task_id": taskID,
			"status":  "not_started",
			"message": "No finalize operation found",
		})
		return
	}

	// Return current state
	finState.mu.RLock()
	defer finState.mu.RUnlock()

	resp := map[string]any{
		"task_id":      finState.TaskID,
		"status":       finState.Status,
		"started_at":   finState.StartedAt,
		"updated_at":   finState.UpdatedAt,
		"step":         finState.Step,
		"progress":     finState.Progress,
		"step_percent": finState.StepPercent,
	}

	if finState.Result != nil {
		resp["result"] = finState.Result
	}
	if finState.Error != "" {
		resp["error"] = finState.Error
	}

	s.jsonResponse(w, resp)
}

// runFinalizeAsync runs the finalize operation asynchronously.
func (s *Server) runFinalizeAsync(taskID string, t *task.Task, _ FinalizeRequest, finState *FinalizeState) {
	ctx := context.Background()

	// Update state to running
	finState.mu.Lock()
	finState.Status = FinalizeStatusRunning
	finState.UpdatedAt = time.Now()
	finState.Step = "Loading configuration"
	finState.Progress = "Preparing finalize executor"
	finState.StepPercent = 5
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	// Load plan and state
	p, err := plan.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("load plan: %w", err))
		return
	}

	st, err := state.LoadFrom(s.workDir, taskID)
	if err != nil {
		st = state.New(taskID)
	}

	// Find or create finalize phase
	var finalizePhase *plan.Phase
	for i := range p.Phases {
		if p.Phases[i].ID == "finalize" {
			finalizePhase = &p.Phases[i]
			break
		}
	}
	if finalizePhase == nil {
		// Create finalize phase if not in plan
		finalizePhase = &plan.Phase{
			ID:     "finalize",
			Status: plan.PhasePending,
		}
	}

	// Update progress
	finState.mu.Lock()
	finState.Step = "Setting up git"
	finState.Progress = "Initializing git service"
	finState.StepPercent = 10
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	// Create git service
	gitCfg := git.Config{
		BranchPrefix: s.orcConfig.BranchPrefix,
		CommitPrefix: s.orcConfig.CommitPrefix,
		WorktreeDir:  s.orcConfig.Worktree.Dir,
	}
	gitSvc, err := git.New(s.workDir, gitCfg)
	if err != nil {
		s.finalizeFailed(taskID, finState, fmt.Errorf("create git service: %w", err))
		return
	}

	// Create session manager (may be nil if no AI resolution needed)
	var mgr session.SessionManager
	// For now, we'll create a basic executor without session manager
	// This handles the git sync and test running

	// Create finalize executor config
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

	finalizeExec := executor.NewFinalizeExecutor(
		mgr,
		executor.WithFinalizeGitSvc(gitSvc),
		executor.WithFinalizePublisher(s.publisher),
		executor.WithFinalizeLogger(s.logger),
		executor.WithFinalizeConfig(execCfg),
		executor.WithFinalizeOrcConfig(s.orcConfig),
		executor.WithFinalizeWorkingDir(s.workDir),
		executor.WithFinalizeTaskDir(task.TaskDirIn(s.workDir, taskID)),
		executor.WithFinalizeStateUpdater(func(updatedState *state.State) {
			// Update progress based on state changes
			finState.mu.Lock()
			if updatedState.CurrentPhase == "finalize" {
				switch updatedState.Status {
				case state.StatusRunning:
					finState.StepPercent = 50
				case state.StatusCompleted:
					finState.StepPercent = 100
				}
			}
			finState.mu.Unlock()
		}),
	)

	// Update progress - starting execution
	finState.mu.Lock()
	finState.Step = "Executing finalize"
	finState.Progress = "Syncing with target branch"
	finState.StepPercent = 20
	finState.mu.Unlock()
	s.publishFinalizeEvent(taskID, finState)

	// Update state to running
	st.StartPhase("finalize")
	if err := st.SaveTo(task.TaskDirIn(s.workDir, taskID)); err != nil {
		s.logger.Warn("failed to save state", "error", err)
	}

	// Execute finalize
	result, err := finalizeExec.Execute(ctx, t, finalizePhase, st)
	if err != nil {
		st.FailPhase("finalize", err)
		_ = st.SaveTo(task.TaskDirIn(s.workDir, taskID))
		s.finalizeFailed(taskID, finState, err)
		return
	}

	// Update state
	switch result.Status {
	case plan.PhaseCompleted:
		st.CompletePhase("finalize", result.CommitSHA)
	case plan.PhaseFailed:
		st.FailPhase("finalize", result.Error)
	}
	_ = st.SaveTo(task.TaskDirIn(s.workDir, taskID))

	// Build result from executor result
	finResult := &FinalizeResult{
		Synced:       result.Status == plan.PhaseCompleted,
		CommitSHA:    result.CommitSHA,
		TargetBranch: targetBranch,
	}

	// Wait for CI and merge if configured (auto/fast profiles only)
	if result.Status == plan.PhaseCompleted && s.orcConfig.ShouldWaitForCI() {
		// Update progress
		finState.mu.Lock()
		finState.Step = "Waiting for CI"
		finState.Progress = "Pushing changes and waiting for CI checks..."
		finState.StepPercent = 85
		finState.mu.Unlock()
		s.publishFinalizeEvent(taskID, finState)

		// Create CI merger and wait for CI/merge
		ciMerger := executor.NewCIMerger(
			s.orcConfig,
			executor.WithCIMergerLogger(s.logger),
			executor.WithCIMergerWorkDir(s.workDir),
		)

		ciErr := ciMerger.WaitForCIAndMerge(ctx, t)

		if ciErr != nil {
			s.logger.Warn("CI wait/merge failed", "task", taskID, "error", ciErr)
			// Don't fail finalize - the sync was successful, just CI/merge failed
			finResult.MergeError = ciErr.Error()
		}
	}

	// Update finalize state to completed
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

	// Auto-commit: finalize completed
	s.autoCommitTaskState(taskID, "finalize completed")

	s.logger.Info("finalize completed", "task", taskID, "commit", result.CommitSHA)
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

	// Auto-commit: finalize failed
	s.autoCommitTaskState(taskID, "finalize failed")

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

// EventFinalize is the event type for finalize progress.
const EventFinalize events.EventType = "finalize"

// TriggerFinalizeOnApproval is called when a PR is approved and auto-trigger is enabled.
// It checks if finalize should run and triggers it asynchronously.
// Returns true if finalize was triggered, false otherwise.
func (s *Server) TriggerFinalizeOnApproval(taskID string) (bool, error) {
	// Check if auto-trigger on approval is enabled
	if !s.orcConfig.ShouldAutoTriggerFinalizeOnApproval() {
		s.logger.Debug("auto-trigger on approval disabled", "task", taskID)
		return false, nil
	}

	// Check if finalize is already running
	if existing := finTracker.get(taskID); existing != nil {
		existing.mu.RLock()
		status := existing.Status
		existing.mu.RUnlock()
		if status == FinalizeStatusRunning || status == FinalizeStatusPending {
			s.logger.Debug("finalize already in progress", "task", taskID, "status", status)
			return false, nil
		}
	}

	// Load task
	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		return false, fmt.Errorf("load task: %w", err)
	}

	// Check if task weight supports finalize
	if !s.orcConfig.ShouldRunFinalize(string(t.Weight)) {
		s.logger.Debug("finalize not applicable for task weight", "task", taskID, "weight", t.Weight)
		return false, nil
	}

	// Check task status - must be completed (has a PR that's approved)
	// Tasks in other states (running, failed) shouldn't be auto-finalized
	if t.Status != task.StatusCompleted {
		s.logger.Debug("task not in completed state", "task", taskID, "status", t.Status)
		return false, nil
	}

	// Check if finalize was already completed
	st, err := state.LoadFrom(s.workDir, taskID)
	if err == nil {
		if phaseState, ok := st.Phases["finalize"]; ok {
			if phaseState.Status == state.StatusCompleted {
				s.logger.Debug("finalize already completed", "task", taskID)
				return false, nil
			}
		}
	}

	// Initialize finalize state
	now := time.Now()
	finState := &FinalizeState{
		TaskID:    taskID,
		Status:    FinalizeStatusPending,
		StartedAt: now,
		UpdatedAt: now,
		Step:      "Initializing",
		Progress:  "Auto-triggered on PR approval",
	}
	finTracker.set(taskID, finState)

	// Publish initial event
	s.publishFinalizeEvent(taskID, finState)

	// Log the auto-trigger
	s.logger.Info("auto-triggering finalize on PR approval", "task", taskID)

	// Start async finalize
	go s.runFinalizeAsync(taskID, t, FinalizeRequest{Force: true}, finState)

	return true, nil
}
