// Package api provides the Connect RPC and REST API server for orc.
// This file implements the WorkflowService Connect RPC service.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"
)

// workflowServer implements the WorkflowServiceHandler interface.
type workflowServer struct {
	orcv1connect.UnimplementedWorkflowServiceHandler
	backend      storage.Backend  // Legacy: single project backend (fallback)
	projectCache *ProjectCache    // Multi-project: cache of backends per project
	globalDB     *db.GlobalDB     // Global DB (workflows, phases, agents)
	resolver     *workflow.Resolver
	cloner       *workflow.Cloner
	cache        *workflow.CacheService
	logger       *slog.Logger
}

// NewWorkflowServer creates a new WorkflowService handler.
func NewWorkflowServer(
	backend storage.Backend,
	globalDB *db.GlobalDB,
	resolver *workflow.Resolver,
	cloner *workflow.Cloner,
	cache *workflow.CacheService,
	logger *slog.Logger,
) orcv1connect.WorkflowServiceHandler {
	return &workflowServer{
		backend:  backend,
		globalDB: globalDB,
		resolver: resolver,
		cloner:   cloner,
		cache:    cache,
		logger:   logger,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *workflowServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *workflowServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

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

	// Convert to proto and collect phase counts and sources
	protoWorkflows := make([]*orcv1.Workflow, len(workflows))
	phaseCounts := make(map[string]int32, len(workflows))
	sources := make(map[string]orcv1.DefinitionSource, len(workflows))

	// Build source map from resolver if available
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

		// Get source from resolver map or fall back to builtin check
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

	// Get phases and variables
	phases, _ := s.globalDB.GetWorkflowPhases(w.ID)
	variables, _ := s.globalDB.GetWorkflowVariables(w.ID)

	// Convert phases to proto
	protoPhases := dbWorkflowPhasesToProto(phases)

	// Fetch and attach phase templates for each phase
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
		ID:           req.Msg.Id,
		Name:         req.Msg.Name,
		WorkflowType: protoWorkflowTypeToString(req.Msg.WorkflowType),
	}
	if req.Msg.Description != nil {
		w.Description = *req.Msg.Description
	}
	if req.Msg.DefaultModel != nil {
		w.DefaultModel = *req.Msg.DefaultModel
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

	// First check the database for the workflow (handles API/CLI-created workflows)
	dbWf, err := s.globalDB.GetWorkflow(req.Msg.Id)
	if err == nil && dbWf != nil {
		// Found in database - update directly
		if dbWf.IsBuiltin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
		}

		// Apply updates
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

		// Save to database
		if err := s.globalDB.SaveWorkflow(dbWf); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow: %w", err))
		}

		return connect.NewResponse(&orcv1.UpdateWorkflowResponse{
			Workflow: dbWorkflowToProto(dbWf),
		}), nil
	}

	// Not found in database - try file-based resolver for YAML workflows
	resolved, err := s.resolver.ResolveWorkflow(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.Id))
	}

	// Cannot modify embedded/built-in workflows
	if resolved.Source == workflow.SourceEmbedded {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Apply updates to the workflow
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

	// Write back to file
	writeLevel := workflow.SourceToWriteLevel(resolved.Source)
	if writeLevel != "" {
		writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
		if _, writeErr := writer.WriteWorkflow(wf, writeLevel); writeErr != nil {
			s.logger.Warn("failed to write workflow file", "id", req.Msg.Id, "error", writeErr)
		}
	}

	// Sync cache to update DB
	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after update", "error", err)
	}

	// Get updated workflow from DB for response
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

