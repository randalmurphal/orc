package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDetectModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4-5-20251101", "opus"},
		{"claude-3-opus-20240229", "opus"},
		{"anthropic.claude-opus-4", "opus"},
		{"claude-sonnet-4-20250514", "sonnet"},
		{"claude-3-5-sonnet-20241022", "sonnet"},
		{"claude-3-haiku-20240307", "haiku"},
		{"claude-haiku-3-5", "haiku"},
		{"gpt-4-turbo", "unknown"},
		{"", "unknown"},
		{"CLAUDE-OPUS-4", "opus"},   // case insensitive
		{"Claude-Sonnet", "sonnet"}, // case insensitive
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := DetectModel(tc.input)
			if got != tc.expected {
				t.Errorf("DetectModel(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestMigration002_AppliesCleanly(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Migrate global schema (both 001 and 002)
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify new columns exist in cost_log
	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cost_log')
		WHERE name IN ('model', 'iteration', 'cache_creation_tokens', 'cache_read_tokens', 'total_tokens', 'initiative_id')
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check columns: %v", err)
	}
	if colCount != 6 {
		t.Errorf("new columns count = %d, want 6", colCount)
	}

	// Verify new tables exist
	tables := []string{"cost_aggregates", "cost_budgets"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not created: %v", table, err)
		}
	}

	// Verify indexes exist
	indexes := []string{"idx_cost_model", "idx_cost_model_timestamp", "idx_cost_initiative"}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %s not created: %v", idx, err)
		}
	}
}

func TestMigration003_AddsDurationMs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Migrate global schema (001, 002, 003)
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify duration_ms column exists in cost_log
	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cost_log')
		WHERE name = 'duration_ms'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("duration_ms column count = %d, want 1", colCount)
	}

	// Verify index exists
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_cost_duration'").Scan(&name)
	if err != nil {
		t.Errorf("index idx_cost_duration not created: %v", err)
	}
}

func TestMigration002_Idempotent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Run migration twice
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("First Migrate failed: %v", err)
	}
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Second Migrate failed: %v", err)
	}
}

func TestMigration002_PreservesData(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Migrate to get schema
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Insert data using old method
	if err := gdb.RecordCost("proj-1", "TASK-001", "implement", 0.05, 1000, 500); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}

	// Verify old data is preserved (model should be empty string)
	var model string
	err = db.QueryRow("SELECT model FROM cost_log WHERE task_id = ?", "TASK-001").Scan(&model)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if model != "" {
		t.Errorf("model = %q, want empty string", model)
	}

	// Get summary should still work
	since := time.Now().Add(-1 * time.Hour)
	summary, err := gdb.GetCostSummary("proj-1", since)
	if err != nil {
		t.Fatalf("GetCostSummary failed: %v", err)
	}
	if summary.TotalCostUSD != 0.05 {
		t.Errorf("TotalCostUSD = %f, want 0.05", summary.TotalCostUSD)
	}
}

