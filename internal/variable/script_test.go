package variable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScriptExecutorBasic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a simple script
	scriptPath := filepath.Join(scriptsDir, "hello.sh")
	scriptContent := `#!/bin/bash
echo "Hello, World!"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "hello.sh",
	}

	output, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", output)
	}
}

func TestScriptExecutorWithArgs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script that uses arguments
	scriptPath := filepath.Join(scriptsDir, "greet.sh")
	scriptContent := `#!/bin/bash
echo "Hello, $1!"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "greet.sh",
		Args: []string{"Claude"},
	}

	output, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "Hello, Claude!" {
		t.Errorf("expected 'Hello, Claude!', got '%s'", output)
	}
}

func TestScriptExecutorTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script that loops with short sleeps
	// This exits more cleanly when killed than a long sleep
	scriptPath := filepath.Join(scriptsDir, "slow.sh")
	scriptContent := `#!/bin/bash
for i in $(seq 1 100); do
    sleep 0.1
done
echo "done"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path:      "slow.sh",
		TimeoutMS: 200, // 200ms timeout
	}

	start := time.Now()
	_, err := executor.Execute(context.Background(), cfg, tmpDir)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}

	// Should timeout quickly, not run for 10 seconds
	if elapsed > 1*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}

	// Error message should indicate timeout
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScriptExecutorNotExecutable(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a non-executable script
	scriptPath := filepath.Join(scriptsDir, "notexec.sh")
	if err := os.WriteFile(scriptPath, []byte("echo hello"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "notexec.sh",
	}

	_, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err == nil {
		t.Error("expected error for non-executable script")
	}
}

func TestScriptExecutorOutsideAllowed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script outside allowed directory
	outsidePath := filepath.Join(tmpDir, "outside.sh")
	if err := os.WriteFile(outsidePath, []byte("#!/bin/bash\necho evil"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: outsidePath, // Absolute path outside allowed dir
	}

	_, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err == nil {
		t.Error("expected error for script outside allowed directory")
	}
}

func TestScriptExecutorNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "nonexistent.sh",
	}

	_, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err == nil {
		t.Error("expected error for non-existent script")
	}
}

func TestScriptExecutorExitCode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script that fails
	scriptPath := filepath.Join(scriptsDir, "fail.sh")
	scriptContent := `#!/bin/bash
echo "error message" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "fail.sh",
	}

	_, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err == nil {
		t.Error("expected error for script with non-zero exit")
	}

	// Check error contains stderr
	if err.Error() != "script exited with code 1: error message" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScriptExecutorEnvironment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script that reads environment
	scriptPath := filepath.Join(scriptsDir, "env.sh")
	scriptContent := `#!/bin/bash
echo "$ORC_PROJECT_ROOT"
`
	// Write and sync to avoid "text file busy" race
	f, err := os.OpenFile(scriptPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	if _, err := f.WriteString(scriptContent); err != nil {
		f.Close()
		t.Fatalf("write script: %v", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		t.Fatalf("sync script: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)

	cfg := &ScriptConfig{
		Path: "env.sh",
	}

	output, err := executor.Execute(context.Background(), cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != tmpDir {
		t.Errorf("expected '%s', got '%s'", tmpDir, output)
	}
}

func TestScriptExecutorContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, DefaultScriptsSubdir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}

	// Create a script that runs indefinitely (until killed)
	// Using a simple loop that checks frequently
	scriptPath := filepath.Join(scriptsDir, "long.sh")
	scriptContent := `#!/bin/bash
for i in $(seq 1 1000); do
    sleep 0.1
done
echo "done"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	executor := NewScriptExecutor(tmpDir)
	executor.DefaultTimeout = 60 * time.Second // Long timeout (won't be reached)

	cfg := &ScriptConfig{
		Path: "long.sh",
	}

	// Create a context with a short deadline
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Execute - should fail due to context timeout
	_, err := executor.Execute(ctx, cfg, tmpDir)
	if err == nil {
		t.Error("expected error after context deadline")
	}
}
