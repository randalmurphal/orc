// Package api provides the Connect RPC and REST API server for orc.
// This file implements the HostingService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/hosting"
	_ "github.com/randalmurphal/orc/internal/hosting/github"
	_ "github.com/randalmurphal/orc/internal/hosting/gitlab"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// HostingProviderFactory creates a hosting provider for dependency injection in tests.
type HostingProviderFactory func(ctx context.Context) (hosting.Provider, error)

// hostingServer implements the HostingServiceHandler interface.
type hostingServer struct {
	orcv1connect.UnimplementedHostingServiceHandler
	backend       storage.Backend
	projectCache  *ProjectCache // Multi-project: cache of backends per project
	projectDir    string
	logger        *slog.Logger
	publisher     events.Publisher
	config        *config.Config
	taskExecutor  TaskExecutorFunc       // Optional: spawns executor for autofix
	clientFactory HostingProviderFactory // Optional: for testing dependency injection
}

// NewHostingServer creates a new HostingService handler.
func NewHostingServer(
	backend storage.Backend,
	projectDir string,
	logger *slog.Logger,
) orcv1connect.HostingServiceHandler {
	return &hostingServer{
		backend:    backend,
		projectDir: projectDir,
		logger:     logger,
	}
}

// NewHostingServerWithExecutor creates a HostingService handler with autofix execution support.
// The executor callback is called by AutofixComment to spawn a WorkflowExecutor goroutine.
// The clientFactory is optional - if nil, uses the default getProvider method.
func NewHostingServerWithExecutor(
	backend storage.Backend,
	projectDir string,
	logger *slog.Logger,
	publisher events.Publisher,
	cfg *config.Config,
	taskExecutor TaskExecutorFunc,
	clientFactory HostingProviderFactory,
) orcv1connect.HostingServiceHandler {
	return &hostingServer{
		backend:       backend,
		projectDir:    projectDir,
		logger:        logger,
		publisher:     publisher,
		config:        cfg,
		taskExecutor:  taskExecutor,
		clientFactory: clientFactory,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *hostingServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *hostingServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

// getProvider creates a hosting provider, checking auth first.
func (s *hostingServer) getProvider(ctx context.Context) (hosting.Provider, error) {
	cfg := hosting.Config{}
	if s.config != nil {
		cfg = hosting.Config{
			Provider:    s.config.Hosting.Provider,
			BaseURL:     s.config.Hosting.BaseURL,
			TokenEnvVar: s.config.Hosting.TokenEnvVar,
		}
	}
	provider, err := hosting.NewProvider(s.projectDir, cfg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create hosting provider: %w", err))
	}
	if err := provider.CheckAuth(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("hosting provider auth failed: %w", err))
	}
	return provider, nil
}

// CreatePR creates a PR for a task.
func (s *hostingServer) CreatePR(
	ctx context.Context,
	req *connect.Request[orcv1.CreatePRRequest],
) (*connect.Response[orcv1.CreatePRResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	// Check if PR already exists
	existingPR, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil && !errors.Is(err, hosting.ErrNoPRFound) {
		s.logger.Warn("failed to check for existing PR", "error", err)
	}
	if err == nil && existingPR != nil {
		return connect.NewResponse(&orcv1.CreatePRResponse{
			Pr:      prToProto(existingPR),
			Created: false,
		}), nil
	}

	// Build PR options
	opts := hosting.PRCreateOptions{
		Head:                t.Branch,
		Draft:               req.Msg.Draft,
		Labels:              req.Msg.Labels,
		Reviewers:           req.Msg.Reviewers,
		TeamReviewers:       req.Msg.TeamReviewers,
		Assignees:           req.Msg.Assignees,
		MaintainerCanModify: req.Msg.MaintainerCanModify,
	}

	if req.Msg.Title != nil && *req.Msg.Title != "" {
		opts.Title = *req.Msg.Title
	} else {
		opts.Title = fmt.Sprintf("[orc] %s: %s", t.Id, t.Title)
	}

	if req.Msg.Body != nil && *req.Msg.Body != "" {
		opts.Body = *req.Msg.Body
	} else {
		opts.Body = buildPRBodyForTaskProto(t)
	}

	if req.Msg.Base != nil && *req.Msg.Base != "" {
		opts.Base = *req.Msg.Base
	} else {
		opts.Base = "main"
	}

	pr, err := provider.CreatePR(ctx, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create PR: %w", err))
	}

	return connect.NewResponse(&orcv1.CreatePRResponse{
		Pr:      prToProto(pr),
		Created: true,
	}), nil
}

// GetPR gets the PR for a task.
func (s *hostingServer) GetPR(
	ctx context.Context,
	req *connect.Request[orcv1.GetPRRequest],
) (*connect.Response[orcv1.GetPRResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	return connect.NewResponse(&orcv1.GetPRResponse{
		Pr: prToProto(pr),
	}), nil
}

// MergePR merges the PR for a task.
func (s *hostingServer) MergePR(
	ctx context.Context,
	req *connect.Request[orcv1.MergePRRequest],
) (*connect.Response[orcv1.MergePRResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	method := "squash"
	if req.Msg.Method != nil && *req.Msg.Method != "" {
		method = *req.Msg.Method
	}

	err = provider.MergePR(ctx, pr.Number, hosting.PRMergeOptions{
		Method:       method,
		DeleteBranch: true,
	})
	if err != nil {
		errMsg := err.Error()
		return connect.NewResponse(&orcv1.MergePRResponse{
			Merged: false,
			Error:  &errMsg,
		}), nil
	}

	// Update task status to completed
	t.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(t); err != nil {
		s.logger.Error("failed to update task status after merge", "task", req.Msg.TaskId, "error", err)
	}

	return connect.NewResponse(&orcv1.MergePRResponse{
		Merged: true,
	}), nil
}

// SyncComments syncs local review comments to PR.
func (s *hostingServer) SyncComments(
	ctx context.Context,
	req *connect.Request[orcv1.SyncCommentsRequest],
) (*connect.Response[orcv1.SyncCommentsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	// Get local review comments from the project-specific backend
	pdb := backend.DB()

	comments, err := pdb.ListReviewComments(req.Msg.TaskId, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list review comments: %w", err))
	}

	if len(comments) == 0 {
		return connect.NewResponse(&orcv1.SyncCommentsResponse{
			Result: &orcv1.SyncResult{Total: 0},
		}), nil
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	result := &orcv1.SyncResult{Total: int32(len(comments))}

	for _, c := range comments {
		// Skip comments without file path or resolved comments
		if c.FilePath == "" {
			continue
		}
		if c.Status == db.CommentStatusResolved || c.Status == db.CommentStatusWontFix {
			result.Resolved++
			continue
		}

		body := formatReviewCommentForPR(c)

		_, err := provider.CreatePRComment(ctx, pr.Number, hosting.PRCommentCreate{
			Body: body,
			Path: c.FilePath,
			Line: c.LineNumber,
		})
		if err != nil {
			s.logger.Warn("failed to sync comment", "comment_id", c.ID, "error", err)
		} else {
			result.Imported++
		}
	}

	return connect.NewResponse(&orcv1.SyncCommentsResponse{
		Result: result,
	}), nil
}

// ImportComments imports PR comments as local review comments.
func (s *hostingServer) ImportComments(
	ctx context.Context,
	req *connect.Request[orcv1.ImportCommentsRequest],
) (*connect.Response[orcv1.ImportCommentsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	prComments, err := provider.ListPRComments(ctx, pr.Number)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list PR comments: %w", err))
	}

	if len(prComments) == 0 {
		return connect.NewResponse(&orcv1.ImportCommentsResponse{
			Imported: 0,
		}), nil
	}

	// Get the project-specific database from the resolved backend
	pdb := backend.DB()

	// Get existing comments for deduplication
	existingComments, err := pdb.ListReviewComments(req.Msg.TaskId, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list existing comments: %w", err))
	}

	existingMap := make(map[string]bool)
	for _, c := range existingComments {
		key := fmt.Sprintf("%s:%d:%s", c.FilePath, c.LineNumber, c.Content[:min(len(c.Content), 50)])
		existingMap[key] = true
	}

	latestRound, _ := pdb.GetLatestReviewRound(req.Msg.TaskId)
	newRound := latestRound + 1

	var imported int32
	var importedComments []*orcv1.PRComment

	for _, pc := range prComments {
		// Skip reply comments
		if pc.ThreadID != 0 {
			continue
		}

		// Check for duplicate
		key := fmt.Sprintf("%s:%d:%s", pc.Path, pc.Line, pc.Body[:min(len(pc.Body), 50)])
		if existingMap[key] {
			continue
		}

		comment := &db.ReviewComment{
			TaskID:      req.Msg.TaskId,
			ReviewRound: newRound,
			FilePath:    pc.Path,
			LineNumber:  pc.Line,
			Content:     fmt.Sprintf("[@%s] %s", pc.Author, pc.Body),
			Severity:    db.SeverityIssue,
			Status:      db.CommentStatusOpen,
		}

		if err := pdb.CreateReviewComment(comment); err != nil {
			s.logger.Warn("failed to import PR comment", "error", err, "path", pc.Path, "line", pc.Line)
		} else {
			imported++
			existingMap[key] = true
			importedComments = append(importedComments, prCommentToProto(&pc))
		}
	}

	return connect.NewResponse(&orcv1.ImportCommentsResponse{
		Imported: imported,
		Comments: importedComments,
	}), nil
}

// GetChecks gets CI check runs for a task's PR.
func (s *hostingServer) GetChecks(
	ctx context.Context,
	req *connect.Request[orcv1.GetChecksRequest],
) (*connect.Response[orcv1.GetChecksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	checks, err := provider.GetCheckRuns(ctx, t.Branch)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get check runs: %w", err))
	}

	// Calculate summary
	summary := &orcv1.CheckSummary{Total: int32(len(checks))}
	var protoChecks []*orcv1.CheckRun

	for _, check := range checks {
		protoChecks = append(protoChecks, checkRunToProto(&check))

		switch check.Status {
		case "completed":
			switch check.Conclusion {
			case "success":
				summary.Passed++
			case "neutral", "skipped", "cancelled", "action_required":
				summary.Neutral++
			default:
				summary.Failed++
			}
		default:
			summary.Pending++
		}
	}

	return connect.NewResponse(&orcv1.GetChecksResponse{
		Checks:  protoChecks,
		Summary: summary,
	}), nil
}

// RefreshPR refreshes PR status for a task.
func (s *hostingServer) RefreshPR(
	ctx context.Context,
	req *connect.Request[orcv1.RefreshPRRequest],
) (*connect.Response[orcv1.RefreshPRResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	// Get PR status summary
	summary, err := provider.GetPRStatusSummary(ctx, pr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get PR status: %w", err))
	}

	// Determine PR status
	prStatus := DeterminePRStatusProto(pr, summary)

	// Update task PR info
	if t.Pr == nil {
		t.Pr = &orcv1.PRInfo{}
	}
	t.Pr.Url = &pr.HTMLURL
	prNumber := int32(pr.Number)
	t.Pr.Number = &prNumber
	t.Pr.Status = prStatus
	t.Pr.ChecksStatus = &summary.ChecksStatus
	t.Pr.Mergeable = summary.Mergeable
	t.Pr.ReviewCount = int32(summary.ReviewCount)
	t.Pr.ApprovalCount = int32(summary.ApprovalCount)
	t.Pr.LastCheckedAt = timestamppb.Now()

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save task: %w", err))
	}

	return connect.NewResponse(&orcv1.RefreshPRResponse{
		Pr: prToProto(pr),
	}), nil
}

// ReplyToComment replies to a PR comment thread.
func (s *hostingServer) ReplyToComment(
	ctx context.Context,
	req *connect.Request[orcv1.ReplyToCommentRequest],
) (*connect.Response[orcv1.ReplyToCommentResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	reply, err := provider.ReplyToComment(ctx, pr.Number, req.Msg.CommentId, req.Msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reply to comment: %w", err))
	}

	return connect.NewResponse(&orcv1.ReplyToCommentResponse{
		Comment: prCommentToProto(reply),
	}), nil
}

// maxCommentBodySize is the maximum size of comment body to include in retry context.
const maxCommentBodySize = 10 * 1024 // 10KB

// AutofixComment triggers an auto-fix for a PR comment.
// This fetches the comment from GitHub, sets up retry context, and spawns an executor.
// The operation returns immediately with success=true once the executor is spawned.
func (s *hostingServer) AutofixComment(
	ctx context.Context,
	req *connect.Request[orcv1.AutofixCommentRequest],
) (*connect.Response[orcv1.AutofixCommentResponse], error) {
	// Validate required fields
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load the task
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	// Validate task state
	if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already running"))
	}
	if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task already completed"))
	}
	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task has no branch"))
	}

	// Get hosting provider - use factory if provided (for tests), otherwise default getProvider
	var provider hosting.Provider
	if s.clientFactory != nil {
		provider, err = s.clientFactory(ctx)
	} else {
		provider, err = s.getProvider(ctx)
	}
	if err != nil {
		// Check for auth errors
		if strings.Contains(err.Error(), "not logged in") || strings.Contains(err.Error(), "auth") {
			return nil, connect.NewError(connect.CodeUnauthenticated,
				errors.New("hosting provider not authenticated"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get hosting provider: %w", err))
	}

	// Find PR for the task branch to get PR number
	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("find PR by branch: %w", err))
	}

	// Fetch the comment from the hosting provider
	comment, err := provider.GetPRComment(ctx, pr.Number, req.Msg.CommentId)
	if err != nil {
		// Check for specific error types
		errStr := err.Error()
		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "404") {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("comment not found: %d", req.Msg.CommentId))
		}
		if strings.Contains(errStr, "rate limit") {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.New("API rate limited, try again later"))
		}
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("fetch comment: %w", err))
	}

	// Build retry context from the comment
	retryContext := buildAutofixRetryContext(comment)

	// Ensure execution state exists
	task.EnsureExecutionProto(t)

	// Get current retry count for tracking
	var currentRetries int32
	if t.Quality != nil {
		currentRetries = t.Quality.TotalRetries
	}

	// Set retry context pointing to implement phase
	task.SetRetryContextProto(t.Execution, "implement", "", "autofix PR comment", retryContext, currentRetries+1)

	// Update task status to running
	task.MarkStartedProto(t)
	implement := "implement"
	t.CurrentPhase = &implement

	// Increment retry counter
	task.EnsureQualityMetricsProto(t)
	t.Quality.TotalRetries++

	// Save task before spawning executor
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish task updated event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	// Spawn executor if callback is set
	// We check for immediate failures (spawn errors) but don't block on slow executors.
	// This satisfies SC-4's "returns immediately" while also catching spawn failures.
	if s.taskExecutor != nil {
		taskID := t.Id
		errChan := make(chan error, 1)
		go func() {
			errChan <- s.taskExecutor(taskID, req.Msg.GetProjectId())
		}()

		// Wait briefly for immediate spawn failures, but don't block on slow executors
		select {
		case err := <-errChan:
			if err != nil {
				// Executor failed to spawn - revert task state
				t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
				errStr := fmt.Sprintf("failed to spawn executor: %v", err)
				task.EnsureExecutionProto(t)
				t.Execution.Error = &errStr
				task.UpdateTimestampProto(t)
				if saveErr := backend.SaveTask(t); saveErr != nil {
					if s.logger != nil {
						s.logger.Error("failed to save task after executor failure",
							"task", taskID, "error", saveErr)
					}
				}
				// Publish failure event
				if s.publisher != nil {
					s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, taskID, t))
				}
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to spawn executor: %w", err))
			}
			// Executor completed immediately (unusual but valid)
		case <-time.After(10 * time.Millisecond):
			// Executor is still running, return success
		}
	}

	// Return success - autofix has started
	return connect.NewResponse(&orcv1.AutofixCommentResponse{
		Result: &orcv1.AutofixResult{
			Success: true,
		},
	}), nil
}

