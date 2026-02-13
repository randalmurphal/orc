// Integration tests for TASK-020: CLI scratchpad command wiring.
//
// These tests verify that the scratchpad command is registered in rootCmd
// and that invoking it through the root command actually reaches the handler
// which calls the backend service.
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
//
// Wiring points verified:
//  1. rootCmd registers scratchpad command — "orc scratchpad" is recognized
//  2. Handler invocation — rootCmd dispatches to the handler which calls
//     backend.GetScratchpadEntries (not just registration)
//
// Deletion test: Remove addCmd(newScratchpadCmd(), ...) from root.go init() →
// rootCmd.Execute() returns "unknown command" error.
package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
)

// TestRootCmd_ScratchpadCommandRegistered verifies that the scratchpad
// command is reachable through the root command tree.
//
// This tests the REGISTRATION wiring in root.go init().
// If newScratchpadCmd() is not added to rootCmd, this test fails.
func TestRootCmd_ScratchpadCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "scratchpad" {
			found = true
			break
		}
	}

	if !found {
		t.Error("scratchpad command not registered in rootCmd — add it via addCmd() in root.go init()")
	}
}

// TestRootCmd_ScratchpadInvokesHandler verifies that running "orc scratchpad TASK-001"
// through the root command actually invokes the handler, which calls the backend.
//
// Deletion test: Remove the addCmd(newScratchpadCmd()) in root.go init() →
// rootCmd.Execute() returns "unknown command" error instead of scratchpad output.
func TestRootCmd_ScratchpadInvokesHandler(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)

	// Seed scratchpad entries so the handler has data to display
	entries := []storage.ScratchpadEntry{
		{TaskID: "TASK-099", PhaseID: "spec", Category: "decision", Content: "Integration test wiring proof", Attempt: 1},
	}
	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry: %v", err)
		}
	}
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Use rootCmd (the PRODUCTION entry point), not newScratchpadCmd() directly
	rootCmd.SetArgs([]string{"scratchpad", "TASK-099"})
	err := rootCmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	// Reset rootCmd args for other tests
	rootCmd.SetArgs(nil)

	if err != nil {
		t.Fatalf("rootCmd scratchpad execution failed: %v\nThis likely means the scratchpad command is not registered in rootCmd", err)
	}

	output := buf.String()

	// Verify the handler actually ran and produced output from the backend
	if !strings.Contains(output, "Integration test wiring proof") {
		// If we get "unknown command" or empty output, the command isn't wired
		t.Errorf("rootCmd scratchpad should produce output from backend\ngot: %s", output)
	}
}

// TestRootCmd_ScratchpadFilterByPhaseViaRoot verifies that --phase flag works
// when invoked through the root command (not just direct command construction).
func TestRootCmd_ScratchpadFilterByPhaseViaRoot(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)

	// Seed entries for two phases
	entries := []storage.ScratchpadEntry{
		{TaskID: "TASK-088", PhaseID: "spec", Category: "decision", Content: "Spec decision via root", Attempt: 1},
		{TaskID: "TASK-088", PhaseID: "implement", Category: "observation", Content: "Implement observation via root", Attempt: 1},
	}
	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry: %v", err)
		}
	}
	_ = backend.Close()

	// Use rootCmd to invoke with --phase filter
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"scratchpad", "TASK-088", "--phase", "implement"})
	err := rootCmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout
	rootCmd.SetArgs(nil)

	if err != nil {
		t.Fatalf("rootCmd scratchpad --phase failed: %v", err)
	}

	output := buf.String()

	// Should contain implement entries
	if !strings.Contains(output, "Implement observation via root") {
		t.Errorf("output should contain implement entry\ngot: %s", output)
	}

	// Should NOT contain spec entries
	if strings.Contains(output, "Spec decision via root") {
		t.Errorf("output should NOT contain spec entry when filtered to implement\ngot: %s", output)
	}
}
