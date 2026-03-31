package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// GetWorkflowDefaults returns the workflow defaults configuration.
func (s *configServer) GetWorkflowDefaults(
	ctx context.Context,
	req *connect.Request[orcv1.GetWorkflowDefaultsRequest],
) (*connect.Response[orcv1.GetWorkflowDefaultsResponse], error) {
	cfg, err := s.loadConfigForProject(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load config: %w", err))
	}

	return connect.NewResponse(&orcv1.GetWorkflowDefaultsResponse{
		WorkflowDefaults: &orcv1.WorkflowDefaults{
			Feature:  cfg.WorkflowDefaults.Feature,
			Bug:      cfg.WorkflowDefaults.Bug,
			Refactor: cfg.WorkflowDefaults.Refactor,
			Chore:    cfg.WorkflowDefaults.Chore,
			Docs:     cfg.WorkflowDefaults.Docs,
			Test:     cfg.WorkflowDefaults.Test,
			Default:  cfg.WorkflowDefaults.Default,
		},
	}), nil
}

// UpdateWorkflowDefaults updates the workflow defaults configuration.
func (s *configServer) UpdateWorkflowDefaults(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateWorkflowDefaultsRequest],
) (*connect.Response[orcv1.UpdateWorkflowDefaultsResponse], error) {
	if req.Msg.WorkflowDefaults == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_defaults is required"))
	}

	cfg, err := s.loadConfigForProject(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load config: %w", err))
	}

	protoDefaults := req.Msg.WorkflowDefaults
	if protoDefaults.Feature != "" {
		cfg.WorkflowDefaults.Feature = protoDefaults.Feature
	}
	if protoDefaults.Bug != "" {
		cfg.WorkflowDefaults.Bug = protoDefaults.Bug
	}
	if protoDefaults.Refactor != "" {
		cfg.WorkflowDefaults.Refactor = protoDefaults.Refactor
	}
	if protoDefaults.Chore != "" {
		cfg.WorkflowDefaults.Chore = protoDefaults.Chore
	}
	if protoDefaults.Docs != "" {
		cfg.WorkflowDefaults.Docs = protoDefaults.Docs
	}
	if protoDefaults.Test != "" {
		cfg.WorkflowDefaults.Test = protoDefaults.Test
	}
	if protoDefaults.Default != "" {
		cfg.WorkflowDefaults.Default = protoDefaults.Default
	}

	configPath := s.getConfigPath(req.Msg.ProjectId)
	if err := cfg.SaveTo(configPath); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save config: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateWorkflowDefaultsResponse{
		WorkflowDefaults: &orcv1.WorkflowDefaults{
			Feature:  cfg.WorkflowDefaults.Feature,
			Bug:      cfg.WorkflowDefaults.Bug,
			Refactor: cfg.WorkflowDefaults.Refactor,
			Chore:    cfg.WorkflowDefaults.Chore,
			Docs:     cfg.WorkflowDefaults.Docs,
			Test:     cfg.WorkflowDefaults.Test,
			Default:  cfg.WorkflowDefaults.Default,
		},
	}), nil
}