// buildAutofixRetryContext builds the retry context string from a PR comment.
// This is what gets injected into the {{RETRY_CONTEXT}} template variable.
func buildAutofixRetryContext(comment *hosting.PRComment) string {
	var sb strings.Builder

	sb.WriteString("## PR Feedback to Address\n\n")

	// Add file and line info if available
	if comment.Path != "" {
		sb.WriteString(fmt.Sprintf("**%s", comment.Path))
		if comment.Line > 0 {
			sb.WriteString(fmt.Sprintf(":%d", comment.Line))
		}
		sb.WriteString("**")
		if comment.Author != "" {
			sb.WriteString(fmt.Sprintf(" (@%s)", comment.Author))
		}
		sb.WriteString("\n")
	} else if comment.Author != "" {
		sb.WriteString(fmt.Sprintf("**@%s**\n", comment.Author))
	}

	// Add the comment body
	body := comment.Body
	if len(body) > maxCommentBodySize {
		body = body[:maxCommentBodySize] + "\n\n(truncated)"
	}
	sb.WriteString("> ")
	// Indent the body for blockquote
	sb.WriteString(strings.ReplaceAll(body, "\n", "\n> "))
	sb.WriteString("\n\n---\n\n")
	sb.WriteString("Please address this feedback and make the necessary changes.\n")

	return sb.String()
}

