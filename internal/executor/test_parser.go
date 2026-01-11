// Package executor provides phase execution capabilities for orc tasks.
package executor

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedTestResult represents parsed test execution results.
type ParsedTestResult struct {
	// Passed is the number of passed tests
	Passed int `json:"passed"`
	// Failed is the number of failed tests
	Failed int `json:"failed"`
	// Skipped is the number of skipped tests
	Skipped int `json:"skipped"`
	// Coverage is the test coverage percentage (0-100)
	Coverage float64 `json:"coverage"`
	// Failures contains details about each failure
	Failures []TestFailure `json:"failures,omitempty"`
	// Duration is the total test execution time
	Duration time.Duration `json:"duration"`
	// Framework is the detected test framework
	Framework string `json:"framework"`
}

// TestFailure represents a single test failure.
type TestFailure struct {
	// Package is the package/module containing the test
	Package string `json:"package,omitempty"`
	// Test is the name of the failing test
	Test string `json:"test"`
	// Message is the failure message/reason
	Message string `json:"message"`
	// File is the source file containing the test
	File string `json:"file,omitempty"`
	// Line is the line number of the failure
	Line int `json:"line,omitempty"`
	// Output is the raw test output for this failure
	Output string `json:"output,omitempty"`
}

// ParseTestOutput auto-detects the test framework and parses output.
func ParseTestOutput(output string) (*ParsedTestResult, error) {
	// Try Go first (most common in this project)
	if isGoTestOutput(output) {
		return ParseGoTestOutput(output)
	}

	// Try Jest
	if isJestOutput(output) {
		return ParseJestOutput(output)
	}

	// Try Pytest
	if isPytestOutput(output) {
		return ParsePytestOutput(output)
	}

	// Return generic result if no framework detected
	return parseGenericOutput(output), nil
}

// isGoTestOutput checks if output is from go test.
func isGoTestOutput(output string) bool {
	return strings.Contains(output, "--- FAIL:") ||
		strings.Contains(output, "--- PASS:") ||
		strings.Contains(output, "=== RUN") ||
		strings.Contains(output, "PASS") && strings.Contains(output, "ok  \t") ||
		strings.Contains(output, "FAIL") && strings.Contains(output, "FAIL\t")
}

// isJestOutput checks if output is from Jest.
func isJestOutput(output string) bool {
	return strings.Contains(output, "PASS ") && strings.Contains(output, ".test.") ||
		strings.Contains(output, "FAIL ") && strings.Contains(output, ".test.") ||
		strings.Contains(output, "Test Suites:") ||
		strings.Contains(output, "Tests:")
}

// isPytestOutput checks if output is from pytest.
func isPytestOutput(output string) bool {
	return strings.Contains(output, "pytest") ||
		strings.Contains(output, "===") && strings.Contains(output, "passed") ||
		strings.Contains(output, "PASSED") && strings.Contains(output, "::") ||
		strings.Contains(output, "FAILED") && strings.Contains(output, "::")
}

