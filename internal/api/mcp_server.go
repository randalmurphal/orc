// Package api provides the Connect RPC and REST API server for orc.
// This file implements the MCPService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/claudeconfig"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
)

// mcpServer implements the MCPServiceHandler interface.
type mcpServer struct {
	orcv1connect.UnimplementedMCPServiceHandler
	workDir     string
	logger      *slog.Logger
	testHomeDir string // For test isolation of GLOBAL scope
}

// NewMCPServer creates a new MCPService handler.
func NewMCPServer(
	workDir string,
	logger *slog.Logger,
) orcv1connect.MCPServiceHandler {
	return &mcpServer{
		workDir: workDir,
		logger:  logger,
	}
}

// getProjectRoot returns the project root directory.
func (s *mcpServer) getProjectRoot() string {
	if s.workDir != "" {
		return s.workDir
	}
	cwd, _ := os.Getwd()
	return cwd
}

// ListMCPServers returns all MCP servers from .mcp.json.
func (s *mcpServer) ListMCPServers(
	ctx context.Context,
	req *connect.Request[orcv1.ListMCPServersRequest],
) (*connect.Response[orcv1.ListMCPServersResponse], error) {
	var config *claudeconfig.MCPConfig
	var err error

	if req.Msg.Scope == orcv1.MCPScope_MCP_SCOPE_GLOBAL {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get home directory: %w", err))
		}
		// Global MCP config is at ~/.claude/.mcp.json
		config, err = claudeconfig.LoadProjectMCPConfig(filepath.Join(homeDir, ".claude"))
		if err != nil {
			// No global MCP config is OK - return empty list
			return connect.NewResponse(&orcv1.ListMCPServersResponse{
				Servers: []*orcv1.MCPServerInfo{},
			}), nil
		}
	} else {
		config, err = claudeconfig.LoadProjectMCPConfig(s.getProjectRoot())
		if err != nil {
			// No MCP config is OK - return empty list
			return connect.NewResponse(&orcv1.ListMCPServersResponse{
				Servers: []*orcv1.MCPServerInfo{},
			}), nil
		}
	}

	infos := config.ListServerInfos()
	protoInfos := make([]*orcv1.MCPServerInfo, 0, len(infos))
	for _, info := range infos {
		protoInfos = append(protoInfos, mcpServerInfoToProto(info))
	}

	return connect.NewResponse(&orcv1.ListMCPServersResponse{
		Servers: protoInfos,
	}), nil
}

// GetMCPServer returns a specific MCP server by name.
func (s *mcpServer) GetMCPServer(
	ctx context.Context,
	req *connect.Request[orcv1.GetMCPServerRequest],
) (*connect.Response[orcv1.GetMCPServerResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("server name required"))
	}

	config, err := claudeconfig.LoadProjectMCPConfig(s.getProjectRoot())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load MCP config: %w", err))
	}

	server := config.GetServer(req.Msg.Name)
	if server == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("MCP server not found: %s", req.Msg.Name))
	}

	return connect.NewResponse(&orcv1.GetMCPServerResponse{
		Server: mcpServerToProto(req.Msg.Name, server),
	}), nil
}

// CreateMCPServer creates a new MCP server in .mcp.json.
func (s *mcpServer) CreateMCPServer(
	ctx context.Context,
	req *connect.Request[orcv1.CreateMCPServerRequest],
) (*connect.Response[orcv1.CreateMCPServerResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		config = claudeconfig.NewMCPConfig()
	}

	// Check if server already exists
	if config.GetServer(req.Msg.Name) != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("MCP server already exists: %s", req.Msg.Name))
	}

	serverType := ""
	if req.Msg.Type != nil {
		serverType = *req.Msg.Type
	}

	command := ""
	if req.Msg.Command != nil {
		command = *req.Msg.Command
	}

	url := ""
	if req.Msg.Url != nil {
		url = *req.Msg.Url
	}

	server := &claudeconfig.MCPServer{
		Type:     serverType,
		Command:  command,
		Args:     req.Msg.Args,
		Env:      req.Msg.Env,
		URL:      url,
		Headers:  req.Msg.Headers,
		Disabled: req.Msg.Disabled,
	}

	if err := config.AddServer(req.Msg.Name, server); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save MCP config: %w", err))
	}

	s.logger.Info("MCP server created", "name", req.Msg.Name)
	return connect.NewResponse(&orcv1.CreateMCPServerResponse{
		Server: mcpServerInfoToProto(config.GetServerInfo(req.Msg.Name)),
	}), nil
}

