package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// validPluginNameRe validates plugin names: alphanumeric, hyphens, underscores only.
// Must start with a letter or number.
var validPluginNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// PluginDetail is a JSON-friendly version of Plugin with all fields exposed.
// The underlying claudeconfig.Plugin has json:"-" on metadata fields,
// so we need this wrapper to return full plugin details via API.
type PluginDetail struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Author      *claudeconfig.PluginAuthor `json:"author,omitempty"`
	Homepage    string                     `json:"homepage,omitempty"`
	Keywords    []string                   `json:"keywords,omitempty"`
	Path        string                     `json:"path"`
	Scope       claudeconfig.PluginScope   `json:"scope"`
	Enabled     bool                       `json:"enabled"`
	Version     string                     `json:"version,omitempty"`
	InstalledAt string                     `json:"installed_at,omitempty"`
	UpdatedAt   string                     `json:"updated_at,omitempty"`
	HasCommands bool                       `json:"has_commands"`
	HasHooks    bool                       `json:"has_hooks"`
	HasScripts  bool                       `json:"has_scripts"`

	// Discovered resources
	Commands   []claudeconfig.PluginCommand   `json:"commands,omitempty"`
	MCPServers []claudeconfig.PluginMCPServer `json:"mcp_servers,omitempty"`
	Hooks      []claudeconfig.PluginHook      `json:"hooks,omitempty"`
}

// pluginToDetail converts a Plugin to a PluginDetail for JSON serialization.
func pluginToDetail(p *claudeconfig.Plugin) PluginDetail {
	d := PluginDetail{
		Name:        p.Name,
		Description: p.Description,
		Homepage:    p.Homepage,
		Keywords:    p.Keywords,
		Path:        p.Path,
		Scope:       p.Scope,
		Enabled:     p.Enabled,
		Version:     p.Version,
		HasCommands: p.HasCommands,
		HasHooks:    p.HasHooks,
		HasScripts:  p.HasScripts,
		Commands:    p.Commands,
		MCPServers:  p.MCPServers,
		Hooks:       p.Hooks,
	}
	if p.Author.Name != "" {
		d.Author = &p.Author
	}
	if !p.InstalledAt.IsZero() {
		d.InstalledAt = p.InstalledAt.Format("2006-01-02T15:04:05Z")
	}
	if !p.UpdatedAt.IsZero() {
		d.UpdatedAt = p.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}
	return d
}

// PluginResponse wraps a plugin with metadata about the operation.
type PluginResponse struct {
	Plugin          *PluginDetail `json:"plugin,omitempty"`
	RequiresRestart bool          `json:"requires_restart"`
	Message         string        `json:"message,omitempty"`
}

// parsePluginScope parses the scope query parameter, defaulting to project.
func parsePluginScope(r *http.Request) claudeconfig.PluginScope {
	if r.URL.Query().Get("scope") == "global" {
		return claudeconfig.PluginScopeGlobal
	}
	return claudeconfig.PluginScopeProject
}

// === Local Plugin Management ===

// handleListPlugins returns all discovered plugins.
// Supports ?scope=global|project to filter by scope.
// Without scope parameter, returns merged list (project overrides global).
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	var infos []claudeconfig.PluginInfo

	switch scope {
	case "global":
		infos, err = svc.ListByScope(claudeconfig.PluginScopeGlobal)
	case "project":
		infos, err = svc.ListByScope(claudeconfig.PluginScopeProject)
	default:
		// Merged list
		infos, err = svc.List()
	}

	if err != nil {
		// Plugin discovery errors are typically "directory not found" which means no plugins
		// Log for debugging but return empty list (not an error for the client)
		s.logger.Debug("plugin discovery", "error", err)
		s.jsonResponse(w, []claudeconfig.PluginInfo{})
		return
	}

	if infos == nil {
		infos = []claudeconfig.PluginInfo{}
	}

	s.jsonResponse(w, infos)
}

