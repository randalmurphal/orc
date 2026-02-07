// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-798: GetCostReport RPC on DashboardService
//
// These tests verify the new GetCostReport endpoint that queries GlobalDB
// cost_log with user/project/model filtering and group_by dimensions.
//
// Success Criteria Coverage:
// - SC-7: GetCostReport returns aggregated cost data from GlobalDB
// - SC-8: GetCostReport filters by user_id and project_id
// - SC-9: GetCostReport groups by requested dimension
//
// Failure Modes:
// - GlobalDB not wired returns CodeInternal
// - Empty GlobalDB returns $0 totals
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-7: GetCostReport returns aggregated cost data from GlobalDB
// ============================================================================

func TestGetCostReport_ReturnsTotalFromGlobalDB(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed cost data in GlobalDB
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Total should be sum of all seeded entries: 50 + 20 + 30 = 100
	if resp.Msg.TotalCostUsd != 100.00 {
		t.Errorf("total_cost_usd = %f, want 100", resp.Msg.TotalCostUsd)
	}
}

func TestGetCostReport_EmptyGlobalDB_ReturnsZero(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if resp.Msg.TotalCostUsd != 0 {
		t.Errorf("total_cost_usd = %f, want 0", resp.Msg.TotalCostUsd)
	}
}

// ============================================================================
// SC-8: GetCostReport filters by user_id and project_id
// ============================================================================

func TestGetCostReport_FilterByUserID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	userID := "user-alice"
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			UserId: &userID,
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// alice's costs: 50 + 20 = 70
	if resp.Msg.TotalCostUsd != 70.00 {
		t.Errorf("total_cost_usd = %f, want 70", resp.Msg.TotalCostUsd)
	}
}

func TestGetCostReport_FilterByProjectID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			ProjectId: "proj-orc",
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// proj-orc: 50 + 20 = 70
	if resp.Msg.TotalCostUsd != 70.00 {
		t.Errorf("total_cost_usd = %f, want 70", resp.Msg.TotalCostUsd)
	}
}

func TestGetCostReport_EmptyFilters_ReturnAll(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if resp.Msg.TotalCostUsd != 100.00 {
		t.Errorf("total_cost_usd = %f, want 100", resp.Msg.TotalCostUsd)
	}
}

// ============================================================================
// SC-9: GetCostReport groups by dimension
// ============================================================================

func TestGetCostReport_GroupByUser(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	groupBy := "user"
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			GroupBy: &groupBy,
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if len(resp.Msg.Breakdowns) < 2 {
		t.Fatalf("expected at least 2 breakdowns, got %d", len(resp.Msg.Breakdowns))
	}

	byKey := make(map[string]float64)
	for _, b := range resp.Msg.Breakdowns {
		byKey[b.Key] = b.CostUsd
	}

	if byKey["user-alice"] != 70.00 {
		t.Errorf("alice = %f, want 70", byKey["user-alice"])
	}
	if byKey["user-bob"] != 30.00 {
		t.Errorf("bob = %f, want 30", byKey["user-bob"])
	}
}

func TestGetCostReport_GroupByProject(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	groupBy := "project"
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			GroupBy: &groupBy,
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range resp.Msg.Breakdowns {
		byKey[b.Key] = b.CostUsd
	}

	// proj-orc: 50 + 20 = 70
	if byKey["proj-orc"] != 70.00 {
		t.Errorf("proj-orc = %f, want 70", byKey["proj-orc"])
	}
	// proj-llmkit: 30
	if byKey["proj-llmkit"] != 30.00 {
		t.Errorf("proj-llmkit = %f, want 30", byKey["proj-llmkit"])
	}
}

func TestGetCostReport_GroupByModel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	groupBy := "model"
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			GroupBy: &groupBy,
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range resp.Msg.Breakdowns {
		byKey[b.Key] = b.CostUsd
	}

	// opus: 50, sonnet: 20, haiku: 30
	if byKey["opus"] != 50.00 {
		t.Errorf("opus = %f, want 50", byKey["opus"])
	}
	if byKey["sonnet"] != 20.00 {
		t.Errorf("sonnet = %f, want 20", byKey["sonnet"])
	}
	if byKey["haiku"] != 30.00 {
		t.Errorf("haiku = %f, want 30", byKey["haiku"])
	}
}

