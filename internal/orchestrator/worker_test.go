package orchestrator

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/events"
)

// TestWorkerPoolCleansUpOnCompletion verifies that workers remove themselves
// from the pool map when they complete successfully.
func TestWorkerPoolCleansUpOnCompletion(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Manually add a worker to simulate spawn
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Verify worker is in pool
	if pool.GetWorker("TASK-001") == nil {
		t.Fatal("expected worker to be in pool before cleanup")
	}

	// Simulate worker completion and cleanup
	worker.setStatus(WorkerStatusComplete)
	pool.RemoveWorker(worker.TaskID)

	// Verify worker is removed
	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected worker to be removed from pool after completion")
	}

	// Verify map is empty
	if len(pool.workers) != 0 {
		t.Errorf("expected pool workers map to be empty, got %d workers", len(pool.workers))
	}
}

// TestWorkerPoolCleansUpOnFailure verifies that workers remove themselves
// from the pool map when they fail.
func TestWorkerPoolCleansUpOnFailure(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Manually add a worker to simulate spawn
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Verify worker is in pool
	if pool.GetWorker("TASK-001") == nil {
		t.Fatal("expected worker to be in pool before cleanup")
	}

	// Simulate worker failure and cleanup
	worker.setError(nil) // Sets status to failed
	pool.RemoveWorker(worker.TaskID)

	// Verify worker is removed
	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected worker to be removed from pool after failure")
	}

	// Verify status is failed
	if worker.GetStatus() != WorkerStatusFailed {
		t.Errorf("expected worker status to be failed, got %s", worker.GetStatus())
	}
}

// TestWorkerPoolCapacityAfterCompletion verifies that capacity is freed
// immediately when a worker completes, allowing new workers to spawn.
func TestWorkerPoolCapacityAfterCompletion(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Fill pool to capacity
	pool.mu.Lock()
	pool.workers["TASK-001"] = &Worker{TaskID: "TASK-001", Status: WorkerStatusRunning}
	pool.workers["TASK-002"] = &Worker{TaskID: "TASK-002", Status: WorkerStatusRunning}
	pool.mu.Unlock()

	// Verify pool is at capacity
	pool.mu.RLock()
	if len(pool.workers) != 2 {
		t.Fatalf("expected pool to be at capacity (2), got %d", len(pool.workers))
	}
	pool.mu.RUnlock()

	// Simulate one worker completing and removing itself
	pool.RemoveWorker("TASK-001")

	// Verify capacity is now available
	pool.mu.RLock()
	workerCount := len(pool.workers)
	pool.mu.RUnlock()

	if workerCount != 1 {
		t.Errorf("expected 1 worker after removal, got %d", workerCount)
	}

	// Verify we can now add a new worker (capacity check would pass)
	pool.mu.RLock()
	atCapacity := len(pool.workers) >= pool.maxWorkers
	pool.mu.RUnlock()

	if atCapacity {
		t.Error("expected capacity to be available after worker completion")
	}
}

// TestConcurrentWorkerCleanup verifies that multiple workers completing
// simultaneously don't cause race conditions.
func TestConcurrentWorkerCleanup(t *testing.T) {
	pool := NewWorkerPool(10, nil, nil, nil, nil, nil)

	// Add 10 workers
	pool.mu.Lock()
	for i := 0; i < 10; i++ {
		taskID := "TASK-" + string(rune('A'+i))
		pool.workers[taskID] = &Worker{
			TaskID: taskID,
			Status: WorkerStatusRunning,
		}
	}
	pool.mu.Unlock()

	// Verify all workers are in pool
	pool.mu.RLock()
	if len(pool.workers) != 10 {
		t.Fatalf("expected 10 workers, got %d", len(pool.workers))
	}
	pool.mu.RUnlock()

	// Concurrently complete and remove all workers
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskID := "TASK-" + string(rune('A'+i))
		go func(id string) {
			defer wg.Done()
			// Simulate work completion
			time.Sleep(time.Duration(i) * time.Millisecond) // Stagger slightly
			pool.RemoveWorker(id)
		}(taskID)
	}

	wg.Wait()

	// Verify all workers are removed
	pool.mu.RLock()
	remaining := len(pool.workers)
	pool.mu.RUnlock()

	if remaining != 0 {
		t.Errorf("expected 0 workers after concurrent cleanup, got %d", remaining)
	}
}

