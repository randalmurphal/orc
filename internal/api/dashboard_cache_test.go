// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-531: Performance - Stats page takes 5+ seconds to load
//
// These tests verify the dashboard cache layer that coalesces concurrent
// LoadAllTasks calls and provides TTL-based caching for dashboard endpoints.
//
// Success Criteria Coverage:
// - SC-1: Concurrent dashboard calls share a single LoadAllTasks invocation
// - SC-4: Cached data reused within TTL; expired cache triggers fresh load
// - SC-5: Backend cache TTL is configurable and shorter than frontend 5-min cache
// - SC-7: GetTopInitiatives uses batch initiative loading (no N+1)
//
// Edge Cases:
// - Empty database returns zero counts
// - Cache invalidation on task write
// - Concurrent cache invalidation + read (no data race)
// - Cache load error propagation (no cached errors)
// - Singleflight error propagation to all waiters
package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
// SC-1: Concurrent dashboard calls share a single LoadAllTasks invocation
// ============================================================================

// TestDashboardCache_ConcurrentCallsShareSingleLoad verifies SC-1 (BDD-1):
// When 6 dashboard API calls arrive concurrently with an empty cache,
// exactly 1 LoadAllTasks call should be made — all requests share the result.
func TestDashboardCache_ConcurrentCallsShareSingleLoad(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed some tasks
	for i := 0; i < 10; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i+1), fmt.Sprintf("Task %d", i+1))
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CompletedAt = timestamppb.Now()
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// Create a counting backend wrapper to track LoadAllTasks calls
	counting := &countingBackend{Backend: backend}

	server := NewDashboardServer(counting, nil)

	// Launch 6 concurrent dashboard calls (simulating frontend parallel fetch)
	var wg sync.WaitGroup
	errs := make([]error, 6)

	wg.Add(6)
	go func() {
		defer wg.Done()
		_, errs[0] = server.GetStats(context.Background(), connect.NewRequest(&orcv1.GetStatsRequest{}))
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = server.GetCostSummary(context.Background(), connect.NewRequest(&orcv1.GetCostSummaryRequest{}))
	}()
	go func() {
		defer wg.Done()
		_, errs[2] = server.GetDailyMetrics(context.Background(), connect.NewRequest(&orcv1.GetDailyMetricsRequest{}))
	}()
	go func() {
		defer wg.Done()
		_, errs[3] = server.GetMetrics(context.Background(), connect.NewRequest(&orcv1.GetMetricsRequest{}))
	}()
	go func() {
		defer wg.Done()
		_, errs[4] = server.GetTopInitiatives(context.Background(), connect.NewRequest(&orcv1.GetTopInitiativesRequest{Limit: 10}))
	}()
	go func() {
		defer wg.Done()
		_, errs[5] = server.GetComparison(context.Background(), connect.NewRequest(&orcv1.GetComparisonRequest{Period: "week"}))
	}()

	wg.Wait()

	// All calls should succeed
	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d failed: %v", i, err)
		}
	}

	// SC-1: LoadAllTasks should have been called at most once thanks to cache/singleflight
	calls := counting.loadAllTasksCalls.Load()
	if calls > 1 {
		t.Errorf("expected LoadAllTasks called at most 1 time, got %d (no caching/singleflight)", calls)
	}
}

// ============================================================================
// SC-4: Cache TTL — reuse within TTL, refresh after expiry
// ============================================================================

// TestDashboardCache_TTL_ReusesWithinWindow verifies SC-4 (BDD-2):
// Two calls within the TTL window return the same data without re-querying.
func TestDashboardCache_TTL_ReusesWithinWindow(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-001", "Task 1")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	counting := &countingBackend{Backend: backend}
	server := NewDashboardServer(counting, nil)

	// First call — populates cache
	resp1, err := server.GetStats(context.Background(), connect.NewRequest(&orcv1.GetStatsRequest{}))
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call — should come from cache
	resp2, err := server.GetStats(context.Background(), connect.NewRequest(&orcv1.GetStatsRequest{}))
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Both should return same completed count
	if resp1.Msg.Stats.TaskCounts.Completed != resp2.Msg.Stats.TaskCounts.Completed {
		t.Error("expected same results from cached data")
	}

	// Should have loaded at most once
	calls := counting.loadAllTasksCalls.Load()
	if calls > 1 {
		t.Errorf("expected 1 LoadAllTasks call (cached second call), got %d", calls)
	}
}

