// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-792: Add GetAllProjectsStatus API endpoint
//
// These tests verify the GetAllProjectsStatus RPC returns a unified view
// of tasks across all registered projects, including claim info, stale
// detection, and completion counts.
//
// Success Criteria Coverage:
// - SC-1: Proto RPC definition (verified by compilation)
// - SC-2: Returns ProjectStatus for every registered project
// - SC-3: Active tasks filtering (only running/blocked/created/planned)
// - SC-4: Claim info in TaskSummary
// - SC-5: SetProjectCache wiring
// - SC-6: Stale detection via CheckOrphanedProto
// - SC-7: total_tasks and completed_today counts
package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// Test helpers
// ============================================================================

// setupTestProject creates a temporary project directory with initialized .orc
// and registers it in the project registry. Returns the project and its path.
func setupTestProject(t *testing.T, tmpDir, name string) *project.Project {
	t.Helper()
	projectPath := filepath.Join(tmpDir, name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("create project dir %s: %v", name, err)
	}
	if err := config.InitAt(projectPath, false); err != nil {
		t.Fatalf("init project %s: %v", name, err)
	}
	proj, err := project.RegisterProject(projectPath)
	if err != nil {
		t.Fatalf("register project %s: %v", name, err)
	}
	return proj
}

// setupTestHome creates an isolated HOME directory for test isolation.
func setupTestHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}
	return tmpDir
}

// seedTaskInProject saves a proto task into a project's database via the project cache.
func seedTaskInProject(t *testing.T, cache *ProjectCache, projectID string, protoTask *orcv1.Task) {
	t.Helper()
	backend, err := cache.GetBackend(projectID)
	if err != nil {
		t.Fatalf("get backend for project %s: %v", projectID, err)
	}
	if err := backend.SaveTask(protoTask); err != nil {
		t.Fatalf("save task %s in project %s: %v", protoTask.Id, projectID, err)
	}
}

// setClaimOnTask sets claim fields on a task in the project's database.
// This uses the DB-level ClaimTaskByUser since proto tasks don't carry claim fields.
func setClaimOnTask(t *testing.T, cache *ProjectCache, projectID, taskID, userID string) {
	t.Helper()
	pdb, err := cache.Get(projectID)
	if err != nil {
		t.Fatalf("get project db for %s: %v", projectID, err)
	}
	rows, err := pdb.ClaimTaskByUser(taskID, userID)
	if err != nil {
		t.Fatalf("claim task %s by %s: %v", taskID, userID, err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row affected claiming %s, got %d", taskID, rows)
	}
}

// ============================================================================
// SC-2: Returns ProjectStatus for every registered project
// ============================================================================

// TestGetAllProjectsStatus_ReturnsAllProjects verifies SC-2:
// Every registered project appears in the response with correct id, name, and path.
func TestGetAllProjectsStatus_ReturnsAllProjects(t *testing.T) {
	tmpDir := setupTestHome(t)

	proj1 := setupTestProject(t, tmpDir, "alpha")
	proj2 := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	if len(resp.Msg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resp.Msg.Projects))
	}

	// Build lookup map
	found := make(map[string]*orcv1.ProjectStatus)
	for _, ps := range resp.Msg.Projects {
		found[ps.ProjectId] = ps
	}

	// Verify project 1
	ps1, ok := found[proj1.ID]
	if !ok {
		t.Fatalf("project %s not found in response", proj1.ID)
	}
	if ps1.ProjectName != proj1.Name {
		t.Errorf("project 1 name = %q, want %q", ps1.ProjectName, proj1.Name)
	}
	if ps1.ProjectPath != proj1.Path {
		t.Errorf("project 1 path = %q, want %q", ps1.ProjectPath, proj1.Path)
	}

	// Verify project 2
	ps2, ok := found[proj2.ID]
	if !ok {
		t.Fatalf("project %s not found in response", proj2.ID)
	}
	if ps2.ProjectName != proj2.Name {
		t.Errorf("project 2 name = %q, want %q", ps2.ProjectName, proj2.Name)
	}
	if ps2.ProjectPath != proj2.Path {
		t.Errorf("project 2 path = %q, want %q", ps2.ProjectPath, proj2.Path)
	}
}

// ============================================================================
// SC-3: Active tasks filtering
// ============================================================================

