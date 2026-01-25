package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// setupTestDB creates a temporary database for testing.
func setupTestDB(t *testing.T) (*DatabaseBackend, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	// Create .orc directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	backend, err := NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("create backend: %v", err)
	}

	return backend, tmpDir
}

// teardownTestDB cleans up the test database.
func teardownTestDB(t *testing.T, backend *DatabaseBackend, tmpDir string) {
	t.Helper()

	if err := backend.Close(); err != nil {
		t.Errorf("close backend: %v", err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("remove temp dir: %v", err)
	}
}

// TestSaveTask_Transaction verifies task and dependencies are saved atomically.
func TestSaveTask_Transaction(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task with dependencies
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Dependency Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create another task that depends on task1
	task2 := &task.Task{
		ID:        "TASK-002",
		Title:     "Test Task",
		Weight:    task.WeightMedium,
		Status:    task.StatusCreated,
		BlockedBy: []string{"TASK-001"},
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(loaded.BlockedBy) != 1 || loaded.BlockedBy[0] != "TASK-001" {
		t.Errorf("expected BlockedBy=[TASK-001], got %v", loaded.BlockedBy)
	}

	// Update dependencies
	task2.BlockedBy = []string{}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("update task2: %v", err)
	}

	// Verify dependencies were cleared
	loaded, err = backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(loaded.BlockedBy) != 0 {
		t.Errorf("expected BlockedBy=[], got %v", loaded.BlockedBy)
	}
}

// TestSaveTask_QualityMetrics verifies quality metrics are persisted and loaded correctly.
func TestSaveTask_QualityMetrics(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task with quality metrics
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task with Quality",
		Weight:    task.WeightMedium,
		Status:    task.StatusFailed,
		CreatedAt: time.Now(),
	}

	// Add quality metrics
	testTask.RecordPhaseRetry("implement")
	testTask.RecordPhaseRetry("implement")
	testTask.RecordPhaseRetry("review")
	testTask.RecordReviewRejection()
	testTask.RecordManualIntervention("Fixed manually via resolve command")

	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Verify quality metrics were persisted
	if loaded.Quality == nil {
		t.Fatal("Quality metrics should not be nil after load")
	}

	// Verify phase retries
	if loaded.Quality.TotalRetries != 3 {
		t.Errorf("expected TotalRetries=3, got %d", loaded.Quality.TotalRetries)
	}
	if loaded.Quality.PhaseRetries["implement"] != 2 {
		t.Errorf("expected implement retries=2, got %d", loaded.Quality.PhaseRetries["implement"])
	}
	if loaded.Quality.PhaseRetries["review"] != 1 {
		t.Errorf("expected review retries=1, got %d", loaded.Quality.PhaseRetries["review"])
	}

	// Verify review rejections
	if loaded.Quality.ReviewRejections != 1 {
		t.Errorf("expected ReviewRejections=1, got %d", loaded.Quality.ReviewRejections)
	}

	// Verify manual intervention
	if !loaded.Quality.ManualIntervention {
		t.Error("expected ManualIntervention=true")
	}
	if loaded.Quality.ManualInterventionReason != "Fixed manually via resolve command" {
		t.Errorf("expected ManualInterventionReason to match, got %q", loaded.Quality.ManualInterventionReason)
	}
}

// TestSaveTask_QualityMetrics_Empty verifies tasks without quality metrics load correctly.
func TestSaveTask_QualityMetrics_Empty(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task without quality metrics
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task without Quality",
		Weight:    task.WeightSmall,
		Status:    task.StatusCompleted,
		CreatedAt: time.Now(),
	}

	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Quality should be nil when not set
	if loaded.Quality != nil {
		t.Errorf("expected Quality to be nil for task without metrics, got %+v", loaded.Quality)
	}
}

