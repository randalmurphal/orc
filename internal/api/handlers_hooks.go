package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// saveGlobalSettings saves settings to ~/.claude/settings.json
func saveGlobalSettings(settings *claudeconfig.Settings) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	globalPath := filepath.Join(homeDir, ".claude", "settings.json")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(globalPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// === Hooks Handlers (settings.json format) ===

// handleListHooks returns all hooks from settings.json.
// Supports ?scope=global to list from ~/.claude/settings.json instead of project.
func (s *Server) handleListHooks(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var settings *claudeconfig.Settings
	var err error

	if scope == "global" {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.getProjectRoot())
	}

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
// Supports ?scope=global to get from ~/.claude/settings.json instead of project.
func (s *Server) handleGetHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var settings *claudeconfig.Settings
	var err error

	if scope == "global" {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.getProjectRoot())
	}

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
// Supports ?scope=global to add to ~/.claude/settings.json instead of project.
func (s *Server) handleCreateHook(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var req struct {
		Event string            `json:"event"`
		Hook  claudeconfig.Hook `json:"hook"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var settings *claudeconfig.Settings
	var err error

	if scope == "global" {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.getProjectRoot())
	}

	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[req.Event] = append(settings.Hooks[req.Event], req.Hook)

	if scope == "global" {
		err = saveGlobalSettings(settings)
	} else {
		err = claudeconfig.SaveProjectSettings(s.getProjectRoot(), settings)
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, req.Hook)
}

// handleUpdateHook updates hooks for a specific event.
// Supports ?scope=global to update in ~/.claude/settings.json instead of project.
func (s *Server) handleUpdateHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var hooks []claudeconfig.Hook
	if err := json.NewDecoder(r.Body).Decode(&hooks); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var settings *claudeconfig.Settings
	var err error

	if scope == "global" {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.getProjectRoot())
	}

	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[eventName] = hooks

	if scope == "global" {
		err = saveGlobalSettings(settings)
	} else {
		err = claudeconfig.SaveProjectSettings(s.getProjectRoot(), settings)
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hooks)
}

// handleDeleteHook removes all hooks for an event type.
// Supports ?scope=global to delete from ~/.claude/settings.json instead of project.
func (s *Server) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var settings *claudeconfig.Settings
	var err error

	if scope == "global" {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.getProjectRoot())
	}

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

	if scope == "global" {
		err = saveGlobalSettings(settings)
	} else {
		err = claudeconfig.SaveProjectSettings(s.getProjectRoot(), settings)
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
