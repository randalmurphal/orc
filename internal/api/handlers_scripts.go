package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === Scripts Handlers ===

// handleListScripts returns all registered scripts.
func (s *Server) handleListScripts(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	scripts, err := svc.List()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list scripts: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, scripts)
}

// handleGetScript returns a specific script by name.
func (s *Server) handleGetScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewScriptService(s.getProjectRoot())

	script, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "script not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, script)
}

// handleCreateScript registers a new script.
func (s *Server) handleCreateScript(w http.ResponseWriter, r *http.Request) {
	var script claudeconfig.ProjectScript
	if err := json.NewDecoder(r.Body).Decode(&script); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	if err := svc.Create(script); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, script)
}

// handleUpdateScript updates an existing script registration.
func (s *Server) handleUpdateScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var script claudeconfig.ProjectScript
	if err := json.NewDecoder(r.Body).Decode(&script); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	if err := svc.Update(name, script); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated script
	updated, _ := svc.Get(script.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteScript removes a script registration.
func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewScriptService(s.getProjectRoot())

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDiscoverScripts auto-discovers scripts in .claude/scripts/.
func (s *Server) handleDiscoverScripts(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	discovered, err := svc.Discover()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to discover scripts: %v", err), http.StatusInternalServerError)
		return
	}

	// Return discovered scripts (not yet registered)
	s.jsonResponse(w, discovered)
}