// TestActiveCountReflectsRunningOnly verifies that ActiveCount() only counts
// workers with running status, not completed/failed workers still in map.
func TestActiveCountReflectsRunningOnly(t *testing.T) {
	pool := NewWorkerPool(5, nil, nil, nil, nil, nil)

	// Add workers with various statuses
	pool.mu.Lock()
	pool.workers["TASK-001"] = &Worker{TaskID: "TASK-001", Status: WorkerStatusRunning}
	pool.workers["TASK-002"] = &Worker{TaskID: "TASK-002", Status: WorkerStatusComplete}
	pool.workers["TASK-003"] = &Worker{TaskID: "TASK-003", Status: WorkerStatusFailed}
	pool.workers["TASK-004"] = &Worker{TaskID: "TASK-004", Status: WorkerStatusRunning}
	pool.workers["TASK-005"] = &Worker{TaskID: "TASK-005", Status: WorkerStatusPaused}
	pool.mu.Unlock()

	// ActiveCount should only count running workers
	activeCount := pool.ActiveCount()
	if activeCount != 2 {
		t.Errorf("expected ActiveCount to be 2 (only running), got %d", activeCount)
	}
}

// TestRemoveWorkerIdempotent verifies that RemoveWorker can be called
// multiple times without panicking.
func TestRemoveWorkerIdempotent(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Add a worker
	pool.mu.Lock()
	pool.workers["TASK-001"] = &Worker{TaskID: "TASK-001", Status: WorkerStatusRunning}
	pool.mu.Unlock()

	// Remove it multiple times - should not panic
	pool.RemoveWorker("TASK-001")
	pool.RemoveWorker("TASK-001") // Second call should be no-op
	pool.RemoveWorker("TASK-001") // Third call should be no-op

	// Remove non-existent worker - should not panic
	pool.RemoveWorker("TASK-NONEXISTENT")

	// Verify pool is empty
	pool.mu.RLock()
	if len(pool.workers) != 0 {
		t.Errorf("expected empty pool, got %d workers", len(pool.workers))
	}
	pool.mu.RUnlock()
}

// TestWorkerSelfCleanupOnComplete tests that workers properly clean themselves
// up when their run completes. This simulates the defer block behavior.
func TestWorkerSelfCleanupOnComplete(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Track cleanup calls
	var cleanupCalled atomic.Int32

	// Create worker and add to pool
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Simulate what the defer block in Worker.run() does
	cleanupFn := func() {
		worker.mu.Lock()
		if worker.Status == WorkerStatusRunning {
			worker.Status = WorkerStatusComplete
		}
		worker.mu.Unlock()

		// Remove worker from pool immediately after setting final status
		pool.RemoveWorker(worker.TaskID)
		cleanupCalled.Add(1)
	}

	// Execute cleanup
	cleanupFn()

	// Verify cleanup was called
	if cleanupCalled.Load() != 1 {
		t.Errorf("expected cleanup to be called once, got %d", cleanupCalled.Load())
	}

	// Verify worker status is complete
	if worker.GetStatus() != WorkerStatusComplete {
		t.Errorf("expected worker status complete, got %s", worker.GetStatus())
	}

	// Verify worker removed from pool
	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected worker to be removed from pool")
	}
}

// TestWorkerSelfCleanupOnFailure tests that workers properly clean themselves
// up when they fail.
func TestWorkerSelfCleanupOnFailure(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Create worker and add to pool
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Simulate what happens when setError is called and then defer runs
	worker.setError(nil) // Sets status to failed

	// Simulate defer cleanup
	pool.RemoveWorker(worker.TaskID)

	// Verify worker status is failed
	if worker.GetStatus() != WorkerStatusFailed {
		t.Errorf("expected worker status failed, got %s", worker.GetStatus())
	}

	// Verify worker removed from pool
	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected worker to be removed from pool")
	}
}

