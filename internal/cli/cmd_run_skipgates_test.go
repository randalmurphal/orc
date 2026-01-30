// Package cli implements the orc command-line interface.
//
// Tests for `orc run --skip-gates` flag.
// These tests use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
package cli

import (
	"testing"
)

// =============================================================================
// SC-6: `orc run --skip-gates` flag exists and parses
// =============================================================================

func TestRunCmd_SkipGatesFlag(t *testing.T) {
	cmd := newRunCmd()

	flag := cmd.Flag("skip-gates")
	if flag == nil {
		t.Fatal("missing --skip-gates flag on run command")
	}

	// Should be a boolean flag
	if flag.Value.Type() != "bool" {
		t.Errorf("--skip-gates should be bool, got %s", flag.Value.Type())
	}

	// Default should be false
	if flag.DefValue != "false" {
		t.Errorf("--skip-gates default = %q, want 'false'", flag.DefValue)
	}
}

// =============================================================================
// SC-7: --skip-gates works with --task (existing task resume)
// =============================================================================

func TestRunCmd_SkipGatesWithTask_FlagCoexists(t *testing.T) {
	cmd := newRunCmd()

	// Both flags should be parseable together
	cmd.SetArgs([]string{"--task", "TASK-001", "--skip-gates", "implement task"})

	// We only test that flag parsing doesn't error.
	// Actual execution requires a full backend which is tested in executor tests.
	if err := cmd.ParseFlags([]string{"--task", "TASK-001", "--skip-gates"}); err != nil {
		t.Fatalf("parsing --skip-gates with --task failed: %v", err)
	}

	val, err := cmd.Flags().GetBool("skip-gates")
	if err != nil {
		t.Fatalf("get skip-gates flag: %v", err)
	}
	if !val {
		t.Error("--skip-gates should be true after setting")
	}
}

// =============================================================================
// Edge case: --skip-gates combined with --force
// =============================================================================

func TestRunCmd_SkipGatesWithForce_FlagsCoexist(t *testing.T) {
	cmd := newRunCmd()

	if err := cmd.ParseFlags([]string{"--skip-gates", "--force"}); err != nil {
		t.Fatalf("parsing --skip-gates with --force failed: %v", err)
	}

	skipGates, _ := cmd.Flags().GetBool("skip-gates")
	force, _ := cmd.Flags().GetBool("force")

	if !skipGates {
		t.Error("--skip-gates should be true")
	}
	if !force {
		t.Error("--force should be true")
	}
}

// =============================================================================
// Edge case: --skip-gates combined with --profile strict
// =============================================================================

func TestRunCmd_SkipGatesWithStrictProfile_FlagsCoexist(t *testing.T) {
	cmd := newRunCmd()

	if err := cmd.ParseFlags([]string{"--skip-gates", "--profile", "strict"}); err != nil {
		t.Fatalf("parsing --skip-gates with --profile strict failed: %v", err)
	}

	skipGates, _ := cmd.Flags().GetBool("skip-gates")
	profile, _ := cmd.Flags().GetString("profile")

	if !skipGates {
		t.Error("--skip-gates should be true")
	}
	if profile != "strict" {
		t.Errorf("profile = %q, want 'strict'", profile)
	}
}
