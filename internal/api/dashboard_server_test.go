// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-553: Stats page shows Avg Task Time as 0:00 and
// Most Active Initiatives as No data
//
// These tests verify the GetTopInitiatives API returns initiative titles
// (not just IDs) for the Most Active Initiatives leaderboard.
//
// Success Criteria Coverage:
// - SC-5: Initiative leaderboard shows initiative title (not ID)
//
// The fix: GetTopInitiatives must load the real initiative title from storage
// instead of using the initiative ID as the title placeholder.
package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-5: GetTopInitiatives returns initiative titles (not IDs)
// ============================================================================

// TestGetTopInitiatives_ReturnsInitiativeTitle verifies SC-5:
// The API response should include the initiative's title, not just the ID.
func TestGetTopInitiatives_ReturnsInitiativeTitle(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative with a specific title
	init := initiative.NewProtoInitiative("INIT-001", "User Authentication Feature")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks linked to this initiative
	initID := "INIT-001"
	task1 := task.NewProtoTask("TASK-001", "Login endpoint")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task1.InitiativeId = &initID
	task1.CompletedAt = timestamppb.Now()

	task2 := task.NewProtoTask("TASK-002", "Logout endpoint")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task2.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task2.InitiativeId = &initID

	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	// VERIFY SC-5: Should return title "User Authentication Feature", not ID "INIT-001"
	if len(resp.Msg.Initiatives) == 0 {
		t.Fatal("expected at least one initiative")
	}

	topInit := resp.Msg.Initiatives[0]
	if topInit.Title != "User Authentication Feature" {
		t.Errorf("expected title 'User Authentication Feature', got '%s'", topInit.Title)
	}
	if topInit.Title == "INIT-001" {
		t.Error("title should be the initiative title, not the ID")
	}
	if topInit.Id != "INIT-001" {
		t.Errorf("expected ID 'INIT-001', got '%s'", topInit.Id)
	}
	if topInit.TaskCount != 2 {
		t.Errorf("expected task count 2, got %d", topInit.TaskCount)
	}
}

