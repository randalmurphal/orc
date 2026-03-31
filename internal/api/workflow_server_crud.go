package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

// ListWorkflows returns all workflows.
func (s *workflowServer) ListWorkflows(
	ctx context.Context,
	req *connect.Request[orcv1.ListWorkflowsRequest],
) (*connect.Response[orcv1.ListWorkflowsResponse], error) {
	workflows, err := s.globalDB.ListWorkflows()
	if err != nil {
		return connect.NewResponse(&orcv1.ListWorkflowsResponse{
			Workflows: []*orcv1.Workflow{},
		}), nil
	}

	protoWorkflows := make([]*orcv1.Workflow, len(workflows))
	phaseCounts := make(map[string]int32, len(workflows))
	sources := make(map[string]orcv1.DefinitionSource, len(workflows))

	var resolvedWorkflows []workflow.ResolvedWorkflow
	if s.resolver != nil {
		resolvedWorkflows, _ = s.resolver.ListWorkflows()
	}
	sourceMap := make(map[string]workflow.Source)
	for _, rw := range resolvedWorkflows {
		sourceMap[rw.Workflow.ID] = rw.Source
	}

	for i, w := range workflows {
		protoWorkflows[i] = dbWorkflowToProto(w)
		phases, _ := s.globalDB.GetWorkflowPhases(w.ID)
		phaseCounts[w.ID] = int32(len(phases))

		if src, ok := sourceMap[w.ID]; ok {
			sources[w.ID] = workflowSourceToProto(src)
		} else if w.IsBuiltin {
			sources[w.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_EMBEDDED
		} else {
			sources[w.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_PROJECT
		}
	}

	return connect.NewResponse(&orcv1.ListWorkflowsResponse{
		Workflows:   protoWorkflows,
		PhaseCounts: phaseCounts,
		Sources:     sources,
	}), nil
}

// GetWorkflow returns a single workflow by ID.
func (s *workflowServer) GetWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.GetWorkflowRequest],
) (*connect.Response[orcv1.GetWorkflowResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	w, err := s.globalDB.GetWorkflow(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.Id))
	}

	phases, _ := s.globalDB.GetWorkflowPhases(w.ID)
	variables, _ := s.globalDB.GetWorkflowVariables(w.ID)
	protoPhases := dbWorkflowPhasesToProto(phases)

	for i, phase := range phases {
		tmpl, err := s.globalDB.GetPhaseTemplate(phase.PhaseTemplateID)
		if err == nil && tmpl != nil {
			protoPhases[i].Template = dbPhaseTemplateToProto(tmpl)
		}
	}

	return connect.NewResponse(&orcv1.GetWorkflowResponse{
		Workflow: &orcv1.WorkflowWithDetails{
			Workflow:  dbWorkflowToProto(w),
			Phases:    protoPhases,
			Variables: dbWorkflowVariablesToProto(variables),
		},
	}), nil
}

// CreateWorkflow creates a new workflow.
func (s *workflowServer) CreateWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.CreateWorkflowRequest],
) (*connect.Response[orcv1.CreateWorkflowResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	w := &db.Workflow{
		ID:   req.Msg.Id,
		Name: req.Msg.Name,
	}
	if req.Msg.Description != nil {
		w.Description = *req.Msg.Description
	}
	if req.Msg.DefaultModel != nil {
		w.DefaultModel = *req.Msg.DefaultModel
	}
	if req.Msg.CompletionAction != nil {
		w.CompletionAction = *req.Msg.CompletionAction
	}
	if req.Msg.TargetBranch != nil {
		w.TargetBranch = *req.Msg.TargetBranch
	}
	if req.Msg.DefaultProvider != nil {
		if err := validateProviderString(*req.Msg.DefaultProvider); err != nil {
			return nil, err
		}
		w.DefaultProvider = *req.Msg.DefaultProvider
	}

	if err := s.globalDB.SaveWorkflow(w); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateWorkflowResponse{
		Workflow: dbWorkflowToProto(w),
	}), nil
}

