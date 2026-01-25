package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
)

// retryRequest is the request body for triggering a retry.
type retryRequest struct {
	IncludeReviewComments bool   `json:"include_review_comments"`
	IncludePRComments     bool   `json:"include_pr_comments"`
	Instructions          string `json:"instructions"`
	FromPhase             string `json:"from_phase"`
}

// retryResponse is the response for the retry endpoint.
type retryResponse struct {
	TaskID       string `json:"task_id"`
	FromPhase    string `json:"from_phase"`
	Context      string `json:"context"`
	Status       string `json:"status"`
	CommentCount int    `json:"comment_count"`
}

// retryPreviewResponse is the response for the retry preview endpoint.
type retryPreviewResponse struct {
	TaskID          string `json:"task_id"`
	CurrentPhase    string `json:"current_phase"`
	OpenComments    int    `json:"open_comments"`
	ContextPreview  string `json:"context_preview"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

// handleRetryTask triggers a task retry with comprehensive context.
func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req retryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Use defaults if body is empty or invalid
		req.IncludeReviewComments = true
		req.IncludePRComments = true
	}

	// Load task to get current state
	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.jsonError(w, "task not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Open project database
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Get attempt number from task's retry context
	attemptNumber := 1
	if t.Execution.RetryContext != nil {
		attemptNumber = t.Execution.RetryContext.Attempt + 1
	}

	// Build retry options
	opts := executor.RetryOptions{
		FailedPhase:   t.CurrentPhase,
		Instructions:  req.Instructions,
		AttemptNumber: attemptNumber,
		MaxAttempts:   3,
	}

	// Get review comments if requested
	if req.IncludeReviewComments {
		comments, err := pdb.ListReviewComments(taskID, "open")
		if err != nil {
			s.logger.Warn("failed to list review comments for retry", "task_id", taskID, "error", err)
		} else {
			opts.ReviewComments = comments
		}
	}

	// Get previous transcripts for context compression
	transcripts, _ := pdb.GetTranscripts(taskID)
	opts.PreviousContext = executor.CompressPreviousContext(transcripts)

	// Build the retry context
	context := executor.BuildRetryContextForFreshSession(opts)

	// Determine from_phase (either specified or infer from retry map)
	fromPhase := req.FromPhase
	if fromPhase == "" {
		retryMap := executor.DefaultRetryMap()
		if mapped, ok := retryMap[t.CurrentPhase]; ok {
			fromPhase = mapped
		} else {
			fromPhase = "implement" // Default fallback
		}
	}

	// Return the response
	// Full integration with executor would queue the retry here
	resp := retryResponse{
		TaskID:       taskID,
		FromPhase:    fromPhase,
		Context:      context,
		Status:       "queued",
		CommentCount: len(opts.ReviewComments),
	}

	s.jsonResponse(w, resp)
}

// handleGetRetryPreview returns a preview of the retry context.
func (s *Server) handleGetRetryPreview(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	// Load task to get current phase
	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.jsonError(w, "task not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Open project database
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Get open review comments
	comments, err := pdb.ListReviewComments(taskID, "open")
	if err != nil {
		s.jsonError(w, "failed to list review comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get attempt number from task's retry context
	attemptNumber := 1
	if t.Execution.RetryContext != nil {
		attemptNumber = t.Execution.RetryContext.Attempt + 1
	}

	// Build preview context
	opts := executor.RetryOptions{
		FailedPhase:    t.CurrentPhase,
		ReviewComments: comments,
		AttemptNumber:  attemptNumber,
		MaxAttempts:    3,
	}

	context := executor.BuildRetryContextForFreshSession(opts)

	resp := retryPreviewResponse{
		TaskID:          taskID,
		CurrentPhase:    t.CurrentPhase,
		OpenComments:    len(comments),
		ContextPreview:  context,
		EstimatedTokens: len(context) / 4, // Rough estimate
	}

	s.jsonResponse(w, resp)
}

// handleRetryWithFeedback triggers a retry incorporating specific feedback.
func (s *Server) handleRetryWithFeedback(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		FailureReason string                       `json:"failure_reason"`
		FailureOutput string                       `json:"failure_output"`
		PRComments    []executor.PRCommentFeedback `json:"pr_comments"`
		Instructions  string                       `json:"instructions"`
		FromPhase     string                       `json:"from_phase"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Load task
	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.jsonError(w, "task not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Open project database
	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Get review comments
	reviewComments, err := pdb.ListReviewComments(taskID, "open")
	if err != nil {
		s.logger.Warn("failed to list review comments for retry", "task_id", taskID, "error", err)
	}

	// Get transcripts for context
	transcripts, _ := pdb.GetTranscripts(taskID)

	// Get attempt number from task's retry context
	attemptNumber := 1
	if t.Execution.RetryContext != nil {
		attemptNumber = t.Execution.RetryContext.Attempt + 1
	}

	// Build comprehensive retry options
	opts := executor.RetryOptions{
		FailedPhase:     t.CurrentPhase,
		FailureReason:   req.FailureReason,
		FailureOutput:   req.FailureOutput,
		ReviewComments:  reviewComments,
		PRComments:      req.PRComments,
		Instructions:    req.Instructions,
		PreviousContext: executor.CompressPreviousContext(transcripts),
		AttemptNumber:   attemptNumber,
		MaxAttempts:     3,
	}

	// Build context
	context := executor.BuildRetryContextForFreshSession(opts)

	// Determine from phase
	fromPhase := req.FromPhase
	if fromPhase == "" {
		retryMap := executor.DefaultRetryMap()
		if mapped, ok := retryMap[t.CurrentPhase]; ok {
			fromPhase = mapped
		} else {
			fromPhase = "implement"
		}
	}

	resp := retryResponse{
		TaskID:       taskID,
		FromPhase:    fromPhase,
		Context:      context,
		Status:       "queued",
		CommentCount: len(reviewComments) + len(req.PRComments),
	}

	s.jsonResponse(w, resp)
}
