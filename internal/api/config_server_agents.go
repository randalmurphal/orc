package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ListAgents returns agents with runtime statistics and status.
// When no scope is specified, returns agents from both project (SQLite) and global sources.
// When scope is PROJECT, returns only SQLite agents.
// When scope is GLOBAL, returns only global agents from .claude/agents/ directory.
func (s *configServer) ListAgents(
	ctx context.Context,
	req *connect.Request[orcv1.ListAgentsRequest],
) (*connect.Response[orcv1.ListAgentsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	var scope orcv1.SettingsScope
	if req.Msg.Scope != nil {
		scope = *req.Msg.Scope
	}

	var protoAgents []*orcv1.Agent

	today := time.Now().Truncate(24 * time.Hour)
	stats, err := pdb.GetAgentStats(today)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to get agent stats", "error", err)
		}
		stats = make(map[string]*db.AgentStats)
	}

	switch scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
	case orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT:
		dbAgents, err := pdb.ListAgents()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list agents: %w", err))
		}
		protoAgents = make([]*orcv1.Agent, len(dbAgents))
		for i, a := range dbAgents {
			protoAgents[i] = dbAgentToProto(a, stats[a.Model], orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT)
		}
	default:
		dbAgents, err := pdb.ListAgents()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list agents: %w", err))
		}
		for _, a := range dbAgents {
			protoAgents = append(protoAgents, dbAgentToProto(a, stats[a.Model], orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT))
		}
	}

	return connect.NewResponse(&orcv1.ListAgentsResponse{
		Agents: protoAgents,
	}), nil
}

// GetAgent returns a single agent by name.
func (s *configServer) GetAgent(
	ctx context.Context,
	req *connect.Request[orcv1.GetAgentRequest],
) (*connect.Response[orcv1.GetAgentResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	agent, err := pdb.GetAgent(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get agent: %w", err))
	}
	if agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.Name))
	}

	return connect.NewResponse(&orcv1.GetAgentResponse{
		Agent: dbAgentToProto(agent, nil, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT),
	}), nil
}

// CreateAgent creates a new custom agent.
func (s *configServer) CreateAgent(
	ctx context.Context,
	req *connect.Request[orcv1.CreateAgentRequest],
) (*connect.Response[orcv1.CreateAgentResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Description == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("description is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	existing, err := pdb.GetAgent(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("check existing agent: %w", err))
	}
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("agent %s already exists", req.Msg.Name))
	}

	agent := &db.Agent{
		ID:          req.Msg.Name,
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		IsBuiltin:   false,
	}
	if req.Msg.Prompt != nil {
		agent.Prompt = *req.Msg.Prompt
	}
	if req.Msg.SystemPrompt != nil {
		agent.SystemPrompt = *req.Msg.SystemPrompt
	}
	if req.Msg.RuntimeConfig != nil {
		agent.RuntimeConfig = *req.Msg.RuntimeConfig
	}
	if req.Msg.Model != nil {
		agent.Model = *req.Msg.Model
	}
	if req.Msg.Tools != nil && len(req.Msg.Tools.Allow) > 0 {
		agent.Tools = req.Msg.Tools.Allow
	}
	if req.Msg.Provider != nil {
		agent.Provider = *req.Msg.Provider
	}

	if err := pdb.SaveAgent(agent); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save agent: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateAgentResponse{
		Agent: dbAgentToProto(agent, nil, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT),
	}), nil
}

// UpdateAgent updates an existing custom agent.
func (s *configServer) UpdateAgent(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateAgentRequest],
) (*connect.Response[orcv1.UpdateAgentResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	agent, err := pdb.GetAgent(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get agent: %w", err))
	}
	if agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.GetId()))
	}
	if agent.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in agent"))
	}

	if req.Msg.Name != nil {
		agent.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		agent.Description = *req.Msg.Description
	}
	if req.Msg.Prompt != nil {
		agent.Prompt = *req.Msg.Prompt
	}
	if req.Msg.SystemPrompt != nil {
		agent.SystemPrompt = *req.Msg.SystemPrompt
	}
	if req.Msg.RuntimeConfig != nil {
		agent.RuntimeConfig = *req.Msg.RuntimeConfig
	}
	if req.Msg.Model != nil {
		agent.Model = *req.Msg.Model
	}
	if req.Msg.Tools != nil {
		agent.Tools = req.Msg.Tools.Allow
	}
	if req.Msg.Provider != nil {
		agent.Provider = *req.Msg.Provider
	}

	if err := pdb.SaveAgent(agent); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save agent: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateAgentResponse{
		Agent: dbAgentToProto(agent, nil, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT),
	}), nil
}

// DeleteAgent deletes a custom agent.
func (s *configServer) DeleteAgent(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteAgentRequest],
) (*connect.Response[orcv1.DeleteAgentResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	agent, err := pdb.GetAgent(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get agent: %w", err))
	}
	if agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.Name))
	}
	if agent.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot delete built-in agent"))
	}

	if err := pdb.DeleteAgent(req.Msg.Name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete agent: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteAgentResponse{
		Message: fmt.Sprintf("Agent %s deleted successfully", req.Msg.Name),
	}), nil
}
