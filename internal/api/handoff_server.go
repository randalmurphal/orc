package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	taskproto "github.com/randalmurphal/orc/internal/task"
)

type handoffServer struct {
	orcv1connect.UnimplementedHandoffServiceHandler
	backend          storage.Backend
	projectCache     *ProjectCache
	logger           *slog.Logger
	pendingDecisions *gate.PendingDecisionStore
}

func NewHandoffServer(
	backend storage.Backend,
	logger *slog.Logger,
	pendingDecisions *gate.PendingDecisionStore,
) orcv1connect.HandoffServiceHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &handoffServer{
		backend:          backend,
		logger:           logger,
		pendingDecisions: pendingDecisions,
	}
}

func (s *handoffServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

func (s *handoffServer) getBackend(projectID string) (storage.Backend, error) {
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

func (s *handoffServer) GenerateHandoff(
	ctx context.Context,
	req *connect.Request[orcv1.GenerateHandoffRequest],
) (*connect.Response[orcv1.GenerateHandoffResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	if req.Msg.GetSourceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id is required"))
	}

	sourceType, err := handoffSourceKind(req.Msg.GetSourceType())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	target, err := handoffTargetKind(req.Msg.GetTarget())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	contextPack, err := s.buildContextPack(ctx, backend, req.Msg.GetProjectId(), sourceType, req.Msg.GetSourceId())
	if err != nil {
		return nil, err
	}
	if contextPack == "" {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generated empty context pack for %s %s", sourceType, req.Msg.GetSourceId()))
	}

	bootstrapPrompt, err := controlplane.BuildBootstrapPrompt(sourceType, contextPack)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("build bootstrap prompt: %w", err))
	}
	cliCommand, err := controlplane.BuildCLICommand(target, bootstrapPrompt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("build cli command: %w", err))
	}

	resp := &orcv1.GenerateHandoffResponse{
		ContextPack:     contextPack,
		BootstrapPrompt: bootstrapPrompt,
		CliCommand:      cliCommand,
	}
	return connect.NewResponse(resp), nil
}

func (s *handoffServer) buildContextPack(
	ctx context.Context,
	backend storage.Backend,
	projectID string,
	sourceType controlplane.HandoffSourceKind,
	sourceID string,
) (string, error) {
	switch sourceType {
	case controlplane.HandoffSourceTask:
		exists, err := backend.TaskExists(sourceID)
		if err != nil {
			return "", connect.NewError(connect.CodeInternal, fmt.Errorf("check task %s: %w", sourceID, err))
		}
		if !exists {
			return "", connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", sourceID))
		}
		taskItem, err := backend.LoadTask(sourceID)
		if err != nil {
			return "", connect.NewError(connect.CodeInternal, fmt.Errorf("load task %s: %w", sourceID, err))
		}
		recommendations, err := backend.LoadAllRecommendations()
		if err != nil {
			return "", connect.NewError(connect.CodeInternal, fmt.Errorf("load recommendations for task %s: %w", sourceID, err))
		}
		return controlplane.BuildTaskContextPack(taskItem, taskproto.GetCurrentPhaseProto(taskItem), recommendations), nil

	case controlplane.HandoffSourceThread:
		thread, err := backend.DB().GetThread(sourceID)
		if err != nil {
			return "", connect.NewError(connect.CodeInternal, fmt.Errorf("load thread %s: %w", sourceID, err))
		}
		if thread == nil {
			return "", connect.NewError(connect.CodeNotFound, fmt.Errorf("thread %s not found", sourceID))
		}
		return controlplane.BuildThreadContextPack(threadToProto(thread)), nil

	case controlplane.HandoffSourceRecommendation:
		recommendation, err := backend.LoadRecommendation(sourceID)
		if err != nil {
			return "", connect.NewError(connect.CodeInternal, fmt.Errorf("load recommendation %s: %w", sourceID, err))
		}
		if recommendation == nil {
			return "", connect.NewError(connect.CodeNotFound, fmt.Errorf("recommendation %s not found", sourceID))
		}
		return buildRecommendationContextPack(recommendation), nil

	case controlplane.HandoffSourceAttentionItem:
		item, err := s.loadAttentionItem(ctx, backend, projectID, sourceID)
		if err != nil {
			return "", err
		}
		if item == nil {
			return "", connect.NewError(connect.CodeNotFound, fmt.Errorf("attention item %s not found", sourceID))
		}
		return controlplane.BuildAttentionItemContextPack(item), nil
	}

	return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported handoff source type %q", sourceType))
}

func (s *handoffServer) loadAttentionItem(
	ctx context.Context,
	backend storage.Backend,
	projectID string,
	sourceID string,
) (*orcv1.AttentionItem, error) {
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks for attention item %s: %w", sourceID, err))
	}
	signals, err := backend.LoadActiveAttentionSignals()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load attention signals for %s: %w", sourceID, err))
	}
	for _, signal := range signals {
		if signal == nil || signal.ProjectID != "" || projectID == "" {
			continue
		}
		signal.ProjectID = projectID
	}

	builder := &attentionDashboardServer{
		backend:          backend,
		projectCache:     s.projectCache,
		logger:           s.logger,
		pendingDecisions: s.pendingDecisions,
	}
	items, err := builder.buildAttentionItems(backend, tasks, signals, projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("build attention items for %s: %w", sourceID, err))
	}
	for _, item := range items {
		if item != nil && item.GetId() == sourceID {
			return item, nil
		}
	}
	return nil, nil
}

func handoffSourceKind(sourceType orcv1.HandoffSourceType) (controlplane.HandoffSourceKind, error) {
	switch sourceType {
	case orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_TASK:
		return controlplane.HandoffSourceTask, nil
	case orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_THREAD:
		return controlplane.HandoffSourceThread, nil
	case orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_RECOMMENDATION:
		return controlplane.HandoffSourceRecommendation, nil
	case orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_ATTENTION_ITEM:
		return controlplane.HandoffSourceAttentionItem, nil
	default:
		return "", fmt.Errorf("invalid source_type %s", sourceType.String())
	}
}

func handoffTargetKind(target orcv1.HandoffTarget) (controlplane.HandoffTargetKind, error) {
	switch target {
	case orcv1.HandoffTarget_HANDOFF_TARGET_CLAUDE_CODE:
		return controlplane.HandoffTargetClaudeCode, nil
	case orcv1.HandoffTarget_HANDOFF_TARGET_CODEX:
		return controlplane.HandoffTargetCodex, nil
	default:
		return "", fmt.Errorf("invalid target %s", target.String())
	}
}
