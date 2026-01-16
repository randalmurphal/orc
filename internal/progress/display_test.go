package progress

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	d := New("TASK-001", false)

	if d.taskID != "TASK-001" {
		t.Errorf("taskID = %s, want TASK-001", d.taskID)
	}

	if d.quiet {
		t.Error("quiet should be false")
	}

	if d.startTime.IsZero() {
		t.Error("startTime is zero")
	}
}

func TestNewQuiet(t *testing.T) {
	d := New("TASK-001", true)

	if !d.quiet {
		t.Error("quiet should be true")
	}
}

func TestPhaseStart(t *testing.T) {
	d := New("TASK-001", true) // quiet mode to suppress output

	d.PhaseStart("implement", 30)

	if d.phase != "implement" {
		t.Errorf("phase = %s, want implement", d.phase)
	}

	if d.maxIter != 30 {
		t.Errorf("maxIter = %d, want 30", d.maxIter)
	}

	if d.iteration != 0 {
		t.Errorf("iteration = %d, want 0", d.iteration)
	}
}

func TestUpdate(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.PhaseStart("implement", 30)
	d.Update(5, 1000)

	if d.iteration != 5 {
		t.Errorf("iteration = %d, want 5", d.iteration)
	}

	if d.tokens != 1000 {
		t.Errorf("tokens = %d, want 1000", d.tokens)
	}
}

func TestPhaseComplete(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.PhaseStart("implement", 30)

	// Should not panic in quiet mode
	d.PhaseComplete("implement", "abc1234567890")
}

func TestPhaseFailed(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	// Should not panic in quiet mode
	d.PhaseFailed("implement", testError("test error"))
}

func TestGatePending(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	// Test different gate types
	d.GatePending("spec", "human")
	d.GatePending("implement", "ai")
	d.GatePending("test", "auto")
}

func TestGateApproved(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.GateApproved("spec")
}

func TestGateRejected(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.GateRejected("spec", "needs more detail")
}

func TestTaskComplete(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.TaskComplete(5000, 10*time.Minute, nil)
}

func TestTaskFailed(t *testing.T) {
	// TaskFailed always prints, even in quiet mode, because errors should
	// never be silently swallowed. This test verifies it doesn't panic.
	d := New("TASK-001", true) // quiet mode

	d.TaskFailed(testError("something went wrong"))
}

func TestTaskInterrupted(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.TaskInterrupted()
}

func TestInfoWarning(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.Info("info message")
	d.Warning("warning message")
}

func TestError(t *testing.T) {
	// Error is shown even in quiet mode, so just verify it doesn't panic
	d := New("TASK-001", true)
	d.Error("error message")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m0s"},
		{65 * time.Minute, "1h5m0s"},
		{2*time.Hour + 30*time.Minute + 45*time.Second, "2h30m45s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
		}
	}
}

type testError string

func (e testError) Error() string { return string(e) }

func TestPluralize(t *testing.T) {
	tests := []struct {
		n        int
		singular string
		plural   string
		expected string
	}{
		{0, "file", "files", "files"},
		{1, "file", "files", "file"},
		{2, "file", "files", "files"},
		{10, "change", "changes", "changes"},
		{1, "change", "changes", "change"},
	}

	for _, tt := range tests {
		result := pluralize(tt.n, tt.singular, tt.plural)
		if result != tt.expected {
			t.Errorf("pluralize(%d, %q, %q) = %s, want %s", tt.n, tt.singular, tt.plural, result, tt.expected)
		}
	}
}

// === Non-Quiet Mode Tests (for coverage of print statements) ===

func TestPhaseStart_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic and should execute the print paths
	d.PhaseStart("implement", 30)

	if d.phase != "implement" {
		t.Errorf("phase = %s, want implement", d.phase)
	}
}

func TestUpdate_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode
	d.PhaseStart("implement", 30)

	// Should not panic and should execute the print paths
	d.Update(5, 1000)

	if d.iteration != 5 {
		t.Errorf("iteration = %d, want 5", d.iteration)
	}
}

func TestPhaseComplete_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode
	d.PhaseStart("implement", 30)

	// Should not panic
	d.PhaseComplete("implement", "abc1234567890")
}

