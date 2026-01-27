// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-552: LinkTasks/UnlinkTask data consistency
//
// These tests verify the LinkTasks and UnlinkTask APIs maintain consistency
// between the task.initiative_id field and the initiative_tasks junction table.
//
// Success Criteria Coverage:
// - SC-5: LinkTasks API maintains data consistency
//
// The fix: LinkTasks must update BOTH:
// 1. task.initiative_id (for task store filtering in frontend)
// 2. initiative_tasks junction table (for initiative detail page)
//
// Without this fix, the InitiativesView shows 0/0 tasks for all initiatives
// because it filters by task.initiativeId which is populated by the task store,
// while InitiativeDetailPage shows correct counts from initiative.tasks.
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-5: LinkTasks API maintains data consistency
// ============================================================================

// TestLinkTasks_UpdatesBothTaskAndJunctionTable verifies SC-5:
// LinkTasks must update both task.initiative_id AND add entry to initiative_tasks table.
// This is the core test for the bug fix - before the fix, only task.initiative_id was updated.
func TestLinkTasks_UpdatesBothTaskAndJunctionTable(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create an initiative
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks to link
	task1 := task.NewProtoTask("TASK-001", "First Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task2 := task.NewProtoTask("TASK-002", "Second Task")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED

	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Link tasks to initiative
	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{"TASK-001", "TASK-002"},
	})

	resp, err := server.LinkTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("LinkTasks failed: %v", err)
	}

	// Verify response contains updated initiative
	if resp.Msg.Initiative == nil {
		t.Fatal("response initiative is nil")
	}

	// CRITICAL VERIFICATION 1: task.initiative_id must be set
	// This is what InitiativesView uses to filter tasks
	loadedTask1, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task1: %v", err)
	}
	if loadedTask1.InitiativeId == nil || *loadedTask1.InitiativeId != "INIT-001" {
		t.Errorf("task1.InitiativeId = %v, want INIT-001", loadedTask1.InitiativeId)
	}

	loadedTask2, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task2: %v", err)
	}
	if loadedTask2.InitiativeId == nil || *loadedTask2.InitiativeId != "INIT-001" {
		t.Errorf("task2.InitiativeId = %v, want INIT-001", loadedTask2.InitiativeId)
	}

	// CRITICAL VERIFICATION 2: initiative_tasks junction table must have entries
	// This is what InitiativeDetailPage uses to show tasks
	loadedInit, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loadedInit.Tasks) != 2 {
		t.Errorf("initiative.Tasks has %d entries, want 2 (junction table not updated)", len(loadedInit.Tasks))
	}

	// Verify task IDs in junction table
	taskIDs := make(map[string]bool)
	for _, taskRef := range loadedInit.Tasks {
		taskIDs[taskRef.Id] = true
	}
	if !taskIDs["TASK-001"] {
		t.Error("TASK-001 not found in initiative.Tasks (junction table)")
	}
	if !taskIDs["TASK-002"] {
		t.Error("TASK-002 not found in initiative.Tasks (junction table)")
	}
}

// TestUnlinkTask_RemovesFromBothTaskAndJunctionTable verifies SC-5:
// UnlinkTask must clear task.initiative_id AND remove entry from initiative_tasks table.
func TestUnlinkTask_RemovesFromBothTaskAndJunctionTable(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create an initiative with a linked task (via junction table)
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	init.Tasks = []*orcv1.TaskRef{
		{Id: "TASK-001", Title: "First Task", Status: orcv1.TaskStatus_TASK_STATUS_CREATED},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative_id already set
	initID := "INIT-001"
	task1 := task.NewProtoTask("TASK-001", "First Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task1.InitiativeId = &initID
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Unlink task from initiative
	req := connect.NewRequest(&orcv1.UnlinkTaskRequest{
		InitiativeId: "INIT-001",
		TaskId:       "TASK-001",
	})

	_, err := server.UnlinkTask(context.Background(), req)
	if err != nil {
		t.Fatalf("UnlinkTask failed: %v", err)
	}

	// CRITICAL VERIFICATION 1: task.initiative_id must be cleared
	loadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loadedTask.InitiativeId != nil {
		t.Errorf("task.InitiativeId = %v, want nil (task.initiative_id not cleared)", *loadedTask.InitiativeId)
	}

	// CRITICAL VERIFICATION 2: initiative_tasks junction table must be cleared
	loadedInit, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loadedInit.Tasks) != 0 {
		t.Errorf("initiative.Tasks has %d entries, want 0 (junction table not cleared)", len(loadedInit.Tasks))
	}
}

// ============================================================================
// Edge Cases: Link Operations
// ============================================================================

// TestLinkTasks_TaskAlreadyLinkedToSameInitiative verifies no-op behavior.
// If task is already linked to the same initiative, should succeed without error.
func TestLinkTasks_TaskAlreadyLinkedToSameInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative with task already linked
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	init.Tasks = []*orcv1.TaskRef{
		{Id: "TASK-001", Title: "First Task", Status: orcv1.TaskStatus_TASK_STATUS_CREATED},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative_id already set
	initID := "INIT-001"
	task1 := task.NewProtoTask("TASK-001", "First Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task1.InitiativeId = &initID
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Try to link same task again
	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{"TASK-001"},
	})

	_, err := server.LinkTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("LinkTasks should succeed for already-linked task: %v", err)
	}

	// Verify task is still linked (only once)
	loadedInit, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	// Junction table should still have exactly 1 entry (not duplicated)
	if len(loadedInit.Tasks) != 1 {
		t.Errorf("initiative.Tasks has %d entries, want 1 (should not duplicate)", len(loadedInit.Tasks))
	}
}

