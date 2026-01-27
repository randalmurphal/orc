// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-382: Implement GET /api/stats/top-files endpoint for file leaderboard
//
// These tests verify the GetTopFiles API returns file modification statistics
// aggregated from completed tasks with git branches.
//
// Success Criteria Coverage:
// - SC-1: Returns up to `limit` files (default 10, max 50) sorted by change_count descending
// - SC-2: Each file includes path, change_count, additions, and deletions fields
// - SC-3: Aggregates file changes across all completed tasks with branches
// - SC-4: Handles tasks with no branch gracefully
// - SC-7: additions and deletions are correctly calculated from git diff
// - SC-8: Existing unit tests pass after implementation
//
// Note: SC-5 and SC-6 (period filtering) require proto changes - see spec.
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-1: Limit enforcement and sorting
// ============================================================================

// TestGetTopFiles_ReturnsFilesWithinLimit verifies SC-1:
// Returns up to `limit` files sorted by change_count descending.
func TestGetTopFiles_ReturnsFilesWithinLimit(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task with a branch
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock diff returns 10 files
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "src/a.go", Additions: 100, Deletions: 50},
		{Path: "src/b.go", Additions: 90, Deletions: 40},
		{Path: "src/c.go", Additions: 80, Deletions: 30},
		{Path: "src/d.go", Additions: 70, Deletions: 20},
		{Path: "src/e.go", Additions: 60, Deletions: 10},
		{Path: "src/f.go", Additions: 50, Deletions: 5},
		{Path: "src/g.go", Additions: 40, Deletions: 4},
		{Path: "src/h.go", Additions: 30, Deletions: 3},
		{Path: "src/i.go", Additions: 20, Deletions: 2},
		{Path: "src/j.go", Additions: 10, Deletions: 1},
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Request only 5 files
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 5,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should return exactly 5 files
	if len(resp.Msg.Files) != 5 {
		t.Errorf("expected 5 files, got %d", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_DefaultLimitIs10 verifies SC-1:
// When limit is 0 or not specified, defaults to 10.
func TestGetTopFiles_DefaultLimitIs10(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task with 15 files
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock diff returns 15 files
	files := make([]diff.FileDiff, 15)
	for i := 0; i < 15; i++ {
		files[i] = diff.FileDiff{Path: "src/file" + string(rune('a'+i)) + ".go", Additions: 100 - i}
	}
	diffSvc.filesByBranch["orc/TASK-001"] = files

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Request with limit=0 (should default to 10)
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 0,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should return default 10 files
	if len(resp.Msg.Files) != 10 {
		t.Errorf("expected default 10 files, got %d", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_NegativeLimit_DefaultsTo10 verifies edge case:
// Negative limit should be treated as default (10).
func TestGetTopFiles_NegativeLimit_DefaultsTo10(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task with 15 files
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	files := make([]diff.FileDiff, 15)
	for i := 0; i < 15; i++ {
		files[i] = diff.FileDiff{Path: "src/file" + string(rune('a'+i)) + ".go", Additions: 100 - i}
	}
	diffSvc.filesByBranch["orc/TASK-001"] = files

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Request with negative limit
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: -1,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should return default 10 files
	if len(resp.Msg.Files) != 10 {
		t.Errorf("expected default 10 files with negative limit, got %d", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_MaxLimitIs50 verifies SC-1:
// Limit exceeding 50 should be clamped to 50.
func TestGetTopFiles_MaxLimitIs50(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task with 60 files
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	files := make([]diff.FileDiff, 60)
	for i := 0; i < 60; i++ {
		files[i] = diff.FileDiff{Path: "src/file" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".go", Additions: 100 - i}
	}
	diffSvc.filesByBranch["orc/TASK-001"] = files

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Request with limit exceeding max
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 100,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should be clamped to max 50 files
	if len(resp.Msg.Files) > 50 {
		t.Errorf("expected max 50 files, got %d", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_SortedByChangeCountDescending verifies SC-1:
// Files should be sorted by change_count in descending order.
func TestGetTopFiles_SortedByChangeCountDescending(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create 3 completed tasks, each modifying different files
	for i := 1; i <= 3; i++ {
		taskID := "TASK-00" + string(rune('0'+i))
		tk := task.NewProtoTask(taskID, "Test task "+taskID)
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		tk.Branch = "orc/" + taskID

		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// TASK-001: modifies file1.go
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "file1.go", Additions: 10, Deletions: 5},
	}
	// TASK-002: modifies file1.go and file2.go
	diffSvc.filesByBranch["orc/TASK-002"] = []diff.FileDiff{
		{Path: "file1.go", Additions: 20, Deletions: 10},
		{Path: "file2.go", Additions: 5, Deletions: 2},
	}
	// TASK-003: modifies file1.go, file2.go, and file3.go
	diffSvc.filesByBranch["orc/TASK-003"] = []diff.FileDiff{
		{Path: "file1.go", Additions: 15, Deletions: 8},
		{Path: "file2.go", Additions: 8, Deletions: 3},
		{Path: "file3.go", Additions: 3, Deletions: 1},
	}

	// file1.go modified by 3 tasks (change_count=3)
	// file2.go modified by 2 tasks (change_count=2)
	// file3.go modified by 1 task (change_count=1)

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Msg.Files))
	}

	// Verify sorted by change_count descending
	if resp.Msg.Files[0].Path != "file1.go" || resp.Msg.Files[0].ChangeCount != 3 {
		t.Errorf("first file should be file1.go with change_count=3, got %s with %d",
			resp.Msg.Files[0].Path, resp.Msg.Files[0].ChangeCount)
	}
	if resp.Msg.Files[1].Path != "file2.go" || resp.Msg.Files[1].ChangeCount != 2 {
		t.Errorf("second file should be file2.go with change_count=2, got %s with %d",
			resp.Msg.Files[1].Path, resp.Msg.Files[1].ChangeCount)
	}
	if resp.Msg.Files[2].Path != "file3.go" || resp.Msg.Files[2].ChangeCount != 1 {
		t.Errorf("third file should be file3.go with change_count=1, got %s with %d",
			resp.Msg.Files[2].Path, resp.Msg.Files[2].ChangeCount)
	}
}

// ============================================================================
// SC-2: Response structure verification
// ============================================================================

// TestGetTopFiles_ResponseContainsAllRequiredFields verifies SC-2:
// Each file must include path, change_count, additions, and deletions fields.
func TestGetTopFiles_ResponseContainsAllRequiredFields(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "src/main.go", Additions: 150, Deletions: 30},
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) == 0 {
		t.Fatal("expected at least one file")
	}

	file := resp.Msg.Files[0]

	// Verify all required fields are present and correct
	if file.Path != "src/main.go" {
		t.Errorf("expected path 'src/main.go', got '%s'", file.Path)
	}
	if file.ChangeCount != 1 {
		t.Errorf("expected change_count 1, got %d", file.ChangeCount)
	}
	if file.Additions != 150 {
		t.Errorf("expected additions 150, got %d", file.Additions)
	}
	if file.Deletions != 30 {
		t.Errorf("expected deletions 30, got %d", file.Deletions)
	}
}

// ============================================================================
// SC-3: Aggregation across multiple tasks
// ============================================================================

// TestGetTopFiles_AggregatesAcrossCompletedTasks verifies SC-3:
// Aggregates file changes across all completed tasks with branches.
func TestGetTopFiles_AggregatesAcrossCompletedTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create 2 completed tasks that both modify the same file
	for i := 1; i <= 2; i++ {
		taskID := "TASK-00" + string(rune('0'+i))
		tk := task.NewProtoTask(taskID, "Test task "+taskID)
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		branchName := "orc/" + taskID
		tk.Branch = branchName

		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Both tasks modify src/main.go with different stats
		diffSvc.filesByBranch[branchName] = []diff.FileDiff{
			{Path: "src/main.go", Additions: 50 * i, Deletions: 10 * i},
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 aggregated file, got %d", len(resp.Msg.Files))
	}

	file := resp.Msg.Files[0]

	// change_count should be 2 (modified by 2 tasks)
	if file.ChangeCount != 2 {
		t.Errorf("expected change_count 2, got %d", file.ChangeCount)
	}

	// additions should be summed: 50 + 100 = 150
	if file.Additions != 150 {
		t.Errorf("expected additions 150 (50+100), got %d", file.Additions)
	}

	// deletions should be summed: 10 + 20 = 30
	if file.Deletions != 30 {
		t.Errorf("expected deletions 30 (10+20), got %d", file.Deletions)
	}
}

// TestGetTopFiles_OnlyIncludesCompletedTasks verifies SC-3:
// Only completed tasks should be included in aggregation.
func TestGetTopFiles_OnlyIncludesCompletedTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task
	completedTask := task.NewProtoTask("TASK-001", "Completed task")
	completedTask.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completedTask.CompletedAt = timestamppb.Now()
	completedTask.Branch = "orc/TASK-001"
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "completed.go", Additions: 100, Deletions: 50},
	}

	// Create running task (should be excluded)
	runningTask := task.NewProtoTask("TASK-002", "Running task")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.Branch = "orc/TASK-002"
	diffSvc.filesByBranch["orc/TASK-002"] = []diff.FileDiff{
		{Path: "running.go", Additions: 200, Deletions: 100},
	}

	// Create failed task (should be excluded)
	failedTask := task.NewProtoTask("TASK-003", "Failed task")
	failedTask.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	failedTask.Branch = "orc/TASK-003"
	diffSvc.filesByBranch["orc/TASK-003"] = []diff.FileDiff{
		{Path: "failed.go", Additions: 300, Deletions: 150},
	}

	for _, tk := range []*orcv1.Task{completedTask, runningTask, failedTask} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should only include the completed task's file
	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file from completed task only, got %d", len(resp.Msg.Files))
	}

	if resp.Msg.Files[0].Path != "completed.go" {
		t.Errorf("expected file from completed task, got '%s'", resp.Msg.Files[0].Path)
	}
}

// ============================================================================
// SC-4: Handle tasks without branches gracefully
// ============================================================================

// TestGetTopFiles_SkipsTasksWithoutBranch verifies SC-4:
// Tasks without branches should be skipped without error.
func TestGetTopFiles_SkipsTasksWithoutBranch(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task WITH branch
	taskWithBranch := task.NewProtoTask("TASK-001", "Task with branch")
	taskWithBranch.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	taskWithBranch.CompletedAt = timestamppb.Now()
	taskWithBranch.Branch = "orc/TASK-001"
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "with_branch.go", Additions: 50, Deletions: 25},
	}

	// Create completed task WITHOUT branch (empty string is the zero value)
	taskNoBranch := task.NewProtoTask("TASK-002", "Task without branch")
	taskNoBranch.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	taskNoBranch.CompletedAt = timestamppb.Now()
	// Branch is empty string (zero value)

	for _, tk := range []*orcv1.Task{taskWithBranch, taskNoBranch} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error when some tasks lack branches: %v", err)
	}

	// Should only include the task with branch
	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file from task with branch, got %d", len(resp.Msg.Files))
	}

	if resp.Msg.Files[0].Path != "with_branch.go" {
		t.Errorf("expected file from task with branch, got '%s'", resp.Msg.Files[0].Path)
	}
}

