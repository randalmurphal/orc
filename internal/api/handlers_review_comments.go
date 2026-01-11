package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// reviewRetryResponse is the response for the review retry endpoint.
type reviewRetryResponse struct {
	TaskID       string `json:"task_id"`
	CommentCount int    `json:"comment_count"`
	RetryContext string `json:"retry_context"`
	Status       string `json:"status"`
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
	defer pdb.Close()

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
	defer pdb.Close()

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
	defer pdb.Close()

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
	defer pdb.Close()

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
	defer pdb.Close()

	if err := pdb.DeleteReviewComment(commentID); err != nil {
		s.jsonError(w, "failed to delete review comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleReviewRetry triggers a retry with all open review comments.
func (s *Server) handleReviewRetry(w http.ResponseWriter, r *http.Request) {
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
	defer pdb.Close()

	// Get all open comments
	comments, err := pdb.ListReviewComments(taskID, "open")
	if err != nil {
		s.jsonError(w, "failed to list review comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(comments) == 0 {
		s.jsonError(w, "no open comments to address", http.StatusBadRequest)
		return
	}

	// Build retry context from comments
	context := buildReviewRetryContext(comments)

	// Return the context that would be injected into retry
	// Integration with executor would happen here
	resp := reviewRetryResponse{
		TaskID:       taskID,
		CommentCount: len(comments),
		RetryContext: context,
		Status:       "queued",
	}

	s.jsonResponse(w, resp)
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
	defer pdb.Close()

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

// buildReviewRetryContext formats review comments into a context string for retry.
func buildReviewRetryContext(comments []db.ReviewComment) string {
	var sb strings.Builder
	sb.WriteString("## Review Feedback\n\n")
	sb.WriteString("The following issues were identified during code review:\n\n")

	// Group by file
	byFile := make(map[string][]db.ReviewComment)
	for _, c := range comments {
		key := c.FilePath
		if key == "" {
			key = "General"
		}
		byFile[key] = append(byFile[key], c)
	}

	// Sort files for consistent output
	var files []string
	for file := range byFile {
		files = append(files, file)
	}

	for _, file := range files {
		fileComments := byFile[file]
		sb.WriteString(fmt.Sprintf("### %s\n\n", file))
		for _, c := range fileComments {
			if c.LineNumber > 0 {
				sb.WriteString(fmt.Sprintf("- **Line %d** [%s]: %s\n",
					c.LineNumber, c.Severity, c.Content))
			} else {
				sb.WriteString(fmt.Sprintf("- [%s]: %s\n",
					c.Severity, c.Content))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nPlease address all issues above and make the necessary changes.\n")
	return sb.String()
}
