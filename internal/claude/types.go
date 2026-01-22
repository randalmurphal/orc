// Package claude re-exports Claude Code configuration types from llmkit/claudeconfig.
// This package provides convenient access to Claude Code native configuration
// without requiring direct imports of llmkit throughout the orc codebase.
package claude

import (
	"github.com/randalmurphal/llmkit/claudeconfig"
)

// Re-export types from llmkit/claudeconfig
type (
	// Skill represents a Claude Code skill from SKILL.md format.
	Skill = claudeconfig.Skill
	// SkillInfo provides summary information for listing skills.
	SkillInfo = claudeconfig.SkillInfo

	// Settings represents Claude Code's settings.json structure.
	Settings = claudeconfig.Settings
	// Hook represents a hook entry in settings.json.
	Hook = claudeconfig.Hook
	// HookEntry represents a single hook action.
	HookEntry = claudeconfig.HookEntry
	// HookEvent represents valid hook event types.
	HookEvent = claudeconfig.HookEvent
	// ToolPermissions defines allow/deny lists for Claude Code tools.
	ToolPermissions = claudeconfig.ToolPermissions

	// ClaudeMD represents a CLAUDE.md file.
	ClaudeMD = claudeconfig.ClaudeMD
	// ClaudeMDHierarchy represents the CLAUDE.md inheritance chain.
	ClaudeMDHierarchy = claudeconfig.ClaudeMDHierarchy

	// SubAgent defines a reusable agent configuration.
	SubAgent = claudeconfig.SubAgent
	// ProjectScript defines a script available to agents.
	ProjectScript = claudeconfig.ProjectScript
	// ToolInfo provides information about a Claude Code tool.
	ToolInfo = claudeconfig.ToolInfo

	// Services
	AgentService  = claudeconfig.AgentService
	ScriptService = claudeconfig.ScriptService

	// Plugin types
	Plugin            = claudeconfig.Plugin
	PluginInfo        = claudeconfig.PluginInfo
	PluginAuthor      = claudeconfig.PluginAuthor
	PluginScope       = claudeconfig.PluginScope
	PluginCommand     = claudeconfig.PluginCommand
	PluginService     = claudeconfig.PluginService
	MarketplacePlugin = claudeconfig.MarketplacePlugin
	PluginUpdateInfo  = claudeconfig.PluginUpdateInfo
)

// Re-export hook event constants
const (
	HookPreToolUse  = claudeconfig.HookPreToolUse
	HookPostToolUse = claudeconfig.HookPostToolUse
	HookPreCompact  = claudeconfig.HookPreCompact
	HookPrePrompt   = claudeconfig.HookPrePrompt
	HookStop        = claudeconfig.HookStop
)

// Re-export plugin scope constants
const (
	PluginScopeGlobal  = claudeconfig.PluginScopeGlobal
	PluginScopeProject = claudeconfig.PluginScopeProject
)

// Re-export errors
var (
	ErrSubAgentNameRequired        = claudeconfig.ErrSubAgentNameRequired
	ErrSubAgentDescriptionRequired = claudeconfig.ErrSubAgentDescriptionRequired
	ErrSubAgentNotFound            = claudeconfig.ErrSubAgentNotFound
	ErrSubAgentAlreadyExists       = claudeconfig.ErrSubAgentAlreadyExists

	ErrScriptNameRequired        = claudeconfig.ErrScriptNameRequired
	ErrScriptPathRequired        = claudeconfig.ErrScriptPathRequired
	ErrScriptDescriptionRequired = claudeconfig.ErrScriptDescriptionRequired
	ErrScriptNotFound            = claudeconfig.ErrScriptNotFound
	ErrScriptAlreadyExists       = claudeconfig.ErrScriptAlreadyExists
)

// Re-export functions
var (
	// Skills
	ParseSkillMD       = claudeconfig.ParseSkillMD
	WriteSkillMD       = claudeconfig.WriteSkillMD
	DiscoverSkills     = claudeconfig.DiscoverSkills
	ListSkillResources = claudeconfig.ListSkillResources

	// Settings
	LoadSettings        = claudeconfig.LoadSettings
	LoadGlobalSettings  = claudeconfig.LoadGlobalSettings
	LoadProjectSettings = claudeconfig.LoadProjectSettings
	SaveProjectSettings = claudeconfig.SaveProjectSettings
	ValidHookEvents     = claudeconfig.ValidHookEvents

	// CLAUDE.md
	LoadClaudeMDHierarchy = claudeconfig.LoadClaudeMDHierarchy
	LoadProjectClaudeMD   = claudeconfig.LoadProjectClaudeMD
	SaveProjectClaudeMD   = claudeconfig.SaveProjectClaudeMD

	// Tools
	AvailableTools  = claudeconfig.AvailableTools
	ToolsByCategory = claudeconfig.ToolsByCategory
	GetTool         = claudeconfig.GetTool
	ToolCategories  = claudeconfig.ToolCategories

	// Service constructors
	NewAgentService  = claudeconfig.NewAgentService
	NewScriptService = claudeconfig.NewScriptService

	// Plugins
	ParsePluginJSON   = claudeconfig.ParsePluginJSON
	DiscoverPlugins   = claudeconfig.DiscoverPlugins
	NewPluginService  = claudeconfig.NewPluginService
	GlobalPluginsDir  = claudeconfig.GlobalPluginsDir
	ProjectPluginsDir = claudeconfig.ProjectPluginsDir
)
