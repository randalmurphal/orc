// Package executor provides the flowgraph-based execution engine for orc.
// This file contains the BackpressureRunner for deterministic quality checks.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
)

// DefaultBackpressureTimeout is the default timeout for backpressure commands.
// Individual commands (tests, lint, build) should complete within this time.
const DefaultBackpressureTimeout = 5 * time.Minute

// BackpressureResult holds the results of deterministic quality checks.
// These are objective, repeatable checks that don't rely on LLM judgment.
type BackpressureResult struct {
	// TestsPassed indicates whether unit tests passed
	TestsPassed bool
	// TestOutput is the raw output from test execution
	TestOutput string

	// LintPassed indicates whether linting passed
	LintPassed bool
	// LintOutput is the raw output from linting
	LintOutput string

	// BuildPassed indicates whether the build succeeded
	BuildPassed bool
	// BuildOutput is the raw output from build
	BuildOutput string

	// TypeCheckPassed indicates whether type checking passed (TypeScript, Python)
	TypeCheckPassed bool
	// TypeCheckOutput is the raw output from type checking
	TypeCheckOutput string

	// AllPassed is true only if all enabled checks passed
	AllPassed bool

	// Duration is how long all checks took
	Duration time.Duration
}

// AsContext formats backpressure failures for injection into the next iteration prompt.
// This provides clear, actionable feedback to the agent about what needs fixing.
func (r *BackpressureResult) AsContext() string {
	if r.AllPassed {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Backpressure Check Failures\n\n")
	sb.WriteString("The following quality checks failed. You MUST fix these issues before marking the phase complete.\n\n")

	if !r.TestsPassed && r.TestOutput != "" {
		sb.WriteString("### Tests Failed\n\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateBackpressureOutput(r.TestOutput, 3000))
		sb.WriteString("\n```\n\n")
	}

	if !r.LintPassed && r.LintOutput != "" {
		sb.WriteString("### Lint Failed\n\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateBackpressureOutput(r.LintOutput, 2000))
		sb.WriteString("\n```\n\n")
	}

	if !r.TypeCheckPassed && r.TypeCheckOutput != "" {
		sb.WriteString("### Type Check Failed\n\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateBackpressureOutput(r.TypeCheckOutput, 2000))
		sb.WriteString("\n```\n\n")
	}

	if !r.BuildPassed && r.BuildOutput != "" {
		sb.WriteString("### Build Failed\n\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateBackpressureOutput(r.BuildOutput, 2000))
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Fix all issues above before claiming completion.\n")
	return sb.String()
}

// HasFailures returns true if any check failed.
func (r *BackpressureResult) HasFailures() bool {
	return !r.AllPassed
}

// FailureSummary returns a brief summary of what failed.
func (r *BackpressureResult) FailureSummary() string {
	if r.AllPassed {
		return "all checks passed"
	}

	var failures []string
	if !r.TestsPassed && r.TestOutput != "" {
		failures = append(failures, "tests")
	}
	if !r.LintPassed && r.LintOutput != "" {
		failures = append(failures, "lint")
	}
	if !r.TypeCheckPassed && r.TypeCheckOutput != "" {
		failures = append(failures, "typecheck")
	}
	if !r.BuildPassed && r.BuildOutput != "" {
		failures = append(failures, "build")
	}

	if len(failures) == 0 {
		return "no failures"
	}
	return strings.Join(failures, ", ") + " failed"
}

// BackpressureRunner executes deterministic quality checks.
type BackpressureRunner struct {
	workDir string
	config  *config.ValidationConfig
	testing *config.TestingConfig
	logger  *slog.Logger
	shell   string // Shell to use for executing commands (bash or sh)
}

// NewBackpressureRunner creates a new backpressure runner.
func NewBackpressureRunner(workDir string, valCfg *config.ValidationConfig, testCfg *config.TestingConfig, logger *slog.Logger) *BackpressureRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackpressureRunner{
		workDir: workDir,
		config:  valCfg,
		testing: testCfg,
		logger:  logger,
		shell:   detectShell(),
	}
}

// detectShell returns the available shell, preferring bash over sh.
func detectShell() string {
	// Try bash first
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash"
	}
	// Fall back to sh (POSIX shell, should always exist)
	if _, err := exec.LookPath("sh"); err == nil {
		return "sh"
	}
	// Default to bash and let it fail if neither exists
	return "bash"
}

// Run executes all configured backpressure checks sequentially.
// Returns results for each check type.
func (r *BackpressureRunner) Run(ctx context.Context) *BackpressureResult {
	start := time.Now()
	result := &BackpressureResult{
		TestsPassed:     true, // Default to true if not run
		LintPassed:      true,
		BuildPassed:     true,
		TypeCheckPassed: true,
	}

	if r.config == nil || !r.config.Enabled {
		result.AllPassed = true
		result.Duration = time.Since(start)
		return result
	}

	// Run checks based on configuration
	if r.config.EnforceTests && r.testing != nil && r.testing.Commands.Unit != "" {
		passed, output := r.runCommand(ctx, r.testing.Commands.Unit, "tests")
		result.TestsPassed = passed
		result.TestOutput = output
	}

	if r.config.EnforceLint && r.config.LintCommand != "" {
		passed, output := r.runCommand(ctx, r.config.LintCommand, "lint")
		result.LintPassed = passed
		result.LintOutput = output
	}

	if r.config.EnforceBuild && r.config.BuildCommand != "" {
		passed, output := r.runCommand(ctx, r.config.BuildCommand, "build")
		result.BuildPassed = passed
		result.BuildOutput = output
	}

	if r.config.EnforceTypeCheck && r.config.TypeCheckCommand != "" {
		passed, output := r.runCommand(ctx, r.config.TypeCheckCommand, "typecheck")
		result.TypeCheckPassed = passed
		result.TypeCheckOutput = output
	}

	// AllPassed is only true if everything that was checked passed
	result.AllPassed = result.TestsPassed && result.LintPassed &&
		result.BuildPassed && result.TypeCheckPassed
	result.Duration = time.Since(start)

	r.logger.Info("backpressure checks completed",
		"all_passed", result.AllPassed,
		"tests", result.TestsPassed,
		"lint", result.LintPassed,
		"build", result.BuildPassed,
		"typecheck", result.TypeCheckPassed,
		"duration", result.Duration,
	)

	return result
}

// runCommand executes a shell command and returns whether it succeeded.
func (r *BackpressureRunner) runCommand(ctx context.Context, command, checkType string) (bool, string) {
	if command == "" {
		return true, ""
	}

	r.logger.Debug("running backpressure check",
		"type", checkType,
		"command", command,
		"workdir", r.workDir,
	)

	// Apply default timeout if context doesn't have a deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultBackpressureTimeout)
		defer cancel()
	}

	// Create command with context for cancellation/timeout
	// Use shell -c to handle complex commands with pipes, etc.
	cmd := exec.CommandContext(ctx, r.shell, "-c", command)
	cmd.Dir = r.workDir

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Combine output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		output += fmt.Sprintf("\n[TIMEOUT] Command exceeded %v timeout", DefaultBackpressureTimeout)
		r.logger.Warn("backpressure check timed out",
			"type", checkType,
			"timeout", DefaultBackpressureTimeout,
		)
		return false, output
	}

	passed := err == nil
	if !passed {
		r.logger.Debug("backpressure check failed",
			"type", checkType,
			"error", err,
			"output_len", len(output),
		)
	}

	return passed, output
}

// truncateBackpressureOutput truncates output to a maximum length, preserving the end
// (which usually contains the most relevant error information).
func truncateBackpressureOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	// Keep the end, which usually has the summary
	return "...[truncated]\n" + output[len(output)-maxLen:]
}

// ShouldSkipBackpressure returns true if backpressure should be skipped for this phase.
func ShouldSkipBackpressure(phaseID string) bool {
	// Only apply backpressure to implement phase
	// Other phases (spec, design, review, docs) don't produce code to validate
	return phaseID != "implement"
}

// DetectProjectCommands auto-detects appropriate commands based on project files.
// Returns commands for: tests, lint, build, typecheck
func DetectProjectCommands(workDir string) (tests, lint, build, typecheck string) {
	// Check for Go project
	if fileExists(workDir, "go.mod") {
		tests = "go test ./..."
		lint = "golangci-lint run ./..."
		build = "go build ./..."
		return
	}

	// Check for Node/TypeScript project
	if fileExists(workDir, "package.json") {
		tests = "npm test --if-present"
		lint = "npm run lint --if-present"
		// Check for TypeScript
		if fileExists(workDir, "tsconfig.json") {
			typecheck = "npm run typecheck --if-present || npx tsc --noEmit"
			build = "npm run build --if-present"
		}
		return
	}

	// Check for Python project
	if fileExists(workDir, "pyproject.toml") || fileExists(workDir, "setup.py") {
		tests = "pytest"
		lint = "ruff check ."
		typecheck = "pyright"
		return
	}

	// Check for Rust project
	if fileExists(workDir, "Cargo.toml") {
		tests = "cargo test"
		lint = "cargo clippy"
		build = "cargo build"
		return
	}

	return "", "", "", ""
}

// fileExists checks if a file exists in the given directory.
func fileExists(dir, filename string) bool {
	path := filepath.Join(dir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// FormatBackpressureForPrompt creates a prompt section from backpressure results.
// This is used when re-prompting the agent after backpressure failure.
func FormatBackpressureForPrompt(result *BackpressureResult) string {
	if result == nil || result.AllPassed {
		return ""
	}

	return fmt.Sprintf(`## Quality Check Failures (Backpressure)

Your previous implementation attempt failed the following quality checks:

%s

**IMPORTANT**: You MUST fix these issues before claiming the phase is complete.
Do NOT output completion JSON until all quality checks pass.

Focus on the specific errors shown above and fix them one by one.`,
		result.AsContext())
}
