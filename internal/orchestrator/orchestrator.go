package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/storage"
)

// Config holds orchestrator configuration.
type Config struct {
	MaxConcurrent int           // Maximum parallel tasks (default: 4)
	PollInterval  time.Duration // State polling interval (default: 2s)
	WorkerTimeout time.Duration // Max time per task (0 = unlimited)
}

// DefaultConfig returns the default orchestrator configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxConcurrent: 4,
		PollInterval:  2 * time.Second,
		WorkerTimeout: 0,
	}
}

// Status represents the orchestrator status.
type Status string

const (
	StatusStopped Status = "stopped"
	StatusRunning Status = "running"
	StatusPaused  Status = "paused"
)

// OrchestratorStatus contains current orchestrator state.
type OrchestratorStatus struct {
	Status         Status   `json:"status"`
	ActiveCount    int      `json:"active_count"`
	MaxConcurrent  int      `json:"max_concurrent"`
	QueueLength    int      `json:"queue_length"`
	CompletedCount int      `json:"completed_count"`
	FailedCount    int      `json:"failed_count"`
	RunningTasks   []string `json:"running_tasks"`
}

// Orchestrator coordinates multiple Claude agents running in parallel.
type Orchestrator struct {
	config     *Config
	orcConfig  *config.Config
	scheduler  *Scheduler
	workerPool *WorkerPool
	publisher  events.Publisher
	gitOps     *git.Git
	promptSvc  *prompt.Service
	backend    storage.Backend
	logger     *slog.Logger

	status      Status
	failedCount int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// New creates a new orchestrator.
func New(cfg *Config, orcConfig *config.Config, publisher events.Publisher, gitOps *git.Git, promptSvc *prompt.Service, backend storage.Backend, logger *slog.Logger) *Orchestrator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Orchestrator{
		config:     cfg,
		orcConfig:  orcConfig,
		scheduler:  NewScheduler(cfg.MaxConcurrent),
		workerPool: NewWorkerPool(cfg.MaxConcurrent, publisher, orcConfig, gitOps, promptSvc, backend),
		publisher:  publisher,
		gitOps:     gitOps,
		promptSvc:  promptSvc,
		backend:    backend,
		logger:     logger,
		status:     StatusStopped,
	}
}

// Start begins orchestration.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.status == StatusRunning {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator already running")
	}

	o.ctx, o.cancel = context.WithCancel(ctx)
	o.status = StatusRunning
	o.mu.Unlock()

	o.logger.Info("orchestrator started",
		"max_concurrent", o.config.MaxConcurrent,
		"poll_interval", o.config.PollInterval)

	// Start main loop
	o.wg.Add(1)
	go o.mainLoop()

	return nil
}

// Stop gracefully stops the orchestrator.
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	if o.status != StatusRunning {
		o.mu.Unlock()
		return nil
	}
	o.status = StatusStopped
	o.mu.Unlock()

	o.cancel()
	o.wg.Wait()

	o.logger.Info("orchestrator stopped")
	return nil
}

// mainLoop is the orchestrator's main execution loop.
func (o *Orchestrator) mainLoop() {
	defer o.wg.Done()

	ticker := time.NewTicker(o.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.tick()
		}
	}
}

// tick performs one orchestration cycle.
func (o *Orchestrator) tick() {
	// Check for completed/failed workers
	o.checkWorkers()

	// Schedule new tasks
	o.scheduleNext()
}

// checkWorkers checks worker status and handles completions/failures.
func (o *Orchestrator) checkWorkers() {
	workers := o.workerPool.GetWorkers()

	for taskID, worker := range workers {
		status := worker.GetStatus()

		switch status {
		case WorkerStatusComplete:
			o.handleWorkerComplete(taskID, worker)
		case WorkerStatusFailed:
			o.handleWorkerFailed(taskID, worker)
		}
	}
}

// handleWorkerComplete handles a completed worker.
// This is idempotent - the worker may have already removed itself from the pool.
func (o *Orchestrator) handleWorkerComplete(taskID string, worker *Worker) {
	// Check if worker still exists in pool (may have already self-removed)
	if o.workerPool.GetWorker(taskID) == nil {
		return // Already cleaned up
	}

	o.logger.Info("task completed", "task_id", taskID)

	// Mark in scheduler
	o.scheduler.MarkCompleted(taskID)

	// Cleanup worktree
	if err := o.workerPool.CleanupWorktree(taskID, true, false); err != nil {
		o.logger.Warn("cleanup worktree failed", "task_id", taskID, "error", err)
	}

	// Remove worker (idempotent - no-op if already removed)
	o.workerPool.RemoveWorker(taskID)

	// Publish event
	if o.publisher != nil {
		o.publisher.Publish(events.Event{
			Type:   events.EventComplete,
			TaskID: taskID,
		})
	}
}