// ListWorkflowRuns returns workflow runs.
func (s *workflowServer) ListWorkflowRuns(
	ctx context.Context,
	req *connect.Request[orcv1.ListWorkflowRunsRequest],
) (*connect.Response[orcv1.ListWorkflowRunsResponse], error) {
	backend, err := s.getBackend("")
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

	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	run, err := backend.GetWorkflowRun(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow run %s not found", req.Msg.Id))
	}

	// Get workflow
	workflow, _ := s.globalDB.GetWorkflow(run.WorkflowID)
	// Get phases
	phases, _ := backend.GetWorkflowRunPhases(run.ID)

	return connect.NewResponse(&orcv1.GetWorkflowRunResponse{
		Run: &orcv1.WorkflowRunWithDetails{
			Run:      dbWorkflowRunToProto(run),
			Workflow: dbWorkflowToProto(workflow),
			Phases:   dbWorkflowRunPhasesToProto(phases),
		},
	}), nil
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

	// Use file-based cloner to create YAML file at project level
	result, err := s.cloner.CloneWorkflow(req.Msg.SourceId, req.Msg.NewId, workflow.WriteLevelProject, false)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, workflow.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("source workflow %s not found", req.Msg.SourceId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("clone workflow: %w", err))
	}

	// Update name if provided
	if req.Msg.NewName != nil && *req.Msg.NewName != "" {
		// Re-read the cloned workflow, update name, and re-write
		resolved, err := s.resolver.ResolveWorkflow(req.Msg.NewId)
		if err == nil && resolved != nil {
			resolved.Workflow.Name = *req.Msg.NewName
			writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
			if _, writeErr := writer.WriteWorkflow(resolved.Workflow, workflow.WriteLevelProject); writeErr != nil {
				s.logger.Warn("failed to update cloned workflow name", "error", writeErr)
			}
		}
	}

	// Sync to database cache
	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after clone", "error", err)
	}

	// Get the cloned workflow from DB for response
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

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Verify phase template exists
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
	if req.Msg.MaxIterationsOverride != nil {
		v := int(*req.Msg.MaxIterationsOverride)
		phase.MaxIterationsOverride = &v
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

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Find the phase
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

	// Apply updates
	if req.Msg.Sequence != nil {
		existingPhase.Sequence = int(*req.Msg.Sequence)
	}
	if len(req.Msg.DependsOn) > 0 {
		existingPhase.DependsOn = dependsOnToJSON(req.Msg.DependsOn)
	}
	if req.Msg.MaxIterationsOverride != nil {
		v := int(*req.Msg.MaxIterationsOverride)
		existingPhase.MaxIterationsOverride = &v
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
	if len(req.Msg.SubAgentsOverride) > 0 {
		existingPhase.SubAgentsOverride = dependsOnToJSON(req.Msg.SubAgentsOverride)
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

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Find the phase by ID to get its template ID
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

	// Delete the phase by workflow ID and template ID
	if err := s.globalDB.DeleteWorkflowPhase(req.Msg.WorkflowId, phaseTemplateID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete phase: %w", err))
	}

	return connect.NewResponse(&orcv1.RemovePhaseResponse{
		Workflow: dbWorkflowToProto(wf),
	}), nil
}

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

	// Verify workflow exists and is not builtin
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

	// Verify workflow exists and is not builtin
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow"))
	}

	// Get existing variable
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

	// Update fields
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

	// Verify workflow exists and is not builtin
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

// ListPhaseTemplates returns all phase templates.
func (s *workflowServer) ListPhaseTemplates(
	ctx context.Context,
	req *connect.Request[orcv1.ListPhaseTemplatesRequest],
) (*connect.Response[orcv1.ListPhaseTemplatesResponse], error) {
	templates, err := s.globalDB.ListPhaseTemplates()
	if err != nil {
		return connect.NewResponse(&orcv1.ListPhaseTemplatesResponse{
			Templates: []*orcv1.PhaseTemplate{},
		}), nil
	}

	// Filter by builtin if requested
	if !req.Msg.IncludeBuiltin {
		var filtered []*db.PhaseTemplate
		for _, t := range templates {
			if !t.IsBuiltin {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	// Build source map from resolver if available
	var resolvedPhases []workflow.ResolvedPhase
	if s.resolver != nil {
		resolvedPhases, _ = s.resolver.ListPhases()
	}
	sourceMap := make(map[string]workflow.Source)
	for _, rp := range resolvedPhases {
		sourceMap[rp.Phase.ID] = rp.Source
	}

	protoTemplates := make([]*orcv1.PhaseTemplate, len(templates))
	sources := make(map[string]orcv1.DefinitionSource, len(templates))

	for i, t := range templates {
		protoTemplates[i] = dbPhaseTemplateToProto(t)

		// Get source from resolver map or fall back to builtin check
		if src, ok := sourceMap[t.ID]; ok {
			sources[t.ID] = workflowSourceToProto(src)
		} else if t.IsBuiltin {
			sources[t.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_EMBEDDED
		} else {
			sources[t.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_PROJECT
		}
	}

	return connect.NewResponse(&orcv1.ListPhaseTemplatesResponse{
		Templates: protoTemplates,
		Sources:   sources,
	}), nil
}

// GetPhaseTemplate returns a single phase template.
func (s *workflowServer) GetPhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.GetPhaseTemplateRequest],
) (*connect.Response[orcv1.GetPhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
	}

	return connect.NewResponse(&orcv1.GetPhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// CreatePhaseTemplate creates a new phase template.
func (s *workflowServer) CreatePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.CreatePhaseTemplateRequest],
) (*connect.Response[orcv1.CreatePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	// Check if exists
	existing, _ := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("phase template %s already exists", req.Msg.Id))
	}

	tmpl := &db.PhaseTemplate{
		ID:               req.Msg.Id,
		Name:             req.Msg.Name,
		PromptSource:     protoPromptSourceToString(req.Msg.PromptSource),
		ProducesArtifact: req.Msg.ProducesArtifact,
		MaxIterations:    int(req.Msg.MaxIterations),
		GateType:         protoGateTypeToString(req.Msg.GateType),
		Checkpoint:       req.Msg.Checkpoint,
		IsBuiltin:        false,
	}
	if req.Msg.Description != nil {
		tmpl.Description = *req.Msg.Description
	}
	if req.Msg.PromptContent != nil {
		tmpl.PromptContent = *req.Msg.PromptContent
	}
	if req.Msg.PromptPath != nil {
		tmpl.PromptPath = *req.Msg.PromptPath
	}
	if req.Msg.OutputSchema != nil {
		tmpl.OutputSchema = *req.Msg.OutputSchema
	}
	if req.Msg.ArtifactType != nil {
		tmpl.ArtifactType = *req.Msg.ArtifactType
	}
	// NOTE: model_override is now set via agent reference, not directly on phase template
	if req.Msg.ThinkingEnabled != nil {
		tmpl.ThinkingEnabled = req.Msg.ThinkingEnabled
	}

	if tmpl.MaxIterations == 0 {
		tmpl.MaxIterations = 20
	}
	if tmpl.PromptSource == "" {
		tmpl.PromptSource = "db"
	}
	if tmpl.GateType == "" {
		tmpl.GateType = "auto"
	}

	if err := s.globalDB.SavePhaseTemplate(tmpl); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.CreatePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// UpdatePhaseTemplate updates an existing phase template.
func (s *workflowServer) UpdatePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.UpdatePhaseTemplateRequest],
) (*connect.Response[orcv1.UpdatePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Resolve the phase template to get source info
	resolved, err := s.resolver.ResolvePhase(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
	}
	if resolved.Source == workflow.SourceEmbedded {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in phase template"))
	}

	// Apply updates to the phase template
	pt := resolved.Phase
	if req.Msg.Name != nil {
		pt.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		pt.Description = *req.Msg.Description
	}
	if req.Msg.PromptSource != nil {
		pt.PromptSource = workflow.PromptSource(protoPromptSourceToString(*req.Msg.PromptSource))
	}
	if req.Msg.PromptContent != nil {
		pt.PromptContent = *req.Msg.PromptContent
	}
	if req.Msg.PromptPath != nil {
		pt.PromptPath = *req.Msg.PromptPath
	}
	if req.Msg.OutputSchema != nil {
		pt.OutputSchema = *req.Msg.OutputSchema
	}
	if req.Msg.ProducesArtifact != nil {
		pt.ProducesArtifact = *req.Msg.ProducesArtifact
	}
	if req.Msg.ArtifactType != nil {
		pt.ArtifactType = *req.Msg.ArtifactType
	}
	if req.Msg.MaxIterations != nil {
		pt.MaxIterations = int(*req.Msg.MaxIterations)
	}
	// NOTE: model_override is now set via agent reference, not directly on phase template
	if req.Msg.ThinkingEnabled != nil {
		pt.ThinkingEnabled = req.Msg.ThinkingEnabled
	}
	if req.Msg.GateType != nil {
		pt.GateType = workflow.GateType(protoGateTypeToString(*req.Msg.GateType))
	}
	if req.Msg.Checkpoint != nil {
		pt.Checkpoint = *req.Msg.Checkpoint
	}

	// Write back to file if source is file-based (not embedded/database)
	writeLevel := workflow.SourceToWriteLevel(resolved.Source)
	if writeLevel != "" {
		writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
		if _, writeErr := writer.WritePhase(pt, writeLevel); writeErr != nil {
			s.logger.Warn("failed to write phase file", "id", req.Msg.Id, "error", writeErr)
			// Fall through to DB update
		}
	}

	// Sync cache to update DB
	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after phase update", "error", err)
	}

	// Get updated phase template from DB for response
	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get updated phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdatePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// DeletePhaseTemplate deletes a phase template.
func (s *workflowServer) DeletePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.DeletePhaseTemplateRequest],
) (*connect.Response[orcv1.DeletePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
	}
	if tmpl.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot delete built-in phase template"))
	}

	// Check if template is used by any workflows
	workflows, err := s.globalDB.ListWorkflows()
	if err == nil {
		for _, wf := range workflows {
			phases, _ := s.globalDB.GetWorkflowPhases(wf.ID)
			for _, p := range phases {
				if p.PhaseTemplateID == req.Msg.Id {
					return nil, connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("phase template is used by workflow: %s", wf.ID))
				}
			}
		}
	}

	if err := s.globalDB.DeletePhaseTemplate(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.DeletePhaseTemplateResponse{
		Message: "deleted",
	}), nil
}