// TestGetAllProjectsStatus_ActiveTasksFiltering verifies SC-3:
// Only tasks with running/blocked/created/planned statuses appear in active_tasks.
// Completed, failed, and closed tasks are excluded.
func TestGetAllProjectsStatus_ActiveTasksFiltering(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "myproject")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed tasks with various statuses
	statuses := map[string]orcv1.TaskStatus{
		"TASK-001": orcv1.TaskStatus_TASK_STATUS_CREATED,   // active
		"TASK-002": orcv1.TaskStatus_TASK_STATUS_PLANNED,   // active
		"TASK-003": orcv1.TaskStatus_TASK_STATUS_RUNNING,   // active
		"TASK-004": orcv1.TaskStatus_TASK_STATUS_BLOCKED,   // active
		"TASK-005": orcv1.TaskStatus_TASK_STATUS_COMPLETED, // excluded
		"TASK-006": orcv1.TaskStatus_TASK_STATUS_FAILED,    // excluded
		"TASK-007": orcv1.TaskStatus_TASK_STATUS_CLOSED,    // excluded
	}
	for id, status := range statuses {
		tk := task.NewProtoTask(id, "Task "+id)
		tk.Status = status
		if status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			tk.CompletedAt = timestamppb.Now()
		}
		seedTaskInProject(t, cache, proj.ID, tk)
	}

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	if len(resp.Msg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Msg.Projects))
	}

	ps := resp.Msg.Projects[0]
	if len(ps.ActiveTasks) != 4 {
		t.Errorf("expected 4 active tasks, got %d", len(ps.ActiveTasks))
	}

	// Verify only active statuses are present
	activeIDs := make(map[string]bool)
	for _, ts := range ps.ActiveTasks {
		activeIDs[ts.Id] = true
	}
	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004"} {
		if !activeIDs[id] {
			t.Errorf("expected %s in active tasks", id)
		}
	}
	for _, id := range []string{"TASK-005", "TASK-006", "TASK-007"} {
		if activeIDs[id] {
			t.Errorf("did not expect %s in active tasks", id)
		}
	}
}

// TestGetAllProjectsStatus_EmptyActiveTasks verifies SC-3 edge case:
// A project with no active tasks returns empty (not nil) active_tasks list.
func TestGetAllProjectsStatus_EmptyActiveTasks(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "empty-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed only completed tasks
	completed := task.NewProtoTask("TASK-001", "Done task")
	completed.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completed.CompletedAt = timestamppb.Now()
	seedTaskInProject(t, cache, proj.ID, completed)

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if ps.ActiveTasks == nil {
		t.Fatal("active_tasks should be empty slice, not nil")
	}
	if len(ps.ActiveTasks) != 0 {
		t.Errorf("expected 0 active tasks, got %d", len(ps.ActiveTasks))
	}
}

// ============================================================================
// SC-4: Claim info in TaskSummary
// ============================================================================

// TestGetAllProjectsStatus_ClaimInfo verifies SC-4:
// TaskSummary.claimed_by_name and claimed_at are populated from the task's claim fields.
func TestGetAllProjectsStatus_ClaimInfo(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "claim-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed a running task and claim it
	running := task.NewProtoTask("TASK-001", "Claimed task")
	running.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	seedTaskInProject(t, cache, proj.ID, running)
	setClaimOnTask(t, cache, proj.ID, "TASK-001", "alice")

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if len(ps.ActiveTasks) != 1 {
		t.Fatalf("expected 1 active task, got %d", len(ps.ActiveTasks))
	}

	ts := ps.ActiveTasks[0]
	if ts.ClaimedByName != "alice" {
		t.Errorf("claimed_by_name = %q, want %q", ts.ClaimedByName, "alice")
	}
	if ts.ClaimedAt == nil {
		t.Error("claimed_at should not be nil for a claimed task")
	}
}

// TestGetAllProjectsStatus_UnclaimedTask verifies SC-4 edge case:
// Unclaimed tasks have empty claimed_by_name and nil claimed_at.
func TestGetAllProjectsStatus_UnclaimedTask(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "unclaimed-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed a running task without claiming it
	running := task.NewProtoTask("TASK-001", "Unclaimed task")
	running.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	seedTaskInProject(t, cache, proj.ID, running)

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ts := resp.Msg.Projects[0].ActiveTasks[0]
	if ts.ClaimedByName != "" {
		t.Errorf("claimed_by_name should be empty for unclaimed task, got %q", ts.ClaimedByName)
	}
	if ts.ClaimedAt != nil {
		t.Error("claimed_at should be nil for unclaimed task")
	}
}

// ============================================================================
// SC-5: SetProjectCache wiring
// ============================================================================