// handleGetPlugin returns a specific plugin by name.
// Supports ?scope=global|project to specify which scope to look in.
func (s *Server) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	plugin, err := svc.Get(name, pluginScope)
	if err != nil {
		s.jsonError(w, "plugin not found", http.StatusNotFound)
		return
	}

	// Return PluginDetail which includes all fields (Plugin has json:"-" on metadata)
	detail := pluginToDetail(plugin)
	s.jsonResponse(w, detail)
}

// handleEnablePlugin enables a plugin in settings.json.
// Supports ?scope=global|project to specify which settings file to update.
func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := svc.Enable(name, pluginScope); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to enable plugin: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, PluginResponse{
		RequiresRestart: true,
		Message:         "Plugin enabled. Restart Claude Code to apply changes.",
	})
}

// handleDisablePlugin disables a plugin in settings.json.
// Supports ?scope=global|project to specify which settings file to update.
func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := svc.Disable(name, pluginScope); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to disable plugin: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, PluginResponse{
		RequiresRestart: true,
		Message:         "Plugin disabled. Restart Claude Code to apply changes.",
	})
}

// handleUninstallPlugin removes a plugin directory.
// Supports ?scope=global|project to specify which scope to remove from.
func (s *Server) handleUninstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	if err := svc.Uninstall(name, pluginScope); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to uninstall plugin: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, PluginResponse{
		RequiresRestart: true,
		Message:         "Plugin uninstalled. Restart Claude Code to apply changes.",
	})
}

// handleListPluginCommands returns the commands for a specific plugin.
func (s *Server) handleListPluginCommands(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	commands, err := svc.ListCommands(name, pluginScope)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list commands: %v", err), http.StatusInternalServerError)
		return
	}

	if commands == nil {
		commands = []claudeconfig.PluginCommand{}
	}

	s.jsonResponse(w, commands)
}

// PluginResourcesResponse contains aggregated resources from all plugins.
type PluginResourcesResponse struct {
	MCPServers []PluginMCPServerWithSource `json:"mcp_servers"`
	Hooks      []PluginHookWithSource      `json:"hooks"`
	Commands   []PluginCommandWithSource   `json:"commands"`
}

// PluginMCPServerWithSource is an MCP server with plugin source info.
type PluginMCPServerWithSource struct {
	claudeconfig.PluginMCPServer
	PluginName  string                   `json:"plugin_name"`
	PluginScope claudeconfig.PluginScope `json:"plugin_scope"`
}

// PluginHookWithSource is a hook with plugin source info.
type PluginHookWithSource struct {
	claudeconfig.PluginHook
	PluginName  string                   `json:"plugin_name"`
	PluginScope claudeconfig.PluginScope `json:"plugin_scope"`
}

// PluginCommandWithSource is a command with plugin source info.
type PluginCommandWithSource struct {
	claudeconfig.PluginCommand
	PluginName  string                   `json:"plugin_name"`
	PluginScope claudeconfig.PluginScope `json:"plugin_scope"`
}