// ParseGoTestOutput parses output from `go test`.
func ParseGoTestOutput(output string) (*ParsedTestResult, error) {
	result := &ParsedTestResult{
		Framework: "go",
	}

	lines := strings.Split(output, "\n")

	// Regex patterns
	failPattern := regexp.MustCompile(`^--- FAIL: (\S+)\s+\(([^)]+)\)`)
	passPattern := regexp.MustCompile(`^--- PASS: (\S+)\s+\(([^)]+)\)`)
	skipPattern := regexp.MustCompile(`^--- SKIP: (\S+)`)
	coverPattern := regexp.MustCompile(`coverage: (\d+\.?\d*)%`)
	locationPattern := regexp.MustCompile(`^\s+(\S+\.go):(\d+):`)
	pkgFailPattern := regexp.MustCompile(`^FAIL\s+(\S+)`)
	pkgPassPattern := regexp.MustCompile(`^ok\s+(\S+)`)
	totalDurationPattern := regexp.MustCompile(`\((\d+\.?\d*)s\)`)

	var currentFailure *TestFailure
	var currentPackage string
	var failureOutput strings.Builder

	for i, line := range lines {
		// Extract package from ok/FAIL lines
		if matches := pkgPassPattern.FindStringSubmatch(line); matches != nil {
			currentPackage = matches[1]
			result.Passed++ // Count package passes
		} else if matches := pkgFailPattern.FindStringSubmatch(line); matches != nil {
			currentPackage = matches[1]
		}

		// Match pass
		if matches := passPattern.FindStringSubmatch(line); matches != nil {
			result.Passed++
			continue
		}

		// Match skip
		if matches := skipPattern.FindStringSubmatch(line); matches != nil {
			result.Skipped++
			continue
		}

		// Match failure start
		if matches := failPattern.FindStringSubmatch(line); matches != nil {
			// Save previous failure
			if currentFailure != nil {
				currentFailure.Output = strings.TrimSpace(failureOutput.String())
				result.Failures = append(result.Failures, *currentFailure)
			}

			result.Failed++
			currentFailure = &TestFailure{
				Test:    matches[1],
				Package: currentPackage,
			}
			failureOutput.Reset()

			// Parse duration
			if dur, err := time.ParseDuration(matches[2]); err == nil {
				result.Duration += dur
			}
			continue
		}

		// Capture failure details
		if currentFailure != nil {
			// Look for file:line reference
			if matches := locationPattern.FindStringSubmatch(line); matches != nil {
				if currentFailure.File == "" {
					currentFailure.File = matches[1]
					lineNum, _ := strconv.Atoi(matches[2])
					currentFailure.Line = lineNum
				}
			}

			// Capture failure message (first non-empty line after failure marker)
			if currentFailure.Message == "" && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "---") {
				currentFailure.Message = strings.TrimSpace(line)
			}

			failureOutput.WriteString(line)
			failureOutput.WriteString("\n")

			// End of failure block
			if i+1 < len(lines) && (strings.HasPrefix(lines[i+1], "---") || strings.HasPrefix(lines[i+1], "===")) {
				currentFailure.Output = strings.TrimSpace(failureOutput.String())
				result.Failures = append(result.Failures, *currentFailure)
				currentFailure = nil
				failureOutput.Reset()
			}
		}

		// Match coverage
		if matches := coverPattern.FindStringSubmatch(line); matches != nil {
			cov, _ := strconv.ParseFloat(matches[1], 64)
			if cov > result.Coverage {
				result.Coverage = cov
			}
		}

		// Match total duration
		if matches := totalDurationPattern.FindStringSubmatch(line); matches != nil {
			dur, _ := strconv.ParseFloat(matches[1], 64)
			result.Duration = time.Duration(dur * float64(time.Second))
		}
	}

	// Don't forget last failure
	if currentFailure != nil {
		currentFailure.Output = strings.TrimSpace(failureOutput.String())
		result.Failures = append(result.Failures, *currentFailure)
	}

	return result, nil
}