func TestPhaseComplete_ShortCommit(t *testing.T) {
	d := New("TASK-001", false)
	d.PhaseStart("test", 10)

	// Short commit (less than 7 chars)
	d.PhaseComplete("test", "abc")
}

func TestPhaseFailed_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.PhaseFailed("implement", testError("test error"))
}

func TestGatePending_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Test all gate types
	d.GatePending("spec", "human")
	d.GatePending("implement", "ai")
	d.GatePending("test", "auto")
	d.GatePending("validate", "unknown") // default icon
}

func TestGateApproved_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.GateApproved("spec")
}

func TestGateRejected_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.GateRejected("spec", "needs more detail")
}

func TestTaskComplete_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.TaskComplete(5000, 10*time.Minute, nil)
}

func TestTaskComplete_WithFileStats(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	stats := &FileChangeStats{
		FilesChanged: 5,
		Additions:    150,
		Deletions:    20,
	}
	// Should not panic and should display file stats
	d.TaskComplete(5000, 10*time.Minute, stats)
}

func TestTaskComplete_WithSingleFile(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	stats := &FileChangeStats{
		FilesChanged: 1,
		Additions:    10,
		Deletions:    5,
	}
	// Should not panic and should display "file" (singular)
	d.TaskComplete(5000, 10*time.Minute, stats)
}

func TestTaskComplete_WithZeroChanges(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	stats := &FileChangeStats{
		FilesChanged: 0,
		Additions:    0,
		Deletions:    0,
	}
	// Should not panic and should NOT display file stats (no changes)
	d.TaskComplete(5000, 10*time.Minute, stats)
}

func TestTaskFailed_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.TaskFailed(testError("something went wrong"))
}

func TestTaskInterrupted_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.TaskInterrupted()
}

func TestInfo_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.Info("info message")
}

func TestWarning_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic
	d.Warning("warning message")
}

// === Activity State Tests ===

