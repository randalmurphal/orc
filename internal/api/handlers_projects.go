// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/state"
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

	// Load tasks from project directory
	tasksDir := filepath.Join(proj.Path, ".orc", "tasks")
	tasks, err := task.LoadAllFrom(tasksDir)
	if err != nil {
		// No tasks dir is OK - return empty list
		s.jsonResponse(w, []*task.Task{})
		return
	}

	if tasks == nil {
		tasks = []*task.Task{}
	}

	s.jsonResponse(w, tasks)
}

// handleCreateProjectTask creates a new task in a project.
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

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		Weight      string `json:"weight,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		s.jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	// Generate ID in project context
	id, err := task.NextIDIn(filepath.Join(proj.Path, ".orc", "tasks"))
	if err != nil {
		s.jsonError(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.New(id, req.Title)
	t.Description = req.Description
	if req.Weight != "" {
		t.Weight = task.Weight(req.Weight)
	} else {
		t.Weight = task.WeightMedium
	}

	// Save in project directory
	if err := t.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	// Create plan from template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
		p = &plan.Plan{
			Version:     1,
			TaskID:      id,
			Weight:      t.Weight,
			Description: "Default plan",
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
			},
		}
	}

	// Save plan in project directory
	if err := p.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}

	t.Status = task.StatusPlanned
	if err := t.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
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

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	if err := os.RemoveAll(taskDir); err != nil {
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

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	s.logger.Info("loaded task", "id", t.ID, "title", t.Title)

	if !t.CanRun() {
		s.jsonError(w, "task cannot be run in current state", http.StatusBadRequest)
		return
	}

	// Load plan
	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Load or create state
	statePath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "state.yaml")
	var st state.State
	if stateData, err := os.ReadFile(statePath); err == nil {
		yaml.Unmarshal(stateData, &st)
	} else {
		st = state.State{
			TaskID:           taskID,
			CurrentPhase:     p.Phases[0].ID,
			Status:           state.StatusRunning,
			CurrentIteration: 1,
			StartedAt:        time.Now(),
			Phases:           make(map[string]*state.PhaseState),
		}
	}

	// Mark task as running
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	savePath := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	s.logger.Info("saving task", "path", savePath)
	if err := t.SaveTo(savePath); err != nil {
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

		cfg := executor.DefaultConfig()
		cfg.WorkDir = projectPath
		exec := executor.New(cfg)
		exec.SetPublisher(s.publisher)

		if err := exec.ExecuteTask(ctx, t, &p, &st); err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}
	}()

	s.jsonResponse(w, map[string]any{
		"status":  "started",
		"task_id": taskID,
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

	t, err := s.loadProjectTask(proj.Path, taskID)
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
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Update state status
	statePath := filepath.Join(taskDir, "state.yaml")
	if stateData, err := os.ReadFile(statePath); err == nil {
		var st state.State
		if err := yaml.Unmarshal(stateData, &st); err == nil {
			st.Status = state.StatusPaused
			if err := st.SaveTo(taskDir); err != nil {
				s.logger.Error("failed to save state", "error", err)
			}
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

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Task must be paused to resume
	if t.Status != task.StatusPaused {
		s.jsonError(w, "task is not paused", http.StatusBadRequest)
		return
	}

	// Load plan
	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Load state
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	statePath := filepath.Join(taskDir, "state.yaml")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		s.jsonError(w, "failed to load state", http.StatusInternalServerError)
		return
	}
	var st state.State
	if err := yaml.Unmarshal(stateData, &st); err != nil {
		s.jsonError(w, "failed to parse state", http.StatusInternalServerError)
		return
	}

	// Update task status
	t.Status = task.StatusRunning
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Update state status
	st.Status = state.StatusRunning
	if st.Phases[st.CurrentPhase] != nil {
		st.Phases[st.CurrentPhase].Status = state.StatusRunning
		st.Phases[st.CurrentPhase].InterruptedAt = nil
	}
	if err := st.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update state", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Capture project path for goroutine
	projectPath := proj.Path

	s.logger.Info("resuming task execution", "task", taskID, "phase", st.CurrentPhase)

	// Start execution in background
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		cfg := executor.DefaultConfig()
		cfg.WorkDir = projectPath
		exec := executor.New(cfg)
		exec.SetPublisher(s.publisher)

		if err := exec.ExecuteTask(ctx, t, &p, &st); err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}
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

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Load plan
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	planPath := filepath.Join(taskDir, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Find target phase
	targetPhase := p.GetPhase(req.Phase)
	if targetPhase == nil {
		s.jsonError(w, "phase not found", http.StatusBadRequest)
		return
	}

	// Load state
	statePath := filepath.Join(taskDir, "state.yaml")
	stateData, err := os.ReadFile(statePath)
	if err != nil && !os.IsNotExist(err) {
		s.jsonError(w, "failed to load state", http.StatusInternalServerError)
		return
	}
	var st state.State
	if err == nil {
		yaml.Unmarshal(stateData, &st)
	}

	// Mark target and all later phases as pending
	foundTarget := false
	for i := range p.Phases {
		if p.Phases[i].ID == req.Phase {
			foundTarget = true
		}
		if foundTarget {
			p.Phases[i].Status = plan.PhasePending
			p.Phases[i].CommitSHA = ""
			if st.Phases[p.Phases[i].ID] != nil {
				st.Phases[p.Phases[i].ID].Status = state.StatusPending
				st.Phases[p.Phases[i].ID].CompletedAt = nil
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
	if err := p.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}
	if err := st.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	if err := t.SaveTo(taskDir); err != nil {
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

// handleGetProjectTaskState returns the state for a project task.
func (s *Server) handleGetProjectTaskState(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	statePath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "state.yaml")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "state not found", http.StatusNotFound)
			return
		}
		s.jsonError(w, "failed to read state", http.StatusInternalServerError)
		return
	}

	var st state.State
	if err := yaml.Unmarshal(data, &st); err != nil {
		s.jsonError(w, "failed to parse state", http.StatusInternalServerError)
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

	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	data, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "plan not found", http.StatusNotFound)
			return
		}
		s.jsonError(w, "failed to read plan", http.StatusInternalServerError)
		return
	}

	var p plan.Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
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

// loadProjectTask loads a task from a specific project path.
func (s *Server) loadProjectTask(projectPath, taskID string) (*task.Task, error) {
	taskPath := filepath.Join(projectPath, ".orc", "tasks", taskID, "task.yaml")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, err
	}

	var t task.Task
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	return &t, nil
}
