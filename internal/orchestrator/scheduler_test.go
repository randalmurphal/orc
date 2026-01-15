package orchestrator

import (
	"sync"
	"testing"
)

// TestSchedulerBasicFlow tests the basic scheduling flow.
func TestSchedulerBasicFlow(t *testing.T) {
	s := NewScheduler(2)

	// Add tasks
	s.AddTask("TASK-001", "Task 1", nil, PriorityDefault)
	s.AddTask("TASK-002", "Task 2", nil, PriorityDefault)

	// Get ready tasks
	ready := s.NextReady(0)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(ready))
	}

	// Verify running count
	if s.RunningCount() != 2 {
		t.Errorf("expected running count 2, got %d", s.RunningCount())
	}

	// Mark first as completed
	s.MarkCompleted("TASK-001")
	if s.RunningCount() != 1 {
		t.Errorf("expected running count 1 after completion, got %d", s.RunningCount())
	}
}

// TestSchedulerCleansUpOnCompletion verifies that completed tasks have their
// map entries cleaned up when no longer needed.
func TestSchedulerCleansUpOnCompletion(t *testing.T) {
	s := NewScheduler(4)

	// Add tasks with dependencies
	// TASK-001 has no deps, TASK-002 depends on TASK-001
	s.AddTask("TASK-001", "Task 1", nil, PriorityDefault)
	s.AddTask("TASK-002", "Task 2", []string{"TASK-001"}, PriorityDefault)

	// Start TASK-001 (only one ready due to dependency)
	ready := s.NextReady(0)
	if len(ready) != 1 || ready[0].TaskID != "TASK-001" {
		t.Fatalf("expected only TASK-001 to be ready, got %v", ready)
	}

	// TASK-001 completes - but TASK-002 still depends on it
	s.MarkCompleted("TASK-001")

	// Verify TASK-001 is still in completed map (TASK-002 depends on it)
	s.mu.RLock()
	_, exists := s.completed["TASK-001"]
	s.mu.RUnlock()
	if !exists {
		t.Error("expected TASK-001 to remain in completed map while TASK-002 depends on it")
	}

	// Now TASK-002 should be ready
	ready = s.NextReady(0)
	if len(ready) != 1 || ready[0].TaskID != "TASK-002" {
		t.Fatalf("expected TASK-002 to be ready now, got %v", ready)
	}

	// Complete TASK-002
	s.MarkCompleted("TASK-002")

	// Now both completed entries should be cleaned up
	s.mu.RLock()
	completedCount := len(s.completed)
	s.mu.RUnlock()

	if completedCount != 0 {
		t.Errorf("expected completed map to be empty after all tasks done, got %d entries", completedCount)
	}
}

// TestSchedulerCleansUpTaskDeps verifies that taskDeps entries are cleaned up
// on task completion.
func TestSchedulerCleansUpTaskDeps(t *testing.T) {
	s := NewScheduler(4)

	// Add tasks with dependencies
	s.AddTask("TASK-001", "Task 1", nil, PriorityDefault)
	s.AddTask("TASK-002", "Task 2", []string{"TASK-001"}, PriorityDefault)
	s.AddTask("TASK-003", "Task 3", []string{"TASK-002"}, PriorityDefault)

	// Verify taskDeps are stored
	s.mu.RLock()
	if len(s.taskDeps) != 3 {
		t.Errorf("expected 3 taskDeps entries, got %d", len(s.taskDeps))
	}
	s.mu.RUnlock()

	// Start and complete TASK-001
	ready := s.NextReady(0)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	s.MarkCompleted("TASK-001")

	// TASK-001's deps should be cleaned up
	s.mu.RLock()
	_, exists := s.taskDeps["TASK-001"]
	taskDepsCount := len(s.taskDeps)
	s.mu.RUnlock()

	if exists {
		t.Error("expected TASK-001 taskDeps to be cleaned up after completion")
	}
	if taskDepsCount != 2 {
		t.Errorf("expected 2 taskDeps entries after TASK-001 completion, got %d", taskDepsCount)
	}
}

// TestSchedulerPreservesTaskDepsOnFailure verifies that taskDeps are preserved
// for failed tasks (to allow Requeue).
func TestSchedulerPreservesTaskDepsOnFailure(t *testing.T) {
	s := NewScheduler(4)

	// Add task with dependencies
	s.AddTask("TASK-001", "Task 1", []string{"TASK-000"}, PriorityDefault)

	// Mark TASK-000 as completed first (so TASK-001 can run)
	s.mu.Lock()
	s.completed["TASK-000"] = true
	s.mu.Unlock()

	// Start TASK-001
	ready := s.NextReady(0)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}

	// Mark TASK-001 as failed
	s.MarkFailed("TASK-001")

	// Verify taskDeps are preserved
	s.mu.RLock()
	deps, exists := s.taskDeps["TASK-001"]
	s.mu.RUnlock()

	if !exists {
		t.Error("expected taskDeps to be preserved for failed task")
	}
	if len(deps) != 1 || deps[0] != "TASK-000" {
		t.Errorf("expected taskDeps to contain TASK-000, got %v", deps)
	}
}

