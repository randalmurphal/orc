// Package executor provides the flowgraph-based execution engine for orc.
// This file contains the QualityCheckRunner for phase-level quality checks.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// detectShell returns the shell to use for command execution.
// Prefers bash for consistency, falls back to sh.
func detectShell() string {
	// Prefer bash for consistent behavior
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash"
	}
	// Fall back to sh
	if _, err := exec.LookPath("sh"); err == nil {
		return "sh"
	}
	// Use SHELL environment variable as last resort
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "sh"
}

// DefaultCheckTimeout is the default timeout for individual quality checks.
const DefaultCheckTimeout = 2 * time.Minute

// CheckResult holds the result of a single quality check.
type CheckResult struct {
	Name      string        `json:"name"`
	Passed    bool          `json:"passed"`
	Output    string        `json:"output,omitempty"`
	Duration  time.Duration `json:"duration"`
	OnFailure string        `json:"on_failure"` // "block", "warn", "skip"
	Skipped   bool          `json:"skipped"`
}

// QualityCheckResult holds the results of all quality checks for a phase.
type QualityCheckResult struct {
	Checks    []CheckResult `json:"checks"`
	AllPassed bool          `json:"all_passed"`
	HasBlocks bool          `json:"has_blocks"` // True if any blocking check failed
	Duration  time.Duration `json:"duration"`
}

// AsContext formats quality check failures for injection into the next iteration prompt.
// This provides clear, actionable feedback to the agent about what needs fixing.
func (r *QualityCheckResult) AsContext() string {
	if r.AllPassed {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Quality Check Failures\n\n")
	sb.WriteString("The following quality checks failed. You MUST fix these issues before marking the phase complete.\n\n")

	for _, check := range r.Checks {
		if check.Skipped || check.Passed {
			continue
		}

		// Capitalize first letter of check name
		checkName := check.Name
		if len(checkName) > 0 {
			checkName = strings.ToUpper(checkName[:1]) + checkName[1:]
		}
		fmt.Fprintf(&sb, "### %s Failed", checkName)
		if check.OnFailure == "warn" {
			sb.WriteString(" (Warning)")
		}
		sb.WriteString("\n\n")

		if check.Output != "" {
			sb.WriteString("```\n")
			sb.WriteString(truncateCheckOutput(check.Output, 3000))
			sb.WriteString("\n```\n\n")
		}
	}

	if r.HasBlocks {
		sb.WriteString("Fix all blocking issues above before claiming completion.\n")
	}
	return sb.String()
}

// FailureSummary returns a brief summary of what failed.
func (r *QualityCheckResult) FailureSummary() string {
	if r.AllPassed {
		return "all checks passed"
	}

	var failures []string
	for _, check := range r.Checks {
		if !check.Passed && !check.Skipped {
			label := check.Name
			if check.OnFailure == "warn" {
				label += " (warn)"
			}
			failures = append(failures, label)
		}
	}

	if len(failures) == 0 {
		return "no failures"
	}
	return strings.Join(failures, ", ") + " failed"
}

// QualityCheckRunner executes quality checks defined at the phase level.
type QualityCheckRunner struct {
	workDir  string
	checks   []db.QualityCheck
	commands map[string]*db.ProjectCommand // name -> command
	logger   *slog.Logger
	shell    string
}

// NewQualityCheckRunner creates a new quality check runner.
// checks: quality checks from phase template or workflow override
// commands: project commands from database (for "code" type checks)
func NewQualityCheckRunner(
	workDir string,
	checks []db.QualityCheck,
	commands map[string]*db.ProjectCommand,
	logger *slog.Logger,
) *QualityCheckRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &QualityCheckRunner{
		workDir:  workDir,
		checks:   checks,
		commands: commands,
		logger:   logger,
		shell:    detectShell(),
	}
}

// Run executes all configured quality checks sequentially.
// Returns results for each check.
func (r *QualityCheckRunner) Run(ctx context.Context) *QualityCheckResult {
	start := time.Now()
	result := &QualityCheckResult{
		Checks:    make([]CheckResult, 0, len(r.checks)),
		AllPassed: true,
		HasBlocks: false,
	}

	if len(r.checks) == 0 {
		result.Duration = time.Since(start)
		return result
	}

	for _, check := range r.checks {
		checkResult := r.runCheck(ctx, check)
		result.Checks = append(result.Checks, checkResult)

		if !checkResult.Passed && !checkResult.Skipped {
			result.AllPassed = false
			if checkResult.OnFailure == "block" || checkResult.OnFailure == "" {
				result.HasBlocks = true
			}
		}
	}

	result.Duration = time.Since(start)

	r.logger.Info("quality checks completed",
		"all_passed", result.AllPassed,
		"has_blocks", result.HasBlocks,
		"check_count", len(result.Checks),
		"duration", result.Duration,
	)

	return result
}

