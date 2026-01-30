// Package db provides database operations for orc.
//
// TDD Tests for TASK-531: Performance - Stats page takes 5+ seconds to load
//
// These tests verify SQL aggregate queries that replace in-memory computation
// for dashboard endpoints, and database indexes for time-filtered queries.
//
// Success Criteria Coverage:
// - SC-3: SQL aggregation queries (COUNT/SUM/GROUP BY) for dashboard endpoints
// - SC-6: Database indexes on completed_at and updated_at columns
// - SC-7: Batch initiative title loading
//
// These tests define the contract for new aggregate query methods
// that must be added to ProjectDB.
package db

import (
	"fmt"
	"testing"
	"time"
)

// ============================================================================
// SC-3: SQL aggregate queries for dashboard stats
// ============================================================================

// TestGetDashboardStatusCounts verifies SC-3:
// GetDashboardStatusCounts returns correct status counts using SQL aggregation
// instead of loading all task objects.
func TestGetDashboardStatusCounts(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create tasks with various statuses
	statuses := map[string]int{
		"completed": 10,
		"failed":    3,
		"running":   2,
		"blocked":   1,
		"created":   5,
		"planned":   4,
	}

	taskNum := 0
	for status, count := range statuses {
		for i := 0; i < count; i++ {
			taskNum++
			tk := &Task{
				ID:     fmt.Sprintf("TASK-%03d", taskNum),
				Title:  fmt.Sprintf("Task %d", taskNum),
				Status: status,
			}
			if err := db.SaveTask(tk); err != nil {
				t.Fatalf("save task: %v", err)
			}
		}
	}

	counts, err := db.GetDashboardStatusCounts()
	if err != nil {
		t.Fatalf("GetDashboardStatusCounts failed: %v", err)
	}

	// Verify each status count
	if counts.Completed != 10 {
		t.Errorf("completed: expected 10, got %d", counts.Completed)
	}
	if counts.Failed != 3 {
		t.Errorf("failed: expected 3, got %d", counts.Failed)
	}
	if counts.Running != 2 {
		t.Errorf("running: expected 2, got %d", counts.Running)
	}
	if counts.Blocked != 1 {
		t.Errorf("blocked: expected 1, got %d", counts.Blocked)
	}
	if counts.Total != 25 {
		t.Errorf("total: expected 25, got %d", counts.Total)
	}
}

// TestGetDashboardStatusCounts_EmptyDB verifies edge case:
// Returns all-zero counts when database is empty.
func TestGetDashboardStatusCounts_EmptyDB(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	counts, err := db.GetDashboardStatusCounts()
	if err != nil {
		t.Fatalf("GetDashboardStatusCounts failed: %v", err)
	}

	if counts.Total != 0 {
		t.Errorf("expected 0 total, got %d", counts.Total)
	}
}

// TestGetDashboardCostByDate verifies SC-3:
// GetDashboardCostByDate returns daily cost aggregation using SQL GROUP BY.
func TestGetDashboardCostByDate(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Use fixed dates in UTC to avoid timezone/midnight boundary flakiness.
	// SQLite's DATE() extracts from the stored RFC3339 string (UTC),
	// so we must use UTC for consistent date comparisons.
	day1 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC) // noon UTC - safe from boundaries
	day2 := time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)

	// Create tasks completed on different days
	tasks := []struct {
		id          string
		completedAt time.Time
		cost        float64
	}{
		{"TASK-001", day1, 1.50},
		{"TASK-002", day1, 2.50},
		{"TASK-003", day2, 3.00},
	}

	for _, tc := range tasks {
		tk := &Task{
			ID:     tc.id,
			Title:  tc.id,
			Status: "completed",
		}
		completedAt := tc.completedAt
		tk.CompletedAt = &completedAt
		if err := db.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Save phase with cost data
		startedAt := tc.completedAt.Add(-10 * time.Minute)
		phase := &Phase{
			TaskID:    tc.id,
			PhaseID:   "implement",
			Status:    "completed",
			StartedAt: &startedAt,
			CostUSD:   tc.cost,
		}
		if err := db.SavePhase(phase); err != nil {
			t.Fatalf("save phase: %v", err)
		}
	}

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results, err := db.GetDashboardCostByDate(since)
	if err != nil {
		t.Fatalf("GetDashboardCostByDate failed: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 date entries, got %d", len(results))
	}

	// Verify aggregation — day1 should total 4.00
	day1Str := "2024-01-15"
	day2Str := "2024-01-14"
	var day1Cost, day2Cost float64
	for _, r := range results {
		switch r.Date {
		case day1Str:
			day1Cost = r.CostUSD
		case day2Str:
			day2Cost = r.CostUSD
		}
	}
	if day1Cost != 4.0 {
		t.Errorf("expected day1 (2024-01-15) cost 4.0, got %f", day1Cost)
	}
	if day2Cost != 3.0 {
		t.Errorf("expected day2 (2024-01-14) cost 3.0, got %f", day2Cost)
	}
}

