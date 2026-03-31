package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ListComments returns all comments for a task.
func (s *taskServer) ListComments(
	ctx context.Context,
	req *connect.Request[orcv1.ListCommentsRequest],
) (*connect.Response[orcv1.ListCommentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	var comments []db.TaskComment

	if req.Msg.AuthorType != nil && *req.Msg.AuthorType != orcv1.AuthorType_AUTHOR_TYPE_UNSPECIFIED {
		authorType := protoToAuthorType(*req.Msg.AuthorType)
		comments, err = pdb.ListTaskCommentsByAuthorType(req.Msg.TaskId, authorType)
	} else if req.Msg.Phase != nil && *req.Msg.Phase != "" {
		comments, err = pdb.ListTaskCommentsByPhase(req.Msg.TaskId, *req.Msg.Phase)
	} else {
		comments, err = pdb.ListTaskComments(req.Msg.TaskId)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list comments: %w", err))
	}

	protoComments := make([]*orcv1.TaskComment, len(comments))
	for i, c := range comments {
		protoComments[i] = taskCommentToProto(&c)
	}

	return connect.NewResponse(&orcv1.ListCommentsResponse{
		Comments: protoComments,
	}), nil
}

// CreateComment creates a new comment on a task.
func (s *taskServer) CreateComment(
	ctx context.Context,
	req *connect.Request[orcv1.CreateCommentRequest],
) (*connect.Response[orcv1.CreateCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	author := "user"
	if req.Msg.Author != nil {
		author = *req.Msg.Author
	}

	authorType := db.AuthorTypeHuman
	if req.Msg.AuthorType != nil {
		authorType = protoToAuthorType(*req.Msg.AuthorType)
	}

	comment := &db.TaskComment{
		TaskID:     req.Msg.TaskId,
		Content:    req.Msg.Content,
		Author:     author,
		AuthorType: authorType,
	}

	if req.Msg.Phase != nil {
		comment.Phase = *req.Msg.Phase
	}

	if err := pdb.CreateTaskComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save comment: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateCommentResponse{
		Comment: taskCommentToProto(comment),
	}), nil
}

// UpdateComment updates an existing comment.
func (s *taskServer) UpdateComment(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateCommentRequest],
) (*connect.Response[orcv1.UpdateCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	comment, err := pdb.GetTaskComment(req.Msg.CommentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get comment: %w", err))
	}
	if comment == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("comment not found"))
	}

	if req.Msg.Content != nil {
		comment.Content = *req.Msg.Content
	}
	if req.Msg.Phase != nil {
		comment.Phase = *req.Msg.Phase
	}

	if err := pdb.UpdateTaskComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update comment: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateCommentResponse{
		Comment: taskCommentToProto(comment),
	}), nil
}

// DeleteComment deletes a comment.
func (s *taskServer) DeleteComment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteCommentRequest],
) (*connect.Response[orcv1.DeleteCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	if err := pdb.DeleteTaskComment(req.Msg.CommentId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete comment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteCommentResponse{
		Message: "Comment deleted",
	}), nil
}

// ListReviewComments returns all review comments for a task.
func (s *taskServer) ListReviewComments(
	ctx context.Context,
	req *connect.Request[orcv1.ListReviewCommentsRequest],
) (*connect.Response[orcv1.ListReviewCommentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	status := ""
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.CommentStatus_COMMENT_STATUS_UNSPECIFIED {
		status = protoToCommentStatus(*req.Msg.Status)
	}

	comments, err := pdb.ListReviewComments(req.Msg.TaskId, status)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list review comments: %w", err))
	}

	protoComments := make([]*orcv1.ReviewComment, len(comments))
	for i, c := range comments {
		protoComments[i] = reviewCommentToProto(&c)
	}

	return connect.NewResponse(&orcv1.ListReviewCommentsResponse{
		Comments: protoComments,
	}), nil
}

// CreateReviewComment creates a new review comment.
func (s *taskServer) CreateReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.CreateReviewCommentRequest],
) (*connect.Response[orcv1.CreateReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	severity := db.SeveritySuggestion
	if req.Msg.Severity != orcv1.CommentSeverity_COMMENT_SEVERITY_UNSPECIFIED {
		severity = protoToCommentSeverity(req.Msg.Severity)
	}

	reviewRound := int(req.Msg.ReviewRound)
	if reviewRound == 0 {
		latest, err := pdb.GetLatestReviewRound(req.Msg.TaskId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get review round: %w", err))
		}
		reviewRound = latest
		if reviewRound == 0 {
			reviewRound = 1
		}
	}

	comment := &db.ReviewComment{
		TaskID:      req.Msg.TaskId,
		ReviewRound: reviewRound,
		Content:     req.Msg.Content,
		Severity:    severity,
	}

	if req.Msg.FilePath != nil {
		comment.FilePath = *req.Msg.FilePath
	}
	if req.Msg.LineNumber != nil {
		comment.LineNumber = int(*req.Msg.LineNumber)
	}

	if err := pdb.CreateReviewComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create review comment: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateReviewCommentResponse{
		Comment: reviewCommentToProto(comment),
	}), nil
}

// UpdateReviewComment updates an existing review comment.
func (s *taskServer) UpdateReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateReviewCommentRequest],
) (*connect.Response[orcv1.UpdateReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	comment, err := pdb.GetReviewComment(req.Msg.CommentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get comment: %w", err))
	}
	if comment == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("comment not found"))
	}

	if req.Msg.Content != nil {
		comment.Content = *req.Msg.Content
	}
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.CommentStatus_COMMENT_STATUS_UNSPECIFIED {
		comment.Status = db.ReviewCommentStatus(protoToCommentStatus(*req.Msg.Status))
		if comment.Status == db.CommentStatusResolved || comment.Status == db.CommentStatusWontFix {
			now := time.Now()
			comment.ResolvedAt = &now
		}
	}

	if err := pdb.UpdateReviewComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update comment: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateReviewCommentResponse{
		Comment: reviewCommentToProto(comment),
	}), nil
}

// DeleteReviewComment deletes a review comment.
func (s *taskServer) DeleteReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteReviewCommentRequest],
) (*connect.Response[orcv1.DeleteReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	pdb := backend.DB()

	if err := pdb.DeleteReviewComment(req.Msg.CommentId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete comment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteReviewCommentResponse{
		Message: "Review comment deleted",
	}), nil
}
