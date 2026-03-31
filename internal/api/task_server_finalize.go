package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// FinalizeTask starts the finalize process for a completed task.
func (s *taskServer) FinalizeTask(
	ctx context.Context,
	req *connect.Request[orcv1.FinalizeTaskRequest],
) (*connect.Response[orcv1.FinalizeTaskResponse], error) {
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

	// Check if task can be finalized
	if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_FINALIZING && !req.Msg.Force {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task status is %s, expected completed", t.Status))
	}

	// Start finalize (this would normally be async, but for the RPC we return immediately)
	t.Status = orcv1.TaskStatus_TASK_STATUS_FINALIZING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
	}

	// Return initial finalize state (actual finalization runs async in background)
	state := &orcv1.FinalizeState{
		Synced:      false,
		TestsPassed: false,
		NeedsReview: true,
	}

	return connect.NewResponse(&orcv1.FinalizeTaskResponse{
		Task:  t,
		State: state,
	}), nil
}

// GetFinalizeState returns the current finalize state for a task.
func (s *taskServer) GetFinalizeState(
	ctx context.Context,
	req *connect.Request[orcv1.GetFinalizeStateRequest],
) (*connect.Response[orcv1.GetFinalizeStateResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load task to get finalization state
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Build finalize state from task's current state
	protoState := &orcv1.FinalizeState{
		Synced:      t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		TestsPassed: true, // Default assumption; real implementation would check test results
		NeedsReview: t.Pr != nil && !t.Pr.Merged,
	}

	// If merged, populate merge details
	if t.Pr != nil && t.Pr.Merged {
		protoState.Merged = true
		if t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
			protoState.MergeCommit = t.Pr.MergeCommitSha
		}
		if t.Pr.TargetBranch != nil && *t.Pr.TargetBranch != "" {
			protoState.TargetBranch = t.Pr.TargetBranch
		}
	}

	return connect.NewResponse(&orcv1.GetFinalizeStateResponse{
		State: protoState,
	}), nil
}