// TestGetTopFiles_SkipsTasksWithEmptyBranch verifies SC-4:
// Tasks with empty branch string should be skipped.
func TestGetTopFiles_SkipsTasksWithEmptyBranch(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create completed task with empty branch string
	taskEmptyBranch := task.NewProtoTask("TASK-001", "Task with empty branch")
	taskEmptyBranch.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	taskEmptyBranch.CompletedAt = timestamppb.Now()
	taskEmptyBranch.Branch = "" // Empty branch

	if err := backend.SaveTask(taskEmptyBranch); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error with empty branch: %v", err)
	}

	// Should return empty list (no valid tasks)
	if len(resp.Msg.Files) != 0 {
		t.Errorf("expected 0 files with empty branch, got %d", len(resp.Msg.Files))
	}
}

// ============================================================================
// SC-7: Additions and deletions from git diff
// ============================================================================

// TestGetTopFiles_AggregatesAdditionsAndDeletions verifies SC-7:
// Additions and deletions should be summed across all tasks modifying the same file.
func TestGetTopFiles_AggregatesAdditionsAndDeletions(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create 3 tasks all modifying the same file
	for i := 1; i <= 3; i++ {
		taskID := "TASK-00" + string(rune('0'+i))
		tk := task.NewProtoTask(taskID, "Test task "+taskID)
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		branchName := "orc/" + taskID
		tk.Branch = branchName

		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Each task adds different amounts
		diffSvc.filesByBranch[branchName] = []diff.FileDiff{
			{Path: "shared.go", Additions: 10 * i, Deletions: 5 * i},
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 aggregated file, got %d", len(resp.Msg.Files))
	}

	file := resp.Msg.Files[0]

	// additions: 10 + 20 + 30 = 60
	expectedAdditions := int32(60)
	if file.Additions != expectedAdditions {
		t.Errorf("expected additions %d, got %d", expectedAdditions, file.Additions)
	}

	// deletions: 5 + 10 + 15 = 30
	expectedDeletions := int32(30)
	if file.Deletions != expectedDeletions {
		t.Errorf("expected deletions %d, got %d", expectedDeletions, file.Deletions)
	}
}

// TestGetTopFiles_BinaryFilesIncludedWithZeroStats verifies edge case:
// Binary files should be included with 0 additions/deletions.
func TestGetTopFiles_BinaryFilesIncludedWithZeroStats(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Binary file with no additions/deletions
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "image.png", Binary: true, Additions: 0, Deletions: 0},
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Binary file should be included
	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file (binary included), got %d", len(resp.Msg.Files))
	}

	file := resp.Msg.Files[0]
	if file.Path != "image.png" {
		t.Errorf("expected binary file 'image.png', got '%s'", file.Path)
	}
	if file.Additions != 0 || file.Deletions != 0 {
		t.Errorf("expected 0 additions/deletions for binary file, got %d/%d",
			file.Additions, file.Deletions)
	}
}

