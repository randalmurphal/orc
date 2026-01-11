package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// handleGetSettings returns merged settings (global + project).
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadSettings(s.getProjectRoot())
	if err != nil {
		// Return empty settings on error
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleGetGlobalSettings returns global-only settings from ~/.claude/settings.json.
func (s *Server) handleGetGlobalSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadGlobalSettings()
	if err != nil {
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleGetProjectSettings returns project-only settings.
func (s *Server) handleGetProjectSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleUpdateSettings saves project settings.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings claudeconfig.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := claudeconfig.SaveProjectSettings(s.getProjectRoot(), &settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, settings)
}
