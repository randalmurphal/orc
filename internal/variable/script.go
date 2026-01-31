package variable

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ScriptExecutor runs scripts in a sandboxed environment.
type ScriptExecutor struct {
	// AllowedDirs are the directories where scripts can be executed from.
	// Scripts outside these directories will be rejected.
	AllowedDirs []string

	// DefaultTimeout is the default script timeout.
	DefaultTimeout time.Duration

	// MaxOutputBytes is the maximum output size to capture.
	MaxOutputBytes int
}

// Default configuration values.
const (
	DefaultScriptTimeout   = 5 * time.Second
	DefaultMaxOutputBytes  = 1 << 20 // 1MB
	DefaultScriptsSubdir   = ".orc/scripts"
)

// NewScriptExecutor creates a new script executor with the given project root.
// Scripts are only allowed to run from {projectRoot}/.orc/scripts/.
func NewScriptExecutor(projectRoot string) *ScriptExecutor {
	return &ScriptExecutor{
		AllowedDirs: []string{
			filepath.Join(projectRoot, DefaultScriptsSubdir),
		},
		DefaultTimeout: DefaultScriptTimeout,
		MaxOutputBytes: DefaultMaxOutputBytes,
	}
}

// Execute runs a script and returns its stdout.
func (se *ScriptExecutor) Execute(ctx context.Context, cfg *ScriptConfig, projectRoot string) (string, error) {
	// Resolve script path
	scriptPath, err := se.resolveScriptPath(cfg.Path, projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve script path: %w", err)
	}

	// Validate script is in allowed directory
	if err := se.validatePath(scriptPath); err != nil {
		return "", err
	}

	// Validate script file exists and is executable
	info, err := os.Stat(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("script not found: %s", scriptPath)
		}
		return "", fmt.Errorf("stat script: %w", err)
	}

	// Check if file is executable (has at least one execute bit)
	if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("script is not executable: %s", scriptPath)
	}

	// Determine timeout
	timeout := se.DefaultTimeout
	if cfg.TimeoutMS > 0 {
		timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Determine working directory
	workDir := projectRoot
	if cfg.WorkDir != "" {
		workDir = cfg.WorkDir
	}

	// Build command
	cmd := exec.CommandContext(execCtx, scriptPath, cfg.Args...)
	cmd.Dir = workDir

	// Set up environment - inherit current env but allow script to see PROJECT_ROOT
	cmd.Env = append(os.Environ(),
		"ORC_PROJECT_ROOT="+projectRoot,
		"ORC_SCRIPT_PATH="+scriptPath,
	)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, max: se.MaxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, max: se.MaxOutputBytes}

	// Execute
	err = cmd.Run()

	// Check for context cancellation (timeout)
	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("script timeout after %s", timeout)
	}

	// Check for execution error
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderrStr := strings.TrimSpace(stderr.String())
			if stderrStr != "" {
				return "", fmt.Errorf("script exited with code %d: %s", exitErr.ExitCode(), stderrStr)
			}
			return "", fmt.Errorf("script exited with code %d", exitErr.ExitCode())
		}
		return "", fmt.Errorf("execute script: %w", err)
	}

	// Return trimmed stdout
	return strings.TrimSpace(stdout.String()), nil
}

// resolveScriptPath resolves the script path to an absolute path.
// If the path is relative, it's resolved relative to .orc/scripts/.
func (se *ScriptExecutor) resolveScriptPath(path, projectRoot string) (string, error) {
	// If absolute, use as-is
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// If it starts with .orc/scripts/, resolve from project root
	if strings.HasPrefix(path, DefaultScriptsSubdir) {
		return filepath.Clean(filepath.Join(projectRoot, path)), nil
	}

	// Otherwise, assume it's relative to .orc/scripts/
	return filepath.Clean(filepath.Join(projectRoot, DefaultScriptsSubdir, path)), nil
}

// validatePath ensures the script path is within an allowed directory.
func (se *ScriptExecutor) validatePath(scriptPath string) error {
	// Get absolute path
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	// Check if path is within any allowed directory
	for _, allowed := range se.AllowedDirs {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}

		// Use HasPrefix after ensuring trailing separator for security
		// This prevents /foo/bar from matching /foo/barbaz
		allowedWithSep := allowedAbs + string(filepath.Separator)
		absWithSep := absPath + string(filepath.Separator)

		if strings.HasPrefix(absWithSep, allowedWithSep) || absPath == allowedAbs {
			return nil
		}
	}

	return fmt.Errorf("script path %s is outside allowed directories", scriptPath)
}

// limitedWriter wraps a writer and limits the amount of data written.
type limitedWriter struct {
	w       *bytes.Buffer
	max     int
	written int
}

func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	remaining := lw.max - lw.written
	if remaining <= 0 {
		return len(p), nil // Discard but pretend we wrote it
	}

	toWrite := p
	if len(p) > remaining {
		toWrite = p[:remaining]
	}

	n, err = lw.w.Write(toWrite)
	lw.written += n

	// Return original length so cmd doesn't get stuck
	return len(p), err
}

