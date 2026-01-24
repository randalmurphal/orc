// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// handleListProjects returns all registered projects.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := project.ListProjects()
	if err != nil {
		s.jsonError(w, "failed to list projects", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if projects == nil {
		projects = []project.Project{}
	}

	s.jsonResponse(w, projects)
}

// handleGetDefaultProject returns the default project ID.
func (s *Server) handleGetDefaultProject(w http.ResponseWriter, r *http.Request) {
	defaultID, err := project.GetDefaultProject()
	if err != nil {
		s.jsonError(w, "failed to get default project", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"default_project": defaultID})
}

// handleSetDefaultProject sets the default project ID.
func (s *Server) handleSetDefaultProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := project.SetDefaultProject(req.ProjectID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "project not found", http.StatusNotFound)
		} else {
			s.jsonError(w, "failed to set default project", http.StatusInternalServerError)
		}
		return
	}

	s.jsonResponse(w, map[string]string{"default_project": req.ProjectID})
}

// handleGetProject returns a specific project.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(id)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, proj)
}

// handleListProjectTasks returns all tasks for a project.
// Query params:
//   - dependency_status: filter by dependency status (blocked, ready, none)
func (s *Server) handleListProjectTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(id)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Load tasks from project using backend
	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		// No database yet is OK - return empty list
		s.jsonResponse(w, []*task.Task{})
		return
	}
	defer func() { _ = backend.Close() }()

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		// No tasks is OK - return empty list
		s.jsonResponse(w, []*task.Task{})
		return
	}

	if tasks == nil {
		tasks = []*task.Task{}
	}

	// Populate computed dependency fields (Blocks, ReferencedBy, IsBlocked, UnmetBlockers, DependencyStatus)
	task.PopulateComputedFields(tasks)

	// Filter by dependency status if requested
	depStatusFilter := r.URL.Query().Get("dependency_status")
	if depStatusFilter != "" {
		var filtered []*task.Task
		for _, t := range tasks {
			if string(t.DependencyStatus) == depStatusFilter {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
		// Ensure we return an empty array after filtering, not null
		if tasks == nil {
			tasks = []*task.Task{}
		}
	}

	s.jsonResponse(w, tasks)
}

// handleCreateProjectTask creates a new task in a project.
// Supports both JSON and multipart/form-data content types.
// With multipart/form-data, files can be attached via "attachments" field.
func (s *Server) handleCreateProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Parse request - supports both JSON and multipart/form-data
	var title, description, weight, category string
	var isMultipart bool

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Parse multipart form (max 32MB)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			s.jsonError(w, "failed to parse form", http.StatusBadRequest)
			return
		}
		title = r.FormValue("title")
		description = r.FormValue("description")
		weight = r.FormValue("weight")
		category = r.FormValue("category")
		isMultipart = true
	} else {
		// Parse as JSON
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
			Weight      string `json:"weight,omitempty"`
			Category    string `json:"category,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		title = req.Title
		description = req.Description
		weight = req.Weight
		category = req.Category
	}

	if title == "" {
		s.jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	// Get project backend
	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	// Generate ID in project context
	id, err := backend.GetNextTaskID()
	if err != nil {
		s.jsonError(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.New(id, title)
	t.Description = description
	if weight != "" {
		t.Weight = task.Weight(weight)
	} else {
		t.Weight = task.WeightMedium
	}
	if category != "" {
		cat := task.Category(category)
		if task.IsValidCategory(cat) {
			t.Category = cat
		}
	}

	// Save task
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	// Task is now created - mark as planned (execution will determine phases)
	t.Status = task.StatusPlanned
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	// Handle file attachments if this was a multipart request
	if isMultipart && r.MultipartForm != nil {
		files := r.MultipartForm.File["attachments"]
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				s.logger.Warn("failed to open attachment",
					"taskID", id,
					"filename", fileHeader.Filename,
					"error", err,
				)
				continue
			}

			filename := filepath.Base(fileHeader.Filename)
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				s.logger.Warn("failed to read attachment",
					"taskID", id,
					"filename", filename,
					"error", err,
				)
				continue
			}

			_, err = backend.SaveAttachment(id, filename, fileHeader.Header.Get("Content-Type"), data)
			if err != nil {
				s.logger.Warn("failed to save attachment",
					"taskID", id,
					"filename", filename,
					"error", err,
				)
			}
		}
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, t)
}

// handleGetProjectTask returns a specific task from a project.
func (s *Server) handleGetProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	s.jsonResponse(w, t)
}

// handleDeleteProjectTask deletes a task from a project.
func (s *Server) handleDeleteProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	if err := backend.DeleteTask(taskID); err != nil {
		s.jsonError(w, "failed to delete task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRunProjectTask starts task execution for a project task.
func (s *Server) handleRunProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	s.logger.Info("handleRunProjectTask", "projectID", projectID, "taskID", taskID)

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	s.logger.Info("resolved project", "name", proj.Name, "path", proj.Path)

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	s.logger.Info("loaded task", "id", t.ID, "title", t.Title)

	if !t.CanRun() {
		s.jsonError(w, "task cannot be run in current state", http.StatusBadRequest)
		return
	}

	// Get workflow ID from task - MUST be set
	workflowID := t.WorkflowID
	if workflowID == "" {
		s.jsonError(w, fmt.Sprintf("task %s has no workflow_id set - cannot run", taskID), http.StatusBadRequest)
		return
	}

	// Mark task as running
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	s.logger.Info("saving task", "taskID", taskID)
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Capture project path for goroutine
	projectPath := proj.Path

	// Start execution in background
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		// Create executor backend (goroutine needs its own backend)
		execBackend, err := s.getProjectBackend(projectPath)
		if err != nil {
			s.logger.Error("failed to create executor backend", "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
			s.ensureTaskStatusConsistent(taskID, err)
			return
		}
		defer func() { _ = execBackend.Close() }()

		// Create git operations
		gitOps, err := git.New(projectPath, git.DefaultConfig())
		if err != nil {
			s.logger.Error("failed to create git ops", "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
			s.ensureTaskStatusConsistent(taskID, err)
			return
		}

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			execBackend,
			execBackend.DB(),
			s.orcConfig,
			projectPath,
			executor.WithWorkflowGitOps(gitOps),
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
		)

		// Build run options
		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      taskID,
			Prompt:      t.Description,
			Category:    t.Category,
		}

		// Execute workflow
		result, err := we.Run(ctx, workflowID, opts)
		if err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID, "run_id", result.RunID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		s.ensureTaskStatusConsistent(taskID, err)
	}()

	// Return task with updated status so frontend can update store immediately
	s.jsonResponse(w, map[string]any{
		"status":  "started",
		"task_id": taskID,
		"task":    t,
	})
}

// handlePauseProjectTask pauses a running project task.
func (s *Server) handlePauseProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	if t.Status != task.StatusRunning {
		s.jsonError(w, "task is not running", http.StatusBadRequest)
		return
	}

	// Cancel the running executor
	s.runningTasksMu.Lock()
	cancel, running := s.runningTasks[taskID]
	s.runningTasksMu.Unlock()
	if running {
		s.logger.Info("cancelling running executor", "task", taskID)
		cancel()
	}

	// Update task status
	t.Status = task.StatusPaused
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Update state status
	if st, err := backend.LoadState(taskID); err == nil && st != nil {
		st.Status = state.StatusPaused
		if err := backend.SaveState(st); err != nil {
			s.logger.Error("failed to save state", "error", err)
		}
	}

	s.jsonResponse(w, map[string]any{
		"status":  "paused",
		"task_id": taskID,
	})
}

// handleResumeProjectTask resumes a paused project task.
func (s *Server) handleResumeProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Task must be paused or failed to resume
	if t.Status != task.StatusPaused && t.Status != task.StatusFailed {
		s.jsonError(w, "task is not paused or failed", http.StatusBadRequest)
		return
	}

	// Get workflow ID from task - MUST be set
	workflowID := t.WorkflowID
	if workflowID == "" {
		s.jsonError(w, fmt.Sprintf("task %s has no workflow_id set - cannot resume", taskID), http.StatusBadRequest)
		return
	}

	// Update task status
	t.Status = task.StatusRunning
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Capture project path for goroutine
	projectPath := proj.Path

	s.logger.Info("resuming task execution", "task", taskID)

	// Start execution in background
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		// Create executor backend (goroutine needs its own backend)
		execBackend, err := s.getProjectBackend(projectPath)
		if err != nil {
			s.logger.Error("failed to create executor backend", "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
			s.ensureTaskStatusConsistent(taskID, err)
			return
		}
		defer func() { _ = execBackend.Close() }()

		// Create git operations
		gitOps, err := git.New(projectPath, git.DefaultConfig())
		if err != nil {
			s.logger.Error("failed to create git ops", "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
			s.ensureTaskStatusConsistent(taskID, err)
			return
		}

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			execBackend,
			execBackend.DB(),
			s.orcConfig,
			projectPath,
			executor.WithWorkflowGitOps(gitOps),
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
		)

		// Build run options for resume
		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      taskID,
			Prompt:      t.Description,
			Category:    t.Category,
		}

		// Execute workflow (WorkflowExecutor handles resume internally)
		result, err := we.Run(ctx, workflowID, opts)
		if err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID, "run_id", result.RunID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		s.ensureTaskStatusConsistent(taskID, err)
	}()

	s.jsonResponse(w, map[string]any{
		"status":  "resumed",
		"task_id": taskID,
	})
}

// handleRewindProjectTask rewinds a task to a previous phase.
func (s *Server) handleRewindProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req struct {
		Phase string `json:"phase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Phase == "" {
		s.jsonError(w, "phase is required", http.StatusBadRequest)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Get workflow phases from database to validate phase exists
	phases, err := s.getWorkflowPhases(backend, t.WorkflowID)
	if err != nil {
		s.jsonError(w, "failed to get workflow phases", http.StatusInternalServerError)
		return
	}

	// Find target phase
	targetFound := false
	for _, p := range phases {
		if p.ID == req.Phase {
			targetFound = true
			break
		}
	}
	if !targetFound {
		s.jsonError(w, "phase not found", http.StatusBadRequest)
		return
	}

	// Load state (may not exist)
	st, _ := backend.LoadState(taskID)
	if st == nil {
		st = &state.State{
			TaskID: taskID,
			Phases: make(map[string]*state.PhaseState),
		}
	}

	// Mark target and all later phases as pending in state
	foundTarget := false
	for _, phase := range phases {
		if phase.ID == req.Phase {
			foundTarget = true
		}
		if foundTarget {
			if st.Phases[phase.ID] != nil {
				st.Phases[phase.ID].Status = state.StatusPending
				st.Phases[phase.ID].CompletedAt = nil
			}
		}
	}

	// Update state to point to target phase
	st.Status = state.StatusPending
	st.CurrentPhase = req.Phase
	st.CurrentIteration = 1
	st.CompletedAt = nil

	// Update task status to allow re-running
	t.Status = task.StatusPlanned
	t.CompletedAt = nil

	// Save all updates
	if err := backend.SaveState(st); err != nil {
		s.jsonError(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	s.logger.Info("rewound task", "task", taskID, "toPhase", req.Phase)

	s.jsonResponse(w, map[string]any{
		"status":  "rewound",
		"task_id": taskID,
		"phase":   req.Phase,
	})
}

// handleEscalateProjectTask escalates a task from Review/QA back to Implementation with context.
// This is used for the ralph-loop style workflow where human reviewers can send tasks back
// to the AI with specific feedback on what needs to be fixed.
func (s *Server) handleEscalateProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		s.jsonError(w, "reason is required for escalation", http.StatusBadRequest)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Task must be in paused or blocked state to be escalated
	if t.Status != task.StatusPaused && t.Status != task.StatusBlocked {
		s.jsonError(w, "task must be paused or blocked to escalate", http.StatusBadRequest)
		return
	}

	// Get workflow phases from database
	phases, err := s.getWorkflowPhases(backend, t.WorkflowID)
	if err != nil {
		s.jsonError(w, "failed to get workflow phases", http.StatusInternalServerError)
		return
	}

	// Find implement phase - this is always where we escalate to
	implementFound := false
	for _, p := range phases {
		if p.ID == "implement" {
			implementFound = true
			break
		}
	}
	if !implementFound {
		s.jsonError(w, "implement phase not found", http.StatusBadRequest)
		return
	}

	// Load state (may not exist)
	st, _ := backend.LoadState(taskID)
	if st == nil {
		st = &state.State{
			TaskID: taskID,
			Phases: make(map[string]*state.PhaseState),
		}
	}

	// Get current phase for context
	currentPhase := st.CurrentPhase
	if currentPhase == "" {
		currentPhase = "review"
	}

	// Get retry attempt number from existing context or start at 1
	attempt := 1
	if st.RetryContext != nil && st.RetryContext.Attempt > 0 {
		attempt = st.RetryContext.Attempt + 1
	}

	// Save escalation context as a retry context file
	_, saveErr := executor.SaveRetryContextFile(
		proj.Path,
		taskID,
		currentPhase,
		"implement",
		"Human escalation: "+req.Reason,
		"", // No output, this is manual escalation
		attempt,
	)
	if saveErr != nil {
		s.logger.Warn("failed to save escalation context file", "error", saveErr)
		// Continue anyway, the reason is still in state
	}

	// Set retry context in state
	st.SetRetryContext(currentPhase, "implement", req.Reason, "", attempt)

	// Mark implement and all later phases as pending in state
	foundTarget := false
	for _, phase := range phases {
		if phase.ID == "implement" {
			foundTarget = true
		}
		if foundTarget {
			if st.Phases[phase.ID] != nil {
				st.Phases[phase.ID].Status = state.StatusPending
				st.Phases[phase.ID].CompletedAt = nil
			}
		}
	}

	// Update state to point to implement phase
	st.Status = state.StatusPending
	st.CurrentPhase = "implement"
	st.CurrentIteration = 1
	st.CompletedAt = nil

	// Update task status to allow re-running
	t.Status = task.StatusPlanned
	t.CompletedAt = nil

	// Save all updates
	if err := backend.SaveState(st); err != nil {
		s.jsonError(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	if err := backend.SaveTask(t); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	s.logger.Info("escalated task", "task", taskID, "reason", req.Reason)

	s.jsonResponse(w, map[string]any{
		"status":  "escalated",
		"task_id": taskID,
		"phase":   "implement",
		"reason":  req.Reason,
		"attempt": attempt,
	})
}

// handleGetProjectTaskState returns the state for a project task.
func (s *Server) handleGetProjectTaskState(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	st, err := backend.LoadState(taskID)
	if err != nil || st == nil {
		s.jsonError(w, "state not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, st)
}

// handleGetProjectTaskPlan returns the plan for a project task.
func (s *Server) handleGetProjectTaskPlan(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	backend, err := s.getProjectBackend(proj.Path)
	if err != nil {
		s.jsonError(w, "failed to access project database", http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	t, err := backend.LoadTask(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Get plan from database workflow phases
	p, err := s.getWorkflowPhasesWithPlan(backend, taskID, t.WorkflowID)
	if err != nil {
		s.jsonError(w, "failed to get workflow phases", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetProjectTaskTranscripts returns transcripts for a project task.
func (s *Server) handleGetProjectTaskTranscripts(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	transcriptsDir := filepath.Join(proj.Path, ".orc", "tasks", taskID, "transcripts")
	entries, err := os.ReadDir(transcriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonResponse(w, []any{})
			return
		}
		s.jsonError(w, "failed to read transcripts", http.StatusInternalServerError)
		return
	}

	var transcripts []map[string]any
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(transcriptsDir, entry.Name()))
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		transcripts = append(transcripts, map[string]any{
			"filename":   entry.Name(),
			"content":    string(content),
			"created_at": info.ModTime().Format(time.RFC3339),
		})
	}

	if transcripts == nil {
		transcripts = []map[string]any{}
	}

	s.jsonResponse(w, transcripts)
}

// getProject loads a project by ID.
func (s *Server) getProject(projectID string) (*project.Project, error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Get(projectID)
}

// getProjectBackend creates a storage backend for a specific project path.
// The caller is responsible for closing the backend when done.
func (s *Server) getProjectBackend(projectPath string) (storage.Backend, error) {
	var storageCfg *config.StorageConfig
	if s.orcConfig != nil {
		storageCfg = &s.orcConfig.Storage
	}
	return storage.NewDatabaseBackend(projectPath, storageCfg)
}

// loadProjectTask loads a task from a specific project using a backend.
func (s *Server) loadProjectTask(projectPath, taskID string) (*task.Task, error) {
	backend, err := s.getProjectBackend(projectPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = backend.Close() }()

	return backend.LoadTask(taskID)
}

// phaseInfo represents basic phase information for API responses.
type phaseInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// getWorkflowPhases returns the phase IDs for a workflow from the database.
func (s *Server) getWorkflowPhases(backend storage.Backend, workflowID string) ([]phaseInfo, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	dbPhases, err := backend.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, err
	}

	phases := make([]phaseInfo, len(dbPhases))
	for i, p := range dbPhases {
		// Get phase template for name
		template, err := backend.GetPhaseTemplate(p.PhaseTemplateID)
		if err != nil || template == nil {
			phases[i] = phaseInfo{ID: p.PhaseTemplateID, Name: p.PhaseTemplateID}
		} else {
			phases[i] = phaseInfo{ID: p.PhaseTemplateID, Name: template.Name}
		}
	}

	return phases, nil
}

// getWorkflowPhasesWithPlan returns phases as an executor.Plan for API compatibility.
func (s *Server) getWorkflowPhasesWithPlan(backend storage.Backend, taskID string, workflowID string) (*executor.Plan, error) {
	phases, err := s.getWorkflowPhases(backend, workflowID)
	if err != nil {
		return nil, err
	}

	planPhases := make([]executor.Phase, len(phases))
	for i, p := range phases {
		planPhases[i] = executor.Phase{
			ID:     p.ID,
			Name:   p.Name,
			Status: executor.PhasePending,
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: planPhases,
	}, nil
}