// ParseJestOutput parses output from Jest.
func ParseJestOutput(output string) (*ParsedTestResult, error) {
	result := &ParsedTestResult{
		Framework: "jest",
	}

	lines := strings.Split(output, "\n")

	// Regex patterns
	summaryPattern := regexp.MustCompile(`Tests:\s+(?:(\d+) failed,?\s*)?(?:(\d+) skipped,?\s*)?(?:(\d+) passed,?\s*)?(\d+) total`)
	failPattern := regexp.MustCompile(`^\s*●\s+(.+)$`)
	fileLinePattern := regexp.MustCompile(`at\s+.+\((.+):(\d+):(\d+)\)`)
	durationPattern := regexp.MustCompile(`Time:\s+(\d+\.?\d*)\s*(ms|s|m)`)
	coverPattern := regexp.MustCompile(`All files\s+\|\s+(\d+\.?\d*)`)

	var currentFailure *TestFailure
	var failureOutput strings.Builder

	for _, line := range lines {
		// Match summary line
		if matches := summaryPattern.FindStringSubmatch(line); matches != nil {
			if matches[1] != "" {
				result.Failed, _ = strconv.Atoi(matches[1])
			}
			if matches[2] != "" {
				result.Skipped, _ = strconv.Atoi(matches[2])
			}
			if matches[3] != "" {
				result.Passed, _ = strconv.Atoi(matches[3])
			}
			continue
		}

		// Match failure marker
		if matches := failPattern.FindStringSubmatch(line); matches != nil {
			// Save previous failure
			if currentFailure != nil {
				currentFailure.Output = strings.TrimSpace(failureOutput.String())
				result.Failures = append(result.Failures, *currentFailure)
			}

			currentFailure = &TestFailure{
				Test: matches[1],
			}
			failureOutput.Reset()
			continue
		}

		// Capture failure details
		if currentFailure != nil {
			failureOutput.WriteString(line)
			failureOutput.WriteString("\n")

			// Look for file:line reference
			if matches := fileLinePattern.FindStringSubmatch(line); matches != nil {
				if currentFailure.File == "" {
					currentFailure.File = matches[1]
					lineNum, _ := strconv.Atoi(matches[2])
					currentFailure.Line = lineNum
				}
			}

			// Capture first error message (expect/toBe assertions)
			if currentFailure.Message == "" && (strings.Contains(line, "expect") || strings.Contains(line, "Expected") || strings.Contains(line, "Received")) {
				currentFailure.Message = strings.TrimSpace(line)
			}
		}

		// Match duration
		if matches := durationPattern.FindStringSubmatch(line); matches != nil {
			dur, _ := strconv.ParseFloat(matches[1], 64)
			unit := matches[2]
			switch unit {
			case "ms":
				result.Duration = time.Duration(dur * float64(time.Millisecond))
			case "s":
				result.Duration = time.Duration(dur * float64(time.Second))
			case "m":
				result.Duration = time.Duration(dur * float64(time.Minute))
			}
		}

		// Match coverage
		if matches := coverPattern.FindStringSubmatch(line); matches != nil {
			cov, _ := strconv.ParseFloat(matches[1], 64)
			if cov > result.Coverage {
				result.Coverage = cov
			}
		}
	}

	// Don't forget last failure
	if currentFailure != nil {
		currentFailure.Output = strings.TrimSpace(failureOutput.String())
		result.Failures = append(result.Failures, *currentFailure)
	}

	return result, nil
}

