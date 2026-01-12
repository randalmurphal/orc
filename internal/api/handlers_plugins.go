package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// PluginDetail is a JSON-friendly version of Plugin with all fields exposed.
// The underlying claudeconfig.Plugin has json:"-" on metadata fields,
// so we need this wrapper to return full plugin details via API.
type PluginDetail struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Author      *claudeconfig.PluginAuthor `json:"author,omitempty"`
	Homepage    string                  `json:"homepage,omitempty"`
	Keywords    []string                `json:"keywords,omitempty"`
	Path        string                  `json:"path"`
	Scope       claudeconfig.PluginScope `json:"scope"`
	Enabled     bool                    `json:"enabled"`
	Version     string                  `json:"version,omitempty"`
	InstalledAt string                  `json:"installed_at,omitempty"`
	UpdatedAt   string                  `json:"updated_at,omitempty"`
	HasCommands bool                    `json:"has_commands"`
	HasHooks    bool                    `json:"has_hooks"`
	HasScripts  bool                    `json:"has_scripts"`
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

// === Marketplace ===

// MarketplaceBrowseResponse contains paginated marketplace results.
type MarketplaceBrowseResponse struct {
	Plugins      []claudeconfig.MarketplacePlugin `json:"plugins"`
	Total        int                              `json:"total"`
	Page         int                              `json:"page"`
	Limit        int                              `json:"limit"`
	Cached       bool                             `json:"cached"`
	CacheAgeSecs int                              `json:"cache_age_seconds,omitempty"`
}

// handleBrowseMarketplace returns available plugins from the marketplace.
func (s *Server) handleBrowseMarketplace(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params with defaults
	page := 1
	limit := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
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
		s.jsonError(w, fmt.Sprintf("marketplace unavailable: %v", err), http.StatusServiceUnavailable)
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
		s.jsonError(w, fmt.Sprintf("search failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	if plugins == nil {
		plugins = []claudeconfig.MarketplacePlugin{}
	}

	s.jsonResponse(w, plugins)
}

// handleGetMarketplacePlugin returns details for a specific marketplace plugin.
func (s *Server) handleGetMarketplacePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	plugin, err := marketplaceSvc.GetPlugin(name)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("plugin not found: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, plugin)
}

// handleInstallPlugin installs a plugin from the marketplace.
func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pluginScope := parsePluginScope(r)

	// Parse optional version from body (ignore decode errors - version is optional)
	var req struct {
		Version string `json:"version"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.logger.Debug("install plugin body decode", "error", err)
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
