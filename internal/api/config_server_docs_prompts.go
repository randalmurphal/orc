package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/v2/claudeconfig"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/prompt"
)

// GetClaudeMd returns CLAUDE.md content.
func (s *configServer) GetClaudeMd(
	ctx context.Context,
	req *connect.Request[orcv1.GetClaudeMdRequest],
) (*connect.Response[orcv1.GetClaudeMdResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var files []*orcv1.ClaudeMd

	homeDir, _ := os.UserHomeDir()
	globalPath := filepath.Join(homeDir, "CLAUDE.md")
	if content, err := os.ReadFile(globalPath); err == nil {
		files = append(files, &orcv1.ClaudeMd{
			Path:    globalPath,
			Content: string(content),
			Scope:   orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL,
		})
	}

	projectPath := filepath.Join(workDir, "CLAUDE.md")
	if content, err := os.ReadFile(projectPath); err == nil {
		files = append(files, &orcv1.ClaudeMd{
			Path:    projectPath,
			Content: string(content),
			Scope:   orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
		})
	}

	return connect.NewResponse(&orcv1.GetClaudeMdResponse{
		Files: files,
	}), nil
}

// UpdateClaudeMd updates CLAUDE.md content.
func (s *configServer) UpdateClaudeMd(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateClaudeMdRequest],
) (*connect.Response[orcv1.UpdateClaudeMdResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var path string
	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		path = filepath.Join(homeDir, "CLAUDE.md")
	} else {
		path = filepath.Join(workDir, "CLAUDE.md")
	}

	if err := os.WriteFile(path, []byte(req.Msg.Content), 0644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write CLAUDE.md: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateClaudeMdResponse{
		ClaudeMd: &orcv1.ClaudeMd{
			Path:    path,
			Content: req.Msg.Content,
			Scope:   req.Msg.Scope,
		},
	}), nil
}

// GetConstitution returns the constitution.
func (s *configServer) GetConstitution(
	ctx context.Context,
	req *connect.Request[orcv1.GetConstitutionRequest],
) (*connect.Response[orcv1.GetConstitutionResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	content, path, err := backend.LoadConstitution()
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("constitution not found"))
	}

	return connect.NewResponse(&orcv1.GetConstitutionResponse{
		Constitution: &orcv1.Constitution{
			Content: content,
			Path:    &path,
		},
	}), nil
}

// UpdateConstitution updates the constitution.
func (s *configServer) UpdateConstitution(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateConstitutionRequest],
) (*connect.Response[orcv1.UpdateConstitutionResponse], error) {
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := backend.SaveConstitution(req.Msg.Content); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	_, path, _ := backend.LoadConstitution()

	return connect.NewResponse(&orcv1.UpdateConstitutionResponse{
		Constitution: &orcv1.Constitution{
			Content: req.Msg.Content,
			Path:    &path,
		},
	}), nil
}

