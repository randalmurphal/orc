package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-6: orc-tdd-discipline.sh blocks non-test file writes, allows test files.
// Tests for SC-8: orc-verify-completion.sh blocks first stop, allows second.

// --- SC-6: TDD Discipline Hook Script Behavior ---

func TestTDDHookScript_Behavior(t *testing.T) {
	// Read the actual hook script from embedded templates
	hookContent, err := templates.Hooks.ReadFile("hooks/orc-tdd-discipline.sh")
	require.NoError(t, err, "hook script should be readable from embedded templates")

	// Write to temp dir for execution
	hookPath := filepath.Join(t.TempDir(), "orc-tdd-discipline.sh")
	require.NoError(t, os.WriteFile(hookPath, hookContent, 0755))

	t.Run("blocks non-test file writes", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": "/some/path/main.go"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		output, err := cmd.CombinedOutput()

		// Should exit 2 (block)
		assert.Error(t, err, "should block non-test file writes")
		if exitErr, ok := err.(*exec.ExitError); ok {
			assert.Equal(t, 2, exitErr.ExitCode(), "exit code should be 2 (block)")
		}
		assert.Contains(t, string(output), "TDD discipline",
			"output should mention TDD discipline")
	})

	t.Run("allows test file writes - Go", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": "/some/path/main_test.go"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should allow Go test file writes (exit 0)")
	})

	t.Run("allows test file writes - TypeScript", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": "/web/src/Component.test.tsx"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should allow TypeScript test file writes (exit 0)")
	})

	t.Run("allows test file writes - JS spec", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": "/src/utils.spec.js"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should allow JS spec file writes (exit 0)")
	})

	t.Run("allows non-file tools", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "echo hello"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should allow non-file tools (exit 0)")
	})

	t.Run("allows Edit tool on test files", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Edit",
			"tool_input": map[string]any{"file_path": "/tests/auth_test.py"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should allow Python test file edits (*_test.py → exit 0)")
	})

	t.Run("blocks Edit on non-test file", func(t *testing.T) {
		input := map[string]any{
			"tool_name":  "Edit",
			"tool_input": map[string]any{"file_path": "/some/path/handler.go"},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		_, err := cmd.CombinedOutput()

		assert.Error(t, err, "should block non-test file edits")
		if exitErr, ok := err.(*exec.ExitError); ok {
			assert.Equal(t, 2, exitErr.ExitCode(), "exit code should be 2 (block)")
		}
	})

	t.Run("exits 0 when no file_path in input", func(t *testing.T) {
		// Malformed input: Write tool with no file_path
		input := map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{},
		}
		inputJSON, _ := json.Marshal(input)

		cmd := exec.Command("bash", hookPath)
		cmd.Stdin = strings.NewReader(string(inputJSON))
		err := cmd.Run()

		assert.NoError(t, err, "should exit 0 (fail-open) when no file_path")
	})

	t.Run("no sqlite3 dependency", func(t *testing.T) {
		// The hook script should NOT reference sqlite3 at all
		assert.NotContains(t, string(hookContent), "sqlite3",
			"TDD hook should not depend on sqlite3")
	})
}

// --- SC-8: Verify Completion Hook Script Behavior ---

func TestVerifyCompletionHook_Behavior(t *testing.T) {
	hookContent, err := templates.Hooks.ReadFile("hooks/orc-verify-completion.sh")
	require.NoError(t, err, "hook script should be readable from embedded templates")

	hookPath := filepath.Join(t.TempDir(), "orc-verify-completion.sh")
	require.NoError(t, os.WriteFile(hookPath, hookContent, 0755))

	// Use a unique task ID to avoid interference between subtests
	taskID := fmt.Sprintf("TASK-TEST-%d", os.Getpid())
	markerFile := fmt.Sprintf("/tmp/orc-verify-completion-%s", taskID)

	// Ensure clean state
	_ = os.Remove(markerFile)
	t.Cleanup(func() { _ = os.Remove(markerFile) })

	t.Run("first stop attempt is blocked", func(t *testing.T) {
		_ = os.Remove(markerFile) // ensure clean

		cmd := exec.Command("bash", hookPath)
		cmd.Env = append(os.Environ(), fmt.Sprintf("ORC_TASK_ID=%s", taskID))
		output, err := cmd.CombinedOutput()

		// Should exit 2 (block)
		assert.Error(t, err, "first stop attempt should be blocked")
		if exitErr, ok := err.(*exec.ExitError); ok {
			assert.Equal(t, 2, exitErr.ExitCode(), "exit code should be 2 (block)")
		}
		assert.Contains(t, string(output), "BEFORE COMPLETING",
			"should contain re-verification prompt")

		// Marker file should have been created
		_, err = os.Stat(markerFile)
		assert.NoError(t, err, "marker file should exist after first stop attempt")
	})

	t.Run("second stop attempt is allowed", func(t *testing.T) {
		// Marker file should exist from the first test
		// If not, create it (in case tests run in different order)
		if _, err := os.Stat(markerFile); os.IsNotExist(err) {
			require.NoError(t, os.WriteFile(markerFile, []byte(""), 0644))
		}

		cmd := exec.Command("bash", hookPath)
		cmd.Env = append(os.Environ(), fmt.Sprintf("ORC_TASK_ID=%s", taskID))
		err := cmd.Run()

		// Should exit 0 (allow)
		assert.NoError(t, err, "second stop attempt should be allowed (exit 0)")

		// Marker file should have been removed
		_, err = os.Stat(markerFile)
		assert.True(t, os.IsNotExist(err), "marker file should be removed after second stop")
	})
}

func TestVerifyCompletionHook_StaleMarker(t *testing.T) {
	hookContent, err := templates.Hooks.ReadFile("hooks/orc-verify-completion.sh")
	require.NoError(t, err)

	hookPath := filepath.Join(t.TempDir(), "orc-verify-completion.sh")
	require.NoError(t, os.WriteFile(hookPath, hookContent, 0755))

	// Use unique task ID
	taskID := fmt.Sprintf("TASK-STALE-%d", os.Getpid())
	markerFile := fmt.Sprintf("/tmp/orc-verify-completion-%s", taskID)

	// Create a "stale" marker from a previous run
	require.NoError(t, os.WriteFile(markerFile, []byte("stale"), 0644))
	t.Cleanup(func() { _ = os.Remove(markerFile) })

	// With stale marker, first invocation should allow (marker exists → exit 0)
	cmd := exec.Command("bash", hookPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ORC_TASK_ID=%s", taskID))
	err = cmd.Run()
	assert.NoError(t, err, "stale marker should be treated as 'second attempt' → exit 0")

	// Marker should be removed
	_, err = os.Stat(markerFile)
	assert.True(t, os.IsNotExist(err), "stale marker should be removed")

	// Now next stop should block (fresh start)
	cmd = exec.Command("bash", hookPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ORC_TASK_ID=%s", taskID))
	_, err = cmd.CombinedOutput()
	assert.Error(t, err, "after stale marker cleanup, first stop should block")
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 2, exitErr.ExitCode())
	}

	// Clean up
	_ = os.Remove(markerFile)
}
