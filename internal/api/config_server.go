// Package api provides the Connect RPC and REST API server for orc.
// This file implements the ConfigService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/claudeconfig"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/storage"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
)

// configServer implements the ConfigServiceHandler interface.
type configServer struct {
	orcv1connect.UnimplementedConfigServiceHandler
	orcConfig    *config.Config
	backend      storage.Backend
	projectCache *ProjectCache
	workDir      string
	logger       *slog.Logger
}

// NewConfigServer creates a new ConfigService handler.
func NewConfigServer(
	orcConfig *config.Config,
	backend storage.Backend,
	workDir string,
	logger *slog.Logger,
) orcv1connect.ConfigServiceHandler {
	return &configServer{
		orcConfig: orcConfig,
		backend:   backend,
		workDir:   workDir,
		logger:    logger,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *configServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Otherwise returns the default backend.
func (s *configServer) getBackend(projectID string) (storage.Backend, error) {
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

// GetConfig returns the ORC configuration.
func (s *configServer) GetConfig(
	ctx context.Context,
	req *connect.Request[orcv1.GetConfigRequest],
) (*connect.Response[orcv1.GetConfigResponse], error) {
	cfg := s.orcConfig
	if cfg == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("config not found"))
	}

	return connect.NewResponse(&orcv1.GetConfigResponse{
		Config: orcConfigToProto(cfg),
	}), nil
}

// ValidModels is the list of allowed model identifiers for the DefaultModel setting.
var ValidModels = []string{
	"claude-sonnet-4-20250514",
	"claude-opus-4-20250514",
	"claude-haiku-3-5-20241022",
}

// UpdateConfig updates the ORC configuration and persists to config.yaml.
func (s *configServer) UpdateConfig(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateConfigRequest],
) (*connect.Response[orcv1.UpdateConfigResponse], error) {
	configPath := filepath.Join(s.workDir, config.OrcDir, config.ConfigFileName)

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		cfg = config.Default()
	}

	// Apply automation updates
	if req.Msg.Automation != nil {
		cfg.Automation.AutoApprove = req.Msg.Automation.AutoApprove
	}

	// Apply execution updates
	if req.Msg.Execution != nil {
		// parallel_tasks: 0 means "not provided" in proto3 (valid range is 1-5)
		if req.Msg.Execution.ParallelTasks != 0 {
			if req.Msg.Execution.ParallelTasks < 1 || req.Msg.Execution.ParallelTasks > 5 {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("parallel_tasks must be between 1 and 5, got %d", req.Msg.Execution.ParallelTasks))
			}
			cfg.Execution.ParallelTasks = int(req.Msg.Execution.ParallelTasks)
		}

		// cost_limit: 0 is valid (means $0), range 0-100
		if req.Msg.Execution.CostLimit < 0 || req.Msg.Execution.CostLimit > 100 {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("cost_limit must be between 0 and 100, got %d", req.Msg.Execution.CostLimit))
		}
		cfg.Execution.CostLimit = int(req.Msg.Execution.CostLimit)
	}

	// Apply completion updates
	if req.Msg.Completion != nil {
		c := req.Msg.Completion
		if c.Action != "" {
			cfg.Completion.Action = c.Action
		}
		cfg.Completion.MergeOnCIPass = c.AutoMerge
		cfg.Completion.DeleteBranch = c.DeleteBranch
		if c.TargetBranch != nil {
			cfg.Completion.TargetBranch = *c.TargetBranch
		}
		if c.Pr != nil {
			cfg.Completion.PR.Draft = c.Pr.Draft
			cfg.Completion.PR.Labels = c.Pr.Labels
			cfg.Completion.PR.Reviewers = c.Pr.Reviewers
			cfg.Completion.PR.TeamReviewers = c.Pr.TeamReviewers
			cfg.Completion.PR.Assignees = c.Pr.Assignees
			cfg.Completion.PR.MaintainerCanModify = c.Pr.MaintainerCanModify
			cfg.Completion.PR.AutoApprove = c.Pr.AutoApprove
			cfg.Completion.PR.AutoMerge = c.Pr.AutoMerge
		}
		if c.Ci != nil {
			cfg.Completion.CI.WaitForCI = c.Ci.WaitForCi
			if c.Ci.CiTimeout > 0 {
				cfg.Completion.CI.CITimeout = time.Duration(c.Ci.CiTimeout) * time.Minute
			}
			if c.Ci.PollInterval > 0 {
				cfg.Completion.CI.PollInterval = time.Duration(c.Ci.PollInterval) * time.Second
			}
			cfg.Completion.CI.MergeOnCIPass = c.Ci.MergeOnCiPass
			if c.Ci.MergeMethod != "" {
				cfg.Completion.CI.MergeMethod = c.Ci.MergeMethod
			}
			cfg.Completion.CI.MergeCommitTemplate = c.Ci.MergeCommitTemplate
			cfg.Completion.CI.SquashCommitTemplate = c.Ci.SquashCommitTemplate
			cfg.Completion.CI.VerifySHAOnMerge = c.Ci.VerifyShaOnMerge
		}
	}

	// Apply Jira config updates
	if req.Msg.Jira != nil {
		j := req.Msg.Jira
		if j.Url != "" {
			cfg.Jira.URL = j.Url
		}
		if j.Email != "" {
			cfg.Jira.Email = j.Email
		}
		if j.TokenEnvVar != "" {
			cfg.Jira.TokenEnvVar = j.TokenEnvVar
		}
		if j.EpicToInitiative != nil {
			cfg.Jira.EpicToInitiative = j.EpicToInitiative
		}
		if j.DefaultWeight != "" {
			cfg.Jira.DefaultWeight = j.DefaultWeight
		}
		if j.DefaultQueue != "" {
			cfg.Jira.DefaultQueue = j.DefaultQueue
		}
		if len(j.CustomFields) > 0 {
			cfg.Jira.CustomFields = j.CustomFields
		}
		if len(j.DefaultProjects) > 0 {
			cfg.Jira.DefaultProjects = j.DefaultProjects
		}
		if len(j.StatusOverrides) > 0 {
			cfg.Jira.StatusOverrides = j.StatusOverrides
		}
		if len(j.CategoryOverrides) > 0 {
			cfg.Jira.CategoryOverrides = j.CategoryOverrides
		}
		if len(j.PriorityOverrides) > 0 {
			cfg.Jira.PriorityOverrides = j.PriorityOverrides
		}
	}

	// Apply claude/model updates
	if req.Msg.Claude != nil && req.Msg.Claude.Model != "" {
		if !slices.Contains(ValidModels, req.Msg.Claude.Model) {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid model: %s", req.Msg.Claude.Model))
		}
		cfg.Model = req.Msg.Claude.Model
	}

	// Persist to file
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("create config directory: %w", err))
	}
	if err := cfg.SaveTo(configPath); err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("save config: %w", err))
	}

	// Update in-memory config
	s.orcConfig = cfg

	return connect.NewResponse(&orcv1.UpdateConfigResponse{
		Config: orcConfigToProto(cfg),
	}), nil
}

