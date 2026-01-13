// Package task provides task management for orc.
package task

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// TestResultsDir is the subdirectory for Playwright test results
	TestResultsDir = "test-results"
	// ScreenshotsSubDir is the subdirectory for screenshots within test-results
	ScreenshotsSubDir = "screenshots"
	// TracesSubDir is the subdirectory for Playwright traces
	TracesSubDir = "traces"
	// ReportFile is the filename for structured test results
	ReportFile = "report.json"
	// HTMLReportFile is the filename for Playwright HTML report
	HTMLReportFile = "index.html"
)

// TestResultStatus represents the overall status of a test run.
type TestResultStatus string

const (
	TestResultStatusPassed  TestResultStatus = "passed"
	TestResultStatusFailed  TestResultStatus = "failed"
	TestResultStatusSkipped TestResultStatus = "skipped"
	TestResultStatusPending TestResultStatus = "pending"
)

// TestResult represents a single test case result.
type TestResult struct {
	// Name is the test name/title
	Name string `json:"name"`

	// Status is the test outcome
	Status TestResultStatus `json:"status"`

	// Duration is the test duration in milliseconds
	Duration int64 `json:"duration"`

	// Error contains failure details if the test failed
	Error string `json:"error,omitempty"`

	// Screenshots lists screenshot filenames taken during this test
	Screenshots []string `json:"screenshots,omitempty"`

	// Trace is the trace file path if available
	Trace string `json:"trace,omitempty"`
}

// TestSuite represents a group of related tests.
type TestSuite struct {
	// Name is the suite name (e.g., file path or describe block)
	Name string `json:"name"`

	// Tests contains individual test results
	Tests []TestResult `json:"tests"`
}

// TestReport represents the complete test run report.
type TestReport struct {
	// Version of the report format
	Version int `json:"version"`

	// Framework used (playwright, jest, vitest, etc.)
	Framework string `json:"framework"`

	// StartedAt is when the test run started
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the test run finished
	CompletedAt time.Time `json:"completed_at"`

	// Duration is the total duration in milliseconds
	Duration int64 `json:"duration"`

	// Summary contains aggregated counts
	Summary TestSummary `json:"summary"`

	// Suites contains test results grouped by suite
	Suites []TestSuite `json:"suites"`

	// Coverage contains code coverage data if available
	Coverage *TestCoverage `json:"coverage,omitempty"`
}

// TestSummary contains aggregated test counts.
type TestSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// TestCoverage contains code coverage information.
type TestCoverage struct {
	// Percentage is the overall coverage percentage
	Percentage float64 `json:"percentage"`

	// Lines contains line coverage details
	Lines *CoverageDetail `json:"lines,omitempty"`

	// Branches contains branch coverage details
	Branches *CoverageDetail `json:"branches,omitempty"`

	// Functions contains function coverage details
	Functions *CoverageDetail `json:"functions,omitempty"`

	// Statements contains statement coverage details
	Statements *CoverageDetail `json:"statements,omitempty"`
}

// CoverageDetail contains coverage counts.
type CoverageDetail struct {
	Total   int     `json:"total"`
	Covered int     `json:"covered"`
	Percent float64 `json:"percent"`
}

// Screenshot represents a test screenshot.
type Screenshot struct {
	// Filename is the screenshot file name
	Filename string `json:"filename"`

	// PageName is the name of the page (extracted from filename)
	PageName string `json:"page_name"`

	// TestName is the associated test name
	TestName string `json:"test_name,omitempty"`

	// Size is the file size in bytes
	Size int64 `json:"size"`

	// CreatedAt is when the screenshot was taken
	CreatedAt time.Time `json:"created_at"`
}

