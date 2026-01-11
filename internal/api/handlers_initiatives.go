package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// handleListInitiatives returns all initiatives.
func (s *Server) handleListInitiatives(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	shared := r.URL.Query().Get("shared") == "true"

	var initiatives []*initiative.Initiative
	var err error

	if status != "" {
		initiatives, err = initiative.ListByStatus(initiative.Status(status), shared)
	} else {
		initiatives, err = initiative.List(shared)
	}
	if err != nil {
		s.jsonError(w, "failed to load initiatives", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if initiatives == nil {
		initiatives = []*initiative.Initiative{}
	}

	s.jsonResponse(w, initiatives)
}

// handleCreateInitiative creates a new initiative.
func (s *Server) handleCreateInitiative(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title  string `json:"title"`
		Vision string `json:"vision,omitempty"`
		Owner  struct {
			Initials    string `json:"initials,omitempty"`
			DisplayName string `json:"display_name,omitempty"`
			Email       string `json:"email,omitempty"`
		} `json:"owner,omitempty"`
		Shared bool `json:"shared,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		s.jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	// Generate next initiative ID
	id, err := initiative.NextID(req.Shared)
	if err != nil {
		s.jsonError(w, "failed to generate initiative ID", http.StatusInternalServerError)
		return
	}

	// Create initiative
	init := initiative.New(id, req.Title)
	init.Vision = req.Vision
	if req.Owner.Initials != "" || req.Owner.DisplayName != "" || req.Owner.Email != "" {
		init.Owner = initiative.Identity{
			Initials:    req.Owner.Initials,
			DisplayName: req.Owner.DisplayName,
			Email:       req.Owner.Email,
		}
	}

	// Save
	var saveErr error
	if req.Shared {
		saveErr = init.SaveShared()
	} else {
		saveErr = init.Save()
	}
	if saveErr != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, init)
}

// handleGetInitiative returns a specific initiative.
func (s *Server) handleGetInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, init)
}

// handleUpdateInitiative updates an initiative.
func (s *Server) handleUpdateInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	// Load existing initiative
	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Parse update request
	var req struct {
		Title  string `json:"title,omitempty"`
		Vision string `json:"vision,omitempty"`
		Status string `json:"status,omitempty"`
		Owner  *struct {
			Initials    string `json:"initials,omitempty"`
			DisplayName string `json:"display_name,omitempty"`
			Email       string `json:"email,omitempty"`
		} `json:"owner,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Apply updates
	if req.Title != "" {
		init.Title = req.Title
	}
	if req.Vision != "" {
		init.Vision = req.Vision
	}
	if req.Status != "" {
		init.Status = initiative.Status(req.Status)
	}
	if req.Owner != nil {
		init.Owner = initiative.Identity{
			Initials:    req.Owner.Initials,
			DisplayName: req.Owner.DisplayName,
			Email:       req.Owner.Email,
		}
	}

	// Save
	if shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, init)
}

// handleDeleteInitiative deletes an initiative.
func (s *Server) handleDeleteInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	if !initiative.Exists(id, shared) {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	if err := initiative.Delete(id, shared); err != nil {
		s.jsonError(w, "failed to delete initiative", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListInitiativeTasks returns tasks linked to an initiative.
func (s *Server) handleListInitiativeTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, init.Tasks)
}

// handleAddInitiativeTask links a task to an initiative.
func (s *Server) handleAddInitiativeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	var req struct {
		TaskID    string   `json:"task_id"`
		DependsOn []string `json:"depends_on,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		s.jsonError(w, "task_id is required", http.StatusBadRequest)
		return
	}

	// Load initiative
	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Load task to get title
	t, err := task.Load(req.TaskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Add task
	init.AddTask(req.TaskID, t.Title, req.DependsOn)

	// Save
	if shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, init.Tasks)
}

// handleAddInitiativeDecision adds a decision to an initiative.
func (s *Server) handleAddInitiativeDecision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	var req struct {
		Decision  string `json:"decision"`
		Rationale string `json:"rationale,omitempty"`
		By        string `json:"by,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Decision == "" {
		s.jsonError(w, "decision is required", http.StatusBadRequest)
		return
	}

	// Load initiative
	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Add decision
	init.AddDecision(req.Decision, req.Rationale, req.By)

	// Save
	if shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, init.Decisions)
}

// handleGetReadyTasks returns tasks that are ready to run (all deps satisfied).
func (s *Server) handleGetReadyTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	shared := r.URL.Query().Get("shared") == "true"

	var init *initiative.Initiative
	var err error
	if shared {
		init, err = initiative.LoadShared(id)
	} else {
		init, err = initiative.Load(id)
	}
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	ready := init.GetReadyTasks()
	if ready == nil {
		ready = []initiative.TaskRef{}
	}

	s.jsonResponse(w, ready)
}
