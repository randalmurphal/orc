package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/task"
)

// createPRRequest is the request body for creating a PR.
type createPRRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Base      string   `json:"base"`
	Labels    []string `json:"labels,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
	Draft     bool     `json:"draft"`
}

// syncCommentsResponse is the response for comment sync.
type syncCommentsResponse struct {
	Synced    int    `json:"synced"`
	Skipped   int    `json:"skipped"`
	Errors    int    `json:"errors"`
	Total     int    `json:"total"`
	PRNumber  int    `json:"pr_number"`
	Message   string `json:"message,omitempty"`
}

// autoFixResponse is the response for auto-fix.
type autoFixResponse struct {
	TaskID    string `json:"task_id"`
	CommentID string `json:"comment_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// handleCreatePR creates a PR for a task.
func (s *Server) handleCreatePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	var req createPRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Use defaults if no body
		req = createPRRequest{}
	}

	// Set defaults if not provided
	if req.Title == "" {
		req.Title = fmt.Sprintf("[orc] %s: %s", t.ID, t.Title)
	}
	if req.Body == "" {
		req.Body = buildPRBody(t)
	}
	if req.Base == "" {
		req.Base = "main"
	}

	// Check gh auth first
	if err := github.CheckGHAuth(r.Context()); err != nil {
		s.jsonError(w, "GitHub CLI not authenticated. Run 'gh auth login' first.", http.StatusUnauthorized)
		return
	}

	client, err := github.NewClient(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create GitHub client: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if PR already exists
	existingPR, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		s.logger.Warn("failed to check for existing PR", "error", err)
	}
	if existingPR != nil {
		s.jsonResponse(w, map[string]any{
			"pr":      existingPR,
			"created": false,
			"message": "PR already exists for this branch",
		})
		return
	}

	pr, err := client.CreatePR(r.Context(), github.PRCreateOptions{
		Title:     req.Title,
		Body:      req.Body,
		Head:      t.Branch,
		Base:      req.Base,
		Draft:     req.Draft,
		Labels:    req.Labels,
		Reviewers: req.Reviewers,
	})
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create PR: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, map[string]any{
		"pr":      pr,
		"created": true,
	})
}

// handleGetPR gets the PR for a task.
func (s *Server) handleGetPR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	if err := github.CheckGHAuth(r.Context()); err != nil {
		s.jsonError(w, "GitHub CLI not authenticated", http.StatusUnauthorized)
		return
	}

	client, err := github.NewClient(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create GitHub client: %v", err), http.StatusInternalServerError)
		return
	}

	pr, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		return
	}

	if pr == nil {
		s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		return
	}

	// Also get comments
	comments, err := client.ListPRComments(r.Context(), pr.Number)
	if err != nil {
		s.logger.Warn("failed to get PR comments", "error", err)
		comments = []github.PRComment{}
	}

	// Get check runs
	checks, err := client.GetCheckRuns(r.Context(), t.Branch)
	if err != nil {
		s.logger.Warn("failed to get check runs", "error", err)
		checks = []github.CheckRun{}
	}

	s.jsonResponse(w, map[string]any{
		"pr":       pr,
		"comments": comments,
		"checks":   checks,
	})
}

// handleSyncPRComments syncs local review comments to PR.
func (s *Server) handleSyncPRComments(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	// Get local review comments
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	comments, err := pdb.ListReviewComments(taskID, "")
	if err != nil {
		s.jsonError(w, "failed to list review comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(comments) == 0 {
		s.jsonResponse(w, syncCommentsResponse{
			Total:   0,
			Message: "no comments to sync",
		})
		return
	}

	if err := github.CheckGHAuth(r.Context()); err != nil {
		s.jsonError(w, "GitHub CLI not authenticated", http.StatusUnauthorized)
		return
	}

	client, err := github.NewClient(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create GitHub client: %v", err), http.StatusInternalServerError)
		return
	}

	// Find PR for branch
	pr, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		return
	}
	if pr == nil {
		s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		return
	}

	// Sync each comment to PR
	resp := syncCommentsResponse{
		Total:    len(comments),
		PRNumber: pr.Number,
	}

	for _, c := range comments {
		// Skip comments without file path (general comments)
		// or skip resolved comments
		if c.FilePath == "" {
			resp.Skipped++
			continue
		}
		if c.Status == db.CommentStatusResolved || c.Status == db.CommentStatusWontFix {
			resp.Skipped++
			continue
		}

		// Format comment body with severity
		body := formatReviewCommentBody(c)

		_, err := client.CreatePRComment(r.Context(), pr.Number, github.PRCommentCreate{
			Body: body,
			Path: c.FilePath,
			Line: c.LineNumber,
		})
		if err != nil {
			s.logger.Warn("failed to sync comment", "comment_id", c.ID, "error", err)
			resp.Errors++
		} else {
			resp.Synced++
		}
	}

	if resp.Synced > 0 {
		resp.Message = fmt.Sprintf("synced %d comments to PR #%d", resp.Synced, pr.Number)
	} else if resp.Errors > 0 {
		resp.Message = fmt.Sprintf("failed to sync comments: %d errors", resp.Errors)
	} else {
		resp.Message = "no comments needed syncing"
	}

	s.jsonResponse(w, resp)
}

