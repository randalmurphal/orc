// Package api provides the Connect RPC and REST API server for orc.
// This file implements the WorkflowService Connect RPC service.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"
)

// workflowServer implements the WorkflowServiceHandler interface.
type workflowServer struct {
	orcv1connect.UnimplementedWorkflowServiceHandler
	backend      storage.Backend // Legacy: single project backend (fallback)
	projectCache *ProjectCache   // Multi-project: cache of backends per project
	globalDB     *db.GlobalDB    // Global DB (workflows, phases, agents)
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

// validateProviderString returns an InvalidArgument error if the provider is not recognized.
// llmkit owns the provider definitions; orc only consults config.IsValidLLMProvider.
func validateProviderString(provider string) error {
	if config.IsValidLLMProvider(provider) {
		return nil
	}
	return connect.NewError(connect.CodeInvalidArgument,
		fmt.Errorf("invalid provider %q (supported: claude, codex)", provider))
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

// Helper functions for conversion

func dbWorkflowToProto(w *db.Workflow) *orcv1.Workflow {
	if w == nil {
		return nil
	}
	result := &orcv1.Workflow{
		Id:              w.ID,
		Name:            w.Name,
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
	if w.DefaultProvider != "" {
		result.DefaultProvider = &w.DefaultProvider
	}
	// Always set completion_action, even if empty (empty means inherit from config)
	result.CompletionAction = &w.CompletionAction
	// Always set target_branch, even if empty (empty means inherit from config)
	result.TargetBranch = &w.TargetBranch
	// Set timestamps
	result.CreatedAt = timestamppb.New(w.CreatedAt)
	result.UpdatedAt = timestamppb.New(w.UpdatedAt)
	return result
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
		if p.RuntimeConfigOverride != "" {
			result[i].RuntimeConfigOverride = &p.RuntimeConfigOverride
		}
		if p.GateTypeOverride != "" {
			gt := stringToProtoGateType(p.GateTypeOverride)
			result[i].GateTypeOverride = &gt
		}
		if p.Condition != "" {
			result[i].Condition = &p.Condition
		}
		if p.ProviderOverride != "" {
			result[i].ProviderOverride = &p.ProviderOverride
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
	if p.RuntimeConfigOverride != "" {
		result.RuntimeConfigOverride = &p.RuntimeConfigOverride
	}
	if p.ProviderOverride != "" {
		result.ProviderOverride = &p.ProviderOverride
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
	// Phase output variable name
	if t.OutputVarName != "" {
		result.OutputVarName = &t.OutputVarName
	}
	// Input variables (JSON array in DB → string slice in proto)
	if t.InputVariables != "" {
		var inputVars []string
		if err := json.Unmarshal([]byte(t.InputVariables), &inputVars); err == nil {
			result.InputVariables = inputVars
		}
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
	if t.RuntimeConfig != "" {
		result.RuntimeConfig = &t.RuntimeConfig
	}
	if t.Provider != "" {
		result.Provider = &t.Provider
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
	case workflow.SourceProjectLocal:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_LOCAL
	case workflow.SourcePersonalGlobal:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_PERSONAL
	default:
		return orcv1.DefinitionSource_DEFINITION_SOURCE_UNSPECIFIED
	}
}