func TestRecordCostExtended_AllFields(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Record with all fields
	entry := CostEntry{
		ProjectID:           "proj-1",
		TaskID:              "TASK-001",
		Phase:               "implement",
		Model:               "opus",
		Iteration:           2,
		CostUSD:             0.15,
		InputTokens:         2000,
		OutputTokens:        1000,
		CacheCreationTokens: 500,
		CacheReadTokens:     300,
		TotalTokens:         3800,
		InitiativeID:        "INIT-001",
		DurationMs:          45678, // 45.678 seconds
	}

	if err := gdb.RecordCostExtended(entry); err != nil {
		t.Fatalf("RecordCostExtended failed: %v", err)
	}

	// Verify all fields were stored
	var (
		model, phase, initID                     string
		iteration, cacheCreate, cacheRead, total int
		durationMs                               int64
		cost                                     float64
	)
	err = db.QueryRow(`
		SELECT model, phase, iteration, cost_usd, cache_creation_tokens,
			   cache_read_tokens, total_tokens, initiative_id, duration_ms
		FROM cost_log WHERE task_id = ?
	`, "TASK-001").Scan(&model, &phase, &iteration, &cost, &cacheCreate, &cacheRead, &total, &initID, &durationMs)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if model != "opus" {
		t.Errorf("model = %q, want opus", model)
	}
	if iteration != 2 {
		t.Errorf("iteration = %d, want 2", iteration)
	}
	if cost != 0.15 {
		t.Errorf("cost = %f, want 0.15", cost)
	}
	if cacheCreate != 500 {
		t.Errorf("cache_creation_tokens = %d, want 500", cacheCreate)
	}
	if cacheRead != 300 {
		t.Errorf("cache_read_tokens = %d, want 300", cacheRead)
	}
	if total != 3800 {
		t.Errorf("total_tokens = %d, want 3800", total)
	}
	if initID != "INIT-001" {
		t.Errorf("initiative_id = %q, want INIT-001", initID)
	}
	if durationMs != 45678 {
		t.Errorf("duration_ms = %d, want 45678", durationMs)
	}
}

func TestGetCostByModel_GroupsCorrectly(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Record costs for different models
	entries := []CostEntry{
		{ProjectID: "proj-1", TaskID: "TASK-001", Phase: "implement", Model: "opus", CostUSD: 0.50},
		{ProjectID: "proj-1", TaskID: "TASK-002", Phase: "implement", Model: "opus", CostUSD: 0.30},
		{ProjectID: "proj-1", TaskID: "TASK-003", Phase: "test", Model: "sonnet", CostUSD: 0.10},
		{ProjectID: "proj-1", TaskID: "TASK-004", Phase: "review", Model: "haiku", CostUSD: 0.02},
		{ProjectID: "proj-2", TaskID: "TASK-005", Phase: "implement", Model: "opus", CostUSD: 0.40},
	}

	for _, e := range entries {
		if err := gdb.RecordCostExtended(e); err != nil {
			t.Fatalf("RecordCostExtended failed: %v", err)
		}
	}

	// Get cost by model for proj-1
	since := time.Now().Add(-1 * time.Hour)
	costs, err := gdb.GetCostByModel("proj-1", since)
	if err != nil {
		t.Fatalf("GetCostByModel failed: %v", err)
	}

	if costs["opus"] != 0.80 {
		t.Errorf("opus cost = %f, want 0.80", costs["opus"])
	}
	if costs["sonnet"] != 0.10 {
		t.Errorf("sonnet cost = %f, want 0.10", costs["sonnet"])
	}
	if costs["haiku"] != 0.02 {
		t.Errorf("haiku cost = %f, want 0.02", costs["haiku"])
	}

	// Get cost by model for all projects
	allCosts, err := gdb.GetCostByModel("", since)
	if err != nil {
		t.Fatalf("GetCostByModel all failed: %v", err)
	}

	if allCosts["opus"] != 1.20 {
		t.Errorf("all opus cost = %f, want 1.20", allCosts["opus"])
	}
}

func TestGetCostTimeseries_DailyGranularity(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Record costs
	entries := []CostEntry{
		{ProjectID: "proj-1", TaskID: "TASK-001", Model: "opus", CostUSD: 0.50, InputTokens: 1000, OutputTokens: 500},
		{ProjectID: "proj-1", TaskID: "TASK-002", Model: "sonnet", CostUSD: 0.10, InputTokens: 500, OutputTokens: 250},
	}

	for _, e := range entries {
		if err := gdb.RecordCostExtended(e); err != nil {
			t.Fatalf("RecordCostExtended failed: %v", err)
		}
	}

	// Get timeseries
	since := time.Now().Add(-1 * time.Hour)
	ts, err := gdb.GetCostTimeseries("proj-1", since, "day")
	if err != nil {
		t.Fatalf("GetCostTimeseries failed: %v", err)
	}

	if len(ts) < 1 {
		t.Fatalf("len(ts) = %d, want >= 1", len(ts))
	}

	// Should have data grouped by model
	var totalCost float64
	for _, agg := range ts {
		totalCost += agg.TotalCostUSD
	}
	if totalCost != 0.60 {
		t.Errorf("total cost = %f, want 0.60", totalCost)
	}
}

