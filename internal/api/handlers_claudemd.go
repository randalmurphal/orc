package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === CLAUDE.md Handlers ===

// handleGetClaudeMD returns the project CLAUDE.md content.
func (s *Server) handleGetClaudeMD(w http.ResponseWriter, r *http.Request) {
	claudeMD, err := claudeconfig.LoadProjectClaudeMD(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load CLAUDE.md: %v", err), http.StatusInternalServerError)
		return
	}
	if claudeMD == nil {
		s.jsonError(w, "CLAUDE.md not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, claudeMD)
}

// handleUpdateClaudeMD saves the project CLAUDE.md.
func (s *Server) handleUpdateClaudeMD(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	if err := claudeconfig.SaveProjectClaudeMD(projectRoot, req.Content); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save CLAUDE.md: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the saved content as a ClaudeMD response
	claudeMD := &claudeconfig.ClaudeMD{
		Path:    filepath.Join(projectRoot, "CLAUDE.md"),
		Content: req.Content,
		Source:  "project",
	}

	s.jsonResponse(w, claudeMD)
}

// handleGetClaudeMDHierarchy returns the full CLAUDE.md inheritance chain.
func (s *Server) handleGetClaudeMDHierarchy(w http.ResponseWriter, r *http.Request) {
	hierarchy, err := claudeconfig.LoadClaudeMDHierarchy(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load hierarchy: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hierarchy)
}
