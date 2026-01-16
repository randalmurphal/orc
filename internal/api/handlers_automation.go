package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TriggerResponse represents a trigger in API responses.
type TriggerResponse struct {
	ID              string                        `json:"id"`
	Type            string                        `json:"type"`
	Description     string                        `json:"description"`
	Enabled         bool                          `json:"enabled"`
	Mode            string                        `json:"mode"`
	Condition       config.TriggerConditionConfig `json:"condition"`
	Action          config.TriggerActionConfig    `json:"action"`
	Cooldown        config.TriggerCooldownConfig  `json:"cooldown"`
	LastTriggeredAt string                        `json:"last_triggered_at,omitempty"`
	TriggerCount    int                           `json:"trigger_count"`
}

// ExecutionResponse represents a trigger execution in API responses.
type ExecutionResponse struct {
	ID            int64  `json:"id"`
	TriggerID     string `json:"trigger_id"`
	TaskID        string `json:"task_id,omitempty"`
	TriggeredAt   string `json:"triggered_at"`
	TriggerReason string `json:"trigger_reason"`
	Status        string `json:"status"`
	CompletedAt   string `json:"completed_at,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// AutomationStatsResponse represents automation statistics.
type AutomationStatsResponse struct {
	TotalTriggers   int `json:"total_triggers"`
	EnabledTriggers int `json:"enabled_triggers"`
	PendingTasks    int `json:"pending_tasks"`
	RunningTasks    int `json:"running_tasks"`
	CompletedTasks  int `json:"completed_tasks"`
}

// handleListTriggers returns all configured triggers.
// GET /api/automation/triggers
func (s *Server) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	if s.orcConfig == nil {
		s.jsonError(w, "configuration not loaded", http.StatusInternalServerError)
		return
	}

	triggers := s.orcConfig.Automation.Triggers
	resp := make([]TriggerResponse, 0, len(triggers))

	for _, t := range triggers {
		resp = append(resp, triggerConfigToResponse(&t))
	}

	s.jsonResponse(w, resp)
}

// handleGetTrigger returns a specific trigger by ID.
// GET /api/automation/triggers/{id}
func (s *Server) handleGetTrigger(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/automation/triggers/")
	// Handle sub-paths
	if idx := strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}

	if id == "" {
		s.jsonError(w, "trigger ID required", http.StatusBadRequest)
		return
	}

	trigger := s.findTrigger(id)
	if trigger == nil {
		s.jsonError(w, "trigger not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, triggerConfigToResponse(trigger))
}

// handleUpdateTrigger updates a trigger (enable/disable).
// PUT /api/automation/triggers/{id}
func (s *Server) handleUpdateTrigger(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/automation/triggers/")

	if id == "" {
		s.jsonError(w, "trigger ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Find the trigger in config
	var trigger *config.TriggerConfig
	var triggerIndex int
	for i := range s.orcConfig.Automation.Triggers {
		if s.orcConfig.Automation.Triggers[i].ID == id {
			trigger = &s.orcConfig.Automation.Triggers[i]
			triggerIndex = i
			break
		}
	}

	if trigger == nil {
		s.jsonError(w, "trigger not found", http.StatusNotFound)
		return
	}

	if req.Enabled != nil {
		// Persist to database first
		if s.automationSvc != nil {
			if err := s.automationSvc.SetTriggerEnabled(r.Context(), id, *req.Enabled); err != nil {
				s.logger.Error("failed to persist trigger enabled state", "trigger", id, "error", err)
				s.jsonError(w, "failed to update trigger", http.StatusInternalServerError)
				return
			}
		}

		// Update in-memory config after successful DB write
		s.orcConfig.Automation.Triggers[triggerIndex].Enabled = *req.Enabled
		s.logger.Info("trigger enabled state changed",
			"trigger", id,
			"enabled", *req.Enabled)
	}

	s.jsonResponse(w, triggerConfigToResponse(&s.orcConfig.Automation.Triggers[triggerIndex]))
}

// handleRunTrigger manually fires a trigger.
// POST /api/automation/triggers/{id}/run
func (s *Server) handleRunTrigger(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/automation/triggers/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "run" {
		s.jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Validate trigger ID format (alphanumeric, dashes, underscores only)
	if !isValidTriggerID(id) {
		s.jsonError(w, "invalid trigger ID format", http.StatusBadRequest)
		return
	}

	if s.automationSvc == nil {
		s.jsonError(w, "automation service not enabled", http.StatusServiceUnavailable)
		return
	}

	trigger := s.findTrigger(id)
	if trigger == nil {
		s.jsonError(w, "trigger not found", http.StatusNotFound)
		return
	}

	// Run the trigger directly (bypasses condition evaluation)
	if err := s.automationSvc.RunTrigger(r.Context(), id); err != nil {
		s.logger.Error("failed to run trigger", "trigger", id, "error", err)
		// Return generic error message to avoid information disclosure
		s.jsonError(w, "failed to run trigger", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{
		"status":  "triggered",
		"trigger": id,
	})
}

// isValidTriggerID validates that a trigger ID contains only safe characters.
func isValidTriggerID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') &&
			(c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// handleGetTriggerHistory returns execution history for a trigger.
// GET /api/automation/triggers/{id}/history
func (s *Server) handleGetTriggerHistory(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/automation/triggers/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "history" {
		s.jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Validate trigger ID format
	if !isValidTriggerID(id) {
		s.jsonError(w, "invalid trigger ID format", http.StatusBadRequest)
		return
	}

	if s.automationSvc == nil {
		s.jsonError(w, "automation service not enabled", http.StatusServiceUnavailable)
		return
	}

	// Verify trigger exists
	trigger := s.findTrigger(id)
	if trigger == nil {
		s.jsonError(w, "trigger not found", http.StatusNotFound)
		return
	}

	// Get executions from database
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "database backend required for automation", http.StatusInternalServerError)
		return
	}
	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	executions, err := adapter.GetRecentExecutions(r.Context(), id, 50)
	if err != nil {
		s.logger.Error("failed to get trigger history", "trigger", id, "error", err)
		s.jsonError(w, "failed to get trigger history", http.StatusInternalServerError)
		return
	}

	resp := make([]ExecutionResponse, 0, len(executions))
	for _, e := range executions {
		resp = append(resp, executionToResponse(e))
	}

	s.jsonResponse(w, resp)
}

// handleResetTrigger resets a trigger's counter.
// POST /api/automation/triggers/{id}/reset
func (s *Server) handleResetTrigger(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/automation/triggers/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "reset" {
		s.jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Validate trigger ID format
	if !isValidTriggerID(id) {
		s.jsonError(w, "invalid trigger ID format", http.StatusBadRequest)
		return
	}

	if s.automationSvc == nil {
		s.jsonError(w, "automation service not enabled", http.StatusServiceUnavailable)
		return
	}

	// Verify trigger exists
	trigger := s.findTrigger(id)
	if trigger == nil {
		s.jsonError(w, "trigger not found", http.StatusNotFound)
		return
	}

	// Reset counter in database
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "database backend required for automation", http.StatusInternalServerError)
		return
	}
	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	if err := adapter.ResetCounter(r.Context(), id, "cooldown"); err != nil {
		s.logger.Error("failed to reset trigger", "trigger", id, "error", err)
		s.jsonError(w, "failed to reset trigger", http.StatusInternalServerError)
		return
	}

	s.logger.Info("trigger counter reset", "trigger", id)
	s.jsonResponse(w, map[string]string{
		"status":  "reset",
		"trigger": id,
	})
}

// handleListAutomationTasks returns all AUTO-* tasks.
// GET /api/automation/tasks
func (s *Server) handleListAutomationTasks(w http.ResponseWriter, r *http.Request) {
	// Use efficient database query via is_automation filter
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		// Fallback for non-database backends: load all and filter
		tasks, err := s.backend.LoadAllTasks()
		if err != nil {
			s.logger.Error("failed to load tasks", "error", err)
			s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
			return
		}
		autoTasks := make([]*task.Task, 0)
		for _, t := range tasks {
			if strings.HasPrefix(t.ID, "AUTO-") {
				autoTasks = append(autoTasks, t)
			}
		}
		s.jsonResponse(w, autoTasks)
		return
	}

	// Efficient path: query only automation tasks
	autoTasks, err := dbBackend.LoadAutomationTasks()
	if err != nil {
		s.logger.Error("failed to load automation tasks", "error", err)
		s.jsonError(w, "failed to load automation tasks", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, autoTasks)
}

// handleGetAutomationStats returns automation statistics.
// GET /api/automation/stats
func (s *Server) handleGetAutomationStats(w http.ResponseWriter, r *http.Request) {
	if s.orcConfig == nil {
		s.jsonError(w, "configuration not loaded", http.StatusInternalServerError)
		return
	}

	stats := AutomationStatsResponse{
		TotalTriggers: len(s.orcConfig.Automation.Triggers),
	}

	for _, t := range s.orcConfig.Automation.Triggers {
		if t.Enabled {
			stats.EnabledTriggers++
		}
	}

	// Use efficient database query via is_automation filter
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if ok {
		// Efficient path: single aggregated query
		taskStats, err := dbBackend.GetAutomationTaskStats()
		if err != nil {
			s.logger.Error("failed to get automation task stats", "error", err)
			s.jsonError(w, "failed to get automation task stats", http.StatusInternalServerError)
			return
		}
		stats.PendingTasks = taskStats.Pending
		stats.RunningTasks = taskStats.Running
		stats.CompletedTasks = taskStats.Completed
	} else {
		// Fallback for non-database backends: load all and filter
		tasks, err := s.backend.LoadAllTasks()
		if err != nil {
			s.logger.Error("failed to load tasks", "error", err)
			s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
			return
		}
		for _, t := range tasks {
			if !strings.HasPrefix(t.ID, "AUTO-") {
				continue
			}
			switch t.Status {
			case "created", "planned":
				stats.PendingTasks++
			case "running":
				stats.RunningTasks++
			case "completed":
				stats.CompletedTasks++
			}
		}
	}

	s.jsonResponse(w, stats)
}

// findTrigger finds a trigger by ID in the config.
func (s *Server) findTrigger(id string) *config.TriggerConfig {
	for i := range s.orcConfig.Automation.Triggers {
		if s.orcConfig.Automation.Triggers[i].ID == id {
			return &s.orcConfig.Automation.Triggers[i]
		}
	}
	return nil
}

// triggerConfigToResponse converts a TriggerConfig to TriggerResponse.
func triggerConfigToResponse(t *config.TriggerConfig) TriggerResponse {
	return TriggerResponse{
		ID:          t.ID,
		Type:        string(t.Type),
		Description: t.Description,
		Enabled:     t.Enabled,
		Mode:        string(t.Mode),
		Condition:   t.Condition,
		Action:      t.Action,
		Cooldown:    t.Cooldown,
	}
}

// executionToResponse converts an Execution to ExecutionResponse.
func executionToResponse(e *automation.Execution) ExecutionResponse {
	resp := ExecutionResponse{
		ID:            e.ID,
		TriggerID:     e.TriggerID,
		TaskID:        e.TaskID,
		TriggerReason: e.TriggerReason,
		Status:        string(e.Status),
	}
	if !e.TriggeredAt.IsZero() {
		resp.TriggeredAt = e.TriggeredAt.Format("2006-01-02T15:04:05Z")
	}
	if e.CompletedAt != nil && !e.CompletedAt.IsZero() {
		resp.CompletedAt = e.CompletedAt.Format("2006-01-02T15:04:05Z")
	}
	if e.ErrorMessage != "" {
		resp.ErrorMessage = e.ErrorMessage
	}
	return resp
}