func TestGetCostTimeseries_Granularities(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Record a cost entry
	entry := CostEntry{
		ProjectID: "proj-1",
		TaskID:    "TASK-001",
		Model:     "opus",
		CostUSD:   0.50,
	}
	if err := gdb.RecordCostExtended(entry); err != nil {
		t.Fatalf("RecordCostExtended failed: %v", err)
	}

	since := time.Now().Add(-1 * time.Hour)

	// Test day granularity
	dayTS, err := gdb.GetCostTimeseries("proj-1", since, "day")
	if err != nil {
		t.Fatalf("GetCostTimeseries day failed: %v", err)
	}
	if len(dayTS) == 0 {
		t.Error("day timeseries empty")
	}

	// Test week granularity
	weekTS, err := gdb.GetCostTimeseries("proj-1", since, "week")
	if err != nil {
		t.Fatalf("GetCostTimeseries week failed: %v", err)
	}
	if len(weekTS) == 0 {
		t.Error("week timeseries empty")
	}

	// Test month granularity
	monthTS, err := gdb.GetCostTimeseries("proj-1", since, "month")
	if err != nil {
		t.Fatalf("GetCostTimeseries month failed: %v", err)
	}
	if len(monthTS) == 0 {
		t.Error("month timeseries empty")
	}
}

func TestCostAggregate_Upsert(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Insert aggregate
	agg := CostAggregate{
		ProjectID:         "proj-1",
		Model:             "opus",
		Phase:             "implement",
		Date:              "2025-01-15",
		TotalCostUSD:      1.50,
		TotalInputTokens:  5000,
		TotalOutputTokens: 2500,
		TotalCacheTokens:  1000,
		TurnCount:         10,
		TaskCount:         3,
	}

	if err := gdb.UpdateCostAggregate(agg); err != nil {
		t.Fatalf("UpdateCostAggregate insert failed: %v", err)
	}

	// Verify insert
	aggs, err := gdb.GetCostAggregates("proj-1", "2025-01-01", "2025-01-31")
	if err != nil {
		t.Fatalf("GetCostAggregates failed: %v", err)
	}
	if len(aggs) != 1 {
		t.Fatalf("len(aggs) = %d, want 1", len(aggs))
	}
	if aggs[0].TotalCostUSD != 1.50 {
		t.Errorf("TotalCostUSD = %f, want 1.50", aggs[0].TotalCostUSD)
	}

	// Update aggregate (upsert)
	agg.TotalCostUSD = 2.00
	agg.TurnCount = 15
	if err := gdb.UpdateCostAggregate(agg); err != nil {
		t.Fatalf("UpdateCostAggregate update failed: %v", err)
	}

	// Verify update
	aggs2, _ := gdb.GetCostAggregates("proj-1", "2025-01-01", "2025-01-31")
	if len(aggs2) != 1 {
		t.Fatalf("len(aggs) after upsert = %d, want 1", len(aggs2))
	}
	if aggs2[0].TotalCostUSD != 2.00 {
		t.Errorf("TotalCostUSD after upsert = %f, want 2.00", aggs2[0].TotalCostUSD)
	}
	if aggs2[0].TurnCount != 15 {
		t.Errorf("TurnCount after upsert = %d, want 15", aggs2[0].TurnCount)
	}
}