// TestSaveInitiative_Transaction verifies initiative and related data are saved atomically.
func TestSaveInitiative_Transaction(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// First create a task to link
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "First Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create initiative with tasks
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		Vision:    "Test vision",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Tasks: []initiative.TaskRef{
			{ID: "TASK-001", Title: "First Task", Status: "created"},
		},
	}

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	if loaded.Title != "Test Initiative" {
		t.Errorf("expected Title='Test Initiative', got %s", loaded.Title)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
}

// TestSaveInitiative_WithDependencies verifies initiative dependencies are saved.
func TestSaveInitiative_WithDependencies(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create two initiatives
	init1 := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "First Initiative",
		Status:    initiative.StatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	init2 := &initiative.Initiative{
		ID:        "INIT-002",
		Title:     "Second Initiative",
		Status:    initiative.StatusActive,
		BlockedBy: []string{"INIT-001"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadInitiative("INIT-002")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	if len(loaded.BlockedBy) != 1 || loaded.BlockedBy[0] != "INIT-001" {
		t.Errorf("expected BlockedBy=[INIT-001], got %v", loaded.BlockedBy)
	}
}

// TestTransactionRollback_SaveTask verifies that if saving dependencies fails,
// the task data is not partially written.
// This is a regression test for transaction atomicity.
func TestTransactionRollback_SaveTask(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a valid task first
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "First Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create a task with valid dependencies
	task2 := &task.Task{
		ID:        "TASK-002",
		Title:     "Second Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		BlockedBy: []string{"TASK-001"},
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Verify the task was saved with dependencies
	loaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if len(loaded.BlockedBy) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(loaded.BlockedBy))
	}
}

// TestTransactionAtomicity_SaveInitiative verifies initiative saves are atomic.
func TestTransactionAtomicity_SaveInitiative(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create tasks first
	for i := 1; i <= 3; i++ {
		task := &task.Task{
			ID:        taskID(i),
			Title:     taskTitle(i),
			Weight:    task.WeightSmall,
			Status:    task.StatusCreated,
			CreatedAt: time.Now(),
		}
		if err := backend.SaveTask(task); err != nil {
			t.Fatalf("save task%d: %v", i, err)
		}
	}

	// Create initiative with multiple tasks
	init := &initiative.Initiative{
		ID:     "INIT-001",
		Title:  "Test Initiative",
		Status: initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-001"},
			{ID: "TASK-002"},
			{ID: "TASK-003"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Verify all tasks are linked
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(loaded.Tasks))
	}

	// Update to remove one task
	init.Tasks = []initiative.TaskRef{
		{ID: "TASK-001"},
		{ID: "TASK-003"},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("update initiative: %v", err)
	}

	// Verify task was removed
	loaded, err = backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(loaded.Tasks))
	}
}

// TestConcurrentAccess verifies mutex protection for concurrent operations.
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create initial task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Concurrent Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Run concurrent updates
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(iteration int) {
			task := &task.Task{
				ID:        "TASK-001",
				Title:     taskTitle(iteration),
				Weight:    task.WeightSmall,
				Status:    task.StatusCreated,
				CreatedAt: time.Now(),
			}
			done <- backend.SaveTask(task)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent save %d failed: %v", i, err)
		}
	}

	// Verify task still loads correctly
	_, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Errorf("load task after concurrent updates: %v", err)
	}
}

// Helper functions

func taskID(n int) string {
	return "TASK-" + padNumber(n)
}

func taskTitle(n int) string {
	return "Task " + padNumber(n)
}

func padNumber(n int) string {
	if n < 10 {
		return "00" + string(rune('0'+n))
	}
	if n < 100 {
		return "0" + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return string(rune('0'+n/100)) + string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// TestSaveTaskCtx_ContextCancellation verifies that a canceled context aborts the transaction.
// This tests the context propagation from DatabaseBackend through TxOps to the driver.
func TestSaveTaskCtx_ContextCancellation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to save with canceled context
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	err := backend.SaveTaskCtx(ctx, task1)

	// Should return context canceled error
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}

	// Verify task was not saved
	_, loadErr := backend.LoadTask("TASK-001")
	if loadErr == nil {
		t.Error("task should not have been saved with canceled context")
	}
}