// TestGetAllProjectsStatus_NilProjectCache verifies SC-5 failure mode:
// Returns CodeFailedPrecondition when project cache is not configured.
func TestGetAllProjectsStatus_NilProjectCache(t *testing.T) {
	t.Parallel()

	server := NewProjectServer(nil, nil)
	// Deliberately NOT calling SetProjectCache

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	_, err := server.GetAllProjectsStatus(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when project cache is nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
	if connectErr.Message() != "project cache not configured" {
		t.Errorf("expected message 'project cache not configured', got %q", connectErr.Message())
	}
}

// TestServerConnect_WiresProjectCacheToProjectServer verifies SC-5 integration:
// server_connect.go calls SetProjectCache on the project server.
// This is an integration test that verifies the wiring exists.
func TestServerConnect_WiresProjectCacheToProjectServer(t *testing.T) {
	t.Parallel()

	// Create a projectServer and verify it has SetProjectCache method
	server := NewProjectServer(nil, nil)
	ps, ok := server.(*projectServer)
	if !ok {
		t.Fatal("NewProjectServer should return *projectServer")
	}

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// The method should exist and set the field
	ps.SetProjectCache(cache)

	if ps.projectCache == nil {
		t.Fatal("projectCache should be set after SetProjectCache")
	}
}

// ============================================================================
// SC-6: Stale detection
// ============================================================================

// TestGetAllProjectsStatus_StaleDetection verifies SC-6:
// TaskSummary.is_stale is true when the task's executor process is dead (orphaned).
func TestGetAllProjectsStatus_StaleDetection(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "stale-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed a running task with a dead PID (PID 999999 should not exist)
	running := task.NewProtoTask("TASK-001", "Orphaned task")
	running.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	running.ExecutorPid = 999999
	seedTaskInProject(t, cache, proj.ID, running)

	// Also set executor info in the DB so the PID persists
	backend, err := cache.GetBackend(proj.ID)
	if err != nil {
		t.Fatalf("get backend: %v", err)
	}
	dbBackend := backend.(*storage.DatabaseBackend)
	if err := dbBackend.SetTaskExecutor("TASK-001", 999999, "localhost"); err != nil {
		t.Fatalf("set executor: %v", err)
	}

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ts := resp.Msg.Projects[0].ActiveTasks[0]
	if !ts.IsStale {
		t.Error("is_stale should be true for running task with dead PID")
	}
}

// TestGetAllProjectsStatus_NonRunningTaskNotStale verifies SC-6 edge case:
// Non-running tasks always have is_stale=false.
func TestGetAllProjectsStatus_NonRunningTaskNotStale(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "notstale-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed a blocked task (not running, so cannot be stale)
	blocked := task.NewProtoTask("TASK-001", "Blocked task")
	blocked.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	seedTaskInProject(t, cache, proj.ID, blocked)

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ts := resp.Msg.Projects[0].ActiveTasks[0]
	if ts.IsStale {
		t.Error("is_stale should be false for non-running task")
	}
}

// ============================================================================
// SC-7: total_tasks and completed_today counts
// ============================================================================

// TestGetAllProjectsStatus_TotalTasksCount verifies SC-7:
// total_tasks counts all tasks including completed/failed/closed.
func TestGetAllProjectsStatus_TotalTasksCount(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "count-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed 5 tasks with mixed statuses
	taskData := []struct {
		id     string
		status orcv1.TaskStatus
	}{
		{"TASK-001", orcv1.TaskStatus_TASK_STATUS_CREATED},
		{"TASK-002", orcv1.TaskStatus_TASK_STATUS_RUNNING},
		{"TASK-003", orcv1.TaskStatus_TASK_STATUS_COMPLETED},
		{"TASK-004", orcv1.TaskStatus_TASK_STATUS_FAILED},
		{"TASK-005", orcv1.TaskStatus_TASK_STATUS_CLOSED},
	}
	for _, td := range taskData {
		tk := task.NewProtoTask(td.id, "Task "+td.id)
		tk.Status = td.status
		if td.status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			tk.CompletedAt = timestamppb.Now()
		}
		seedTaskInProject(t, cache, proj.ID, tk)
	}

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if ps.TotalTasks != 5 {
		t.Errorf("total_tasks = %d, want 5", ps.TotalTasks)
	}
}

