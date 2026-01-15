package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/orc/internal/template"
)

// === Template Handlers ===

// handleListTemplates returns all available templates.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := template.List()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list templates: %v", err), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, templates)
}

// handleGetTemplate returns a specific template by name.
func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	t, err := template.Load(name)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("template %q not found", name), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, t)
}

// handleCreateTemplate creates a template from a task.
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID      string `json:"task_id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Global      bool   `json:"global,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := template.ValidateName(req.Name); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if template.Exists(req.Name) {
		s.jsonError(w, fmt.Sprintf("template %q already exists", req.Name), http.StatusConflict)
		return
	}

	if req.TaskID == "" {
		s.jsonError(w, "task_id is required", http.StatusBadRequest)
		return
	}

	t, err := template.SaveFromTask(req.TaskID, req.Name, req.Description, req.Global, s.backend)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create template: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, t)
}

// handleDeleteTemplate removes a template.
func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	t, err := template.Load(name)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("template %q not found", name), http.StatusNotFound)
		return
	}

	if t.Scope == template.ScopeBuiltin {
		s.jsonError(w, "cannot delete built-in template", http.StatusForbidden)
		return
	}

	if err := t.Delete(); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete template: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