// TestGetTopFiles_DeletedFilesIncluded verifies edge case:
// Deleted files should be included in results.
func TestGetTopFiles_DeletedFilesIncluded(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Deleted file (only deletions, no additions)
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "deprecated.go", Status: "deleted", Additions: 0, Deletions: 100},
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file (deleted included), got %d", len(resp.Msg.Files))
	}

	file := resp.Msg.Files[0]
	if file.Path != "deprecated.go" {
		t.Errorf("expected deleted file 'deprecated.go', got '%s'", file.Path)
	}
	if file.Deletions != 100 {
		t.Errorf("expected 100 deletions for deleted file, got %d", file.Deletions)
	}
}

// ============================================================================
// Empty/Error cases
// ============================================================================

// TestGetTopFiles_NoCompletedTasks_ReturnsEmpty verifies failure mode:
// When no completed tasks exist, should return empty array (not error).
func TestGetTopFiles_NoCompletedTasks_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create only non-completed tasks
	runningTask := task.NewProtoTask("TASK-001", "Running task")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.Branch = "orc/TASK-001"

	if err := backend.SaveTask(runningTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error with no completed tasks: %v", err)
	}

	if len(resp.Msg.Files) != 0 {
		t.Errorf("expected empty files array, got %d files", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_NoTasks_ReturnsEmpty verifies failure mode:
// When no tasks exist at all, should return empty array.
func TestGetTopFiles_NoTasks_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error with no tasks: %v", err)
	}

	if len(resp.Msg.Files) != 0 {
		t.Errorf("expected empty files array, got %d files", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_GitError_SkipsTaskContinues verifies failure mode:
// When git diff fails for a task, skip it and continue with others.
func TestGetTopFiles_GitError_SkipsTaskContinues(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create task that will fail git diff
	failingTask := task.NewProtoTask("TASK-001", "Failing task")
	failingTask.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	failingTask.CompletedAt = timestamppb.Now()
	failingTask.Branch = "orc/TASK-001"
	diffSvc.errorByBranch["orc/TASK-001"] = true // Will return error

	// Create task that will succeed
	goodTask := task.NewProtoTask("TASK-002", "Good task")
	goodTask.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	goodTask.CompletedAt = timestamppb.Now()
	goodTask.Branch = "orc/TASK-002"
	diffSvc.filesByBranch["orc/TASK-002"] = []diff.FileDiff{
		{Path: "good.go", Additions: 50, Deletions: 25},
	}

	for _, tk := range []*orcv1.Task{failingTask, goodTask} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error when some diffs fail: %v", err)
	}

	// Should still return the good task's file
	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file from good task, got %d", len(resp.Msg.Files))
	}

	if resp.Msg.Files[0].Path != "good.go" {
		t.Errorf("expected file from good task, got '%s'", resp.Msg.Files[0].Path)
	}
}

// TestGetTopFiles_EmptyDiff_SkipsFile verifies edge case:
// Tasks with empty diffs should not contribute to file counts.
func TestGetTopFiles_EmptyDiff_SkipsFile(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Empty diff - no files changed
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 0 {
		t.Errorf("expected 0 files from empty diff, got %d", len(resp.Msg.Files))
	}
}

// ============================================================================
// Task filtering via task_id parameter
// ============================================================================

// TestGetTopFiles_FilterByTaskID verifies task_id parameter:
// When task_id is provided, only include files from that task.
func TestGetTopFiles_FilterByTaskID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create two completed tasks
	task1 := task.NewProtoTask("TASK-001", "Task 1")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task1.CompletedAt = timestamppb.Now()
	task1.Branch = "orc/TASK-001"
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "task1_file.go", Additions: 100, Deletions: 50},
	}

	task2 := task.NewProtoTask("TASK-002", "Task 2")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task2.CompletedAt = timestamppb.Now()
	task2.Branch = "orc/TASK-002"
	diffSvc.filesByBranch["orc/TASK-002"] = []diff.FileDiff{
		{Path: "task2_file.go", Additions: 200, Deletions: 100},
	}

	for _, tk := range []*orcv1.Task{task1, task2} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Filter by TASK-001
	taskID := "TASK-001"
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit:  10,
		TaskId: &taskID,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	// Should only include task1's file
	if len(resp.Msg.Files) != 1 {
		t.Fatalf("expected 1 file from TASK-001, got %d", len(resp.Msg.Files))
	}

	if resp.Msg.Files[0].Path != "task1_file.go" {
		t.Errorf("expected 'task1_file.go', got '%s'", resp.Msg.Files[0].Path)
	}
}