// ParsePytestOutput parses output from pytest.
func ParsePytestOutput(output string) (*ParsedTestResult, error) {
	result := &ParsedTestResult{
		Framework: "pytest",
	}

	lines := strings.Split(output, "\n")

	// Regex patterns
	summaryPattern := regexp.MustCompile(`(\d+) passed|(\d+) failed|(\d+) skipped|(\d+) error`)
	failPattern := regexp.MustCompile(`^FAILED\s+(\S+)::(\S+)`)
	fileLinePattern := regexp.MustCompile(`^(\S+\.py):(\d+):`)
	durationPattern := regexp.MustCompile(`in\s+(\d+\.?\d*)(s|ms)`)
	coverPattern := regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+(\d+)%`)

	var currentFailure *TestFailure
	var failureOutput strings.Builder
	inFailureBlock := false

	for _, line := range lines {
		// Match summary line
		if matches := summaryPattern.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, match := range matches {
				if match[1] != "" {
					result.Passed, _ = strconv.Atoi(match[1])
				}
				if match[2] != "" {
					result.Failed, _ = strconv.Atoi(match[2])
				}
				if match[3] != "" {
					result.Skipped, _ = strconv.Atoi(match[3])
				}
			}
		}

		// Match failure start
		if matches := failPattern.FindStringSubmatch(line); matches != nil {
			// Save previous failure
			if currentFailure != nil {
				currentFailure.Output = strings.TrimSpace(failureOutput.String())
				result.Failures = append(result.Failures, *currentFailure)
			}

			currentFailure = &TestFailure{
				Package: matches[1],
				Test:    matches[2],
			}
			failureOutput.Reset()
			inFailureBlock = true
			continue
		}

		// Capture failure details
		if inFailureBlock && currentFailure != nil {
			failureOutput.WriteString(line)
			failureOutput.WriteString("\n")

			// Look for file:line reference
			if matches := fileLinePattern.FindStringSubmatch(line); matches != nil {
				if currentFailure.File == "" {
					currentFailure.File = matches[1]
					lineNum, _ := strconv.Atoi(matches[2])
					currentFailure.Line = lineNum
				}
			}

			// Capture assertion error
			if currentFailure.Message == "" && (strings.Contains(line, "assert") || strings.Contains(line, "Error") || strings.Contains(line, "Exception")) {
				currentFailure.Message = strings.TrimSpace(line)
			}

			// End of failure block
			if strings.HasPrefix(line, "=====") || strings.HasPrefix(line, "_____") {
				currentFailure.Output = strings.TrimSpace(failureOutput.String())
				result.Failures = append(result.Failures, *currentFailure)
				currentFailure = nil
				inFailureBlock = false
			}
		}

		// Match duration
		if matches := durationPattern.FindStringSubmatch(line); matches != nil {
			dur, _ := strconv.ParseFloat(matches[1], 64)
			unit := matches[2]
			if unit == "ms" {
				result.Duration = time.Duration(dur * float64(time.Millisecond))
			} else {
				result.Duration = time.Duration(dur * float64(time.Second))
			}
		}

		// Match coverage
		if matches := coverPattern.FindStringSubmatch(line); matches != nil {
			cov, _ := strconv.ParseFloat(matches[1], 64)
			if cov > result.Coverage {
				result.Coverage = cov
			}
		}
	}

	// Don't forget last failure
	if currentFailure != nil {
		currentFailure.Output = strings.TrimSpace(failureOutput.String())
		result.Failures = append(result.Failures, *currentFailure)
	}

	return result, nil
}

// parseGenericOutput creates a basic result for unknown output.
func parseGenericOutput(output string) *ParsedTestResult {
	result := &ParsedTestResult{
		Framework: "unknown",
	}

	// Simple heuristics
	lowerOutput := strings.ToLower(output)

	if strings.Contains(lowerOutput, "fail") || strings.Contains(lowerOutput, "error") {
		result.Failed = 1
		result.Failures = []TestFailure{{
			Test:    "unknown",
			Message: "Test execution contained failures or errors",
			Output:  output,
		}}
	} else if strings.Contains(lowerOutput, "pass") || strings.Contains(lowerOutput, "ok") {
		result.Passed = 1
	}

	return result
}

// BuildTestRetryContext builds structured retry context from test failures.
func BuildTestRetryContext(phase string, result *ParsedTestResult) string {
	if result == nil || len(result.Failures) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("## Previous Test Failures\n\n")
	b.WriteString("The following tests failed in the previous attempt:\n\n")

	for i, f := range result.Failures {
		if i >= 10 {
			b.WriteString("... and ")
			b.WriteString(strconv.Itoa(len(result.Failures) - 10))
			b.WriteString(" more failures\n")
			break
		}

		b.WriteString("### ")
		if f.Package != "" {
			b.WriteString(f.Package)
			b.WriteString(".")
		}
		b.WriteString(f.Test)
		b.WriteString("\n\n")

		if f.File != "" {
			b.WriteString("**File**: `")
			b.WriteString(f.File)
			if f.Line > 0 {
				b.WriteString(":")
				b.WriteString(strconv.Itoa(f.Line))
			}
			b.WriteString("`\n\n")
		}

		if f.Message != "" {
			b.WriteString("**Error**:\n```\n")
			b.WriteString(f.Message)
			b.WriteString("\n```\n\n")
		}

		if f.Output != "" && len(f.Output) < 1000 {
			b.WriteString("**Output**:\n```\n")
			b.WriteString(f.Output)
			b.WriteString("\n```\n\n")
		}
	}

	if result.Coverage > 0 {
		b.WriteString("\n## Coverage\n\n")
		b.WriteString("Current coverage: ")
		b.WriteString(strconv.FormatFloat(result.Coverage, 'f', 1, 64))
		b.WriteString("%\n")
	}

	return b.String()
}