// GetSettings returns Claude Code settings.
func (s *configServer) GetSettings(
	ctx context.Context,
	req *connect.Request[orcv1.GetSettingsRequest],
) (*connect.Response[orcv1.GetSettingsResponse], error) {
	var settings *claudeconfig.Settings
	var err error

	switch req.Msg.Scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		settings, err = claudeconfig.LoadGlobalSettings()
	case orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT:
		settings, err = claudeconfig.LoadProjectSettings(s.workDir)
	default:
		// Merged (default)
		settings, err = claudeconfig.LoadSettings(s.workDir)
	}

	if err != nil {
		// Return empty settings on error
		settings = &claudeconfig.Settings{}
	}

	return connect.NewResponse(&orcv1.GetSettingsResponse{
		Settings: claudeSettingsToProto(settings),
	}), nil
}

// UpdateSettings updates Claude Code settings.
func (s *configServer) UpdateSettings(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateSettingsRequest],
) (*connect.Response[orcv1.UpdateSettingsResponse], error) {
	settings := protoToClaudeSettings(req.Msg.Settings)

	var err error
	switch req.Msg.Scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		err = claudeconfig.SaveGlobalSettings(settings)
	default:
		err = claudeconfig.SaveProjectSettings(s.workDir, settings)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save settings: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateSettingsResponse{
		Settings: claudeSettingsToProto(settings),
	}), nil
}

