package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// PluginResponse wraps a plugin with metadata about the operation.
type PluginResponse struct {
	Plugin         *claudeconfig.Plugin `json:"plugin"`
	RequiresRestart bool                `json:"requires_restart"`
	Message        string              `json:"message,omitempty"`
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
		// No plugins is OK - return empty list
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
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
	}

	plugin, err := svc.Get(name, pluginScope)
	if err != nil {
		s.jsonError(w, "plugin not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, plugin)
}

// handleEnablePlugin enables a plugin in settings.json.
// Supports ?scope=global|project to specify which settings file to update.
func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
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
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
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
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
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
	scope := r.URL.Query().Get("scope")

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
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
	// Parse pagination params
	page := 1
	limit := 20
	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
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
	scope := r.URL.Query().Get("scope")

	// Parse optional version from body
	var req struct {
		Version string `json:"version"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
		return
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
	}

	plugin, err := marketplaceSvc.Install(name, req.Version, pluginScope, s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to install plugin: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, PluginResponse{
		Plugin:          plugin,
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

	// Get all installed plugins
	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	// Get plugins from both scopes
	globalPlugins, _ := claudeconfig.DiscoverPlugins(filepath.Join(homeDir, ".claude"))
	projectPlugins, _ := claudeconfig.DiscoverPlugins(filepath.Join(s.getProjectRoot(), ".claude"))

	// Combine
	var allPlugins []*claudeconfig.Plugin
	allPlugins = append(allPlugins, globalPlugins...)
	allPlugins = append(allPlugins, projectPlugins...)

	// Check for updates
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

	// Silence unused variable warning
	_ = svc

	s.jsonResponse(w, updates)
}

// handleUpdatePlugin updates a specific plugin to the latest version.
func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	// For now, update is uninstall + reinstall
	// This is a simplified implementation

	svc, err := claudeconfig.NewPluginService(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create plugin service: %v", err), http.StatusInternalServerError)
		return
	}

	pluginScope := claudeconfig.PluginScopeProject
	if scope == "global" {
		pluginScope = claudeconfig.PluginScopeGlobal
	}

	// Check if plugin exists
	plugin, err := svc.Get(name, pluginScope)
	if err != nil {
		s.jsonError(w, "plugin not found", http.StatusNotFound)
		return
	}

	// Uninstall
	if err := svc.Uninstall(name, pluginScope); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to uninstall for update: %v", err), http.StatusInternalServerError)
		return
	}

	// Reinstall from marketplace
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")
	marketplaceSvc := claudeconfig.NewMarketplaceService(claudeDir)

	newPlugin, err := marketplaceSvc.Install(name, "", pluginScope, s.getProjectRoot())
	if err != nil {
		// Try to restore - this is best effort
		s.jsonError(w, fmt.Sprintf("failed to reinstall plugin: %v (old version removed)", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, PluginResponse{
		Plugin:          newPlugin,
		RequiresRestart: true,
		Message:         fmt.Sprintf("Plugin updated from %s to %s. Restart Claude Code to apply.", plugin.Version, newPlugin.Version),
	})
}