// TestDashboardCache_TTL_RefreshesAfterExpiry verifies SC-4 (BDD-3):
// After TTL expires, a new LoadAllTasks call is made.
func TestDashboardCache_TTL_RefreshesAfterExpiry(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-001", "Task 1")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	counting := &countingBackend{Backend: backend}

	// Create cache with a very short TTL for testing
	cache := newDashboardCache(counting, 50*time.Millisecond)

	// First call
	tasks1, err := cache.Tasks()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if len(tasks1) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks1))
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Second call — should trigger fresh load
	tasks2, err := cache.Tasks()
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if len(tasks2) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks2))
	}

	calls := counting.loadAllTasksCalls.Load()
	if calls < 2 {
		t.Errorf("expected at least 2 LoadAllTasks calls after TTL expiry, got %d", calls)
	}
}

// TestDashboardCache_InvalidateOnWrite verifies SC-4 (BDD-4):
// When a task is saved, the cache is invalidated so the next read gets fresh data.
func TestDashboardCache_InvalidateOnWrite(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-001", "Task 1")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	counting := &countingBackend{Backend: backend}
	cache := newDashboardCache(counting, 30*time.Second)

	// First call — populates cache
	tasks1, err := cache.Tasks()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(tasks1) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks1))
	}

	// Add another task
	tk2 := task.NewProtoTask("TASK-002", "Task 2")
	tk2.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Invalidate cache (simulating write-through invalidation)
	cache.Invalidate()

	// Next call should get fresh data with 2 tasks
	tasks2, err := cache.Tasks()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(tasks2) != 2 {
		t.Errorf("expected 2 tasks after invalidation, got %d", len(tasks2))
	}
}

// ============================================================================
// SC-7: GetTopInitiatives batch loading (no N+1)
// ============================================================================

