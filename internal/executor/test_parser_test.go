package executor

import (
	"testing"
	"time"
)

func TestParseGoTestOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		output        string
		wantPassed    int
		wantFailed    int
		wantSkipped   int
		wantCoverage  float64
		wantFailures  int
		wantFramework string
	}{
		{
			name: "all passing",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSub
--- PASS: TestSub (0.00s)
PASS
ok  	example.com/math	0.005s`,
			wantPassed:    2, // 2 individual tests
			wantFailed:    0,
			wantFramework: "go",
		},
		{
			name: "with failure",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestBroken
    math_test.go:15: expected 4, got 5
--- FAIL: TestBroken (0.00s)
FAIL
FAIL	example.com/math	0.005s`,
			wantPassed:    1,
			wantFailed:    1,
			wantFailures:  1,
			wantFramework: "go",
		},
		{
			name: "with skip",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSkipped
--- SKIP: TestSkipped (0.00s)
    math_test.go:20: skipping in short mode
PASS
ok  	example.com/math	0.005s`,
			wantPassed:    1, // Only count individual test passes
			wantSkipped:   1,
			wantFramework: "go",
		},
		{
			name: "with coverage",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
PASS
coverage: 85.7% of statements
ok  	example.com/math	0.005s`,
			wantPassed:    1, // Only count individual test passes
			wantCoverage:  85.7,
			wantFramework: "go",
		},
		{
			name: "multiple packages summary only",
			output: `ok  	example.com/math	0.005s
ok  	example.com/string	0.003s
FAIL	example.com/broken	0.002s`,
			wantPassed:    0, // Package summary lines don't count as individual tests
			wantFramework: "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseGoTestOutput(tt.output)
			if err != nil {
				t.Fatalf("ParseGoTestOutput() error = %v", err)
			}

			if result.Framework != tt.wantFramework {
				t.Errorf("Framework = %v, want %v", result.Framework, tt.wantFramework)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Failed != tt.wantFailed {
				t.Errorf("Failed = %v, want %v", result.Failed, tt.wantFailed)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
			if result.Coverage != tt.wantCoverage {
				t.Errorf("Coverage = %v, want %v", result.Coverage, tt.wantCoverage)
			}
			if len(result.Failures) != tt.wantFailures {
				t.Errorf("Failures count = %v, want %v", len(result.Failures), tt.wantFailures)
			}
		})
	}
}

func TestParseJestOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		output        string
		wantPassed    int
		wantFailed    int
		wantSkipped   int
		wantFramework string
	}{
		{
			name: "all passing",
			output: `PASS src/math.test.js
  Math operations
    ✓ adds numbers (3 ms)
    ✓ subtracts numbers (1 ms)

Test Suites: 1 passed, 1 total
Tests:       2 passed, 2 total
Time:        1.234 s`,
			wantPassed:    2,
			wantFailed:    0,
			wantFramework: "jest",
		},
		{
			name: "with failures",
			output: `FAIL src/math.test.js
  Math operations
    ✓ adds numbers (3 ms)
    ✕ broken test (5 ms)

  ● Math operations › broken test

    expect(received).toBe(expected)

    Expected: 4
    Received: 5

      at Object.<anonymous> (src/math.test.js:15:12)

Test Suites: 1 failed, 1 total
Tests:       1 failed, 1 passed, 2 total
Time:        1.234 s`,
			wantPassed:    1,
			wantFailed:    1,
			wantFramework: "jest",
		},
		{
			name: "with skipped",
			output: `PASS src/math.test.js
  Math operations
    ✓ adds numbers (3 ms)
    ○ skipped test

Test Suites: 1 passed, 1 total
Tests:       1 skipped, 1 passed, 2 total
Time:        0.5 s`,
			wantPassed:    1,
			wantSkipped:   1,
			wantFramework: "jest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseJestOutput(tt.output)
			if err != nil {
				t.Fatalf("ParseJestOutput() error = %v", err)
			}

			if result.Framework != tt.wantFramework {
				t.Errorf("Framework = %v, want %v", result.Framework, tt.wantFramework)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Failed != tt.wantFailed {
				t.Errorf("Failed = %v, want %v", result.Failed, tt.wantFailed)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
		})
	}
}

func TestParsePytestOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		output        string
		wantPassed    int
		wantFailed    int
		wantSkipped   int
		wantFramework string
	}{
		{
			name: "all passing",
			output: `============================= test session starts ==============================
collected 2 items

test_math.py ..                                                          [100%]

============================== 2 passed in 0.12s ===============================`,
			wantPassed:    2,
			wantFailed:    0,
			wantFramework: "pytest",
		},
		{
			name: "with failure",
			output: `============================= test session starts ==============================
collected 2 items

test_math.py .F                                                          [100%]

=================================== FAILURES ===================================
_________________________ test_broken __________________________

    def test_broken():
>       assert 1 + 1 == 3
E       assert 2 == 3

test_math.py:10: AssertionError
=========================== short test summary info ============================
FAILED test_math.py::test_broken
============================== 1 failed, 1 passed in 0.15s =====================`,
			wantPassed:    1,
			wantFailed:    1,
			wantFramework: "pytest",
		},
		{
			name: "with skipped",
			output: `============================= test session starts ==============================
collected 2 items

test_math.py .s                                                          [100%]

======================= 1 passed, 1 skipped in 0.10s ===========================`,
			wantPassed:    1,
			wantSkipped:   1,
			wantFramework: "pytest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePytestOutput(tt.output)
			if err != nil {
				t.Fatalf("ParsePytestOutput() error = %v", err)
			}

			if result.Framework != tt.wantFramework {
				t.Errorf("Framework = %v, want %v", result.Framework, tt.wantFramework)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Failed != tt.wantFailed {
				t.Errorf("Failed = %v, want %v", result.Failed, tt.wantFailed)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
		})
	}
}