// handleListPluginResources returns aggregated resources from all plugins.
// This allows the MCP Servers and Hooks views to show plugin-provided resources.
func (s *Server) handleListPluginResources(w http.ResponseWriter, r *http.Request) {
	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	// Get plugins from both scopes (errors typically mean "directory not found" = no plugins)
	globalInfos, globalErr := svc.ListByScope(claudeconfig.PluginScopeGlobal)
	if globalErr != nil {
		s.logger.Debug("list global plugins for resources", "error", globalErr)
	}
	projectInfos, projectErr := svc.ListByScope(claudeconfig.PluginScopeProject)
	if projectErr != nil {
		s.logger.Debug("list project plugins for resources", "error", projectErr)
	}

	var response PluginResourcesResponse

	// Helper to process plugins from a scope
	processPlugins := func(infos []claudeconfig.PluginInfo, scope claudeconfig.PluginScope) {
		for _, info := range infos {
			plugin, err := svc.Get(info.Name, scope)
			if err != nil {
				continue
			}

			// Add MCP servers
			for _, server := range plugin.MCPServers {
				response.MCPServers = append(response.MCPServers, PluginMCPServerWithSource{
					PluginMCPServer: server,
					PluginName:      plugin.Name,
					PluginScope:     scope,
				})
			}

			// Add hooks
			for _, hook := range plugin.Hooks {
				response.Hooks = append(response.Hooks, PluginHookWithSource{
					PluginHook:  hook,
					PluginName:  plugin.Name,
					PluginScope: scope,
				})
			}

			// Add commands
			for _, cmd := range plugin.Commands {
				response.Commands = append(response.Commands, PluginCommandWithSource{
					PluginCommand: cmd,
					PluginName:    plugin.Name,
					PluginScope:   scope,
				})
			}
		}
	}

	processPlugins(globalInfos, claudeconfig.PluginScopeGlobal)
	processPlugins(projectInfos, claudeconfig.PluginScopeProject)

	// Ensure non-nil slices for JSON
	if response.MCPServers == nil {
		response.MCPServers = []PluginMCPServerWithSource{}
	}
	if response.Hooks == nil {
		response.Hooks = []PluginHookWithSource{}
	}
	if response.Commands == nil {
		response.Commands = []PluginCommandWithSource{}
	}

	s.jsonResponse(w, response)
}

// === Marketplace ===

// MarketplaceBrowseResponse contains paginated marketplace results.
type MarketplaceBrowseResponse struct {
	Plugins      []claudeconfig.MarketplacePlugin `json:"plugins"`
	Total        int                              `json:"total"`
	Page         int                              `json:"page"`
	Limit        int                              `json:"limit"`
	Cached       bool                             `json:"cached"`
	CacheAgeSecs int                              `json:"cache_age_seconds,omitempty"`
	IsMock       bool                             `json:"is_mock,omitempty"`
	Message      string                           `json:"message,omitempty"`
}

// sampleMarketplacePlugins returns sample plugins for when the marketplace is unavailable.
// This allows the UI to demonstrate functionality and provides useful plugin examples.
func sampleMarketplacePlugins() []claudeconfig.MarketplacePlugin {
	return []claudeconfig.MarketplacePlugin{
		{
			Name:        "orc",
			Description: "Task orchestration plugin for Claude Code with phased execution and git worktree isolation",
			Author:      claudeconfig.PluginAuthor{Name: "Randal Murphy", URL: "https://github.com/randalmurphal"},
			Version:     "1.0.0",
			Repository:  "https://github.com/randalmurphal/orc-claude-plugin",
			Downloads:   1250,
			Keywords:    []string{"orchestration", "tasks", "git", "worktree", "automation"},
		},
		{
			Name:        "memory",
			Description: "Persistent memory and context management for Claude Code sessions",
			Author:      claudeconfig.PluginAuthor{Name: "Claude Community"},
			Version:     "0.2.1",
			Repository:  "https://github.com/anthropics/claude-code-memory",
			Downloads:   3420,
			Keywords:    []string{"memory", "context", "persistence", "session"},
		},
		{
			Name:        "git-workflow",
			Description: "Enhanced git workflow commands including interactive rebase, cherry-pick, and conflict resolution",
			Author:      claudeconfig.PluginAuthor{Name: "DevTools Team"},
			Version:     "1.2.0",
			Repository:  "https://github.com/devtools/git-workflow-plugin",
			Downloads:   2180,
			Keywords:    []string{"git", "workflow", "rebase", "merge", "conflicts"},
		},
		{
			Name:        "test-runner",
			Description: "Intelligent test runner with coverage tracking, watch mode, and failure analysis",
			Author:      claudeconfig.PluginAuthor{Name: "Testing Guild"},
			Version:     "0.5.0",
			Repository:  "https://github.com/testing-guild/test-runner-plugin",
			Downloads:   1890,
			Keywords:    []string{"testing", "coverage", "tdd", "jest", "pytest"},
		},
		{
			Name:        "code-review",
			Description: "Automated code review with style checking, security scanning, and best practice suggestions",
			Author:      claudeconfig.PluginAuthor{Name: "Quality Assurance"},
			Version:     "0.8.2",
			Repository:  "https://github.com/qa-tools/code-review-plugin",
			Downloads:   2540,
			Keywords:    []string{"review", "lint", "security", "quality", "static-analysis"},
		},
		{
			Name:        "docs-generator",
			Description: "Generate and maintain documentation from code comments and structure",
			Author:      claudeconfig.PluginAuthor{Name: "DocuMentor"},
			Version:     "0.3.0",
			Repository:  "https://github.com/documenter/docs-generator",
			Downloads:   980,
			Keywords:    []string{"documentation", "readme", "api-docs", "markdown"},
		},
	}
}