func TestActivityState_String(t *testing.T) {
	tests := []struct {
		state    ActivityState
		expected string
	}{
		{ActivityIdle, "Idle"},
		{ActivityWaitingAPI, "Waiting for API"},
		{ActivityStreaming, "Receiving response"},
		{ActivityRunningTool, "Running tool"},
		{ActivityProcessing, "Processing"},
		{ActivitySpecAnalyzing, "Analyzing codebase"},
		{ActivitySpecWriting, "Writing specification"},
		{ActivityState("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("ActivityState.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSetActivity_Quiet(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	// Should not panic
	d.SetActivity(ActivityWaitingAPI)
	d.SetActivity(ActivityStreaming)
	d.SetActivity(ActivityRunningTool)
}

func TestSetActivity_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic and should print
	d.SetActivity(ActivityWaitingAPI)
	d.SetActivity(ActivityRunningTool)
}

func TestSetActivity_SpecPhase_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Should not panic and should print spec-specific messages
	d.SetActivity(ActivitySpecAnalyzing)
	d.SetActivity(ActivitySpecWriting)
}

func TestActivityState_IsSpecPhaseActivity(t *testing.T) {
	tests := []struct {
		state    ActivityState
		expected bool
	}{
		{ActivityIdle, false},
		{ActivityWaitingAPI, false},
		{ActivityStreaming, false},
		{ActivityRunningTool, false},
		{ActivityProcessing, false},
		{ActivitySpecAnalyzing, true},
		{ActivitySpecWriting, true},
		{ActivityState("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsSpecPhaseActivity(); got != tt.expected {
				t.Errorf("ActivityState.IsSpecPhaseActivity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHeartbeat_Quiet(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	// Should not print in quiet mode
	d.Heartbeat()
}

func TestHeartbeat_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	d.SetActivity(ActivityWaitingAPI)
	// Should print a dot
	d.Heartbeat()
}

func TestHeartbeat_SpecPhase_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// Spec phase activities should also emit heartbeat dots
	d.SetActivity(ActivitySpecAnalyzing)
	d.Heartbeat()

	d.SetActivity(ActivitySpecWriting)
	d.Heartbeat()
}

func TestHeartbeat_WithElapsedTime(t *testing.T) {
	d := New("TASK-001", false)

	d.SetActivity(ActivityWaitingAPI)
	// Manually set activity start to make elapsed > 2 minutes
	d.mu.Lock()
	d.activityStart = time.Now().Add(-3 * time.Minute)
	d.mu.Unlock()

	// Should print a dot with elapsed time
	d.Heartbeat()
}

func TestIdleWarning(t *testing.T) {
	d := New("TASK-001", false)

	// Should always print (even in quiet mode)
	d.IdleWarning(5 * time.Minute)
}

func TestTurnTimeout(t *testing.T) {
	d := New("TASK-001", false)

	// Should always print
	d.TurnTimeout(10 * time.Minute)
}

func TestActivityUpdate_Quiet(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	d.PhaseStart("implement", 10)
	d.SetActivity(ActivityStreaming)

	// Should not print
	d.ActivityUpdate()
}

func TestActivityUpdate_NonQuiet(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	d.PhaseStart("implement", 10)
	d.Update(3, 1000)
	d.SetActivity(ActivityStreaming)

	// Should print activity status
	d.ActivityUpdate()
}

func TestCancelled(t *testing.T) {
	d := New("TASK-001", false)

	// Should always print
	d.Cancelled()
}

// === TaskBlocked Tests ===

func TestTaskBlocked_Basic(t *testing.T) {
	d := New("TASK-001", false) // non-quiet mode

	// TaskBlocked always prints since it requires user action
	d.TaskBlocked(5000, 10*time.Minute, "sync conflict")
}

func TestTaskBlocked_Quiet(t *testing.T) {
	d := New("TASK-001", true) // quiet mode

	// TaskBlocked should still print even in quiet mode since it requires action
	d.TaskBlocked(5000, 10*time.Minute, "sync conflict")
}

func TestTaskBlockedWithContext_NoContext(t *testing.T) {
	d := New("TASK-001", false)

	// Should not panic with nil context
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", nil)
}

func TestTaskBlockedWithContext_WithWorktreePath(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath: ".orc/worktrees/orc-TASK-001",
	}

	// Should display worktree path
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_WithConflictFiles(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath:  ".orc/worktrees/orc-TASK-001",
		ConflictFiles: []string{"internal/foo.go", "internal/bar.go"},
	}

	// Should list conflicted files
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_RebaseStrategy(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath: ".orc/worktrees/orc-TASK-001",
		SyncStrategy: SyncStrategyRebase,
		TargetBranch: "main",
	}

	// Should show rebase instructions (git rebase, git rebase --continue)
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_MergeStrategy(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath: ".orc/worktrees/orc-TASK-001",
		SyncStrategy: SyncStrategyMerge,
		TargetBranch: "develop",
	}

	// Should show merge instructions (git merge, git commit)
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_DefaultTargetBranch(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath: ".orc/worktrees/orc-TASK-001",
		SyncStrategy: SyncStrategyRebase,
		TargetBranch: "", // Empty should default to "main"
	}

	// Should default to "main" for target branch in commands
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_EmptyWorktreePath(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath: "", // Empty worktree path
		SyncStrategy: SyncStrategyRebase,
	}

	// Should skip enhanced guidance when worktree path is empty
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

func TestTaskBlockedWithContext_FullContext(t *testing.T) {
	d := New("TASK-001", false)

	ctx := &BlockedContext{
		WorktreePath:  ".orc/worktrees/orc-TASK-001",
		ConflictFiles: []string{"file1.go", "file2.go", "file3.go"},
		SyncStrategy:  SyncStrategyRebase,
		TargetBranch:  "main",
	}

	// Full context with all fields populated
	d.TaskBlockedWithContext(5000, 10*time.Minute, "sync conflict", ctx)
}

// === SyncStrategy Tests ===

func TestSyncStrategy_Constants(t *testing.T) {
	// Verify constant values are as expected
	if SyncStrategyRebase != "rebase" {
		t.Errorf("SyncStrategyRebase = %q, want %q", SyncStrategyRebase, "rebase")
	}
	if SyncStrategyMerge != "merge" {
		t.Errorf("SyncStrategyMerge = %q, want %q", SyncStrategyMerge, "merge")
	}
}