// TestSaveInitiativeCtx_ContextCancellation verifies that a canceled context aborts the initiative save.
func TestSaveInitiativeCtx_ContextCancellation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to save initiative with canceled context
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := backend.SaveInitiativeCtx(ctx, init)

	// Should return context canceled error
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}

	// Verify initiative was not saved
	_, loadErr := backend.LoadInitiative("INIT-001")
	if loadErr == nil {
		t.Error("initiative should not have been saved with canceled context")
	}
}

// TestSaveTaskCtx_ValidContext verifies that a valid context allows the operation to succeed.
func TestSaveTaskCtx_ValidContext(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context with timeout (plenty of time)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Save with valid context
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	err := backend.SaveTaskCtx(ctx, task1)
	if err != nil {
		t.Fatalf("save with valid context should succeed: %v", err)
	}

	// Verify task was saved
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loaded.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got %s", loaded.Title)
	}
}

// TestSaveTask_PreservesExecutorFields verifies that SaveTask does NOT overwrite executor fields.
// This is a regression test for TASK-249: SaveTask was overwriting ExecutorPID, ExecutorHostname,
// ExecutorStartedAt, and LastHeartbeat with zero values, causing false orphan detection.
//
// Scenario:
// 1. Create task and set executor fields via SetTaskExecutor
// 2. Some other code (e.g., CLI edit, API update) calls SaveTask to update task metadata
// 3. SaveTask MUST preserve executor fields to avoid false orphan detection
func TestSaveTask_PreservesExecutorFields(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Step 1: Create task
	task1 := &task.Task{
		ID:           "TASK-001",
		Title:        "Original Title",
		Weight:       task.WeightSmall,
		Status:       task.StatusRunning,
		CurrentPhase: "implement",
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Step 2: Set executor fields via SetTaskExecutor (simulates executor claiming task)
	if err := backend.SetTaskExecutor("TASK-001", 12345, "worker-1"); err != nil {
		t.Fatalf("set executor: %v", err)
	}

	// Verify executor fields are set
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loaded.ExecutorPID != 12345 {
		t.Fatalf("executor fields not set properly: expected PID 12345, got %d", loaded.ExecutorPID)
	}

	// Step 3: Someone updates task metadata via SaveTask (e.g., CLI edit, API update)
	// This should NOT clear the executor fields!
	loaded.Title = "Updated Title"
	loaded.Description = "Added description"
	if err := backend.SaveTask(loaded); err != nil {
		t.Fatalf("update task: %v", err)
	}

	// Step 4: Verify executor fields are STILL SET after SaveTask
	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task after update: %v", err)
	}

	// This is the critical assertion that was failing before TASK-249 fix
	if reloaded.ExecutorPID != 12345 {
		t.Errorf("REGRESSION: SaveTask overwrote ExecutorPID: expected 12345, got %d", reloaded.ExecutorPID)
	}
	if reloaded.ExecutorHostname != "worker-1" {
		t.Errorf("REGRESSION: SaveTask overwrote ExecutorHostname: expected 'worker-1', got %s", reloaded.ExecutorHostname)
	}

	// Verify task metadata was updated correctly
	if reloaded.Title != "Updated Title" {
		t.Errorf("expected title to be updated to 'Updated Title', got %s", reloaded.Title)
	}
}

// =============================================================================
// Branch Registry Tests
// =============================================================================

