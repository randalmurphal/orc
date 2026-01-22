package api

import (
	"net/http"

	"github.com/randalmurphal/orc/internal/db"
)

// handleGetReviewFindings returns all review findings for a task.
func (s *Server) handleGetReviewFindings(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	findings, err := pdb.GetAllReviewFindings(taskID)
	if err != nil {
		s.jsonError(w, "failed to get review findings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if findings == nil {
		findings = []*db.ReviewFindings{}
	}

	s.jsonResponse(w, findings)
}
