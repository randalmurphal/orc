// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-674: Export/Import/Scan for MCP servers between project and global .mcp.json files
//
// These tests verify that mcpServer export/import/scan methods correctly:
// - Export MCP servers from source scope's .mcp.json to destination scope's .mcp.json (merge)
// - Scan source scope's .mcp.json against compare_to scope's .mcp.json for new/modified servers
// - Import MCP servers from source to destination (reject duplicates)
//
// Tests will NOT COMPILE until:
// 1. Proto types added: ExportMCPServersRequest/Response, ScanMCPServersRequest/Response,
//    ImportMCPServersRequest/Response, DiscoveredMCPServer
// 2. RPCs added to MCPService: ExportMCPServers, ScanMCPServers, ImportMCPServers
// 3. Handler methods implemented on mcpServer
// 4. testHomeDir field added to mcpServer struct
package api

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/llmkit/claudeconfig"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// --- Test helpers for MCP export/import tests ---

// newTestMCPServerForExport creates an mcpServer with isolated temp dirs for
// both project (workDir) and global (testHomeDir) scopes.
// Returns the server, projectDir, and globalDir.
func newTestMCPServerForExport(t *testing.T) (*mcpServer, string, string) {
	t.Helper()
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	s := &mcpServer{
		workDir:     projectDir,
		logger:      slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		testHomeDir: globalDir,
	}

	return s, projectDir, globalDir
}

// seedMCPServer writes an MCP server entry into the .mcp.json at the given configDir.
// For project scope, configDir is the project root.
// For global scope, configDir is filepath.Join(globalDir, ".claude").
func seedMCPServer(t *testing.T, configDir, name, serverType, cmd, url string) {
	t.Helper()

	// Ensure the directory exists (needed for global scope where .claude/ may not exist)
	require.NoError(t, os.MkdirAll(configDir, 0755))

	config, err := claudeconfig.LoadProjectMCPConfig(configDir)
	if err != nil {
		config = claudeconfig.NewMCPConfig()
	}

	server := &claudeconfig.MCPServer{
		Type:    serverType,
		Command: cmd,
		URL:     url,
	}

	err = config.AddServer(name, server)
	require.NoError(t, err)

	err = claudeconfig.SaveProjectMCPConfig(configDir, config)
	require.NoError(t, err)
}

// ============================================================================
// ExportMCPServers tests
// ============================================================================

