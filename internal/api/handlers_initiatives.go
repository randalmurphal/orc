package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// makeBatchTaskLoader creates a TaskLoader that batch-loads all tasks upfront.
// This eliminates N+1 queries when enriching initiative task statuses.
// Returns a closure that looks up tasks from the pre-loaded map.
func (s *Server) makeBatchTaskLoader() initiative.TaskLoader {
	// Load all tasks once (2 queries: tasks + dependencies)
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		// Fallback: return empty loader that won't update any statuses
		return func(taskID string) (string, string, error) {
			return "", "", nil
		}
	}

	// Build lookup map
	taskMap := make(map[string]*task.Task, len(allTasks))
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	return func(taskID string) (status string, title string, err error) {
		if t, ok := taskMap[taskID]; ok {
			return string(t.Status), t.Title, nil
		}
		return "", "", nil
	}
}

// handleListInitiatives returns all initiatives.
func (s *Server) handleListInitiatives(w http.ResponseWriter, r *http.Request) {
	// Note: status filter and shared parameter are ignored - all initiatives come from backend
	// Filter by status can be done in-memory if needed
	_ = r.URL.Query().Get("status")
	_ = r.URL.Query().Get("shared")

	initiatives, err := s.backend.LoadAllInitiatives()
	if err != nil {
		s.jsonError(w, "failed to load initiatives", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if initiatives == nil {
		initiatives = []*initiative.Initiative{}
	}

	// Populate computed fields (Blocks)
	initiative.PopulateComputedFields(initiatives)

	// Enrich task statuses with actual values from database
	loader := s.makeBatchTaskLoader()
	for _, init := range initiatives {
		init.EnrichTaskStatuses(loader)
	}

	s.jsonResponse(w, initiatives)
}

// handleCreateInitiative creates a new initiative.
func (s *Server) handleCreateInitiative(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string   `json:"title"`
		Vision    string   `json:"vision,omitempty"`
		BlockedBy []string `json:"blocked_by,omitempty"`
		Owner     struct {
			Initials    string `json:"initials,omitempty"`
			DisplayName string `json:"display_name,omitempty"`
			Email       string `json:"email,omitempty"`
		} `json:"owner,omitempty"`
		Shared bool `json:"shared,omitempty"` // Ignored - all initiatives stored in DB
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
	id, err := s.backend.GetNextInitiativeID()
	if err != nil {
		s.jsonError(w, "failed to generate initiative ID", http.StatusInternalServerError)
		return
	}

	// Validate blocked_by references
	if len(req.BlockedBy) > 0 {
		allInits, err := s.backend.LoadAllInitiatives()
		if err != nil {
			s.jsonError(w, "failed to load initiatives for validation", http.StatusInternalServerError)
			return
		}
		existingIDs := make(map[string]bool)
		for _, init := range allInits {
			existingIDs[init.ID] = true
		}
		if errs := initiative.ValidateBlockedBy(id, req.BlockedBy, existingIDs); len(errs) > 0 {
			s.jsonError(w, errs[0].Error(), http.StatusBadRequest)
			return
		}
	}

	// Create initiative
	init := initiative.New(id, req.Title)
	init.Vision = req.Vision
	init.BlockedBy = req.BlockedBy
	if req.Owner.Initials != "" || req.Owner.DisplayName != "" || req.Owner.Email != "" {
		init.Owner = initiative.Identity{
			Initials:    req.Owner.Initials,
			DisplayName: req.Owner.DisplayName,
			Email:       req.Owner.Email,
		}
	}

	// Save to database
	if err := s.backend.SaveInitiative(init); err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, init)
}

// handleGetInitiative returns a specific initiative.
func (s *Server) handleGetInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Load all initiatives to populate computed fields
	allInits, err := s.backend.LoadAllInitiatives()
	if err == nil && len(allInits) > 0 {
		// Build map to find our initiative in the list
		for i, all := range allInits {
			if all.ID == id {
				allInits[i] = init // Use the already loaded one
				break
			}
		}
		initiative.PopulateComputedFields(allInits)
	}

	// Enrich task statuses with actual values from database
	init.EnrichTaskStatuses(s.makeBatchTaskLoader())

	s.jsonResponse(w, init)
}

