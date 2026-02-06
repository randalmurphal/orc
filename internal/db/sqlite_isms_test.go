package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// =============================================================================
// SC-2: buildTimeseriesQuery uses driver.DateFormat instead of hardcoded strftime
// =============================================================================

// TestBuildTimeseriesQuery_UsesDriverDateFormat verifies that buildTimeseriesQuery
// accepts a driver and produces dialect-correct SQL.
// This is an integration test: it verifies the wiring between buildTimeseriesQuery
// and the driver's DateFormat method.
// Covers SC-2.
func TestBuildTimeseriesQuery_UsesDriverDateFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		drv         driver.Driver
		granularity string
		wantContain string // fragment that MUST appear in the query
		wantAbsent  string // fragment that MUST NOT appear
	}{
		{
			name:        "sqlite day uses strftime",
			drv:         driver.NewSQLite(),
			granularity: "day",
			wantContain: "strftime('%Y-%m-%d', timestamp)",
			wantAbsent:  "TO_CHAR",
		},
		{
			name:        "sqlite week uses strftime with %W",
			drv:         driver.NewSQLite(),
			granularity: "week",
			wantContain: "strftime('%Y-W%W', timestamp)",
			wantAbsent:  "TO_CHAR",
		},
		{
			name:        "sqlite month uses strftime with %Y-%m",
			drv:         driver.NewSQLite(),
			granularity: "month",
			wantContain: "strftime('%Y-%m', timestamp)",
			wantAbsent:  "TO_CHAR",
		},
		{
			name:        "postgres day uses TO_CHAR",
			drv:         driver.NewPostgres(),
			granularity: "day",
			wantContain: "TO_CHAR(timestamp, 'YYYY-MM-DD')",
			wantAbsent:  "strftime",
		},
		{
			name:        "postgres week uses TO_CHAR with IW",
			drv:         driver.NewPostgres(),
			granularity: "week",
			wantContain: "TO_CHAR",
			wantAbsent:  "strftime",
		},
		{
			name:        "postgres month uses TO_CHAR",
			drv:         driver.NewPostgres(),
			granularity: "month",
			wantContain: "TO_CHAR",
			wantAbsent:  "strftime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildTimeseriesQuery(tt.drv, tt.granularity, false)

			if !strings.Contains(query, tt.wantContain) {
				t.Errorf("query should contain %q for %s dialect, got:\n%s",
					tt.wantContain, tt.drv.Dialect(), query)
			}
			if tt.wantAbsent != "" && strings.Contains(query, tt.wantAbsent) {
				t.Errorf("query should NOT contain %q for %s dialect, got:\n%s",
					tt.wantAbsent, tt.drv.Dialect(), query)
			}
		})
	}
}

// TestBuildTimeseriesQuery_WithProjectFilter verifies the project filter is added.
// Covers SC-2.
func TestBuildTimeseriesQuery_WithProjectFilter(t *testing.T) {
	t.Parallel()
	drv := driver.NewSQLite()

	// Without project filter
	queryNoProject := buildTimeseriesQuery(drv, "day", false)
	if strings.Contains(queryNoProject, "project_id = ?") {
		t.Error("query without project filter should not contain 'project_id = ?'")
	}

	// With project filter
	queryWithProject := buildTimeseriesQuery(drv, "day", true)
	if !strings.Contains(queryWithProject, "project_id = ?") {
		t.Error("query with project filter should contain 'project_id = ?'")
	}
}

// TestStrftimeFormat_Removed verifies that the strftimeFormat function
// no longer exists (it should be removed as part of SC-2).
// This is a source-code level verification test.
// Covers SC-2.
func TestStrftimeFormat_Removed(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("global.go")
	if err != nil {
		t.Fatalf("read global.go: %v", err)
	}

	// strftimeFormat should be removed entirely
	if strings.Contains(string(content), "func strftimeFormat(") {
		t.Error("strftimeFormat() function still exists in global.go - should be removed per SC-2")
	}
}

// =============================================================================
// SC-3: No datetime('now') in target files
// =============================================================================

// TestNoHardcodedDatetimeNow verifies that datetime('now') is not hardcoded
// in the target files. The spec explicitly requires all occurrences be replaced
// with Driver().Now().
// Covers SC-3.
func TestNoHardcodedDatetimeNow(t *testing.T) {
	t.Parallel()

	targetFiles := []string{
		"global.go",
		"branch.go",
		"phase_output.go",
		"project.go",
		"subtask.go",
	}

	for _, file := range targetFiles {
		t.Run(file, func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}

			// Count occurrences of datetime('now') outside of comments
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Skip comment lines
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, "datetime('now')") {
					t.Errorf("%s:%d contains hardcoded datetime('now'): %s",
						file, i+1, trimmed)
				}
			}
		})
	}
}

// =============================================================================
// SC-4: UpdateBranchActivity uses driver.DateFormat instead of hardcoded strftime
// =============================================================================

