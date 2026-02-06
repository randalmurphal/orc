package db

import (
	"path/filepath"
	"testing"
	"time"
)

// ============================================================================
// SC-7, SC-8, SC-9: GetCostReport database query function
//
// Tests the new GlobalDB.GetCostReport() method that supports:
// - Filtering by user_id, project_id, and since timestamp
// - Grouping by user, project, or model dimensions
// - Returning aggregated cost breakdowns
// ============================================================================

// CostReportFilter defines the filters for cost report queries.
// This struct will be implemented in global.go.
// type CostReportFilter struct {
//     UserID    string
//     ProjectID string
//     Since     time.Time
//     GroupBy   string // "user", "project", "model", or "" for total only
// }

// CostReportResult contains the aggregated cost report data.
// type CostReportResult struct {
//     TotalCostUSD float64
//     Breakdowns   []CostBreakdownEntry
// }

// CostBreakdownEntry represents a single breakdown entry in the cost report.
// type CostBreakdownEntry struct {
//     Key     string
//     CostUSD float64
// }

// newTestGlobalDB creates a GlobalDB backed by a temp file for testing.
func newTestGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	rawDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := rawDB.Migrate("global"); err != nil {
		_ = rawDB.Close()
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: rawDB}
	t.Cleanup(func() { _ = gdb.Close() })
	return gdb
}

// seedCostData inserts test cost entries for cost report tests.
func seedCostData(t *testing.T, gdb *GlobalDB) {
	t.Helper()
	entries := []CostEntry{
		// alice on project orc, opus model
		{ProjectID: "proj-orc", TaskID: "TASK-001", Phase: "implement", Model: "opus", CostUSD: 30.00, UserID: "user-alice", InputTokens: 10000, OutputTokens: 5000, TotalTokens: 15000},
		{ProjectID: "proj-orc", TaskID: "TASK-002", Phase: "review", Model: "sonnet", CostUSD: 20.00, UserID: "user-alice", InputTokens: 8000, OutputTokens: 3000, TotalTokens: 11000},
		// bob on project orc, sonnet model
		{ProjectID: "proj-orc", TaskID: "TASK-003", Phase: "implement", Model: "sonnet", CostUSD: 15.00, UserID: "user-bob", InputTokens: 6000, OutputTokens: 2000, TotalTokens: 8000},
		// alice on project llmkit, haiku model
		{ProjectID: "proj-llmkit", TaskID: "TASK-004", Phase: "spec", Model: "haiku", CostUSD: 5.00, UserID: "user-alice", InputTokens: 2000, OutputTokens: 1000, TotalTokens: 3000},
		// bob on project llmkit, opus model
		{ProjectID: "proj-llmkit", TaskID: "TASK-005", Phase: "implement", Model: "opus", CostUSD: 25.00, UserID: "user-bob", InputTokens: 9000, OutputTokens: 4000, TotalTokens: 13000},
		// entry with no user_id (legacy pre-TASK-786 data)
		{ProjectID: "proj-orc", TaskID: "TASK-006", Phase: "implement", Model: "sonnet", CostUSD: 10.00, UserID: "", InputTokens: 4000, OutputTokens: 2000, TotalTokens: 6000},
		// entry with empty model
		{ProjectID: "proj-orc", TaskID: "TASK-007", Phase: "docs", Model: "", CostUSD: 2.00, UserID: "user-alice", InputTokens: 1000, OutputTokens: 500, TotalTokens: 1500},
	}
	for _, e := range entries {
		if err := gdb.RecordCostExtended(e); err != nil {
			t.Fatalf("seed cost data: %v", err)
		}
	}
}

// --- SC-7: GetCostReport returns aggregated data ---

func TestGetCostReport_TotalOnly_NoFilters(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Total should be sum of all entries: 30 + 20 + 15 + 5 + 25 + 10 + 2 = 107
	expectedTotal := 107.00
	if result.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, expectedTotal)
	}

	// Without GroupBy, no breakdowns
	if len(result.Breakdowns) != 0 {
		t.Errorf("expected 0 breakdowns without GroupBy, got %d", len(result.Breakdowns))
	}
}

func TestGetCostReport_EmptyDatabase_ReturnsZero(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if result.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", result.TotalCostUSD)
	}
}

// --- SC-8: Filter by user_id and project_id ---

func TestGetCostReport_FilterByUserID(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		UserID: "user-alice",
		Since:  time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// alice's costs: 30 + 20 + 5 + 2 = 57
	expectedTotal := 57.00
	if result.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, expectedTotal)
	}
}

func TestGetCostReport_FilterByProjectID(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		ProjectID: "proj-orc",
		Since:     time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// proj-orc costs: 30 + 20 + 15 + 10 + 2 = 77
	expectedTotal := 77.00
	if result.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, expectedTotal)
	}
}

func TestGetCostReport_FilterByUserAndProject(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		UserID:    "user-alice",
		ProjectID: "proj-orc",
		Since:     time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// alice on proj-orc: 30 + 20 + 2 = 52
	expectedTotal := 52.00
	if result.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, expectedTotal)
	}
}

func TestGetCostReport_FilterBySinceDate(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	// Future date should return 0
	result, err := gdb.GetCostReport(CostReportFilter{
		Since: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if result.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD with future since = %f, want 0", result.TotalCostUSD)
	}
}

func TestGetCostReport_EmptyFilters_ReturnsAll(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Should return all data (107)
	expectedTotal := 107.00
	if result.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, expectedTotal)
	}
}

// --- SC-9: Group by user, project, model ---