// DeleteConstitution deletes the constitution.
func (s *configServer) DeleteConstitution(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteConstitutionRequest],
) (*connect.Response[orcv1.DeleteConstitutionResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := backend.DeleteConstitution(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.DeleteConstitutionResponse{
		Message: "constitution deleted",
	}), nil
}

// GetConfigStats returns configuration stats for the settings page.
func (s *configServer) GetConfigStats(
	ctx context.Context,
	req *connect.Request[orcv1.GetConfigStatsRequest],
) (*connect.Response[orcv1.GetConfigStatsResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	stats := &orcv1.ConfigStats{}
	homeDir, _ := os.UserHomeDir()
	globalClaudeDir := filepath.Join(homeDir, ".claude")
	projectClaudeDir := filepath.Join(workDir, ".claude")

	globalSkills, _ := claudeconfig.DiscoverSkills(globalClaudeDir)
	projectSkills, _ := claudeconfig.DiscoverSkills(projectClaudeDir)
	globalCommands := discoverCommands(globalClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL)
	projectCommands := discoverCommands(projectClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT)
	stats.SlashCommandsCount = int32(len(globalSkills) + len(projectSkills) + len(globalCommands) + len(projectCommands))

	var claudeMdSize int64
	if info, err := os.Stat(filepath.Join(homeDir, "CLAUDE.md")); err == nil {
		claudeMdSize += info.Size()
	}
	if info, err := os.Stat(filepath.Join(workDir, "CLAUDE.md")); err == nil {
		claudeMdSize += info.Size()
	}
	stats.ClaudeMdSize = claudeMdSize

	mcpCount, _ := claudeconfig.CountMCPServers(workDir)
	stats.McpServersCount = int32(mcpCount)

	settings, _ := claudeconfig.LoadSettings(workDir)
	if settings != nil && settings.Permissions != nil {
		if len(settings.Permissions.Allow) > 0 && len(settings.Permissions.Deny) == 0 {
			stats.PermissionsProfile = "allowlist"
		} else if len(settings.Permissions.Deny) > 0 && len(settings.Permissions.Allow) == 0 {
			stats.PermissionsProfile = "denylist"
		} else if len(settings.Permissions.Allow) > 0 && len(settings.Permissions.Deny) > 0 {
			stats.PermissionsProfile = "mixed"
		} else {
			stats.PermissionsProfile = "default"
		}
	} else {
		stats.PermissionsProfile = "default"
	}

	return connect.NewResponse(&orcv1.GetConfigStatsResponse{
		Stats: stats,
	}), nil
}

// ListPrompts returns all available prompts.
func (s *configServer) ListPrompts(
	ctx context.Context,
	req *connect.Request[orcv1.ListPromptsRequest],
) (*connect.Response[orcv1.ListPromptsResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	svc := prompt.NewService(filepath.Join(workDir, ".orc"))
	prompts, err := svc.List()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list prompts: %w", err))
	}

	protoPrompts := make([]*orcv1.PromptTemplate, len(prompts))
	for i, p := range prompts {
		protoPrompts[i] = promptInfoToProto(&p)
	}

	return connect.NewResponse(&orcv1.ListPromptsResponse{
		Prompts: protoPrompts,
	}), nil
}

// GetPrompt returns a specific prompt.
func (s *configServer) GetPrompt(
	ctx context.Context,
	req *connect.Request[orcv1.GetPromptRequest],
) (*connect.Response[orcv1.GetPromptResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	svc := prompt.NewService(filepath.Join(workDir, ".orc"))
	p, err := svc.Get(req.Msg.Phase)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prompt not found"))
	}

	return connect.NewResponse(&orcv1.GetPromptResponse{
		Prompt: promptToProto(p),
	}), nil
}

// GetDefaultPrompt returns the default prompt for a phase.
func (s *configServer) GetDefaultPrompt(
	ctx context.Context,
	req *connect.Request[orcv1.GetDefaultPromptRequest],
) (*connect.Response[orcv1.GetDefaultPromptResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	svc := prompt.NewService(filepath.Join(workDir, ".orc"))
	p, err := svc.GetDefault(req.Msg.Phase)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("default prompt not found"))
	}

	return connect.NewResponse(&orcv1.GetDefaultPromptResponse{
		Prompt: promptToProto(p),
	}), nil
}

// UpdatePrompt updates a prompt.
func (s *configServer) UpdatePrompt(
	ctx context.Context,
	req *connect.Request[orcv1.UpdatePromptRequest],
) (*connect.Response[orcv1.UpdatePromptResponse], error) {
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	svc := prompt.NewService(filepath.Join(workDir, ".orc"))
	if err := svc.Save(req.Msg.Phase, req.Msg.Content); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save prompt: %w", err))
	}

	p, err := svc.Get(req.Msg.Phase)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reload prompt: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdatePromptResponse{
		Prompt: promptToProto(p),
	}), nil
}

// DeletePrompt deletes a custom prompt.
func (s *configServer) DeletePrompt(
	ctx context.Context,
	req *connect.Request[orcv1.DeletePromptRequest],
) (*connect.Response[orcv1.DeletePromptResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	svc := prompt.NewService(filepath.Join(workDir, ".orc"))

	if !svc.HasOverride(req.Msg.Phase) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no override exists for this phase"))
	}

	if err := svc.Delete(req.Msg.Phase); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete prompt: %w", err))
	}

	return connect.NewResponse(&orcv1.DeletePromptResponse{
		Message: "prompt deleted",
	}), nil
}

// ListPromptVariables lists available prompt variables.
func (s *configServer) ListPromptVariables(
	ctx context.Context,
	req *connect.Request[orcv1.ListPromptVariablesRequest],
) (*connect.Response[orcv1.ListPromptVariablesResponse], error) {
	vars := prompt.GetVariableReference()
	protoVars := make([]*orcv1.PromptVariable, 0, len(vars))
	for name, description := range vars {
		protoVars = append(protoVars, &orcv1.PromptVariable{
			Name:        name,
			Description: description,
		})
	}

	return connect.NewResponse(&orcv1.ListPromptVariablesResponse{
		Variables: protoVars,
	}), nil
}
