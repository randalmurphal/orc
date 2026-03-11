package executor

import (
	"context"
	"testing"
	"time"
)

func TestTurnWatchdogTripsAfterInactivity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wd := NewTurnWatchdog(30*time.Millisecond, cancel)
	wd.Start(ctx)

	select {
	case <-ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watchdog did not cancel context")
	}

	if !wd.Tripped() {
		t.Fatal("expected watchdog to trip")
	}
}

func TestTurnWatchdogRecordActivityDelaysTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wd := NewTurnWatchdog(80*time.Millisecond, cancel)
	wd.Start(ctx)

	time.Sleep(40 * time.Millisecond)
	wd.RecordActivity()

	select {
	case <-ctx.Done():
		t.Fatal("watchdog tripped too early after activity")
	case <-time.After(50 * time.Millisecond):
	}
}