// GetSettingsHierarchy returns settings with source information.
func (s *configServer) GetSettingsHierarchy(
	ctx context.Context,
	req *connect.Request[orcv1.GetSettingsHierarchyRequest],
) (*connect.Response[orcv1.GetSettingsHierarchyResponse], error) {
	globalSettings, _ := claudeconfig.LoadGlobalSettings()
	projectSettings, _ := claudeconfig.LoadProjectSettings(s.workDir)
	mergedSettings, _ := claudeconfig.LoadSettings(s.workDir)

	return connect.NewResponse(&orcv1.GetSettingsHierarchyResponse{
		Hierarchy: &orcv1.SettingsHierarchy{
			Global:  claudeSettingsToProto(globalSettings),
			Project: claudeSettingsToProto(projectSettings),
			Merged:  claudeSettingsToProto(mergedSettings),
		},
	}), nil
}

// ListHooks returns all hooks.
func (s *configServer) ListHooks(
	ctx context.Context,
	req *connect.Request[orcv1.ListHooksRequest],
) (*connect.Response[orcv1.ListHooksResponse], error) {
	var settings *claudeconfig.Settings
	var err error
	var scope orcv1.SettingsScope

	if req.Msg.Scope != nil {
		scope = *req.Msg.Scope
	}

	switch scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		settings, err = claudeconfig.LoadGlobalSettings()
	default:
		settings, err = claudeconfig.LoadProjectSettings(s.workDir)
		scope = orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	}

	if err != nil || settings == nil || settings.Hooks == nil {
		return connect.NewResponse(&orcv1.ListHooksResponse{
			Hooks: []*orcv1.Hook{},
		}), nil
	}

	var hooks []*orcv1.Hook
	for event, eventHooks := range settings.Hooks {
		for _, h := range eventHooks {
			// Each Hook contains a Matcher and array of HookEntry
			for _, entry := range h.Hooks {
				hooks = append(hooks, &orcv1.Hook{
					Name:    entry.Command, // Use command as identifier
					Event:   stringToProtoHookEvent(event),
					Command: entry.Command,
					Enabled: true,
					Scope:   scope,
					Matcher: func() *string {
						if h.Matcher != "" {
							return &h.Matcher
						}
						return nil
					}(),
				})
			}
		}
	}

	return connect.NewResponse(&orcv1.ListHooksResponse{
		Hooks: hooks,
	}), nil
}

// CreateHook creates a new hook.
func (s *configServer) CreateHook(
	ctx context.Context,
	req *connect.Request[orcv1.CreateHookRequest],
) (*connect.Response[orcv1.CreateHookResponse], error) {
	var settings *claudeconfig.Settings
	var err error

	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.workDir)
	}

	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	event := protoHookEventToString(req.Msg.Event)
	matcher := ""
	if req.Msg.Matcher != nil {
		matcher = *req.Msg.Matcher
	}

	// Create new hook entry
	hookEntry := claudeconfig.HookEntry{
		Type:    "command",
		Command: req.Msg.Command,
	}

	// Find or create hook with matching matcher
	found := false
	for i, h := range settings.Hooks[event] {
		if h.Matcher == matcher {
			settings.Hooks[event][i].Hooks = append(settings.Hooks[event][i].Hooks, hookEntry)
			found = true
			break
		}
	}
	if !found {
		settings.Hooks[event] = append(settings.Hooks[event], claudeconfig.Hook{
			Matcher: matcher,
			Hooks:   []claudeconfig.HookEntry{hookEntry},
		})
	}

	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		err = claudeconfig.SaveGlobalSettings(settings)
	} else {
		err = claudeconfig.SaveProjectSettings(s.workDir, settings)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save settings: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateHookResponse{
		Hook: &orcv1.Hook{
			Name:    req.Msg.Command,
			Event:   req.Msg.Event,
			Command: req.Msg.Command,
			Matcher: req.Msg.Matcher,
			Enabled: true,
			Scope:   req.Msg.Scope,
		},
	}), nil
}

// UpdateHook updates an existing hook.
func (s *configServer) UpdateHook(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateHookRequest],
) (*connect.Response[orcv1.UpdateHookResponse], error) {
	// Hook updates are complex due to nested structure
	// For now, recommend delete + create workflow
	return nil, connect.NewError(connect.CodeUnimplemented,
		errors.New("hook updates not supported - use delete + create"))
}