// TestResultsInfo contains a summary of test results for a task.
type TestResultsInfo struct {
	// HasResults indicates if test results exist
	HasResults bool `json:"has_results"`

	// Report contains the test report if available
	Report *TestReport `json:"report,omitempty"`

	// Screenshots lists all available screenshots
	Screenshots []Screenshot `json:"screenshots"`

	// HasTraces indicates if trace files are available
	HasTraces bool `json:"has_traces"`

	// TraceFiles lists trace file names
	TraceFiles []string `json:"trace_files,omitempty"`

	// HasHTMLReport indicates if an HTML report is available
	HasHTMLReport bool `json:"has_html_report"`
}

// TestResultsPath returns the full path to the test-results directory for a task.
func TestResultsPath(projectDir, taskID string) string {
	return filepath.Join(projectDir, OrcDir, TasksDir, taskID, TestResultsDir)
}

// ScreenshotsPath returns the full path to the screenshots directory for a task.
func ScreenshotsPath(projectDir, taskID string) string {
	return filepath.Join(TestResultsPath(projectDir, taskID), ScreenshotsSubDir)
}

// TracesPath returns the full path to the traces directory for a task.
func TracesPath(projectDir, taskID string) string {
	return filepath.Join(TestResultsPath(projectDir, taskID), TracesSubDir)
}

// GetTestResults retrieves test results for a task.
func GetTestResults(projectDir, taskID string) (*TestResultsInfo, error) {
	resultsDir := TestResultsPath(projectDir, taskID)

	info := &TestResultsInfo{
		HasResults:  false,
		Screenshots: []Screenshot{},
	}

	// Check if test-results directory exists
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		return info, nil
	}

	info.HasResults = true

	// Load report.json if it exists
	reportPath := filepath.Join(resultsDir, ReportFile)
	if data, err := os.ReadFile(reportPath); err == nil {
		var report TestReport
		if err := json.Unmarshal(data, &report); err == nil {
			info.Report = &report
		}
	}

	// List screenshots
	screenshotsDir := filepath.Join(resultsDir, ScreenshotsSubDir)
	if entries, err := os.ReadDir(screenshotsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filename := entry.Name()
			contentType := detectContentType(filename)
			if !isImageContentType(contentType) {
				continue
			}

			fileInfo, err := entry.Info()
			if err != nil {
				continue
			}

			screenshot := Screenshot{
				Filename:  filename,
				PageName:  extractPageName(filename),
				Size:      fileInfo.Size(),
				CreatedAt: fileInfo.ModTime(),
			}

			info.Screenshots = append(info.Screenshots, screenshot)
		}

		// Sort by creation time (newest first)
		sort.Slice(info.Screenshots, func(i, j int) bool {
			return info.Screenshots[i].CreatedAt.After(info.Screenshots[j].CreatedAt)
		})
	}

	// Check for traces
	tracesDir := filepath.Join(resultsDir, TracesSubDir)
	if entries, err := os.ReadDir(tracesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info.TraceFiles = append(info.TraceFiles, entry.Name())
			info.HasTraces = true
		}
	}

	// Check for HTML report
	htmlReportPath := filepath.Join(resultsDir, HTMLReportFile)
	if _, err := os.Stat(htmlReportPath); err == nil {
		info.HasHTMLReport = true
	}

	return info, nil
}

// ListScreenshots returns all screenshots for a task.
func ListScreenshots(projectDir, taskID string) ([]Screenshot, error) {
	screenshotsDir := ScreenshotsPath(projectDir, taskID)

	entries, err := os.ReadDir(screenshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Screenshot{}, nil
		}
		return nil, fmt.Errorf("read screenshots directory: %w", err)
	}

	screenshots := []Screenshot{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		contentType := detectContentType(filename)
		if !isImageContentType(contentType) {
			continue
		}

		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		screenshot := Screenshot{
			Filename:  filename,
			PageName:  extractPageName(filename),
			Size:      fileInfo.Size(),
			CreatedAt: fileInfo.ModTime(),
		}

		screenshots = append(screenshots, screenshot)
	}

	// Sort by creation time (newest first)
	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].CreatedAt.After(screenshots[j].CreatedAt)
	})

	return screenshots, nil
}