// Helper functions

func buildPRBodyForTaskProto(t *orcv1.Task) string {
	var sb strings.Builder
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("Task: **%s**\n\n", t.Title))

	desc := task.GetDescriptionProto(t)
	if desc != "" {
		sb.WriteString("### Description\n\n")
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Generated by [orc](https://github.com/randalmurphal/orc)*\n")

	return sb.String()
}

func formatReviewCommentForPR(c db.ReviewComment) string {
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

func prToProto(pr *hosting.PR) *orcv1.PR {
	result := &orcv1.PR{
		Number:    int32(pr.Number),
		Title:     pr.Title,
		Body:      pr.Body,
		State:     pr.State,
		HtmlUrl:   pr.HTMLURL,
		Head:      pr.HeadBranch,
		Base:      pr.BaseBranch,
		Draft:     pr.Draft,
		HeadSha:   pr.HeadSHA,
		Labels:    pr.Labels,
		Assignees: pr.Assignees,
	}

	// Parse timestamps (GitHub returns ISO 8601 strings)
	if pr.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, pr.CreatedAt); err == nil {
			result.CreatedAt = timestamppb.New(t)
		}
	}
	if pr.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, pr.UpdatedAt); err == nil {
			result.UpdatedAt = timestamppb.New(t)
		}
	}

	// Mergeable is a bool, convert to *bool for proto optional
	result.Mergeable = &pr.Mergeable

	return result
}

func prCommentToProto(c *hosting.PRComment) *orcv1.PRComment {
	result := &orcv1.PRComment{
		Id:     c.ID,
		Body:   c.Body,
		Author: c.Author,
	}
	// Parse string timestamp
	if c.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
			result.CreatedAt = timestamppb.New(t)
		}
	}
	if c.Path != "" {
		result.Path = &c.Path
	}
	if c.Line > 0 {
		line := int32(c.Line)
		result.Line = &line
	}
	if c.ThreadID != 0 {
		result.ThreadId = &c.ThreadID
	}
	return result
}

func checkRunToProto(c *hosting.CheckRun) *orcv1.CheckRun {
	result := &orcv1.CheckRun{
		Id:     c.ID,
		Name:   c.Name,
		Status: c.Status,
	}
	if c.Conclusion != "" {
		result.Conclusion = &c.Conclusion
	}
	// Note: Go type hosting.CheckRun doesn't have StartedAt, CompletedAt, HTMLURL
	// Proto fields left unset (optional fields remain nil)
	return result
}