// DeleteHook deletes a hook.
func (s *configServer) DeleteHook(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteHookRequest],
) (*connect.Response[orcv1.DeleteHookResponse], error) {
	var settings *claudeconfig.Settings
	var err error

	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		settings, err = claudeconfig.LoadGlobalSettings()
	} else {
		settings, err = claudeconfig.LoadProjectSettings(s.workDir)
	}

	if err != nil || settings == nil || settings.Hooks == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("hook not found"))
	}

	// Find and delete hook by command name
	found := false
	for event, hooks := range settings.Hooks {
		for i := range hooks {
			for j, entry := range hooks[i].Hooks {
				if entry.Command == req.Msg.Name {
					// Remove the entry
					hooks[i].Hooks = append(hooks[i].Hooks[:j], hooks[i].Hooks[j+1:]...)
					// If no more entries, remove the hook
					if len(hooks[i].Hooks) == 0 {
						settings.Hooks[event] = append(hooks[:i], hooks[i+1:]...)
					} else {
						settings.Hooks[event] = hooks
					}
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("hook not found"))
	}

	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		err = claudeconfig.SaveGlobalSettings(settings)
	} else {
		err = claudeconfig.SaveProjectSettings(s.workDir, settings)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save settings: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteHookResponse{
		Message: "hook deleted",
	}), nil
}

// ListSkills returns all skills.
// When no scope is specified, returns ALL skills (global + project) to match GetConfigStats behavior.
// When a specific scope is provided, returns only skills from that scope.
func (s *configServer) ListSkills(
	ctx context.Context,
	req *connect.Request[orcv1.ListSkillsRequest],
) (*connect.Response[orcv1.ListSkillsResponse], error) {
	// When no scope specified, return ALL skills (global + project)
	// This matches GetConfigStats.slashCommandsCount behavior
	if req.Msg.Scope == nil {
		var protoSkills []*orcv1.Skill

		// Collect global skills and commands
		homeDir, err := os.UserHomeDir()
		if err == nil {
			globalClaudeDir := filepath.Join(homeDir, ".claude")
			globalSkills, _ := claudeconfig.DiscoverSkills(globalClaudeDir)
			for _, skill := range globalSkills {
				protoSkills = append(protoSkills, claudeSkillToProto(skill, orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL))
			}
			protoSkills = append(protoSkills, discoverCommands(globalClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL)...)
		}

		// Collect project skills and commands
		projectClaudeDir := filepath.Join(s.workDir, ".claude")
		projectSkills, _ := claudeconfig.DiscoverSkills(projectClaudeDir)
		for _, skill := range projectSkills {
			protoSkills = append(protoSkills, claudeSkillToProto(skill, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT))
		}
		protoSkills = append(protoSkills, discoverCommands(projectClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT)...)

		return connect.NewResponse(&orcv1.ListSkillsResponse{
			Skills: protoSkills,
		}), nil
	}

	// Scope-specific behavior (preserved from original)
	var claudeDir string
	scope := *req.Msg.Scope

	if scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		claudeDir = filepath.Join(homeDir, ".claude")
	} else {
		claudeDir = filepath.Join(s.workDir, ".claude")
	}

	var protoSkills []*orcv1.Skill

	skills, err := claudeconfig.DiscoverSkills(claudeDir)
	if err == nil {
		for _, skill := range skills {
			protoSkills = append(protoSkills, claudeSkillToProto(skill, scope))
		}
	}

	protoSkills = append(protoSkills, discoverCommands(claudeDir, scope)...)

	return connect.NewResponse(&orcv1.ListSkillsResponse{
		Skills: protoSkills,
	}), nil
}

// CreateSkill creates a new skill.
func (s *configServer) CreateSkill(
	ctx context.Context,
	req *connect.Request[orcv1.CreateSkillRequest],
) (*connect.Response[orcv1.CreateSkillResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	var skillDir string
	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		skillDir = filepath.Join(homeDir, ".claude", "skills", req.Msg.Name)
	} else {
		skillDir = filepath.Join(s.workDir, ".claude", "skills", req.Msg.Name)
	}

	skill := &claudeconfig.Skill{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Content:     req.Msg.Content,
	}

	if err := claudeconfig.WriteSkillMD(skill, skillDir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create skill: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateSkillResponse{
		Skill: claudeSkillToProto(skill, req.Msg.Scope),
	}), nil
}

