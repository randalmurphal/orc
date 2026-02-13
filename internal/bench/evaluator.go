package bench

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// EvalResult holds the automated evaluation results for a run.
type EvalResult struct {
	TestPass     bool
	BuildSuccess bool
	TestOutput   string // Combined stdout+stderr from test command
	BuildOutput  string // Combined stdout+stderr from build command
	Duration     time.Duration
}

// Evaluator runs automated checks against a benchmark workspace.
type Evaluator struct{}

// NewEvaluator creates a new evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// RunAll executes all automated evaluations for a completed run.
// If the task has a TestPatch, it's applied to the worktree before running tests.
// The model never sees the test patch — it's evaluation-only.
func (e *Evaluator) RunAll(workDir string, project *Project, task *Task) (*EvalResult, error) {
	result := &EvalResult{}
	start := time.Now()

	// Apply test patch from reference PR (evaluation-only, model never saw this)
	if task.TestPatch != "" {
		if err := e.applyTestPatch(workDir, task.TestPatch, task.PreFixCommit); err != nil {
			return nil, fmt.Errorf("apply test patch: %w", err)
		}
	}

	// Build check
	if project.BuildCmd != "" {
		result.BuildSuccess, result.BuildOutput = e.runCmd(workDir, project.BuildCmd)
	} else {
		result.BuildSuccess = true
	}

	// Test check — exit code 0 = pass, non-zero = fail
	if project.TestCmd != "" {
		result.TestPass, result.TestOutput = e.runCmd(workDir, project.TestCmd)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// applyTestPatch applies a git diff patch to the worktree.
// This adds the test files from the reference PR so the test suite
// can verify whether the model's source fix actually works.
//
// The model may have modified the same test files (adding its own tests),
// so we first reset those files to the pre-fix commit state, then apply
// the patch cleanly. The model's tests are irrelevant — only the shipped
// PR tests matter for evaluation.
func (e *Evaluator) applyTestPatch(workDir, patch, preFixCommit string) error {
	// Parse which files the patch touches
	files := patchFiles(patch)

	// Reset those files to the pre-fix commit state, discarding any model
	// changes to test files. We use the explicit commit hash because the
	// model likely committed its changes (via Claude Code), moving HEAD.
	if len(files) > 0 {
		args := append([]string{"checkout", preFixCommit, "--"}, files...)
		cmd := exec.Command("git", args...)
		cmd.Dir = workDir
		if err := cmd.Run(); err != nil {
			// File might not exist at pre-fix commit (new file) — that's fine
			_ = err
		}
	}

	// Now apply the patch cleanly onto the original test files
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

// patchFiles extracts file paths from a unified diff patch.
// Looks for "--- a/path" and "+++ b/path" lines.
func patchFiles(patch string) []string {
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(patch))
	for scanner.Scan() {
		line := scanner.Text()
		for _, prefix := range []string{"+++ b/", "--- a/"} {
			if strings.HasPrefix(line, prefix) {
				f := strings.TrimPrefix(line, prefix)
				if f != "/dev/null" && !seen[f] {
					seen[f] = true
				}
			}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	return files
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
