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
	t.Parallel()
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
	t.Parallel()
	// Create a tracker
	config := ResourceTrackerConfig{
		Enabled:            true,
		MemoryThresholdMB:  100,
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
			{PID: 400, PPID: 1, Command: "orphaned-process", IsMCP: false},        // New, PPID=1 = orphan
			{PID: 500, PPID: 200, Command: "new-child-process", IsMCP: false},     // New, has parent = not orphan
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

// TestMemoryTracking verifies memory delta calculation.
func TestMemoryTracking(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestIsOrcRelatedProcess verifies orc-related process detection.
func TestIsOrcRelatedProcess(t *testing.T) {
	t.Parallel()
	tests := []struct {
		command      string
		isOrcRelated bool
	}{
		// Browser processes (also orc-related)
		{"playwright-server", true},
		{"chromium --headless", true},
		{"/usr/bin/chrome", true},
		{"firefox --headless", true},

		// Claude Code and Node.js processes
		{"claude --task=TASK-001", true},
		{"~/.claude/local/claude --dangerously-skip-permissions", true},
		{"node server.js", true},
		{"node --version", true},
		{"/usr/bin/node", true},
		{"npx playwright test", true},
		{"npm install", true},

		// MCP server processes
		{"mcp-server-playwright", true},
		{"playwright-mcp-server", true},
		{"/path/to/mcp", true},

		// System processes (should NOT match)
		{"systemd-timedated", false},
		{"/usr/lib/systemd/systemd-timedated", false},
		{"/usr/lib/snapper/systemd-helper --cleanup", false},
		{"/usr/sbin/snapperd", false},
		{"snapper", false},
		{"bash", false},
		{"vim", false},
		{"python script.py", false},
		{"go test", false},
		{"init", false},
		{"/usr/bin/dbus-daemon", false},
		{"kworker/0:0", false},
		{"[kthreadd]", false},

		// Edge cases - these don't contain "node" as a standalone word
		{"nodemon", false}, // Not "node" - nodemon is a different program
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := IsOrcRelatedProcess(tt.command)
			if result != tt.isOrcRelated {
				t.Errorf("IsOrcRelatedProcess(%q) = %v, want %v", tt.command, result, tt.isOrcRelated)
			}
		})
	}
}

// TestOrphanDetectionFilterSystemProcesses verifies system process filtering.
func TestOrphanDetectionFilterSystemProcesses(t *testing.T) {
	t.Parallel()
	config := ResourceTrackerConfig{
		Enabled:               true,
		MemoryThresholdMB:     100,
		FilterSystemProcesses: true, // Enable filtering (the new default)
	}
	tracker := NewResourceTracker(config, slog.Default())

	tracker.beforeSnapshot = &ProcessSnapshot{
		Processes:    []ProcessInfo{},
		ProcessCount: 0,
	}

	tracker.afterSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			// System processes - should be filtered out
			{PID: 100, PPID: 1, Command: "systemd-timedated", IsMCP: false, IsOrcRelated: false},
			{PID: 101, PPID: 1, Command: "/usr/lib/snapper/systemd-helper --cleanup", IsMCP: false, IsOrcRelated: false},
			{PID: 102, PPID: 1, Command: "/usr/sbin/snapperd", IsMCP: false, IsOrcRelated: false},
			// Orc-related processes - should be detected
			{PID: 200, PPID: 1, Command: "chromium --headless", IsMCP: true, IsOrcRelated: true},
			{PID: 201, PPID: 1, Command: "node playwright-server.js", IsMCP: false, IsOrcRelated: true},
			{PID: 202, PPID: 1, Command: "claude --task=TASK-001", IsMCP: false, IsOrcRelated: true},
		},
		ProcessCount: 6,
	}

	orphans := tracker.DetectOrphans()

	// Should only find orc-related orphans (chromium, node, claude), NOT system processes
	if len(orphans) != 3 {
		t.Errorf("expected 3 orc-related orphans, got %d", len(orphans))
		for _, o := range orphans {
			t.Logf("  orphan: PID=%d Command=%s IsOrcRelated=%v", o.PID, o.Command, o.IsOrcRelated)
		}
	}

	// Verify all orphans are orc-related
	for _, o := range orphans {
		if !o.IsOrcRelated {
			t.Errorf("expected only orc-related orphans, got: %s", o.Command)
		}
	}

	// Verify system processes were NOT flagged
	for _, o := range orphans {
		if strings.Contains(o.Command, "systemd") || strings.Contains(o.Command, "snapper") {
			t.Errorf("system process should not be flagged as orphan: %s", o.Command)
		}
	}
}

