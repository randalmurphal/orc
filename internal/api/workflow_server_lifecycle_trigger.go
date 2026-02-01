package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// validLifecycleEvents lists the accepted event values for lifecycle triggers.
var validLifecycleEvents = map[string]bool{
	"on_task_created":       true,
	"on_task_completed":     true,
	"on_task_failed":        true,
	"on_initiative_planned": true,
}

// validLifecycleModes lists the accepted mode values for lifecycle triggers.
var validLifecycleModes = map[string]bool{
	"gate":     true,
	"reaction": true,
}

// AddLifecycleTrigger appends a new lifecycle trigger to a workflow.
func (s *workflowServer) AddLifecycleTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.AddLifecycleTriggerRequest],
) (*connect.Response[orcv1.AddLifecycleTriggerResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.Event == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event is required"))
	}
	if !validLifecycleEvents[req.Msg.Event] {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid event %q: must be one of on_task_created, on_task_completed, on_task_failed, on_initiative_planned", req.Msg.Event))
	}
	if req.Msg.AgentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_id is required"))
	}

	// Validate mode if provided
	mode := "gate" // default
	if req.Msg.Mode != nil {
		mode = *req.Msg.Mode
		if !validLifecycleModes[mode] {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid mode %q: must be \"gate\" or \"reaction\"", mode))
		}
	}

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Verify agent exists
	agent, err := s.globalDB.GetAgent(req.Msg.AgentId)
	if err != nil || agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.AgentId))
	}

	// Parse existing triggers
	triggers, err := db.ParseWorkflowTriggers(wf.Triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse workflow triggers: %w", err))
	}

	// Build new trigger
	trigger := db.WorkflowTrigger{
		Event:   req.Msg.Event,
		AgentID: req.Msg.AgentId,
		Mode:    mode,
		Enabled: req.Msg.Enabled,
	}

	// Parse input config if provided
	if req.Msg.InputConfig != nil {
		cfg, err := db.ParseGateInputConfig(*req.Msg.InputConfig)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid input_config: %w", err))
		}
		trigger.InputConfig = cfg
	}

	// Parse output config if provided
	if req.Msg.OutputConfig != nil {
		cfg, err := db.ParseGateOutputConfig(*req.Msg.OutputConfig)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid output_config: %w", err))
		}
		trigger.OutputConfig = cfg
	}

	// Append and save
	triggers = append(triggers, trigger)
	jsonStr, err := db.MarshalWorkflowTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal workflow triggers: %w", err))
	}
	wf.Triggers = jsonStr

	if err := s.globalDB.SaveWorkflow(wf); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.AddLifecycleTriggerResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}

// UpdateLifecycleTrigger modifies a lifecycle trigger at the specified index.
func (s *workflowServer) UpdateLifecycleTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateLifecycleTriggerRequest],
) (*connect.Response[orcv1.UpdateLifecycleTriggerResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	// Validate event if provided
	if req.Msg.Event != nil {
		if !validLifecycleEvents[*req.Msg.Event] {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid event %q: must be one of on_task_created, on_task_completed, on_task_failed, on_initiative_planned", *req.Msg.Event))
		}
	}

	// Validate mode if provided
	if req.Msg.Mode != nil {
		if !validLifecycleModes[*req.Msg.Mode] {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid mode %q: must be \"gate\" or \"reaction\"", *req.Msg.Mode))
		}
	}

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Parse existing triggers
	triggers, err := db.ParseWorkflowTriggers(wf.Triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse workflow triggers: %w", err))
	}

	// Validate index
	idx := int(req.Msg.TriggerIndex)
	if idx < 0 || idx >= len(triggers) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("trigger_index %d out of range (0..%d)", idx, len(triggers)-1))
	}

	// Validate agent if being changed
	if req.Msg.AgentId != nil {
		agent, err := s.globalDB.GetAgent(*req.Msg.AgentId)
		if err != nil || agent == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", *req.Msg.AgentId))
		}
		triggers[idx].AgentID = *req.Msg.AgentId
	}

	// Apply partial updates
	if req.Msg.Event != nil {
		triggers[idx].Event = *req.Msg.Event
	}
	if req.Msg.Mode != nil {
		triggers[idx].Mode = *req.Msg.Mode
	}
	if req.Msg.Enabled != nil {
		triggers[idx].Enabled = *req.Msg.Enabled
	}
	if req.Msg.InputConfig != nil {
		cfg, err := db.ParseGateInputConfig(*req.Msg.InputConfig)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid input_config: %w", err))
		}
		triggers[idx].InputConfig = cfg
	}
	if req.Msg.OutputConfig != nil {
		cfg, err := db.ParseGateOutputConfig(*req.Msg.OutputConfig)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid output_config: %w", err))
		}
		triggers[idx].OutputConfig = cfg
	}

	// Save
	jsonStr, err := db.MarshalWorkflowTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal workflow triggers: %w", err))
	}
	wf.Triggers = jsonStr

	if err := s.globalDB.SaveWorkflow(wf); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateLifecycleTriggerResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}

// RemoveLifecycleTrigger removes a lifecycle trigger at the specified index.
func (s *workflowServer) RemoveLifecycleTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveLifecycleTriggerRequest],
) (*connect.Response[orcv1.RemoveLifecycleTriggerResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Parse existing triggers
	triggers, err := db.ParseWorkflowTriggers(wf.Triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse workflow triggers: %w", err))
	}

	// Validate index
	idx := int(req.Msg.TriggerIndex)
	if idx < 0 || idx >= len(triggers) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("trigger_index %d out of range (0..%d)", idx, len(triggers)-1))
	}

	// Remove at index
	triggers = append(triggers[:idx], triggers[idx+1:]...)

	// Save
	jsonStr, err := db.MarshalWorkflowTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal workflow triggers: %w", err))
	}
	wf.Triggers = jsonStr

	if err := s.globalDB.SaveWorkflow(wf); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.RemoveLifecycleTriggerResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}
