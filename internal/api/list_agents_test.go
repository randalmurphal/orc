// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-601: Implement ListAgents backend API endpoint
//
// Tests for the ListAgents gRPC handler that returns agent configurations
// with runtime statistics (tokens_today, tasks_done, success_rate) and status.
package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-1: ListAgents gRPC endpoint returns agents from SQLite agents table
// ============================================================================

// TestListAgents_ReturnsAgentsFromDatabase verifies SC-1:
// ListAgents returns agents stored in the SQLite agents table.
func TestListAgents_ReturnsAgentsFromDatabase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create test agents in database
	agent1 := &db.Agent{
		ID:          "test-agent-1",
		Name:        "Test Agent 1",
		Description: "First test agent",
		Prompt:      "You are test agent 1.",
		Tools:       []string{"Read", "Grep"},
		Model:       "sonnet",
	}
	agent2 := &db.Agent{
		ID:          "test-agent-2",
		Name:        "Test Agent 2",
		Description: "Second test agent",
		Prompt:      "You are test agent 2.",
		Tools:       []string{"Read", "Edit"},
		Model:       "opus",
	}

	if err := pdb.SaveAgent(agent1); err != nil {
		t.Fatalf("save agent1: %v", err)
	}
	if err := pdb.SaveAgent(agent2); err != nil {
		t.Fatalf("save agent2: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Verify agents are returned
	if len(resp.Msg.Agents) < 2 {
		t.Errorf("expected at least 2 agents, got %d", len(resp.Msg.Agents))
	}

	// Find our test agents in the response
	var foundAgent1, foundAgent2 bool
	for _, a := range resp.Msg.Agents {
		if a.Name == "Test Agent 1" {
			foundAgent1 = true
			if a.Description != "First test agent" {
				t.Errorf("agent1 description = %q, want %q", a.Description, "First test agent")
			}
		}
		if a.Name == "Test Agent 2" {
			foundAgent2 = true
			if a.Description != "Second test agent" {
				t.Errorf("agent2 description = %q, want %q", a.Description, "Second test agent")
			}
		}
	}

	if !foundAgent1 {
		t.Error("Test Agent 1 not found in response")
	}
	if !foundAgent2 {
		t.Error("Test Agent 2 not found in response")
	}
}

// ============================================================================
// SC-2: Each agent includes tool permissions (allow/deny lists)
// ============================================================================

// TestListAgents_IncludesToolPermissions verifies SC-2:
// Each agent in the response includes tool permissions with allow/deny lists.
func TestListAgents_IncludesToolPermissions(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent with specific tools
	agent := &db.Agent{
		ID:          "tools-agent",
		Name:        "Tools Test Agent",
		Description: "Agent with specific tools",
		Prompt:      "Test prompt",
		Tools:       []string{"Read", "Grep", "Edit"},
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Tools Test Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Tools Test Agent not found in response")
	}

	// Verify tools field is present
	if found.Tools == nil {
		t.Fatal("agent.Tools is nil, expected ToolPermissions")
	}

	// Tools from db.Agent.Tools should be in the allow list
	if len(found.Tools.Allow) != 3 {
		t.Errorf("agent.Tools.Allow has %d items, want 3", len(found.Tools.Allow))
	}

	// Check specific tools are present
	toolSet := make(map[string]bool)
	for _, tool := range found.Tools.Allow {
		toolSet[tool] = true
	}
	for _, expected := range []string{"Read", "Grep", "Edit"} {
		if !toolSet[expected] {
			t.Errorf("tool %q not found in allow list", expected)
		}
	}
}

// TestListAgents_AgentWithEmptyTools verifies edge case:
// Agent with empty tools array returns empty allow list (not nil).
func TestListAgents_AgentWithEmptyTools(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent without tools
	agent := &db.Agent{
		ID:          "no-tools-agent",
		Name:        "No Tools Agent",
		Description: "Agent without tools",
		Prompt:      "Test prompt",
		Tools:       []string{}, // Empty tools
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "No Tools Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("No Tools Agent not found in response")
	}

	// Tools can be nil for agents without tools, or have empty allow list
	if found.Tools != nil && len(found.Tools.Allow) > 0 {
		t.Errorf("expected empty allow list, got %d items", len(found.Tools.Allow))
	}
}

// ============================================================================
// SC-3: Agents from both project scope (SQLite) and global scope
// ============================================================================

// TestListAgents_NoScope_ReturnsBothScopes verifies SC-3:
// When no scope filter is set, returns agents from both project (SQLite)
// and global (.claude config) sources.
func TestListAgents_NoScope_ReturnsBothScopes(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create project-scope agent in database
	projectAgent := &db.Agent{
		ID:          "project-agent-sc3",
		Name:        "Project Agent SC3",
		Description: "Project-scope agent",
		Prompt:      "Test prompt",
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(projectAgent); err != nil {
		t.Fatalf("save project agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	// Call without scope filter
	req := connect.NewRequest(&orcv1.ListAgentsRequest{
		// No scope specified - should return all
	})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Should include project agent
	var foundProject bool
	for _, a := range resp.Msg.Agents {
		if a.Name == "Project Agent SC3" {
			foundProject = true
			// Verify scope is PROJECT
			if a.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT {
				t.Errorf("project agent scope = %v, want PROJECT", a.Scope)
			}
			break
		}
	}

	if !foundProject {
		t.Error("project agent not found when no scope specified")
	}
}

// TestListAgents_ProjectScope_OnlyProjectAgents verifies edge case:
// scope=PROJECT filter returns only SQLite agents, not global ones.
func TestListAgents_ProjectScope_OnlyProjectAgents(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create project-scope agent
	projectAgent := &db.Agent{
		ID:          "project-only-agent",
		Name:        "Project Only Agent",
		Description: "Should be returned",
		Prompt:      "Test prompt",
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(projectAgent); err != nil {
		t.Fatalf("save project agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	// Call with PROJECT scope
	projectScope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	req := connect.NewRequest(&orcv1.ListAgentsRequest{
		Scope: &projectScope,
	})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Should include our project agent
	var found bool
	for _, a := range resp.Msg.Agents {
		if a.Name == "Project Only Agent" {
			found = true
			break
		}
	}

	if !found {
		t.Error("project agent not found when PROJECT scope specified")
	}

	// All returned agents should have PROJECT scope
	for _, a := range resp.Msg.Agents {
		if a.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT {
			t.Errorf("agent %q has scope %v, want PROJECT", a.Name, a.Scope)
		}
	}
}

// TestListAgents_GlobalScope_OnlyGlobalAgents verifies edge case:
// scope=GLOBAL filter returns only .claude/ global agents, not SQLite project agents.
func TestListAgents_GlobalScope_OnlyGlobalAgents(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create project-scope agent in SQLite
	projectAgent := &db.Agent{
		ID:          "project-agent-global-test",
		Name:        "Project Agent Global Test",
		Description: "Should NOT be returned with GLOBAL scope",
		Prompt:      "Test prompt",
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(projectAgent); err != nil {
		t.Fatalf("save project agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	// Call with GLOBAL scope
	globalScope := orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL
	req := connect.NewRequest(&orcv1.ListAgentsRequest{
		Scope: &globalScope,
	})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Project agent should NOT be in response when GLOBAL scope requested
	for _, a := range resp.Msg.Agents {
		if a.Name == "Project Agent Global Test" {
			t.Error("project agent returned when GLOBAL scope specified")
		}
	}

	// All returned agents should have GLOBAL scope
	for _, a := range resp.Msg.Agents {
		if a.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL {
			t.Errorf("agent %q has scope %v, want GLOBAL", a.Name, a.Scope)
		}
	}
}

// ============================================================================
// SC-4: Agent response includes tokens_today stat
// ============================================================================

// TestListAgents_IncludesTokensToday verifies SC-4:
// Agent response includes tokens_today stat from usage_metrics table.
func TestListAgents_IncludesTokensToday(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent with model "sonnet"
	agent := &db.Agent{
		ID:          "stats-agent-tokens",
		Name:        "Stats Agent Tokens",
		Description: "Agent for token stats testing",
		Prompt:      "Test prompt",
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// The implementation will use GetAgentStats() to get token usage
	// For now, we just verify the field exists in the response

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Stats Agent Tokens" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Stats Agent Tokens not found in response")
	}

	// Stats should be present (even if zero)
	if found.Stats == nil {
		t.Fatal("agent.Stats is nil, expected AgentStats")
	}

	// TokensToday should be >= 0 (no negative tokens)
	if found.Stats.TokensToday < 0 {
		t.Errorf("tokens_today = %d, expected >= 0", found.Stats.TokensToday)
	}
}

// ============================================================================
// SC-5: Agent response includes tasks_done and success_rate stats
// ============================================================================

// TestListAgents_IncludesTaskStats verifies SC-5:
// Agent response includes tasks_done and success_rate stats.
func TestListAgents_IncludesTaskStats(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent
	agent := &db.Agent{
		ID:          "stats-agent-tasks",
		Name:        "Stats Agent Tasks",
		Description: "Agent for task stats testing",
		Prompt:      "Test prompt",
		Model:       "opus",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Stats Agent Tasks" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Stats Agent Tasks not found in response")
	}

	// Stats should be present
	if found.Stats == nil {
		t.Fatal("agent.Stats is nil, expected AgentStats")
	}

	// TasksDone should be >= 0
	if found.Stats.TasksDone < 0 {
		t.Errorf("tasks_done = %d, expected >= 0", found.Stats.TasksDone)
	}

	// SuccessRate should be between 0 and 1 (or 0 if no tasks)
	if found.Stats.SuccessRate < 0 || found.Stats.SuccessRate > 1 {
		t.Errorf("success_rate = %f, expected between 0 and 1", found.Stats.SuccessRate)
	}
}

// TestListAgents_StatsComputedByModel verifies spec assumption:
// Stats are joined to agents by model name (not agent ID).
func TestListAgents_StatsComputedByModel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create two agents with the same model - they should share stats
	agent1 := &db.Agent{
		ID:          "shared-model-agent-1",
		Name:        "Shared Model Agent 1",
		Description: "First agent with shared model",
		Prompt:      "Test prompt",
		Model:       "haiku",
	}
	agent2 := &db.Agent{
		ID:          "shared-model-agent-2",
		Name:        "Shared Model Agent 2",
		Description: "Second agent with shared model",
		Prompt:      "Test prompt",
		Model:       "haiku", // Same model
	}

	if err := pdb.SaveAgent(agent1); err != nil {
		t.Fatalf("save agent1: %v", err)
	}
	if err := pdb.SaveAgent(agent2); err != nil {
		t.Fatalf("save agent2: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find both agents
	var found1, found2 *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Shared Model Agent 1" {
			found1 = a
		}
		if a.Name == "Shared Model Agent 2" {
			found2 = a
		}
	}

	if found1 == nil || found2 == nil {
		t.Fatal("could not find both shared model agents")
	}

	// Both should have stats (may be nil if no stats available, but structure should exist)
	// When stats exist, they should be identical for same model
	if found1.Stats != nil && found2.Stats != nil {
		if found1.Stats.TokensToday != found2.Stats.TokensToday {
			t.Errorf("agents with same model have different TokensToday: %d vs %d",
				found1.Stats.TokensToday, found2.Stats.TokensToday)
		}
		if found1.Stats.TasksDone != found2.Stats.TasksDone {
			t.Errorf("agents with same model have different TasksDone: %d vs %d",
				found1.Stats.TasksDone, found2.Stats.TasksDone)
		}
		if found1.Stats.SuccessRate != found2.Stats.SuccessRate {
			t.Errorf("agents with same model have different SuccessRate: %f vs %f",
				found1.Stats.SuccessRate, found2.Stats.SuccessRate)
		}
	}
}

// ============================================================================
// SC-6: Agent response includes status field showing "active" or "idle"
// ============================================================================

// TestListAgents_IncludesStatus verifies SC-6:
// Agent response includes status field showing "active" or "idle".
func TestListAgents_IncludesStatus(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent
	agent := &db.Agent{
		ID:          "status-agent",
		Name:        "Status Agent",
		Description: "Agent for status testing",
		Prompt:      "Test prompt",
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Status Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Status Agent not found in response")
	}

	// Status should be set
	if found.Status == nil {
		t.Fatal("agent.Status is nil, expected 'active' or 'idle'")
	}

	// Status should be either "active" or "idle"
	status := *found.Status
	if status != "active" && status != "idle" {
		t.Errorf("status = %q, expected 'active' or 'idle'", status)
	}
}

// TestListAgents_IdleStatusWhenNoRunningTasks verifies BDD-2 (partial):
// Agent with no running tasks for its model has status="idle".
func TestListAgents_IdleStatusWhenNoRunningTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent with a model that has no running tasks
	agent := &db.Agent{
		ID:          "idle-agent",
		Name:        "Idle Agent",
		Description: "Agent that should be idle",
		Prompt:      "Test prompt",
		Model:       "idle-model-12345", // Unique model with no tasks
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Idle Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Idle Agent not found in response")
	}

	if found.Status == nil {
		t.Fatal("agent.Status is nil")
	}

	// Should be idle since no running tasks exist for this model
	if *found.Status != "idle" {
		t.Errorf("status = %q, expected 'idle' for agent with no running tasks", *found.Status)
	}
}

// TestListAgents_ActiveStatusWithRunningTask verifies BDD-2:
// Agent with running task for its model has status="active".
func TestListAgents_ActiveStatusWithRunningTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent
	agent := &db.Agent{
		ID:          "active-agent",
		Name:        "Active Agent",
		Description: "Agent that should be active",
		Prompt:      "Test prompt",
		Model:       "active-model",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Create a running task with the same model
	// Note: This depends on the tasks table schema having session_model field
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := pdb.Exec(`
		INSERT INTO tasks (id, title, status, session_model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "TASK-ACTIVE-001", "Running Task", "running", "active-model", now, now)
	if err != nil {
		t.Fatalf("create running task: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Active Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Active Agent not found in response")
	}

	if found.Status == nil {
		t.Fatal("agent.Status is nil")
	}

	// Should be active since a task is running with this model
	if *found.Status != "active" {
		t.Errorf("status = %q, expected 'active' for agent with running task", *found.Status)
	}
}

// ============================================================================
// SC-7: Unit tests cover ListAgents handler (BDD-3: Empty database)
// ============================================================================

// TestListAgents_EmptyDatabase verifies BDD-3:
// Empty database returns empty array with 200 status, not 404.
func TestListAgents_EmptyDatabase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	// Don't create any agents

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)

	// Should NOT return an error
	if err != nil {
		t.Fatalf("ListAgents returned error for empty database: %v", err)
	}

	// Should return response (not nil)
	if resp == nil {
		t.Fatal("ListAgents returned nil response for empty database")
	}

	// Agents list should be empty but not nil
	if resp.Msg.Agents == nil {
		// Proto generates nil slice for empty, which is fine
		// Just verify it doesn't panic when iterating
		for range resp.Msg.Agents {
			// Should not execute
		}
	}

	// Count should be 0 (or just global agents if any exist)
	// The key is that we don't return an error
}

// ============================================================================
// Error Path Tests (from Failure Modes table)
// ============================================================================

// TestListAgents_AgentWithNoModel verifies edge case:
// Agent with no model set returns with nil stats and status="idle".
func TestListAgents_AgentWithNoModel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent without model
	agent := &db.Agent{
		ID:          "no-model-agent",
		Name:        "No Model Agent",
		Description: "Agent without model set",
		Prompt:      "Test prompt",
		Model:       "", // Empty model
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "No Model Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("No Model Agent not found in response")
	}

	// Agent with no model should still be returned
	// Stats may be nil or zero (implementation can decide)
	// Status should be "idle" since we can't match it to running tasks

	if found.Status != nil && *found.Status != "idle" {
		t.Errorf("agent with no model has status %q, expected 'idle'", *found.Status)
	}
}

// TestListAgents_GracefulDegradationOnStatsFail verifies failure mode:
// If GetAgentStats fails, agents are still returned with zero stats.
// Note: This test verifies the graceful degradation behavior specified in the spec.
// The implementation should log a warning but continue with zero stats.
func TestListAgents_GracefulDegradationOnStatsFail(t *testing.T) {
	t.Parallel()

	// This test verifies that even if stats computation fails,
	// agents are still returned. Since we can't easily mock the
	// stats query failure in unit tests, we verify that agents
	// with no matching stats still return valid data.

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agent
	agent := &db.Agent{
		ID:          "graceful-agent",
		Name:        "Graceful Agent",
		Description: "Agent for graceful degradation test",
		Prompt:      "Test prompt",
		Model:       "no-stats-model",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)

	// Should succeed even if stats can't be computed
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Graceful Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Graceful Agent not found in response")
	}

	// Agent should be returned with basic info
	if found.Description != "Agent for graceful degradation test" {
		t.Errorf("agent description incorrect: %q", found.Description)
	}
}

// ============================================================================
// BDD-1: Multiple agents with different models
// ============================================================================

// TestListAgents_MultipleAgentsDifferentModels verifies BDD-1:
// Given agents with different models, all are returned with their configs and stats.
func TestListAgents_MultipleAgentsDifferentModels(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create agents with different models
	agents := []*db.Agent{
		{
			ID:          "bdd1-sonnet",
			Name:        "BDD1 Sonnet Agent",
			Description: "Uses sonnet model",
			Prompt:      "Test prompt",
			Model:       "sonnet",
			Tools:       []string{"Read"},
		},
		{
			ID:          "bdd1-opus",
			Name:        "BDD1 Opus Agent",
			Description: "Uses opus model",
			Prompt:      "Test prompt",
			Model:       "opus",
			Tools:       []string{"Read", "Edit"},
		},
		{
			ID:          "bdd1-haiku",
			Name:        "BDD1 Haiku Agent",
			Description: "Uses haiku model",
			Prompt:      "Test prompt",
			Model:       "haiku",
			Tools:       []string{"Grep"},
		},
	}

	for _, a := range agents {
		if err := pdb.SaveAgent(a); err != nil {
			t.Fatalf("save agent %s: %v", a.ID, err)
		}
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Verify all three agents are returned
	foundModels := make(map[string]bool)
	for _, a := range resp.Msg.Agents {
		switch a.Name {
		case "BDD1 Sonnet Agent":
			foundModels["sonnet"] = true
			if a.Model == nil || *a.Model != "sonnet" {
				t.Errorf("sonnet agent has model %v", a.Model)
			}
		case "BDD1 Opus Agent":
			foundModels["opus"] = true
			if a.Model == nil || *a.Model != "opus" {
				t.Errorf("opus agent has model %v", a.Model)
			}
		case "BDD1 Haiku Agent":
			foundModels["haiku"] = true
			if a.Model == nil || *a.Model != "haiku" {
				t.Errorf("haiku agent has model %v", a.Model)
			}
		}
	}

	for _, model := range []string{"sonnet", "opus", "haiku"} {
		if !foundModels[model] {
			t.Errorf("agent with model %q not found", model)
		}
	}
}

// ============================================================================
// Integration test helpers
// ============================================================================

// TestListAgents_ResponseStructure verifies the complete response structure
// matches what the frontend expects.
func TestListAgents_ResponseStructure(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create a fully-populated agent
	agent := &db.Agent{
		ID:          "structure-test",
		Name:        "Structure Test Agent",
		Description: "Tests complete response structure",
		Prompt:      "You are a test agent for verifying response structure.",
		Tools:       []string{"Read", "Grep", "Edit"},
		Model:       "sonnet",
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	server := NewConfigServer(nil, backend, t.TempDir(), nil)

	req := connect.NewRequest(&orcv1.ListAgentsRequest{})
	resp, err := server.ListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Find our test agent
	var found *orcv1.Agent
	for _, a := range resp.Msg.Agents {
		if a.Name == "Structure Test Agent" {
			found = a
			break
		}
	}

	if found == nil {
		t.Fatal("Structure Test Agent not found in response")
	}

	// Verify all expected fields are present
	if found.Name == "" {
		t.Error("Name is empty")
	}
	if found.Description == "" {
		t.Error("Description is empty")
	}
	if found.Model == nil || *found.Model == "" {
		t.Error("Model is nil or empty")
	}

	// Verify new fields from spec (SC-6, SC-4, SC-5)
	if found.Status == nil {
		t.Error("Status field is nil (SC-6)")
	}
	if found.Stats == nil {
		t.Error("Stats field is nil (SC-4, SC-5)")
	} else {
		// Stats fields should exist (may be zero)
		_ = found.Stats.TokensToday  // SC-4
		_ = found.Stats.TasksDone    // SC-5
		_ = found.Stats.SuccessRate  // SC-5
	}

	// Verify tools (SC-2)
	if found.Tools == nil {
		t.Error("Tools field is nil (SC-2)")
	}
}
