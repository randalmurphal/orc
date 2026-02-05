package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// HeartbeatChecker provides the last heartbeat time for staleness checks.
type HeartbeatChecker interface {
	LastHeartbeatTime() (time.Time, error)
}

// IdleGuardConfig configures the IdleGuard behavior.
type IdleGuardConfig struct {
	// CheckInterval is how often the guard polls the heartbeat.
	CheckInterval time.Duration
	// StaleTimeout is the duration after which a heartbeat is considered stale.
	StaleTimeout time.Duration
	// Checker provides the heartbeat time to check.
	Checker HeartbeatChecker
	// OnStale is called when a stale heartbeat is detected.
	// The age parameter is how long since the last heartbeat.
	OnStale func(age time.Duration)
	// Logger for diagnostic output.
	Logger *slog.Logger
}

// IdleGuard monitors heartbeat freshness at the executor level.
// It periodically checks whether the heartbeat has been updated within the
// configured timeout, and fires a callback if staleness is detected.
type IdleGuard struct {
	config IdleGuardConfig
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewIdleGuard creates an IdleGuard with the given configuration.
func NewIdleGuard(config IdleGuardConfig) *IdleGuard {
	return &IdleGuard{
		config: config,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start begins the heartbeat monitoring loop in a background goroutine.
// The loop runs until Stop() is called or the context is canceled.
func (g *IdleGuard) Start(ctx context.Context) {
	go g.run(ctx)
}

// Stop signals the monitoring loop to stop and waits for it to finish.
func (g *IdleGuard) Stop() {
	close(g.stopCh)
	<-g.doneCh
}

func (g *IdleGuard) run(ctx context.Context) {
	defer close(g.doneCh)

	ticker := time.NewTicker(g.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.check()
		}
	}
}

func (g *IdleGuard) check() {
	lastHB, err := g.config.Checker.LastHeartbeatTime()
	if err != nil {
		g.config.Logger.Warn("idle guard: heartbeat check failed", "error", err)
		return
	}

	age := time.Since(lastHB)
	if age > g.config.StaleTimeout {
		g.config.OnStale(age)
	}
}

// TaskLoader loads a task by ID. Satisfied by storage.Backend.
type TaskLoader interface {
	LoadTask(id string) (*orcv1.Task, error)
}

// taskHeartbeatChecker implements HeartbeatChecker by loading the task from storage.
type taskHeartbeatChecker struct {
	loader TaskLoader
	taskID string
}

func (c *taskHeartbeatChecker) LastHeartbeatTime() (time.Time, error) {
	t, err := c.loader.LoadTask(c.taskID)
	if err != nil {
		return time.Time{}, fmt.Errorf("load task %s: %w", c.taskID, err)
	}
	if t.LastHeartbeat == nil {
		return time.Time{}, nil
	}
	return t.LastHeartbeat.AsTime(), nil
}
