package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// makeTaskLoader creates a TaskLoader that fetches task status from task.yaml files.
func (s *Server) makeTaskLoader() initiative.TaskLoader {
	return func(taskID string) (status string, title string, err error) {
		t, err := task.LoadFrom(s.workDir, taskID)
		if err != nil {
			// Task not found or unreadable - return empty to use fallback
			return "", "", nil
		}
		return string(t.Status), t.Title, nil
	}
}

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

	// Populate computed fields (Blocks)
	initiative.PopulateComputedFields(initiatives)

	// Enrich task statuses with actual values from task.yaml files
	loader := s.makeTaskLoader()
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

	// Validate blocked_by references
	if len(req.BlockedBy) > 0 {
		allInits, err := initiative.List(req.Shared)
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

	// Auto-commit initiative creation
	s.autoCommitInitiative(init, "created")

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

	// Load all initiatives to populate computed fields
	allInits, err := initiative.List(shared)
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

	// Enrich task statuses with actual values from task.yaml files
	init.EnrichTaskStatuses(s.makeTaskLoader())

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
		allInits, err := initiative.List(shared)
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

	// Auto-commit initiative update
	s.autoCommitInitiative(init, "updated")

	// Reload all initiatives to populate computed fields for response
	allInits, err := initiative.List(shared)
	if err == nil && len(allInits) > 0 {
		for i, all := range allInits {
			if all.ID == id {
				allInits[i] = init
				break
			}
		}
		initiative.PopulateComputedFields(allInits)
	}

	// Enrich task statuses with actual values from task.yaml files
	init.EnrichTaskStatuses(s.makeTaskLoader())

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

	// Auto-commit initiative deletion
	s.autoCommitInitiativeDeletion(id)

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

	// Return tasks with actual status from task.yaml files
	tasks := init.GetTasksWithStatus(s.makeTaskLoader())
	s.jsonResponse(w, tasks)
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
	t, err := task.LoadFrom(s.workDir, req.TaskID)
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

	// Auto-commit: task added to initiative
	s.autoCommitInitiative(init, "task "+req.TaskID+" added")

	// Return tasks with actual status from task.yaml files
	tasks := init.GetTasksWithStatus(s.makeTaskLoader())
	s.jsonResponse(w, tasks)
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

	// Auto-commit: decision added
	s.autoCommitInitiative(init, "decision added")

	s.jsonResponse(w, init.Decisions)
}

// handleRemoveInitiativeTask removes a task from an initiative.
func (s *Server) handleRemoveInitiativeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	taskID := r.PathValue("taskId")
	shared := r.URL.Query().Get("shared") == "true"

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

	// Remove task
	if !init.RemoveTask(taskID) {
		s.jsonError(w, "task not found in initiative", http.StatusNotFound)
		return
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

	// Auto-commit: task removed from initiative
	s.autoCommitInitiative(init, "task "+taskID+" removed")

	w.WriteHeader(http.StatusNoContent)
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

	// Use actual task status from task.yaml files
	ready := init.GetReadyTasksWithLoader(s.makeTaskLoader())
	if ready == nil {
		ready = []initiative.TaskRef{}
	}

	s.jsonResponse(w, ready)
}

// autoCommitInitiative commits an initiative change to git if auto-commit is enabled.
// This is a non-blocking operation that logs warnings on failure.
func (s *Server) autoCommitInitiative(init *initiative.Initiative, action string) {
	// Use tasks.disable_auto_commit since there's no separate initiative setting
	if s.orcConfig == nil || s.orcConfig.Tasks.DisableAutoCommit {
		return
	}

	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		s.logger.Debug("skip initiative auto-commit: could not find project root", "error", err)
		return
	}

	commitCfg := initiative.CommitConfig{
		ProjectRoot:  projectRoot,
		CommitPrefix: s.orcConfig.CommitPrefix,
		Logger:       s.logger,
	}
	if err := initiative.CommitAndSync(init, action, commitCfg); err != nil {
		s.logger.Warn("failed to auto-commit initiative", "id", init.ID, "action", action, "error", err)
	}
}

// autoCommitInitiativeDeletion commits an initiative deletion to git if auto-commit is enabled.
func (s *Server) autoCommitInitiativeDeletion(initID string) {
	if s.orcConfig == nil || s.orcConfig.Tasks.DisableAutoCommit {
		return
	}

	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		s.logger.Debug("skip initiative auto-commit: could not find project root", "error", err)
		return
	}

	commitCfg := initiative.CommitConfig{
		ProjectRoot:  projectRoot,
		CommitPrefix: s.orcConfig.CommitPrefix,
		Logger:       s.logger,
	}
	if err := initiative.CommitDeletion(initID, commitCfg); err != nil {
		s.logger.Warn("failed to auto-commit initiative deletion", "id", initID, "error", err)
	}
}
