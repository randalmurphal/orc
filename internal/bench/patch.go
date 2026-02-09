package bench

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/util"
)

// PRRef holds parsed components of a GitHub PR URL.
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// ParsePRURL extracts owner, repo, and PR number from a GitHub PR URL.
// Accepts "https://github.com/<owner>/<repo>/pull/<number>".
func ParsePRURL(url string) (*PRRef, error) {
	// Strip trailing slash
	url = strings.TrimRight(url, "/")

	// Try to parse github.com/<owner>/<repo>/pull/<number>
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if after, ok := strings.CutPrefix(url, prefix); ok {
			url = after
			break
		}
	}

	parts := strings.Split(url, "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return nil, fmt.Errorf("invalid PR URL format, expected github.com/<owner>/<repo>/pull/<number>")
	}

	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid PR number %q: %w", parts[3], err)
	}

	return &PRRef{
		Owner:  parts[0],
		Repo:   parts[1],
		Number: num,
	}, nil
}

// DiffBlock represents a single file's portion of a unified diff.
type DiffBlock struct {
	FilePath string // From "+++ b/path"
	Content  string // Full block including "diff --git" header
	IsNew    bool   // True if "--- /dev/null" (new file)
}

// ParseDiffBlocks splits a unified diff into per-file blocks.
// Each block starts with "diff --git a/... b/..." and extends to the next
// diff header or end of string.
func ParseDiffBlocks(diff string) []DiffBlock {
	if diff == "" {
		return nil
	}

	var blocks []DiffBlock
	var current strings.Builder
	var currentPath string
	var isNew bool

	scanner := bufio.NewScanner(strings.NewReader(diff))
	// Increase buffer for large diffs
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			// Save previous block if any
			if current.Len() > 0 && currentPath != "" {
				blocks = append(blocks, DiffBlock{
					FilePath: currentPath,
					Content:  current.String(),
					IsNew:    isNew,
				})
			}
			current.Reset()
			currentPath = ""
			isNew = false
		}

		current.WriteString(line)
		current.WriteByte('\n')

		// Extract file path from +++ line
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			currentPath = after
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			// File deleted — path comes from --- line, but we don't care about deleted files for test patches
			currentPath = "/dev/null"
		}
		if strings.HasPrefix(line, "--- /dev/null") {
			isNew = true
		}
	}

	// Save final block
	if current.Len() > 0 && currentPath != "" {
		blocks = append(blocks, DiffBlock{
			FilePath: currentPath,
			Content:  current.String(),
			IsNew:    isNew,
		})
	}

	return blocks
}

// TestFilePatterns maps project language to patterns that identify test files.
// Simple globs use filepath.Match; directory prefixes use strings.HasPrefix.
var TestFilePatterns = map[string][]string{
	"go":         {"*_test.go"},
	"python":     {"test_*.py", "*_test.py", "conftest.py"},
	"typescript": {"*.test.ts", "*.spec.ts", "*.test.tsx", "*.spec.tsx"},
	"cpp":        {"*_test.cpp", "*_tests.cpp", "test_*.cpp"},
}

// testDirPrefixes are directory prefixes that indicate test directories.
// Files under these paths are always considered test files regardless of name.
var testDirPrefixes = map[string][]string{
	"python":     {"tests/"},
	"typescript": {"__tests__/"},
	"cpp":        {"tests/"},
}