// GetScreenshot returns a specific screenshot's metadata and reader.
func GetScreenshot(projectDir, taskID, filename string) (*Screenshot, io.ReadCloser, error) {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return nil, nil, fmt.Errorf("invalid filename")
	}

	screenshotPath := filepath.Join(ScreenshotsPath(projectDir, taskID), filename)

	file, err := os.Open(screenshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("screenshot not found")
		}
		return nil, nil, fmt.Errorf("open screenshot: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("stat screenshot: %w", err)
	}

	screenshot := &Screenshot{
		Filename:  filename,
		PageName:  extractPageName(filename),
		Size:      info.Size(),
		CreatedAt: info.ModTime(),
	}

	return screenshot, file, nil
}

// SaveTestReport saves a test report to the task's test-results directory.
func SaveTestReport(projectDir, taskID string, report *TestReport) error {
	resultsDir := TestResultsPath(projectDir, taskID)

	// Create test-results directory if it doesn't exist
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("create test-results directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(resultsDir, ReportFile)
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// SaveScreenshot saves a screenshot to the task's screenshots directory.
func SaveScreenshot(projectDir, taskID, filename string, reader io.Reader) (*Screenshot, error) {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return nil, fmt.Errorf("invalid filename")
	}

	screenshotsDir := ScreenshotsPath(projectDir, taskID)

	// Create screenshots directory if it doesn't exist
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		return nil, fmt.Errorf("create screenshots directory: %w", err)
	}

	screenshotPath := filepath.Join(screenshotsDir, filename)

	// Write to a temp file first for atomic write
	tmpFile, err := os.CreateTemp(screenshotsDir, ".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Copy content to temp file
	size, err := io.Copy(tmpFile, reader)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("write screenshot: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Rename to final location (atomic on POSIX)
	if err := os.Rename(tmpPath, screenshotPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("save screenshot: %w", err)
	}

	return &Screenshot{
		Filename:  filename,
		PageName:  extractPageName(filename),
		Size:      size,
		CreatedAt: time.Now(),
	}, nil
}

// GetHTMLReport returns a reader for the HTML report if it exists.
func GetHTMLReport(projectDir, taskID string) (io.ReadCloser, error) {
	reportPath := filepath.Join(TestResultsPath(projectDir, taskID), HTMLReportFile)

	file, err := os.Open(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("HTML report not found")
		}
		return nil, fmt.Errorf("open HTML report: %w", err)
	}

	return file, nil
}

// GetTrace returns a reader for a trace file if it exists.
func GetTrace(projectDir, taskID, filename string) (io.ReadCloser, error) {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return nil, fmt.Errorf("invalid filename")
	}

	tracePath := filepath.Join(TracesPath(projectDir, taskID), filename)

	file, err := os.Open(tracePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("trace not found")
		}
		return nil, fmt.Errorf("open trace: %w", err)
	}

	return file, nil
}

// extractPageName extracts a readable page name from a screenshot filename.
// Common Playwright screenshot patterns:
// - pagename-1.png
// - Test-Name-pagename.png
// - screenshot-pagename-step1.png
func extractPageName(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Replace common separators with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove common prefixes
	prefixes := []string{"screenshot ", "Screenshot "}
	for _, prefix := range prefixes {
		name = strings.TrimPrefix(name, prefix)
	}

	// Capitalize first letter of each word for readability
	words := strings.Fields(name)
	if len(words) > 0 {
		return strings.Join(words, " ")
	}

	return filename
}

// InitTestResultsDir creates the test-results directory structure for a task.
func InitTestResultsDir(projectDir, taskID string) error {
	dirs := []string{
		TestResultsPath(projectDir, taskID),
		ScreenshotsPath(projectDir, taskID),
		TracesPath(projectDir, taskID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}
