package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === Tools Handlers ===

// handleListTools returns all available Claude Code tools.
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	// Check if grouping by category is requested
	if r.URL.Query().Get("by_category") == "true" {
		byCategory := claudeconfig.ToolsByCategory()
		s.jsonResponse(w, byCategory)
		return
	}

	tools := claudeconfig.AvailableTools()
	s.jsonResponse(w, tools)
}

// handleGetToolPermissions returns tool permission settings.
func (s *Server) handleGetToolPermissions(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		// No settings = no permissions configured
		s.jsonResponse(w, &claudeconfig.ToolPermissions{})
		return
	}

	var perms *claudeconfig.ToolPermissions
	if err := settings.GetExtension("tool_permissions", &perms); err != nil || perms == nil {
		s.jsonResponse(w, &claudeconfig.ToolPermissions{})
		return
	}

	s.jsonResponse(w, perms)
}

// handleUpdateToolPermissions saves tool permission settings.
func (s *Server) handleUpdateToolPermissions(w http.ResponseWriter, r *http.Request) {
	var perms claudeconfig.ToolPermissions
	if err := json.NewDecoder(r.Body).Decode(&perms); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	settings.SetExtension("tool_permissions", perms)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, perms)
}
