package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// WorkerStatus represents the status of a worker.
type WorkerStatus string

const (
	WorkerStatusIdle     WorkerStatus = "idle"
	WorkerStatusRunning  WorkerStatus = "running"
	WorkerStatusPaused   WorkerStatus = "paused"
	WorkerStatusComplete WorkerStatus = "complete"
	WorkerStatusFailed   WorkerStatus = "failed"
)

// Worker executes a single task in its worktree.
type Worker struct {
	ID           string
	TaskID       string
	WorktreePath string
	Status       WorkerStatus
	StartedAt    time.Time
	Error        error

	ctx       context.Context
	cancel    context.CancelFunc
	cmd       *exec.Cmd
	eventChan chan events.Event
	mu        sync.RWMutex
}

// WorkerPool manages a pool of workers executing tasks.
type WorkerPool struct {
	workers    map[string]*Worker
	maxWorkers int
	publisher  events.Publisher
	cfg        *config.Config
	gitOps     *git.Git
	promptSvc  *prompt.Service
	backend    storage.Backend
	eventChan  chan events.Event
	mu         sync.RWMutex
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(maxWorkers int, publisher events.Publisher, cfg *config.Config, gitOps *git.Git, promptSvc *prompt.Service, backend storage.Backend) *WorkerPool {
	return &WorkerPool{
		workers:    make(map[string]*Worker),
		maxWorkers: maxWorkers,
		publisher:  publisher,
		cfg:        cfg,
		gitOps:     gitOps,
		promptSvc:  promptSvc,
		backend:    backend,
		eventChan:  make(chan events.Event, 100),
	}
}

// SpawnWorker creates and starts a worker for a task.
func (p *WorkerPool) SpawnWorker(ctx context.Context, t *task.Task, pln *plan.Plan, st *state.State) (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if worker already exists
	if _, exists := p.workers[t.ID]; exists {
		return nil, fmt.Errorf("worker already exists for task %s", t.ID)
	}

	// Check capacity
	if len(p.workers) >= p.maxWorkers {
		return nil, fmt.Errorf("worker pool at capacity (%d)", p.maxWorkers)
	}

	// Setup worktree
	setup, err := executor.SetupWorktree(t.ID, p.cfg, p.gitOps)
	if err != nil {
		return nil, fmt.Errorf("setup worktree: %w", err)
	}

	// Create worker context
	workerCtx, cancel := context.WithCancel(ctx)

	worker := &Worker{
		ID:           fmt.Sprintf("worker-%s", t.ID),
		TaskID:       t.ID,
		WorktreePath: setup.Path,
		Status:       WorkerStatusRunning,
		StartedAt:    time.Now(),
		ctx:          workerCtx,
		cancel:       cancel,
		eventChan:    p.eventChan,
	}

	p.workers[t.ID] = worker

	// Start execution in goroutine
	go worker.run(p, t, pln, st)

	return worker, nil
}

// run executes the task in the worktree.
// Iterates through all phases until completion, failure, or cancellation.
func (w *Worker) run(pool *WorkerPool, t *task.Task, pln *plan.Plan, st *state.State) {
	defer func() {
		w.mu.Lock()
		if w.Status == WorkerStatusRunning {
			w.Status = WorkerStatusComplete
		}
		w.mu.Unlock()

		// Remove worker from pool immediately after setting final status.
		// This ensures capacity is freed without waiting for the next tick.
		pool.RemoveWorker(w.TaskID)
	}()

	// Iterate through phases until done
	for {
		// Get current phase
		currentPhase := pln.CurrentPhase()
		if currentPhase == nil {
			w.setStatus(WorkerStatusComplete)
			return
		}

		// Get phase prompt
		promptData, err := pool.promptSvc.Get(currentPhase.ID)
		if err != nil {
			w.setError(fmt.Errorf("load phase prompt: %w", err))
			return
		}
		phasePrompt := promptData.Content

		// Create ralph state file in worktree
		mgr := executor.NewRalphStateManager(w.WorktreePath)
		err = mgr.Create(t.ID, currentPhase.ID, phasePrompt,
			executor.WithMaxIterations(30),
			executor.WithCompletionPromise("PHASE_COMPLETE"),
		)
		if err != nil {
			w.setError(fmt.Errorf("create ralph state: %w", err))
			return
		}

		// Build claude command
		args := []string{
			"-p", phasePrompt,
			"--dangerously-skip-permissions",
		}

		if pool.cfg != nil && pool.cfg.Model != "" {
			args = append(args, "--model", pool.cfg.Model)
		}

		w.cmd = exec.CommandContext(w.ctx, "claude", args...)
		w.cmd.Dir = w.WorktreePath
		w.cmd.Stdout = os.Stdout
		w.cmd.Stderr = os.Stderr

		// Publish start event
		pool.publishEvent(events.Event{
			Type:   events.EventPhase,
			TaskID: t.ID,
			Data: map[string]any{
				"phase":    currentPhase.ID,
				"status":   "started",
				"worktree": w.WorktreePath,
			},
		})

		// Run claude
		if err := w.cmd.Run(); err != nil {
			// Check if context was cancelled
			if w.ctx.Err() != nil {
				w.setStatus(WorkerStatusPaused)
				return
			}
			w.setError(fmt.Errorf("claude execution: %w", err))
			return
		}

		// Check if ralph state file was removed (completion)
		if !mgr.Exists() {
			// Phase completed
			st.CompletePhase(currentPhase.ID, "")
			if pool.backend != nil {
				_ = pool.backend.SaveState(st)
			}

			pool.publishEvent(events.Event{
				Type:   events.EventPhase,
				TaskID: t.ID,
				Data: map[string]any{
					"phase":  currentPhase.ID,
					"status": "completed",
				},
			})

			// Mark phase as completed in plan
			pln.GetPhase(currentPhase.ID).Status = plan.PhaseCompleted
			if pool.backend != nil {
				_ = pool.backend.SavePlan(pln, t.ID)
			}

			// Check if more phases - loop continues with next iteration
			nextPhase := pln.CurrentPhase()
			if nextPhase == nil {
				// Task complete
				st.Complete()
				if pool.backend != nil {
					_ = pool.backend.SaveState(st)
				}
				w.setStatus(WorkerStatusComplete)
				return
			}
			// Next iteration will process nextPhase
			continue
		}

		// Ralph state file still exists - phase not complete, wait for external completion
		return
	}
}

// setStatus sets the worker status.
func (w *Worker) setStatus(status WorkerStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = status
}

// setError sets the worker error and status.
func (w *Worker) setError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Error = err
	w.Status = WorkerStatusFailed
}