// ClonePhaseTemplate clones a phase template to a new ID.
func (s *workflowServer) ClonePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.ClonePhaseTemplateRequest],
) (*connect.Response[orcv1.ClonePhaseTemplateResponse], error) {
	if req.Msg.SourceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id is required"))
	}
	if req.Msg.NewId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_id is required"))
	}

	// Use file-based cloner to create YAML file at project level
	result, err := s.cloner.ClonePhase(req.Msg.SourceId, req.Msg.NewId, workflow.WriteLevelProject, false)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, workflow.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("source phase template %s not found", req.Msg.SourceId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("clone phase template: %w", err))
	}

	// Update name if provided
	if req.Msg.NewName != nil && *req.Msg.NewName != "" {
		// Re-read the cloned phase, update name, and re-write
		resolved, err := s.resolver.ResolvePhase(req.Msg.NewId)
		if err == nil && resolved != nil {
			resolved.Phase.Name = *req.Msg.NewName
			writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
			if _, writeErr := writer.WritePhase(resolved.Phase, workflow.WriteLevelProject); writeErr != nil {
				s.logger.Warn("failed to update cloned phase template name", "error", writeErr)
			}
		}
	}

	// Sync to database cache
	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after clone", "error", err)
	}

	// Get the cloned phase template from DB for response
	clone, err := s.globalDB.GetPhaseTemplate(req.Msg.NewId)
	if err != nil {
		s.logger.Warn("failed to get cloned phase template from DB", "id", req.Msg.NewId, "error", err)
		// Return a partial response with the result info
		return connect.NewResponse(&orcv1.ClonePhaseTemplateResponse{
			Template: &orcv1.PhaseTemplate{
				Id:   result.DestID,
				Name: result.DestID,
			},
		}), nil
	}

	return connect.NewResponse(&orcv1.ClonePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(clone),
	}), nil
}