// UpdateSkill updates an existing skill.
func (s *configServer) UpdateSkill(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateSkillRequest],
) (*connect.Response[orcv1.UpdateSkillResponse], error) {
	var baseDir string
	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		baseDir = filepath.Join(homeDir, ".claude", "skills")
	} else {
		baseDir = filepath.Join(s.workDir, ".claude", "skills")
	}

	skillDir := filepath.Join(baseDir, req.Msg.Name)

	// Check if skill exists
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("skill not found"))
	}

	// Load existing skill
	skill, err := claudeconfig.ParseSkillMD(skillPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load skill: %w", err))
	}

	// Apply updates
	if req.Msg.Description != nil {
		skill.Description = *req.Msg.Description
	}
	if req.Msg.Content != nil {
		skill.Content = *req.Msg.Content
	}

	if err := claudeconfig.WriteSkillMD(skill, skillDir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update skill: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateSkillResponse{
		Skill: claudeSkillToProto(skill, req.Msg.Scope),
	}), nil
}

// DeleteSkill deletes a skill.
func (s *configServer) DeleteSkill(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteSkillRequest],
) (*connect.Response[orcv1.DeleteSkillResponse], error) {
	var skillDir string
	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		skillDir = filepath.Join(homeDir, ".claude", "skills", req.Msg.Name)
	} else {
		skillDir = filepath.Join(s.workDir, ".claude", "skills", req.Msg.Name)
	}

	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("skill not found"))
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete skill: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteSkillResponse{
		Message: "skill deleted",
	}), nil
}

