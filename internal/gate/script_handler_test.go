package gate

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScriptHandler_ExitZero_NoOverride(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/approve.sh", "#!/bin/sh\ncat > /dev/null\nexit 0\n")

	h := NewScriptHandler(slog.Default())
	result, err := h.Run(context.Background(), scriptPath, `{"decision":"approved"}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Override {
		t.Error("expected Override=false for exit 0")
	}
}

func TestScriptHandler_ExitNonZero_OverridesDecision(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/reject.sh", "#!/bin/sh\ncat > /dev/null\nexit 1\n")

	h := NewScriptHandler(slog.Default())
	result, err := h.Run(context.Background(), scriptPath, `{"decision":"approved"}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
	if !result.Override {
		t.Error("expected Override=true for non-zero exit")
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason for override")
	}
}

func TestScriptHandler_ReceivesExactJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	outputFile := filepath.Join(tmpDir, "captured_stdin.txt")

	// Script writes stdin to a file so we can verify exact content
	scriptPath := writeScript(t, tmpDir, ".orc/scripts/capture.sh",
		"#!/bin/sh\ncat > \""+outputFile+"\"\nexit 0\n")

	inputJSON := `{"decision":"approved","reason":"all criteria met","score":95}`
	h := NewScriptHandler(slog.Default())
	_, err := h.Run(context.Background(), scriptPath, inputJSON, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	captured, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read captured stdin: %v", err)
	}

	if string(captured) != inputJSON {
		t.Errorf("expected stdin %q, got %q", inputJSON, string(captured))
	}
}

func TestScriptHandler_CapturesStdout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/verbose.sh",
		"#!/bin/sh\ncat > /dev/null\necho \"script output line 1\"\necho \"script output line 2\"\nexit 0\n")

	h := NewScriptHandler(slog.Default())
	result, err := h.Run(context.Background(), scriptPath, `{}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stdout, "script output line 1") {
		t.Errorf("expected stdout to contain 'script output line 1', got %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "script output line 2") {
		t.Errorf("expected stdout to contain 'script output line 2', got %q", result.Stdout)
	}
}

func TestScriptHandler_CapturesStderr(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/errors.sh",
		"#!/bin/sh\ncat > /dev/null\necho \"warning: something happened\" >&2\nexit 1\n")

	h := NewScriptHandler(slog.Default())
	result, err := h.Run(context.Background(), scriptPath, `{}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stderr, "warning: something happened") {
		t.Errorf("expected stderr to contain warning, got %q", result.Stderr)
	}
}

func TestScriptHandler_ExitCode2(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/exit2.sh",
		"#!/bin/sh\ncat > /dev/null\nexit 2\n")

	h := NewScriptHandler(slog.Default())
	result, err := h.Run(context.Background(), scriptPath, `{}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 2 {
		t.Errorf("expected exit code 2, got %d", result.ExitCode)
	}
	if !result.Override {
		t.Error("expected Override=true for exit code 2")
	}
}

// --- SC-2: Path validation security ---

func TestValidateScriptPath_RelativeWithinOrc(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)
	writeScript(t, tmpDir, ".orc/scripts/validate.sh", "#!/bin/sh\nexit 0\n")

	resolved, err := ValidateScriptPath("scripts/validate.sh", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, ".orc", "scripts", "validate.sh")
	if resolved != expected {
		t.Errorf("expected resolved path %q, got %q", expected, resolved)
	}
}

func TestValidateScriptPath_TraversalOutsideOrc(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	_, err := ValidateScriptPath("../../../etc/passwd", tmpDir)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "traversal") && !strings.Contains(err.Error(), "outside") {
		t.Errorf("expected error about path traversal or outside, got: %v", err)
	}
}

func TestValidateScriptPath_TraversalWithDotDot(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	_, err := ValidateScriptPath("scripts/../../../etc/passwd", tmpDir)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestValidateScriptPath_AbsolutePathAccepted(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	absScript := filepath.Join(tmpDir, "custom_script.sh")
	if err := os.WriteFile(absScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	resolved, err := ValidateScriptPath(absScript, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved != absScript {
		t.Errorf("expected absolute path %q returned as-is, got %q", absScript, resolved)
	}
}

func TestValidateScriptPath_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := ValidateScriptPath("", "/some/project")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

// --- SC-3: Timeout enforcement ---

func TestScriptHandler_TimeoutExceeded(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/slow.sh",
		"#!/bin/sh\ncat > /dev/null\nsleep 30\nexit 0\n")

	h := NewScriptHandler(slog.Default(), WithScriptTimeout(100*time.Millisecond))
	_, err := h.Run(context.Background(), scriptPath, `{}`, tmpDir)
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
	if !strings.Contains(err.Error(), "deadline") &&
		!strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "killed") &&
		!strings.Contains(err.Error(), "signal") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func TestScriptHandler_CustomTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/fast.sh",
		"#!/bin/sh\ncat > /dev/null\nexit 0\n")

	h := NewScriptHandler(slog.Default(), WithScriptTimeout(5*time.Second))
	result, err := h.Run(context.Background(), scriptPath, `{}`, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestScriptHandler_ContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	setupOrcScriptDir(t, tmpDir)

	scriptPath := writeScript(t, tmpDir, ".orc/scripts/blocked.sh",
		"#!/bin/sh\ncat > /dev/null\nsleep 30\nexit 0\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	h := NewScriptHandler(slog.Default())
	_, err := h.Run(ctx, scriptPath, `{}`, tmpDir)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// --- Test helpers ---

// setupOrcScriptDir creates the .orc/scripts/ directory structure.
func setupOrcScriptDir(t *testing.T, projectDir string) {
	t.Helper()
	scriptsDir := filepath.Join(projectDir, ".orc", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}
}

// writeScript writes a script file with executable permissions and returns
// its absolute path.
func writeScript(t *testing.T, projectDir, relPath, content string) string {
	t.Helper()
	absPath := filepath.Join(projectDir, relPath)
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write script %s: %v", absPath, err)
	}
	return absPath
}