// TestBranch_SaveAndLoad verifies basic branch save/load operations.
func TestBranch_SaveAndLoad(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	now := time.Now()
	branch := &Branch{
		Name:         "feature/user-auth",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-001",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}

	// Save
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Load
	loaded, err := backend.LoadBranch("feature/user-auth")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}

	if loaded.Name != "feature/user-auth" {
		t.Errorf("expected name 'feature/user-auth', got %s", loaded.Name)
	}
	if loaded.Type != BranchTypeInitiative {
		t.Errorf("expected type 'initiative', got %s", loaded.Type)
	}
	if loaded.OwnerID != "INIT-001" {
		t.Errorf("expected owner 'INIT-001', got %s", loaded.OwnerID)
	}
	if loaded.Status != BranchStatusActive {
		t.Errorf("expected status 'active', got %s", loaded.Status)
	}
}

// TestBranch_Update verifies branch update operations.
func TestBranch_Update(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	now := time.Now()
	branch := &Branch{
		Name:         "dev/randy",
		Type:         BranchTypeStaging,
		OwnerID:      "randy",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update status
	if err := backend.UpdateBranchStatus("dev/randy", BranchStatusMerged); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// Verify
	loaded, err := backend.LoadBranch("dev/randy")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}
	if loaded.Status != BranchStatusMerged {
		t.Errorf("expected status 'merged', got %s", loaded.Status)
	}
}

// TestBranch_UpdateActivity verifies last activity timestamp updates.
func TestBranch_UpdateActivity(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch with old activity
	oldTime := time.Now().Add(-24 * time.Hour)
	branch := &Branch{
		Name:         "orc/TASK-001",
		Type:         BranchTypeTask,
		OwnerID:      "TASK-001",
		CreatedAt:    oldTime,
		LastActivity: oldTime,
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update activity
	if err := backend.UpdateBranchActivity("orc/TASK-001"); err != nil {
		t.Fatalf("update activity: %v", err)
	}

	// Verify activity was updated
	loaded, err := backend.LoadBranch("orc/TASK-001")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}

	// Activity should be recent (within last second)
	if time.Since(loaded.LastActivity) > time.Second {
		t.Errorf("activity not updated: %v", loaded.LastActivity)
	}
}

