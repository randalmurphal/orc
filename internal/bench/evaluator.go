package bench

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EvalResult holds the automated evaluation results for a run.
type EvalResult struct {
	TestPass         bool
	TestCount        int    // Total tests in post-patch run
	RegressionCount  int    // Failures in pre-patch run (original tests broken by model)
	BuildSuccess     bool
	BuildOutput      string // Combined stdout+stderr from build command
	TestOutput       string // Combined stdout+stderr from post-patch test command
	LintWarnings     int
	LintOutput       string
	SecurityFindings int
	SecurityOutput   string
	Duration         time.Duration
}

// Evaluator runs automated checks against a benchmark workspace.
type Evaluator struct {
	logger *slog.Logger
}

// NewEvaluator creates a new evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{logger: slog.Default()}
}

// RunAll executes all automated evaluations for a completed run.
//
// Evaluation flow:
//  1. Build check (model's code compiles?)
//  2. Lint check (if project.LintCmd configured)
//  3. Security scan (if project.SecurityCmd configured)
//  4. Pre-patch tests (original tests on model's code → regression detection)
//  5. Apply test patch from reference PR
//  6. Post-patch tests (all tests including reference → main pass/fail)
//
// Lint/security run before test patch so they evaluate only the model's code.
// The model never sees the test patch — it's evaluation-only.
func (e *Evaluator) RunAll(workDir string, project *Project, task *Task) (*EvalResult, error) {
	result := &EvalResult{}
	start := time.Now()

	// Step 1: Build check
	if project.BuildCmd != "" {
		result.BuildSuccess, result.BuildOutput = e.runCmd(workDir, project.BuildCmd)
	} else {
		result.BuildSuccess = true
	}

	// Step 2: Lint check (model's code only, before test patch)
	if project.LintCmd != "" {
		_, result.LintOutput = e.runCmd(workDir, project.LintCmd)
		result.LintWarnings = countOutputLines(result.LintOutput)
	}

	// Step 3: Security scan (model's code only, before test patch)
	if project.SecurityCmd != "" {
		_, result.SecurityOutput = e.runCmd(workDir, project.SecurityCmd)
		result.SecurityFindings = countOutputLines(result.SecurityOutput)
	}

	// Step 4: Pre-patch tests (regression detection)
	// Run original tests against model's code. Failures here = model broke
	// something that was working at the pre-fix commit.
	if result.BuildSuccess && project.TestCmd != "" {
		prePatchPass, prePatchOutput := e.runCmd(workDir, project.TestCmd)
		if !prePatchPass {
			_, failures := parseTestCounts(prePatchOutput, project.Language)
			result.RegressionCount = failures
			// If we can't parse individual failures but the run failed,
			// count at least 1 regression.
			if result.RegressionCount == 0 {
				result.RegressionCount = 1
			}
		}
	}

	// Step 5: Apply test patch from reference PR
	if task.TestPatch != "" {
		if err := e.applyTestPatch(workDir, task.TestPatch, task.PreFixCommit); err != nil {
			e.logger.Warn("apply test patch failed", "error", err)
			// Don't bail — still report what we have. TestPass stays false.
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	// Step 6: Post-patch tests (main evaluation)
	// Skip if build failed — test command will just repeat build errors.
	if result.BuildSuccess && project.TestCmd != "" {
		result.TestPass, result.TestOutput = e.runCmd(workDir, project.TestCmd)
		total, _ := parseTestCounts(result.TestOutput, project.Language)
		result.TestCount = total
	}

	result.Duration = time.Since(start)
	return result, nil
}

// applyTestPatch applies a git diff patch to the worktree.
// This adds the test files from the reference PR so the test suite
// can verify whether the model's source fix actually works.
//
// The model may have modified the same test files (adding its own tests),
// so we first reset modified files to the pre-fix commit state, then remove
// any model-created files that collide with new files in the patch, and
// finally apply the patch cleanly.
func (e *Evaluator) applyTestPatch(workDir, patch, preFixCommit string) error {
	files := patchFileInfo(patch)

	// Reset modified files to pre-fix state (per-file, not batch).
	// These are files that existed at preFixCommit and may have been changed by the model.
	for _, f := range files.modified {
		cmd := exec.Command("git", "checkout", preFixCommit, "--", f)
		cmd.Dir = workDir
		if err := cmd.Run(); err != nil {
			e.logger.Debug("checkout reset failed (file may not exist at pre-fix)", "file", f, "error", err)
		}
	}

	// Remove model's versions of new files so git apply can create them.
	// These are files that don't exist at preFixCommit (--- /dev/null in patch)
	// but the model may have created at the same path.
	for _, f := range files.created {
		path := filepath.Join(workDir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			e.logger.Debug("remove new file collision failed", "file", f, "error", err)
		}
	}

	// Apply the patch
	cmd := exec.Command("git", "apply", "--allow-empty", "-")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(patch)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// patchFileSet categorizes files in a unified diff patch.
type patchFileSet struct {
	modified []string // Files that exist at preFixCommit (--- a/path)
	created  []string // New files (--- /dev/null → +++ b/path)
}

// patchFileInfo parses a unified diff to classify files as modified or created.
// A file is "created" when its --- line is /dev/null. Otherwise it's "modified".
func patchFileInfo(patch string) patchFileSet {
	var result patchFileSet
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(patch))
	var prevLine string

	for scanner.Scan() {
		line := scanner.Text()

		// When we see "+++ b/path", check the previous "---" line
		// to determine if this is a new file or modification.
		if f, ok := strings.CutPrefix(line, "+++ b/"); ok {
			if f == "/dev/null" || seen[f] {
				prevLine = line
				continue
			}
			seen[f] = true

			if prevLine == "--- /dev/null" {
				result.created = append(result.created, f)
			} else {
				result.modified = append(result.modified, f)
			}
		}

		prevLine = line
	}

	return result
}

// parseTestCounts extracts test counts from command output.
// Best-effort: returns (0, 0) if the format isn't recognized. Never errors.
func parseTestCounts(output, language string) (total, failures int) {
	switch strings.ToLower(language) {
	case "go", "golang":
		return parseGoTestCounts(output)
	case "python":
		return parsePytestCounts(output)
	case "typescript", "javascript":
		return parseJestCounts(output)
	case "c++", "cpp", "c":
		return parseCTestCounts(output)
	default:
		return 0, 0
	}
}

// Go test output patterns:
//
//	--- PASS: TestFoo (0.01s)
//	--- FAIL: TestBar (0.02s)
//	ok  	go.etcd.io/bbolt	282.758s
//	FAIL	go.etcd.io/bbolt [build failed]
var (
	goTestResultRe = regexp.MustCompile(`^--- (PASS|FAIL): `)
	goPackageOkRe  = regexp.MustCompile(`^ok\s+\S+`)
	goPackageFailRe = regexp.MustCompile(`^FAIL\s+\S+`)
)

func parseGoTestCounts(output string) (total, failures int) {
	var passes, fails int

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if m := goTestResultRe.FindStringSubmatch(line); len(m) > 1 {
			if m[1] == "PASS" {
				passes++
			} else {
				fails++
			}
		}
	}

	// If we found individual test results, use them
	if passes+fails > 0 {
		return passes + fails, fails
	}

	// Fallback: count package-level results (less granular but still useful)
	scanner = bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if goPackageOkRe.MatchString(line) {
			passes++
		} else if goPackageFailRe.MatchString(line) {
			fails++
		}
	}

	return passes + fails, fails
}

