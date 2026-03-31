package api

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// PerformAttentionAction handles actions on attention items.
func (s *attentionDashboardServer) PerformAttentionAction(
	ctx context.Context,
	req *connect.Request[orcv1.PerformAttentionActionRequest],
) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	projectID, attentionItemID, err := parseAttentionItemIdentifier(req.Msg.GetProjectId(), req.Msg.AttentionItemId)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}), nil
	}

	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	action := req.Msg.Action
	parts := strings.SplitN(attentionItemID, "-", 2)
	if len(parts) != 2 {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("invalid attention item ID format: %s", attentionItemID),
		}), nil
	}

	targetID := parts[1]

	switch action {
	case orcv1.AttentionAction_ATTENTION_ACTION_VIEW:
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
	case orcv1.AttentionAction_ATTENTION_ACTION_RETRY:
		return s.handleRetryAction(backend, projectID, targetID)
	case orcv1.AttentionAction_ATTENTION_ACTION_APPROVE:
		return s.handleApproveAction(backend, projectID, targetID, req.Msg.DecisionOptionId)
	case orcv1.AttentionAction_ATTENTION_ACTION_REJECT:
		return s.handleRejectAction(backend, projectID, targetID)
	case orcv1.AttentionAction_ATTENTION_ACTION_SKIP:
		return s.handleSkipAction(backend, projectID, targetID, req.Msg.Reason)
	case orcv1.AttentionAction_ATTENTION_ACTION_FORCE:
		return s.handleForceAction(backend, projectID, targetID, req.Msg.Reason)
	case orcv1.AttentionAction_ATTENTION_ACTION_RESOLVE:
		return s.handleResolveAction(backend, projectID, targetID, req.Msg.Comment)
	default:
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown action: %s", action.String()),
		}), nil
	}
}

// handleRetryAction handles retry actions on failed tasks.
func (s *attentionDashboardServer) handleRetryAction(backend storage.Backend, projectID string, taskID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be retried (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, projectID, originalTask, t, "dashboard_retry"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to update task attention state: %v", err),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}

// handleApproveAction handles approval of pending decisions.
func (s *attentionDashboardServer) handleApproveAction(
	backend storage.Backend,
	projectID string,
	decisionID string,
	selectedOptionID string,
) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	if s.pendingDecisions == nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: "pending decisions not available",
		}), nil
	}
	decision, ok := s.pendingDecisions.Get(projectID, decisionID)
	if !ok {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("decision not found: %s", decisionID),
		}), nil
	}
	if selectedOptionID != "" && !pendingDecisionHasOption(decision, selectedOptionID) {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("decision option not found: %s", selectedOptionID),
		}), nil
	}

	resolvedBy := "dashboard"
	_, err := resolvePendingDecision(
		backend,
		s.pendingDecisions,
		s.publisher,
		projectID,
		decisionID,
		true,
		"",
		resolvedBy,
		selectedOptionID,
	)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}

// handleRejectAction handles rejection of pending decisions.
func (s *attentionDashboardServer) handleRejectAction(backend storage.Backend, projectID string, decisionID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	if s.pendingDecisions == nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: "pending decisions not available",
		}), nil
	}
	if _, ok := s.pendingDecisions.Get(projectID, decisionID); !ok {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("decision not found: %s", decisionID),
		}), nil
	}

	resolvedBy := "dashboard"
	_, err := resolvePendingDecision(backend, s.pendingDecisions, s.publisher, projectID, decisionID, false, "", resolvedBy, "")
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}

// handleSkipAction handles skipping a blocked task (moves it back to planned).
func (s *attentionDashboardServer) handleSkipAction(backend storage.Backend, projectID, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be skipped (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	t.BlockedBy = nil
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, projectID, originalTask, t, "dashboard_skip"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to update task attention state: %v", err),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}

// handleForceAction handles forcing a blocked task to continue (sets to running).
func (s *attentionDashboardServer) handleForceAction(backend storage.Backend, projectID, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be forced (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, projectID, originalTask, t, "dashboard_force"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to update task attention state: %v", err),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}

// handleResolveAction handles resolving a failed task (sets to planned for retry).
func (s *attentionDashboardServer) handleResolveAction(backend storage.Backend, projectID, taskID, comment string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be resolved (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	originalTask := proto.Clone(t).(*orcv1.Task)
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, s.publisher, projectID, originalTask, t, "dashboard_resolve"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to update task attention state: %v", err),
		}), nil
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{Success: true}), nil
}