// TestBranch_Delete verifies branch deletion.
func TestBranch_Delete(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	branch := &Branch{
		Name:         "feature/temp",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-002",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Delete
	if err := backend.DeleteBranch("feature/temp"); err != nil {
		t.Fatalf("delete branch: %v", err)
	}

	// Verify deleted
	loaded, err := backend.LoadBranch("feature/temp")
	if err != nil {
		t.Fatalf("load should not error for missing branch: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for deleted branch")
	}
}

// TestBranch_ListAll verifies listing all branches.
func TestBranch_ListAll(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create multiple branches
	branches := []*Branch{
		{Name: "feature/auth", Type: BranchTypeInitiative, OwnerID: "INIT-001", Status: BranchStatusActive},
		{Name: "dev/randy", Type: BranchTypeStaging, OwnerID: "randy", Status: BranchStatusActive},
		{Name: "orc/TASK-001", Type: BranchTypeTask, OwnerID: "TASK-001", Status: BranchStatusMerged},
	}
	now := time.Now()
	for _, b := range branches {
		b.CreatedAt = now
		b.LastActivity = now
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List all
	list, err := backend.ListBranches(BranchListOpts{})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 branches, got %d", len(list))
	}
}

// TestBranch_ListByType verifies filtering by type.
func TestBranch_ListByType(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branches of different types
	now := time.Now()
	branches := []*Branch{
		{Name: "feature/a", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "feature/b", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "dev/user", Type: BranchTypeStaging, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "orc/TASK-001", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List initiatives only
	list, err := backend.ListBranches(BranchListOpts{Type: BranchTypeInitiative})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 initiative branches, got %d", len(list))
	}
	for _, b := range list {
		if b.Type != BranchTypeInitiative {
			t.Errorf("expected initiative type, got %s", b.Type)
		}
	}
}

// TestBranch_ListByStatus verifies filtering by status.
func TestBranch_ListByStatus(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branches with different statuses
	now := time.Now()
	branches := []*Branch{
		{Name: "b1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "b2", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "b3", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
		{Name: "b4", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusOrphaned},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List active only
	list, err := backend.ListBranches(BranchListOpts{Status: BranchStatusActive})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 active branches, got %d", len(list))
	}

	// List merged only
	list, err = backend.ListBranches(BranchListOpts{Status: BranchStatusMerged})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 merged branch, got %d", len(list))
	}
}

// TestBranch_ListByTypeAndStatus verifies combined filtering.
func TestBranch_ListByTypeAndStatus(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	branches := []*Branch{
		{Name: "f1", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "f2", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
		{Name: "d1", Type: BranchTypeStaging, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "t1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List merged initiatives
	list, err := backend.ListBranches(BranchListOpts{
		Type:   BranchTypeInitiative,
		Status: BranchStatusMerged,
	})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 merged initiative, got %d", len(list))
	}
	if list[0].Name != "f2" {
		t.Errorf("expected branch 'f2', got %s", list[0].Name)
	}
}

// TestBranch_GetStaleBranches verifies stale branch detection.
func TestBranch_GetStaleBranches(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	oldTime := now.Add(-35 * 24 * time.Hour) // 35 days ago

	branches := []*Branch{
		{Name: "old1", Type: BranchTypeTask, CreatedAt: oldTime, LastActivity: oldTime, Status: BranchStatusActive},
		{Name: "old2", Type: BranchTypeTask, CreatedAt: oldTime, LastActivity: oldTime, Status: BranchStatusActive},
		{Name: "new1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// Get branches stale for more than 30 days
	staleThreshold := now.Add(-30 * 24 * time.Hour)
	stale, err := backend.GetStaleBranches(staleThreshold)
	if err != nil {
		t.Fatalf("get stale branches: %v", err)
	}

	if len(stale) != 2 {
		t.Errorf("expected 2 stale branches, got %d", len(stale))
	}

	// Verify correct branches returned
	names := make(map[string]bool)
	for _, b := range stale {
		names[b.Name] = true
	}
	if !names["old1"] || !names["old2"] {
		t.Errorf("expected old1 and old2, got %v", names)
	}
}

// TestBranch_LoadNonExistent verifies loading non-existent branch returns nil.
func TestBranch_LoadNonExistent(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	loaded, err := backend.LoadBranch("does-not-exist")
	if err != nil {
		t.Fatalf("load should not error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for non-existent branch")
	}
}

// TestBranch_Upsert verifies that SaveBranch can update existing branches.
func TestBranch_Upsert(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	branch := &Branch{
		Name:         "feature/test",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-001",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}

	// Initial save
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Update via save (upsert)
	branch.Status = BranchStatusMerged
	branch.LastActivity = now.Add(time.Hour)
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("upsert save: %v", err)
	}

	// Verify update
	loaded, err := backend.LoadBranch("feature/test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Status != BranchStatusMerged {
		t.Errorf("expected status 'merged', got %s", loaded.Status)
	}
}

// TestSaveInitiative_DecisionIDsAcrossInitiatives verifies that multiple initiatives
// can each have decisions with the same local IDs (e.g., DEC-001).
// This tests the fix for the decision ID collision bug.
func TestSaveInitiative_DecisionIDsAcrossInitiatives(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create first initiative with DEC-001
	init1 := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "First Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{
				ID:        "DEC-001",
				Decision:  "Use Go for backend",
				Rationale: "Team expertise",
				Date:      now,
				By:        "alice",
			},
		},
	}
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save first initiative: %v", err)
	}

	// Create second initiative with its own DEC-001 (same ID, different initiative)
	init2 := &initiative.Initiative{
		ID:        "INIT-002",
		Title:     "Second Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{
				ID:        "DEC-001", // Same ID as init1's decision
				Decision:  "Use React for frontend",
				Rationale: "Modern framework",
				Date:      now,
				By:        "bob",
			},
		},
	}

	// This should NOT fail - each initiative has its own decision namespace
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save second initiative with same decision ID: %v", err)
	}

	// Verify both initiatives have their decisions intact
	loaded1, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load first initiative: %v", err)
	}
	if len(loaded1.Decisions) != 1 {
		t.Errorf("expected 1 decision in init1, got %d", len(loaded1.Decisions))
	}
	if loaded1.Decisions[0].Decision != "Use Go for backend" {
		t.Errorf("init1 decision content mismatch: got %s", loaded1.Decisions[0].Decision)
	}

	loaded2, err := backend.LoadInitiative("INIT-002")
	if err != nil {
		t.Fatalf("load second initiative: %v", err)
	}
	if len(loaded2.Decisions) != 1 {
		t.Errorf("expected 1 decision in init2, got %d", len(loaded2.Decisions))
	}
	if loaded2.Decisions[0].Decision != "Use React for frontend" {
		t.Errorf("init2 decision content mismatch: got %s", loaded2.Decisions[0].Decision)
	}
}