// TestUpdateBranchActivity_NoHardcodedStrftime verifies that branch.go no longer
// contains hardcoded strftime calls. The UpdateBranchActivity function should use
// Driver().DateFormat() instead.
// Covers SC-4.
func TestNoHardcodedStrftimeInBranch(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("branch.go")
	if err != nil {
		t.Fatalf("read branch.go: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "strftime(") {
			t.Errorf("branch.go:%d contains hardcoded strftime: %s",
				i+1, trimmed)
		}
	}
}

// TestUpdateBranchActivity_SetsRFC3339Timestamp verifies that UpdateBranchActivity
// writes a timestamp that can be parsed as RFC3339, regardless of dialect.
// This is a behavioral test using SQLite (the available test database).
// Covers SC-4.
func TestUpdateBranchActivity_SetsRFC3339Timestamp(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create a branch
	now := time.Now().UTC()
	branch := &Branch{
		Name:         "test-branch",
		Type:         BranchTypeTask,
		OwnerID:      "TASK-001",
		BaseBranch:   "main",
		Status:       BranchStatusActive,
		CreatedAt:    now,
		LastActivity: now.Add(-1 * time.Hour), // Set to an hour ago
	}
	if err := pdb.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update activity
	if err := pdb.UpdateBranchActivity("test-branch"); err != nil {
		t.Fatalf("update branch activity: %v", err)
	}

	// Retrieve and verify the timestamp is parseable as RFC3339
	got, err := pdb.GetBranch("test-branch")
	if err != nil {
		t.Fatalf("get branch: %v", err)
	}
	if got == nil {
		t.Fatal("branch not found after update")
	}

	// LastActivity should be recent (within last minute)
	if time.Since(got.LastActivity) > time.Minute {
		t.Errorf("LastActivity = %v, expected recent timestamp (within last minute)", got.LastActivity)
	}
}

// =============================================================================
// SC-2 + SC-4 Integration: GetCostTimeseries works end-to-end with driver
// =============================================================================

// TestGetCostTimeseries_UsesDriver verifies that the full GetCostTimeseries path
// works correctly when buildTimeseriesQuery uses the driver.
// This is a sociable test — it uses real SQLite but verifies the wiring from
// GetCostTimeseries through buildTimeseriesQuery to the driver.
// Covers SC-2.
func TestGetCostTimeseries_UsesDriver(t *testing.T) {
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
		ProjectID:   "proj-1",
		TaskID:      "TASK-001",
		Model:       "opus",
		CostUSD:     0.50,
		InputTokens: 1000,
	}
	if err := gdb.RecordCostExtended(entry); err != nil {
		t.Fatalf("RecordCostExtended failed: %v", err)
	}

	since := time.Now().Add(-1 * time.Hour)

	// All granularities should work without strftime being hardcoded
	for _, granularity := range []string{"day", "week", "month"} {
		t.Run(granularity, func(t *testing.T) {
			ts, err := gdb.GetCostTimeseries("proj-1", since, granularity)
			if err != nil {
				t.Fatalf("GetCostTimeseries(%q) failed: %v", granularity, err)
			}
			if len(ts) == 0 {
				t.Errorf("GetCostTimeseries(%q) returned empty results", granularity)
			}
		})
	}
}

// =============================================================================
// SC-3 Integration: SetBudget uses Driver().Now() end-to-end
// =============================================================================

// TestSetBudget_WritesTimestamp verifies that SetBudget correctly writes
// timestamps using the driver's Now() helper.
// Covers SC-3 (datetime('now') removed from global.go).
func TestSetBudget_WritesTimestamp(t *testing.T) {
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

	budget := CostBudget{
		ProjectID:             "proj-1",
		MonthlyLimitUSD:       100.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          "2025-01",
		CurrentMonthSpent:     25.00,
	}
	if err := gdb.SetBudget(budget); err != nil {
		t.Fatalf("SetBudget failed: %v", err)
	}

	// Verify updated_at was set (proves Now() works)
	got, err := gdb.GetBudget("proj-1")
	if err != nil {
		t.Fatalf("GetBudget failed: %v", err)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero after SetBudget")
	}
	if time.Since(got.UpdatedAt) > time.Minute {
		t.Errorf("UpdatedAt = %v, expected recent timestamp", got.UpdatedAt)
	}
}

// =============================================================================
// SC-3 Integration: ApproveSubtask uses Driver().Now() end-to-end
// =============================================================================

// TestApproveSubtask_SetsTimestamp verifies that ApproveSubtask correctly writes
// the approved_at timestamp using Driver().Now().
// Covers SC-3 (datetime('now') removed from subtask.go).
func TestApproveSubtask_SetsTimestamp(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create task record for FK constraint
	createTestTask(t, pdb, "TASK-001")

	// Queue a subtask
	st, err := pdb.QueueSubtask("TASK-001", "sub task", "description", "user1")
	if err != nil {
		t.Fatalf("queue subtask: %v", err)
	}

	// Approve it
	approved, err := pdb.ApproveSubtask(st.ID, "admin")
	if err != nil {
		t.Fatalf("approve subtask: %v", err)
	}

	// Verify approved_at was set
	if approved.ApprovedAt == nil {
		t.Fatal("ApprovedAt should not be nil after approval")
	}
	if time.Since(*approved.ApprovedAt) > time.Minute {
		t.Errorf("ApprovedAt = %v, expected recent timestamp", *approved.ApprovedAt)
	}
}

