package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/randalmurphal/orc/internal/db"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// syncTaskToDB ensures a task exists in the database by loading from YAML if needed.
// This is used for foreign key constraints (e.g., review_comments references tasks).
func (s *Server) syncTaskToDB(pdb *db.ProjectDB, taskID string) error {
	// Check if task already exists in database
	existing, err := pdb.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("check task in db: %w", err)
	}
	if existing != nil {
		return nil // Task already synced
	}

	// Load from YAML and sync to database
	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		return fmt.Errorf("load task from yaml: %w", err)
	}

	dbTask := &db.Task{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Weight:       string(t.Weight),
		Status:       string(t.Status),
		CurrentPhase: t.CurrentPhase,
		Branch:       t.Branch,
		Queue:        string(t.GetQueue()),
		Priority:     string(t.GetPriority()),
		Category:     string(t.GetCategory()),
		CreatedAt:    t.CreatedAt,
		StartedAt:    t.StartedAt,
		CompletedAt:  t.CompletedAt,
	}

	if err := pdb.SaveTask(dbTask); err != nil {
		return fmt.Errorf("sync task to db: %w", err)
	}

	return nil
}

// handleListTasks returns all tasks with optional pagination.
// Note: This endpoint uses the server's workDir which may not be a valid orc project.
// Prefer using /api/projects/{id}/tasks for explicit project-scoped operations.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
	tasks, err := task.LoadAllFrom(tasksDir)
	if err != nil {
		// If the tasks directory doesn't exist, return empty list
		// This handles the case where server is started from a non-project directory
		s.jsonResponse(w, []*task.Task{})
		return
	}

	// Ensure we return an empty array, not null
	if tasks == nil {
		tasks = []*task.Task{}
	}

	// Check for pagination params
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// If no pagination requested, return all tasks (backward compatible)
	if pageStr == "" && limitStr == "" {
		s.jsonResponse(w, tasks)
		return
	}

	// Parse pagination params
	page := 1
	limit := 20 // default limit
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Calculate pagination
	total := len(tasks)
	totalPages := (total + limit - 1) / limit
	start := (page - 1) * limit
	end := start + limit

	// Bounds checking
	if start >= total {
		start = total
		end = total
	}
	if end > total {
		end = total
	}

	pagedTasks := tasks[start:end]

	s.jsonResponse(w, map[string]any{
		"tasks":       pagedTasks,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// createTaskRequest holds the parsed task creation request parameters.
type createTaskRequest struct {
	Title       string
	Description string
	Weight      string
	Queue       string
	Priority    string
	Category    string
}

// parseCreateTaskRequest parses the task creation request from either JSON or multipart form.
// Returns the request parameters, form (if multipart), and any error.
func (s *Server) parseCreateTaskRequest(r *http.Request) (*createTaskRequest, bool, error) {
	contentType := r.Header.Get("Content-Type")

	// Check if this is a multipart form request
	if contentType != "" && len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Parse multipart form (max 32MB)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, false, fmt.Errorf("failed to parse form: %w", err)
		}

		return &createTaskRequest{
			Title:       r.FormValue("title"),
			Description: r.FormValue("description"),
			Weight:      r.FormValue("weight"),
			Queue:       r.FormValue("queue"),
			Priority:    r.FormValue("priority"),
			Category:    r.FormValue("category"),
		}, true, nil
	}

	// Default: parse as JSON
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		Weight      string `json:"weight,omitempty"`
		Queue       string `json:"queue,omitempty"`
		Priority    string `json:"priority,omitempty"`
		Category    string `json:"category,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, false, fmt.Errorf("invalid request body: %w", err)
	}

	return &createTaskRequest{
		Title:       req.Title,
		Description: req.Description,
		Weight:      req.Weight,
		Queue:       req.Queue,
		Priority:    req.Priority,
		Category:    req.Category,
	}, false, nil
}

// handleCreateTask creates a new task.
// Supports both JSON and multipart/form-data content types.
// With multipart/form-data, files can be attached via "attachments" field.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	req, isMultipart, err := s.parseCreateTaskRequest(r)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		s.jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
	id, err := task.NextIDIn(tasksDir)
	if err != nil {
		s.jsonError(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.New(id, req.Title)
	t.Description = req.Description
	if req.Weight != "" {
		t.Weight = task.Weight(req.Weight)
	} else {
		// Default to medium if not specified
		t.Weight = task.WeightMedium
	}

	// Set queue (defaults to active)
	if req.Queue != "" {
		queue := task.Queue(req.Queue)
		if !task.IsValidQueue(queue) {
			s.jsonError(w, fmt.Sprintf("invalid queue: %s (valid: active, backlog)", req.Queue), http.StatusBadRequest)
			return
		}
		t.Queue = queue
	}

	// Set priority (defaults to normal)
	if req.Priority != "" {
		priority := task.Priority(req.Priority)
		if !task.IsValidPriority(priority) {
			s.jsonError(w, fmt.Sprintf("invalid priority: %s (valid: critical, high, normal, low)", req.Priority), http.StatusBadRequest)
			return
		}
		t.Priority = priority
	}

	// Set category (defaults to feature)
	if req.Category != "" {
		category := task.Category(req.Category)
		if !task.IsValidCategory(category) {
			s.jsonError(w, fmt.Sprintf("invalid category: %s (valid: feature, bug, refactor, chore, docs, test)", req.Category), http.StatusBadRequest)
			return
		}
		t.Category = category
	}

	taskDir := task.TaskDirIn(s.workDir, id)
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	// Create plan from template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
		// If template not found, use default plan
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

	// Save plan to taskDir
	if err := p.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}

	// Update task status to planned
	t.Status = task.StatusPlanned
	if err := t.SaveTo(taskDir); err != nil {
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
			_, err = task.SaveAttachment(s.workDir, id, filename, file)
			file.Close()
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

// handleGetTask returns a specific task.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	s.jsonResponse(w, t)
}

// handleDeleteTask deletes a task.
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check if task is running
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	// Delete task
	if err := task.DeleteIn(s.workDir, id); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete task: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateTask updates task fields (title, description, weight, queue, priority).
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Load existing task
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Cannot update running tasks
	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot update running task", http.StatusConflict)
		return
	}

	// Parse request body
	var req struct {
		Title       *string           `json:"title,omitempty"`
		Description *string           `json:"description,omitempty"`
		Weight      *string           `json:"weight,omitempty"`
		Queue       *string           `json:"queue,omitempty"`
		Priority    *string           `json:"priority,omitempty"`
		Category    *string           `json:"category,omitempty"`
		Metadata    map[string]string `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Track if weight changed
	oldWeight := t.Weight
	weightChanged := false

	// Apply updates (only update fields that are provided)
	if req.Title != nil {
		if *req.Title == "" {
			s.jsonError(w, "title cannot be empty", http.StatusBadRequest)
			return
		}
		t.Title = *req.Title
	}

	if req.Description != nil {
		t.Description = *req.Description
	}

	if req.Weight != nil {
		weight := task.Weight(*req.Weight)
		if !task.IsValidWeight(weight) {
			s.jsonError(w, fmt.Sprintf("invalid weight: %s", *req.Weight), http.StatusBadRequest)
			return
		}
		if t.Weight != weight {
			t.Weight = weight
			weightChanged = true
		}
	}

	if req.Queue != nil {
		queue := task.Queue(*req.Queue)
		if !task.IsValidQueue(queue) {
			s.jsonError(w, fmt.Sprintf("invalid queue: %s (valid: active, backlog)", *req.Queue), http.StatusBadRequest)
			return
		}
		t.Queue = queue
	}

	if req.Priority != nil {
		priority := task.Priority(*req.Priority)
		if !task.IsValidPriority(priority) {
			s.jsonError(w, fmt.Sprintf("invalid priority: %s (valid: critical, high, normal, low)", *req.Priority), http.StatusBadRequest)
			return
		}
		t.Priority = priority
	}

	if req.Category != nil {
		category := task.Category(*req.Category)
		if !task.IsValidCategory(category) {
			s.jsonError(w, fmt.Sprintf("invalid category: %s (valid: feature, bug, refactor, chore, docs, test)", *req.Category), http.StatusBadRequest)
			return
		}
		t.Category = category
	}

	if req.Metadata != nil {
		if t.Metadata == nil {
			t.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			if v == "" {
				delete(t.Metadata, k)
			} else {
				t.Metadata[k] = v
			}
		}
	}

	// Save updated task
	taskDir := task.TaskDirIn(s.workDir, id)
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	// Regenerate plan if weight changed
	if weightChanged {
		result, err := plan.RegeneratePlanForTask(s.workDir, t)
		if err != nil {
			// Plan regeneration failed - return error to client
			// The task has been saved with new weight, but plan is stale
			s.logger.Error("failed to regenerate plan for weight change",
				"taskID", id,
				"oldWeight", oldWeight,
				"newWeight", t.Weight,
				"error", err,
			)
			s.jsonError(w, fmt.Sprintf("task updated but plan regeneration failed: %v", err), http.StatusInternalServerError)
			return
		}
		s.logger.Info("plan regenerated for weight change",
			"taskID", id,
			"oldWeight", oldWeight,
			"newWeight", t.Weight,
			"preservedPhases", result.PreservedPhases,
			"resetPhases", result.ResetPhases,
		)
	}

	s.jsonResponse(w, t)
}