// handleAutoFixComment queues an auto-fix for a PR comment.
func (s *Server) handleAutoFixComment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Check if comment exists
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer pdb.Close()

	comment, err := pdb.GetReviewComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	// This endpoint prepares the context for auto-fix
	// The actual fix would be triggered through the executor with retry context
	resp := autoFixResponse{
		TaskID:    taskID,
		CommentID: commentID,
		Status:    "prepared",
		Message:   fmt.Sprintf("Auto-fix prepared for comment on %s:%d - '%s'", comment.FilePath, comment.LineNumber, truncateString(comment.Content, 50)),
	}

	// Store the auto-fix intent in task metadata for executor pickup
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["autofix_comment_id"] = commentID
	t.Metadata["autofix_file"] = comment.FilePath
	t.Metadata["autofix_line"] = strconv.Itoa(comment.LineNumber)
	t.Metadata["autofix_content"] = comment.Content

	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to save task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp.Status = "queued"
	resp.Message = fmt.Sprintf("Auto-fix queued for task %s, comment %s", taskID, commentID)

	s.jsonResponse(w, resp)
}

// handleMergePR merges the PR for a task.
func (s *Server) handleMergePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	var req struct {
		Method       string `json:"method"` // merge, squash, rebase
		DeleteBranch bool   `json:"delete_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Method = "squash"
		req.DeleteBranch = true
	}

	if req.Method == "" {
		req.Method = "squash"
	}

	if err := github.CheckGHAuth(r.Context()); err != nil {
		s.jsonError(w, "GitHub CLI not authenticated", http.StatusUnauthorized)
		return
	}

	client, err := github.NewClient(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create GitHub client: %v", err), http.StatusInternalServerError)
		return
	}

	pr, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		return
	}
	if pr == nil {
		s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		return
	}

	err = client.MergePR(r.Context(), pr.Number, github.PRMergeOptions{
		Method:       req.Method,
		DeleteBranch: req.DeleteBranch,
	})
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to merge PR: %v", err), http.StatusInternalServerError)
		return
	}

	// Update task status to completed
	t.Status = task.StatusCompleted
	if err := t.Save(); err != nil {
		s.logger.Warn("failed to update task status after merge", "error", err)
	}

	s.jsonResponse(w, map[string]any{
		"merged":    true,
		"pr_number": pr.Number,
		"method":    req.Method,
		"message":   fmt.Sprintf("PR #%d merged successfully", pr.Number),
	})
}

// handleListPRChecks lists CI check runs for a task's PR.
func (s *Server) handleListPRChecks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.Load(taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	if err := github.CheckGHAuth(r.Context()); err != nil {
		s.jsonError(w, "GitHub CLI not authenticated", http.StatusUnauthorized)
		return
	}

	client, err := github.NewClient(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create GitHub client: %v", err), http.StatusInternalServerError)
		return
	}

	checks, err := client.GetCheckRuns(r.Context(), t.Branch)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to get check runs: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate summary
	var passed, failed, pending int
	for _, check := range checks {
		switch check.Status {
		case "completed":
			if check.Conclusion == "success" {
				passed++
			} else {
				failed++
			}
		default:
			pending++
		}
	}

	s.jsonResponse(w, map[string]any{
		"checks": checks,
		"summary": map[string]int{
			"passed":  passed,
			"failed":  failed,
			"pending": pending,
			"total":   len(checks),
		},
	})
}

// buildPRBody creates the default PR body for a task.
func buildPRBody(t *task.Task) string {
	var sb strings.Builder
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("Task: **%s**\n\n", t.Title))

	if t.Description != "" {
		sb.WriteString("### Description\n\n")
		sb.WriteString(t.Description)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Generated by [orc](https://github.com/randalmurphal/orc)*\n")

	return sb.String()
}

// formatReviewCommentBody formats a review comment for GitHub.
func formatReviewCommentBody(c db.ReviewComment) string {
	var severity string
	switch c.Severity {
	case db.SeverityBlocker:
		severity = "BLOCKER"
	case db.SeverityIssue:
		severity = "Issue"
	default:
		severity = "Suggestion"
	}

	return fmt.Sprintf("**[%s]** %s", severity, c.Content)
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