// UpdateWorkflow updates an existing workflow.
func (s *workflowServer) UpdateWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateWorkflowRequest],
) (*connect.Response[orcv1.UpdateWorkflowResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	dbWf, err := s.globalDB.GetWorkflow(req.Msg.Id)
	if err == nil && dbWf != nil {
		if dbWf.IsBuiltin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
		}

		if req.Msg.Name != nil {
			dbWf.Name = *req.Msg.Name
		}
		if req.Msg.Description != nil {
			dbWf.Description = *req.Msg.Description
		}
		if req.Msg.DefaultModel != nil {
			dbWf.DefaultModel = *req.Msg.DefaultModel
		}
		if req.Msg.DefaultThinking != nil {
			dbWf.DefaultThinking = *req.Msg.DefaultThinking
		}
		if req.Msg.CompletionAction != nil {
			dbWf.CompletionAction = *req.Msg.CompletionAction
		}
		if req.Msg.TargetBranch != nil {
			dbWf.TargetBranch = *req.Msg.TargetBranch
		}
		if req.Msg.DefaultProvider != nil {
			if err := validateProviderString(*req.Msg.DefaultProvider); err != nil {
				return nil, err
			}
			dbWf.DefaultProvider = *req.Msg.DefaultProvider
		}

		if err := s.globalDB.SaveWorkflow(dbWf); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
		}

		return connect.NewResponse(&orcv1.UpdateWorkflowResponse{
			Workflow: dbWorkflowToProto(dbWf),
		}), nil
	}

	resolved, err := s.resolver.ResolveWorkflow(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.Id))
	}
	if resolved.Source == workflow.SourceEmbedded {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	wf := resolved.Workflow
	if req.Msg.Name != nil {
		wf.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		wf.Description = *req.Msg.Description
	}
	if req.Msg.DefaultModel != nil {
		wf.DefaultModel = *req.Msg.DefaultModel
	}
	if req.Msg.DefaultThinking != nil {
		wf.DefaultThinking = *req.Msg.DefaultThinking
	}
	if req.Msg.CompletionAction != nil {
		wf.CompletionAction = *req.Msg.CompletionAction
	}
	if req.Msg.TargetBranch != nil {
		wf.TargetBranch = *req.Msg.TargetBranch
	}
	if req.Msg.DefaultProvider != nil {
		if err := validateProviderString(*req.Msg.DefaultProvider); err != nil {
			return nil, err
		}
		wf.DefaultProvider = *req.Msg.DefaultProvider
	}

	writeLevel := workflow.SourceToWriteLevel(resolved.Source)
	if writeLevel != "" {
		writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
		if _, writeErr := writer.WriteWorkflow(wf, writeLevel); writeErr != nil {
			s.logger.Warn("failed to write workflow file", "id", req.Msg.Id, "error", writeErr)
		}
	}

	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after update", "error", err)
	}

	w, err := s.globalDB.GetWorkflow(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get updated workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateWorkflowResponse{
		Workflow: dbWorkflowToProto(w),
	}), nil
}

// DeleteWorkflow deletes a workflow.
func (s *workflowServer) DeleteWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteWorkflowRequest],
) (*connect.Response[orcv1.DeleteWorkflowResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	if err := s.globalDB.DeleteWorkflow(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete workflow: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteWorkflowResponse{}), nil
}

// CloneWorkflow creates a copy of an existing workflow.
func (s *workflowServer) CloneWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.CloneWorkflowRequest],
) (*connect.Response[orcv1.CloneWorkflowResponse], error) {
	if req.Msg.SourceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id is required"))
	}
	if req.Msg.NewId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_id is required"))
	}

	result, err := s.cloner.CloneWorkflow(req.Msg.SourceId, req.Msg.NewId, workflow.WriteLevelProject, false)
	if err != nil {
		if errors.Is(err, workflow.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("source workflow %s not found", req.Msg.SourceId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("clone workflow: %w", err))
	}

	if req.Msg.NewName != nil && *req.Msg.NewName != "" {
		resolved, err := s.resolver.ResolveWorkflow(req.Msg.NewId)
		if err == nil && resolved != nil {
			resolved.Workflow.Name = *req.Msg.NewName
			writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
			if _, writeErr := writer.WriteWorkflow(resolved.Workflow, workflow.WriteLevelProject); writeErr != nil {
				s.logger.Warn("failed to update cloned workflow name", "error", writeErr)
			}
		}
	}

	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after clone", "error", err)
	}

	clone, err := s.globalDB.GetWorkflow(req.Msg.NewId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get cloned workflow: %w", err))
	}

	s.logger.Info("cloned workflow",
		"source", req.Msg.SourceId,
		"dest", req.Msg.NewId,
		"path", result.DestPath,
	)

	return connect.NewResponse(&orcv1.CloneWorkflowResponse{
		Workflow: dbWorkflowToProto(clone),
	}), nil
}