// TestGetTopInitiatives_BatchLoading_NoN1 verifies SC-7:
// GetTopInitiatives for tasks spanning 10 initiatives issues ≤ 2 queries total
// (one for tasks, one batch query for initiative titles).
func TestGetTopInitiatives_BatchLoading_NoN1(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create 10 initiatives
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("INIT-%03d", i+1)
		init := initiative.NewProtoInitiative(id, fmt.Sprintf("Initiative %d", i+1))
		if err := backend.SaveInitiativeProto(init); err != nil {
			t.Fatalf("save initiative: %v", err)
		}

		// Create 2 tasks per initiative
		for j := 0; j < 2; j++ {
			initID := id
			tk := task.NewProtoTask(fmt.Sprintf("TASK-%s-%d", id, j), fmt.Sprintf("Task %d-%d", i, j))
			tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
			tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
			tk.InitiativeId = &initID
			tk.CompletedAt = timestamppb.Now()
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("save task: %v", err)
			}
		}
	}

	counting := &countingBackend{Backend: backend}
	server := NewDashboardServer(counting, nil)

	resp, err := server.GetTopInitiatives(context.Background(), connect.NewRequest(&orcv1.GetTopInitiativesRequest{Limit: 10}))
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	// Should return all 10 initiatives
	if len(resp.Msg.Initiatives) != 10 {
		t.Errorf("expected 10 initiatives, got %d", len(resp.Msg.Initiatives))
	}

	// SC-7: Should use batch loading — at most 2 queries total
	// (1 LoadAllTasks + 1 batch LoadInitiatives), NOT 1 + 10 individual loads
	initCalls := counting.loadInitiativeCalls.Load()
	if initCalls > 2 {
		t.Errorf("expected ≤ 2 initiative load calls (batch), got %d (N+1 pattern)", initCalls)
	}

	// All initiatives should have real titles (not IDs)
	for _, init := range resp.Msg.Initiatives {
		if init.Title == init.Id {
			t.Errorf("initiative %s has ID as title — batch loading may have failed", init.Id)
		}
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestDashboardCache_EmptyDatabase verifies edge case:
// All endpoints return zero counts when database is empty.
func TestDashboardCache_EmptyDatabase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewDashboardServer(backend, nil)

	// GetStats with empty DB
	statsResp, err := server.GetStats(context.Background(), connect.NewRequest(&orcv1.GetStatsRequest{}))
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if statsResp.Msg.Stats.TaskCounts.All != 0 {
		t.Errorf("expected 0 total tasks, got %d", statsResp.Msg.Stats.TaskCounts.All)
	}

	// GetMetrics with empty DB
	metricsResp, err := server.GetMetrics(context.Background(), connect.NewRequest(&orcv1.GetMetricsRequest{}))
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if metricsResp.Msg.Metrics.TasksCompleted != 0 {
		t.Errorf("expected 0 completed tasks, got %d", metricsResp.Msg.Metrics.TasksCompleted)
	}

	// GetTopInitiatives with empty DB
	initResp, err := server.GetTopInitiatives(context.Background(), connect.NewRequest(&orcv1.GetTopInitiativesRequest{Limit: 10}))
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}
	if len(initResp.Msg.Initiatives) != 0 {
		t.Errorf("expected 0 initiatives, got %d", len(initResp.Msg.Initiatives))
	}

	// GetComparison with empty DB
	compResp, err := server.GetComparison(context.Background(), connect.NewRequest(&orcv1.GetComparisonRequest{Period: "week"}))
	if err != nil {
		t.Fatalf("GetComparison failed: %v", err)
	}
	if compResp.Msg.Comparison.Current.TasksCompleted != 0 {
		t.Errorf("expected 0 current completed tasks, got %d", compResp.Msg.Comparison.Current.TasksCompleted)
	}
}

// TestDashboardCache_ConcurrentReadWrite verifies no data race
// when cache is invalidated while reads are in progress.
func TestDashboardCache_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed initial data
	tk := task.NewProtoTask("TASK-001", "Task 1")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	cache := newDashboardCache(backend, 30*time.Second)

	// Run concurrent reads and invalidations
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.Tasks()
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Invalidate()
		}()
	}

	wg.Wait()
	// Test passes if no race detected (run with -race)
}

// TestDashboardCache_LoadError_NotCached verifies failure mode:
// If LoadAllTasks returns an error, the error is NOT cached.
// Next call should try again.
func TestDashboardCache_LoadError_NotCached(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a failing backend for the first call
	failing := &failingBackend{
		Backend:   backend,
		failCount: 1, // Fail first call only
	}

	cache := newDashboardCache(failing, 30*time.Second)

	// First call should fail
	_, err := cache.Tasks()
	if err == nil {
		t.Fatal("expected error from failing backend")
	}

	// Second call should succeed (error not cached)
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	tasks, err := cache.Tasks()
	if err != nil {
		t.Fatalf("second call should succeed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

// TestDashboardCache_SingleflightError verifies failure mode:
// When singleflight load fails, ALL concurrent waiters receive the error.
func TestDashboardCache_SingleflightError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	failing := &failingBackend{
		Backend:   backend,
		failCount: 100, // Always fail
	}

	cache := newDashboardCache(failing, 30*time.Second)

	var wg sync.WaitGroup
	errors := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errors[idx] = cache.Tasks()
		}(i)
	}

	wg.Wait()

	// All should receive errors
	for i, err := range errors {
		if err == nil {
			t.Errorf("waiter %d: expected error, got nil", i)
		}
	}
}

