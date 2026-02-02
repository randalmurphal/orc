// Package api provides the Connect RPC and REST API server for orc.
// This file implements the FeedbackService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

// TaskPauser interface for dependency injection of task pause functionality.
type TaskPauser interface {
	PauseTask(taskID, projectID string) error
}

// feedbackServer implements the FeedbackServiceHandler interface.
type feedbackServer struct {
	orcv1connect.UnimplementedFeedbackServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	publisher    events.Publisher
	logger       *slog.Logger
	taskPauser   TaskPauser
}

// SetProjectCache sets the project cache for multi-project support.
func (s *feedbackServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
func (s *feedbackServer) getBackend(projectID string) (storage.Backend, error) {
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

// NewFeedbackServer creates a new FeedbackService handler.
func NewFeedbackServer(
	backend storage.Backend,
	publisher events.Publisher,
	logger *slog.Logger,
) orcv1connect.FeedbackServiceHandler {
	return &feedbackServer{
		backend:   backend,
		publisher: publisher,
		logger:    logger,
	}
}

// NewFeedbackServerWithPauser creates a new FeedbackService handler with task pauser support.
func NewFeedbackServerWithPauser(
	backend storage.Backend,
	publisher events.Publisher,
	logger *slog.Logger,
	pauser TaskPauser,
) orcv1connect.FeedbackServiceHandler {
	return &feedbackServer{
		backend:    backend,
		publisher:  publisher,
		logger:     logger,
		taskPauser: pauser,
	}
}

// AddFeedback adds feedback for a task.
func (s *feedbackServer) AddFeedback(
	ctx context.Context,
	req *connect.Request[orcv1.AddFeedbackRequest],
) (*connect.Response[orcv1.AddFeedbackResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	taskID := req.Msg.TaskId
	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task_id is required"))
	}

	// Validate task exists
	task, err := backend.LoadTask(taskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	// Validate feedback type
	if req.Msg.Type == orcv1.FeedbackType_FEEDBACK_TYPE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("feedback type is required"))
	}

	// Validate timing
	if req.Msg.Timing == orcv1.FeedbackTiming_FEEDBACK_TIMING_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("feedback timing is required"))
	}

	// Validate text
	if req.Msg.Text == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("feedback text is required"))
	}

	// For inline type, require file and line
	if req.Msg.Type == orcv1.FeedbackType_FEEDBACK_TYPE_INLINE {
		if req.Msg.File == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file is required for inline feedback"))
		}
		if req.Msg.Line == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("line is required for inline feedback"))
		}
	}

	// Convert proto type to storage type
	feedback := &storage.Feedback{
		TaskID:   taskID,
		Type:     feedbackTypeToString(req.Msg.Type),
		Text:     req.Msg.Text,
		Timing:   feedbackTimingToString(req.Msg.Timing),
		File:     req.Msg.File,
		Line:     int(req.Msg.Line),
		Received: false,
	}

	// Save feedback
	if err := backend.SaveFeedback(feedback); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save feedback: %w", err))
	}

	// If timing is NOW and task is running, pause the task
	if req.Msg.Timing == orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW &&
		task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING &&
		s.taskPauser != nil {
		if err := s.taskPauser.PauseTask(taskID, req.Msg.GetProjectId()); err != nil {
			// Log but don't fail - feedback was saved
			if s.logger != nil {
				s.logger.Warn("failed to pause task for NOW feedback", "task_id", taskID, "error", err)
			}
		}
	}

	return connect.NewResponse(&orcv1.AddFeedbackResponse{
		Feedback: feedbackToProto(feedback),
	}), nil
}

// ListFeedback lists feedback for a task.
func (s *feedbackServer) ListFeedback(
	ctx context.Context,
	req *connect.Request[orcv1.ListFeedbackRequest],
) (*connect.Response[orcv1.ListFeedbackResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	taskID := req.Msg.TaskId
	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task_id is required"))
	}

	// Validate task exists
	if _, err := backend.LoadTask(taskID); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	feedbackList, err := backend.ListFeedback(taskID, req.Msg.ExcludeReceived)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list feedback: %w", err))
	}

	protoFeedback := make([]*orcv1.Feedback, len(feedbackList))
	for i, f := range feedbackList {
		protoFeedback[i] = feedbackToProto(f)
	}

	return connect.NewResponse(&orcv1.ListFeedbackResponse{
		Feedback: protoFeedback,
	}), nil
}

// SendFeedback sends all pending feedback to the agent.
func (s *feedbackServer) SendFeedback(
	ctx context.Context,
	req *connect.Request[orcv1.SendFeedbackRequest],
) (*connect.Response[orcv1.SendFeedbackResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	taskID := req.Msg.TaskId
	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task_id is required"))
	}

	// Validate task exists
	if _, err := backend.LoadTask(taskID); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	// Mark all pending feedback as received
	count, err := backend.MarkFeedbackReceived(taskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send feedback: %w", err))
	}

	return connect.NewResponse(&orcv1.SendFeedbackResponse{
		SentCount: int32(count),
	}), nil
}

// DeleteFeedback deletes specific feedback.
func (s *feedbackServer) DeleteFeedback(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteFeedbackRequest],
) (*connect.Response[orcv1.DeleteFeedbackResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	taskID := req.Msg.TaskId
	feedbackID := req.Msg.FeedbackId

	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task_id is required"))
	}
	if feedbackID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("feedback_id is required"))
	}

	// Validate task exists
	if _, err := backend.LoadTask(taskID); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	// Delete feedback
	if err := backend.DeleteFeedback(taskID, feedbackID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete feedback: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteFeedbackResponse{}), nil
}

// Helper functions for type conversion

func feedbackTypeToString(t orcv1.FeedbackType) string {
	switch t {
	case orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL:
		return "general"
	case orcv1.FeedbackType_FEEDBACK_TYPE_INLINE:
		return "inline"
	case orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL:
		return "approval"
	case orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION:
		return "direction"
	default:
		return "general"
	}
}

func stringToFeedbackType(s string) orcv1.FeedbackType {
	switch s {
	case "general":
		return orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL
	case "inline":
		return orcv1.FeedbackType_FEEDBACK_TYPE_INLINE
	case "approval":
		return orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL
	case "direction":
		return orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION
	default:
		return orcv1.FeedbackType_FEEDBACK_TYPE_UNSPECIFIED
	}
}

func feedbackTimingToString(t orcv1.FeedbackTiming) string {
	switch t {
	case orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW:
		return "now"
	case orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE:
		return "when_done"
	case orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL:
		return "manual"
	default:
		return "when_done"
	}
}

func stringToFeedbackTiming(s string) orcv1.FeedbackTiming {
	switch s {
	case "now":
		return orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW
	case "when_done":
		return orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE
	case "manual":
		return orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL
	default:
		return orcv1.FeedbackTiming_FEEDBACK_TIMING_UNSPECIFIED
	}
}

func feedbackToProto(f *storage.Feedback) *orcv1.Feedback {
	proto := &orcv1.Feedback{
		Id:       f.ID,
		TaskId:   f.TaskID,
		Type:     stringToFeedbackType(f.Type),
		Text:     f.Text,
		Timing:   stringToFeedbackTiming(f.Timing),
		File:     f.File,
		Line:     int32(f.Line),
		Received: f.Received,
	}
	if f.SentAt != nil {
		proto.SentAt = timestamppb.New(*f.SentAt)
	}
	return proto
}
