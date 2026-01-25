package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// createReviewCommentRequest is the request body for creating a review comment.
type createReviewCommentRequest struct {
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	Content     string `json:"content"`
	Severity    string `json:"severity"`
	ReviewRound int    `json:"review_round"`
}

// updateReviewCommentRequest is the request body for updating a review comment.
type updateReviewCommentRequest struct {
	Content    string `json:"content,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Status     string `json:"status,omitempty"`
	ResolvedBy string `json:"resolved_by,omitempty"`
}

// handleListReviewComments returns all review comments for a task.
func (s *Server) handleListReviewComments(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	comments, err := pdb.ListReviewComments(taskID, status)
	if err != nil {
		s.jsonError(w, "failed to list review comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if comments == nil {
		comments = []db.ReviewComment{}
	}

	s.jsonResponse(w, comments)
}

// handleCreateReviewComment creates a new review comment.
func (s *Server) handleCreateReviewComment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req createReviewCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		s.jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	// Validate severity
	severity := db.ReviewCommentSeverity(req.Severity)
	if severity == "" {
		severity = db.SeveritySuggestion
	} else if severity != db.SeveritySuggestion && severity != db.SeverityIssue && severity != db.SeverityBlocker {
		s.jsonError(w, "invalid severity: must be suggestion, issue, or blocker", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Ensure task exists in database for foreign key constraint
	if err := s.syncTaskToDB(pdb, taskID); err != nil {
		s.jsonError(w, "failed to sync task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get latest review round if not specified
	reviewRound := req.ReviewRound
	if reviewRound == 0 {
		latest, err := pdb.GetLatestReviewRound(taskID)
		if err != nil {
			s.jsonError(w, "failed to get review round: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reviewRound = latest
		if reviewRound == 0 {
			reviewRound = 1
		}
	}

	comment := &db.ReviewComment{
		TaskID:      taskID,
		ReviewRound: reviewRound,
		FilePath:    req.FilePath,
		LineNumber:  req.LineNumber,
		Content:     req.Content,
		Severity:    severity,
	}

	if err := pdb.CreateReviewComment(comment); err != nil {
		s.jsonError(w, "failed to create review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, comment)
}

// handleGetReviewComment retrieves a single review comment.
func (s *Server) handleGetReviewComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetReviewComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, comment)
}

// handleUpdateReviewComment updates a review comment.
func (s *Server) handleUpdateReviewComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetReviewComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	var req updateReviewCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content != "" {
		comment.Content = req.Content
	}
	if req.Severity != "" {
		severity := db.ReviewCommentSeverity(req.Severity)
		if severity != db.SeveritySuggestion && severity != db.SeverityIssue && severity != db.SeverityBlocker {
			s.jsonError(w, "invalid severity: must be suggestion, issue, or blocker", http.StatusBadRequest)
			return
		}
		comment.Severity = severity
	}
	if req.Status != "" {
		status := db.ReviewCommentStatus(req.Status)
		if status != db.CommentStatusOpen && status != db.CommentStatusResolved && status != db.CommentStatusWontFix {
			s.jsonError(w, "invalid status: must be open, resolved, or wont_fix", http.StatusBadRequest)
			return
		}
		comment.Status = status
		if status == db.CommentStatusResolved || status == db.CommentStatusWontFix {
			now := time.Now()
			comment.ResolvedAt = &now
			if req.ResolvedBy != "" {
				comment.ResolvedBy = req.ResolvedBy
			}
		}
	}

	if err := pdb.UpdateReviewComment(comment); err != nil {
		s.jsonError(w, "failed to update review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, comment)
}

// handleDeleteReviewComment removes a review comment.
func (s *Server) handleDeleteReviewComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteReviewComment(commentID); err != nil {
		s.jsonError(w, "failed to delete review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleReviewRetry triggers a retry with all open review comments.
// TODO: This feature requires the workflow system to be fully implemented.
func (s *Server) handleReviewRetry(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, "review retry is temporarily unavailable during workflow migration", http.StatusNotImplemented)
}

// handleGetReviewStats returns statistics about review comments for a task.
func (s *Server) handleGetReviewStats(w http.ResponseWriter, r *http.Request) {
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

	openCount, err := pdb.CountOpenComments(taskID)
	if err != nil {
		s.jsonError(w, "failed to count open comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	blockerCount, err := pdb.CountBlockerComments(taskID)
	if err != nil {
		s.jsonError(w, "failed to count blocker comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	latestRound, err := pdb.GetLatestReviewRound(taskID)
	if err != nil {
		s.jsonError(w, "failed to get latest review round: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count
	allComments, err := pdb.ListReviewComments(taskID, "")
	if err != nil {
		s.jsonError(w, "failed to list review comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	stats := map[string]any{
		"task_id":        taskID,
		"total_comments": len(allComments),
		"open_comments":  openCount,
		"blocker_count":  blockerCount,
		"latest_round":   latestRound,
		"can_proceed":    blockerCount == 0,
	}

	s.jsonResponse(w, stats)
}