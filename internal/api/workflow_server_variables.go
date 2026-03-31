package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// AddVariable adds a variable to a workflow.
func (s *workflowServer) AddVariable(
	ctx context.Context,
	req *connect.Request[orcv1.AddVariableRequest],
) (*connect.Response[orcv1.AddVariableResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	variable := &db.WorkflowVariable{
		WorkflowID:      req.Msg.WorkflowId,
		Name:            req.Msg.Name,
		SourceType:      protoVariableSourceTypeToString(req.Msg.SourceType),
		SourceConfig:    req.Msg.SourceConfig,
		Required:        req.Msg.Required,
		CacheTTLSeconds: int(req.Msg.CacheTtlSeconds),
	}
	if req.Msg.Description != nil {
		variable.Description = *req.Msg.Description
	}
	if req.Msg.DefaultValue != nil {
		variable.DefaultValue = *req.Msg.DefaultValue
	}
	if req.Msg.Extract != nil {
		variable.Extract = *req.Msg.Extract
	}

	if err := s.globalDB.SaveWorkflowVariable(variable); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save variable: %w", err))
	}

	return connect.NewResponse(&orcv1.AddVariableResponse{
		Variable: dbWorkflowVariableToProto(variable),
	}), nil
}

// UpdateVariable updates an existing variable in a workflow.
func (s *workflowServer) UpdateVariable(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateVariableRequest],
) (*connect.Response[orcv1.UpdateVariableResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	variables, err := s.globalDB.GetWorkflowVariables(req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow variables: %w", err))
	}

	var existingVar *db.WorkflowVariable
	for _, v := range variables {
		if v.Name == req.Msg.Name {
			existingVar = v
			break
		}
	}
	if existingVar == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("variable %s not found in workflow", req.Msg.Name))
	}

	existingVar.SourceType = protoVariableSourceTypeToString(req.Msg.SourceType)
	existingVar.SourceConfig = req.Msg.SourceConfig
	existingVar.Required = req.Msg.Required
	existingVar.CacheTTLSeconds = int(req.Msg.CacheTtlSeconds)

	if req.Msg.Description != nil {
		existingVar.Description = *req.Msg.Description
	}
	if req.Msg.DefaultValue != nil {
		existingVar.DefaultValue = *req.Msg.DefaultValue
	}
	if req.Msg.Extract != nil {
		existingVar.Extract = *req.Msg.Extract
	}

	if err := s.globalDB.SaveWorkflowVariable(existingVar); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save variable: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateVariableResponse{
		Variable: dbWorkflowVariableToProto(existingVar),
	}), nil
}

// RemoveVariable removes a variable from a workflow.
func (s *workflowServer) RemoveVariable(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveVariableRequest],
) (*connect.Response[orcv1.RemoveVariableResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	if err := s.globalDB.DeleteWorkflowVariable(req.Msg.WorkflowId, req.Msg.Name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete variable: %w", err))
	}

	return connect.NewResponse(&orcv1.RemoveVariableResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}