// TestValidationResult represents the outcome of validating test results.
type TestValidationResult struct {
	// Valid indicates if the test results meet requirements
	Valid bool `json:"valid"`
	// Reason explains why validation failed (if !Valid)
	Reason string `json:"reason,omitempty"`
	// Details contains additional information
	Details map[string]any `json:"details,omitempty"`
}

// ValidateTestResults checks if test results meet the configured requirements.
func ValidateTestResults(result *ParsedTestResult, coverageThreshold int, required bool) *TestValidationResult {
	if result == nil {
		if required {
			return &TestValidationResult{
				Valid:  false,
				Reason: "no test results found",
			}
		}
		return &TestValidationResult{Valid: true}
	}

	// Check for failures
	if result.Failed > 0 {
		return &TestValidationResult{
			Valid:  false,
			Reason: "tests failed",
			Details: map[string]any{
				"failed":    result.Failed,
				"passed":    result.Passed,
				"failures":  result.Failures,
				"framework": result.Framework,
			},
		}
	}

	// Check coverage threshold
	if coverageThreshold > 0 && result.Coverage < float64(coverageThreshold) {
		return &TestValidationResult{
			Valid:  false,
			Reason: "coverage below threshold",
			Details: map[string]any{
				"coverage":   result.Coverage,
				"threshold":  coverageThreshold,
				"difference": float64(coverageThreshold) - result.Coverage,
			},
		}
	}

	// All checks passed
	return &TestValidationResult{
		Valid: true,
		Details: map[string]any{
			"passed":    result.Passed,
			"coverage":  result.Coverage,
			"framework": result.Framework,
		},
	}
}

// CheckCoverageThreshold checks if coverage meets the required threshold.
func CheckCoverageThreshold(coverage float64, threshold int) (bool, string) {
	if threshold <= 0 {
		return true, ""
	}

	if coverage < float64(threshold) {
		return false, "coverage " + strconv.FormatFloat(coverage, 'f', 1, 64) +
			"% below threshold " + strconv.Itoa(threshold) + "%"
	}

	return true, ""
}

// ShouldSkipTestPhase checks if the test phase should be skipped for a given weight.
func ShouldSkipTestPhase(weight string, skipForWeights []string) bool {
	for _, skip := range skipForWeights {
		if weight == skip {
			return true
		}
	}
	return false
}

// BuildCoverageRetryContext builds retry context specifically for coverage failures.
func BuildCoverageRetryContext(coverage float64, threshold int, result *ParsedTestResult) string {
	var b strings.Builder

	b.WriteString("## Coverage Below Threshold\n\n")
	b.WriteString("Current coverage: ")
	b.WriteString(strconv.FormatFloat(coverage, 'f', 1, 64))
	b.WriteString("%\n")
	b.WriteString("Required threshold: ")
	b.WriteString(strconv.Itoa(threshold))
	b.WriteString("%\n\n")
	b.WriteString("Please add more tests to increase code coverage.\n\n")

	if result != nil {
		b.WriteString("### Test Summary\n\n")
		b.WriteString("- Framework: ")
		b.WriteString(result.Framework)
		b.WriteString("\n")
		b.WriteString("- Passed: ")
		b.WriteString(strconv.Itoa(result.Passed))
		b.WriteString("\n")
		b.WriteString("- Failed: ")
		b.WriteString(strconv.Itoa(result.Failed))
		b.WriteString("\n")
		b.WriteString("- Skipped: ")
		b.WriteString(strconv.Itoa(result.Skipped))
		b.WriteString("\n")
	}

	return b.String()
}