// TestSchedulerRequeueUsesDeps verifies that Requeue restores deps correctly.
func TestSchedulerRequeueUsesDeps(t *testing.T) {
	s := NewScheduler(4)

	// Add task with dependencies
	s.AddTask("TASK-001", "Task 1", []string{"TASK-000"}, PriorityDefault)

	// Mark TASK-000 as completed first
	s.mu.Lock()
	s.completed["TASK-000"] = true
	s.mu.Unlock()

	// Start and fail TASK-001
	ready := s.NextReady(0)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	s.MarkFailed("TASK-001")

	// Requeue TASK-001
	s.Requeue("TASK-001", "Task 1 Retry", PriorityDefault)

	// Verify task is back in queue
	if s.QueueLength() != 1 {
		t.Errorf("expected queue length 1 after requeue, got %d", s.QueueLength())
	}

	// Start again and verify deps are intact
	ready = s.NextReady(0)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task after requeue, got %d", len(ready))
	}

	if len(ready[0].DependsOn) != 1 || ready[0].DependsOn[0] != "TASK-000" {
		t.Errorf("expected requeued task to have deps [TASK-000], got %v", ready[0].DependsOn)
	}
}

// TestSchedulerChainedDependencies tests cleanup with a chain of dependencies.
func TestSchedulerChainedDependencies(t *testing.T) {
	s := NewScheduler(4)

	// A -> B -> C -> D (chain of dependencies)
	s.AddTask("A", "Task A", nil, PriorityDefault)
	s.AddTask("B", "Task B", []string{"A"}, PriorityDefault)
	s.AddTask("C", "Task C", []string{"B"}, PriorityDefault)
	s.AddTask("D", "Task D", []string{"C"}, PriorityDefault)

	// Process the chain
	for _, taskID := range []string{"A", "B", "C", "D"} {
		ready := s.NextReady(0)
		if len(ready) != 1 || ready[0].TaskID != taskID {
			t.Fatalf("expected %s to be ready, got %v", taskID, ready)
		}
		s.MarkCompleted(taskID)
	}

	// All maps should be empty
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.completed) != 0 {
		t.Errorf("expected completed map to be empty, got %d entries", len(s.completed))
	}
	if len(s.taskDeps) != 0 {
		t.Errorf("expected taskDeps map to be empty, got %d entries", len(s.taskDeps))
	}
	if len(s.running) != 0 {
		t.Errorf("expected running map to be empty, got %d entries", len(s.running))
	}
}

// TestSchedulerDiamondDependencies tests cleanup with diamond-shaped dependencies.
func TestSchedulerDiamondDependencies(t *testing.T) {
	s := NewScheduler(4)

	// Diamond: A -> B, A -> C, B -> D, C -> D
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	s.AddTask("A", "Task A", nil, PriorityDefault)
	s.AddTask("B", "Task B", []string{"A"}, PriorityDefault)
	s.AddTask("C", "Task C", []string{"A"}, PriorityDefault)
	s.AddTask("D", "Task D", []string{"B", "C"}, PriorityDefault)

	// Complete A
	ready := s.NextReady(0)
	if len(ready) != 1 || ready[0].TaskID != "A" {
		t.Fatalf("expected A to be ready, got %v", ready)
	}
	s.MarkCompleted("A")

	// A should remain in completed because B and C depend on it
	s.mu.RLock()
	aInCompleted := s.completed["A"]
	s.mu.RUnlock()
	if !aInCompleted {
		t.Error("expected A to remain in completed while B and C are pending")
	}

	// B and C should now be ready
	ready = s.NextReady(0)
	if len(ready) != 2 {
		t.Fatalf("expected B and C to be ready, got %d tasks", len(ready))
	}

	// Complete B
	s.MarkCompleted("B")

	// A should remain because C (running) still needs it
	s.mu.RLock()
	aInCompleted = s.completed["A"]
	s.mu.RUnlock()
	if !aInCompleted {
		t.Error("expected A to remain in completed while C is running")
	}

	// Complete C
	s.MarkCompleted("C")

	// Now A should be cleaned up (D depends on B and C, not A)
	s.mu.RLock()
	aInCompleted = s.completed["A"]
	s.mu.RUnlock()
	if aInCompleted {
		t.Error("expected A to be cleaned up after B and C complete")
	}

	// D should be ready
	ready = s.NextReady(0)
	if len(ready) != 1 || ready[0].TaskID != "D" {
		t.Fatalf("expected D to be ready, got %v", ready)
	}

	// Complete D
	s.MarkCompleted("D")

	// All maps should be empty
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.completed) != 0 {
		t.Errorf("expected completed map to be empty, got %d entries", len(s.completed))
	}
	if len(s.taskDeps) != 0 {
		t.Errorf("expected taskDeps map to be empty, got %d entries", len(s.taskDeps))
	}
}