// GetStatus returns the current worker status.
func (w *Worker) GetStatus() WorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

// GetError returns the worker error if any.
func (w *Worker) GetError() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Error
}

// Stop stops the worker.
func (w *Worker) Stop() {
	w.cancel()
}

// publishEvent publishes an event if publisher is available.
func (p *WorkerPool) publishEvent(event events.Event) {
	if p.publisher != nil {
		p.publisher.Publish(event)
	}
}

// StopWorker stops a specific worker.
func (p *WorkerPool) StopWorker(taskID string) error {
	p.mu.Lock()
	worker, exists := p.workers[taskID]
	p.mu.Unlock()

	if !exists {
		return fmt.Errorf("worker not found for task %s", taskID)
	}

	worker.Stop()
	return nil
}

// GetWorker returns a worker by task ID.
func (p *WorkerPool) GetWorker(taskID string) *Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workers[taskID]
}

// ActiveCount returns the number of active workers.
func (p *WorkerPool) ActiveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, w := range p.workers {
		if w.GetStatus() == WorkerStatusRunning {
			count++
		}
	}
	return count
}

// RemoveWorker removes a worker from the pool.
func (p *WorkerPool) RemoveWorker(taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.workers, taskID)
}

// CleanupWorktree cleans up the worktree for a task.
func (p *WorkerPool) CleanupWorktree(taskID string, completed, failed bool) error {
	if executor.ShouldCleanupWorktree(completed, failed, p.cfg) {
		return executor.CleanupWorktree(taskID, p.gitOps)
	}
	return nil
}

// GetWorkers returns all workers.
func (p *WorkerPool) GetWorkers() map[string]*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy
	workers := make(map[string]*Worker, len(p.workers))
	for k, v := range p.workers {
		workers[k] = v
	}
	return workers
}

// WorktreePath returns the worktree path for a task.
func WorktreePath(taskID string, cfg *config.Config) string {
	worktreeDir := ".orc/worktrees"
	if cfg != nil && cfg.Worktree.Dir != "" {
		worktreeDir = cfg.Worktree.Dir
	}
	return filepath.Join(worktreeDir, fmt.Sprintf("orc-%s", taskID))
}