// UpdateMCPServer updates an existing MCP server.
func (s *mcpServer) UpdateMCPServer(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateMCPServerRequest],
) (*connect.Response[orcv1.UpdateMCPServerResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("server name required"))
	}

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load MCP config: %w", err))
	}

	existing := config.GetServer(req.Msg.Name)
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("MCP server not found: %s", req.Msg.Name))
	}

	// Update fields
	if req.Msg.Type != nil {
		existing.Type = *req.Msg.Type
	}
	if req.Msg.Command != nil {
		existing.Command = *req.Msg.Command
	}
	if len(req.Msg.Args) > 0 {
		existing.Args = req.Msg.Args
	}
	if len(req.Msg.Env) > 0 {
		existing.Env = req.Msg.Env
	}
	if req.Msg.Url != nil {
		existing.URL = *req.Msg.Url
	}
	if len(req.Msg.Headers) > 0 {
		existing.Headers = req.Msg.Headers
	}
	if req.Msg.Disabled != nil {
		existing.Disabled = *req.Msg.Disabled
	}

	// Validate updated server
	if err := existing.IsValid(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save MCP config: %w", err))
	}

	s.logger.Info("MCP server updated", "name", req.Msg.Name)
	return connect.NewResponse(&orcv1.UpdateMCPServerResponse{
		Server: mcpServerInfoToProto(config.GetServerInfo(req.Msg.Name)),
	}), nil
}

// DeleteMCPServer removes an MCP server from .mcp.json.
func (s *mcpServer) DeleteMCPServer(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteMCPServerRequest],
) (*connect.Response[orcv1.DeleteMCPServerResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("server name required"))
	}

	projectRoot := s.getProjectRoot()
	config, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load MCP config: %w", err))
	}

	if !config.RemoveServer(req.Msg.Name) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("MCP server not found: %s", req.Msg.Name))
	}

	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save MCP config: %w", err))
	}

	s.logger.Info("MCP server deleted", "name", req.Msg.Name)
	return connect.NewResponse(&orcv1.DeleteMCPServerResponse{}), nil
}

// mcpServerInfoToProto converts claudeconfig.MCPServerInfo to proto.
func mcpServerInfoToProto(info *claudeconfig.MCPServerInfo) *orcv1.MCPServerInfo {
	if info == nil {
		return nil
	}

	proto := &orcv1.MCPServerInfo{
		Name:      info.Name,
		Type:      info.Type,
		Disabled:  info.Disabled,
		HasEnv:    info.HasEnv,
		EnvCount:  int32(info.EnvCount),
		ArgsCount: int32(info.ArgsCount),
	}

	if info.Command != "" {
		proto.Command = &info.Command
	}
	if info.URL != "" {
		proto.Url = &info.URL
	}

	return proto
}

// mcpServerToProto converts claudeconfig.MCPServer to proto.
func mcpServerToProto(name string, server *claudeconfig.MCPServer) *orcv1.MCPServer {
	if server == nil {
		return nil
	}

	proto := &orcv1.MCPServer{
		Name:     name,
		Type:     server.GetTransportType(),
		Args:     server.Args,
		Env:      server.Env,
		Headers:  server.Headers,
		Disabled: server.Disabled,
	}

	if server.Command != "" {
		proto.Command = &server.Command
	}
	if server.URL != "" {
		proto.Url = &server.URL
	}

	return proto
}
