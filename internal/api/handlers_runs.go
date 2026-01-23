package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// handleListWorkflowRuns returns workflow runs with optional filtering.
func (s *Server) handleListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	opts := db.WorkflowRunListOpts{}

	// Parse query parameters
	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = status
	}
	if workflowID := r.URL.Query().Get("workflow_id"); workflowID != "" {
		opts.WorkflowID = workflowID
	}
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		opts.TaskID = taskID
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			opts.Limit = l
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 50 // Default limit
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			opts.Offset = o
		}
	}

	runs, err := s.backend.ListWorkflowRuns(opts)
	if err != nil {
		s.jsonError(w, "failed to list workflow runs", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if runs == nil {
		runs = []*db.WorkflowRun{}
	}

	s.jsonResponse(w, runs)
}

// handleGetWorkflowRun returns a single workflow run with its phases.
func (s *Server) handleGetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.backend.GetWorkflowRun(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow run", http.StatusInternalServerError)
		return
	}
	if run == nil {
		s.jsonError(w, "workflow run not found", http.StatusNotFound)
		return
	}

	// Get run phases
	phases, err := s.backend.GetWorkflowRunPhases(id)
	if err != nil {
		s.jsonError(w, "failed to get run phases", http.StatusInternalServerError)
		return
	}
	if phases == nil {
		phases = []*db.WorkflowRunPhase{}
	}

	// Return enriched run
	response := struct {
		*db.WorkflowRun
		Phases []*db.WorkflowRunPhase `json:"phases"`
	}{
		WorkflowRun: run,
		Phases:      phases,
	}

	s.jsonResponse(w, response)
}

// handleTriggerWorkflowRun triggers a new workflow run.
func (s *Server) handleTriggerWorkflowRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkflowID   string            `json:"workflow_id"`
		Prompt       string            `json:"prompt"`
		Instructions string            `json:"instructions,omitempty"`
		ContextType  string            `json:"context_type,omitempty"` // default, task, branch, standalone
		TaskID       string            `json:"task_id,omitempty"`
		Branch       string            `json:"branch,omitempty"`
		PRID         int               `json:"pr_id,omitempty"`
		Category     string            `json:"category,omitempty"`
		Variables    map[string]string `json:"variables,omitempty"`
		Stream       bool              `json:"stream,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.WorkflowID == "" {
		s.jsonError(w, "workflow_id is required", http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		s.jsonError(w, "prompt is required", http.StatusBadRequest)
		return
	}

	// Verify workflow exists
	wf, err := s.backend.GetWorkflow(req.WorkflowID)
	if err != nil || wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	// Determine context type
	contextType := executor.ContextDefault
	if req.ContextType != "" {
		switch req.ContextType {
		case "task":
			contextType = executor.ContextTask
		case "branch":
			contextType = executor.ContextBranch
		case "pr":
			contextType = executor.ContextPR
		case "standalone":
			contextType = executor.ContextStandalone
		}
	}

	// Load config
	orcConfig, err := config.Load()
	if err != nil {
		s.jsonError(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	// Get project database
	pdb, err := db.OpenProject(s.workDir)
	if err != nil {
		s.jsonError(w, "failed to open project database", http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	// Create git operations
	gitOps, err := git.New(s.workDir, git.DefaultConfig())
	if err != nil {
		s.jsonError(w, "failed to initialize git", http.StatusInternalServerError)
		return
	}

	// Create workflow executor
	claudePath := orcConfig.ClaudePath
	if claudePath == "" {
		claudePath = "claude"
	}

	we := executor.NewWorkflowExecutor(
		s.backend,
		pdb,
		orcConfig,
		s.workDir,
		executor.WithWorkflowGitOps(gitOps),
		executor.WithWorkflowClaudePath(claudePath),
		executor.WithWorkflowPublisher(s.publisher),
		executor.WithWorkflowLogger(s.logger),
		executor.WithWorkflowAutomationService(s.automationSvc),
	)

	// Build options
	category := task.CategoryFeature
	if req.Category != "" {
		category = task.Category(req.Category)
	}

	opts := executor.WorkflowRunOptions{
		ContextType:  contextType,
		Prompt:       req.Prompt,
		Instructions: req.Instructions,
		TaskID:       req.TaskID,
		Branch:       req.Branch,
		PRID:         req.PRID,
		Category:     category,
		Variables:    req.Variables,
		Stream:       req.Stream,
	}

	// Execute workflow in background
	go func() {
		ctx := context.Background()
		result, err := we.Run(ctx, req.WorkflowID, opts)
		if err != nil {
			slog.Error("workflow run failed",
				"workflow", req.WorkflowID,
				"error", err,
			)
			return
		}

		slog.Info("workflow run completed",
			"run_id", result.RunID,
			"workflow", req.WorkflowID,
			"success", result.Success,
		)
	}()

	// Return immediately with run info
	// The run ID will be generated when the executor starts
	s.jsonResponse(w, map[string]interface{}{
		"status":      "started",
		"workflow_id": req.WorkflowID,
		"message":     "Workflow run started. Use GET /api/workflow-runs to check status.",
	})
}

// handleCancelWorkflowRun cancels a running workflow.
func (s *Server) handleCancelWorkflowRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.backend.GetWorkflowRun(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow run", http.StatusInternalServerError)
		return
	}
	if run == nil {
		s.jsonError(w, "workflow run not found", http.StatusNotFound)
		return
	}

	// Check if run can be cancelled
	if run.Status != string(workflow.RunStatusRunning) &&
		run.Status != string(workflow.RunStatusPending) {
		s.jsonError(w, "cannot cancel run with status: "+run.Status, http.StatusConflict)
		return
	}

	// Update status
	run.Status = string(workflow.RunStatusCancelled)
	run.Error = "cancelled via API"
	now := time.Now()
	run.CompletedAt = &now

	if err := s.backend.SaveWorkflowRun(run); err != nil {
		s.jsonError(w, "failed to update workflow run", http.StatusInternalServerError)
		return
	}

	// TODO: Signal the running process to stop
	// This requires tracking the execution context

	s.jsonResponse(w, map[string]string{
		"status":  "cancelled",
		"run_id":  id,
		"message": "Workflow run marked as cancelled. Running process may need manual termination.",
	})
}

// handleGetWorkflowRunTranscript returns the transcript for a workflow run.
func (s *Server) handleGetWorkflowRunTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.backend.GetWorkflowRun(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow run", http.StatusInternalServerError)
		return
	}
	if run == nil {
		s.jsonError(w, "workflow run not found", http.StatusNotFound)
		return
	}

	// If run has a task, get transcripts from task
	if run.TaskID != nil {
		transcripts, err := s.backend.GetTranscripts(*run.TaskID)
		if err != nil {
			s.jsonError(w, "failed to get transcripts", http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, map[string]interface{}{
			"run_id":      id,
			"task_id":     *run.TaskID,
			"transcripts": transcripts,
		})
		return
	}

	// For non-task runs, return run phases with any available info
	phases, _ := s.backend.GetWorkflowRunPhases(id)

	s.jsonResponse(w, map[string]interface{}{
		"run_id": id,
		"phases": phases,
		"note":   "Full transcripts available for task-attached runs",
	})
}
