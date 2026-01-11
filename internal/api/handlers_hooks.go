package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === Hooks Handlers (settings.json format) ===

// handleListHooks returns all hooks from settings.json.
func (s *Server) handleListHooks(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		// No settings file is OK - return empty hooks
		s.jsonResponse(w, map[string][]claudeconfig.Hook{})
		return
	}

	hooks := settings.Hooks
	if hooks == nil {
		hooks = make(map[string][]claudeconfig.Hook)
	}

	s.jsonResponse(w, hooks)
}

// handleGetHookTypes returns available hook event types.
func (s *Server) handleGetHookTypes(w http.ResponseWriter, r *http.Request) {
	events := claudeconfig.ValidHookEvents()
	s.jsonResponse(w, events)
}

// handleGetHook returns hooks for a specific event type.
func (s *Server) handleGetHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "settings not found", http.StatusNotFound)
		return
	}

	hooks, exists := settings.Hooks[eventName]
	if !exists {
		s.jsonError(w, "no hooks for this event", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, hooks)
}

// handleCreateHook adds a hook to settings.json.
func (s *Server) handleCreateHook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Event string            `json:"event"`
		Hook  claudeconfig.Hook `json:"hook"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[req.Event] = append(settings.Hooks[req.Event], req.Hook)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, req.Hook)
}

// handleUpdateHook updates hooks for a specific event.
func (s *Server) handleUpdateHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	var hooks []claudeconfig.Hook
	if err := json.NewDecoder(r.Body).Decode(&hooks); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[eventName] = hooks

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hooks)
}

// handleDeleteHook removes all hooks for an event type.
func (s *Server) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		s.jsonError(w, "settings not found", http.StatusNotFound)
		return
	}

	if settings.Hooks == nil {
		s.jsonError(w, "no hooks configured", http.StatusNotFound)
		return
	}

	// Check if event exists before deleting
	if _, exists := settings.Hooks[eventName]; !exists {
		s.jsonError(w, "hook event not found", http.StatusNotFound)
		return
	}

	delete(settings.Hooks, eventName)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