// handleWorkerFailed handles a failed worker.
// This is idempotent - the worker may have already removed itself from the pool.
func (o *Orchestrator) handleWorkerFailed(taskID string, worker *Worker) {
	// Check if worker still exists in pool (may have already self-removed)
	if o.workerPool.GetWorker(taskID) == nil {
		return // Already cleaned up
	}

	err := worker.GetError()
	o.logger.Error("task failed", "task_id", taskID, "error", err)

	o.mu.Lock()
	o.failedCount++
	o.mu.Unlock()

	// Mark in scheduler
	o.scheduler.MarkFailed(taskID)

	// Cleanup worktree (keep for debugging)
	if cleanupErr := o.workerPool.CleanupWorktree(taskID, false, true); cleanupErr != nil {
		o.logger.Warn("cleanup worktree failed", "task_id", taskID, "error", cleanupErr)
	}

	// Remove worker (idempotent - no-op if already removed)
	o.workerPool.RemoveWorker(taskID)

	// Publish event
	if o.publisher != nil {
		errMsg := "unknown error"
		if err != nil {
			errMsg = err.Error()
		}
		o.publisher.Publish(events.Event{
			Type:   events.EventError,
			TaskID: taskID,
			Data: map[string]any{
				"error": errMsg,
			},
		})
	}
}

// scheduleNext schedules the next available tasks.
func (o *Orchestrator) scheduleNext() {
	// Get ready tasks
	ready := o.scheduler.NextReady(0)
	if len(ready) == 0 {
		return
	}

	for _, scheduled := range ready {
		if err := o.spawnTask(scheduled.TaskID); err != nil {
			o.logger.Error("spawn task failed",
				"task_id", scheduled.TaskID,
				"error", err)
			o.scheduler.MarkFailed(scheduled.TaskID)
		}
	}
}

// spawnTask spawns a worker for a task.
func (o *Orchestrator) spawnTask(taskID string) error {
	// Load task (includes execution state in task.Execution)
	t, err := o.backend.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}

	// Create plan dynamically from task weight
	pln := createPlanForWeight(taskID, t.Weight)

	// Spawn worker (task's Execution field contains execution state)
	_, err = o.workerPool.SpawnWorker(o.ctx, t, pln)
	if err != nil {
		return fmt.Errorf("spawn worker: %w", err)
	}

	o.logger.Info("task started", "task_id", taskID)
	return nil
}

// AddTask adds a task to the orchestrator queue.
func (o *Orchestrator) AddTask(taskID, title string, dependsOn []string, priority TaskPriority) {
	o.scheduler.AddTask(taskID, title, dependsOn, priority)
}

// AddTasksFromInitiative adds all pending tasks from an initiative.
func (o *Orchestrator) AddTasksFromInitiative(init *initiative.Initiative) error {
	for _, taskRef := range init.Tasks {
		if taskRef.Status == "pending" {
			o.AddTask(taskRef.ID, taskRef.Title, taskRef.DependsOn, PriorityDefault)
		}
	}
	return nil
}

// AddPendingTasks adds all pending tasks from the task store.
func (o *Orchestrator) AddPendingTasks() error {
	tasks, err := o.backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_CREATED || t.Status == orcv1.TaskStatus_TASK_STATUS_PLANNED {
			o.AddTask(t.Id, t.Title, nil, PriorityDefault)
		}
	}
	return nil
}

// Status returns the current orchestrator status.
func (o *Orchestrator) Status() *OrchestratorStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return &OrchestratorStatus{
		Status:         o.status,
		ActiveCount:    o.workerPool.ActiveCount(),
		MaxConcurrent:  o.config.MaxConcurrent,
		QueueLength:    o.scheduler.QueueLength(),
		CompletedCount: o.scheduler.CompletedCount(),
		FailedCount:    o.failedCount,
		RunningTasks:   o.scheduler.GetRunningTasks(),
	}
}

// Wait blocks until all tasks are complete.
func (o *Orchestrator) Wait() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			if o.scheduler.IsComplete() && o.workerPool.ActiveCount() == 0 {
				return
			}
		}
	}
}

// createPlanForWeight creates an execution plan based on task weight.
// Plans are created dynamically for execution, not stored.
func createPlanForWeight(taskID string, weight orcv1.TaskWeight) *executor.Plan {
	var phases []executor.PhaseDisplay

	switch weight {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "breakdown", Name: "Breakdown", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	default:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}
}