// TestLinkTasks_TaskLinkedToDifferentInitiative verifies re-linking behavior.
// If task is linked to a different initiative, it should be moved to the new one.
func TestLinkTasks_TaskLinkedToDifferentInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create two initiatives
	init1 := initiative.NewProtoInitiative("INIT-001", "First Initiative")
	init1.Tasks = []*orcv1.TaskRef{
		{Id: "TASK-001", Title: "Task to Move", Status: orcv1.TaskStatus_TASK_STATUS_CREATED},
	}
	if err := backend.SaveInitiativeProto(init1); err != nil {
		t.Fatalf("save initiative 1: %v", err)
	}

	init2 := initiative.NewProtoInitiative("INIT-002", "Second Initiative")
	if err := backend.SaveInitiativeProto(init2); err != nil {
		t.Fatalf("save initiative 2: %v", err)
	}

	// Create task linked to first initiative
	initID := "INIT-001"
	task1 := task.NewProtoTask("TASK-001", "Task to Move")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task1.InitiativeId = &initID
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Link task to second initiative (should move it)
	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-002",
		TaskIds:      []string{"TASK-001"},
	})

	_, err := server.LinkTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("LinkTasks failed: %v", err)
	}

	// Verify task.initiative_id points to new initiative
	loadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loadedTask.InitiativeId == nil || *loadedTask.InitiativeId != "INIT-002" {
		t.Errorf("task.InitiativeId = %v, want INIT-002", loadedTask.InitiativeId)
	}

	// Verify task is in new initiative's junction table
	loadedInit2, err := backend.LoadInitiativeProto("INIT-002")
	if err != nil {
		t.Fatalf("load initiative 2: %v", err)
	}
	if len(loadedInit2.Tasks) != 1 {
		t.Errorf("INIT-002.Tasks has %d entries, want 1", len(loadedInit2.Tasks))
	}

	// Verify task was removed from old initiatives junction table
	loadedInit1, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("load initiative 1: %v", err)
	}
	if len(loadedInit1.Tasks) != 0 {
		t.Errorf("INIT-001.Tasks has %d entries, want 0 (task should be removed)", len(loadedInit1.Tasks))
	}
}

// TestLinkTasks_EmptyTaskList verifies error for empty task list.
func TestLinkTasks_EmptyTaskList(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{}, // Empty list
	})

	_, err := server.LinkTasks(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty task list")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

// TestLinkTasks_NonExistentTask verifies handling of non-existent tasks.
// Current behavior: skip non-existent tasks silently.
func TestLinkTasks_NonExistentTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create only one task
	task1 := task.NewProtoTask("TASK-001", "Existing Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Try to link one existing and one non-existent task
	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{"TASK-001", "TASK-NONEXISTENT"},
	})

	// Should succeed (skip non-existent)
	_, err := server.LinkTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("LinkTasks should succeed, skipping non-existent tasks: %v", err)
	}

	// Verify existing task was linked
	loadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loadedTask.InitiativeId == nil || *loadedTask.InitiativeId != "INIT-001" {
		t.Errorf("task.InitiativeId = %v, want INIT-001", loadedTask.InitiativeId)
	}
}

// TestLinkTasks_NonExistentInitiative verifies error for non-existent initiative.
func TestLinkTasks_NonExistentInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	task1 := task.NewProtoTask("TASK-001", "Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-NONEXISTENT",
		TaskIds:      []string{"TASK-001"},
	})

	_, err := server.LinkTasks(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// Edge Cases: Unlink Operations
// ============================================================================

// TestUnlinkTask_TaskNotLinkedToInitiative verifies error handling.
func TestUnlinkTask_TaskNotLinkedToInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative without linked tasks
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task without initiative_id
	task1 := task.NewProtoTask("TASK-001", "Unlinked Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.UnlinkTaskRequest{
		InitiativeId: "INIT-001",
		TaskId:       "TASK-001",
	})

	_, err := server.UnlinkTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for task not linked to initiative")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// TestUnlinkTask_TaskLinkedToDifferentInitiative verifies error handling.
