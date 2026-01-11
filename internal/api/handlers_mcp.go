package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === MCP Server Handlers ===

// handleListMCPServers returns all MCP servers from .mcp.json.
func (s *Server) handleListMCPServers(w http.ResponseWriter, r *http.Request) {
	config, err := claudeconfig.LoadProjectMCPConfig(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	// Return list with server info
	infos := config.ListServerInfos()
	if infos == nil {
		infos = []*claudeconfig.MCPServerInfo{}
	}

	s.jsonResponse(w, infos)
}

// handleGetMCPServer returns a specific MCP server by name.
func (s *Server) handleGetMCPServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	config, err := claudeconfig.LoadProjectMCPConfig(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	server := config.GetServer(name)
	if server == nil {
		s.jsonError(w, "MCP server not found", http.StatusNotFound)
		return
	}

	// Return full server config with name
	response := map[string]any{
		"name":     name,
		"type":     server.GetTransportType(),
		"command":  server.Command,
		"args":     server.Args,
		"env":      server.Env,
		"url":      server.URL,
		"headers":  server.Headers,
		"disabled": server.Disabled,
	}

	s.jsonResponse(w, response)
}

// handleCreateMCPServer creates a new MCP server in .mcp.json.
func (s *Server) handleCreateMCPServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string            `json:"name"`
		Type     string            `json:"type,omitempty"`
		Command  string            `json:"command,omitempty"`
		Args     []string          `json:"args,omitempty"`
		Env      map[string]string `json:"env,omitempty"`
		URL      string            `json:"url,omitempty"`
		Headers  []string          `json:"headers,omitempty"`
		Disabled bool              `json:"disabled,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		config = claudeconfig.NewMCPConfig()
	}

	// Check if server already exists
	if config.GetServer(req.Name) != nil {
		s.jsonError(w, "MCP server already exists", http.StatusConflict)
		return
	}

	server := &claudeconfig.MCPServer{
		Type:     req.Type,
		Command:  req.Command,
		Args:     req.Args,
		Env:      req.Env,
		URL:      req.URL,
		Headers:  req.Headers,
		Disabled: req.Disabled,
	}

	if err := config.AddServer(req.Name, server); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, config.GetServerInfo(req.Name))
}

// handleUpdateMCPServer updates an existing MCP server.
func (s *Server) handleUpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Type     string            `json:"type,omitempty"`
		Command  string            `json:"command,omitempty"`
		Args     []string          `json:"args,omitempty"`
		Env      map[string]string `json:"env,omitempty"`
		URL      string            `json:"url,omitempty"`
		Headers  []string          `json:"headers,omitempty"`
		Disabled *bool             `json:"disabled,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	existing := config.GetServer(name)
	if existing == nil {
		s.jsonError(w, "MCP server not found", http.StatusNotFound)
		return
	}

	// Update fields
	if req.Type != "" {
		existing.Type = req.Type
	}
	if req.Command != "" {
		existing.Command = req.Command
	}
	if req.Args != nil {
		existing.Args = req.Args
	}
	if req.Env != nil {
		existing.Env = req.Env
	}
	if req.URL != "" {
		existing.URL = req.URL
	}
	if req.Headers != nil {
		existing.Headers = req.Headers
	}
	if req.Disabled != nil {
		existing.Disabled = *req.Disabled
	}

	// Validate updated server
	if err := existing.IsValid(); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, config.GetServerInfo(name))
}

// handleDeleteMCPServer removes an MCP server from .mcp.json.
func (s *Server) handleDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	if !config.RemoveServer(name) {
		s.jsonError(w, "MCP server not found", http.StatusNotFound)
		return
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save MCP config: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