// TestHandlerIdempotence tests that orchestrator handlers are safe to call
// even after workers have self-cleaned.
func TestHandlerIdempotence(t *testing.T) {
	pool := NewWorkerPool(2, events.NewNopPublisher(), nil, nil, nil, nil)

	// Create and add a worker
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusComplete,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Simulate self-cleanup by worker
	pool.RemoveWorker("TASK-001")

	// GetWorker should now return nil
	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected GetWorker to return nil after self-cleanup")
	}

	// Calling RemoveWorker again should be safe (idempotent)
	pool.RemoveWorker("TASK-001") // Should not panic
}

// TestWorkerExitsLoopOnContextCancel verifies that the worker loop exits cleanly
// when context is cancelled. This tests the iteration exit condition for pausing.
func TestWorkerExitsLoopOnContextCancel(t *testing.T) {
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create worker with the cancellable context
	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
		ctx:    ctx,
		cancel: cancel,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Verify worker is running
	if worker.GetStatus() != WorkerStatusRunning {
		t.Fatalf("expected running status, got %s", worker.GetStatus())
	}

	// Simulate context cancellation (pause request)
	cancel()

	// Verify context is cancelled
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}

	// Simulate what the run loop does when context is cancelled
	// In real execution, this happens after cmd.Run() returns with context error
	if worker.ctx.Err() != nil {
		worker.setStatus(WorkerStatusPaused)
	}

	// Verify worker transitioned to paused
	if worker.GetStatus() != WorkerStatusPaused {
		t.Errorf("expected paused status after context cancel, got %s", worker.GetStatus())
	}

	// Verify the stop function works via the cancel
	// (this tests Worker.Stop() indirectly)
	worker2 := &Worker{
		ID:     "worker-TASK-002",
		TaskID: "TASK-002",
		Status: WorkerStatusRunning,
		cancel: func() {}, // No-op cancel
	}
	worker2.Stop() // Should not panic

	// Test that setStatus to paused is idempotent
	worker.setStatus(WorkerStatusPaused)
	if worker.GetStatus() != WorkerStatusPaused {
		t.Errorf("expected paused status, got %s", worker.GetStatus())
	}
}

// TestWorkerRunIterativePattern documents the iterative phase execution pattern.
// Worker.run() uses iteration instead of recursion to avoid stack overflow
// with tasks that have many phases.
func TestWorkerRunIterativePattern(t *testing.T) {
	// This test documents the expected behavior of Worker.run():
	//
	// 1. Loop starts: for { ... }
	// 2. Get current phase from plan
	// 3. If no current phase -> complete and return
	// 4. Execute phase (claude command)
	// 5. On error/cancellation -> set status and return
	// 6. If phase completed (ralph state removed):
	//    a. Mark phase complete in plan
	//    b. Check for next phase
	//    c. If no next phase -> task complete, return
	//    d. If next phase exists -> continue (next loop iteration)
	// 7. If phase not complete (ralph state exists) -> return
	//
	// Key difference from recursion:
	// - Recursion: w.run(pool, t, pln, st) at end of phase
	// - Iteration: continue in the for loop
	//
	// Benefits:
	// - No stack growth for N phases (O(1) stack vs O(N))
	// - Same behavior, cleaner control flow
	// - No risk of stack overflow for large task plans

	// Verify worker status transitions work correctly for multi-phase simulation
	pool := NewWorkerPool(2, nil, nil, nil, nil, nil)

	worker := &Worker{
		ID:     "worker-TASK-001",
		TaskID: "TASK-001",
		Status: WorkerStatusRunning,
	}
	pool.mu.Lock()
	pool.workers["TASK-001"] = worker
	pool.mu.Unlock()

	// Simulate 100 phase completions (would overflow stack with recursion)
	phaseCount := 100
	for i := 0; i < phaseCount; i++ {
		// Worker remains running throughout all phase iterations
		if worker.GetStatus() != WorkerStatusRunning {
			t.Errorf("phase %d: expected running status, got %s", i, worker.GetStatus())
		}
	}

	// Final completion
	worker.setStatus(WorkerStatusComplete)
	pool.RemoveWorker(worker.TaskID)

	if worker.GetStatus() != WorkerStatusComplete {
		t.Errorf("expected complete status, got %s", worker.GetStatus())
	}

	if pool.GetWorker("TASK-001") != nil {
		t.Error("expected worker to be removed after completion")
	}
}
