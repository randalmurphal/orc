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

	"github.com/randalmurphal/llmkit/v2/claudeconfig"
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
	globalDB     *db.GlobalDB
	workDir      string
	logger       *slog.Logger
	testHomeDir  string // For test isolation of GLOBAL destination
}

// SetGlobalDB sets the GlobalDB dependency for hook/skill CRUD operations.
func (s *configServer) SetGlobalDB(gdb *db.GlobalDB) {
	s.globalDB = gdb
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

// getWorkDir returns the work directory for the given project ID.
// Uses projectCache to resolve project-specific paths, falls back to default workDir.
func (s *configServer) getWorkDir(projectID string) (string, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetProjectPath(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return "", fmt.Errorf("project_id specified but no project cache configured")
	}
	return s.workDir, nil
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
// Short names (sonnet, opus, haiku) are preferred and resolved by Claude Code.
var ValidModels = []string{
	"sonnet",
	"opus",
	"haiku",
}

// UpdateConfig updates the ORC configuration and persists to config.yaml.
func (s *configServer) UpdateConfig(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateConfigRequest],
) (*connect.Response[orcv1.UpdateConfigResponse], error) {
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	configPath := filepath.Join(workDir, config.OrcDir, config.ConfigFileName)

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
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var settings *claudeconfig.Settings

	switch req.Msg.Scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		settings, err = claudeconfig.LoadGlobalSettings()
	case orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT:
		settings, err = claudeconfig.LoadProjectSettings(workDir)
	default:
		// Merged (default)
		settings, err = claudeconfig.LoadSettings(workDir)
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
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	settings := protoToClaudeSettings(req.Msg.Settings)

	switch req.Msg.Scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		err = claudeconfig.SaveGlobalSettings(settings)
	default:
		err = claudeconfig.SaveProjectSettings(workDir, settings)
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
	workDir, err := s.getWorkDir(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	globalSettings, _ := claudeconfig.LoadGlobalSettings()
	projectSettings, _ := claudeconfig.LoadProjectSettings(workDir)
	mergedSettings, _ := claudeconfig.LoadSettings(workDir)

	return connect.NewResponse(&orcv1.GetSettingsHierarchyResponse{
		Hierarchy: &orcv1.SettingsHierarchy{
			Global:  claudeSettingsToProto(globalSettings),
			Project: claudeSettingsToProto(projectSettings),
			Merged:  claudeSettingsToProto(mergedSettings),
		},
	}), nil
}

// dbAgentToProto converts a db.Agent to proto Agent with stats and status.
func dbAgentToProto(a *db.Agent, stats *db.AgentStats, scope orcv1.SettingsScope) *orcv1.Agent {
	agent := &orcv1.Agent{
		Id:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Scope:       scope,
		IsBuiltin:   a.IsBuiltin,
	}

	// Set model if present
	if a.Model != "" {
		agent.Model = &a.Model
	}

	// Set provider if present
	if a.Provider != "" {
		agent.Provider = &a.Provider
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
		Claude: &orcv1.RuntimeConfig{
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

// hookScriptToProto converts a db.HookScript to proto Hook.
func hookScriptToProto(hs *db.HookScript) *orcv1.Hook {
	return &orcv1.Hook{
		Id:          hs.ID,
		Name:        hs.Name,
		Description: hs.Description,
		Content:     hs.Content,
		EventType:   hs.EventType,
		IsBuiltin:   hs.IsBuiltin,
	}
}

// dbSkillToProto converts a db.Skill to proto Skill.
func dbSkillToProto(s *db.Skill) *orcv1.Skill {
	return &orcv1.Skill{
		Id:              s.ID,
		Name:            s.Name,
		Description:     s.Description,
		Content:         s.Content,
		IsBuiltin:       s.IsBuiltin,
		SupportingFiles: s.SupportingFiles,
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

// loadConfigForProject loads configuration for a specific project, or returns server config if projectID is empty.
func (s *configServer) loadConfigForProject(projectID string) (*config.Config, error) {
	if projectID == "" {
		// Return the config that was provided to the server (for tests and current behavior)
		return s.orcConfig, nil
	}

	// For project-specific config, we'd need to implement project-specific config loading
	// For now, return the current config as a fallback
	return s.orcConfig, nil
}

// getConfigPath returns the config file path for a project, using the server's work directory.
func (s *configServer) getConfigPath(projectID string) string {
	if s.workDir != "" {
		return filepath.Join(s.workDir, config.OrcDir, config.ConfigFileName)
	}

	// For project-specific configs, we'd construct the path differently
	// For now, return the default path
	return filepath.Join(config.OrcDir, config.ConfigFileName)
}