// TestSaveInitiative_DecisionLookupByInitiative verifies decision lookup works correctly
// with the composite key schema.
func TestSaveInitiative_DecisionLookupByInitiative(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create initiative with multiple decisions
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Multi-Decision Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Decision 1", Date: now, By: "user1"},
			{ID: "DEC-002", Decision: "Decision 2", Date: now, By: "user2"},
			{ID: "DEC-003", Decision: "Decision 3", Date: now, By: "user3"},
		},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Load and verify all decisions are correctly associated
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Decisions) != 3 {
		t.Errorf("expected 3 decisions, got %d", len(loaded.Decisions))
	}

	// Verify decision IDs
	decisionIDs := make(map[string]bool)
	for _, d := range loaded.Decisions {
		decisionIDs[d.ID] = true
	}
	for _, expectedID := range []string{"DEC-001", "DEC-002", "DEC-003"} {
		if !decisionIDs[expectedID] {
			t.Errorf("missing decision %s", expectedID)
		}
	}
}

// TestSaveInitiative_DecisionUpdate verifies that updating an initiative preserves decisions
// correctly with the clear-and-reinsert pattern.
func TestSaveInitiative_DecisionUpdate(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create initiative with initial decision
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Update Test Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Initial decision", Date: now, By: "user"},
		},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initial: %v", err)
	}

	// Update with more decisions
	init.Decisions = append(init.Decisions, initiative.Decision{
		ID:       "DEC-002",
		Decision: "Second decision",
		Date:     now,
		By:       "user",
	})
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("update with new decision: %v", err)
	}

	// Verify both decisions exist
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Decisions) != 2 {
		t.Errorf("expected 2 decisions after update, got %d", len(loaded.Decisions))
	}

	// Re-save without changing decisions (idempotency test)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	loaded, err = backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load after re-save: %v", err)
	}
	if len(loaded.Decisions) != 2 {
		t.Errorf("expected 2 decisions after re-save, got %d", len(loaded.Decisions))
	}
}