// GetClaudeMd returns CLAUDE.md content.
func (s *configServer) GetClaudeMd(
	ctx context.Context,
	req *connect.Request[orcv1.GetClaudeMdRequest],
) (*connect.Response[orcv1.GetClaudeMdResponse], error) {
	var files []*orcv1.ClaudeMd

	// Check global CLAUDE.md
	homeDir, _ := os.UserHomeDir()
	globalPath := filepath.Join(homeDir, "CLAUDE.md")
	if content, err := os.ReadFile(globalPath); err == nil {
		files = append(files, &orcv1.ClaudeMd{
			Path:    globalPath,
			Content: string(content),
			Scope:   orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL,
		})
	}

	// Check project CLAUDE.md
	projectPath := filepath.Join(s.workDir, "CLAUDE.md")
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
	var path string
	if req.Msg.Scope == orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		path = filepath.Join(homeDir, "CLAUDE.md")
	} else {
		path = filepath.Join(s.workDir, "CLAUDE.md")
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
	// TODO: use req.Msg.GetProjectId() once config.proto GetConstitutionRequest has project_id
	backend, err := s.getBackend("")
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

	// TODO: use req.Msg.GetProjectId() once config.proto UpdateConstitutionRequest has project_id
	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := backend.SaveConstitution(req.Msg.Content); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Reload to get the path
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
	// TODO: use req.Msg.GetProjectId() once config.proto DeleteConstitutionRequest has project_id
	backend, err := s.getBackend("")
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
	stats := &orcv1.ConfigStats{}

	// Count skills (slash commands)
	homeDir, _ := os.UserHomeDir()
	globalClaudeDir := filepath.Join(homeDir, ".claude")
	projectClaudeDir := filepath.Join(s.workDir, ".claude")

	globalSkills, _ := claudeconfig.DiscoverSkills(globalClaudeDir)
	projectSkills, _ := claudeconfig.DiscoverSkills(projectClaudeDir)
	globalCommands := discoverCommands(globalClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL)
	projectCommands := discoverCommands(projectClaudeDir, orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT)
	stats.SlashCommandsCount = int32(len(globalSkills) + len(projectSkills) + len(globalCommands) + len(projectCommands))

	// Get CLAUDE.md size (sum of global + project)
	var claudeMdSize int64
	if info, err := os.Stat(filepath.Join(homeDir, "CLAUDE.md")); err == nil {
		claudeMdSize += info.Size()
	}
	if info, err := os.Stat(filepath.Join(s.workDir, "CLAUDE.md")); err == nil {
		claudeMdSize += info.Size()
	}
	stats.ClaudeMdSize = claudeMdSize

	// Count MCP servers from ~/.claude.json and .mcp.json
	mcpCount, _ := claudeconfig.CountMCPServers(s.workDir)
	stats.McpServersCount = int32(mcpCount)

	// Get permissions profile
	settings, _ := claudeconfig.LoadSettings(s.workDir)
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
	svc := prompt.NewService(filepath.Join(s.workDir, ".orc"))
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
	svc := prompt.NewService(filepath.Join(s.workDir, ".orc"))
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
	svc := prompt.NewService(filepath.Join(s.workDir, ".orc"))
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

	svc := prompt.NewService(filepath.Join(s.workDir, ".orc"))
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
	svc := prompt.NewService(filepath.Join(s.workDir, ".orc"))

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
	// GetVariableReference returns map[string]string (name -> description)
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

// ListAgents returns agents with runtime statistics and status.
// When no scope is specified, returns agents from both project (SQLite) and global sources.
// When scope is PROJECT, returns only SQLite agents.
// When scope is GLOBAL, returns only global agents from .claude/agents/ directory.
func (s *configServer) ListAgents(
	ctx context.Context,
	req *connect.Request[orcv1.ListAgentsRequest],
) (*connect.Response[orcv1.ListAgentsResponse], error) {
	// TODO: use req.Msg.GetProjectId() once config.proto ListAgentsRequest has project_id
	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	// Determine scope
	var scope orcv1.SettingsScope
	if req.Msg.Scope != nil {
		scope = *req.Msg.Scope
	}

	var protoAgents []*orcv1.Agent

	// Get stats (keyed by model) for all agents
	today := time.Now().Truncate(24 * time.Hour)
	stats, err := pdb.GetAgentStats(today)
	if err != nil {
		// Log but don't fail - stats are optional (graceful degradation)
		if s.logger != nil {
			s.logger.Warn("failed to get agent stats", "error", err)
		}
		stats = make(map[string]*db.AgentStats)
	}

	// Handle scope-based filtering
	switch scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		// Return only global agents (from .claude/agents/ directory)
		// For now, we don't have global agent discovery, return empty
		// Future: use claudeconfig.DiscoverAgents() when available

	case orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT:
		// Return only project agents from SQLite
		dbAgents, err := pdb.ListAgents()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list agents: %w", err))
		}
		protoAgents = make([]*orcv1.Agent, len(dbAgents))
		for i, a := range dbAgents {
			protoAgents[i] = dbAgentToProto(a, stats[a.Model], orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT)
		}

	default:
		// No scope specified - return all agents (project + global)
		// First, get project agents from SQLite
		dbAgents, err := pdb.ListAgents()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list agents: %w", err))
		}
		for _, a := range dbAgents {
			protoAgents = append(protoAgents, dbAgentToProto(a, stats[a.Model], orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT))
		}
		// Future: also append global agents from .claude/agents/ when available
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

	// TODO: use req.Msg.GetProjectId() once config.proto GetAgentRequest has project_id
	backend, err := s.getBackend("")
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

	// TODO: use req.Msg.GetProjectId() once config.proto CreateAgentRequest has project_id
	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	// Check if agent already exists
	existing, err := pdb.GetAgent(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("check existing agent: %w", err))
	}
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("agent %s already exists", req.Msg.Name))
	}

	// Build agent from request
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
	if req.Msg.ClaudeConfig != nil {
		agent.ClaudeConfig = *req.Msg.ClaudeConfig
	}
	if req.Msg.Model != nil {
		agent.Model = *req.Msg.Model
	}
	if req.Msg.Tools != nil && len(req.Msg.Tools.Allow) > 0 {
		agent.Tools = req.Msg.Tools.Allow
	}

	// Save to database
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

	// TODO: use req.Msg.GetProjectId() once config.proto UpdateAgentRequest has project_id
	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	// Get existing agent
	agent, err := pdb.GetAgent(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get agent: %w", err))
	}
	if agent == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %s not found", req.Msg.GetId()))
	}

	// Cannot modify built-in agents
	if agent.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in agent"))
	}

	// Apply updates
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
	if req.Msg.ClaudeConfig != nil {
		agent.ClaudeConfig = *req.Msg.ClaudeConfig
	}
	if req.Msg.Model != nil {
		agent.Model = *req.Msg.Model
	}
	if req.Msg.Tools != nil {
		agent.Tools = req.Msg.Tools.Allow
	}

	// Save updates
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

	// TODO: use req.Msg.GetProjectId() once config.proto DeleteAgentRequest has project_id
	backend, err := s.getBackend("")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pdb := backend.DB()

	// Get agent to check if it exists and is not built-in
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

	// Delete agent
	if err := pdb.DeleteAgent(req.Msg.Name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete agent: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteAgentResponse{
		Message: fmt.Sprintf("Agent %s deleted successfully", req.Msg.Name),
	}), nil
}

