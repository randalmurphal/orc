package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// RetryTask retries a failed task from a specific phase.
func (s *taskServer) RetryTask(
	ctx context.Context,
	req *connect.Request[orcv1.RetryTaskRequest],
) (*connect.Response[orcv1.RetryTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task can be retried
	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be retried"))
	}

	// Set up retry context
	fromPhase := "implement"
	if req.Msg.FromPhase != nil && *req.Msg.FromPhase != "" {
		fromPhase = *req.Msg.FromPhase
	}

	// Ensure execution state is initialized
	task.EnsureExecutionProto(t)

	// Get current retry count
	var currentRetries int32
	if t.Quality != nil {
		currentRetries = t.Quality.TotalRetries
	}

	// Set retry state in task metadata
	instructions := ""
	if req.Msg.Instructions != nil {
		instructions = *req.Msg.Instructions
	}
	task.SetRetryState(t, fromPhase, "", "manual retry", instructions, currentRetries+1)

	// Reset status
	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if t.Execution != nil {
		t.Execution.Error = nil
	}
	task.UpdateTimestampProto(t)

	// Increment retry counter
	task.EnsureQualityMetricsProto(t)
	t.Quality.TotalRetries++

	if err := transitionTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), originalTask, t, "task_retry"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.RetryTaskResponse{
		Task:    t,
		Message: fmt.Sprintf("Task will retry from phase: %s", fromPhase),
	}), nil
}

// RetryPreview returns information about what a retry would do.
func (s *taskServer) RetryPreview(
	ctx context.Context,
	req *connect.Request[orcv1.RetryPreviewRequest],
) (*connect.Response[orcv1.RetryPreviewResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Determine phases that would be rerun
	// Use current_phase as the starting point since phases no longer track running/failed status
	phasesToRerun := []string{}
	currentPhase := task.GetCurrentPhaseProto(t)
	if currentPhase != "" {
		phasesToRerun = append(phasesToRerun, currentPhase)
	}

	// If no failed phases, default to implement
	if len(phasesToRerun) == 0 {
		phasesToRerun = []string{"implement"}
	}

	info := &orcv1.RetryPreviewInfo{
		FromPhase:     phasesToRerun[0],
		PhasesToRerun: phasesToRerun,
	}

	if t.Execution != nil && t.Execution.Error != nil && *t.Execution.Error != "" {
		info.LastError = t.Execution.Error
	}

	return connect.NewResponse(&orcv1.RetryPreviewResponse{
		Info: info,
	}), nil
}