// handleUpdateInitiative updates an initiative.
func (s *Server) handleUpdateInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	// Load existing initiative
	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Parse update request
	var req struct {
		Title     string    `json:"title,omitempty"`
		Vision    string    `json:"vision,omitempty"`
		Status    string    `json:"status,omitempty"`
		BlockedBy *[]string `json:"blocked_by,omitempty"`
		Owner     *struct {
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

	// Handle blocked_by update
	if req.BlockedBy != nil {
		allInits, err := s.backend.LoadAllInitiatives()
		if err != nil {
			s.jsonError(w, "failed to load initiatives for validation", http.StatusInternalServerError)
			return
		}
		initMap := make(map[string]*initiative.Initiative)
		for _, i := range allInits {
			initMap[i.ID] = i
		}
		if err := init.SetBlockedBy(*req.BlockedBy, initMap); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Save to database
	if err := s.backend.SaveInitiative(init); err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	// Reload all initiatives to populate computed fields for response
	allInits, err := s.backend.LoadAllInitiatives()
	if err == nil && len(allInits) > 0 {
		for i, all := range allInits {
			if all.ID == id {
				allInits[i] = init
				break
			}
		}
		initiative.PopulateComputedFields(allInits)
	}

	// Enrich task statuses with actual values from database
	init.EnrichTaskStatuses(s.makeBatchTaskLoader())

	s.jsonResponse(w, init)
}

// handleDeleteInitiative deletes an initiative.
func (s *Server) handleDeleteInitiative(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	// Check if initiative exists by trying to load it
	_, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	if err := s.backend.DeleteInitiative(id); err != nil {
		s.jsonError(w, "failed to delete initiative", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListInitiativeTasks returns tasks linked to an initiative.
func (s *Server) handleListInitiativeTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Return tasks with actual status from database
	tasks := init.GetTasksWithStatus(s.makeBatchTaskLoader())
	s.jsonResponse(w, tasks)
}

// handleAddInitiativeTask links a task to an initiative.
func (s *Server) handleAddInitiativeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

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
	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Load task to get title
	t, err := s.backend.LoadTask(req.TaskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Add task
	init.AddTask(req.TaskID, t.Title, req.DependsOn)

	// Save to database
	if err := s.backend.SaveInitiative(init); err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	// Return tasks with actual status from database
	tasks := init.GetTasksWithStatus(s.makeBatchTaskLoader())
	s.jsonResponse(w, tasks)
}

// handleAddInitiativeDecision adds a decision to an initiative.
func (s *Server) handleAddInitiativeDecision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

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
	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Add decision
	init.AddDecision(req.Decision, req.Rationale, req.By)

	// Save to database
	if err := s.backend.SaveInitiative(init); err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, init.Decisions)
}

// handleRemoveInitiativeTask removes a task from an initiative.
func (s *Server) handleRemoveInitiativeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	taskID := r.PathValue("taskId")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	// Load initiative
	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Remove task
	if !init.RemoveTask(taskID) {
		s.jsonError(w, "task not found in initiative", http.StatusNotFound)
		return
	}

	// Save to database
	if err := s.backend.SaveInitiative(init); err != nil {
		s.jsonError(w, "failed to save initiative", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetReadyTasks returns tasks that are ready to run (all deps satisfied).
func (s *Server) handleGetReadyTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives come from backend
	_ = r.URL.Query().Get("shared")

	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Use actual task status from database
	ready := init.GetReadyTasksWithLoader(s.makeBatchTaskLoader())
	if ready == nil {
		ready = []initiative.TaskRef{}
	}

	s.jsonResponse(w, ready)
}
