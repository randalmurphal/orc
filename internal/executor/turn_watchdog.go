package executor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const DefaultProviderInactivityTimeout = 20 * time.Minute

type TurnWatchdog struct {
	timeout      time.Duration
	lastActivity time.Time
	cancel       context.CancelFunc

	mu      sync.RWMutex
	tripped bool
}

func NewTurnWatchdog(timeout time.Duration, cancel context.CancelFunc) *TurnWatchdog {
	if timeout <= 0 {
		timeout = DefaultProviderInactivityTimeout
	}
	return &TurnWatchdog{
		timeout:      timeout,
		lastActivity: time.Now(),
		cancel:       cancel,
	}
}

func (w *TurnWatchdog) Start(ctx context.Context) {
	if w == nil {
		return
	}

	interval := w.timeout / 4
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	if interval > time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if w.IdleFor() < w.timeout {
					continue
				}
				w.mu.Lock()
				if w.tripped {
					w.mu.Unlock()
					return
				}
				w.tripped = true
				w.mu.Unlock()
				if w.cancel != nil {
					w.cancel()
				}
				return
			}
		}
	}()
}

func (w *TurnWatchdog) RecordActivity() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastActivity = time.Now()
}

func (w *TurnWatchdog) IdleFor() time.Duration {
	if w == nil {
		return 0
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return time.Since(w.lastActivity)
}

func (w *TurnWatchdog) Tripped() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tripped
}

func (w *TurnWatchdog) Timeout() time.Duration {
	if w == nil {
		return 0
	}
	return w.timeout
}

func (w *TurnWatchdog) Error(provider string) error {
	if !w.Tripped() {
		return nil
	}
	if provider == "" {
		provider = "provider"
	}
	return fmt.Errorf("%s stalled after %v without output", provider, w.timeout)
}