// dbAgentToProto converts a db.Agent to proto Agent with stats and status.
func dbAgentToProto(a *db.Agent, stats *db.AgentStats, scope orcv1.SettingsScope) *orcv1.Agent {
	agent := &orcv1.Agent{
		Name:        a.Name,
		Description: a.Description,
		Scope:       scope,
	}

	// Set model if present
	if a.Model != "" {
		agent.Model = &a.Model
	}

	// Set prompt if present
	if a.Prompt != "" {
		agent.Prompt = &a.Prompt
	}

	// Set tools if present
	if len(a.Tools) > 0 {
		agent.Tools = &orcv1.ToolPermissions{
			Allow: a.Tools,
		}
	}

	// Set status - "active" if running tasks exist for this model, else "idle"
	status := "idle"
	if stats != nil && stats.IsActive {
		status = "active"
	}
	agent.Status = &status

	// Set stats
	if stats != nil {
		agent.Stats = &orcv1.AgentStats{
			TokensToday: int64(stats.TokensToday),
			TasksDone:   int32(stats.TasksDoneTotal),
			SuccessRate: stats.SuccessRate,
		}
	} else {
		// Return zero stats if no stats available
		agent.Stats = &orcv1.AgentStats{
			TokensToday: 0,
			TasksDone:   0,
			SuccessRate: 0,
		}
	}

	return agent
}

// discoverCommands reads .claude/commands/ for flat .md files and returns them as proto Skills.
// Non-.md files and subdirectories are ignored.
func discoverCommands(claudeDir string, scope orcv1.SettingsScope) []*orcv1.Skill {
	commandsDir := filepath.Join(claudeDir, "commands")
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		return nil
	}

	var commands []*orcv1.Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".md")]
		content, err := os.ReadFile(filepath.Join(commandsDir, entry.Name()))
		if err != nil {
			continue
		}
		commands = append(commands, &orcv1.Skill{
			Name:    name,
			Content: string(content),
			Scope:   scope,
		})
	}
	return commands
}

// === Conversion helpers ===

func orcConfigToProto(cfg *config.Config) *orcv1.Config {
	parallelTasks := cfg.Execution.ParallelTasks
	if parallelTasks == 0 {
		parallelTasks = 2 // Default
	}
	costLimit := cfg.Execution.CostLimit
	if costLimit == 0 && cfg.Execution.ParallelTasks == 0 {
		costLimit = 25 // Default when no execution config exists
	}

	result := &orcv1.Config{
		Automation: &orcv1.AutomationConfig{
			Profile:     string(cfg.Profile),
			AutoApprove: cfg.Automation.AutoApprove,
		},
		Completion: &orcv1.CompletionConfig{
			Action:       cfg.Completion.Action,
			AutoMerge:    cfg.Completion.MergeOnCIPass,
			DeleteBranch: cfg.Completion.DeleteBranch,
			Pr: &orcv1.PRConfig{
				Draft:               cfg.Completion.PR.Draft,
				Labels:              cfg.Completion.PR.Labels,
				Reviewers:           cfg.Completion.PR.Reviewers,
				TeamReviewers:       cfg.Completion.PR.TeamReviewers,
				Assignees:           cfg.Completion.PR.Assignees,
				MaintainerCanModify: cfg.Completion.PR.MaintainerCanModify,
				AutoApprove:         cfg.Completion.PR.AutoApprove,
				AutoMerge:           cfg.Completion.PR.AutoMerge,
			},
			Ci: &orcv1.CIConfig{
				WaitForCi:            cfg.Completion.CI.WaitForCI,
				CiTimeout:            int32(cfg.Completion.CI.CITimeout / time.Minute),
				PollInterval:         int32(cfg.Completion.CI.PollInterval / time.Second),
				MergeOnCiPass:        cfg.Completion.CI.MergeOnCIPass,
				MergeMethod:          cfg.Completion.CI.MergeMethod,
				MergeCommitTemplate:  cfg.Completion.CI.MergeCommitTemplate,
				SquashCommitTemplate: cfg.Completion.CI.SquashCommitTemplate,
				VerifyShaOnMerge:     cfg.Completion.CI.VerifySHAOnMerge,
			},
		},
		Claude: &orcv1.ClaudeConfig{
			Model: cfg.Model,
		},
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: int32(parallelTasks),
			CostLimit:     int32(costLimit),
		},
	}
	if cfg.Completion.TargetBranch != "" {
		result.Completion.TargetBranch = &cfg.Completion.TargetBranch
	}

	// Jira config
	jiraCfg := &orcv1.JiraConfig{
		Url:               cfg.Jira.URL,
		Email:             cfg.Jira.Email,
		TokenEnvVar:       cfg.Jira.GetTokenEnvVar(),
		DefaultWeight:     cfg.Jira.DefaultWeight,
		DefaultQueue:      cfg.Jira.DefaultQueue,
		CustomFields:      cfg.Jira.CustomFields,
		DefaultProjects:   cfg.Jira.DefaultProjects,
		StatusOverrides:   cfg.Jira.StatusOverrides,
		CategoryOverrides: cfg.Jira.CategoryOverrides,
		PriorityOverrides: cfg.Jira.PriorityOverrides,
	}
	if cfg.Jira.EpicToInitiative != nil {
		jiraCfg.EpicToInitiative = cfg.Jira.EpicToInitiative
	}
	result.Jira = jiraCfg

	return result
}

