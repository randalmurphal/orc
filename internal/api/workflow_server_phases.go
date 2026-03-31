package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// AddPhase adds a phase to a workflow.
func (s *workflowServer) AddPhase(
	ctx context.Context,
	req *connect.Request[orcv1.AddPhaseRequest],
) (*connect.Response[orcv1.AddPhaseResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.PhaseTemplateId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phase_template_id is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.PhaseTemplateId)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.PhaseTemplateId))
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      req.Msg.WorkflowId,
		PhaseTemplateID: req.Msg.PhaseTemplateId,
		Sequence:        int(req.Msg.Sequence),
		DependsOn:       dependsOnToJSON(req.Msg.DependsOn),
	}
	if req.Msg.ModelOverride != nil {
		phase.ModelOverride = *req.Msg.ModelOverride
	}
	if req.Msg.ThinkingOverride != nil {
		phase.ThinkingOverride = req.Msg.ThinkingOverride
	}
	if req.Msg.GateTypeOverride != nil {
		phase.GateTypeOverride = protoGateTypeToString(*req.Msg.GateTypeOverride)
	}
	if req.Msg.Condition != nil {
		phase.Condition = *req.Msg.Condition
	}
	if req.Msg.AgentOverride != nil {
		phase.AgentOverride = *req.Msg.AgentOverride
	}
	if len(req.Msg.SubAgentsOverride) > 0 {
		phase.SubAgentsOverride = dependsOnToJSON(req.Msg.SubAgentsOverride)
	}
	if req.Msg.ProviderOverride != nil {
		if err := validateProviderString(*req.Msg.ProviderOverride); err != nil {
			return nil, err
		}
		phase.ProviderOverride = *req.Msg.ProviderOverride
	}

	if err := s.globalDB.SaveWorkflowPhase(phase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save phase: %w", err))
	}

	return connect.NewResponse(&orcv1.AddPhaseResponse{
		Phase: dbWorkflowPhaseToProto(phase),
	}), nil
}

// UpdatePhase updates a phase in a workflow.
func (s *workflowServer) UpdatePhase(
	ctx context.Context,
	req *connect.Request[orcv1.UpdatePhaseRequest],
) (*connect.Response[orcv1.UpdatePhaseResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	phases, err := s.globalDB.GetWorkflowPhases(req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow phases: %w", err))
	}

	var existingPhase *db.WorkflowPhase
	for _, p := range phases {
		if p.ID == int(req.Msg.PhaseId) {
			existingPhase = p
			break
		}
	}
	if existingPhase == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase %d not found in workflow", req.Msg.PhaseId))
	}

	if req.Msg.Sequence != nil {
		existingPhase.Sequence = int(*req.Msg.Sequence)
	}
	if len(req.Msg.DependsOn) > 0 {
		existingPhase.DependsOn = dependsOnToJSON(req.Msg.DependsOn)
	}
	if req.Msg.ModelOverride != nil {
		existingPhase.ModelOverride = *req.Msg.ModelOverride
	}
	if req.Msg.ThinkingOverride != nil {
		existingPhase.ThinkingOverride = req.Msg.ThinkingOverride
	}
	if req.Msg.GateTypeOverride != nil {
		existingPhase.GateTypeOverride = protoGateTypeToString(*req.Msg.GateTypeOverride)
	}
	if req.Msg.Condition != nil {
		existingPhase.Condition = *req.Msg.Condition
	}
	if req.Msg.AgentOverride != nil {
		existingPhase.AgentOverride = *req.Msg.AgentOverride
	}
	if req.Msg.SubAgentsOverrideSet != nil && *req.Msg.SubAgentsOverrideSet {
		existingPhase.SubAgentsOverride = dependsOnToJSON(req.Msg.SubAgentsOverride)
	}
	if req.Msg.RuntimeConfigOverride != nil {
		existingPhase.RuntimeConfigOverride = *req.Msg.RuntimeConfigOverride
	}
	if req.Msg.LoopConfig != nil {
		existingPhase.LoopConfig = *req.Msg.LoopConfig
	}
	if req.Msg.ProviderOverride != nil {
		if err := validateProviderString(*req.Msg.ProviderOverride); err != nil {
			return nil, err
		}
		existingPhase.ProviderOverride = *req.Msg.ProviderOverride
	}

	if err := s.globalDB.SaveWorkflowPhase(existingPhase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save phase: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdatePhaseResponse{
		Phase: dbWorkflowPhaseToProto(existingPhase),
	}), nil
}

// RemovePhase removes a phase from a workflow.
func (s *workflowServer) RemovePhase(
	ctx context.Context,
	req *connect.Request[orcv1.RemovePhaseRequest],
) (*connect.Response[orcv1.RemovePhaseResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	phases, err := s.globalDB.GetWorkflowPhases(req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow phases: %w", err))
	}

	var phaseTemplateID string
	for _, p := range phases {
		if p.ID == int(req.Msg.PhaseId) {
			phaseTemplateID = p.PhaseTemplateID
			break
		}
	}
	if phaseTemplateID == "" {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase %d not found in workflow", req.Msg.PhaseId))
	}

	if err := s.globalDB.DeleteWorkflowPhase(req.Msg.WorkflowId, phaseTemplateID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete phase: %w", err))
	}

	return connect.NewResponse(&orcv1.RemovePhaseResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}