// handleBrowseMarketplace returns available plugins from the marketplace.
// Falls back to sample plugins when the marketplace is unavailable.
func (s *Server) handleBrowseMarketplace(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params with defaults and bounds
	page := 1
	limit := 20
	const maxPage = 10000 // Prevent integer overflow
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= maxPage {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	plugins, total, err := marketplaceSvc.Browse(page, limit)
	if err != nil {
		// Fallback to sample plugins when marketplace is unavailable
		s.logger.Debug("marketplace unavailable, using sample plugins", "error", err)
		samplePlugins := sampleMarketplacePlugins()

		// Apply pagination to sample plugins
		start := (page - 1) * limit
		end := start + limit
		if start >= len(samplePlugins) {
			start = 0
			end = 0
		}
		if end > len(samplePlugins) {
			end = len(samplePlugins)
		}

		var paginatedSample []claudeconfig.MarketplacePlugin
		if start < len(samplePlugins) {
			paginatedSample = samplePlugins[start:end]
		} else {
			paginatedSample = []claudeconfig.MarketplacePlugin{}
		}

		s.jsonResponse(w, MarketplaceBrowseResponse{
			Plugins: paginatedSample,
			Total:   len(samplePlugins),
			Page:    page,
			Limit:   limit,
			IsMock:  true,
			Message: "Showing sample plugins. The official Claude Code plugin marketplace is not yet available. Install plugins manually via 'claude plugin add <github-repo>'.",
		})
		return
	}

	cacheAge := int(marketplaceSvc.CacheAge().Seconds())

	s.jsonResponse(w, MarketplaceBrowseResponse{
		Plugins:      plugins,
		Total:        total,
		Page:         page,
		Limit:        limit,
		Cached:       marketplaceSvc.IsCacheValid(),
		CacheAgeSecs: cacheAge,
	})
}

// handleSearchMarketplace searches for plugins in the marketplace.
// Falls back to searching sample plugins when the marketplace is unavailable.
func (s *Server) handleSearchMarketplace(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.jsonError(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	plugins, err := marketplaceSvc.Search(query)
	if err != nil {
		// Fallback to searching sample plugins
		s.logger.Debug("marketplace search unavailable, searching sample plugins", "error", err)
		samplePlugins := sampleMarketplacePlugins()
		queryLower := strings.ToLower(query)

		var results []claudeconfig.MarketplacePlugin
		for _, p := range samplePlugins {
			if strings.Contains(strings.ToLower(p.Name), queryLower) ||
				strings.Contains(strings.ToLower(p.Description), queryLower) {
				results = append(results, p)
				continue
			}
			for _, kw := range p.Keywords {
				if strings.Contains(strings.ToLower(kw), queryLower) {
					results = append(results, p)
					break
				}
			}
		}

		if results == nil {
			results = []claudeconfig.MarketplacePlugin{}
		}
		s.jsonResponse(w, results)
		return
	}

	if plugins == nil {
		plugins = []claudeconfig.MarketplacePlugin{}
	}

	s.jsonResponse(w, plugins)
}

// handleGetMarketplacePlugin returns details for a specific marketplace plugin.
// Falls back to sample plugins when the marketplace is unavailable.
func (s *Server) handleGetMarketplacePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validPluginNameRe.MatchString(name) {
		s.jsonError(w, "invalid plugin name", http.StatusBadRequest)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	plugin, err := marketplaceSvc.GetPlugin(name)
	if err != nil {
		// Fallback to searching sample plugins
		s.logger.Debug("marketplace get plugin unavailable, searching sample plugins", "error", err)
		for _, p := range sampleMarketplacePlugins() {
			if p.Name == name {
				s.jsonResponse(w, p)
				return
			}
		}
		s.jsonError(w, "plugin not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, plugin)
}

// handleInstallPlugin installs a plugin from the marketplace.
func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validPluginNameRe.MatchString(name) {
		s.jsonError(w, "invalid plugin name", http.StatusBadRequest)
		return
	}
	pluginScope := parsePluginScope(r)

	// Parse optional version from body
	var req struct {
		Version string `json:"version"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Malformed JSON is a client error
			s.jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	plugin, err := marketplaceSvc.Install(name, req.Version, pluginScope, s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to install plugin: %v", err), http.StatusInternalServerError)
		return
	}

	detail := pluginToDetail(plugin)
	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, PluginResponse{
		Plugin:          &detail,
		RequiresRestart: true,
		Message:         "Plugin installed. Restart Claude Code to load.",
	})
}

// === Updates ===

// handleCheckPluginUpdates checks for available updates for installed plugins.
func (s *Server) handleCheckPluginUpdates(w http.ResponseWriter, r *http.Request) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	// Discover plugins from both scopes (errors mean no plugins in that scope)
	globalPlugins, globalErr := claudeconfig.DiscoverPlugins(filepath.Join(homeDir, ".claude"))
	projectPlugins, projectErr := claudeconfig.DiscoverPlugins(filepath.Join(s.getProjectRoot(), ".claude"))

	if globalErr != nil {
		s.logger.Debug("discover global plugins", "error", globalErr)
	}
	if projectErr != nil {
		s.logger.Debug("discover project plugins", "error", projectErr)
	}

	// Combine all plugins
	var allPlugins []*claudeconfig.Plugin
	allPlugins = append(allPlugins, globalPlugins...)
	allPlugins = append(allPlugins, projectPlugins...)

	// Check for updates via marketplace
	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	updates, err := marketplaceSvc.CheckUpdates(allPlugins)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to check updates: %v", err), http.StatusServiceUnavailable)
		return
	}

	if updates == nil {
		updates = []claudeconfig.PluginUpdateInfo{}
	}

	s.jsonResponse(w, updates)
}

// handleUpdatePlugin updates a specific plugin to the latest version.
func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	// Update is uninstall + reinstall from marketplace
	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if plugin exists and get current version
	plugin, err := svc.Get(name, pluginScope)
	if err != nil {
		s.jsonError(w, "plugin not found", http.StatusNotFound)
		return
	}
	oldVersion := plugin.Version

	// Uninstall current version
	if err := svc.Uninstall(name, pluginScope); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to uninstall for update: %v", err), http.StatusInternalServerError)
		return
	}

	// Reinstall from marketplace
	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}
	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	newPlugin, err := marketplaceSvc.Install(name, "", pluginScope, s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to reinstall plugin: %v (old version removed)", err), http.StatusInternalServerError)
		return
	}

	detail := pluginToDetail(newPlugin)
	s.jsonResponse(w, PluginResponse{
		Plugin:          &detail,
		RequiresRestart: true,
		Message:         fmt.Sprintf("Plugin updated from %s to %s. Restart Claude Code to apply.", oldVersion, newPlugin.Version),
	})
}
