package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// splitAndTrim splits a comma-separated string and trims whitespace from each element.
// Returns nil for empty input.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

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

// handleListTasks returns all tasks with optional pagination and filtering.
// Query params:
//   - initiative: filter by initiative ID (e.g., ?initiative=INIT-001)
//   - dependency_status: filter by dependency status (blocked, ready, none)
//   - page: page number for pagination
//   - limit: items per page (max 100)
//
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

	// Filter by initiative if requested
	initiativeFilter := r.URL.Query().Get("initiative")
	if initiativeFilter != "" {
		var filtered []*task.Task
		for _, t := range tasks {
			if t.InitiativeID == initiativeFilter {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
		// Ensure we return an empty array after filtering, not null
		if tasks == nil {
			tasks = []*task.Task{}
		}
	}

	// Populate computed dependency fields (Blocks, ReferencedBy, DependencyStatus)
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
	Title        string
	Description  string
	Weight       string
	Queue        string
	Priority     string
	Category     string
	InitiativeID string
	BlockedBy    []string
	RelatedTo    []string
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

		// Parse comma-separated blocked_by and related_to from form values
		var blockedBy, relatedTo []string
		if val := r.FormValue("blocked_by"); val != "" {
			blockedBy = splitAndTrim(val)
		}
		if val := r.FormValue("related_to"); val != "" {
			relatedTo = splitAndTrim(val)
		}

		return &createTaskRequest{
			Title:        r.FormValue("title"),
			Description:  r.FormValue("description"),
			Weight:       r.FormValue("weight"),
			Queue:        r.FormValue("queue"),
			Priority:     r.FormValue("priority"),
			Category:     r.FormValue("category"),
			InitiativeID: r.FormValue("initiative_id"),
			BlockedBy:    blockedBy,
			RelatedTo:    relatedTo,
		}, true, nil
	}

	// Default: parse as JSON
	var req struct {
		Title        string   `json:"title"`
		Description  string   `json:"description,omitempty"`
		Weight       string   `json:"weight,omitempty"`
		Queue        string   `json:"queue,omitempty"`
		Priority     string   `json:"priority,omitempty"`
		Category     string   `json:"category,omitempty"`
		InitiativeID string   `json:"initiative_id,omitempty"`
		BlockedBy    []string `json:"blocked_by,omitempty"`
		RelatedTo    []string `json:"related_to,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, false, fmt.Errorf("invalid request body: %w", err)
	}

	return &createTaskRequest{
		Title:        req.Title,
		Description:  req.Description,
		Weight:       req.Weight,
		Queue:        req.Queue,
		Priority:     req.Priority,
		Category:     req.Category,
		InitiativeID: req.InitiativeID,
		BlockedBy:    req.BlockedBy,
		RelatedTo:    req.RelatedTo,
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

	// Link to initiative if specified
	if req.InitiativeID != "" {
		// Verify initiative exists
		if !initiative.Exists(req.InitiativeID, false) {
			s.jsonError(w, fmt.Sprintf("initiative %s not found", req.InitiativeID), http.StatusBadRequest)
			return
		}
		t.SetInitiative(req.InitiativeID)
	}

	// Set dependencies
	if len(req.BlockedBy) > 0 || len(req.RelatedTo) > 0 {
		// Build map of existing task IDs for validation
		existingTasks, err := task.LoadAllFrom(tasksDir)
		if err != nil {
			s.jsonError(w, "failed to load existing tasks for validation", http.StatusInternalServerError)
			return
		}
		existingIDs := make(map[string]bool)
		for _, existing := range existingTasks {
			existingIDs[existing.ID] = true
		}

		// Validate blocked_by references
		if errs := task.ValidateBlockedBy(id, req.BlockedBy, existingIDs); len(errs) > 0 {
			s.jsonError(w, errs[0].Error(), http.StatusBadRequest)
			return
		}

		// Validate related_to references
		if errs := task.ValidateRelatedTo(id, req.RelatedTo, existingIDs); len(errs) > 0 {
			s.jsonError(w, errs[0].Error(), http.StatusBadRequest)
			return
		}

		t.BlockedBy = req.BlockedBy
		t.RelatedTo = req.RelatedTo
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

	// Sync task to initiative if linked
	if t.HasInitiative() {
		init, err := initiative.Load(t.InitiativeID)
		if err == nil {
			init.AddTask(t.ID, t.Title, nil)
			if err := init.Save(); err != nil {
				s.logger.Warn("failed to sync task to initiative",
					"taskID", id,
					"initiativeID", t.InitiativeID,
					"error", err,
				)
			}
		}
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

	// Load all tasks to compute Blocks, ReferencedBy, and DependencyStatus
	tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
	allTasks, err := task.LoadAllFrom(tasksDir)
	if err == nil && len(allTasks) > 0 {
		// Build task map for dependency checking
		taskMap := make(map[string]*task.Task)
		for _, other := range allTasks {
			taskMap[other.ID] = other
		}

		t.Blocks = task.ComputeBlocks(t.ID, allTasks)
		t.ReferencedBy = task.ComputeReferencedBy(t.ID, allTasks)
		t.UnmetBlockers = t.GetUnmetDependencies(taskMap)
		t.IsBlocked = len(t.UnmetBlockers) > 0
		t.DependencyStatus = t.ComputeDependencyStatus()
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

	// Remove from initiative if linked
	if t.HasInitiative() {
		if init, err := initiative.Load(t.InitiativeID); err == nil {
			init.RemoveTask(t.ID)
			if err := init.Save(); err != nil {
				s.logger.Warn("failed to remove task from initiative on delete",
					"taskID", id,
					"initiativeID", t.InitiativeID,
					"error", err,
				)
			}
		}
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
		Title        *string           `json:"title,omitempty"`
		Description  *string           `json:"description,omitempty"`
		Weight       *string           `json:"weight,omitempty"`
		Queue        *string           `json:"queue,omitempty"`
		Priority     *string           `json:"priority,omitempty"`
		Category     *string           `json:"category,omitempty"`
		InitiativeID *string           `json:"initiative_id,omitempty"`
		BlockedBy    *[]string         `json:"blocked_by,omitempty"`
		RelatedTo    *[]string         `json:"related_to,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
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

	// Track initiative change for bidirectional sync
	oldInitiative := t.InitiativeID
	initiativeChanged := false

	if req.InitiativeID != nil {
		// Empty string means unlink
		if *req.InitiativeID != "" {
			// Verify initiative exists
			if !initiative.Exists(*req.InitiativeID, false) {
				s.jsonError(w, fmt.Sprintf("initiative %s not found", *req.InitiativeID), http.StatusBadRequest)
				return
			}
		}
		if t.InitiativeID != *req.InitiativeID {
			t.SetInitiative(*req.InitiativeID)
			initiativeChanged = true
		}
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

	// Handle dependency updates
	if req.BlockedBy != nil || req.RelatedTo != nil {
		// Build map of existing task IDs for validation
		tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
		existingTasks, err := task.LoadAllFrom(tasksDir)
		if err != nil {
			s.jsonError(w, "failed to load existing tasks for validation", http.StatusInternalServerError)
			return
		}
		existingIDs := make(map[string]bool)
		taskMap := make(map[string]*task.Task)
		for _, existing := range existingTasks {
			existingIDs[existing.ID] = true
			taskMap[existing.ID] = existing
		}

		if req.BlockedBy != nil {
			// Validate blocked_by references
			if errs := task.ValidateBlockedBy(id, *req.BlockedBy, existingIDs); len(errs) > 0 {
				s.jsonError(w, errs[0].Error(), http.StatusBadRequest)
				return
			}

			// Check for circular dependencies with all new blockers at once
			// This catches cases where individual blockers are fine but combined they create a cycle
			if cycle := task.DetectCircularDependencyWithAll(id, *req.BlockedBy, taskMap); cycle != nil {
				s.jsonError(w, fmt.Sprintf("circular dependency detected: %s", strings.Join(cycle, " -> ")), http.StatusBadRequest)
				return
			}

			t.BlockedBy = *req.BlockedBy
		}

		if req.RelatedTo != nil {
			// Validate related_to references
			if errs := task.ValidateRelatedTo(id, *req.RelatedTo, existingIDs); len(errs) > 0 {
				s.jsonError(w, errs[0].Error(), http.StatusBadRequest)
				return
			}
			t.RelatedTo = *req.RelatedTo
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

	// Handle initiative change - sync bidirectionally
	if initiativeChanged {
		// Remove from old initiative if it was linked
		if oldInitiative != "" {
			if oldInit, err := initiative.Load(oldInitiative); err == nil {
				oldInit.RemoveTask(t.ID)
				if err := oldInit.Save(); err != nil {
					s.logger.Warn("failed to remove task from old initiative",
						"taskID", id,
						"initiativeID", oldInitiative,
						"error", err,
					)
				}
			}
		}
		// Add to new initiative if linking
		if t.HasInitiative() {
			if newInit, err := initiative.Load(t.InitiativeID); err == nil {
				newInit.AddTask(t.ID, t.Title, nil)
				if err := newInit.Save(); err != nil {
					s.logger.Warn("failed to add task to new initiative",
						"taskID", id,
						"initiativeID", t.InitiativeID,
						"error", err,
					)
				}
			}
		}
	}

	s.jsonResponse(w, t)
}

// DependencyGraph represents the dependency relationships for a task.
type DependencyGraph struct {
	TaskID       string           `json:"task_id"`
	BlockedBy    []DependencyInfo `json:"blocked_by"`
	Blocks       []DependencyInfo `json:"blocks"`
	RelatedTo    []DependencyInfo `json:"related_to"`
	ReferencedBy []DependencyInfo `json:"referenced_by"`
	UnmetDeps    []string         `json:"unmet_dependencies,omitempty"`
	CanRun       bool             `json:"can_run"`
}

// DependencyInfo provides details about a dependency.
type DependencyInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	IsMet  bool   `json:"is_met,omitempty"`
}

// handleGetDependencies returns the full dependency graph for a task.
func (s *Server) handleGetDependencies(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Load all tasks to build the graph
	tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
	allTasks, err := task.LoadAllFrom(tasksDir)
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Build task map for lookups
	taskMap := make(map[string]*task.Task)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	// Helper to create DependencyInfo
	toInfo := func(taskID string, checkMet bool) DependencyInfo {
		info := DependencyInfo{ID: taskID}
		if dep, exists := taskMap[taskID]; exists {
			info.Title = dep.Title
			info.Status = string(dep.Status)
			if checkMet {
				info.IsMet = dep.Status == task.StatusCompleted
			}
		} else {
			info.Title = "(not found)"
			info.Status = "unknown"
			if checkMet {
				info.IsMet = false
			}
		}
		return info
	}

	// Build dependency info lists
	blockedBy := make([]DependencyInfo, 0, len(t.BlockedBy))
	for _, depID := range t.BlockedBy {
		blockedBy = append(blockedBy, toInfo(depID, true))
	}

	blocks := make([]DependencyInfo, 0)
	for _, other := range allTasks {
		for _, blocker := range other.BlockedBy {
			if blocker == id {
				blocks = append(blocks, toInfo(other.ID, false))
				break
			}
		}
	}

	relatedTo := make([]DependencyInfo, 0, len(t.RelatedTo))
	for _, relID := range t.RelatedTo {
		relatedTo = append(relatedTo, toInfo(relID, false))
	}

	referencedBy := make([]DependencyInfo, 0)
	refs := task.ComputeReferencedBy(id, allTasks)
	for _, refID := range refs {
		referencedBy = append(referencedBy, toInfo(refID, false))
	}

	// Check for unmet dependencies
	unmet := t.GetUnmetDependencies(taskMap)

	graph := DependencyGraph{
		TaskID:       id,
		BlockedBy:    blockedBy,
		Blocks:       blocks,
		RelatedTo:    relatedTo,
		ReferencedBy: referencedBy,
		UnmetDeps:    unmet,
		CanRun:       len(unmet) == 0,
	}

	s.jsonResponse(w, graph)
}
