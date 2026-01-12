package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/prompt"
)

// promptService returns a prompt service for the current project.
func (s *Server) promptService() *prompt.Service {
	orcDir := filepath.Join(s.workDir, ".orc")
	return prompt.NewService(orcDir)
}

// handleListPrompts returns all available prompts.
func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	svc := s.promptService()
	prompts, err := svc.List()
	if err != nil {
		s.jsonError(w, "failed to list prompts", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, prompts)
}

// handleGetPromptVariables returns template variable documentation.
func (s *Server) handleGetPromptVariables(w http.ResponseWriter, r *http.Request) {
	vars := prompt.GetVariableReference()
	s.jsonResponse(w, vars)
}

// handleGetPrompt returns a specific prompt by phase.
func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := s.promptService()

	p, err := svc.Get(phase)
	if err != nil {
		s.jsonError(w, "prompt not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetPromptDefault returns the embedded default prompt for a phase.
func (s *Server) handleGetPromptDefault(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := s.promptService()

	p, err := svc.GetDefault(phase)
	if err != nil {
		s.jsonError(w, "default prompt not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleSavePrompt saves a project prompt override.
func (s *Server) handleSavePrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		s.jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	svc := s.promptService()
	if err := svc.Save(phase, req.Content); err != nil {
		s.jsonError(w, "failed to save prompt", http.StatusInternalServerError)
		return
	}

	// Return updated prompt
	p, err := svc.Get(phase)
	if err != nil {
		s.jsonError(w, "failed to reload prompt", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, p)
}

// handleDeletePrompt deletes a project prompt override.
func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := s.promptService()

	// Check if override exists
	if !svc.HasOverride(phase) {
		s.jsonError(w, "no override exists for this phase", http.StatusNotFound)
		return
	}

	if err := svc.Delete(phase); err != nil {
		s.jsonError(w, "failed to delete prompt", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
