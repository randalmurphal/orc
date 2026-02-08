package bench

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// EvalResult holds the automated evaluation results for a run.
type EvalResult struct {
	TestPass         bool
	TestCount        int
	RegressionCount  int
	LintWarnings     int
	BuildSuccess     bool
	SecurityFindings int
	Duration         time.Duration
}

// Evaluator runs automated checks against a benchmark workspace.
type Evaluator struct{}

// NewEvaluator creates a new evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// RunAll executes all automated evaluations for a completed run.
func (e *Evaluator) RunAll(workDir string, project *Project, task *Task) (*EvalResult, error) {
	result := &EvalResult{}
	start := time.Now()

	// Build check
	if project.BuildCmd != "" {
		result.BuildSuccess = e.runBuild(workDir, project.BuildCmd)
	} else {
		result.BuildSuccess = true // No build command = assume success
	}

	// Test check (fail-to-pass: new tests should pass)
	if project.TestCmd != "" {
		pass, count, regressions := e.runTests(workDir, project.TestCmd, task)
		result.TestPass = pass
		result.TestCount = count
		result.RegressionCount = regressions
	}

	// Lint check
	if project.LintCmd != "" {
		result.LintWarnings = e.runLint(workDir, project.LintCmd)
	}

	// Security scan
	if project.SecurityCmd != "" {
		result.SecurityFindings = e.runSecurity(workDir, project.SecurityCmd)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runBuild executes the build command.
func (e *Evaluator) runBuild(workDir, buildCmd string) bool {
	_, _, err := runShell(workDir, buildCmd)
	return err == nil
}

// runTests executes the test command and checks for pass/fail.
// Returns (all_target_tests_pass, total_test_count, regressions_count).
func (e *Evaluator) runTests(workDir, testCmd string, task *Task) (pass bool, count, regressions int) {
	stdout, stderr, err := runShell(workDir, testCmd)
	combined := stdout + "\n" + stderr

	// Try to extract test counts from output (language-specific)
	count = extractTestCount(combined)

	if err != nil {
		// Tests failed. Check if it's regressions (pass_to_pass tests failing)
		// vs expected failures (fail_to_pass tests failing).
		regressions = countRegressions(combined, task.PassToPassTests)
		return false, count, regressions
	}

	// All tests passed
	// Check that fail_to_pass tests actually ran (not just silently skipped)
	if len(task.FailToPassTests) > 0 {
		ran := countTestsRun(combined, task.FailToPassTests)
		if ran < len(task.FailToPassTests) {
			// Some target tests didn't run — that's not a pass
			return false, count, 0
		}
	}

	return true, count, 0
}

// runLint executes the lint command and counts warnings.
func (e *Evaluator) runLint(workDir, lintCmd string) int {
	stdout, stderr, _ := runShell(workDir, lintCmd)
	combined := stdout + "\n" + stderr
	return countLintWarnings(combined)
}

// runSecurity executes a security scan and counts findings.
func (e *Evaluator) runSecurity(workDir, securityCmd string) int {
	stdout, _, _ := runShell(workDir, securityCmd)
	return countSecurityFindings(stdout)
}

// runShell executes a shell command and returns stdout, stderr, and error.
func runShell(dir, cmdStr string) (string, string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// extractTestCount tries to extract the number of tests from output.
// Supports Go, Python (pytest), Node (vitest/jest), and Rust (cargo test).
func extractTestCount(output string) int {
	lines := strings.Split(output, "\n")

	// Go: count "--- PASS:" and "--- FAIL:" lines (verbose output)
	// or count "ok" / "FAIL" package summary lines
	goPass, goFail := 0, 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Go verbose: "--- PASS: TestFoo (0.01s)" / "--- FAIL: TestBar (0.01s)"
		if strings.HasPrefix(trimmed, "--- PASS:") {
			goPass++
			continue
		}
		if strings.HasPrefix(trimmed, "--- FAIL:") {
			goFail++
			continue
		}

		// Python pytest: "5 passed, 2 failed" or "5 passed"
		if strings.Contains(trimmed, "passed") && (strings.Contains(trimmed, "failed") || strings.Contains(trimmed, "error") || strings.Contains(trimmed, "warning")) {
			return extractNumberBefore(trimmed, "passed") + extractNumberBefore(trimmed, "failed")
		}
		// pytest summary: "5 passed" (no failures)
		if strings.Contains(trimmed, " passed") && strings.HasPrefix(trimmed, "=") {
			return extractNumberBefore(trimmed, "passed")
		}

		// Rust: "test result: ok. 5 passed; 0 failed"
		if strings.HasPrefix(trimmed, "test result:") {
			return extractNumberBefore(trimmed, "passed") + extractNumberBefore(trimmed, "failed")
		}

		// Node vitest/jest: "Tests:  5 passed, 0 failed"
		if strings.HasPrefix(trimmed, "Tests:") || strings.HasPrefix(trimmed, "Test Suites:") {
			return extractNumberBefore(trimmed, "passed") + extractNumberBefore(trimmed, "failed")
		}
	}

	// If we found Go test markers, use those
	if goPass+goFail > 0 {
		return goPass + goFail
	}

	return 0
}

// extractNumberBefore extracts the number immediately before a keyword.
func extractNumberBefore(line, keyword string) int {
	idx := strings.Index(line, keyword)
	if idx <= 0 {
		return 0
	}
	// Walk backwards from idx to find the number
	end := idx - 1
	for end >= 0 && line[end] == ' ' {
		end--
	}
	start := end
	for start >= 0 && line[start] >= '0' && line[start] <= '9' {
		start--
	}
	start++
	if start > end {
		return 0
	}
	var n int
	fmt.Sscanf(line[start:end+1], "%d", &n)
	return n
}

// countRegressions checks if any pass_to_pass tests appear on failure lines.
// Only counts a test as regressed if its name appears on the same line as a FAIL indicator.
func countRegressions(output string, passToPass []string) int {
	if len(passToPass) == 0 {
		return 0
	}
	count := 0
	lines := strings.Split(output, "\n")
	for _, test := range passToPass {
		for _, line := range lines {
			if strings.Contains(line, test) &&
				(strings.Contains(line, "FAIL") || strings.Contains(line, "failed") || strings.Contains(line, "FAILED")) {
				count++
				break // Found this test as a regression, move to next test
			}
		}
	}
	return count
}

// countTestsRun checks how many of the target tests appear in test output.
func countTestsRun(output string, tests []string) int {
	count := 0
	for _, test := range tests {
		if strings.Contains(output, test) {
			count++
		}
	}
	return count
}

// lintLinePattern matches linter output lines like "file.go:10:5: message"
// or "/path/file.ts:10:5: message" (file:line:col: format used by most linters).
var lintLinePattern = regexp.MustCompile(`^\S+\.\w+:\d+:\d+:`)

// countLintWarnings counts warning lines in linter output.
// Matches the standard file:line:col: format used by golangci-lint, ESLint, etc.
func countLintWarnings(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if lintLinePattern.MatchString(strings.TrimSpace(line)) {
			count++
		}
	}
	return count
}

// countSecurityFindings tries to extract finding count from security tool output.
func countSecurityFindings(output string) int {
	// Try JSON output first (many tools support it)
	var jsonResult struct {
		Findings []json.RawMessage `json:"findings"`
		Issues   []json.RawMessage `json:"issues"`
		Results  []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &jsonResult); err == nil {
		n := len(jsonResult.Findings) + len(jsonResult.Issues) + len(jsonResult.Results)
		if n > 0 {
			return n
		}
	}

	// Fall back to counting warning/error lines
	count := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(line, "vulnerability") || strings.Contains(line, "finding") || strings.Contains(line, "issue") {
			count++
		}
	}
	return count
}