// runCheck executes a single quality check.
func (r *QualityCheckRunner) runCheck(ctx context.Context, check db.QualityCheck) CheckResult {
	result := CheckResult{
		Name:      check.Name,
		OnFailure: check.OnFailure,
	}

	// Default on_failure to "block" if not specified
	if result.OnFailure == "" {
		result.OnFailure = "block"
	}

	// Handle disabled checks
	if !check.Enabled {
		result.Passed = true
		result.Skipped = true
		return result
	}

	// Handle "skip" on_failure mode
	if check.OnFailure == "skip" {
		result.Passed = true
		result.Skipped = true
		return result
	}

	// Resolve the command to run
	command := check.Command
	if command == "" && check.Type == "code" {
		// Look up command from project commands
		if projCmd, ok := r.commands[check.Name]; ok {
			if projCmd.Enabled {
				// Use short_command if UseShort is set and short_command exists
				if check.UseShort && projCmd.ShortCommand != "" {
					command = projCmd.ShortCommand
				} else {
					command = projCmd.Command
				}
			}
		}
	}

	// If no command found, treat as passed (check not applicable)
	if command == "" {
		r.logger.Info("quality check skipped - no command configured",
			"name", check.Name,
			"type", check.Type,
			"hint", "use 'orc config commands set' to configure",
		)
		result.Passed = true
		result.Skipped = true
		return result
	}

	// Determine timeout
	timeout := DefaultCheckTimeout
	if check.TimeoutMs > 0 {
		timeout = time.Duration(check.TimeoutMs) * time.Millisecond
	}

	// Execute the command
	start := time.Now()
	passed, output := r.runCommand(ctx, command, check.Name, timeout)
	result.Duration = time.Since(start)
	result.Passed = passed
	result.Output = output

	return result
}

// runCommand executes a shell command and returns whether it succeeded.
func (r *QualityCheckRunner) runCommand(ctx context.Context, command, checkName string, timeout time.Duration) (bool, string) {
	r.logger.Debug("running quality check",
		"name", checkName,
		"command", command,
		"workdir", r.workDir,
		"timeout", timeout,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command with context for cancellation/timeout
	cmd := exec.CommandContext(ctx, r.shell, "-c", command)
	cmd.Dir = r.workDir
	// Set GOWORK=off to avoid go.work issues in worktrees
	cmd.Env = append(os.Environ(), "GOWORK=off")

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
		output += fmt.Sprintf("\n[TIMEOUT] Command exceeded %v timeout", timeout)
		r.logger.Warn("quality check timed out",
			"name", checkName,
			"timeout", timeout,
		)
		return false, output
	}

	passed := err == nil
	if passed {
		r.logger.Info("quality check passed", "name", checkName)
	} else {
		r.logger.Info("quality check failed",
			"name", checkName,
			"error", err,
			"output_len", len(output),
		)
	}

	return passed, output
}

// truncateCheckOutput truncates output to a maximum length, preserving the end
// (which usually contains the most relevant error information).
func truncateCheckOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	// Keep the end, which usually has the summary
	return "...[truncated]\n" + output[len(output)-maxLen:]
}

// FormatQualityChecksForPrompt creates a prompt section from quality check results.
// This is used when re-prompting the agent after quality check failures.
func FormatQualityChecksForPrompt(result *QualityCheckResult) string {
	if result == nil || result.AllPassed {
		return ""
	}

	return fmt.Sprintf(`## Quality Check Failures

Your previous implementation attempt failed the following quality checks:

%s

**IMPORTANT**: You MUST fix these issues before claiming the phase is complete.
Do NOT output completion JSON until all quality checks pass.

Focus on the specific errors shown above and fix them one by one.`,
		result.AsContext())
}

// LoadQualityChecksForPhase resolves the quality checks to run for a phase.
// It handles workflow-level overrides and parses the JSON configuration.
// Returns nil if no checks are configured (phase doesn't run quality checks).
func LoadQualityChecksForPhase(
	phaseTemplate *db.PhaseTemplate,
	workflowPhase *db.WorkflowPhase,
) ([]db.QualityCheck, error) {
	// Check for workflow-level override first
	checksJSON := ""
	if workflowPhase != nil && workflowPhase.QualityChecksOverride != "" {
		checksJSON = workflowPhase.QualityChecksOverride
		// Empty array "[]" means disable all checks
		if checksJSON == "[]" {
			return nil, nil
		}
	} else if phaseTemplate != nil {
		checksJSON = phaseTemplate.QualityChecks
	}

	if checksJSON == "" {
		return nil, nil
	}

	return db.ParseQualityChecks(checksJSON)
}