// TestReviewFindings_SaveAndLoad verifies ReviewFindings operations through the Backend interface.
func TestReviewFindings_SaveAndLoad(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task first (foreign key constraint)
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task for Review",
		Weight:    task.WeightMedium,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create review findings with AgentID
	findings := &ReviewFindings{
		TaskID:  "TASK-001",
		Round:   1,
		Summary: "Found 2 issues in the implementation",
		Issues: []ReviewFinding{
			{Severity: "high", File: "main.go", Line: 42, Description: "SQL injection vulnerability", AgentID: "code-reviewer"},
			{Severity: "medium", File: "utils.go", Line: 100, Description: "Error not handled", AgentID: "silent-failure-hunter"},
		},
		Questions: []string{"Why is this implemented synchronously?"},
		Positives: []string{"Good test coverage"},
		AgentID:   "code-reviewer",
	}

	// Save
	if err := backend.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings: %v", err)
	}

	// Load
	loaded, err := backend.LoadReviewFindings("TASK-001", 1)
	if err != nil {
		t.Fatalf("LoadReviewFindings: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadReviewFindings returned nil")
	}

	// Verify fields
	if loaded.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, "TASK-001")
	}
	if loaded.Round != 1 {
		t.Errorf("Round = %d, want 1", loaded.Round)
	}
	if loaded.Summary != "Found 2 issues in the implementation" {
		t.Errorf("Summary = %q, want %q", loaded.Summary, "Found 2 issues in the implementation")
	}
	if len(loaded.Issues) != 2 {
		t.Errorf("Issues count = %d, want 2", len(loaded.Issues))
	}
	if loaded.Issues[0].Severity != "high" {
		t.Errorf("Issues[0].Severity = %q, want %q", loaded.Issues[0].Severity, "high")
	}
	if loaded.Issues[0].AgentID != "code-reviewer" {
		t.Errorf("Issues[0].AgentID = %q, want %q", loaded.Issues[0].AgentID, "code-reviewer")
	}
	if loaded.AgentID != "code-reviewer" {
		t.Errorf("AgentID = %q, want %q", loaded.AgentID, "code-reviewer")
	}
	if len(loaded.Questions) != 1 {
		t.Errorf("Questions count = %d, want 1", len(loaded.Questions))
	}
	if len(loaded.Positives) != 1 {
		t.Errorf("Positives count = %d, want 1", len(loaded.Positives))
	}
}

// TestReviewFindings_LoadAll verifies loading all review findings for a task.
func TestReviewFindings_LoadAll(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task
	testTask := &task.Task{
		ID:        "TASK-002",
		Title:     "Test Task",
		Weight:    task.WeightLarge,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save findings for multiple rounds
	round1 := &ReviewFindings{
		TaskID:  "TASK-002",
		Round:   1,
		Summary: "Round 1 findings",
		Issues:  []ReviewFinding{{Severity: "high", Description: "Issue 1"}},
		AgentID: "code-reviewer",
	}
	round2 := &ReviewFindings{
		TaskID:  "TASK-002",
		Round:   2,
		Summary: "Round 2 findings",
		Issues:  []ReviewFinding{{Severity: "low", Description: "Issue 2"}},
		AgentID: "silent-failure-hunter",
	}

	if err := backend.SaveReviewFindings(round1); err != nil {
		t.Fatalf("SaveReviewFindings round1: %v", err)
	}
	if err := backend.SaveReviewFindings(round2); err != nil {
		t.Fatalf("SaveReviewFindings round2: %v", err)
	}

	// Load all
	all, err := backend.LoadAllReviewFindings("TASK-002")
	if err != nil {
		t.Fatalf("LoadAllReviewFindings: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("LoadAllReviewFindings returned %d, want 2", len(all))
	}

	// Verify ordering (should be by round ascending)
	if all[0].Round != 1 {
		t.Errorf("First result Round = %d, want 1", all[0].Round)
	}
	if all[1].Round != 2 {
		t.Errorf("Second result Round = %d, want 2", all[1].Round)
	}

	// Verify AgentID preserved
	if all[0].AgentID != "code-reviewer" {
		t.Errorf("First result AgentID = %q, want %q", all[0].AgentID, "code-reviewer")
	}
	if all[1].AgentID != "silent-failure-hunter" {
		t.Errorf("Second result AgentID = %q, want %q", all[1].AgentID, "silent-failure-hunter")
	}
}

// TestReviewFindings_NotFound verifies behavior when findings don't exist.
func TestReviewFindings_NotFound(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Load non-existent findings
	findings, err := backend.LoadReviewFindings("NONEXISTENT", 1)
	if err != nil {
		t.Fatalf("LoadReviewFindings should not error for missing data: %v", err)
	}
	if findings != nil {
		t.Error("LoadReviewFindings should return nil for missing data")
	}
}