// TestGetTopInitiatives_MultipleInitiativesSortedByTaskCount verifies SC-4:
// Initiatives should be sorted by task count descending.
func TestGetTopInitiatives_MultipleInitiativesSortedByTaskCount(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create three initiatives with different titles
	initA := initiative.NewProtoInitiative("INIT-A", "Initiative A")
	initB := initiative.NewProtoInitiative("INIT-B", "Initiative B")
	initC := initiative.NewProtoInitiative("INIT-C", "Initiative C")

	for _, init := range []*orcv1.Initiative{initA, initB, initC} {
		if err := backend.SaveInitiativeProto(init); err != nil {
			t.Fatalf("save initiative %s: %v", init.Id, err)
		}
	}

	// Create tasks:
	// - INIT-A: 10 tasks
	// - INIT-B: 5 tasks
	// - INIT-C: 2 tasks
	initAID := "INIT-A"
	initBID := "INIT-B"
	initCID := "INIT-C"

	// 10 tasks for INIT-A
	for i := 0; i < 10; i++ {
		tk := task.NewProtoTask("TASK-A-"+string(rune('0'+i)), "Task A")
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.InitiativeId = &initAID
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// 5 tasks for INIT-B
	for i := 0; i < 5; i++ {
		tk := task.NewProtoTask("TASK-B-"+string(rune('0'+i)), "Task B")
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.InitiativeId = &initBID
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// 2 tasks for INIT-C
	for i := 0; i < 2; i++ {
		tk := task.NewProtoTask("TASK-C-"+string(rune('0'+i)), "Task C")
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.InitiativeId = &initCID
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	// VERIFY SC-4: Should be sorted by task count descending
	if len(resp.Msg.Initiatives) != 3 {
		t.Fatalf("expected 3 initiatives, got %d", len(resp.Msg.Initiatives))
	}

	// First should be Initiative A with 10 tasks
	if resp.Msg.Initiatives[0].Title != "Initiative A" {
		t.Errorf("first initiative should be 'Initiative A', got '%s'", resp.Msg.Initiatives[0].Title)
	}
	if resp.Msg.Initiatives[0].TaskCount != 10 {
		t.Errorf("first initiative should have 10 tasks, got %d", resp.Msg.Initiatives[0].TaskCount)
	}

	// Second should be Initiative B with 5 tasks
	if resp.Msg.Initiatives[1].Title != "Initiative B" {
		t.Errorf("second initiative should be 'Initiative B', got '%s'", resp.Msg.Initiatives[1].Title)
	}
	if resp.Msg.Initiatives[1].TaskCount != 5 {
		t.Errorf("second initiative should have 5 tasks, got %d", resp.Msg.Initiatives[1].TaskCount)
	}

	// Third should be Initiative C with 2 tasks
	if resp.Msg.Initiatives[2].Title != "Initiative C" {
		t.Errorf("third initiative should be 'Initiative C', got '%s'", resp.Msg.Initiatives[2].Title)
	}
	if resp.Msg.Initiatives[2].TaskCount != 2 {
		t.Errorf("third initiative should have 2 tasks, got %d", resp.Msg.Initiatives[2].TaskCount)
	}
}

// TestGetTopInitiatives_LimitRespected verifies the limit parameter works.
func TestGetTopInitiatives_LimitRespected(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create 5 initiatives
	for i := 0; i < 5; i++ {
		id := "INIT-" + string(rune('A'+i))
		init := initiative.NewProtoInitiative(id, "Initiative "+string(rune('A'+i)))
		if err := backend.SaveInitiativeProto(init); err != nil {
			t.Fatalf("save initiative: %v", err)
		}

		// Create one task per initiative
		initID := id
		tk := task.NewProtoTask("TASK-"+id, "Task")
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.InitiativeId = &initID
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServer(backend, nil)

	// Request only 3 initiatives
	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 3,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	// Should return at most 3
	if len(resp.Msg.Initiatives) > 3 {
		t.Errorf("expected at most 3 initiatives, got %d", len(resp.Msg.Initiatives))
	}
}

// TestGetTopInitiatives_EmptyTitle_FallbackToID verifies edge case:
// When an initiative has an empty title, the API should fall back to the ID.
func TestGetTopInitiatives_EmptyTitle_FallbackToID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative with empty title
	init := &orcv1.Initiative{
		Id:    "INIT-001",
		Title: "", // Empty title
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create a task linked to this initiative
	initID := "INIT-001"
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.InitiativeId = &initID

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	if len(resp.Msg.Initiatives) == 0 {
		t.Fatal("expected at least one initiative")
	}

	// When title is empty, should fall back to ID
	topInit := resp.Msg.Initiatives[0]
	if topInit.Title != "INIT-001" {
		t.Errorf("expected title to fall back to ID 'INIT-001', got '%s'", topInit.Title)
	}
}

// TestGetTopInitiatives_InitiativeNotInStorage verifies edge case:
// When a task references an initiative that doesn't exist in storage,
// the API should use the initiative ID as the title.
func TestGetTopInitiatives_InitiativeNotInStorage(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task linked to an initiative that doesn't exist
	initID := "INIT-NONEXISTENT"
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.InitiativeId = &initID

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	if len(resp.Msg.Initiatives) == 0 {
		t.Fatal("expected at least one initiative")
	}

	// When initiative not found, should use ID as title (fallback behavior)
	topInit := resp.Msg.Initiatives[0]
	if topInit.Title != "INIT-NONEXISTENT" {
		t.Errorf("expected title to be ID 'INIT-NONEXISTENT', got '%s'", topInit.Title)
	}
}

// TestGetTopInitiatives_NoInitiatives verifies empty state handling.
func TestGetTopInitiatives_NoInitiatives(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// No tasks with initiatives
	tk := task.NewProtoTask("TASK-001", "Task without initiative")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	// No InitiativeId set

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetTopInitiativesRequest{
		Limit: 10,
	})

	resp, err := server.GetTopInitiatives(context.Background(), req)
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	// Should return empty list, not error
	if len(resp.Msg.Initiatives) != 0 {
		t.Errorf("expected 0 initiatives, got %d", len(resp.Msg.Initiatives))
	}
}

// ============================================================================
// GetMetrics tests for SC-1, SC-2: Avg Task Time
// ============================================================================

// TestGetMetrics_ReturnsAvgTaskDurationSeconds verifies SC-1:
// GetMetrics should calculate and return the average task duration.
func TestGetMetrics_ReturnsAvgTaskDurationSeconds(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	now := time.Now()

	// Create completed tasks with known durations
	// Task 1: 60 seconds
	task1 := task.NewProtoTask("TASK-001", "Task 1")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task1.StartedAt = timestamppb.New(now.Add(-2 * time.Hour))
	task1.CompletedAt = timestamppb.New(now.Add(-2*time.Hour + 60*time.Second))

	// Task 2: 120 seconds
	task2 := task.NewProtoTask("TASK-002", "Task 2")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task2.StartedAt = timestamppb.New(now.Add(-1 * time.Hour))
	task2.CompletedAt = timestamppb.New(now.Add(-1*time.Hour + 120*time.Second))

	// Task 3: 180 seconds
	task3 := task.NewProtoTask("TASK-003", "Task 3")
	task3.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task3.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task3.StartedAt = timestamppb.New(now.Add(-30 * time.Minute))
	task3.CompletedAt = timestamppb.New(now.Add(-30*time.Minute + 180*time.Second))

	for _, tk := range []*orcv1.Task{task1, task2, task3} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetMetricsRequest{})

	resp, err := server.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Average of 60, 120, 180 = 120 seconds
	expectedAvg := 120.0
	actualAvg := resp.Msg.Metrics.AvgTaskDurationSeconds
	if actualAvg != expectedAvg {
		t.Errorf("expected avg duration %v, got %v", expectedAvg, actualAvg)
	}
}

// TestGetMetrics_NoCompletedTasks_ReturnsZero verifies edge case:
// When no tasks have completed, avg duration should be 0.
func TestGetMetrics_NoCompletedTasks_ReturnsZero(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a running task (not completed)
	tk := task.NewProtoTask("TASK-001", "Running task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.StartedAt = timestamppb.Now()
	// No CompletedAt

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetMetricsRequest{})

	resp, err := server.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if resp.Msg.Metrics.AvgTaskDurationSeconds != 0 {
		t.Errorf("expected 0 avg duration for no completed tasks, got %v", resp.Msg.Metrics.AvgTaskDurationSeconds)
	}
}

// TestGetMetrics_TasksWithoutTimestamps_ReturnsZero verifies edge case:
// When completed tasks don't have valid timestamps, avg duration should be 0.
func TestGetMetrics_TasksWithoutTimestamps_ReturnsZero(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create completed task without StartedAt
	tk := task.NewProtoTask("TASK-001", "Task without timestamps")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	// No StartedAt

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetMetricsRequest{})

	resp, err := server.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Without valid timestamps, can't calculate duration
	if resp.Msg.Metrics.AvgTaskDurationSeconds != 0 {
		t.Errorf("expected 0 avg duration for tasks without timestamps, got %v", resp.Msg.Metrics.AvgTaskDurationSeconds)
	}
}

// TestGetMetrics_RespectsPeriodFilter verifies SC-2:
// When period is specified, only tasks in that period should be counted.
func TestGetMetrics_RespectsPeriodFilter(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	now := time.Now()

	// Task completed yesterday (within "week" period): 100 seconds
	task1 := task.NewProtoTask("TASK-001", "Recent task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task1.StartedAt = timestamppb.New(now.Add(-24 * time.Hour))
	task1.CompletedAt = timestamppb.New(now.Add(-24*time.Hour + 100*time.Second))

	// Task completed 2 weeks ago (outside "week" period): 500 seconds
	task2 := task.NewProtoTask("TASK-002", "Old task")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task2.StartedAt = timestamppb.New(now.Add(-14 * 24 * time.Hour))
	task2.CompletedAt = timestamppb.New(now.Add(-14*24*time.Hour + 500*time.Second))

	for _, tk := range []*orcv1.Task{task1, task2} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServer(backend, nil)

	// Request metrics for "week" period
	period := "week"
	req := connect.NewRequest(&orcv1.GetMetricsRequest{
		Period: &period,
	})

	resp, err := server.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Only task1 (100 seconds) should be counted
	if resp.Msg.Metrics.AvgTaskDurationSeconds != 100.0 {
		t.Errorf("expected avg duration 100 for week period, got %v", resp.Msg.Metrics.AvgTaskDurationSeconds)
	}
}
