package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ListWorkflowRuns returns workflow runs.
func (s *workflowServer) ListWorkflowRuns(
	ctx context.Context,
	req *connect.Request[orcv1.ListWorkflowRunsRequest],
) (*connect.Response[orcv1.ListWorkflowRunsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	opts := db.WorkflowRunListOpts{}
	if req.Msg.WorkflowId != nil {
		opts.WorkflowID = *req.Msg.WorkflowId
	}
	if req.Msg.TaskId != nil {
		opts.TaskID = *req.Msg.TaskId
	}

	runs, err := backend.ListWorkflowRuns(opts)
	if err != nil {
		return connect.NewResponse(&orcv1.ListWorkflowRunsResponse{
			Runs: []*orcv1.WorkflowRun{},
		}), nil
	}

	protoRuns := make([]*orcv1.WorkflowRun, len(runs))
	for i, r := range runs {
		protoRuns[i] = dbWorkflowRunToProto(r)
	}

	return connect.NewResponse(&orcv1.ListWorkflowRunsResponse{
		Runs: protoRuns,
	}), nil
}

// GetWorkflowRun returns a single workflow run.
func (s *workflowServer) GetWorkflowRun(
	ctx context.Context,
	req *connect.Request[orcv1.GetWorkflowRunRequest],
) (*connect.Response[orcv1.GetWorkflowRunResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	run, err := backend.GetWorkflowRun(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow run %s not found", req.Msg.Id))
	}

	wf, _ := s.globalDB.GetWorkflow(run.WorkflowID)
	phases, _ := backend.GetWorkflowRunPhases(run.ID)

	return connect.NewResponse(&orcv1.GetWorkflowRunResponse{
		Run: &orcv1.WorkflowRunWithDetails{
			Run:      dbWorkflowRunToProto(run),
			Workflow: dbWorkflowToProto(wf),
			Phases:   dbWorkflowRunPhasesToProto(phases),
		},
	}), nil
}

// StartWorkflowRun starts a new workflow run.
func (s *workflowServer) StartWorkflowRun(
	ctx context.Context,
	req *connect.Request[orcv1.StartWorkflowRunRequest],
) (*connect.Response[orcv1.StartWorkflowRunResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}
	if req.Msg.Prompt == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prompt is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}

	run := &db.WorkflowRun{
		WorkflowID:  req.Msg.WorkflowId,
		ContextType: protoContextTypeToString(req.Msg.ContextType),
		Prompt:      req.Msg.Prompt,
		Status:      "pending",
	}
	if req.Msg.Instructions != nil {
		run.Instructions = *req.Msg.Instructions
	}
	if req.Msg.ContextData != nil && req.Msg.ContextData.TaskId != nil {
		run.TaskID = req.Msg.ContextData.TaskId
	}

	if err := backend.SaveWorkflowRun(run); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow run: %w", err))
	}

	return connect.NewResponse(&orcv1.StartWorkflowRunResponse{
		Run: dbWorkflowRunToProto(run),
	}), nil
}

// CancelWorkflowRun cancels a running workflow.
func (s *workflowServer) CancelWorkflowRun(
	ctx context.Context,
	req *connect.Request[orcv1.CancelWorkflowRunRequest],
) (*connect.Response[orcv1.CancelWorkflowRunResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	run, err := backend.GetWorkflowRun(req.Msg.Id)
	if err != nil || run == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow run %s not found", req.Msg.Id))
	}

	if run.Status != "running" && run.Status != "pending" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("cannot cancel run with status: %s", run.Status))
	}

	run.Status = "cancelled"
	run.Error = "cancelled via API"

	if err := backend.SaveWorkflowRun(run); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow run: %w", err))
	}

	return connect.NewResponse(&orcv1.CancelWorkflowRunResponse{
		Run: dbWorkflowRunToProto(run),
	}), nil
}
