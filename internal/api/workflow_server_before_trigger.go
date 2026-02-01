package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// validBeforeTriggerModes lists the accepted mode values for before-phase triggers.
var validBeforeTriggerModes = map[string]bool{
	"gate":     true,
	"reaction": true,
}

// AddBeforePhaseTrigger appends a new before-phase trigger to a workflow phase.
func (s *workflowServer) AddBeforePhaseTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.AddBeforePhaseTriggerRequest],
) (*connect.Response[orcv1.AddBeforePhaseTriggerResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.AgentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_id is required"))
	}

	// Validate mode if provided
	mode := "gate" // default
	if req.Msg.Mode != nil {
		mode = *req.Msg.Mode
		if !validBeforeTriggerModes[mode] {
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

	// Find phase by ID
	phase, err := s.findWorkflowPhase(req.Msg.WorkflowId, req.Msg.PhaseId)
	if err != nil {
		return nil, err
	}

	// Verify agent exists
	agent, err := s.globalDB.GetAgent(req.Msg.AgentId)
	if err != nil || agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.AgentId))
	}

	// Parse existing triggers
	triggers, err := db.ParseBeforeTriggers(phase.BeforeTriggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse before triggers: %w", err))
	}

	// Build new trigger
	trigger := db.BeforePhaseTrigger{
		AgentID: req.Msg.AgentId,
		Mode:    mode,
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
	jsonStr, err := db.MarshalBeforeTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal before triggers: %w", err))
	}
	phase.BeforeTriggers = jsonStr

	if err := s.globalDB.SaveWorkflowPhase(phase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow phase: %w", err))
	}

	return connect.NewResponse(&orcv1.AddBeforePhaseTriggerResponse{
		Phase: dbWorkflowPhaseToProto(phase),
	}), nil
}

// UpdateBeforePhaseTrigger modifies a before-phase trigger at the specified index.
func (s *workflowServer) UpdateBeforePhaseTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateBeforePhaseTriggerRequest],
) (*connect.Response[orcv1.UpdateBeforePhaseTriggerResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	// Validate mode if provided
	if req.Msg.Mode != nil {
		if !validBeforeTriggerModes[*req.Msg.Mode] {
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

	// Find phase by ID
	phase, err := s.findWorkflowPhase(req.Msg.WorkflowId, req.Msg.PhaseId)
	if err != nil {
		return nil, err
	}

	// Parse existing triggers
	triggers, err := db.ParseBeforeTriggers(phase.BeforeTriggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse before triggers: %w", err))
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
	if req.Msg.Mode != nil {
		triggers[idx].Mode = *req.Msg.Mode
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
	jsonStr, err := db.MarshalBeforeTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal before triggers: %w", err))
	}
	phase.BeforeTriggers = jsonStr

	if err := s.globalDB.SaveWorkflowPhase(phase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow phase: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateBeforePhaseTriggerResponse{
		Phase: dbWorkflowPhaseToProto(phase),
	}), nil
}

// RemoveBeforePhaseTrigger removes a before-phase trigger at the specified index.
func (s *workflowServer) RemoveBeforePhaseTrigger(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveBeforePhaseTriggerRequest],
) (*connect.Response[orcv1.RemoveBeforePhaseTriggerResponse], error) {
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

	// Find phase by ID
	phase, err := s.findWorkflowPhase(req.Msg.WorkflowId, req.Msg.PhaseId)
	if err != nil {
		return nil, err
	}

	// Parse existing triggers
	triggers, err := db.ParseBeforeTriggers(phase.BeforeTriggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("parse before triggers: %w", err))
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
	jsonStr, err := db.MarshalBeforeTriggers(triggers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal before triggers: %w", err))
	}
	phase.BeforeTriggers = jsonStr

	if err := s.globalDB.SaveWorkflowPhase(phase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow phase: %w", err))
	}

	return connect.NewResponse(&orcv1.RemoveBeforePhaseTriggerResponse{
		Phase: dbWorkflowPhaseToProto(phase),
	}), nil
}

// findWorkflowPhase looks up a workflow phase by workflow ID and phase DB ID.
// Returns a connect error if not found.
func (s *workflowServer) findWorkflowPhase(workflowID string, phaseID int32) (*db.WorkflowPhase, error) {
	phases, err := s.globalDB.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow phases: %w", err))
	}

	for _, p := range phases {
		if p.ID == int(phaseID) {
			return p, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase %d not found in workflow %s", phaseID, workflowID))
}