// pytest output: "====== 15 passed, 3 failed in 2.53s ======"
// Also handles: "15 passed in 2.53s" (no failures)
var pytestSummaryRe = regexp.MustCompile(`(\d+) passed(?:.*?(\d+) failed)?`)

func parsePytestCounts(output string) (total, failures int) {
	// Scan from the end — summary is the last line
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if m := pytestSummaryRe.FindStringSubmatch(lines[i]); len(m) > 1 {
			passed, _ := strconv.Atoi(m[1])
			failed := 0
			if m[2] != "" {
				failed, _ = strconv.Atoi(m[2])
			}
			return passed + failed, failed
		}
	}
	return 0, 0
}

// jest/vitest output: "Tests:  3 failed, 42 passed, 45 total"
// Also: "Tests:  45 passed, 45 total"
var jestSummaryRe = regexp.MustCompile(`Tests:\s+(?:(\d+) failed,\s+)?(\d+) passed,\s+(\d+) total`)

func parseJestCounts(output string) (total, failures int) {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if m := jestSummaryRe.FindStringSubmatch(lines[i]); len(m) > 1 {
			failed := 0
			if m[1] != "" {
				failed, _ = strconv.Atoi(m[1])
			}
			t, _ := strconv.Atoi(m[3])
			return t, failed
		}
	}
	return 0, 0
}

// ctest output: "100% tests passed, 0 tests failed out of 45"
var ctestSummaryRe = regexp.MustCompile(`(\d+) tests? failed out of (\d+)`)

func parseCTestCounts(output string) (total, failures int) {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if m := ctestSummaryRe.FindStringSubmatch(lines[i]); len(m) > 1 {
			failed, _ := strconv.Atoi(m[1])
			t, _ := strconv.Atoi(m[2])
			return t, failed
		}
	}
	return 0, 0
}

// countOutputLines counts non-empty, non-noise lines in tool output.
// Used for lint warnings and security findings where each finding is a line.
func countOutputLines(output string) int {
	if output == "" {
		return 0
	}

	count := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Skip common noise lines from linters
		if isLintNoise(line) {
			continue
		}
		count++
	}
	return count
}

// isLintNoise returns true for lines that aren't actual findings.
var lintNoisePatterns = []string{
	"level=info",
	"level=warn msg=\"",
	"Run Time:",
	"Total:",
	"Linters:",
}

func isLintNoise(line string) bool {
	for _, pattern := range lintNoisePatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// runCmd executes a shell command and returns (success, output).
// Output is combined stdout+stderr, truncated to 64KB to avoid bloating the DB.
func (e *Evaluator) runCmd(workDir, cmdStr string) (bool, string) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = workDir
	// Disable Go test caching so each trial's code is actually tested.
	// Without this, Go reuses cached results across worktrees via ~/.cache/go-build/,
	// meaning trial N+1 may report "pass (cached)" without testing its actual code.
	cmd.Env = append(os.Environ(), "GOFLAGS=-count=1")
	out, err := cmd.CombinedOutput()

	output := string(out)
	const maxOutput = 64 * 1024
	if len(output) > maxOutput {
		output = output[len(output)-maxOutput:]
		output = "... (truncated)\n" + output
	}

	return err == nil, output
}