// TestGetTopFiles_FilterByTaskID_NotFound verifies task_id parameter:
// When task_id doesn't exist, return empty array (not error).
func TestGetTopFiles_FilterByTaskID_NotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create a task
	tk := task.NewProtoTask("TASK-001", "Task 1")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	tk.Branch = "orc/TASK-001"
	diffSvc.filesByBranch["orc/TASK-001"] = []diff.FileDiff{
		{Path: "file.go", Additions: 100, Deletions: 50},
	}

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	// Filter by non-existent task
	taskID := "TASK-999"
	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit:  10,
		TaskId: &taskID,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles should not error with non-existent task_id: %v", err)
	}

	if len(resp.Msg.Files) != 0 {
		t.Errorf("expected 0 files for non-existent task_id, got %d", len(resp.Msg.Files))
	}
}

// ============================================================================
// BDD Scenarios from spec
// ============================================================================

// TestGetTopFiles_BDD1_LimitEnforcement verifies BDD-1:
// Given 10 completed tasks each modifying different files
// When GetTopFiles is called with limit=5
// Then response contains exactly 5 files with highest change_count
func TestGetTopFiles_BDD1_LimitEnforcement(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create 10 completed tasks, each modifying a unique file
	for i := 1; i <= 10; i++ {
		taskID := "TASK-0" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		tk := task.NewProtoTask(taskID, "Test task")
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		branchName := "orc/" + taskID
		tk.Branch = branchName

		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Each task modifies a unique file
		fileName := "file" + string(rune('a'-1+i)) + ".go"
		diffSvc.filesByBranch[branchName] = []diff.FileDiff{
			{Path: fileName, Additions: 100 - i, Deletions: 50 - i},
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 5,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 5 {
		t.Errorf("BDD-1: expected exactly 5 files, got %d", len(resp.Msg.Files))
	}
}

// TestGetTopFiles_BDD3_Aggregation verifies BDD-3:
// Given 2 tasks both modified `src/main.go`
// When GetTopFiles is called
// Then `src/main.go` has change_count=2 and additions/deletions are summed
func TestGetTopFiles_BDD3_Aggregation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	diffSvc := newMockDiffService()

	// Create 2 tasks both modifying src/main.go
	for i := 1; i <= 2; i++ {
		taskID := "TASK-00" + string(rune('0'+i))
		tk := task.NewProtoTask(taskID, "Test task")
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		branchName := "orc/" + taskID
		tk.Branch = branchName

		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		diffSvc.filesByBranch[branchName] = []diff.FileDiff{
			{Path: "src/main.go", Additions: 50, Deletions: 20},
		}
	}

	server := NewDashboardServerWithDiff(backend, nil, diffSvc)

	req := connect.NewRequest(&orcv1.GetTopFilesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopFiles(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopFiles failed: %v", err)
	}

	if len(resp.Msg.Files) != 1 {
		t.Fatalf("BDD-3: expected 1 aggregated file, got %d", len(resp.Msg.Files))
	}

	file := resp.Msg.Files[0]
	if file.Path != "src/main.go" {
		t.Errorf("BDD-3: expected path 'src/main.go', got '%s'", file.Path)
	}
	if file.ChangeCount != 2 {
		t.Errorf("BDD-3: expected change_count=2, got %d", file.ChangeCount)
	}
	// Summed: 50+50=100 additions, 20+20=40 deletions
	if file.Additions != 100 {
		t.Errorf("BDD-3: expected additions=100, got %d", file.Additions)
	}
	if file.Deletions != 40 {
		t.Errorf("BDD-3: expected deletions=40, got %d", file.Deletions)
	}
}

// ============================================================================
// Mock diff service for testing
// ============================================================================

// mockDiffService implements the minimum interface needed for testing.
type mockDiffService struct {
	filesByBranch map[string][]diff.FileDiff
	errorByBranch map[string]bool
	baseBranch    string
}

func newMockDiffService() *mockDiffService {
	return &mockDiffService{
		filesByBranch: make(map[string][]diff.FileDiff),
		errorByBranch: make(map[string]bool),
		baseBranch:    "main",
	}
}

// GetFileList returns mock diff data for a branch.
func (m *mockDiffService) GetFileList(ctx context.Context, base, head string) ([]diff.FileDiff, error) {
	if m.errorByBranch[head] {
		return nil, context.DeadlineExceeded // Simulate git error
	}
	files, ok := m.filesByBranch[head]
	if !ok {
		return []diff.FileDiff{}, nil
	}
	return files, nil
}

// Ensure mockDiffService implements DiffServicer interface from dashboard_server.go
var _ DiffServicer = (*mockDiffService)(nil)

// ============================================================================
// SC-8: Ensure existing tests still pass
// ============================================================================

// Note: SC-8 is verified by running `go test ./internal/api/...` after implementation.
// The existing tests in dashboard_server_test.go should continue to pass.