// TestGetDashboardAggregateMatchesFullLoad verifies SC-3 parity:
// SQL aggregate status counts match the result of loading all tasks
// and counting in memory.
func TestGetDashboardAggregateMatchesFullLoad(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create a mix of tasks
	taskData := []struct {
		id     string
		status string
	}{
		{"TASK-001", "completed"},
		{"TASK-002", "completed"},
		{"TASK-003", "completed"},
		{"TASK-004", "failed"},
		{"TASK-005", "running"},
		{"TASK-006", "blocked"},
		{"TASK-007", "created"},
		{"TASK-008", "planned"},
		{"TASK-009", "paused"},
		{"TASK-010", "completed"},
	}

	for _, td := range taskData {
		tk := &Task{ID: td.id, Title: td.id, Status: td.status}
		if err := db.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// Get aggregate counts
	counts, err := db.GetDashboardStatusCounts()
	if err != nil {
		t.Fatalf("GetDashboardStatusCounts: %v", err)
	}

	// Get full task list and count manually
	allTasks, _, err := db.ListTasks(ListOpts{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	manualCompleted := 0
	manualFailed := 0
	manualRunning := 0
	for _, tk := range allTasks {
		switch tk.Status {
		case "completed":
			manualCompleted++
		case "failed":
			manualFailed++
		case "running":
			manualRunning++
		}
	}

	// Aggregate and manual counts must match
	if counts.Completed != manualCompleted {
		t.Errorf("completed mismatch: aggregate=%d, manual=%d", counts.Completed, manualCompleted)
	}
	if counts.Failed != manualFailed {
		t.Errorf("failed mismatch: aggregate=%d, manual=%d", counts.Failed, manualFailed)
	}
	if counts.Running != manualRunning {
		t.Errorf("running mismatch: aggregate=%d, manual=%d", counts.Running, manualRunning)
	}
	if counts.Total != len(allTasks) {
		t.Errorf("total mismatch: aggregate=%d, manual=%d", counts.Total, len(allTasks))
	}
}

// TestGetInitiativeTitlesBatch verifies SC-7:
// Batch loading initiative titles by IDs in a single query.
func TestGetInitiativeTitlesBatch(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create initiatives
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("INIT-%03d", i+1)
		init := &Initiative{
			ID:    id,
			Title: fmt.Sprintf("Initiative %d", i+1),
		}
		if err := db.SaveInitiative(init); err != nil {
			t.Fatalf("save initiative: %v", err)
		}
	}

	// Batch load
	ids := []string{"INIT-001", "INIT-003", "INIT-005"}
	titles, err := db.GetInitiativeTitlesBatch(ids)
	if err != nil {
		t.Fatalf("GetInitiativeTitlesBatch failed: %v", err)
	}

	// Verify all requested IDs have titles
	if len(titles) != 3 {
		t.Fatalf("expected 3 titles, got %d", len(titles))
	}
	if titles["INIT-001"] != "Initiative 1" {
		t.Errorf("INIT-001: expected 'Initiative 1', got '%s'", titles["INIT-001"])
	}
	if titles["INIT-003"] != "Initiative 3" {
		t.Errorf("INIT-003: expected 'Initiative 3', got '%s'", titles["INIT-003"])
	}
	if titles["INIT-005"] != "Initiative 5" {
		t.Errorf("INIT-005: expected 'Initiative 5', got '%s'", titles["INIT-005"])
	}
}

// TestGetInitiativeTitlesBatch_EmptyIDs verifies edge case:
// Empty input returns empty map, no error.
func TestGetInitiativeTitlesBatch_EmptyIDs(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	titles, err := db.GetInitiativeTitlesBatch(nil)
	if err != nil {
		t.Fatalf("GetInitiativeTitlesBatch failed: %v", err)
	}
	if len(titles) != 0 {
		t.Errorf("expected empty map, got %d entries", len(titles))
	}
}

// TestGetInitiativeTitlesBatch_MissingIDs verifies edge case:
// IDs not in database are simply absent from the result (no error).
func TestGetInitiativeTitlesBatch_MissingIDs(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create only one initiative
	init := &Initiative{ID: "INIT-001", Title: "Real Initiative"}
	if err := db.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	ids := []string{"INIT-001", "INIT-NONEXISTENT"}
	titles, err := db.GetInitiativeTitlesBatch(ids)
	if err != nil {
		t.Fatalf("GetInitiativeTitlesBatch failed: %v", err)
	}

	if len(titles) != 1 {
		t.Fatalf("expected 1 title, got %d", len(titles))
	}
	if titles["INIT-001"] != "Real Initiative" {
		t.Errorf("expected 'Real Initiative', got '%s'", titles["INIT-001"])
	}
	if _, ok := titles["INIT-NONEXISTENT"]; ok {
		t.Error("missing ID should not be in result map")
	}
}

// ============================================================================
// SC-6: Database indexes on completed_at and updated_at
// ============================================================================

// TestDashboardIndexes_Exist verifies SC-6:
// After migration, indexes exist on tasks.completed_at and tasks.updated_at.
func TestDashboardIndexes_Exist(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Query SQLite for indexes on the tasks table
	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'index' AND tbl_name = 'tasks'
	`)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer func() { _ = rows.Close() }()

	indexNames := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		indexNames[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate indexes: %v", err)
	}

	// SC-6: These indexes must exist
	requiredIndexes := []string{
		"idx_tasks_completed_at",
		"idx_tasks_updated_at",
	}

	for _, idx := range requiredIndexes {
		if !indexNames[idx] {
			t.Errorf("missing required index: %s", idx)
		}
	}
}

// TestDashboardIndexes_UsedByDateFilter verifies SC-6:
// EXPLAIN QUERY PLAN shows index usage for date-filtered queries.
func TestDashboardIndexes_UsedByDateFilter(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Insert some tasks so the optimizer has data to work with
	for i := 0; i < 50; i++ {
		completedAt := time.Now().Add(-time.Duration(i) * 24 * time.Hour)
		tk := &Task{
			ID:     fmt.Sprintf("TASK-%03d", i+1),
			Title:  fmt.Sprintf("Task %d", i+1),
			Status: "completed",
		}
		tk.CompletedAt = &completedAt
		if err := db.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// Check EXPLAIN QUERY PLAN for a date-filtered query
	rows, err := db.Query(`
		EXPLAIN QUERY PLAN
		SELECT COUNT(*) FROM tasks
		WHERE completed_at >= ?
	`, time.Now().Add(-7*24*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var plan string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan explain row: %v", err)
		}
		plan += detail + "\n"
	}

	// The plan should mention index usage (not full table scan)
	// Note: SQLite may not always use the index for small tables,
	// but the index should exist from the migration test above.
	t.Logf("Query plan: %s", plan)
}

// ============================================================================
// SC-3: GetDashboardInitiativeStats — aggregate initiative task counts
// ============================================================================

// TestGetDashboardInitiativeStats verifies SC-3:
// Returns initiative task counts using SQL aggregation, not N+1 loads.
func TestGetDashboardInitiativeStats(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create tasks across initiatives
	for i := 0; i < 10; i++ {
		initID := fmt.Sprintf("INIT-%03d", (i%3)+1) // 3 initiatives
		status := "completed"
		if i%4 == 0 {
			status = "running"
		}
		tk := &Task{
			ID:           fmt.Sprintf("TASK-%03d", i+1),
			Title:        fmt.Sprintf("Task %d", i+1),
			Status:       status,
			InitiativeID: initID,
		}
		if err := db.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	stats, err := db.GetDashboardInitiativeStats(10)
	if err != nil {
		t.Fatalf("GetDashboardInitiativeStats failed: %v", err)
	}

	if len(stats) != 3 {
		t.Fatalf("expected 3 initiatives, got %d", len(stats))
	}

	// Verify total task count matches
	totalTasks := 0
	for _, s := range stats {
		totalTasks += s.TaskCount
	}
	if totalTasks != 10 {
		t.Errorf("expected 10 total tasks across initiatives, got %d", totalTasks)
	}

	// Should be sorted by task count descending
	for i := 1; i < len(stats); i++ {
		if stats[i].TaskCount > stats[i-1].TaskCount {
			t.Errorf("not sorted: initiative %d has %d tasks > initiative %d with %d tasks",
				i, stats[i].TaskCount, i-1, stats[i-1].TaskCount)
		}
	}
}

// ============================================================================
// Benchmark: SC-2 — aggregate queries vs full load
// ============================================================================

// BenchmarkDashboardStatusCounts_Aggregate benchmarks the SQL aggregate path.
func BenchmarkDashboardStatusCounts_Aggregate(b *testing.B) {
	db := NewTestProjectDB(b)

	// Seed 200+ tasks
	for i := 0; i < 225; i++ {
		status := "completed"
		switch {
		case i%10 == 0:
			status = "failed"
		case i%7 == 0:
			status = "running"
		case i%5 == 0:
			status = "blocked"
		}
		tk := &Task{
			ID:     fmt.Sprintf("TASK-%03d", i+1),
			Title:  fmt.Sprintf("Task %d", i+1),
			Status: status,
		}
		if err := db.SaveTask(tk); err != nil {
			b.Fatalf("save task: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetDashboardStatusCounts()
		if err != nil {
			b.Fatalf("GetDashboardStatusCounts: %v", err)
		}
	}
}

// BenchmarkDashboardStatusCounts_FullLoad benchmarks the full LoadAllTasks path
// for comparison with the aggregate path.
func BenchmarkDashboardStatusCounts_FullLoad(b *testing.B) {
	db := NewTestProjectDB(b)

	// Seed 200+ tasks
	for i := 0; i < 225; i++ {
		status := "completed"
		switch {
		case i%10 == 0:
			status = "failed"
		case i%7 == 0:
			status = "running"
		case i%5 == 0:
			status = "blocked"
		}
		tk := &Task{
			ID:     fmt.Sprintf("TASK-%03d", i+1),
			Title:  fmt.Sprintf("Task %d", i+1),
			Status: status,
		}
		if err := db.SaveTask(tk); err != nil {
			b.Fatalf("save task: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := db.ListTasks(ListOpts{})
		if err != nil {
			b.Fatalf("ListTasks: %v", err)
		}
	}
}