// TestOrphanDetectionFilterSystemProcessesDisabled verifies original behavior when filtering is disabled.
func TestOrphanDetectionFilterSystemProcessesDisabled(t *testing.T) {
	t.Parallel()
	config := ResourceTrackerConfig{
		Enabled:               true,
		MemoryThresholdMB:     100,
		FilterSystemProcesses: false, // Disabled - original behavior
	}
	tracker := NewResourceTracker(config, slog.Default())

	tracker.beforeSnapshot = &ProcessSnapshot{
		Processes:    []ProcessInfo{},
		ProcessCount: 0,
	}

	tracker.afterSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			// System processes
			{PID: 100, PPID: 1, Command: "systemd-timedated", IsMCP: false, IsOrcRelated: false},
			{PID: 101, PPID: 1, Command: "/usr/lib/snapper/systemd-helper", IsMCP: false, IsOrcRelated: false},
			// Orc-related processes
			{PID: 200, PPID: 1, Command: "chromium --headless", IsMCP: true, IsOrcRelated: true},
		},
		ProcessCount: 3,
	}

	orphans := tracker.DetectOrphans()

	// Should find ALL orphans since filtering is disabled
	if len(orphans) != 3 {
		t.Errorf("expected 3 orphans (all processes), got %d", len(orphans))
	}
}

// TestDetectOrphansNoSnapshots verifies detection handles missing snapshots.
func TestDetectOrphansNoSnapshots(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestResourceTrackingDuringTask verifies the full resource tracking lifecycle.
// This is an integration test that simulates what happens during task execution.
func TestResourceTrackingDuringTask(t *testing.T) {
	t.Parallel()
	// Create a buffer to capture log output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	config := ResourceTrackerConfig{
		Enabled:            true,
		MemoryThresholdMB:  100,
			}
	tracker := NewResourceTracker(config, logger)

	// Simulate: take snapshot before task execution (like ExecuteTask does)
	err := tracker.SnapshotBefore()
	if err != nil {
		t.Fatalf("SnapshotBefore failed: %v", err)
	}

	// Verify "before" log was emitted
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "resource snapshot taken (before)") {
		t.Error("expected 'resource snapshot taken (before)' log message")
	}
	if !strings.Contains(logOutput, "processes=") {
		t.Error("expected processes count in log")
	}
	if !strings.Contains(logOutput, "memory_mb=") {
		t.Error("expected memory_mb in log")
	}

	// Simulate: task runs (in real execution, processes may spawn and die)
	// For this test, we just wait a tiny bit to ensure different timestamps
	// In a real scenario, MCP servers/browsers might spawn here

	// Simulate: take snapshot after task execution (like runResourceAnalysis does)
	err = tracker.SnapshotAfter()
	if err != nil {
		t.Fatalf("SnapshotAfter failed: %v", err)
	}

	// Verify "after" log was emitted
	logOutput = logBuf.String()
	if !strings.Contains(logOutput, "resource snapshot taken (after)") {
		t.Error("expected 'resource snapshot taken (after)' log message")
	}

	// Verify both snapshots have data
	beforeSnap := tracker.GetBeforeSnapshot()
	afterSnap := tracker.GetAfterSnapshot()
	if beforeSnap == nil || afterSnap == nil {
		t.Fatal("expected both snapshots to be captured")
	}

	// Verify snapshots contain process data
	if beforeSnap.ProcessCount == 0 {
		t.Error("expected before snapshot to have processes")
	}
	if afterSnap.ProcessCount == 0 {
		t.Error("expected after snapshot to have processes")
	}

	// Detect orphans (should be empty or few in a clean test environment)
	orphans := tracker.DetectOrphans()
	// We don't assert orphan count since it depends on system state
	// But we verify the detection ran without error
	t.Logf("Detected %d orphans during test", len(orphans))

	// Check memory growth (should complete without error)
	delta := tracker.CheckMemoryGrowth()
	t.Logf("Memory delta: %.1f MB", delta)

	// Reset for next task
	tracker.Reset()
	if tracker.GetBeforeSnapshot() != nil || tracker.GetAfterSnapshot() != nil {
		t.Error("expected snapshots to be cleared after Reset")
	}
}

