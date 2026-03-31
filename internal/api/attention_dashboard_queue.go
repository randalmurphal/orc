package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// UpdateQueueOrganization handles queue organization updates.
func (s *attentionDashboardServer) UpdateQueueOrganization(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateQueueOrganizationRequest],
) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	switch update := req.Msg.Update.(type) {
	case *orcv1.UpdateQueueOrganizationRequest_SwimlaneState:
		return s.handleSwimlaneStateUpdate(backend, update.SwimlaneState)
	case *orcv1.UpdateQueueOrganizationRequest_TaskReorder:
		return s.handleTaskReorderUpdate(backend, req.Msg.GetProjectId(), update.TaskReorder)
	default:
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: "unknown update type",
		}), nil
	}
}

// handleSwimlaneStateUpdate handles updating swimlane collapsed/expanded state.
func (s *attentionDashboardServer) handleSwimlaneStateUpdate(backend storage.Backend, swimlaneState *orcv1.SwimlaneStateUpdate) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	if s.logger != nil {
		s.logger.Info("Swimlane state updated",
			"initiative_id", swimlaneState.InitiativeId,
			"collapsed", swimlaneState.Collapsed,
		)
	}

	return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
		Success: true,
	}), nil
}

// handleTaskReorderUpdate handles reordering tasks within or between initiatives.
func (s *attentionDashboardServer) handleTaskReorderUpdate(
	backend storage.Backend,
	projectID string,
	taskReorder *orcv1.TaskReorderUpdate,
) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	t, err := backend.LoadTask(taskReorder.TaskId)
	if err != nil {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskReorder.TaskId),
		}), nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be reordered (status: %s)", taskReorder.TaskId, t.Status.String()),
		}), nil
	}

	targetInitiativeID := taskReorder.TargetInitiativeId
	if targetInitiativeID == "" {
		t.InitiativeId = nil
	} else {
		if _, err := backend.LoadInitiativeProto(targetInitiativeID); err != nil {
			return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("target initiative %s not found", targetInitiativeID),
			}), nil
		}
		t.InitiativeId = &targetInitiativeID
	}

	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	if s.publisher != nil {
		publishTaskUpdatedEvent(s.publisher, projectID, t)
	}

	if s.logger != nil {
		s.logger.Info("Task reordered",
			"task_id", taskReorder.TaskId,
			"target_initiative", targetInitiativeID,
			"position", taskReorder.NewPosition,
		)
	}

	return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
		Success: true,
	}), nil
}