func TestBudget_CRUD(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create budget
	budget := CostBudget{
		ProjectID:             "proj-1",
		MonthlyLimitUSD:       100.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          "2025-01",
		CurrentMonthSpent:     25.00,
	}

	if err := gdb.SetBudget(budget); err != nil {
		t.Fatalf("SetBudget create failed: %v", err)
	}

	// Read budget
	got, err := gdb.GetBudget("proj-1")
	if err != nil {
		t.Fatalf("GetBudget failed: %v", err)
	}
	if got.MonthlyLimitUSD != 100.00 {
		t.Errorf("MonthlyLimitUSD = %f, want 100.00", got.MonthlyLimitUSD)
	}
	if got.AlertThresholdPercent != 80 {
		t.Errorf("AlertThresholdPercent = %d, want 80", got.AlertThresholdPercent)
	}
	if got.CurrentMonth != "2025-01" {
		t.Errorf("CurrentMonth = %q, want 2025-01", got.CurrentMonth)
	}
	if got.CurrentMonthSpent != 25.00 {
		t.Errorf("CurrentMonthSpent = %f, want 25.00", got.CurrentMonthSpent)
	}

	// Update budget
	budget.MonthlyLimitUSD = 150.00
	budget.CurrentMonthSpent = 50.00
	if err := gdb.SetBudget(budget); err != nil {
		t.Fatalf("SetBudget update failed: %v", err)
	}

	got2, _ := gdb.GetBudget("proj-1")
	if got2.MonthlyLimitUSD != 150.00 {
		t.Errorf("MonthlyLimitUSD after update = %f, want 150.00", got2.MonthlyLimitUSD)
	}
	if got2.CurrentMonthSpent != 50.00 {
		t.Errorf("CurrentMonthSpent after update = %f, want 50.00", got2.CurrentMonthSpent)
	}

	// Read non-existent budget - should return (nil, nil), not error
	notFound, err := gdb.GetBudget("nonexistent-project")
	if err != nil {
		t.Errorf("GetBudget for nonexistent project returned error: %v", err)
	}
	if notFound != nil {
		t.Errorf("GetBudget for nonexistent project returned %v, want nil", notFound)
	}
}

func TestBudgetStatus(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	currentMonth := time.Now().UTC().Format("2006-01")

	// Create budget for current month
	budget := CostBudget{
		ProjectID:             "proj-1",
		MonthlyLimitUSD:       100.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth,
		CurrentMonthSpent:     85.00,
	}

	if err := gdb.SetBudget(budget); err != nil {
		t.Fatalf("SetBudget failed: %v", err)
	}

	// Get budget status
	status, err := gdb.GetBudgetStatus("proj-1")
	if err != nil {
		t.Fatalf("GetBudgetStatus failed: %v", err)
	}

	if status.MonthlyLimitUSD != 100.00 {
		t.Errorf("MonthlyLimitUSD = %f, want 100.00", status.MonthlyLimitUSD)
	}
	if status.CurrentMonthSpent != 85.00 {
		t.Errorf("CurrentMonthSpent = %f, want 85.00", status.CurrentMonthSpent)
	}
	if status.PercentUsed != 85.00 {
		t.Errorf("PercentUsed = %f, want 85.00", status.PercentUsed)
	}
	if !status.AtAlertThreshold {
		t.Error("AtAlertThreshold = false, want true (85% >= 80%)")
	}
	if status.OverBudget {
		t.Error("OverBudget = true, want false (85 < 100)")
	}

	// Test over budget
	budget.CurrentMonthSpent = 120.00
	_ = gdb.SetBudget(budget)

	status2, _ := gdb.GetBudgetStatus("proj-1")
	if !status2.OverBudget {
		t.Error("OverBudget = false, want true (120 > 100)")
	}
	if status2.PercentUsed != 120.00 {
		t.Errorf("PercentUsed = %f, want 120.00", status2.PercentUsed)
	}

	// Test GetBudgetStatus for nonexistent project - should return (nil, nil)
	statusNotFound, err := gdb.GetBudgetStatus("nonexistent-project")
	if err != nil {
		t.Errorf("GetBudgetStatus for nonexistent project returned error: %v", err)
	}
	if statusNotFound != nil {
		t.Errorf("GetBudgetStatus for nonexistent project returned %v, want nil", statusNotFound)
	}
}

