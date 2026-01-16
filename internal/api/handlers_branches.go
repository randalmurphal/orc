// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/randalmurphal/orc/internal/git"
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
// Query params:
//   - type: filter by branch type (initiative|staging|task)
//   - status: filter by status (active|merged|stale|orphaned)
//   - page: page number for pagination (default: 1)
//   - limit: items per page (default: 20, max: 100)
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	// Parse and validate filter params
	branchType := r.URL.Query().Get("type")
	branchStatus := r.URL.Query().Get("status")

	// Validate type if provided
	validTypes := map[string]bool{"": true, "initiative": true, "staging": true, "task": true}
	if !validTypes[branchType] {
		s.jsonError(w, "invalid type: must be initiative, staging, or task", http.StatusBadRequest)
		return
	}

	// Validate status if provided
	validStatuses := map[string]bool{"": true, "active": true, "merged": true, "stale": true, "orphaned": true}
	if !validStatuses[branchStatus] {
		s.jsonError(w, "invalid status: must be active, merged, stale, or orphaned", http.StatusBadRequest)
		return
	}

	opts := storage.BranchListOpts{
		Type:   storage.BranchType(branchType),
		Status: storage.BranchStatus(branchStatus),
	}

	branches, err := s.backend.ListBranches(opts)
	if err != nil {
		s.logger.Error("failed to list branches", "error", err)
		s.jsonError(w, "failed to list branches", http.StatusInternalServerError)
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

	// Check for pagination params
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// If no pagination requested, return all branches (backward compatible)
	if pageStr == "" && limitStr == "" {
		s.jsonResponse(w, result)
		return
	}

	// Parse pagination params
	page := 1
	limit := 20 // default limit
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Calculate pagination
	total := len(result)
	totalPages := (total + limit - 1) / limit
	start := (page - 1) * limit
	end := start + limit

	// Bounds checking
	if start >= total {
		start = total
		end = total
	}
	if end > total {
		end = total
	}

	pagedBranches := result[start:end]

	s.jsonResponse(w, map[string]any{
		"branches":    pagedBranches,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// handleGetBranch returns a single branch by name.
// GET /api/branches/{name}
func (s *Server) handleGetBranch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.jsonError(w, "branch name is required", http.StatusBadRequest)
		return
	}

	// Validate branch name to prevent injection attacks
	if err := git.ValidateBranchName(name); err != nil {
		s.jsonError(w, "invalid branch name", http.StatusBadRequest)
		return
	}

	branch, err := s.backend.LoadBranch(name)
	if err != nil {
		s.logger.Error("failed to load branch", "name", name, "error", err)
		s.jsonError(w, "failed to load branch", http.StatusInternalServerError)
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

	// Validate branch name to prevent injection attacks
	if err := git.ValidateBranchName(name); err != nil {
		s.jsonError(w, "invalid branch name", http.StatusBadRequest)
		return
	}

	var req updateBranchStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
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
		s.logger.Error("failed to update branch status", "name", name, "error", err)
		s.jsonError(w, "failed to update branch status", http.StatusInternalServerError)
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

	// Validate branch name to prevent injection attacks
	if err := git.ValidateBranchName(name); err != nil {
		s.jsonError(w, "invalid branch name", http.StatusBadRequest)
		return
	}

	if err := s.backend.DeleteBranch(name); err != nil {
		s.logger.Error("failed to delete branch", "name", name, "error", err)
		s.jsonError(w, "failed to delete branch", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
