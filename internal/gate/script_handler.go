package gate

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultScriptTimeout = 30 * time.Second

// ScriptResult holds the outcome of running a gate output script.
type ScriptResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Override bool   // True if non-zero exit (script wants to override gate decision)
	Reason   string // Explanation when Override is true
}

// ScriptHandler runs gate output scripts and captures their results.
type ScriptHandler struct {
	logger  *slog.Logger
	timeout time.Duration
}

// ScriptHandlerOption configures a ScriptHandler.
type ScriptHandlerOption func(*ScriptHandler)

// WithScriptTimeout sets the timeout for script execution.
func WithScriptTimeout(d time.Duration) ScriptHandlerOption {
	return func(h *ScriptHandler) {
		h.timeout = d
	}
}

// NewScriptHandler creates a new ScriptHandler with the given options.
func NewScriptHandler(logger *slog.Logger, opts ...ScriptHandlerOption) *ScriptHandler {
	h := &ScriptHandler{
		logger:  logger,
		timeout: defaultScriptTimeout,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Run executes the script at scriptPath, piping gateOutputJSON to its stdin.
// Non-zero exit codes are not errors — they indicate the script wants to
// override the gate decision. Errors are reserved for infrastructure failures
// (script not found, timeout, permission denied, etc.).
func (h *ScriptHandler) Run(ctx context.Context, scriptPath, gateOutputJSON, projectDir string) (*ScriptResult, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = projectDir
	cmd.Stdin = strings.NewReader(gateOutputJSON)
	cmd.WaitDelay = time.Second // Allow I/O to drain after context cancellation

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	h.logger.Debug("running gate script", "path", scriptPath, "project", projectDir)

	err := cmd.Run()

	result := &ScriptResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		// Context cancellation/timeout takes priority — it's an infrastructure error
		// even if the process exited with a signal.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("run gate script %s: %w", scriptPath, ctx.Err())
		}
		// Non-zero exit is a valid override, not an error.
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Override = true
			result.Reason = fmt.Sprintf("script exited with code %d", result.ExitCode)
			if result.Stderr != "" {
				result.Reason += ": " + strings.TrimSpace(result.Stderr)
			}
			return result, nil
		}
		// Infrastructure error (not found, permission denied, etc.)
		return nil, fmt.Errorf("run gate script %s: %w", scriptPath, err)
	}

	return result, nil
}

// ValidateScriptPath validates and resolves a script path.
// Relative paths are resolved under <projectDir>/.orc/.
// Absolute paths are accepted as-is.
// Path traversal outside .orc/ is rejected for relative paths.
// Empty paths return an error.
func ValidateScriptPath(scriptPath, projectDir string) (string, error) {
	if scriptPath == "" {
		return "", fmt.Errorf("validate script path: empty path")
	}

	// Absolute paths are accepted as-is.
	if filepath.IsAbs(scriptPath) {
		return scriptPath, nil
	}

	// Relative paths resolve under <projectDir>/.orc/
	orcDir := filepath.Join(projectDir, ".orc")
	resolved := filepath.Join(orcDir, scriptPath)
	resolved = filepath.Clean(resolved)

	// Ensure the resolved path is still under .orc/
	cleanOrc := filepath.Clean(orcDir) + string(filepath.Separator)
	if !strings.HasPrefix(resolved+string(filepath.Separator), cleanOrc) && resolved != filepath.Clean(orcDir) {
		return "", fmt.Errorf("validate script path: path traversal outside .orc/ directory: %s", scriptPath)
	}

	return resolved, nil
}
