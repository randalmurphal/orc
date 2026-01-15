// Package executor contains task execution logic.
package executor

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestProcessSnapshot verifies snapshot captures correct fields.
func TestProcessSnapshot(t *testing.T) {
	// Create a resource tracker with tracking enabled
	config := ResourceTrackerConfig{
		Enabled:           true,
		MemoryThresholdMB: 100,
	}
	tracker := NewResourceTracker(config, slog.Default())

	// Take a snapshot
	err := tracker.SnapshotBefore()
	if err != nil {
		t.Fatalf("SnapshotBefore failed: %v", err)
	}

	// Verify snapshot was captured
	snapshot := tracker.GetBeforeSnapshot()
	if snapshot == nil {
		t.Fatal("expected before snapshot to be captured")
	}

	// Verify snapshot has processes
	if snapshot.ProcessCount == 0 {
		t.Error("expected snapshot to contain processes")
	}

	// Verify process fields are populated
	if len(snapshot.Processes) > 0 {
		p := snapshot.Processes[0]
		if p.PID == 0 {
			t.Error("expected PID to be non-zero")
		}
		// Command might be empty for kernel threads, but most should have a command
	}

	// Verify timestamp is set
	if snapshot.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}

	// Verify total memory is calculated
	if snapshot.ProcessCount > 0 && snapshot.TotalMemoryMB == 0 {
		t.Log("warning: total memory is zero (might be expected in some environments)")
	}
}

// TestOrphanDetection verifies orphan detection logic with mock data.
func TestOrphanDetection(t *testing.T) {
	// Create a tracker
	config := ResourceTrackerConfig{
		Enabled:            true,
		MemoryThresholdMB:  100,
		LogOrphanedMCPOnly: false,
	}
	tracker := NewResourceTracker(config, slog.Default())

	// Manually set up before/after snapshots to test detection logic
	// Before: processes A, B, C exist
	tracker.beforeSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			{PID: 100, PPID: 1, Command: "init"},
			{PID: 200, PPID: 100, Command: "parent-process"},
			{PID: 300, PPID: 200, Command: "child-process"},
		},
		ProcessCount: 3,
	}

	// After: A, B exist; C still exists (not orphan, parent still exists)
	// D is new and orphaned (PPID 1 = reparented to init)
	// E is new but has existing parent (not orphan)
	tracker.afterSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			{PID: 100, PPID: 1, Command: "init"},
			{PID: 200, PPID: 100, Command: "parent-process"},
			{PID: 300, PPID: 200, Command: "child-process"},
			{PID: 400, PPID: 1, Command: "orphaned-process", IsMCP: false},     // New, PPID=1 = orphan
			{PID: 500, PPID: 200, Command: "new-child-process", IsMCP: false},  // New, has parent = not orphan
			{PID: 600, PPID: 999, Command: "orphan-missing-parent", IsMCP: false}, // New, parent doesn't exist = orphan
		},
		ProcessCount: 6,
	}

	orphans := tracker.DetectOrphans()

	// Should find 2 orphans: PID 400 (PPID=1) and PID 600 (missing parent)
	if len(orphans) != 2 {
		t.Errorf("expected 2 orphans, got %d", len(orphans))
		for _, o := range orphans {
			t.Logf("  orphan: PID=%d PPID=%d Command=%s", o.PID, o.PPID, o.Command)
		}
	}

	// Verify the orphans are the expected ones
	foundOrphaned := false
	foundMissingParent := false
	for _, o := range orphans {
		if o.PID == 400 {
			foundOrphaned = true
		}
		if o.PID == 600 {
			foundMissingParent = true
		}
	}

	if !foundOrphaned {
		t.Error("expected to find orphaned-process (PID 400)")
	}
	if !foundMissingParent {
		t.Error("expected to find orphan-missing-parent (PID 600)")
	}
}

// TestOrphanDetectionMCPOnly verifies MCP-only filtering works.
func TestOrphanDetectionMCPOnly(t *testing.T) {
	config := ResourceTrackerConfig{
		Enabled:            true,
		MemoryThresholdMB:  100,
		LogOrphanedMCPOnly: true, // Only log MCP orphans
	}
	tracker := NewResourceTracker(config, slog.Default())

	tracker.beforeSnapshot = &ProcessSnapshot{
		Processes:    []ProcessInfo{},
		ProcessCount: 0,
	}

	tracker.afterSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			{PID: 100, PPID: 1, Command: "random-process", IsMCP: false},
			{PID: 200, PPID: 1, Command: "chromium --headless", IsMCP: true},
			{PID: 300, PPID: 1, Command: "playwright-server", IsMCP: true},
		},
		ProcessCount: 3,
	}

	orphans := tracker.DetectOrphans()

	// Should only find MCP orphans (chromium and playwright)
	if len(orphans) != 2 {
		t.Errorf("expected 2 MCP orphans, got %d", len(orphans))
	}

	for _, o := range orphans {
		if !o.IsMCP {
			t.Errorf("expected only MCP orphans, got non-MCP: %s", o.Command)
		}
	}
}

