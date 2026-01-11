package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// handleListTasks returns all tasks with optional pagination.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := task.LoadAll()
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
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

// handleCreateTask creates a new task.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
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

	id, err := task.NextID()
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

	if err := t.Save(); err != nil {
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

	// Save plan
	if err := p.Save(id); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}

	// Update task status to planned
	t.Status = task.StatusPlanned
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, t)
}

// handleGetTask returns a specific task.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
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
	t, err := task.Load(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	// Delete task
	if err := task.Delete(id); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete task: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
