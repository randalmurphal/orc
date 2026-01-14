package executor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

func TestNewCIMerger(t *testing.T) {
	cfg := config.Default()
	logger := slog.Default()
	workDir := "/tmp/test"
	taskDir := "/tmp/test/tasks/TASK-001"

	merger := NewCIMerger(cfg, logger, workDir, taskDir)

	if merger == nil {
		t.Fatal("NewCIMerger returned nil")
	}
	if merger.cfg != cfg {
		t.Error("cfg not set correctly")
	}
	if merger.logger != logger {
		t.Error("logger not set correctly")
	}
	if merger.workingDir != workDir {
		t.Errorf("workingDir = %q, want %q", merger.workingDir, workDir)
	}
	if merger.taskDir != taskDir {
		t.Errorf("taskDir = %q, want %q", merger.taskDir, taskDir)
	}
}

func TestNewCIMerger_NilLogger(t *testing.T) {
	cfg := config.Default()
	merger := NewCIMerger(cfg, nil, "/tmp", "/tmp")

	if merger.logger == nil {
		t.Error("logger should default to slog.Default()")
	}
}

func TestWaitForCIAndMerge_Disabled(t *testing.T) {
	cfg := config.Default()
	// Set profile to strict which doesn't support auto CI wait
	cfg.Profile = config.ProfileStrict
	cfg.Completion.WaitForCI = false

	merger := NewCIMerger(cfg, nil, "/tmp", "/tmp")

	tsk := &task.Task{
		ID: "TASK-001",
	}

	result, err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SkippedMerge {
		t.Error("expected SkippedMerge to be true when WaitForCI is disabled")
	}
}

func TestWaitForCIAndMerge_NoPRURL(t *testing.T) {
	cfg := config.Default()
	cfg.Profile = config.ProfileAuto
	cfg.Completion.WaitForCI = true

	merger := NewCIMerger(cfg, nil, "/tmp", "/tmp")

	// Task without PR info
	tsk := &task.Task{
		ID: "TASK-001",
	}

	_, err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err == nil {
		t.Fatal("expected error when PR URL is missing")
	}
	if err.Error() != "no PR URL to wait for" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCICheckStatus_Parse(t *testing.T) {
	tests := []struct {
		name       string
		passed     int
		failed     int
		pending    int
		wantPassed bool
		wantFailed bool
	}{
		{
			name:       "all passed",
			passed:     3,
			failed:     0,
			pending:    0,
			wantPassed: true,
			wantFailed: false,
		},
		{
			name:       "some failed",
			passed:     2,
			failed:     1,
			pending:    0,
			wantPassed: false,
			wantFailed: true,
		},
		{
			name:       "some pending",
			passed:     2,
			failed:     0,
			pending:    1,
			wantPassed: false,
			wantFailed: false,
		},
		{
			name:       "failed and pending",
			passed:     1,
			failed:     1,
			pending:    1,
			wantPassed: false,
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CICheckStatus{
				AllPassed:  tt.failed == 0 && tt.pending == 0,
				AnyFailed:  tt.failed > 0,
				AnyPending: tt.pending > 0,
			}

			if result.AllPassed != tt.wantPassed {
				t.Errorf("AllPassed = %v, want %v", result.AllPassed, tt.wantPassed)
			}
			if result.AnyFailed != tt.wantFailed {
				t.Errorf("AnyFailed = %v, want %v", result.AnyFailed, tt.wantFailed)
			}
		})
	}
}

func TestCIMergeResult_Fields(t *testing.T) {
	result := &CIMergeResult{
		Pushed:       true,
		CIPassed:     true,
		Merged:       true,
		MergeCommit:  "abc123",
		CIDetails:    "3 passed, 0 failed, 0 pending",
		TimedOut:     false,
		SkippedMerge: false,
	}

	if !result.Pushed {
		t.Error("Pushed should be true")
	}
	if !result.CIPassed {
		t.Error("CIPassed should be true")
	}
	if !result.Merged {
		t.Error("Merged should be true")
	}
	if result.MergeCommit != "abc123" {
		t.Errorf("MergeCommit = %q, want %q", result.MergeCommit, "abc123")
	}
}

func TestConfigHelpers(t *testing.T) {
	t.Run("ShouldWaitForCI", func(t *testing.T) {
		cfg := config.Default()
		cfg.Profile = config.ProfileAuto
		cfg.Completion.WaitForCI = true

		if !cfg.ShouldWaitForCI() {
			t.Error("ShouldWaitForCI should be true for auto profile with WaitForCI=true")
		}

		cfg.Profile = config.ProfileStrict
		if cfg.ShouldWaitForCI() {
			t.Error("ShouldWaitForCI should be false for strict profile")
		}
	})

	t.Run("ShouldMergeOnCIPass", func(t *testing.T) {
		cfg := config.Default()
		cfg.Profile = config.ProfileAuto
		cfg.Completion.WaitForCI = true
		cfg.Completion.MergeOnCIPass = true

		if !cfg.ShouldMergeOnCIPass() {
			t.Error("ShouldMergeOnCIPass should be true when both WaitForCI and MergeOnCIPass are enabled")
		}

		cfg.Completion.WaitForCI = false
		if cfg.ShouldMergeOnCIPass() {
			t.Error("ShouldMergeOnCIPass should be false when WaitForCI is disabled")
		}
	})

	t.Run("GetCITimeout", func(t *testing.T) {
		cfg := config.Default()

		// Default timeout
		timeout := cfg.GetCITimeout()
		if timeout != 10*time.Minute {
			t.Errorf("default timeout = %v, want 10m", timeout)
		}

		// Custom timeout
		cfg.Completion.CITimeout = 5 * time.Minute
		timeout = cfg.GetCITimeout()
		if timeout != 5*time.Minute {
			t.Errorf("custom timeout = %v, want 5m", timeout)
		}
	})
}