func TestGetCostReport_GroupByUser(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "user",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Should have 3 groups: user-alice, user-bob, unattributed (empty user_id)
	if len(result.Breakdowns) < 2 {
		t.Fatalf("expected at least 2 breakdowns by user, got %d", len(result.Breakdowns))
	}

	// Build a lookup map
	byKey := make(map[string]float64)
	for _, b := range result.Breakdowns {
		byKey[b.Key] = b.CostUSD
	}

	// alice: 30 + 20 + 5 + 2 = 57
	if byKey["user-alice"] != 57.00 {
		t.Errorf("alice cost = %f, want 57", byKey["user-alice"])
	}
	// bob: 15 + 25 = 40
	if byKey["user-bob"] != 40.00 {
		t.Errorf("bob cost = %f, want 40", byKey["user-bob"])
	}
	// unattributed (empty user_id entry with key "unattributed"): 10
	if byKey["unattributed"] != 10.00 {
		t.Errorf("unattributed cost = %f, want 10", byKey["unattributed"])
	}
}

func TestGetCostReport_GroupByProject(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "project",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range result.Breakdowns {
		byKey[b.Key] = b.CostUSD
	}

	// proj-orc: 30 + 20 + 15 + 10 + 2 = 77
	if byKey["proj-orc"] != 77.00 {
		t.Errorf("proj-orc cost = %f, want 77", byKey["proj-orc"])
	}
	// proj-llmkit: 5 + 25 = 30
	if byKey["proj-llmkit"] != 30.00 {
		t.Errorf("proj-llmkit cost = %f, want 30", byKey["proj-llmkit"])
	}
}

func TestGetCostReport_GroupByModel(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "model",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range result.Breakdowns {
		byKey[b.Key] = b.CostUSD
	}

	// opus: 30 + 25 = 55
	if byKey["opus"] != 55.00 {
		t.Errorf("opus cost = %f, want 55", byKey["opus"])
	}
	// sonnet: 20 + 15 + 10 = 45
	if byKey["sonnet"] != 45.00 {
		t.Errorf("sonnet cost = %f, want 45", byKey["sonnet"])
	}
	// haiku: 5
	if byKey["haiku"] != 5.00 {
		t.Errorf("haiku cost = %f, want 5", byKey["haiku"])
	}
	// unknown (empty model): 2
	if byKey["unknown"] != 2.00 {
		t.Errorf("unknown cost = %f, want 2", byKey["unknown"])
	}
}

// --- Edge Cases ---

func TestGetCostReport_GroupByUserWithAllUnattributed(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)

	// Insert entries with no user_id
	for i := 0; i < 3; i++ {
		if err := gdb.RecordCostExtended(CostEntry{
			ProjectID: "proj-1", TaskID: "TASK-001", Phase: "implement",
			Model: "sonnet", CostUSD: 10.00, UserID: "",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	result, err := gdb.GetCostReport(CostReportFilter{
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "user",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Should have exactly 1 group: "unattributed"
	if len(result.Breakdowns) != 1 {
		t.Fatalf("expected 1 breakdown (all unattributed), got %d", len(result.Breakdowns))
	}
	if result.Breakdowns[0].Key != "unattributed" {
		t.Errorf("key = %q, want 'unattributed'", result.Breakdowns[0].Key)
	}
	if result.Breakdowns[0].CostUSD != 30.00 {
		t.Errorf("cost = %f, want 30", result.Breakdowns[0].CostUSD)
	}
}

func TestGetCostReport_GroupByModelWithEmptyModel(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)

	// Insert entries with empty model
	if err := gdb.RecordCostExtended(CostEntry{
		ProjectID: "proj-1", TaskID: "TASK-001", Phase: "implement",
		Model: "", CostUSD: 8.00, UserID: "user-1",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	result, err := gdb.GetCostReport(CostReportFilter{
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "model",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	// Empty model should appear as "unknown"
	if len(result.Breakdowns) != 1 {
		t.Fatalf("expected 1 breakdown, got %d", len(result.Breakdowns))
	}
	if result.Breakdowns[0].Key != "unknown" {
		t.Errorf("key = %q, want 'unknown'", result.Breakdowns[0].Key)
	}
}

func TestGetCostReport_GroupByAndFilterCombined(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	// Group by model but filter to only alice's costs
	result, err := gdb.GetCostReport(CostReportFilter{
		UserID:  "user-alice",
		Since:   time.Now().Add(-1 * time.Hour),
		GroupBy: "model",
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	byKey := make(map[string]float64)
	for _, b := range result.Breakdowns {
		byKey[b.Key] = b.CostUSD
	}

	// alice's opus: 30
	if byKey["opus"] != 30.00 {
		t.Errorf("alice opus = %f, want 30", byKey["opus"])
	}
	// alice's sonnet: 20
	if byKey["sonnet"] != 20.00 {
		t.Errorf("alice sonnet = %f, want 20", byKey["sonnet"])
	}
	// alice's haiku: 5
	if byKey["haiku"] != 5.00 {
		t.Errorf("alice haiku = %f, want 5", byKey["haiku"])
	}
	// alice's unknown (empty model): 2
	if byKey["unknown"] != 2.00 {
		t.Errorf("alice unknown = %f, want 2", byKey["unknown"])
	}

	// Total should be 57
	if result.TotalCostUSD != 57.00 {
		t.Errorf("total = %f, want 57", result.TotalCostUSD)
	}
}

func TestGetCostReport_NoGroupBy_ReturnsNoBreakdowns(t *testing.T) {
	t.Parallel()
	gdb := newTestGlobalDB(t)
	seedCostData(t, gdb)

	result, err := gdb.GetCostReport(CostReportFilter{
		Since: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GetCostReport failed: %v", err)
	}

	if len(result.Breakdowns) != 0 {
		t.Errorf("expected empty breakdowns without GroupBy, got %d entries", len(result.Breakdowns))
	}
}