// TestMemoryTracking verifies memory delta calculation.
func TestMemoryTracking(t *testing.T) {
	config := ResourceTrackerConfig{
		Enabled:           true,
		MemoryThresholdMB: 100,
	}

	// Use a buffer to capture log output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	tracker := NewResourceTracker(config, logger)

	// Set up snapshots with known memory values
	tracker.beforeSnapshot = &ProcessSnapshot{
		TotalMemoryMB: 1000.0,
		ProcessCount:  10,
	}

	tracker.afterSnapshot = &ProcessSnapshot{
		TotalMemoryMB: 1050.0, // 50MB growth - below threshold
		ProcessCount:  12,
	}

	// Check memory growth - should not warn (below threshold)
	delta := tracker.CheckMemoryGrowth()
	if delta != 50.0 {
		t.Errorf("expected delta of 50.0, got %f", delta)
	}

	// Should not have logged a warning
	if strings.Contains(logBuf.String(), "memory growth exceeded") {
		t.Error("should not warn when below threshold")
	}

	// Now test with memory above threshold
	logBuf.Reset()
	tracker.afterSnapshot = &ProcessSnapshot{
		TotalMemoryMB: 1150.0, // 150MB growth - above threshold
		ProcessCount:  15,
	}

	delta = tracker.CheckMemoryGrowth()
	if delta != 150.0 {
		t.Errorf("expected delta of 150.0, got %f", delta)
	}

	// Should have logged a warning
	if !strings.Contains(logBuf.String(), "memory growth exceeded threshold") {
		t.Error("should warn when above threshold")
	}
}

// TestResourceTrackerConfig verifies config enables/disables tracking.
func TestResourceTrackerConfig(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		expectCapture bool
	}{
		{
			name:          "enabled captures snapshots",
			enabled:       true,
			expectCapture: true,
		},
		{
			name:          "disabled skips snapshots",
			enabled:       false,
			expectCapture: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ResourceTrackerConfig{
				Enabled:           tt.enabled,
				MemoryThresholdMB: 100,
			}
			tracker := NewResourceTracker(config, slog.Default())

			// Take snapshot
			err := tracker.SnapshotBefore()
			if err != nil {
				t.Fatalf("SnapshotBefore failed: %v", err)
			}

			// Check if snapshot was captured
			snapshot := tracker.GetBeforeSnapshot()
			if tt.expectCapture && snapshot == nil {
				t.Error("expected snapshot to be captured when enabled")
			}
			if !tt.expectCapture && snapshot != nil {
				t.Error("expected no snapshot when disabled")
			}
		})
	}
}

// TestReset verifies Reset clears snapshots.
func TestReset(t *testing.T) {
	config := ResourceTrackerConfig{
		Enabled:           true,
		MemoryThresholdMB: 100,
	}
	tracker := NewResourceTracker(config, slog.Default())

	// Take snapshots
	if err := tracker.SnapshotBefore(); err != nil {
		t.Fatalf("SnapshotBefore failed: %v", err)
	}
	if err := tracker.SnapshotAfter(); err != nil {
		t.Fatalf("SnapshotAfter failed: %v", err)
	}

	// Verify snapshots exist
	if tracker.GetBeforeSnapshot() == nil || tracker.GetAfterSnapshot() == nil {
		t.Fatal("expected both snapshots to exist")
	}

	// Reset
	tracker.Reset()

	// Verify snapshots are cleared
	if tracker.GetBeforeSnapshot() != nil {
		t.Error("expected before snapshot to be cleared")
	}
	if tracker.GetAfterSnapshot() != nil {
		t.Error("expected after snapshot to be cleared")
	}
}

// TestIsMCPProcess verifies MCP process detection.
func TestIsMCPProcess(t *testing.T) {
	tests := []struct {
		command string
		isMCP   bool
	}{
		// MCP processes
		{"playwright-server", true},
		{"chromium --headless", true},
		{"chromium-browser", true},
		{"/usr/bin/chrome", true},
		{"google-chrome-stable", true},
		{"firefox --headless", true},
		{"firefox-esr", true},
		{"webkit2gtk", true},
		{"puppeteer-browser", true},
		{"selenium-server", true},
		{"CHROMIUM", true}, // Case insensitive
		{"PlayWright", true},

		// Non-MCP processes
		{"bash", false},
		{"vim", false},
		{"node server.js", false},
		{"python script.py", false},
		{"go test", false},
		{"orc run TASK-001", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := IsMCPProcess(tt.command)
			if result != tt.isMCP {
				t.Errorf("IsMCPProcess(%q) = %v, want %v", tt.command, result, tt.isMCP)
			}
		})
	}
}

// TestDetectOrphansNoSnapshots verifies detection handles missing snapshots.
func TestDetectOrphansNoSnapshots(t *testing.T) {
	config := ResourceTrackerConfig{
		Enabled:           true,
		MemoryThresholdMB: 100,
	}
	tracker := NewResourceTracker(config, slog.Default())

	// No snapshots taken - should return nil
	orphans := tracker.DetectOrphans()
	if orphans != nil {
		t.Error("expected nil orphans when no snapshots")
	}

	// Only before snapshot
	tracker.beforeSnapshot = &ProcessSnapshot{Processes: []ProcessInfo{}}
	orphans = tracker.DetectOrphans()
	if orphans != nil {
		t.Error("expected nil orphans when only before snapshot")
	}
}

// TestCheckMemoryGrowthNoSnapshots verifies memory check handles missing snapshots.
func TestCheckMemoryGrowthNoSnapshots(t *testing.T) {
	config := ResourceTrackerConfig{
		Enabled:           true,
		MemoryThresholdMB: 100,
	}
	tracker := NewResourceTracker(config, slog.Default())

	// No snapshots - should return 0
	delta := tracker.CheckMemoryGrowth()
	if delta != 0 {
		t.Errorf("expected 0 delta with no snapshots, got %f", delta)
	}
}