// TestDashboardCache_NullDates verifies edge case:
// Tasks with NULL completed_at/updated_at are excluded from time-filtered
// aggregates but included in total counts.
func TestDashboardCache_NullDates(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Task with no completed_at (still running)
	tk := task.NewProtoTask("TASK-001", "Running task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	// No CompletedAt set

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	resp, err := server.GetStats(context.Background(), connect.NewRequest(&orcv1.GetStatsRequest{}))
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Should appear in total count
	if resp.Msg.Stats.TaskCounts.All != 1 {
		t.Errorf("expected 1 total task, got %d", resp.Msg.Stats.TaskCounts.All)
	}
	// But not in completed
	if resp.Msg.Stats.TaskCounts.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", resp.Msg.Stats.TaskCounts.Completed)
	}
}

// TestDashboardCache_AllTasksCreatedToday verifies edge case:
// When all tasks are from today, daily metrics show a single day.
func TestDashboardCache_AllTasksCreatedToday(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	now := time.Now()
	for i := 0; i < 5; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i+1), fmt.Sprintf("Task %d", i+1))
		tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
		tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		tk.CreatedAt = timestamppb.New(now)
		tk.CompletedAt = timestamppb.New(now)
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	server := NewDashboardServer(backend, nil)

	resp, err := server.GetDailyMetrics(context.Background(), connect.NewRequest(&orcv1.GetDailyMetricsRequest{Days: 30}))
	if err != nil {
		t.Fatalf("GetDailyMetrics failed: %v", err)
	}

	// Should have exactly 1 day
	if len(resp.Msg.Stats.Days) != 1 {
		t.Errorf("expected 1 day in metrics, got %d", len(resp.Msg.Stats.Days))
	}
	if len(resp.Msg.Stats.Days) > 0 && resp.Msg.Stats.Days[0].TasksCompleted != 5 {
		t.Errorf("expected 5 completed on that day, got %d", resp.Msg.Stats.Days[0].TasksCompleted)
	}
}

// TestTopInitiatives_MissingTitle_FallsBackToID verifies edge case:
// When an initiative title is empty, display the initiative ID instead.
func TestTopInitiatives_MissingTitle_FallsBackToID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Save initiative with empty title
	init := &orcv1.Initiative{Id: "INIT-001", Title: ""}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	initID := "INIT-001"
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	tk.InitiativeId = &initID
	tk.CompletedAt = timestamppb.Now()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewDashboardServer(backend, nil)

	resp, err := server.GetTopInitiatives(context.Background(), connect.NewRequest(&orcv1.GetTopInitiativesRequest{Limit: 10}))
	if err != nil {
		t.Fatalf("GetTopInitiatives failed: %v", err)
	}

	if len(resp.Msg.Initiatives) == 0 {
		t.Fatal("expected at least one initiative")
	}
	if resp.Msg.Initiatives[0].Title != "INIT-001" {
		t.Errorf("expected fallback to ID 'INIT-001', got '%s'", resp.Msg.Initiatives[0].Title)
	}
}

// ============================================================================
// Test Helpers — counting/failing backend wrappers
// ============================================================================

// countingBackend wraps a Backend and counts calls to specific methods.
// Used to verify caching/singleflight reduces backend calls.
type countingBackend struct {
	storage.Backend
	loadAllTasksCalls  atomic.Int64
	loadInitiativeCalls atomic.Int64
}

func (c *countingBackend) LoadAllTasks() ([]*orcv1.Task, error) {
	c.loadAllTasksCalls.Add(1)
	return c.Backend.LoadAllTasks()
}

func (c *countingBackend) LoadInitiative(id string) (*initiative.Initiative, error) {
	c.loadInitiativeCalls.Add(1)
	return c.Backend.LoadInitiative(id)
}

// failingBackend wraps a Backend and fails the first N LoadAllTasks calls.
type failingBackend struct {
	storage.Backend
	failCount int
	callCount atomic.Int64
}

func (f *failingBackend) LoadAllTasks() ([]*orcv1.Task, error) {
	n := f.callCount.Add(1)
	if int(n) <= f.failCount {
		return nil, fmt.Errorf("simulated database error")
	}
	return f.Backend.LoadAllTasks()
}