func TestGetCostReport_GroupByProvider(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed cost data with different providers
	entries := []db.CostEntry{
		// claude provider
		{ProjectID: "proj-orc", TaskID: "TASK-001", Phase: "implement", Model: "opus", Provider: "claude", CostUSD: 50.00, UserID: "user-alice", InputTokens: 10000, OutputTokens: 5000, TotalTokens: 15000},
		// codex provider
		{ProjectID: "proj-orc", TaskID: "TASK-002", Phase: "review", Model: "codex-mini", Provider: "codex", CostUSD: 20.00, UserID: "user-alice", InputTokens: 8000, OutputTokens: 3000, TotalTokens: 11000},
		// ollama provider
		{ProjectID: "proj-llmkit", TaskID: "TASK-003", Phase: "implement", Model: "llama3", Provider: "ollama", CostUSD: 30.00, UserID: "user-bob", InputTokens: 6000, OutputTokens: 2000, TotalTokens: 8000},
	}
	for _, e := range entries {
		if err := globalDB.RecordCostExtended(e); err != nil {
			t.Fatalf("seed cost: %v", err)
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	groupBy := "provider"
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{
			GroupBy: &groupBy,
		}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range resp.Msg.Breakdowns {
		byKey[b.Key] = b.CostUsd
	}

	// claude: 50
	if byKey["claude"] != 50.00 {
		t.Errorf("claude = %f, want 50", byKey["claude"])
	}
	// codex: 20
	if byKey["codex"] != 20.00 {
		t.Errorf("codex = %f, want 20", byKey["codex"])
	}
	// ollama: 30
	if byKey["ollama"] != 30.00 {
		t.Errorf("ollama = %f, want 30", byKey["ollama"])
	}

	if len(resp.Msg.Breakdowns) != 3 {
		t.Errorf("expected 3 breakdowns, got %d", len(resp.Msg.Breakdowns))
	}
}

func TestGetCostReport_NoGroupBy_ReturnsTotalOnly(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	seedGlobalDBCosts(t, globalDB)

	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if len(resp.Msg.Breakdowns) != 0 {
		t.Errorf("expected 0 breakdowns without group_by, got %d", len(resp.Msg.Breakdowns))
	}
}

// ============================================================================
// Failure Modes
// ============================================================================

func TestGetCostReport_NoGlobalDB_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewDashboardServerWithDiff(backend, nil, nil)
	// Intentionally NOT calling SetGlobalDB

	_, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err == nil {
		t.Fatal("expected error when GlobalDB not configured")
	}

	// Should be a CodeInternal error
	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInternal {
		t.Errorf("error code = %v, want CodeInternal", connectErr.Code())
	}
}

// ============================================================================
// Integration: SetGlobalDB wiring
// ============================================================================

func TestDashboardServer_SetGlobalDB_AllowsCostReport(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Wire via SetGlobalDB (same as server_connect.go will do)
	server := NewDashboardServerWithDiff(backend, nil, nil)
	server.SetGlobalDB(globalDB)

	// Should work without error
	resp, err := server.GetCostReport(
		context.Background(),
		connect.NewRequest(&orcv1.GetCostReportRequest{}),
	)
	if err != nil {
		t.Fatalf("GetCostReport after SetGlobalDB failed: %v", err)
	}
	if resp.Msg.TotalCostUsd != 0 {
		t.Errorf("expected 0 for empty DB, got %f", resp.Msg.TotalCostUsd)
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// seedGlobalDBCosts inserts test cost entries into the GlobalDB.
func seedGlobalDBCosts(t *testing.T, globalDB *db.GlobalDB) {
	t.Helper()

	entries := []db.CostEntry{
		// alice on proj-orc, opus
		{ProjectID: "proj-orc", TaskID: "TASK-001", Phase: "implement", Model: "opus", CostUSD: 50.00, UserID: "user-alice", InputTokens: 10000, OutputTokens: 5000, TotalTokens: 15000},
		// alice on proj-orc, sonnet
		{ProjectID: "proj-orc", TaskID: "TASK-002", Phase: "review", Model: "sonnet", CostUSD: 20.00, UserID: "user-alice", InputTokens: 8000, OutputTokens: 3000, TotalTokens: 11000},
		// bob on proj-llmkit, haiku
		{ProjectID: "proj-llmkit", TaskID: "TASK-003", Phase: "implement", Model: "haiku", CostUSD: 30.00, UserID: "user-bob", InputTokens: 6000, OutputTokens: 2000, TotalTokens: 8000},
	}
	for _, e := range entries {
		if err := globalDB.RecordCostExtended(e); err != nil {
			t.Fatalf("seed cost: %v", err)
		}
	}
}