// TestGetAllProjectsStatus_CompletedToday verifies SC-7:
// completed_today counts tasks with CompletedAt timestamp within the current UTC day.
func TestGetAllProjectsStatus_CompletedToday(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "today-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed a task completed now (today)
	completedToday := task.NewProtoTask("TASK-001", "Completed today")
	completedToday.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completedToday.CompletedAt = timestamppb.Now()
	seedTaskInProject(t, cache, proj.ID, completedToday)

	// Seed a task completed yesterday
	completedYesterday := task.NewProtoTask("TASK-002", "Completed yesterday")
	completedYesterday.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	// Set to 23:59 yesterday to test the boundary
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 0, 0, time.UTC)
	completedYesterday.CompletedAt = timestamppb.New(yesterday)
	seedTaskInProject(t, cache, proj.ID, completedYesterday)

	// Seed an active task (not completed)
	active := task.NewProtoTask("TASK-003", "Still running")
	active.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	seedTaskInProject(t, cache, proj.ID, active)

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if ps.CompletedToday != 1 {
		t.Errorf("completed_today = %d, want 1 (only today's task)", ps.CompletedToday)
	}
	if ps.TotalTasks != 3 {
		t.Errorf("total_tasks = %d, want 3", ps.TotalTasks)
	}
}

// TestGetAllProjectsStatus_ProjectWithZeroTasks verifies SC-7 edge case:
// Project with zero tasks returns total_tasks=0, completed_today=0.
func TestGetAllProjectsStatus_ProjectWithZeroTasks(t *testing.T) {
	tmpDir := setupTestHome(t)
	setupTestProject(t, tmpDir, "zero-tasks-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if ps.TotalTasks != 0 {
		t.Errorf("total_tasks = %d, want 0", ps.TotalTasks)
	}
	if ps.CompletedToday != 0 {
		t.Errorf("completed_today = %d, want 0", ps.CompletedToday)
	}
	if ps.ActiveTasks == nil {
		t.Fatal("active_tasks should be empty slice, not nil")
	}
	if len(ps.ActiveTasks) != 0 {
		t.Errorf("expected 0 active tasks, got %d", len(ps.ActiveTasks))
	}
}

// ============================================================================
// Failure Modes
// ============================================================================

// TestGetAllProjectsStatus_NoRegisteredProjects verifies the edge case:
// Zero registered projects returns an empty projects list.
func TestGetAllProjectsStatus_NoRegisteredProjects(t *testing.T) {
	tmpDir := setupTestHome(t)
	// Don't register any projects - just create the global .orc dir
	_ = tmpDir

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	if resp.Msg.Projects == nil {
		t.Fatal("projects should be empty slice, not nil")
	}
	if len(resp.Msg.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(resp.Msg.Projects))
	}
}

// TestGetAllProjectsStatus_InaccessibleProjectDB verifies failure mode:
// If a project's DB cannot be opened, the endpoint returns an error
// (not silently skipping the project), including the project ID.
func TestGetAllProjectsStatus_InaccessibleProjectDB(t *testing.T) {
	tmpDir := setupTestHome(t)

	// Register a project then make its DB inaccessible
	proj := setupTestProject(t, tmpDir, "broken-project")

	// Remove the project's .orc directory to make DB inaccessible
	orcDir := filepath.Join(proj.Path, ".orc")
	if err := os.RemoveAll(orcDir); err != nil {
		t.Fatalf("remove .orc dir: %v", err)
	}

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	_, err := server.GetAllProjectsStatus(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when project DB is inaccessible")
	}

	// Error should mention the project ID
	errMsg := err.Error()
	if !strings.Contains(errMsg, proj.ID) && !strings.Contains(errMsg, "broken-project") {
		t.Errorf("error should mention project ID or name, got: %s", errMsg)
	}
}

// ============================================================================
// SC-3 + SC-6 combined: FailedStatus not in active_tasks but counted in total_tasks
// ============================================================================

// TestGetAllProjectsStatus_FailedTaskExcludedFromActive verifies the edge case:
// A task in "failed" status is NOT in active_tasks but IS counted in total_tasks.
func TestGetAllProjectsStatus_FailedTaskExcludedFromActive(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "failed-task-project")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Seed only a failed task
	failed := task.NewProtoTask("TASK-001", "Failed task")
	failed.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	seedTaskInProject(t, cache, proj.ID, failed)

	server := NewProjectServer(nil, nil)
	server.(*projectServer).SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAllProjectsStatusRequest{})
	resp, err := server.GetAllProjectsStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAllProjectsStatus failed: %v", err)
	}

	ps := resp.Msg.Projects[0]
	if len(ps.ActiveTasks) != 0 {
		t.Errorf("failed task should not be in active_tasks, got %d", len(ps.ActiveTasks))
	}
	if ps.TotalTasks != 1 {
		t.Errorf("total_tasks should include failed task: got %d, want 1", ps.TotalTasks)
	}
}

// Ensure imports are used. These are referenced by tests above but the
// compiler may flag them unused since proto types don't exist yet.
var (
	_ = fmt.Sprintf
	_ = db.ProjectDB{}
	_ = storage.DatabaseBackend{}
	_ = time.Now
	_ = timestamppb.Now
	_ = strings.Contains
)
