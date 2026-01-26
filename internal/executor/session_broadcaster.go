package executor

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

const (
	// SessionBroadcastInterval is how often session updates are broadcast while tasks are running.
	SessionBroadcastInterval = 10 * time.Second
)

// SessionBroadcaster manages periodic and event-driven session update broadcasts.
// It tracks session metrics (duration, tokens, cost, running tasks) and broadcasts
// updates to WebSocket clients via the event publisher.
//
// Thread-safe: All methods can be called concurrently.
type SessionBroadcaster struct {
	publisher *events.PublishHelper
	backend   storage.Backend
	globalDB  *db.GlobalDB
	logger    *slog.Logger
	workDir   string

	// Session start time (set when first task starts)
	sessionStart time.Time

	// Pause state
	isPaused atomic.Bool

	// Ticker management
	mu       sync.Mutex
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
	cancelFn context.CancelFunc

	// Running task count (updated atomically for fast reads)
	tasksRunning atomic.Int32
}

// NewSessionBroadcaster creates a new session broadcaster.
func NewSessionBroadcaster(
	publisher *events.PublishHelper,
	backend storage.Backend,
	globalDB *db.GlobalDB,
	workDir string,
	logger *slog.Logger,
) *SessionBroadcaster {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionBroadcaster{
		publisher: publisher,
		backend:   backend,
		globalDB:  globalDB,
		workDir:   workDir,
		logger:    logger,
	}
}

// OnTaskStart should be called when a task begins execution.
// Broadcasts an immediate session update and starts the periodic ticker if not already running.
func (sb *SessionBroadcaster) OnTaskStart(ctx context.Context) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Initialize session start time on first task
	if sb.sessionStart.IsZero() {
		sb.sessionStart = time.Now()
	}

	// Increment running count
	sb.tasksRunning.Add(1)

	// Start ticker if not running
	if !sb.running {
		sb.startTickerLocked(ctx)
	}

	// Broadcast immediate update (task started)
	sb.broadcastLocked()
}

// OnTaskComplete should be called when a task finishes (completed, failed, paused).
// Broadcasts an immediate session update and stops the ticker if no tasks are running.
func (sb *SessionBroadcaster) OnTaskComplete(ctx context.Context) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Decrement running count
	newCount := sb.tasksRunning.Add(-1)

	// Broadcast immediate update (task completed/tokens changed)
	sb.broadcastLocked()

	// Stop ticker if no tasks running
	if newCount <= 0 && sb.running {
		sb.stopTickerLocked()
	}
}

// OnPauseChanged should be called when the executor pause state changes.
// Broadcasts an immediate session update with the new pause state.
func (sb *SessionBroadcaster) OnPauseChanged(isPaused bool) {
	sb.isPaused.Store(isPaused)

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Broadcast immediate update (pause state changed)
	sb.broadcastLocked()
}

// Stop stops the broadcaster and cleans up resources.
func (sb *SessionBroadcaster) Stop() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.running {
		sb.stopTickerLocked()
	}
}

// GetCurrentMetrics returns the current session metrics without broadcasting.
// Useful for serving initial state to reconnecting clients.
func (sb *SessionBroadcaster) GetCurrentMetrics() events.SessionUpdate {
	return sb.buildUpdate()
}

// startTickerLocked starts the periodic broadcast ticker.
// Must be called with mu held.
func (sb *SessionBroadcaster) startTickerLocked(ctx context.Context) {
	ticker := time.NewTicker(SessionBroadcastInterval)
	stopCh := make(chan struct{})

	sb.ticker = ticker
	sb.stopCh = stopCh
	sb.running = true

	// Create a cancellable context for the ticker goroutine
	tickerCtx, cancel := context.WithCancel(ctx)
	sb.cancelFn = cancel

	// Pass ticker and stopCh as parameters to avoid race with stopTickerLocked
	go sb.tickerLoop(tickerCtx, ticker, stopCh)
}

// stopTickerLocked stops the periodic broadcast ticker.
// Must be called with mu held.
func (sb *SessionBroadcaster) stopTickerLocked() {
	if sb.ticker != nil {
		sb.ticker.Stop()
		sb.ticker = nil
	}
	if sb.cancelFn != nil {
		sb.cancelFn()
		sb.cancelFn = nil
	}
	if sb.stopCh != nil {
		close(sb.stopCh)
		sb.stopCh = nil
	}
	sb.running = false
}

// tickerLoop runs the periodic broadcast loop.
// ticker and stopCh are passed as parameters to avoid race conditions.
func (sb *SessionBroadcaster) tickerLoop(ctx context.Context, ticker *time.Ticker, stopCh <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			sb.mu.Lock()
			if sb.running {
				sb.broadcastLocked()
			}
			sb.mu.Unlock()
		}
	}
}

// broadcastLocked publishes a session update event.
// Must be called with mu held.
func (sb *SessionBroadcaster) broadcastLocked() {
	if sb.publisher == nil {
		return
	}
	update := sb.buildUpdate()
	sb.publisher.Session(update)
	sb.logger.Debug("session update broadcast",
		"duration_seconds", update.DurationSeconds,
		"tasks_running", update.TasksRunning,
		"total_tokens", update.TotalTokens,
		"cost_usd", update.EstimatedCostUSD,
	)
}

// buildUpdate constructs the current session update from data sources.
func (sb *SessionBroadcaster) buildUpdate() events.SessionUpdate {
	update := events.SessionUpdate{
		TasksRunning: int(sb.tasksRunning.Load()),
		IsPaused:     sb.isPaused.Load(),
	}

	// Calculate duration
	if !sb.sessionStart.IsZero() {
		update.DurationSeconds = int64(time.Since(sb.sessionStart).Seconds())
	}

	// Get token/cost data from global DB (cross-project aggregation)
	if sb.globalDB != nil {
		// Get today's cost summary to avoid counting historical costs
		today := time.Now().UTC().Truncate(24 * time.Hour)
		summary, err := sb.globalDB.GetCostSummary(sb.workDir, today)
		if err != nil {
			sb.logger.Debug("failed to get cost summary", "error", err)
		} else if summary != nil {
			update.TotalTokens = summary.TotalInput + summary.TotalOutput
			update.InputTokens = summary.TotalInput
			update.OutputTokens = summary.TotalOutput
			update.EstimatedCostUSD = summary.TotalCostUSD
		}
	}

	// If no global DB, try to get running task count from backend
	if sb.backend != nil && update.TasksRunning == 0 {
		// Fall back to counting from backend if atomic counter is wrong
		tasks, err := sb.backend.LoadAllTasksProto()
		if err == nil {
			count := 0
			for _, t := range tasks {
				if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
					count++
				}
			}
			update.TasksRunning = count
		}
	}

	return update
}