func TestUnlinkTask_TaskLinkedToDifferentInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create two initiatives
	init1 := initiative.NewProtoInitiative("INIT-001", "First Initiative")
	init1.Tasks = []*orcv1.TaskRef{
		{Id: "TASK-001", Title: "Task", Status: orcv1.TaskStatus_TASK_STATUS_CREATED},
	}
	if err := backend.SaveInitiativeProto(init1); err != nil {
		t.Fatalf("save initiative 1: %v", err)
	}

	init2 := initiative.NewProtoInitiative("INIT-002", "Second Initiative")
	if err := backend.SaveInitiativeProto(init2); err != nil {
		t.Fatalf("save initiative 2: %v", err)
	}

	// Create task linked to first initiative
	initID := "INIT-001"
	task1 := task.NewProtoTask("TASK-001", "Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task1.InitiativeId = &initID
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Try to unlink from wrong initiative
	req := connect.NewRequest(&orcv1.UnlinkTaskRequest{
		InitiativeId: "INIT-002",
		TaskId:       "TASK-001",
	})

	_, err := server.UnlinkTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for task linked to different initiative")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// TestUnlinkTask_NonExistentTask verifies error handling.
func TestUnlinkTask_NonExistentTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.UnlinkTaskRequest{
		InitiativeId: "INIT-001",
		TaskId:       "TASK-NONEXISTENT",
	})

	_, err := server.UnlinkTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// Data Consistency Tests
// ============================================================================

// TestLinkTasks_ProgressCalculation verifies that linked tasks can be counted correctly.
// This tests the complete data flow that InitiativesView depends on.
func TestLinkTasks_ProgressCalculation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create 3 tasks: 2 completed, 1 running
	task1 := task.NewProtoTask("TASK-001", "Completed Task 1")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task2 := task.NewProtoTask("TASK-002", "Completed Task 2")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task3 := task.NewProtoTask("TASK-003", "Running Task")
	task3.Weight = orcv1.TaskWeight_TASK_WEIGHT_LARGE
	task3.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING

	for _, tk := range []*orcv1.Task{task1, task2, task3} {
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %s: %v", tk.Id, err)
		}
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Link all tasks to initiative
	req := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{"TASK-001", "TASK-002", "TASK-003"},
	})

	_, err := server.LinkTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("LinkTasks failed: %v", err)
	}

	// Load all tasks and count by initiative_id (simulates frontend task store)
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load all tasks: %v", err)
	}

	// Count tasks linked to INIT-001 via task.initiative_id
	var linkedCount, completedCount int
	for _, tk := range allTasks {
		if tk.InitiativeId != nil && *tk.InitiativeId == "INIT-001" {
			linkedCount++
			if tk.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
				completedCount++
			}
		}
	}

	// Should show 2/3 tasks completed
	if linkedCount != 3 {
		t.Errorf("linked task count via task.initiative_id = %d, want 3", linkedCount)
	}
	if completedCount != 2 {
		t.Errorf("completed task count = %d, want 2", completedCount)
	}

	// Also verify via initiative.Tasks (junction table)
	loadedInit, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loadedInit.Tasks) != 3 {
		t.Errorf("initiative.Tasks count = %d, want 3", len(loadedInit.Tasks))
	}
}

// TestListInitiativeTasks_ReturnsLinkedTasks verifies the ListInitiativeTasks RPC
// returns tasks that have been linked via LinkTasks.
func TestListInitiativeTasks_ReturnsLinkedTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create and save tasks
	task1 := task.NewProtoTask("TASK-001", "First Task")
	task1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	task2 := task.NewProtoTask("TASK-002", "Second Task")
	task2.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED

	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Link tasks
	linkReq := connect.NewRequest(&orcv1.LinkTasksRequest{
		InitiativeId: "INIT-001",
		TaskIds:      []string{"TASK-001", "TASK-002"},
	})
	_, err := server.LinkTasks(context.Background(), linkReq)
	if err != nil {
		t.Fatalf("LinkTasks failed: %v", err)
	}

	// List initiative tasks
	listReq := connect.NewRequest(&orcv1.ListInitiativeTasksRequest{
		InitiativeId: "INIT-001",
	})
	resp, err := server.ListInitiativeTasks(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListInitiativeTasks failed: %v", err)
	}

	// Verify both tasks are returned
	if len(resp.Msg.Tasks) != 2 {
		t.Errorf("ListInitiativeTasks returned %d tasks, want 2", len(resp.Msg.Tasks))
	}

	taskIDs := make(map[string]bool)
	for _, tk := range resp.Msg.Tasks {
		taskIDs[tk.Id] = true
	}
	if !taskIDs["TASK-001"] {
		t.Error("TASK-001 not returned by ListInitiativeTasks")
	}
	if !taskIDs["TASK-002"] {
		t.Error("TASK-002 not returned by ListInitiativeTasks")
	}
}