// TestSchedulerConcurrentCompletions tests concurrent task completions don't cause races.
func TestSchedulerConcurrentCompletions(t *testing.T) {
	s := NewScheduler(10)

	// Add 10 independent tasks
	for i := 0; i < 10; i++ {
		taskID := "TASK-" + string(rune('A'+i))
		s.AddTask(taskID, "Task "+taskID, nil, PriorityDefault)
	}

	// Get all ready
	ready := s.NextReady(10)
	if len(ready) != 10 {
		t.Fatalf("expected 10 ready tasks, got %d", len(ready))
	}

	// Complete all concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskID := "TASK-" + string(rune('A'+i))
		go func(id string) {
			defer wg.Done()
			s.MarkCompleted(id)
		}(taskID)
	}
	wg.Wait()

	// Verify all maps are clean
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.completed) != 0 {
		t.Errorf("expected completed map to be empty, got %d entries", len(s.completed))
	}
	if len(s.running) != 0 {
		t.Errorf("expected running map to be empty, got %d entries", len(s.running))
	}
	if len(s.taskDeps) != 0 {
		t.Errorf("expected taskDeps map to be empty, got %d entries", len(s.taskDeps))
	}
}

// TestSchedulerCompletedCountAccuracy tests that CompletedCount returns accurate
// count considering cleanup.
func TestSchedulerCompletedCountAccuracy(t *testing.T) {
	s := NewScheduler(4)

	// Add independent tasks
	s.AddTask("TASK-001", "Task 1", nil, PriorityDefault)
	s.AddTask("TASK-002", "Task 2", nil, PriorityDefault)

	// Start both
	ready := s.NextReady(0)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}

	// Complete first - should be immediately cleaned since no one depends on it
	s.MarkCompleted("TASK-001")

	// CompletedCount should reflect the cleanup
	// Since no task depends on TASK-001, it gets cleaned up immediately
	if s.CompletedCount() != 0 {
		t.Errorf("expected CompletedCount to be 0 (cleaned up), got %d", s.CompletedCount())
	}

	// Complete second
	s.MarkCompleted("TASK-002")

	if s.CompletedCount() != 0 {
		t.Errorf("expected CompletedCount to be 0 after all cleanup, got %d", s.CompletedCount())
	}
}

// TestSchedulerRemoveTask tests the RemoveTask method for permanent removal.
func TestSchedulerRemoveTask(t *testing.T) {
	s := NewScheduler(4)

	// Add task
	s.AddTask("TASK-001", "Task 1", []string{"TASK-000"}, PriorityDefault)

	// Mark as failed but don't requeue
	s.mu.Lock()
	s.completed["TASK-000"] = true
	s.mu.Unlock()

	ready := s.NextReady(0)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}

	s.MarkFailed("TASK-001")

	// Verify taskDeps still exists
	s.mu.RLock()
	_, exists := s.taskDeps["TASK-001"]
	s.mu.RUnlock()
	if !exists {
		t.Fatal("expected taskDeps to exist for failed task")
	}

	// Permanently remove task
	s.RemoveTask("TASK-001")

	// Verify taskDeps is cleaned up
	s.mu.RLock()
	_, exists = s.taskDeps["TASK-001"]
	s.mu.RUnlock()
	if exists {
		t.Error("expected taskDeps to be cleaned up after RemoveTask")
	}
}

// TestSchedulerIsComplete tests the IsComplete method.
func TestSchedulerIsComplete(t *testing.T) {
	s := NewScheduler(4)

	// Empty scheduler should be complete
	if !s.IsComplete() {
		t.Error("expected empty scheduler to be complete")
	}

	// Add a task
	s.AddTask("TASK-001", "Task 1", nil, PriorityDefault)
	if s.IsComplete() {
		t.Error("expected scheduler with queued task to not be complete")
	}

	// Start the task
	s.NextReady(0)
	if s.IsComplete() {
		t.Error("expected scheduler with running task to not be complete")
	}

	// Complete the task
	s.MarkCompleted("TASK-001")
	if !s.IsComplete() {
		t.Error("expected scheduler to be complete after last task completed")
	}
}
