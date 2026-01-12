package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === CLAUDE.md Handlers ===

// handleGetClaudeMD returns CLAUDE.md content.
// Supports ?scope=global|user|project (default: project)
func (s *Server) handleGetClaudeMD(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var claudeMD *claudeconfig.ClaudeMD
	var err error

	homeDir, _ := os.UserHomeDir()
	switch scope {
	case "global":
		claudeMD, err = claudeconfig.LoadClaudeMD(filepath.Join(homeDir, ".claude", "CLAUDE.md"))
	case "user":
		claudeMD, err = claudeconfig.LoadClaudeMD(filepath.Join(homeDir, "CLAUDE.md"))
	default:
		claudeMD, err = claudeconfig.LoadProjectClaudeMD(s.getProjectRoot())
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load CLAUDE.md: %v", err), http.StatusInternalServerError)
		return
	}
	if claudeMD == nil {
		// Return empty content instead of 404 for editing purposes
		homeDir, _ := os.UserHomeDir()
		path := ""
		switch scope {
		case "global":
			path = filepath.Join(homeDir, ".claude", "CLAUDE.md")
		case "user":
			path = filepath.Join(homeDir, "CLAUDE.md")
		default:
			path = filepath.Join(s.getProjectRoot(), "CLAUDE.md")
		}
		claudeMD = &claudeconfig.ClaudeMD{
			Path:    path,
			Content: "",
			Source:  scope,
		}
	}

	s.jsonResponse(w, claudeMD)
}

// handleUpdateClaudeMD saves CLAUDE.md.
// Supports ?scope=global|user|project (default: project)
func (s *Server) handleUpdateClaudeMD(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var savePath string
	var source string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	switch scope {
	case "global":
		savePath = filepath.Join(homeDir, ".claude", "CLAUDE.md")
		source = "global"
	case "user":
		savePath = filepath.Join(homeDir, "CLAUDE.md")
		source = "user"
	default:
		savePath = filepath.Join(s.getProjectRoot(), "CLAUDE.md")
		source = "project"
	}

	// Ensure directory exists for global
	if scope == "global" {
		if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
			s.jsonError(w, fmt.Sprintf("failed to create directory: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Write the file
	if err := os.WriteFile(savePath, []byte(req.Content), 0644); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save CLAUDE.md: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the saved content as a ClaudeMD response
	claudeMD := &claudeconfig.ClaudeMD{
		Path:    savePath,
		Content: req.Content,
		Source:  source,
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