// =============================================================================
// SC-3 Integration: UpdateBranchStatus uses Driver().Now()
// =============================================================================

// TestUpdateBranchStatus_SetsTimestamp verifies that UpdateBranchStatus correctly
// writes the last_activity timestamp using Driver().Now().
// Covers SC-3 (datetime('now') removed from branch.go).
func TestUpdateBranchStatus_SetsTimestamp(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now().UTC()
	branch := &Branch{
		Name:         "status-test-branch",
		Type:         BranchTypeTask,
		OwnerID:      "TASK-002",
		BaseBranch:   "main",
		Status:       BranchStatusActive,
		CreatedAt:    now,
		LastActivity: now.Add(-2 * time.Hour),
	}
	if err := pdb.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update status
	if err := pdb.UpdateBranchStatus("status-test-branch", BranchStatusStale); err != nil {
		t.Fatalf("update branch status: %v", err)
	}

	// Verify last_activity was updated to a recent timestamp
	got, err := pdb.GetBranch("status-test-branch")
	if err != nil {
		t.Fatalf("get branch: %v", err)
	}
	if got.Status != BranchStatusStale {
		t.Errorf("Status = %q, want %q", got.Status, BranchStatusStale)
	}
	if time.Since(got.LastActivity) > time.Minute {
		t.Errorf("LastActivity = %v, expected recent timestamp", got.LastActivity)
	}
}

// =============================================================================
// SC-3 Integration: SaveSpecForTask uses Driver().Now()
// =============================================================================

// TestSaveSpecForTask_UsesDriverNow verifies that SaveSpecForTask works
// correctly when datetime('now') is replaced with Driver().Now().
// Covers SC-3 (datetime('now') removed from phase_output.go).
func TestSaveSpecForTask_UsesDriverNow(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create task record for FK constraint
	createTestTask(t, pdb, "TASK-001")

	// SaveSpecForTask creates workflow/run records with datetime('now')
	if err := pdb.SaveSpecForTask("TASK-001", "# Test Spec\nContent here", "import"); err != nil {
		t.Fatalf("SaveSpecForTask failed: %v", err)
	}

	// Verify spec can be retrieved
	content, err := pdb.GetSpecForTask("TASK-001")
	if err != nil {
		t.Fatalf("GetSpecForTask failed: %v", err)
	}
	if content != "# Test Spec\nContent here" {
		t.Errorf("spec content = %q, want '# Test Spec\\nContent here'", content)
	}
}

// =============================================================================
// SC-3: No hardcoded strftime in global.go
// =============================================================================

// TestNoHardcodedStrftimeInGlobal verifies that global.go no longer contains
// hardcoded strftime calls outside of comments.
// Covers SC-2.
func TestNoHardcodedStrftimeInGlobal(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("global.go")
	if err != nil {
		t.Fatalf("read global.go: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "strftime(") {
			t.Errorf("global.go:%d contains hardcoded strftime: %s",
				i+1, trimmed)
		}
	}
}

// =============================================================================
// SC-3 Integration: StoreDetection uses Driver().Now() in SQLite branch
// =============================================================================

// TestStoreDetection_UsesDriverNow verifies that StoreDetection correctly writes
// timestamps. The SQLite branch should use Driver().Now() for consistency.
// Covers SC-3 (project.go uses Driver().Now()).
func TestStoreDetection_UsesDriverNow(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	detection := &Detection{
		Language:    "go",
		Frameworks:  []string{"cobra"},
		BuildTools:  []string{"make"},
		HasTests:    true,
		TestCommand: "go test ./...",
		LintCommand: "golangci-lint run",
	}

	if err := pdb.StoreDetection(detection); err != nil {
		t.Fatalf("store detection: %v", err)
	}

	// Load and verify
	got, err := pdb.LoadDetection()
	if err != nil {
		t.Fatalf("load detection: %v", err)
	}
	if got == nil {
		t.Fatal("detection not found")
	}
	if got.Language != "go" {
		t.Errorf("Language = %q, want go", got.Language)
	}
	// Verify detected_at was set
	if got.DetectedAt.IsZero() {
		t.Error("DetectedAt should not be zero")
	}
	if time.Since(got.DetectedAt) > time.Minute {
		t.Errorf("DetectedAt = %v, expected recent timestamp", got.DetectedAt)
	}
}

// createTestTask inserts a minimal task record to satisfy FK constraints.
func createTestTask(t *testing.T, pdb *ProjectDB, taskID string) {
	t.Helper()
	_, err := pdb.Exec(`INSERT INTO tasks (id, title, status) VALUES (?, ?, ?)`,
		taskID, "test task", "pending")
	if err != nil {
		t.Fatalf("create test task %s: %v", taskID, err)
	}
}
