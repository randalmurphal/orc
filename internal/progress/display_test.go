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

	d.TaskComplete(5000, 10*time.Minute)
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
	d.TaskComplete(5000, 10*time.Minute)
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