func TestGlobalDB_CostWorkflow(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// End-to-end workflow: record costs, get by model, get timeseries, manage budget

	// 1. Record costs using extended method
	entries := []CostEntry{
		{ProjectID: "proj-1", TaskID: "TASK-001", Phase: "implement", Model: "opus", CostUSD: 0.50, InputTokens: 2000, OutputTokens: 1000, CacheCreationTokens: 100, CacheReadTokens: 50, TotalTokens: 3150, InitiativeID: "INIT-001"},
		{ProjectID: "proj-1", TaskID: "TASK-001", Phase: "test", Model: "sonnet", CostUSD: 0.05, InputTokens: 500, OutputTokens: 250, TotalTokens: 750},
		{ProjectID: "proj-1", TaskID: "TASK-002", Phase: "implement", Model: "opus", CostUSD: 0.30, InputTokens: 1500, OutputTokens: 800, TotalTokens: 2300, InitiativeID: "INIT-001"},
	}

	for _, e := range entries {
		if err := gdb.RecordCostExtended(e); err != nil {
			t.Fatalf("RecordCostExtended failed: %v", err)
		}
	}

	// 2. Verify GetCostSummary still works (backward compatibility)
	since := time.Now().Add(-1 * time.Hour)
	summary, err := gdb.GetCostSummary("proj-1", since)
	if err != nil {
		t.Fatalf("GetCostSummary failed: %v", err)
	}
	if summary.TotalCostUSD != 0.85 {
		t.Errorf("TotalCostUSD = %f, want 0.85", summary.TotalCostUSD)
	}
	if summary.EntryCount != 3 {
		t.Errorf("EntryCount = %d, want 3", summary.EntryCount)
	}

	// 3. Get cost by model
	byModel, err := gdb.GetCostByModel("proj-1", since)
	if err != nil {
		t.Fatalf("GetCostByModel failed: %v", err)
	}
	if byModel["opus"] != 0.80 {
		t.Errorf("opus cost = %f, want 0.80", byModel["opus"])
	}
	if byModel["sonnet"] != 0.05 {
		t.Errorf("sonnet cost = %f, want 0.05", byModel["sonnet"])
	}

	// 4. Get timeseries
	ts, err := gdb.GetCostTimeseries("proj-1", since, "day")
	if err != nil {
		t.Fatalf("GetCostTimeseries failed: %v", err)
	}
	if len(ts) < 1 {
		t.Errorf("timeseries empty")
	}

	// 5. Create and use aggregates
	agg := CostAggregate{
		ProjectID:         "proj-1",
		Model:             "opus",
		Phase:             "implement",
		Date:              time.Now().Format("2006-01-02"),
		TotalCostUSD:      0.80,
		TotalInputTokens:  3500,
		TotalOutputTokens: 1800,
		TotalCacheTokens:  150,
		TurnCount:         2,
		TaskCount:         2,
	}
	if err := gdb.UpdateCostAggregate(agg); err != nil {
		t.Fatalf("UpdateCostAggregate failed: %v", err)
	}

	// 6. Set up budget
	currentMonth := time.Now().UTC().Format("2006-01")
	budget := CostBudget{
		ProjectID:             "proj-1",
		MonthlyLimitUSD:       50.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth,
		CurrentMonthSpent:     0.85,
	}
	if err := gdb.SetBudget(budget); err != nil {
		t.Fatalf("SetBudget failed: %v", err)
	}

	// 7. Check budget status
	status, err := gdb.GetBudgetStatus("proj-1")
	if err != nil {
		t.Fatalf("GetBudgetStatus failed: %v", err)
	}
	if status.OverBudget {
		t.Error("should not be over budget")
	}
	expectedPercent := (0.85 / 50.00) * 100
	// Use tolerance for float comparison
	if diff := status.PercentUsed - expectedPercent; diff < -0.01 || diff > 0.01 {
		t.Errorf("PercentUsed = %f, want %f", status.PercentUsed, expectedPercent)
	}
}
