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

// RunTask starts execution of a task.
// This validates the task can be run and spawns an executor via callback.
func (s *taskServer) RunTask(
	ctx context.Context,
	req *connect.Request[orcv1.RunTaskRequest],
) (*connect.Response[orcv1.RunTaskResponse], error) {
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

	// Validate workflow_id BEFORE any status changes
	workflowID := t.GetWorkflowId()
	if workflowID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task has no workflow_id set"))
	}

	// Validate task status allows running
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already running"))
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already completed"))
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is paused - use resume instead"))
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is blocked"))
	case orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_FAILED: // Failed tasks can be retried
		// OK to run
	default:
		// Allow other statuses (FINALIZING, etc.) to proceed for flexibility
	}

	// Check if task is blocked by dependencies
	allTasks, _ := backend.LoadAllTasks()
	if allTasks != nil {
		taskMap := make(map[string]*orcv1.Task)
		for _, at := range allTasks {
			taskMap[at.Id] = at
		}
		unmet := task.GetUnmetDependenciesProto(t, taskMap)
		if len(unmet) > 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task is blocked by: %v", unmet))
		}
	}

	// Store original status for rollback if executor fails
	originalStatus := t.Status
	originalTask := proto.Clone(t).(*orcv1.Task)

	// Set task to running
	task.MarkStartedProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), originalTask, t, "task_run"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Spawn executor if callback is set
	if s.taskExecutor != nil {
		if err := s.taskExecutor(t.Id, req.Msg.GetProjectId()); err != nil {
			// Executor failed to spawn - revert status
			revertedTask := proto.Clone(originalTask).(*orcv1.Task)
			revertedTask.Status = originalStatus
			task.UpdateTimestampProto(revertedTask)
			if saveErr := persistTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), revertedTask, "task_run_revert"); saveErr != nil {
				// Log but don't mask the original error
				if s.logger != nil {
					s.logger.Error("failed to revert task status after executor failure",
						"task", t.Id, "error", saveErr)
				}
			}
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("spawn executor: %w", err))
		}
	}

	return connect.NewResponse(&orcv1.RunTaskResponse{
		Task: t,
	}), nil
}

// PauseTask pauses a running task.
func (s *taskServer) PauseTask(
	ctx context.Context,
	req *connect.Request[orcv1.PauseTaskRequest],
) (*connect.Response[orcv1.PauseTaskResponse], error) {
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

	// Check if task is running
	if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is not running"))
	}

	// Set task to paused
	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), originalTask, t, "task_pause"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.PauseTaskResponse{
		Task: t,
	}), nil
}

// ResumeTask resumes a paused or failed task.
func (s *taskServer) ResumeTask(
	ctx context.Context,
	req *connect.Request[orcv1.ResumeTaskRequest],
) (*connect.Response[orcv1.ResumeTaskResponse], error) {
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

	// Check if task can be resumed
	if t.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be resumed"))
	}

	// Set task to running
	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), originalTask, t, "task_resume"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.ResumeTaskResponse{
		Task: t,
	}), nil
}

// SkipBlock removes the dependency blocking the task.
func (s *taskServer) SkipBlock(
	ctx context.Context,
	req *connect.Request[orcv1.SkipBlockRequest],
) (*connect.Response[orcv1.SkipBlockResponse], error) {
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

	// Clear blocked_by and reset status if blocked
	if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED || len(t.BlockedBy) > 0 {
		originalTask := proto.Clone(t).(*orcv1.Task)
		t.BlockedBy = nil
		if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
		}
		task.UpdateTimestampProto(t)

		if err := transitionTaskWithAttentionSync(backend, s.publisher, req.Msg.GetProjectId(), originalTask, t, "task_skip_block"); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&orcv1.SkipBlockResponse{
		Task: t,
	}), nil
}

// PauseAllTasks pauses all currently running tasks.
func (s *taskServer) PauseAllTasks(
	ctx context.Context,
	req *connect.Request[orcv1.PauseAllTasksRequest],
) (*connect.Response[orcv1.PauseAllTasksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	var pausedTasks []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
			t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
			if err := backend.SaveTask(t); err != nil {
				s.logger.Warn("failed to pause task", "task_id", t.Id, "error", err)
				continue
			}
			pausedTasks = append(pausedTasks, t)
			publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
		}
	}

	return connect.NewResponse(&orcv1.PauseAllTasksResponse{
		Tasks: pausedTasks,
		Count: int32(len(pausedTasks)),
	}), nil
}

// ResumeAllTasks resumes all paused tasks.
func (s *taskServer) ResumeAllTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ResumeAllTasksRequest],
) (*connect.Response[orcv1.ResumeAllTasksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	var resumedTasks []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_PAUSED {
			t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
			if err := backend.SaveTask(t); err != nil {
				s.logger.Warn("failed to resume task", "task_id", t.Id, "error", err)
				continue
			}
			resumedTasks = append(resumedTasks, t)
			publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
		}
	}

	return connect.NewResponse(&orcv1.ResumeAllTasksResponse{
		Tasks: resumedTasks,
		Count: int32(len(resumedTasks)),
	}), nil
}
