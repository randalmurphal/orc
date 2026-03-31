package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

// runTests runs the test suite after sync.
func (e *FinalizeExecutor) runTests(ctx context.Context, _ *orcv1.Task, _ config.FinalizeConfig) (*ParsedTestResult, error) {
	testCmd := "go test ./... -v -race"
	if e.orcConfig != nil && e.orcConfig.Testing.Commands.Unit != "" {
		testCmd = e.orcConfig.Testing.Commands.Unit
	}

	e.logger.Info("running tests", "command", testCmd)

	workDir := e.workingDir
	if workDir == "" {
		return nil, fmt.Errorf("executor workingDir not set: cannot run tests safely")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", testCmd)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOWORK=off")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result, parseErr := ParseTestOutput(outputStr)
	if parseErr != nil {
		e.logger.Warn("failed to parse test output", "error", parseErr)
		result = &ParsedTestResult{Framework: "unknown"}
	}

	if err != nil || result.Failed > 0 {
		return result, fmt.Errorf("tests failed: %d failures", result.Failed)
	}

	e.logger.Info("tests passed", "passed", result.Passed, "coverage", result.Coverage)
	return result, nil
}

// tryFixTests attempts to fix test failures using Claude with retry logic.
func (e *FinalizeExecutor) tryFixTests(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	_ *orcv1.ExecutionState,
	testResult *ParsedTestResult,
) (bool, error) {
	const maxFixAttempts = 3
	cfg := e.getFinalizeConfig()

	model := e.config.Model
	if model == "" {
		model = "opus"
	}

	currentResult := testResult

	for attempt := 1; attempt <= maxFixAttempts; attempt++ {
		e.logger.Info("attempting to fix tests",
			"task", t.Id,
			"attempt", attempt,
			"max_attempts", maxFixAttempts,
			"failures", currentResult.Failed,
		)

		prompt := buildTestFixPromptWithAttempt(t, currentResult, attempt, maxFixAttempts)

		var turnExec TurnExecutor
		sessionID := fmt.Sprintf("%s-test-fix-%d", t.Id, attempt)
		if e.turnExecutor != nil {
			turnExec = e.turnExecutor
		} else {
			claudeOpts := []ClaudeExecutorOption{
				WithClaudePath(e.claudePath),
				WithClaudeWorkdir(e.workingDir),
				WithClaudeModel(model),
				WithClaudeSessionID(sessionID),
				WithClaudeMaxTurns(10),
				WithClaudeLogger(e.logger),
				WithClaudePhaseID(p.ID),
				WithClaudeBackend(e.backend),
				WithClaudeTaskID(t.Id),
			}
			turnExec = NewClaudeExecutor(claudeOpts...)
		}

		_, err := turnExec.ExecuteTurn(ctx, prompt)
		if err != nil {
			e.logger.Warn("test fix turn failed", "attempt", attempt, "error", err)
			if attempt < maxFixAttempts {
				continue
			}
			return false, fmt.Errorf("test fix turn failed after %d attempts: %w", attempt, err)
		}

		newResult, testErr := e.runTests(ctx, t, cfg)
		if testErr == nil && newResult.Failed == 0 {
			e.logger.Info("tests fixed successfully", "attempt", attempt, "passed", newResult.Passed)
			return true, nil
		}

		if newResult != nil {
			e.logger.Warn("tests still failing after fix attempt",
				"attempt", attempt,
				"previous_failures", currentResult.Failed,
				"current_failures", newResult.Failed,
			)
			currentResult = newResult
		}
	}

	return false, fmt.Errorf("tests still failing after %d fix attempts: %d failures remain", maxFixAttempts, currentResult.Failed)
}

// buildTestFixPrompt creates the prompt for fixing test failures.
func buildTestFixPrompt(t *orcv1.Task, testResult *ParsedTestResult) string {
	var sb strings.Builder

	sb.WriteString("# Test Failure Fix Task\n\n")
	sb.WriteString("You are fixing test failures for task: ")
	sb.WriteString(t.Id)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	sb.WriteString("## Test Failures\n\n")
	for i, f := range testResult.Failures {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... and %d more failures\n", len(testResult.Failures)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n", f.Test))
		if f.File != "" {
			sb.WriteString(fmt.Sprintf("**File**: `%s:%d`\n", f.File, f.Line))
		}
		if f.Message != "" {
			sb.WriteString(fmt.Sprintf("**Error**: %s\n", f.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Analyze each failing test\n")
	sb.WriteString("2. Fix the code or test as appropriate\n")
	sb.WriteString("3. The fix should preserve all intended functionality\n")
	sb.WriteString("4. Do NOT remove tests to fix failures\n")
	sb.WriteString("5. When done, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Fixed X test failures"}`)
	sb.WriteString("\n\nIf you cannot fix the tests, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
}

// buildTestFixPromptWithAttempt creates the prompt for fixing test failures with retry context.
func buildTestFixPromptWithAttempt(t *orcv1.Task, testResult *ParsedTestResult, attempt, maxAttempts int) string {
	var sb strings.Builder

	sb.WriteString("# Test Failure Fix Task\n\n")
	sb.WriteString("You are fixing test failures for task: ")
	sb.WriteString(t.Id)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	if attempt > 1 {
		sb.WriteString("## Retry Context\n\n")
		sb.WriteString(fmt.Sprintf("**Attempt %d of %d** - Previous fix attempts did not resolve all failures.\n", attempt, maxAttempts))
		sb.WriteString("Try a different approach. Consider:\n")
		sb.WriteString("- The test expectations may need adjustment (not the test removal)\n")
		sb.WriteString("- A different implementation approach may be needed\n")
		sb.WriteString("- There may be side effects from previous fixes causing new failures\n")
		sb.WriteString("- Check for race conditions or timing issues\n\n")
	}

	sb.WriteString("## Test Failures\n\n")
	sb.WriteString(fmt.Sprintf("**%d failing tests** (showing up to 5):\n\n", testResult.Failed))
	for i, f := range testResult.Failures {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... and %d more failures\n", len(testResult.Failures)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n", f.Test))
		if f.File != "" {
			sb.WriteString(fmt.Sprintf("**File**: `%s:%d`\n", f.File, f.Line))
		}
		if f.Message != "" {
			sb.WriteString(fmt.Sprintf("**Error**: %s\n", f.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Analyze each failing test\n")
	sb.WriteString("2. Fix the code or test as appropriate\n")
	sb.WriteString("3. The fix should preserve all intended functionality\n")
	sb.WriteString("4. Do NOT remove tests to fix failures\n")
	sb.WriteString("5. When done, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Fixed X test failures"}`)
	sb.WriteString("\n\nIf you cannot fix the tests, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
}

// buildTestFailureContext creates context for test failure escalation.
func buildTestFailureContext(testResult *ParsedTestResult) string {
	if testResult == nil {
		return "Tests failed with unknown results"
	}
	return BuildTestRetryContext("finalize", testResult)
}
