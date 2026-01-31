// Package task provides task management for orc.
package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/util"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// protoJSONMarshaler configures JSON output for test report files.
var protoJSONMarshaler = protojson.MarshalOptions{
	Multiline:       true,
	Indent:          "  ",
	UseProtoNames:   true, // Use snake_case field names in JSON
	UseEnumNumbers:  false,
	EmitUnpopulated: false,
}

// protoJSONUnmarshaler configures JSON input parsing.
var protoJSONUnmarshaler = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

// TestResultsPath returns the full path to the test-results directory for a task.
// During execution, projectDir is typically the worktree path.
func TestResultsPath(projectDir, taskID string) string {
	return filepath.Join(projectDir, ".orc-test-results", taskID)
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
func GetTestResults(projectDir, taskID string) (*orcv1.TestResultsInfo, error) {
	resultsDir := TestResultsPath(projectDir, taskID)

	info := &orcv1.TestResultsInfo{
		HasResults:  false,
		Screenshots: []*orcv1.Screenshot{},
	}

	// Check if test-results directory exists
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		return info, nil
	}

	info.HasResults = true

	// Load report.json if it exists
	reportPath := filepath.Join(resultsDir, ReportFile)
	if data, err := os.ReadFile(reportPath); err == nil {
		var report orcv1.TestReport
		if err := protoJSONUnmarshaler.Unmarshal(data, &report); err == nil {
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
			contentType := DetectContentType(filename)
			if !IsImageContentType(contentType) {
				continue
			}

			fileInfo, err := entry.Info()
			if err != nil {
				continue
			}

			screenshot := &orcv1.Screenshot{
				Filename:  filename,
				PageName:  extractPageName(filename),
				Size:      fileInfo.Size(),
				CreatedAt: timestamppb.New(fileInfo.ModTime()),
			}

			info.Screenshots = append(info.Screenshots, screenshot)
		}

		// Sort by creation time (newest first)
		sort.Slice(info.Screenshots, func(i, j int) bool {
			return info.Screenshots[i].CreatedAt.AsTime().After(info.Screenshots[j].CreatedAt.AsTime())
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
		info.HasHtmlReport = true
	}

	return info, nil
}

// ListScreenshots returns all screenshots for a task.
func ListScreenshots(projectDir, taskID string) ([]*orcv1.Screenshot, error) {
	screenshotsDir := ScreenshotsPath(projectDir, taskID)

	entries, err := os.ReadDir(screenshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*orcv1.Screenshot{}, nil
		}
		return nil, fmt.Errorf("read screenshots directory: %w", err)
	}

	screenshots := []*orcv1.Screenshot{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		contentType := DetectContentType(filename)
		if !IsImageContentType(contentType) {
			continue
		}

		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		screenshot := &orcv1.Screenshot{
			Filename:  filename,
			PageName:  extractPageName(filename),
			Size:      fileInfo.Size(),
			CreatedAt: timestamppb.New(fileInfo.ModTime()),
		}

		screenshots = append(screenshots, screenshot)
	}

	// Sort by creation time (newest first)
	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].CreatedAt.AsTime().After(screenshots[j].CreatedAt.AsTime())
	})

	return screenshots, nil
}

// GetScreenshot returns a specific screenshot's metadata and reader.
func GetScreenshot(projectDir, taskID, filename string) (*orcv1.Screenshot, io.ReadCloser, error) {
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
		_ = file.Close()
		return nil, nil, fmt.Errorf("stat screenshot: %w", err)
	}

	screenshot := &orcv1.Screenshot{
		Filename:  filename,
		PageName:  extractPageName(filename),
		Size:      info.Size(),
		CreatedAt: timestamppb.New(info.ModTime()),
	}

	return screenshot, file, nil
}

// SaveTestReport saves a test report to the task's test-results directory.
func SaveTestReport(projectDir, taskID string, report *orcv1.TestReport) error {
	resultsDir := TestResultsPath(projectDir, taskID)

	// Create test-results directory if it doesn't exist
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("create test-results directory: %w", err)
	}

	data, err := protoJSONMarshaler.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(resultsDir, ReportFile)
	if err := util.AtomicWriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// SaveScreenshot saves a screenshot to the task's screenshots directory.
func SaveScreenshot(projectDir, taskID, filename string, reader io.Reader) (*orcv1.Screenshot, error) {
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
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("write screenshot: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Rename to final location (atomic on POSIX)
	if err := os.Rename(tmpPath, screenshotPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("save screenshot: %w", err)
	}

	return &orcv1.Screenshot{
		Filename:  filename,
		PageName:  extractPageName(filename),
		Size:      size,
		CreatedAt: timestamppb.Now(),
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
