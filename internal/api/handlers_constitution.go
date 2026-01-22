package api

import (
	"encoding/json"
	"net/http"
)

// ConstitutionResponse represents the constitution data returned by the API.
type ConstitutionResponse struct {
	Content string `json:"content"`
	Version string `json:"version"`
	Exists  bool   `json:"exists"`
}

// ConstitutionRequest is the request body for setting constitution.
type ConstitutionRequest struct {
	Content string `json:"content"`
	Version string `json:"version"`
}

// handleGetConstitution returns the current project constitution.
func (s *Server) handleGetConstitution(w http.ResponseWriter, r *http.Request) {
	content, version, err := s.backend.LoadConstitution()
	if err != nil {
		// Return empty constitution (not an error - just not set)
		s.jsonResponse(w, ConstitutionResponse{
			Content: "",
			Version: "",
			Exists:  false,
		})
		return
	}

	s.jsonResponse(w, ConstitutionResponse{
		Content: content,
		Version: version,
		Exists:  content != "",
	})
}

// handleUpdateConstitution saves or updates the project constitution.
func (s *Server) handleUpdateConstitution(w http.ResponseWriter, r *http.Request) {
	var req ConstitutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Default version to 1.0.0 if not provided
	version := req.Version
	if version == "" {
		version = "1.0.0"
	}

	if err := s.backend.SaveConstitution(req.Content, version); err != nil {
		s.jsonError(w, "failed to save constitution: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, ConstitutionResponse{
		Content: req.Content,
		Version: version,
		Exists:  true,
	})
}

// handleDeleteConstitution removes the project constitution.
func (s *Server) handleDeleteConstitution(w http.ResponseWriter, r *http.Request) {
	if err := s.backend.DeleteConstitution(); err != nil {
		s.jsonError(w, "failed to delete constitution: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, ConstitutionResponse{
		Content: "",
		Version: "",
		Exists:  false,
	})
}
