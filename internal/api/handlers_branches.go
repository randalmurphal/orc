// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/storage"
)

// branchResponse is the API response for a branch.
type branchResponse struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	OwnerID      string `json:"owner_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	LastActivity string `json:"last_activity"`
	Status       string `json:"status"`
}

// handleListBranches returns all tracked branches.
// GET /api/branches
// Query params: type (initiative|staging|task), status (active|merged|stale|orphaned)
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	// Parse filter params
	branchType := r.URL.Query().Get("type")
	branchStatus := r.URL.Query().Get("status")

	opts := storage.BranchListOpts{
		Type:   storage.BranchType(branchType),
		Status: storage.BranchStatus(branchStatus),
	}

	branches, err := s.backend.ListBranches(opts)
	if err != nil {
		s.jsonError(w, "failed to list branches: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	result := make([]branchResponse, len(branches))
	for i, b := range branches {
		result[i] = branchResponse{
			Name:         b.Name,
			Type:         string(b.Type),
			OwnerID:      b.OwnerID,
			CreatedAt:    b.CreatedAt.Format("2006-01-02T15:04:05Z"),
			LastActivity: b.LastActivity.Format("2006-01-02T15:04:05Z"),
			Status:       string(b.Status),
		}
	}

	s.jsonResponse(w, result)
}

// handleGetBranch returns a single branch by name.
// GET /api/branches/{name}
func (s *Server) handleGetBranch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.jsonError(w, "branch name is required", http.StatusBadRequest)
		return
	}

	branch, err := s.backend.LoadBranch(name)
	if err != nil {
		s.jsonError(w, "failed to load branch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if branch == nil {
		s.jsonError(w, "branch not found", http.StatusNotFound)
		return
	}

	result := branchResponse{
		Name:         branch.Name,
		Type:         string(branch.Type),
		OwnerID:      branch.OwnerID,
		CreatedAt:    branch.CreatedAt.Format("2006-01-02T15:04:05Z"),
		LastActivity: branch.LastActivity.Format("2006-01-02T15:04:05Z"),
		Status:       string(branch.Status),
	}

	s.jsonResponse(w, result)
}

// updateBranchStatusRequest is the request body for updating branch status.
type updateBranchStatusRequest struct {
	Status string `json:"status"`
}

// handleUpdateBranchStatus updates a branch's status.
// PATCH /api/branches/{name}/status
func (s *Server) handleUpdateBranchStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.jsonError(w, "branch name is required", http.StatusBadRequest)
		return
	}

	var req updateBranchStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":   true,
		"merged":   true,
		"stale":    true,
		"orphaned": true,
	}
	if !validStatuses[req.Status] {
		s.jsonError(w, "invalid status: must be active, merged, stale, or orphaned", http.StatusBadRequest)
		return
	}

	if err := s.backend.UpdateBranchStatus(name, storage.BranchStatus(req.Status)); err != nil {
		s.jsonError(w, "failed to update branch status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteBranch removes a branch from the registry.
// DELETE /api/branches/{name}
func (s *Server) handleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.jsonError(w, "branch name is required", http.StatusBadRequest)
		return
	}

	if err := s.backend.DeleteBranch(name); err != nil {
		s.jsonError(w, "failed to delete branch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