// GetPromptContent returns the prompt content for a phase template.
func (s *workflowServer) GetPromptContent(
	ctx context.Context,
	req *connect.Request[orcv1.GetPromptContentRequest],
) (*connect.Response[orcv1.GetPromptContentResponse], error) {
	if req.Msg.PhaseTemplateId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phase_template_id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.PhaseTemplateId)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.PhaseTemplateId))
	}

	var content string
	switch tmpl.PromptSource {
	case "db":
		content = tmpl.PromptContent
	case "embedded":
		content = "<!-- Embedded prompt at: " + tmpl.PromptPath + " -->"
	case "file":
		content = "<!-- File prompt at: " + tmpl.PromptPath + " -->"
	}

	resp := &orcv1.GetPromptContentResponse{
		Content: content,
		Source:  stringToProtoPromptSource(tmpl.PromptSource),
	}
	if tmpl.PromptPath != "" {
		resp.Path = &tmpl.PromptPath
	}

	return connect.NewResponse(resp), nil
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

	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Verify workflow exists
	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}

	// Create workflow run record
	run := &db.WorkflowRun{
		WorkflowID:  req.Msg.WorkflowId,
		ContextType: protoContextTypeToString(req.Msg.ContextType),
		Prompt:      req.Msg.Prompt,
		Status:      "pending",
	}
	if req.Msg.Instructions != nil {
		run.Instructions = *req.Msg.Instructions
	}
	if req.Msg.ContextData != nil {
		if req.Msg.ContextData.TaskId != nil {
			run.TaskID = req.Msg.ContextData.TaskId
		}
	}

	if err := backend.SaveWorkflowRun(run); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow run: %w", err))
	}

	// Note: Actual execution is handled separately by the workflow executor
	// This just creates the run record

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

	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	run, err := backend.GetWorkflowRun(req.Msg.Id)
	if err != nil || run == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow run %s not found", req.Msg.Id))
	}

	// Check if run can be cancelled
	if run.Status != "running" && run.Status != "pending" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("cannot cancel run with status: %s", run.Status))
	}

	// Update status
	run.Status = "cancelled"
	run.Error = "cancelled via API"

	if err := backend.SaveWorkflowRun(run); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save workflow run: %w", err))
	}

	return connect.NewResponse(&orcv1.CancelWorkflowRunResponse{
		Run: dbWorkflowRunToProto(run),
	}), nil
}

// SaveWorkflowLayout bulk-saves node positions for workflow phases.
func (s *workflowServer) SaveWorkflowLayout(
	ctx context.Context,
	req *connect.Request[orcv1.SaveWorkflowLayoutRequest],
) (*connect.Response[orcv1.SaveWorkflowLayoutResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}
	if wf.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in workflow layout"))
	}

	positions := make(map[string][2]float64, len(req.Msg.Positions))
	for _, p := range req.Msg.Positions {
		positions[p.PhaseTemplateId] = [2]float64{p.PositionX, p.PositionY}
	}

	if err := backend.UpdateWorkflowPhasePositions(req.Msg.WorkflowId, positions); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update positions: %w", err))
	}

	return connect.NewResponse(&orcv1.SaveWorkflowLayoutResponse{
		Success: true,
	}), nil
}