func TestExportMCPServers_CopiesToDestination(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Seed two servers in project scope
	seedMCPServer(t, projectDir, "my-server", "stdio", "node", "")
	seedMCPServer(t, projectDir, "other-server", "sse", "", "http://localhost:3000")

	// Export only "my-server" from project to global
	req := connect.NewRequest(&orcv1.ExportMCPServersRequest{
		ServerNames: []string{"my-server"},
		Source:      orcv1.MCPScope_MCP_SCOPE_PROJECT,
		Destination: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ExportMCPServers(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.ExportedCount)

	// Verify the server appears in global .mcp.json
	globalConfigDir := filepath.Join(globalDir, ".claude")
	globalConfig, err := claudeconfig.LoadProjectMCPConfig(globalConfigDir)
	require.NoError(t, err)

	exported := globalConfig.GetServer("my-server")
	require.NotNil(t, exported, "exported server should exist in global config")
	assert.Equal(t, "node", exported.Command)

	// "other-server" should NOT be in global (wasn't exported)
	assert.Nil(t, globalConfig.GetServer("other-server"))
}

func TestExportMCPServers_MergesWithExisting(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Pre-seed global with an existing server
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "existing-server", "stdio", "python", "")

	// Seed project with a new server
	seedMCPServer(t, projectDir, "new-server", "sse", "", "http://localhost:8080")

	// Export new-server from project to global
	req := connect.NewRequest(&orcv1.ExportMCPServersRequest{
		ServerNames: []string{"new-server"},
		Source:      orcv1.MCPScope_MCP_SCOPE_PROJECT,
		Destination: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ExportMCPServers(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.ExportedCount)

	// Verify both servers exist in global
	globalConfig, err := claudeconfig.LoadProjectMCPConfig(globalConfigDir)
	require.NoError(t, err)

	assert.NotNil(t, globalConfig.GetServer("existing-server"), "pre-existing server should still be present")
	assert.NotNil(t, globalConfig.GetServer("new-server"), "newly exported server should be present")
}

func TestExportMCPServers_NonexistentServer_ReturnsError(t *testing.T) {
	t.Parallel()
	server, projectDir, _ := newTestMCPServerForExport(t)

	// Seed one server so the config file exists
	seedMCPServer(t, projectDir, "real-server", "stdio", "node", "")

	req := connect.NewRequest(&orcv1.ExportMCPServersRequest{
		ServerNames: []string{"nonexistent"},
		Source:      orcv1.MCPScope_MCP_SCOPE_PROJECT,
		Destination: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	_, err := server.ExportMCPServers(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestExportMCPServers_EmptyNames_ReturnsError(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestMCPServerForExport(t)

	req := connect.NewRequest(&orcv1.ExportMCPServersRequest{
		ServerNames: []string{},
		Source:      orcv1.MCPScope_MCP_SCOPE_PROJECT,
		Destination: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	_, err := server.ExportMCPServers(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ============================================================================
// ScanMCPServers tests
// ============================================================================

func TestScanMCPServers_FindsNewServers(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Project has a server; global does not
	seedMCPServer(t, projectDir, "project-only", "stdio", "node", "")

	// Ensure global .claude dir exists but has no .mcp.json (or empty config)
	globalConfigDir := filepath.Join(globalDir, ".claude")
	require.NoError(t, os.MkdirAll(globalConfigDir, 0755))

	req := connect.NewRequest(&orcv1.ScanMCPServersRequest{
		Source:    orcv1.MCPScope_MCP_SCOPE_PROJECT,
		CompareTo: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ScanMCPServers(context.Background(), req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Servers, 1)
	assert.Equal(t, "project-only", resp.Msg.Servers[0].Name)
	assert.Equal(t, "new", resp.Msg.Servers[0].Status)
	assert.Equal(t, "stdio", resp.Msg.Servers[0].Type)

	// Command should be populated
	require.NotNil(t, resp.Msg.Servers[0].Command)
	assert.Equal(t, "node", *resp.Msg.Servers[0].Command)
}

func TestScanMCPServers_FindsModifiedServers(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Same name in both scopes, but different command
	seedMCPServer(t, projectDir, "shared-server", "stdio", "node-v2", "")
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "shared-server", "stdio", "node-v1", "")

	req := connect.NewRequest(&orcv1.ScanMCPServersRequest{
		Source:    orcv1.MCPScope_MCP_SCOPE_PROJECT,
		CompareTo: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ScanMCPServers(context.Background(), req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Servers, 1)
	assert.Equal(t, "shared-server", resp.Msg.Servers[0].Name)
	assert.Equal(t, "modified", resp.Msg.Servers[0].Status)
}

func TestScanMCPServers_SkipsSyncedServers(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Identical server in both scopes
	seedMCPServer(t, projectDir, "synced-server", "stdio", "node", "")
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "synced-server", "stdio", "node", "")

	req := connect.NewRequest(&orcv1.ScanMCPServersRequest{
		Source:    orcv1.MCPScope_MCP_SCOPE_PROJECT,
		CompareTo: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ScanMCPServers(context.Background(), req)
	require.NoError(t, err)

	// Synced server should NOT appear in results
	for _, s := range resp.Msg.Servers {
		assert.NotEqual(t, "synced-server", s.Name,
			"synced server should not appear in scan results")
	}
}

func TestScanMCPServers_EmptySource_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	server, _, globalDir := newTestMCPServerForExport(t)

	// Project has no .mcp.json, global has a server
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "global-only", "stdio", "python", "")

	req := connect.NewRequest(&orcv1.ScanMCPServersRequest{
		Source:    orcv1.MCPScope_MCP_SCOPE_PROJECT,
		CompareTo: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})

	resp, err := server.ScanMCPServers(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Servers)
}

// ============================================================================
// ImportMCPServers tests
// ============================================================================

func TestImportMCPServers_CopiesToDestination(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Seed global with servers to import from
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "import-me", "stdio", "npx", "")
	seedMCPServer(t, globalConfigDir, "import-me-too", "sse", "", "http://localhost:9090")

	req := connect.NewRequest(&orcv1.ImportMCPServersRequest{
		ServerNames: []string{"import-me", "import-me-too"},
		Source:      orcv1.MCPScope_MCP_SCOPE_GLOBAL,
		Destination: orcv1.MCPScope_MCP_SCOPE_PROJECT,
	})

	resp, err := server.ImportMCPServers(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Msg.ImportedCount)

	// Verify servers in project .mcp.json
	projectConfig, err := claudeconfig.LoadProjectMCPConfig(projectDir)
	require.NoError(t, err)

	imported1 := projectConfig.GetServer("import-me")
	require.NotNil(t, imported1)
	assert.Equal(t, "npx", imported1.Command)

	imported2 := projectConfig.GetServer("import-me-too")
	require.NotNil(t, imported2)
	assert.Equal(t, "http://localhost:9090", imported2.URL)
}

func TestImportMCPServers_RejectsDuplicates(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)

	// Server exists in both source (global) and destination (project)
	globalConfigDir := filepath.Join(globalDir, ".claude")
	seedMCPServer(t, globalConfigDir, "duplicate-server", "stdio", "node", "")
	seedMCPServer(t, projectDir, "duplicate-server", "stdio", "python", "")

	req := connect.NewRequest(&orcv1.ImportMCPServersRequest{
		ServerNames: []string{"duplicate-server"},
		Source:      orcv1.MCPScope_MCP_SCOPE_GLOBAL,
		Destination: orcv1.MCPScope_MCP_SCOPE_PROJECT,
	})

	_, err := server.ImportMCPServers(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	// Verify original is unchanged in destination
	projectConfig, err := claudeconfig.LoadProjectMCPConfig(projectDir)
	require.NoError(t, err)
	existing := projectConfig.GetServer("duplicate-server")
	require.NotNil(t, existing)
	assert.Equal(t, "python", existing.Command, "original server should be unchanged")
}

// ============================================================================
// Round-trip: export → scan → import
// ============================================================================

func TestExportScanImportRoundTrip_MCP(t *testing.T) {
	t.Parallel()
	server, projectDir, globalDir := newTestMCPServerForExport(t)
	globalConfigDir := filepath.Join(globalDir, ".claude")

	// 1. Seed project with servers
	seedMCPServer(t, projectDir, "roundtrip-stdio", "stdio", "node", "")
	seedMCPServer(t, projectDir, "roundtrip-sse", "sse", "", "http://localhost:4000")

	// 2. Export from project to global
	exportReq := connect.NewRequest(&orcv1.ExportMCPServersRequest{
		ServerNames: []string{"roundtrip-stdio", "roundtrip-sse"},
		Source:      orcv1.MCPScope_MCP_SCOPE_PROJECT,
		Destination: orcv1.MCPScope_MCP_SCOPE_GLOBAL,
	})
	exportResp, err := server.ExportMCPServers(context.Background(), exportReq)
	require.NoError(t, err)
	assert.Equal(t, int32(2), exportResp.Msg.ExportedCount)

	// 3. Verify servers in global config
	globalConfig, err := claudeconfig.LoadProjectMCPConfig(globalConfigDir)
	require.NoError(t, err)
	assert.NotNil(t, globalConfig.GetServer("roundtrip-stdio"))
	assert.NotNil(t, globalConfig.GetServer("roundtrip-sse"))

	// 4. Scan global vs project - should find no differences (all synced)
	scanReq := connect.NewRequest(&orcv1.ScanMCPServersRequest{
		Source:    orcv1.MCPScope_MCP_SCOPE_GLOBAL,
		CompareTo: orcv1.MCPScope_MCP_SCOPE_PROJECT,
	})
	scanResp, err := server.ScanMCPServers(context.Background(), scanReq)
	require.NoError(t, err)
	assert.Empty(t, scanResp.Msg.Servers, "all servers are synced, scan should return nothing")

	// 5. Add a new server to global only
	seedMCPServer(t, globalConfigDir, "global-new", "stdio", "go", "")

	// 6. Scan global vs project - should find the new one
	scanResp, err = server.ScanMCPServers(context.Background(), scanReq)
	require.NoError(t, err)
	require.Len(t, scanResp.Msg.Servers, 1)
	assert.Equal(t, "global-new", scanResp.Msg.Servers[0].Name)
	assert.Equal(t, "new", scanResp.Msg.Servers[0].Status)

	// 7. Import the new server from global to project
	importReq := connect.NewRequest(&orcv1.ImportMCPServersRequest{
		ServerNames: []string{"global-new"},
		Source:      orcv1.MCPScope_MCP_SCOPE_GLOBAL,
		Destination: orcv1.MCPScope_MCP_SCOPE_PROJECT,
	})
	importResp, err := server.ImportMCPServers(context.Background(), importReq)
	require.NoError(t, err)
	assert.Equal(t, int32(1), importResp.Msg.ImportedCount)

	// 8. Verify in project config
	projectConfig, err := claudeconfig.LoadProjectMCPConfig(projectDir)
	require.NoError(t, err)
	imported := projectConfig.GetServer("global-new")
	require.NotNil(t, imported, "imported server should exist in project config")
	assert.Equal(t, "go", imported.Command)
}