// TestResourceTrackingLifecycleWithMockOrphans tests the full lifecycle with injected orphans.
func TestResourceTrackingLifecycleWithMockOrphans(t *testing.T) {
	t.Parallel()
	// Create a buffer to capture log output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	config := ResourceTrackerConfig{
		Enabled:            true,
		MemoryThresholdMB:  50, // Low threshold to trigger warning
			}
	tracker := NewResourceTracker(config, logger)

	// Set up mock "before" state - simulates system before task
	tracker.beforeSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			{PID: 1, PPID: 0, Command: "init", MemoryMB: 10},
			{PID: 100, PPID: 1, Command: "orc", MemoryMB: 50},
		},
		TotalMemoryMB: 1000,
		ProcessCount:  2,
	}

	// Set up mock "after" state - simulates system after task with orphans
	tracker.afterSnapshot = &ProcessSnapshot{
		Processes: []ProcessInfo{
			{PID: 1, PPID: 0, Command: "init", MemoryMB: 10},
			{PID: 100, PPID: 1, Command: "orc", MemoryMB: 50},
			// New orphaned MCP processes (parent is init = reparented)
			{PID: 200, PPID: 1, Command: "chromium --headless", MemoryMB: 200, IsMCP: true},
			{PID: 201, PPID: 1, Command: "playwright-mcp-server", MemoryMB: 100, IsMCP: true},
			// Non-MCP orphan
			{PID: 300, PPID: 1, Command: "zombie-child", MemoryMB: 5, IsMCP: false},
		},
		TotalMemoryMB: 1100, // 100MB growth
		ProcessCount:  5,
	}

	// Detect orphans
	orphans := tracker.DetectOrphans()

	// Verify we found the orphans (3 new processes with PPID=1)
	if len(orphans) != 3 {
		t.Errorf("expected 3 orphans, got %d", len(orphans))
	}

	// Verify MCP processes are flagged
	mcpCount := 0
	for _, o := range orphans {
		if o.IsMCP {
			mcpCount++
		}
	}
	if mcpCount != 2 {
		t.Errorf("expected 2 MCP orphans, got %d", mcpCount)
	}

	// Verify warning was logged for orphans
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "orphaned processes detected") {
		t.Error("expected orphan warning log")
	}
	if !strings.Contains(logOutput, "[MCP]") {
		t.Error("expected MCP tag in orphan log")
	}

	// Check memory growth - should trigger warning (100MB > 50MB threshold)
	delta := tracker.CheckMemoryGrowth()
	if delta != 100.0 {
		t.Errorf("expected 100MB delta, got %.1f", delta)
	}

	// Verify memory warning was logged (re-read buffer after CheckMemoryGrowth)
	logOutput = logBuf.String()
	if !strings.Contains(logOutput, "memory growth exceeded threshold") {
		t.Error("expected memory growth warning log")
	}
}