func claudeSettingsToProto(s *claudeconfig.Settings) *orcv1.Settings {
	if s == nil {
		return &orcv1.Settings{}
	}

	result := &orcv1.Settings{
		Permissions: make(map[string]bool),
	}

	if s.Permissions != nil {
		for _, tool := range s.Permissions.Allow {
			result.Permissions[tool] = true
		}
		for _, tool := range s.Permissions.Deny {
			result.Permissions[tool] = false
		}
	}

	return result
}

func protoToClaudeSettings(s *orcv1.Settings) *claudeconfig.Settings {
	if s == nil {
		return &claudeconfig.Settings{}
	}

	result := &claudeconfig.Settings{}

	if len(s.Permissions) > 0 {
		result.Permissions = &claudeconfig.ToolPermissions{}
		for tool, allowed := range s.Permissions {
			if allowed {
				result.Permissions.Allow = append(result.Permissions.Allow, tool)
			} else {
				result.Permissions.Deny = append(result.Permissions.Deny, tool)
			}
		}
	}

	return result
}

func stringToProtoHookEvent(event string) orcv1.HookEvent {
	switch event {
	case "PreToolUse":
		return orcv1.HookEvent_HOOK_EVENT_PRE_TOOL_USE
	case "PostToolUse":
		return orcv1.HookEvent_HOOK_EVENT_POST_TOOL_USE
	case "Notification":
		return orcv1.HookEvent_HOOK_EVENT_NOTIFICATION
	case "Stop":
		return orcv1.HookEvent_HOOK_EVENT_STOP
	default:
		return orcv1.HookEvent_HOOK_EVENT_UNSPECIFIED
	}
}

func protoHookEventToString(event orcv1.HookEvent) string {
	switch event {
	case orcv1.HookEvent_HOOK_EVENT_PRE_TOOL_USE:
		return "PreToolUse"
	case orcv1.HookEvent_HOOK_EVENT_POST_TOOL_USE:
		return "PostToolUse"
	case orcv1.HookEvent_HOOK_EVENT_NOTIFICATION:
		return "Notification"
	case orcv1.HookEvent_HOOK_EVENT_STOP:
		return "Stop"
	default:
		return ""
	}
}

func claudeSkillToProto(s *claudeconfig.Skill, scope orcv1.SettingsScope) *orcv1.Skill {
	return &orcv1.Skill{
		Name:        s.Name,
		Description: s.Description,
		Content:     s.Content,
		// Note: UserInvocable is a proto field but claudeconfig.Skill doesn't track it
		// It's determined by skill naming convention or config
		Scope: scope,
	}
}

// promptInfoToProto converts a PromptInfo to proto (used for List).
// PromptInfo only has Phase, Source, HasOverride, Variables - no content.
func promptInfoToProto(p *prompt.PromptInfo) *orcv1.PromptTemplate {
	return &orcv1.PromptTemplate{
		Phase:    p.Phase,
		IsCustom: p.HasOverride,
		// Note: PromptInfo doesn't include Content - use promptToProto for full content
	}
}

// promptToProto converts a Prompt to proto (used for Get/GetDefault).
// Prompt includes Content.
func promptToProto(p *prompt.Prompt) *orcv1.PromptTemplate {
	return &orcv1.PromptTemplate{
		Phase:    p.Phase,
		Content:  p.Content,
		IsCustom: p.Source != prompt.SourceEmbedded,
	}
}