// IsTestFile returns true if the file path matches a test file pattern
// for the given language.
func IsTestFile(filePath, language string) bool {
	base := filepath.Base(filePath)

	// Check directory prefixes first
	if prefixes, ok := testDirPrefixes[language]; ok {
		for _, prefix := range prefixes {
			if strings.HasPrefix(filePath, prefix) {
				return true
			}
		}
	}

	// Check file name patterns
	patterns, ok := TestFilePatterns[language]
	if !ok {
		return false
	}
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// SplitTestPatch separates a full PR diff into test-only and source-only portions.
// Returns the test patch content and lists of test/source file paths.
func SplitTestPatch(diff, language string) (testPatch string, testFiles, sourceFiles []string) {
	blocks := ParseDiffBlocks(diff)

	var testContent strings.Builder
	for _, block := range blocks {
		if block.FilePath == "/dev/null" {
			// Deleted file — skip for test patches
			continue
		}
		if IsTestFile(block.FilePath, language) {
			testContent.WriteString(block.Content)
			testFiles = append(testFiles, block.FilePath)
		} else {
			sourceFiles = append(sourceFiles, block.FilePath)
		}
	}

	return testContent.String(), testFiles, sourceFiles
}

// FetchPRDiff uses `gh pr diff` to get the full unified diff for a PR.
func FetchPRDiff(ref *PRRef) (string, error) {
	cmd := exec.Command("gh", "pr", "diff",
		strconv.Itoa(ref.Number),
		"--repo", fmt.Sprintf("%s/%s", ref.Owner, ref.Repo),
	)

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return "", fmt.Errorf("gh pr diff %s/%s#%d: %s: %w", ref.Owner, ref.Repo, ref.Number, strings.TrimSpace(stderr), err)
	}

	return string(out), nil
}

// ValidatePatch checks that a test patch is structurally valid.
func ValidatePatch(patch string) error {
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("patch is empty")
	}
	if !strings.Contains(patch, "diff --git") {
		return fmt.Errorf("patch has no diff headers")
	}
	if !strings.Contains(patch, "@@") {
		return fmt.Errorf("patch has no hunk headers")
	}
	return nil
}

// DefaultPatchesDir returns ~/.orc/bench/patches/.
func DefaultPatchesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".orc", "bench", "patches"), nil
}

// SavePatch writes a test patch to the patches directory.
func SavePatch(patchesDir, taskID, content string) (string, error) {
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		return "", fmt.Errorf("create patches dir: %w", err)
	}

	path := filepath.Join(patchesDir, taskID+".patch")
	if err := util.AtomicWriteFileString(path, content, 0644); err != nil {
		return "", fmt.Errorf("write patch %s: %w", path, err)
	}

	return path, nil
}

// ExtractionStatus classifies the outcome for a single task.
type ExtractionStatus string

const (
	StatusExtracted     ExtractionStatus = "extracted"
	StatusAlreadyExists ExtractionStatus = "exists"
	StatusNoTests       ExtractionStatus = "no_tests"
	StatusNoURL         ExtractionStatus = "no_url"
	StatusFetchFailed   ExtractionStatus = "fetch_err"
)

// ExtractionResult holds the outcome of extracting a patch for one task.
type ExtractionResult struct {
	TaskID      string
	ProjectID   string
	Status      ExtractionStatus
	PatchPath   string
	TestFiles   []string
	SourceFiles []string
	Error       error
}

// ExtractOptions controls the extraction process.
type ExtractOptions struct {
	Force      bool
	TaskIDs    []string // empty = all tasks
	PatchesDir string
	DryRun     bool
	SuitePath  string // for suite.yaml update
}