func TestParseTestOutput_AutoDetect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		output        string
		wantFramework string
	}{
		{
			name: "detects go",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
PASS
ok  	example.com/math	0.005s`,
			wantFramework: "go",
		},
		{
			name: "detects jest",
			output: `PASS src/math.test.js
Test Suites: 1 passed, 1 total
Tests:       2 passed, 2 total`,
			wantFramework: "jest",
		},
		{
			name: "detects pytest",
			output: `============================= test session starts ==============================
collected 2 items
test_math.py ..                                                          [100%]
============================== 2 passed in 0.12s ===============================`,
			wantFramework: "pytest",
		},
		{
			name:          "unknown falls back",
			output:        "some random output",
			wantFramework: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTestOutput(tt.output)
			if err != nil {
				t.Fatalf("ParseTestOutput() error = %v", err)
			}

			if result.Framework != tt.wantFramework {
				t.Errorf("Framework = %v, want %v", result.Framework, tt.wantFramework)
			}
		})
	}
}

func TestBuildTestRetryContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		result       *ParsedTestResult
		wantEmpty    bool
		wantContains []string
	}{
		{
			name:      "nil result",
			result:    nil,
			wantEmpty: true,
		},
		{
			name: "no failures",
			result: &ParsedTestResult{
				Passed: 5,
			},
			wantEmpty: true,
		},
		{
			name: "with failures",
			result: &ParsedTestResult{
				Failed: 2,
				Failures: []TestFailure{
					{
						Package: "example.com/math",
						Test:    "TestBroken",
						File:    "math_test.go",
						Line:    15,
						Message: "expected 4, got 5",
					},
				},
			},
			wantContains: []string{
				"## Previous Test Failures",
				"example.com/math.TestBroken",
				"math_test.go:15",
				"expected 4, got 5",
			},
		},
		{
			name: "with coverage",
			result: &ParsedTestResult{
				Failed:   1,
				Coverage: 75.5,
				Failures: []TestFailure{
					{Test: "TestBroken"},
				},
			},
			wantContains: []string{
				"## Coverage",
				"75.5%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTestRetryContext("test", tt.result)

			if tt.wantEmpty && got != "" {
				t.Errorf("BuildTestRetryContext() = %v, want empty", got)
			}

			for _, want := range tt.wantContains {
				if !containsString(got, want) {
					t.Errorf("BuildTestRetryContext() missing %q", want)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParsedTestResult_Duration(t *testing.T) {
	t.Parallel()
	output := `=== RUN   TestAdd
--- PASS: TestAdd (0.50s)
PASS
ok  	example.com/math	(1.234s)`

	result, _ := ParseGoTestOutput(output)

	// Should capture duration
	if result.Duration < time.Second {
		t.Errorf("Duration = %v, want >= 1s", result.Duration)
	}
}

func TestValidateTestResults(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		result            *ParsedTestResult
		coverageThreshold int
		required          bool
		wantValid         bool
		wantReason        string
	}{
		{
			name:              "nil result not required",
			result:            nil,
			coverageThreshold: 0,
			required:          false,
			wantValid:         true,
		},
		{
			name:              "nil result required",
			result:            nil,
			coverageThreshold: 0,
			required:          true,
			wantValid:         false,
			wantReason:        "no test results found",
		},
		{
			name: "tests failed",
			result: &ParsedTestResult{
				Failed: 2,
				Passed: 5,
			},
			coverageThreshold: 0,
			required:          true,
			wantValid:         false,
			wantReason:        "tests failed",
		},
		{
			name: "coverage below threshold",
			result: &ParsedTestResult{
				Passed:   10,
				Coverage: 65.0,
			},
			coverageThreshold: 80,
			required:          true,
			wantValid:         false,
			wantReason:        "coverage below threshold",
		},
		{
			name: "all passing with coverage",
			result: &ParsedTestResult{
				Passed:   10,
				Coverage: 85.0,
			},
			coverageThreshold: 80,
			required:          true,
			wantValid:         true,
		},
		{
			name: "no coverage threshold",
			result: &ParsedTestResult{
				Passed:   10,
				Coverage: 50.0,
			},
			coverageThreshold: 0,
			required:          true,
			wantValid:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateTestResults(tt.result, tt.coverageThreshold, tt.required)

			if got.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestCheckCoverageThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		coverage  float64
		threshold int
		wantPass  bool
	}{
		{
			name:      "no threshold",
			coverage:  50.0,
			threshold: 0,
			wantPass:  true,
		},
		{
			name:      "meets threshold",
			coverage:  85.0,
			threshold: 80,
			wantPass:  true,
		},
		{
			name:      "exactly at threshold",
			coverage:  80.0,
			threshold: 80,
			wantPass:  true,
		},
		{
			name:      "below threshold",
			coverage:  75.0,
			threshold: 80,
			wantPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass, _ := CheckCoverageThreshold(tt.coverage, tt.threshold)
			if pass != tt.wantPass {
				t.Errorf("CheckCoverageThreshold() = %v, want %v", pass, tt.wantPass)
			}
		})
	}
}

func TestShouldSkipTestPhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		weight         string
		skipForWeights []string
		wantSkip       bool
	}{
		{
			name:           "skip trivial",
			weight:         "trivial",
			skipForWeights: []string{"trivial"},
			wantSkip:       true,
		},
		{
			name:           "don't skip medium",
			weight:         "medium",
			skipForWeights: []string{"trivial"},
			wantSkip:       false,
		},
		{
			name:           "empty skip list",
			weight:         "trivial",
			skipForWeights: []string{},
			wantSkip:       false,
		},
		{
			name:           "multiple in skip list",
			weight:         "small",
			skipForWeights: []string{"trivial", "small"},
			wantSkip:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSkipTestPhase(tt.weight, tt.skipForWeights)
			if got != tt.wantSkip {
				t.Errorf("ShouldSkipTestPhase() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}

func TestBuildCoverageRetryContext(t *testing.T) {
	t.Parallel()
	result := &ParsedTestResult{
		Framework: "go",
		Passed:    10,
		Failed:    0,
		Skipped:   2,
	}

	ctx := BuildCoverageRetryContext(65.0, 80, result)

	wantContains := []string{
		"Coverage Below Threshold",
		"65.0%",
		"80%",
		"Framework: go",
		"Passed: 10",
		"Skipped: 2",
	}

	for _, want := range wantContains {
		if !containsString(ctx, want) {
			t.Errorf("BuildCoverageRetryContext() missing %q", want)
		}
	}
}
