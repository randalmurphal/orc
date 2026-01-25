// Package api provides the Connect RPC and REST API server for orc.
// This file implements the GitHubService Connect RPC service.
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
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// githubServer implements the GitHubServiceHandler interface.
type githubServer struct {
	orcv1connect.UnimplementedGitHubServiceHandler
	backend    storage.Backend
	projectDir string
	logger     *slog.Logger
}

// NewGitHubServer creates a new GitHubService handler.
func NewGitHubServer(
	backend storage.Backend,
	projectDir string,
	logger *slog.Logger,
) orcv1connect.GitHubServiceHandler {
	return &githubServer{
		backend:    backend,
		projectDir: projectDir,
		logger:     logger,
	}
}

// getClient creates a GitHub client, checking auth first.
func (s *githubServer) getClient(ctx context.Context) (*github.Client, error) {
	if err := github.CheckGHAuth(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("GitHub CLI not authenticated. Run 'gh auth login' first"))
	}

	client, err := github.NewClient(s.projectDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create GitHub client: %w", err))
	}
	return client, nil
}

// CreatePR creates a PR for a task.
func (s *githubServer) CreatePR(
	ctx context.Context,
	req *connect.Request[orcv1.CreatePRRequest],
) (*connect.Response[orcv1.CreatePRResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	// Check if PR already exists
	existingPR, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil && !errors.Is(err, github.ErrNoPRFound) {
		s.logger.Warn("failed to check for existing PR", "error", err)
	}
	if err == nil && existingPR != nil {
		return connect.NewResponse(&orcv1.CreatePRResponse{
			Pr:      ghPRToProto(existingPR),
			Created: false,
		}), nil
	}

	// Build PR options
	opts := github.PRCreateOptions{
		Head:      t.Branch,
		Draft:     req.Msg.Draft,
		Labels:    req.Msg.Labels,
		Reviewers: req.Msg.Reviewers,
	}

	if req.Msg.Title != nil && *req.Msg.Title != "" {
		opts.Title = *req.Msg.Title
	} else {
		opts.Title = fmt.Sprintf("[orc] %s: %s", t.ID, t.Title)
	}

	if req.Msg.Body != nil && *req.Msg.Body != "" {
		opts.Body = *req.Msg.Body
	} else {
		opts.Body = buildPRBodyForTask(t)
	}

	if req.Msg.Base != nil && *req.Msg.Base != "" {
		opts.Base = *req.Msg.Base
	} else {
		opts.Base = "main"
	}

	pr, err := client.CreatePR(ctx, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create PR: %w", err))
	}

	return connect.NewResponse(&orcv1.CreatePRResponse{
		Pr:      ghPRToProto(pr),
		Created: true,
	}), nil
}

// GetPR gets the PR for a task.
func (s *githubServer) GetPR(
	ctx context.Context,
	req *connect.Request[orcv1.GetPRRequest],
) (*connect.Response[orcv1.GetPRResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	return connect.NewResponse(&orcv1.GetPRResponse{
		Pr: ghPRToProto(pr),
	}), nil
}

// MergePR merges the PR for a task.
func (s *githubServer) MergePR(
	ctx context.Context,
	req *connect.Request[orcv1.MergePRRequest],
) (*connect.Response[orcv1.MergePRResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	method := "squash"
	if req.Msg.Method != nil && *req.Msg.Method != "" {
		method = *req.Msg.Method
	}

	err = client.MergePR(ctx, pr.Number, github.PRMergeOptions{
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
	t.Status = task.StatusCompleted
	if err := s.backend.SaveTask(t); err != nil {
		s.logger.Error("failed to update task status after merge", "task", req.Msg.TaskId, "error", err)
	}

	return connect.NewResponse(&orcv1.MergePRResponse{
		Merged: true,
	}), nil
}

// SyncComments syncs local review comments to PR.
func (s *githubServer) SyncComments(
	ctx context.Context,
	req *connect.Request[orcv1.SyncCommentsRequest],
) (*connect.Response[orcv1.SyncCommentsResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	// Get local review comments
	pdb, err := db.OpenProject(s.projectDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	comments, err := pdb.ListReviewComments(req.Msg.TaskId, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list review comments: %w", err))
	}

	if len(comments) == 0 {
		return connect.NewResponse(&orcv1.SyncCommentsResponse{
			Result: &orcv1.SyncResult{Total: 0},
		}), nil
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
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

		_, err := client.CreatePRComment(ctx, pr.Number, github.PRCommentCreate{
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
func (s *githubServer) ImportComments(
	ctx context.Context,
	req *connect.Request[orcv1.ImportCommentsRequest],
) (*connect.Response[orcv1.ImportCommentsResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	prComments, err := client.ListPRComments(ctx, pr.Number)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list PR comments: %w", err))
	}

	if len(prComments) == 0 {
		return connect.NewResponse(&orcv1.ImportCommentsResponse{
			Imported: 0,
		}), nil
	}

	pdb, err := db.OpenProject(s.projectDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

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
			importedComments = append(importedComments, ghPRCommentToProto(&pc))
		}
	}

	return connect.NewResponse(&orcv1.ImportCommentsResponse{
		Imported: imported,
		Comments: importedComments,
	}), nil
}

// GetChecks gets CI check runs for a task's PR.
func (s *githubServer) GetChecks(
	ctx context.Context,
	req *connect.Request[orcv1.GetChecksRequest],
) (*connect.Response[orcv1.GetChecksResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	checks, err := client.GetCheckRuns(ctx, t.Branch)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get check runs: %w", err))
	}

	// Calculate summary
	summary := &orcv1.CheckSummary{Total: int32(len(checks))}
	var protoChecks []*orcv1.CheckRun

	for _, check := range checks {
		protoChecks = append(protoChecks, ghCheckRunToProto(&check))

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
func (s *githubServer) RefreshPR(
	ctx context.Context,
	req *connect.Request[orcv1.RefreshPRRequest],
) (*connect.Response[orcv1.RefreshPRResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	// Get PR status summary
	summary, err := client.GetPRStatusSummary(ctx, pr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get PR status: %w", err))
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

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save task: %w", err))
	}

	return connect.NewResponse(&orcv1.RefreshPRResponse{
		Pr: ghPRToProto(pr),
	}), nil
}

// ReplyToComment replies to a PR comment thread.
func (s *githubServer) ReplyToComment(
	ctx context.Context,
	req *connect.Request[orcv1.ReplyToCommentRequest],
) (*connect.Response[orcv1.ReplyToCommentResponse], error) {
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	if t.Branch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task has no branch"))
	}

	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}

	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no PR found for task branch"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find PR: %w", err))
	}

	reply, err := client.ReplyToComment(ctx, pr.Number, req.Msg.CommentId, req.Msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reply to comment: %w", err))
	}

	return connect.NewResponse(&orcv1.ReplyToCommentResponse{
		Comment: ghPRCommentToProto(reply),
	}), nil
}

// AutofixComment triggers an auto-fix for a PR comment.
// Note: This is a complex operation that involves rerunning the task.
// The Connect RPC version returns the result without starting background execution.
func (s *githubServer) AutofixComment(
	ctx context.Context,
	req *connect.Request[orcv1.AutofixCommentRequest],
) (*connect.Response[orcv1.AutofixCommentResponse], error) {
	// Autofix requires the full server context (running tasks map, etc.)
	// For now, return unimplemented - the REST handler should be used
	return nil, connect.NewError(connect.CodeUnimplemented,
		fmt.Errorf("autofix via Connect RPC not implemented - use REST endpoint"))
}

// Helper functions

func buildPRBodyForTask(t *task.Task) string {
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

func ghPRToProto(pr *github.PR) *orcv1.PR {
	result := &orcv1.PR{
		Number:  int32(pr.Number),
		Title:   pr.Title,
		Body:    pr.Body,
		State:   pr.State,
		HtmlUrl: pr.HTMLURL,
		Head:    pr.HeadBranch,
		Base:    pr.BaseBranch,
		Draft:   pr.Draft,
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

func ghPRCommentToProto(c *github.PRComment) *orcv1.PRComment {
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

func ghCheckRunToProto(c *github.CheckRun) *orcv1.CheckRun {
	result := &orcv1.CheckRun{
		Id:     c.ID,
		Name:   c.Name,
		Status: c.Status,
	}
	if c.Conclusion != "" {
		result.Conclusion = &c.Conclusion
	}
	// Note: Go type github.CheckRun doesn't have StartedAt, CompletedAt, HTMLURL
	// Proto fields left unset (optional fields remain nil)
	return result
}
