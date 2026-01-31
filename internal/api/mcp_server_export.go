// Package api provides the Connect RPC and REST API server for orc.
// This file implements export/import/scan handlers for MCP servers.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/claudeconfig"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// resolveMCPDir returns the directory containing .mcp.json for the given scope.
// For PROJECT scope, this is the project root (workDir).
// For GLOBAL scope, this is ~/.claude (or testHomeDir/.claude for tests).
func (s *mcpServer) resolveMCPDir(scope orcv1.MCPScope) (string, error) {
	switch scope {
	case orcv1.MCPScope_MCP_SCOPE_PROJECT:
		return s.getProjectRoot(), nil
	case orcv1.MCPScope_MCP_SCOPE_GLOBAL:
		if s.testHomeDir != "" {
			return filepath.Join(s.testHomeDir, ".claude"), nil
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		return filepath.Join(homeDir, ".claude"), nil
	default:
		return "", fmt.Errorf("scope must be PROJECT or GLOBAL")
	}
}

// ExportMCPServers copies named MCP servers from source scope to destination scope.
// Export overwrites same-named servers in the destination (intentional user action).
func (s *mcpServer) ExportMCPServers(
	ctx context.Context,
	req *connect.Request[orcv1.ExportMCPServersRequest],
) (*connect.Response[orcv1.ExportMCPServersResponse], error) {
	if len(req.Msg.ServerNames) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one server name required"))
	}

	srcDir, err := s.resolveMCPDir(req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve source: %w", err))
	}

	dstDir, err := s.resolveMCPDir(req.Msg.Destination)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve destination: %w", err))
	}

	srcConfig, err := claudeconfig.LoadProjectMCPConfig(srcDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load source MCP config: %w", err))
	}

	// Load or create destination config
	dstConfig, err := claudeconfig.LoadProjectMCPConfig(dstDir)
	if err != nil {
		dstConfig = claudeconfig.NewMCPConfig()
	}

	var exported int32
	for _, name := range req.Msg.ServerNames {
		server := srcConfig.GetServer(name)
		if server == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("MCP server not found in source: %s", name))
		}

		// Remove existing server with same name if present (export overwrites)
		dstConfig.RemoveServer(name)

		if err := dstConfig.AddServer(name, server); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("add server %s to destination: %w", name, err))
		}
		exported++
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create destination directory: %w", err))
	}

	if err := claudeconfig.SaveProjectMCPConfig(dstDir, dstConfig); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save destination MCP config: %w", err))
	}

	s.logger.Info("MCP servers exported", "count", exported, "source", req.Msg.Source, "destination", req.Msg.Destination)
	return connect.NewResponse(&orcv1.ExportMCPServersResponse{
		ExportedCount: exported,
	}), nil
}

// ScanMCPServers scans source scope for servers that are new or modified compared to compare_to scope.
func (s *mcpServer) ScanMCPServers(
	ctx context.Context,
	req *connect.Request[orcv1.ScanMCPServersRequest],
) (*connect.Response[orcv1.ScanMCPServersResponse], error) {
	srcDir, err := s.resolveMCPDir(req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve source: %w", err))
	}

	compareDir, err := s.resolveMCPDir(req.Msg.CompareTo)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve compare_to: %w", err))
	}

	srcConfig, err := claudeconfig.LoadProjectMCPConfig(srcDir)
	if err != nil {
		// No source config means nothing to scan
		return connect.NewResponse(&orcv1.ScanMCPServersResponse{
			Servers: []*orcv1.DiscoveredMCPServer{},
		}), nil
	}

	compareConfig, err := claudeconfig.LoadProjectMCPConfig(compareDir)
	if err != nil {
		// No compare config means all source servers are "new"
		compareConfig = claudeconfig.NewMCPConfig()
	}

	var discovered []*orcv1.DiscoveredMCPServer
	for _, info := range srcConfig.ListServerInfos() {
		srcServer := srcConfig.GetServer(info.Name)
		if srcServer == nil {
			continue
		}

		compareServer := compareConfig.GetServer(info.Name)

		if compareServer == nil {
			// New server
			d := &orcv1.DiscoveredMCPServer{
				Name:   info.Name,
				Type:   srcServer.GetTransportType(),
				Status: "new",
			}
			if srcServer.Command != "" {
				d.Command = &srcServer.Command
			}
			if srcServer.URL != "" {
				d.Url = &srcServer.URL
			}
			discovered = append(discovered, d)
		} else {
			// Compare by JSON equality
			srcJSON, _ := json.Marshal(srcServer)
			compareJSON, _ := json.Marshal(compareServer)
			if string(srcJSON) != string(compareJSON) {
				d := &orcv1.DiscoveredMCPServer{
					Name:   info.Name,
					Type:   srcServer.GetTransportType(),
					Status: "modified",
				}
				if srcServer.Command != "" {
					d.Command = &srcServer.Command
				}
				if srcServer.URL != "" {
					d.Url = &srcServer.URL
				}
				discovered = append(discovered, d)
			}
		}
	}

	return connect.NewResponse(&orcv1.ScanMCPServersResponse{
		Servers: discovered,
	}), nil
}

// ImportMCPServers copies named MCP servers from source scope to destination scope.
// Import rejects duplicates (servers that already exist in destination).
func (s *mcpServer) ImportMCPServers(
	ctx context.Context,
	req *connect.Request[orcv1.ImportMCPServersRequest],
) (*connect.Response[orcv1.ImportMCPServersResponse], error) {
	if len(req.Msg.ServerNames) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one server name required"))
	}

	srcDir, err := s.resolveMCPDir(req.Msg.Source)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve source: %w", err))
	}

	dstDir, err := s.resolveMCPDir(req.Msg.Destination)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("resolve destination: %w", err))
	}

	srcConfig, err := claudeconfig.LoadProjectMCPConfig(srcDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load source MCP config: %w", err))
	}

	// Load or create destination config
	dstConfig, err := claudeconfig.LoadProjectMCPConfig(dstDir)
	if err != nil {
		dstConfig = claudeconfig.NewMCPConfig()
	}

	// Check for duplicates before making any changes
	for _, name := range req.Msg.ServerNames {
		if dstConfig.GetServer(name) != nil {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("MCP server %q already exists in destination", name))
		}
	}

	var imported int32
	for _, name := range req.Msg.ServerNames {
		server := srcConfig.GetServer(name)
		if server == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("MCP server not found in source: %s", name))
		}

		if err := dstConfig.AddServer(name, server); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("add server %s to destination: %w", name, err))
		}
		imported++
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create destination directory: %w", err))
	}

	if err := claudeconfig.SaveProjectMCPConfig(dstDir, dstConfig); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save destination MCP config: %w", err))
	}

	s.logger.Info("MCP servers imported", "count", imported, "source", req.Msg.Source, "destination", req.Msg.Destination)
	return connect.NewResponse(&orcv1.ImportMCPServersResponse{
		ImportedCount: imported,
	}), nil
}