// ValidateWorkflow checks workflow structure for cycles, invalid dependency
// references, and invalid loop_to_phase references.
func (s *workflowServer) ValidateWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.ValidateWorkflowRequest],
) (*connect.Response[orcv1.ValidateWorkflowResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}

	phases, err := s.globalDB.GetWorkflowPhases(req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow phases: %w", err))
	}

	var issues []*orcv1.ValidationIssue

	// Build phase ID set for reference validation
	phaseIDs := make(map[string]bool, len(phases))
	for _, p := range phases {
		phaseIDs[p.PhaseTemplateID] = true
	}

	// Check for invalid dependency references
	for _, p := range phases {
		deps := parseDependsOnJSON(p.DependsOn)
		for _, dep := range deps {
			if !phaseIDs[dep] {
				issues = append(issues, &orcv1.ValidationIssue{
					Severity: "error",
					Message:  fmt.Sprintf("phase %q depends on non-existent phase %q", p.PhaseTemplateID, dep),
					PhaseIds: []string{p.PhaseTemplateID},
				})
			}
		}
	}

	// Check for cycles using Kahn's algorithm (same approach as executor/topo_sort.go
	// but reports invalid refs as errors instead of silently skipping)
	cyclePhases := detectCycles(phases, phaseIDs)
	if len(cyclePhases) > 0 {
		issues = append(issues, &orcv1.ValidationIssue{
			Severity: "error",
			Message:  fmt.Sprintf("cycle detected involving phases: %s", strings.Join(cyclePhases, ", ")),
			PhaseIds: cyclePhases,
		})
	}

	// Check for invalid loop_to_phase references
	for _, p := range phases {
		if p.LoopConfig == "" {
			continue
		}
		var lc struct {
			LoopToPhase string `json:"loop_to_phase"`
		}
		if err := json.Unmarshal([]byte(p.LoopConfig), &lc); err != nil {
			continue
		}
		if lc.LoopToPhase != "" && !phaseIDs[lc.LoopToPhase] {
			issues = append(issues, &orcv1.ValidationIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("phase %q has loop_to_phase referencing non-existent phase %q", p.PhaseTemplateID, lc.LoopToPhase),
				PhaseIds: []string{p.PhaseTemplateID},
			})
		}
	}

	hasErrors := false
	for _, issue := range issues {
		if issue.Severity == "error" {
			hasErrors = true
			break
		}
	}
	valid := !hasErrors

	return connect.NewResponse(&orcv1.ValidateWorkflowResponse{
		Valid:  valid,
		Issues: issues,
	}), nil
}

// parseDependsOnJSON extracts phase template IDs from a JSON array string.
func parseDependsOnJSON(raw string) []string {
	if raw == "" || raw == "[]" {
		return nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		return nil
	}
	return deps
}