// ExtractPatches runs the full extraction pipeline for all tasks.
func ExtractPatches(ctx context.Context, store *Store, projects map[string]*Project, opts ExtractOptions) ([]ExtractionResult, error) {
	tasks, err := store.ListTasks(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Filter to requested tasks if specified
	taskFilter := make(map[string]bool)
	for _, id := range opts.TaskIDs {
		taskFilter[id] = true
	}

	var results []ExtractionResult
	for _, task := range tasks {
		if len(taskFilter) > 0 && !taskFilter[task.ID] {
			continue
		}

		result := extractOne(task, projects, opts)
		results = append(results, result)
	}

	return results, nil
}

// extractOne extracts a test patch for a single task.
func extractOne(task *Task, projects map[string]*Project, opts ExtractOptions) ExtractionResult {
	result := ExtractionResult{
		TaskID:    task.ID,
		ProjectID: task.ProjectID,
	}

	// Check for PR URL
	if task.ReferencePRURL == "" {
		result.Status = StatusNoURL
		return result
	}

	// Check if patch file already exists
	patchPath := filepath.Join(opts.PatchesDir, task.ID+".patch")
	if !opts.Force {
		if _, err := os.Stat(patchPath); err == nil {
			result.Status = StatusAlreadyExists
			result.PatchPath = patchPath
			return result
		}
	}

	// Get project for language
	project, ok := projects[task.ProjectID]
	if !ok {
		result.Status = StatusFetchFailed
		result.Error = fmt.Errorf("unknown project %s", task.ProjectID)
		return result
	}

	// Parse PR URL
	ref, err := ParsePRURL(task.ReferencePRURL)
	if err != nil {
		result.Status = StatusFetchFailed
		result.Error = fmt.Errorf("parse PR URL: %w", err)
		return result
	}

	// Fetch full diff
	diff, err := FetchPRDiff(ref)
	if err != nil {
		result.Status = StatusFetchFailed
		result.Error = err
		return result
	}

	// Split into test/source
	testPatch, testFiles, sourceFiles := SplitTestPatch(diff, project.Language)
	result.TestFiles = testFiles
	result.SourceFiles = sourceFiles

	if testPatch == "" {
		result.Status = StatusNoTests
		return result
	}

	// Validate
	if err := ValidatePatch(testPatch); err != nil {
		result.Status = StatusFetchFailed
		result.Error = fmt.Errorf("validate patch: %w", err)
		return result
	}

	// Save (unless dry run)
	if !opts.DryRun {
		path, err := SavePatch(opts.PatchesDir, task.ID, testPatch)
		if err != nil {
			result.Status = StatusFetchFailed
			result.Error = err
			return result
		}
		result.PatchPath = path
	} else {
		result.PatchPath = patchPath // would-be path
	}

	result.Status = StatusExtracted
	return result
}

// UpdateSuiteYAML adds test_patch_file references to tasks in suite.yaml.
// Uses line-by-line editing to preserve YAML formatting and comments.
func UpdateSuiteYAML(suitePath string, results []ExtractionResult) error {
	// Build set of task IDs that were extracted
	extracted := make(map[string]string) // taskID -> relative patch path
	for _, r := range results {
		if r.Status == StatusExtracted {
			extracted[r.TaskID] = "patches/" + r.TaskID + ".patch"
		}
	}
	if len(extracted) == 0 {
		return nil
	}

	data, err := os.ReadFile(suitePath)
	if err != nil {
		return fmt.Errorf("read suite.yaml: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var output []string
	var currentTaskID string
	hasTestPatchFile := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect task ID lines (e.g., "  - id: bbolt-002")
		if strings.HasPrefix(trimmed, "- id: ") {
			// If we were tracking a task that needs a patch and never found test_patch_file
			if currentTaskID != "" && !hasTestPatchFile {
				if patchRel, ok := extracted[currentTaskID]; ok {
					// Insert test_patch_file before the current line
					// Find the right indentation (match the id line's indentation + 2)
					indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " "))+2)
					output = append(output, indent+`test_patch_file: "`+patchRel+`"`)
				}
			}

			// Parse the new task ID
			idStr := strings.TrimPrefix(trimmed, "- id: ")
			currentTaskID = strings.Trim(idStr, `"' `)
			hasTestPatchFile = false
		}

		// Check if this task already has test_patch_file
		if strings.HasPrefix(trimmed, "test_patch_file:") {
			hasTestPatchFile = true
		}

		output = append(output, line)

		// Insert test_patch_file after reference_pr_url for extracted tasks
		if currentTaskID != "" && !hasTestPatchFile {
			if _, needsPatch := extracted[currentTaskID]; needsPatch {
				if strings.HasPrefix(trimmed, "reference_pr_url:") {
					patchRel := extracted[currentTaskID]
					indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))
					output = append(output, indent+`test_patch_file: "`+patchRel+`"`)
					hasTestPatchFile = true
				}
			}
		}

		_ = i // avoid unused warning
	}

	// Handle last task if it needs a patch
	if currentTaskID != "" && !hasTestPatchFile {
		if patchRel, ok := extracted[currentTaskID]; ok {
			output = append(output, `    test_patch_file: "`+patchRel+`"`)
		}
	}

	result := strings.Join(output, "\n")
	return util.AtomicWriteFileString(suitePath, result, 0644)
}
