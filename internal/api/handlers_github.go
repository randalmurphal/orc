package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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
	Synced   int    `json:"synced"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
	Total    int    `json:"total"`
	PRNumber int    `json:"pr_number"`
	Message  string `json:"message,omitempty"`
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

	t, err := task.LoadFrom(s.workDir, taskID)
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
		// Use defaults if no body or invalid JSON
		s.logger.Debug("using default PR options", "reason", "empty or invalid body")
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
	if err != nil && !errors.Is(err, github.ErrNoPRFound) {
		s.logger.Warn("failed to check for existing PR", "error", err)
	}
	if err == nil && existingPR != nil {
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

	t, err := task.LoadFrom(s.workDir, taskID)
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
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
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

	t, err := task.LoadFrom(s.workDir, taskID)
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
	defer func() { _ = pdb.Close() }()

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
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
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

// handleAutoFixComment triggers an auto-fix for a PR comment by rewinding to implement phase.
func (s *Server) handleAutoFixComment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	t, err := task.LoadFrom(s.workDir, taskID)
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
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetReviewComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	// Load plan and state for the rewind/retry
	p, err := plan.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.jsonError(w, "plan not found: "+err.Error(), http.StatusNotFound)
		return
	}

	st, err := state.LoadFrom(s.workDir, taskID)
	if err != nil {
		// Create new state if it doesn't exist
		st = state.New(taskID)
	}

	// Build retry context with the comment as PR feedback
	prFeedback := executor.PRCommentFeedback{
		Author:   "reviewer",
		Body:     comment.Content,
		FilePath: comment.FilePath,
		Line:     comment.LineNumber,
	}

	// Also get any other open review comments for context
	openComments, _ := pdb.ListReviewComments(taskID, "open")

	opts := executor.RetryOptions{
		FailedPhase:    t.CurrentPhase,
		FailureReason:  fmt.Sprintf("Auto-fix requested for comment: %s", truncateString(comment.Content, 100)),
		PRComments:     []executor.PRCommentFeedback{prFeedback},
		ReviewComments: openComments,
		AttemptNumber:  1,
		MaxAttempts:    3,
		Instructions:   fmt.Sprintf("Fix the issue in %s at line %d: %s", comment.FilePath, comment.LineNumber, comment.Content),
	}

	// Build and set retry context in state
	retryContext := executor.BuildRetryContextForFreshSession(opts)
	st.SetRetryContext(t.CurrentPhase, "implement", opts.FailureReason, retryContext, 1)

	// Save state with retry context
	if err := st.SaveTo(task.TaskDirIn(s.workDir, taskID)); err != nil {
		s.jsonError(w, "failed to save state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Store the auto-fix intent in task metadata
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["autofix_comment_id"] = commentID
	t.Metadata["autofix_file"] = comment.FilePath
	t.Metadata["autofix_line"] = strconv.Itoa(comment.LineNumber)
	t.Metadata["autofix_content"] = comment.Content

	// Update task status to allow re-run
	if t.Status == task.StatusCompleted || t.Status == task.StatusFailed {
		t.Status = task.StatusPlanned
	}

	if err := t.SaveTo(task.TaskDirIn(s.workDir, taskID)); err != nil {
		s.jsonError(w, "failed to save task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-commit: auto-fix triggered
	s.autoCommitTask(t, "auto-fix triggered")

	// Create cancellable context for execution
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function for later cancellation
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Start execution in background goroutine
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		execCfg := executor.ConfigFromOrc(s.orcConfig)
		execCfg.WorkDir = s.workDir
		exec := executor.NewWithConfig(execCfg, s.orcConfig)
		exec.SetPublisher(s.publisher)

		// Resume from implement phase with retry context
		err := exec.ResumeFromPhase(ctx, t, p, st, "implement")
		if err != nil {
			s.logger.Error("auto-fix execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("auto-fix execution completed", "task", taskID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		// Reload and publish final state
		if finalState, err := state.LoadFrom(s.workDir, taskID); err == nil {
			s.Publish(taskID, Event{Type: "state", Data: finalState})
		}
	}()

	resp := autoFixResponse{
		TaskID:    taskID,
		CommentID: commentID,
		Status:    "running",
		Message:   fmt.Sprintf("Auto-fix started for task %s, addressing comment on %s:%d", taskID, comment.FilePath, comment.LineNumber),
	}

	s.jsonResponse(w, resp)
}

// handleMergePR merges the PR for a task.
func (s *Server) handleMergePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.LoadFrom(s.workDir, taskID)
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
		// Use defaults if no body or invalid JSON
		s.logger.Debug("using default merge options", "reason", "empty or invalid body")
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
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
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
	var warning string
	if err := t.Save(); err != nil {
		s.logger.Error("failed to update task status after merge", "task", taskID, "error", err)
		warning = "task status not updated: " + err.Error()
	} else {
		// Auto-commit: PR merged
		s.autoCommitTask(t, "merged")
	}

	response := map[string]any{
		"merged":    true,
		"pr_number": pr.Number,
		"method":    req.Method,
		"message":   fmt.Sprintf("PR #%d merged successfully", pr.Number),
	}
	if warning != "" {
		response["warning"] = warning
	}

	s.jsonResponse(w, response)
}

// handleListPRChecks lists CI check runs for a task's PR.
func (s *Server) handleListPRChecks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.LoadFrom(s.workDir, taskID)
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

	// Calculate summary with proper conclusion categorization
	var passed, failed, pending, neutral int
	for _, check := range checks {
		switch check.Status {
		case "completed":
			switch check.Conclusion {
			case "success":
				passed++
			case "neutral", "skipped", "cancelled":
				// These aren't failures - count separately
				neutral++
			case "action_required":
				// Treat action_required as needing attention but not failure
				neutral++
			default:
				// failure, timed_out, stale, startup_failure, etc.
				failed++
			}
		default:
			// queued, in_progress, waiting, pending, requested
			pending++
		}
	}

	s.jsonResponse(w, map[string]any{
		"checks": checks,
		"summary": map[string]int{
			"passed":  passed,
			"failed":  failed,
			"pending": pending,
			"neutral": neutral,
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

// replyToCommentRequest is the request body for replying to a PR comment.
type replyToCommentRequest struct {
	Body string `json:"body"`
}

// handleReplyToPRComment replies to a PR comment thread.
func (s *Server) handleReplyToPRComment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Branch == "" {
		s.jsonError(w, "task has no branch", http.StatusBadRequest)
		return
	}

	var req replyToCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Body == "" {
		s.jsonError(w, "body is required", http.StatusBadRequest)
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
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Parse comment ID as int64
	threadID, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		s.jsonError(w, "invalid comment ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Reply to the comment thread
	reply, err := client.ReplyToComment(r.Context(), pr.Number, threadID, req.Body)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to reply to comment: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]any{
		"reply":     reply,
		"pr_number": pr.Number,
		"thread_id": threadID,
		"message":   "Reply posted successfully",
	})
}

// importPRCommentsResponse is the response for importing PR comments.
type importPRCommentsResponse struct {
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
	Total    int    `json:"total"`
	PRNumber int    `json:"pr_number"`
	Message  string `json:"message,omitempty"`
}

// handleImportPRComments imports PR comments as local review comments.
func (s *Server) handleImportPRComments(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.LoadFrom(s.workDir, taskID)
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

	// Find PR for branch
	pr, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Get PR comments from GitHub
	prComments, err := client.ListPRComments(r.Context(), pr.Number)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list PR comments: %v", err), http.StatusInternalServerError)
		return
	}

	if len(prComments) == 0 {
		s.jsonResponse(w, importPRCommentsResponse{
			Total:    0,
			PRNumber: pr.Number,
			Message:  "no comments to import",
		})
		return
	}

	// Open project database
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Get existing comments to check for duplicates
	existingComments, err := pdb.ListReviewComments(taskID, "")
	if err != nil {
		s.jsonError(w, "failed to list existing comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build a map of existing comments for deduplication
	// Key: filepath:line:content_prefix
	existingMap := make(map[string]bool)
	for _, c := range existingComments {
		key := fmt.Sprintf("%s:%d:%s", c.FilePath, c.LineNumber, truncateString(c.Content, 50))
		existingMap[key] = true
	}

	// Get latest review round
	latestRound, _ := pdb.GetLatestReviewRound(taskID)
	newRound := latestRound + 1

	resp := importPRCommentsResponse{
		Total:    len(prComments),
		PRNumber: pr.Number,
	}

	// Import each comment
	for _, pc := range prComments {
		// Skip reply comments (part of a thread)
		if pc.ThreadID != 0 {
			resp.Skipped++
			continue
		}

		// Check for duplicate
		key := fmt.Sprintf("%s:%d:%s", pc.Path, pc.Line, truncateString(pc.Body, 50))
		if existingMap[key] {
			resp.Skipped++
			continue
		}

		// Create new review comment
		comment := &db.ReviewComment{
			TaskID:      taskID,
			ReviewRound: newRound,
			FilePath:    pc.Path,
			LineNumber:  pc.Line,
			Content:     fmt.Sprintf("[@%s] %s", pc.Author, pc.Body),
			Severity:    db.SeverityIssue, // Default to issue for PR comments
			Status:      db.CommentStatusOpen,
		}

		if err := pdb.CreateReviewComment(comment); err != nil {
			s.logger.Warn("failed to import PR comment", "error", err, "path", pc.Path, "line", pc.Line)
			resp.Errors++
		} else {
			resp.Imported++
			// Add to existing map to prevent duplicate imports in same batch
			existingMap[key] = true
		}
	}

	if resp.Imported > 0 {
		resp.Message = fmt.Sprintf("imported %d comments from PR #%d (round %d)", resp.Imported, pr.Number, newRound)
	} else if resp.Skipped > 0 {
		resp.Message = fmt.Sprintf("all %d comments already exist or are replies", resp.Skipped)
	} else {
		resp.Message = "no new comments to import"
	}

	s.jsonResponse(w, resp)
}

// handleRefreshPRStatus triggers an on-demand refresh of PR status for a task.
func (s *Server) handleRefreshPRStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := task.LoadFrom(s.workDir, taskID)
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

	// Find PR by branch
	pr, err := client.FindPRByBranch(r.Context(), t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			s.jsonError(w, "no PR found for task branch", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to find PR: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Get PR status summary
	summary, err := client.GetPRStatusSummary(r.Context(), pr)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to get PR status: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine PR status
	prStatus := DeterminePRStatus(pr, summary)

	// Update task PR info
	if t.PR == nil {
		t.PR = &task.PRInfo{}
	}
	t.PR.URL = pr.HTMLURL
	t.PR.Number = pr.Number
	t.PR.Status = prStatus
	t.PR.ChecksStatus = summary.ChecksStatus
	t.PR.Mergeable = summary.Mergeable
	t.PR.ReviewCount = summary.ReviewCount
	t.PR.ApprovalCount = summary.ApprovalCount
	now := time.Now()
	t.PR.LastCheckedAt = &now

	// Save task
	if err := t.SaveTo(task.TaskDirIn(s.workDir, taskID)); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	// Auto-commit: PR status updated
	s.autoCommitTask(t, "PR status updated")

	s.jsonResponse(w, map[string]any{
		"pr":      pr,
		"status":  t.PR,
		"reviews": summary,
		"message": fmt.Sprintf("PR #%d status refreshed", pr.Number),
		"task_id": taskID,
	})
}