// detectCycles uses Kahn's algorithm to find phases involved in cycles.
// Only considers dependencies that reference phases in the provided phaseIDs set.
func detectCycles(phases []*db.WorkflowPhase, phaseIDs map[string]bool) []string {
	adjacency := make(map[string][]string, len(phases))
	inDegree := make(map[string]int, len(phases))

	for _, p := range phases {
		inDegree[p.PhaseTemplateID] = 0
	}

	for _, p := range phases {
		deps := parseDependsOnJSON(p.DependsOn)
		seen := make(map[string]bool, len(deps))
		for _, dep := range deps {
			if seen[dep] || !phaseIDs[dep] {
				continue
			}
			seen[dep] = true
			adjacency[dep] = append(adjacency[dep], p.PhaseTemplateID)
			inDegree[p.PhaseTemplateID]++
		}
	}

	// Collect zero-indegree phases
	var queue []string
	for _, p := range phases {
		if inDegree[p.PhaseTemplateID] == 0 {
			queue = append(queue, p.PhaseTemplateID)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++
		for _, depID := range adjacency[current] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	if processed == len(phases) {
		return nil
	}

	var cycled []string
	for id, deg := range inDegree {
		if deg > 0 {
			cycled = append(cycled, id)
		}
	}
	sort.Strings(cycled)
	return cycled
}

// Helper functions for conversion

func dbWorkflowToProto(w *db.Workflow) *orcv1.Workflow {
	if w == nil {
		return nil
	}
	result := &orcv1.Workflow{
		Id:              w.ID,
		Name:            w.Name,
		WorkflowType:    stringToProtoWorkflowType(w.WorkflowType),
		DefaultThinking: w.DefaultThinking,
		IsBuiltin:       w.IsBuiltin,
	}
	if w.Description != "" {
		result.Description = &w.Description
	}
	if w.DefaultModel != "" {
		result.DefaultModel = &w.DefaultModel
	}
	if w.BasedOn != "" {
		result.BasedOn = &w.BasedOn
	}
	return result
}

func stringToProtoWorkflowType(s string) orcv1.WorkflowType {
	switch s {
	case "task":
		return orcv1.WorkflowType_WORKFLOW_TYPE_TASK
	case "branch":
		return orcv1.WorkflowType_WORKFLOW_TYPE_BRANCH
	case "standalone":
		return orcv1.WorkflowType_WORKFLOW_TYPE_STANDALONE
	default:
		return orcv1.WorkflowType_WORKFLOW_TYPE_UNSPECIFIED
	}
}

func protoWorkflowTypeToString(t orcv1.WorkflowType) string {
	switch t {
	case orcv1.WorkflowType_WORKFLOW_TYPE_TASK:
		return "task"
	case orcv1.WorkflowType_WORKFLOW_TYPE_BRANCH:
		return "branch"
	case orcv1.WorkflowType_WORKFLOW_TYPE_STANDALONE:
		return "standalone"
	default:
		return "task"
	}
}

func dbWorkflowPhasesToProto(phases []*db.WorkflowPhase) []*orcv1.WorkflowPhase {
	result := make([]*orcv1.WorkflowPhase, len(phases))
	for i, p := range phases {
		result[i] = &orcv1.WorkflowPhase{
			Id:              int32(p.ID),
			WorkflowId:      p.WorkflowID,
			PhaseTemplateId: p.PhaseTemplateID,
			Sequence:        int32(p.Sequence),
		}
		if p.MaxIterationsOverride != nil {
			v := int32(*p.MaxIterationsOverride)
			result[i].MaxIterationsOverride = &v
		}
		if p.ModelOverride != "" {
			result[i].ModelOverride = &p.ModelOverride
		}
		if p.ThinkingOverride != nil {
			result[i].ThinkingOverride = p.ThinkingOverride
		}
		if p.DependsOn != "" {
			var deps []string
			if err := json.Unmarshal([]byte(p.DependsOn), &deps); err == nil {
				result[i].DependsOn = deps
			}
		}
		if p.PositionX != nil {
			result[i].PositionX = p.PositionX
		}
		if p.PositionY != nil {
			result[i].PositionY = p.PositionY
		}
		if p.LoopConfig != "" {
			result[i].LoopConfig = &p.LoopConfig
		}
		// Agent overrides (must match dbWorkflowPhaseToProto)
		if p.AgentOverride != "" {
			result[i].AgentOverride = &p.AgentOverride
		}
		if p.SubAgentsOverride != "" {
			var subAgentIDs []string
			if err := json.Unmarshal([]byte(p.SubAgentsOverride), &subAgentIDs); err == nil {
				result[i].SubAgentsOverride = subAgentIDs
			}
		}
	}
	return result
}

func dbWorkflowVariablesToProto(vars []*db.WorkflowVariable) []*orcv1.WorkflowVariable {
	result := make([]*orcv1.WorkflowVariable, len(vars))
	for i, v := range vars {
		result[i] = dbWorkflowVariableToProto(v)
	}
	return result
}

func dbWorkflowRunToProto(r *db.WorkflowRun) *orcv1.WorkflowRun {
	if r == nil {
		return nil
	}
	result := &orcv1.WorkflowRun{
		Id:          r.ID,
		WorkflowId:  r.WorkflowID,
		ContextType: stringToProtoContextType(r.ContextType),
		TaskId:      r.TaskID,
		Prompt:      r.Prompt,
		Status:      stringToProtoRunStatus(r.Status),
	}
	if r.Instructions != "" {
		result.Instructions = &r.Instructions
	}
	if r.CurrentPhase != "" {
		result.CurrentPhase = &r.CurrentPhase
	}
	return result
}

func stringToProtoContextType(s string) orcv1.ContextType {
	switch s {
	case "task":
		return orcv1.ContextType_CONTEXT_TYPE_TASK
	case "branch":
		return orcv1.ContextType_CONTEXT_TYPE_BRANCH
	case "pr":
		return orcv1.ContextType_CONTEXT_TYPE_PR
	case "standalone":
		return orcv1.ContextType_CONTEXT_TYPE_STANDALONE
	case "tag":
		return orcv1.ContextType_CONTEXT_TYPE_TAG
	default:
		return orcv1.ContextType_CONTEXT_TYPE_UNSPECIFIED
	}
}

func stringToProtoRunStatus(s string) orcv1.RunStatus {
	switch s {
	case "pending":
		return orcv1.RunStatus_RUN_STATUS_PENDING
	case "running":
		return orcv1.RunStatus_RUN_STATUS_RUNNING
	case "paused":
		return orcv1.RunStatus_RUN_STATUS_PAUSED
	case "completed":
		return orcv1.RunStatus_RUN_STATUS_COMPLETED
	case "failed":
		return orcv1.RunStatus_RUN_STATUS_FAILED
	case "cancelled":
		return orcv1.RunStatus_RUN_STATUS_CANCELLED
	default:
		return orcv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func dbWorkflowRunPhasesToProto(phases []*db.WorkflowRunPhase) []*orcv1.WorkflowRunPhase {
	result := make([]*orcv1.WorkflowRunPhase, len(phases))
	for i, p := range phases {
		result[i] = &orcv1.WorkflowRunPhase{
			Id:              int32(p.ID),
			WorkflowRunId:   p.WorkflowRunID,
			PhaseTemplateId: p.PhaseTemplateID,
			Status:          stringToProtoPhaseStatus(p.Status),
			Iterations:      int32(p.Iterations),
			InputTokens:     int32(p.InputTokens),
			OutputTokens:    int32(p.OutputTokens),
			CostUsd:         p.CostUSD,
		}
		if p.CommitSHA != "" {
			result[i].CommitSha = &p.CommitSHA
		}
	}
	return result
}

func stringToProtoPhaseStatus(s string) orcv1.PhaseStatus {
	switch s {
	case "pending":
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	case "completed":
		return orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	case "skipped":
		return orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
	// Legacy values - all map to pending (not completed)
	case "running", "failed", "paused", "interrupted", "blocked":
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	default:
		return orcv1.PhaseStatus_PHASE_STATUS_UNSPECIFIED
	}
}

func dbWorkflowPhaseToProto(p *db.WorkflowPhase) *orcv1.WorkflowPhase {
	if p == nil {
		return nil
	}
	result := &orcv1.WorkflowPhase{
		Id:              int32(p.ID),
		WorkflowId:      p.WorkflowID,
		PhaseTemplateId: p.PhaseTemplateID,
		Sequence:        int32(p.Sequence),
	}
	// Agent overrides
	if p.AgentOverride != "" {
		result.AgentOverride = &p.AgentOverride
	}
	if p.SubAgentsOverride != "" {
		var subAgentIDs []string
		if err := json.Unmarshal([]byte(p.SubAgentsOverride), &subAgentIDs); err == nil {
			result.SubAgentsOverride = subAgentIDs
		}
	}
	if p.MaxIterationsOverride != nil {
		v := int32(*p.MaxIterationsOverride)
		result.MaxIterationsOverride = &v
	}
	if p.ModelOverride != "" {
		result.ModelOverride = &p.ModelOverride
	}
	if p.ThinkingOverride != nil {
		result.ThinkingOverride = p.ThinkingOverride
	}
	if p.GateTypeOverride != "" {
		gt := stringToProtoGateType(p.GateTypeOverride)
		result.GateTypeOverride = &gt
	}
	if p.Condition != "" {
		result.Condition = &p.Condition
	}
	if p.DependsOn != "" {
		var deps []string
		if err := json.Unmarshal([]byte(p.DependsOn), &deps); err == nil {
			result.DependsOn = deps
		}
	}
	if p.PositionX != nil {
		result.PositionX = p.PositionX
	}
	if p.PositionY != nil {
		result.PositionY = p.PositionY
	}
	if p.LoopConfig != "" {
		result.LoopConfig = &p.LoopConfig
	}
	return result
}

func dbWorkflowVariableToProto(v *db.WorkflowVariable) *orcv1.WorkflowVariable {
	if v == nil {
		return nil
	}
	result := &orcv1.WorkflowVariable{
		Id:              int32(v.ID),
		WorkflowId:      v.WorkflowID,
		Name:            v.Name,
		SourceType:      stringToProtoVariableSourceType(v.SourceType),
		SourceConfig:    v.SourceConfig,
		Required:        v.Required,
		CacheTtlSeconds: int32(v.CacheTTLSeconds),
	}
	if v.Description != "" {
		result.Description = &v.Description
	}
	if v.DefaultValue != "" {
		result.DefaultValue = &v.DefaultValue
	}
	if v.Extract != "" {
		result.Extract = &v.Extract
	}
	return result
}

func dbPhaseTemplateToProto(t *db.PhaseTemplate) *orcv1.PhaseTemplate {
	if t == nil {
		return nil
	}
	result := &orcv1.PhaseTemplate{
		Id:               t.ID,
		Name:             t.Name,
		PromptSource:     stringToProtoPromptSource(t.PromptSource),
		ProducesArtifact: t.ProducesArtifact,
		MaxIterations:    int32(t.MaxIterations),
		GateType:         stringToProtoGateType(t.GateType),
		Checkpoint:       t.Checkpoint,
		IsBuiltin:        t.IsBuiltin,
	}
	if t.Description != "" {
		result.Description = &t.Description
	}
	if t.PromptContent != "" {
		result.PromptContent = &t.PromptContent
	}
	if t.PromptPath != "" {
		result.PromptPath = &t.PromptPath
	}
	if t.OutputSchema != "" {
		result.OutputSchema = &t.OutputSchema
	}
	if t.ArtifactType != "" {
		result.ArtifactType = &t.ArtifactType
	}
	// Agent references (WHO runs this phase)
	if t.AgentID != "" {
		result.AgentId = &t.AgentID
	}
	if t.SubAgents != "" {
		var subAgentIDs []string
		if err := json.Unmarshal([]byte(t.SubAgents), &subAgentIDs); err == nil {
			result.SubAgentIds = subAgentIDs
		}
	}

	// Execution config
	if t.ThinkingEnabled != nil {
		result.ThinkingEnabled = t.ThinkingEnabled
	}
	if t.RetryFromPhase != "" {
		result.RetryFromPhase = &t.RetryFromPhase
	}
	if t.RetryPromptPath != "" {
		result.RetryPromptPath = &t.RetryPromptPath
	}
	return result
}

func stringToProtoPromptSource(s string) orcv1.PromptSource {
	switch s {
	case "embedded":
		return orcv1.PromptSource_PROMPT_SOURCE_EMBEDDED
	case "db":
		return orcv1.PromptSource_PROMPT_SOURCE_DB
	case "file":
		return orcv1.PromptSource_PROMPT_SOURCE_FILE
	default:
		return orcv1.PromptSource_PROMPT_SOURCE_UNSPECIFIED
	}
}

func protoPromptSourceToString(ps orcv1.PromptSource) string {
	switch ps {
	case orcv1.PromptSource_PROMPT_SOURCE_EMBEDDED:
		return "embedded"
	case orcv1.PromptSource_PROMPT_SOURCE_DB:
		return "db"
	case orcv1.PromptSource_PROMPT_SOURCE_FILE:
		return "file"
	default:
		return "db"
	}
}

func stringToProtoGateType(s string) orcv1.GateType {
	switch s {
	case "auto":
		return orcv1.GateType_GATE_TYPE_AUTO
	case "human":
		return orcv1.GateType_GATE_TYPE_HUMAN
	case "skip":
		return orcv1.GateType_GATE_TYPE_SKIP
	default:
		return orcv1.GateType_GATE_TYPE_UNSPECIFIED
	}
}

func protoGateTypeToString(gt orcv1.GateType) string {
	switch gt {
	case orcv1.GateType_GATE_TYPE_AUTO:
		return "auto"
	case orcv1.GateType_GATE_TYPE_HUMAN:
		return "human"
	case orcv1.GateType_GATE_TYPE_SKIP:
		return "skip"
	default:
		return "auto"
	}
}

func stringToProtoVariableSourceType(s string) orcv1.VariableSourceType {
	switch s {
	case "static":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_STATIC
	case "env":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_ENV
	case "script":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_SCRIPT
	case "api":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_API
	case "phase_output":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_PHASE_OUTPUT
	case "prompt_fragment":
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_PROMPT_FRAGMENT
	default:
		return orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_UNSPECIFIED
	}
}

func protoVariableSourceTypeToString(vst orcv1.VariableSourceType) string {
	switch vst {
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_STATIC:
		return "static"
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_ENV:
		return "env"
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_SCRIPT:
		return "script"
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_API:
		return "api"
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_PHASE_OUTPUT:
		return "phase_output"
	case orcv1.VariableSourceType_VARIABLE_SOURCE_TYPE_PROMPT_FRAGMENT:
		return "prompt_fragment"
	default:
		return "static"
	}
}

func protoContextTypeToString(ct orcv1.ContextType) string {
	switch ct {
	case orcv1.ContextType_CONTEXT_TYPE_TASK:
		return "task"
	case orcv1.ContextType_CONTEXT_TYPE_BRANCH:
		return "branch"
	case orcv1.ContextType_CONTEXT_TYPE_PR:
		return "pr"
	case orcv1.ContextType_CONTEXT_TYPE_STANDALONE:
		return "standalone"
	case orcv1.ContextType_CONTEXT_TYPE_TAG:
		return "tag"
	default:
		return "task"
	}
}

// dependsOnToJSON converts []string to JSON array string for db storage
func dependsOnToJSON(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	b, _ := json.Marshal(deps)
	return string(b)
}

// workflowSourceToProto converts a workflow.Source to a proto DefinitionSource.
func workflowSourceToProto(s workflow.Source) orcv1.DefinitionSource {
	switch s {
	case workflow.SourceEmbedded:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_EMBEDDED
	case workflow.SourceProject:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_PROJECT
	case workflow.SourceProjectShared:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_SHARED
	case workflow.SourceProjectLocal:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_LOCAL
	case workflow.SourcePersonalGlobal:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_PERSONAL
	default:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_UNSPECIFIED
	}
}
