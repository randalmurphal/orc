package task

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTestResultsPath(t *testing.T) {
	result := TestResultsPath("/project", "TASK-001")
	expected := filepath.Join("/project", ".orc-test-results", "TASK-001")
	if result != expected {
		t.Errorf("TestResultsPath() = %q, want %q", result, expected)
	}
}

func TestScreenshotsPath(t *testing.T) {
	result := ScreenshotsPath("/project", "TASK-001")
	expected := filepath.Join("/project", ".orc-test-results", "TASK-001", ScreenshotsSubDir)
	if result != expected {
		t.Errorf("ScreenshotsPath() = %q, want %q", result, expected)
	}
}

func TestTracesPath(t *testing.T) {
	result := TracesPath("/project", "TASK-001")
	expected := filepath.Join("/project", ".orc-test-results", "TASK-001", TracesSubDir)
	if result != expected {
		t.Errorf("TracesPath() = %q, want %q", result, expected)
	}
}

func TestGetTestResults_NoResults(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Don't create the test-results directory - verify GetTestResults
	// handles the case where no results directory exists at all.

	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if info.HasResults {
		t.Error("HasResults should be false when no test-results directory exists")
	}

	if len(info.Screenshots) != 0 {
		t.Errorf("Screenshots should be empty, got %d", len(info.Screenshots))
	}

	if info.HasTraces {
		t.Error("HasTraces should be false")
	}

	if info.HasHtmlReport {
		t.Error("HasHtmlReport should be false")
	}
}

func TestGetTestResults_WithReport(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create test-results directory with report
	resultsDir := TestResultsPath(tmpDir, taskID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create and save report using proto types
	report := &orcv1.TestReport{
		Version:     1,
		Framework:   "playwright",
		StartedAt:   timestamppb.Now(),
		CompletedAt: timestamppb.Now(),
		DurationMs:  60000,
		Summary: &orcv1.TestSummary{
			Total:   10,
			Passed:  8,
			Failed:  1,
			Skipped: 1,
		},
		Suites: []*orcv1.TestSuite{
			{
				Name: "example.spec.ts",
				Tests: []*orcv1.TestResult{
					{Name: "test 1", Status: orcv1.TestResultStatus_TEST_RESULT_STATUS_PASSED, DurationMs: 1000},
					{Name: "test 2", Status: orcv1.TestResultStatus_TEST_RESULT_STATUS_FAILED, DurationMs: 2000, Error: strPtr("assertion failed")},
				},
			},
		},
	}

	if err := SaveTestReport(tmpDir, taskID, report); err != nil {
		t.Fatal(err)
	}

	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if !info.HasResults {
		t.Error("HasResults should be true")
	}

	if info.Report == nil {
		t.Fatal("Report should not be nil")
	}

	if info.Report.Framework != "playwright" {
		t.Errorf("Framework = %q, want %q", info.Report.Framework, "playwright")
	}

	if info.Report.Summary.Passed != 8 {
		t.Errorf("Summary.Passed = %d, want 8", info.Report.Summary.Passed)
	}
}

func TestGetTestResults_WithScreenshots(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create screenshots directory
	screenshotsDir := ScreenshotsPath(tmpDir, taskID)
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test screenshots
	screenshots := []string{"homepage.png", "dashboard-1.png", "login-page.jpg"}
	for _, name := range screenshots {
		if err := os.WriteFile(filepath.Join(screenshotsDir, name), []byte("fake image data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-image file (should be excluded)
	if err := os.WriteFile(filepath.Join(screenshotsDir, "data.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if !info.HasResults {
		t.Error("HasResults should be true")
	}

	if len(info.Screenshots) != len(screenshots) {
		t.Errorf("Screenshots count = %d, want %d", len(info.Screenshots), len(screenshots))
	}
}

func TestGetTestResults_WithTraces(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create traces directory
	tracesDir := TracesPath(tmpDir, taskID)
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create trace files
	traces := []string{"trace-1.zip", "trace-2.zip"}
	for _, name := range traces {
		if err := os.WriteFile(filepath.Join(tracesDir, name), []byte("fake trace data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if !info.HasTraces {
		t.Error("HasTraces should be true")
	}

	if len(info.TraceFiles) != len(traces) {
		t.Errorf("TraceFiles count = %d, want %d", len(info.TraceFiles), len(traces))
	}
}

func TestGetTestResults_WithHTMLReport(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create test-results directory with HTML report
	resultsDir := TestResultsPath(tmpDir, taskID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create index.html
	if err := os.WriteFile(filepath.Join(resultsDir, HTMLReportFile), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if !info.HasHtmlReport {
		t.Error("HasHtmlReport should be true")
	}
}

func TestListScreenshots_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// No screenshots directory
	screenshots, err := ListScreenshots(tmpDir, taskID)
	if err != nil {
		t.Fatalf("ListScreenshots() error = %v", err)
	}

	if len(screenshots) != 0 {
		t.Errorf("Screenshots count = %d, want 0", len(screenshots))
	}
}

func TestListScreenshots_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create screenshots directory
	screenshotsDir := ScreenshotsPath(tmpDir, taskID)
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create screenshots
	if err := os.WriteFile(filepath.Join(screenshotsDir, "test-1.png"), []byte("PNG data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(screenshotsDir, "test-2.jpg"), []byte("JPEG data"), 0644); err != nil {
		t.Fatal(err)
	}

	screenshots, err := ListScreenshots(tmpDir, taskID)
	if err != nil {
		t.Fatalf("ListScreenshots() error = %v", err)
	}

	if len(screenshots) != 2 {
		t.Errorf("Screenshots count = %d, want 2", len(screenshots))
	}
}

func TestGetScreenshot(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create screenshot
	screenshotsDir := ScreenshotsPath(tmpDir, taskID)
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("PNG image data")
	if err := os.WriteFile(filepath.Join(screenshotsDir, "test.png"), content, 0644); err != nil {
		t.Fatal(err)
	}

	screenshot, reader, err := GetScreenshot(tmpDir, taskID, "test.png")
	if err != nil {
		t.Fatalf("GetScreenshot() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	if screenshot.Filename != "test.png" {
		t.Errorf("Filename = %q, want %q", screenshot.Filename, "test.png")
	}

	if screenshot.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", screenshot.Size, len(content))
	}

	// Read and verify content
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("Read content does not match")
	}
}

func TestGetScreenshot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create screenshots directory but no file
	screenshotsDir := ScreenshotsPath(tmpDir, taskID)
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, _, err := GetScreenshot(tmpDir, taskID, "nonexistent.png")
	if err == nil {
		t.Error("GetScreenshot() should fail for nonexistent file")
	}
}

func TestGetScreenshot_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.png",
		"..",
		".",
	}

	for _, filename := range invalidFilenames {
		_, _, err := GetScreenshot(tmpDir, taskID, filename)
		if err == nil {
			t.Errorf("GetScreenshot(%q) should fail", filename)
		}
	}
}

func TestSaveTestReport(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc-test-results", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	report := &orcv1.TestReport{
		Version:   1,
		Framework: "playwright",
		Summary: &orcv1.TestSummary{
			Total:  5,
			Passed: 5,
		},
	}

	if err := SaveTestReport(tmpDir, taskID, report); err != nil {
		t.Fatalf("SaveTestReport() error = %v", err)
	}

	// Verify report was saved by reading it back
	info, err := GetTestResults(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetTestResults() error = %v", err)
	}

	if info.Report == nil {
		t.Fatal("Report should not be nil")
	}

	if info.Report.Framework != "playwright" {
		t.Errorf("Framework = %q, want %q", info.Report.Framework, "playwright")
	}
}

func TestSaveScreenshot(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc-test-results", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("PNG image data")
	reader := bytes.NewReader(content)

	screenshot, err := SaveScreenshot(tmpDir, taskID, "test.png", reader)
	if err != nil {
		t.Fatalf("SaveScreenshot() error = %v", err)
	}

	if screenshot.Filename != "test.png" {
		t.Errorf("Filename = %q, want %q", screenshot.Filename, "test.png")
	}

	if screenshot.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", screenshot.Size, len(content))
	}

	// Verify file was created
	savedPath := filepath.Join(ScreenshotsPath(tmpDir, taskID), "test.png")
	savedContent, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if !bytes.Equal(savedContent, content) {
		t.Error("Saved content does not match original")
	}
}

func TestSaveScreenshot_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc-test-results", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.png",
		"..",
		".",
	}

	for _, filename := range invalidFilenames {
		reader := bytes.NewReader([]byte("test"))
		_, err := SaveScreenshot(tmpDir, taskID, filename, reader)
		if err == nil {
			t.Errorf("SaveScreenshot(%q) should fail", filename)
		}
	}
}

func TestGetHTMLReport(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create HTML report
	resultsDir := TestResultsPath(tmpDir, taskID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("<html><body>Test Report</body></html>")
	if err := os.WriteFile(filepath.Join(resultsDir, HTMLReportFile), content, 0644); err != nil {
		t.Fatal(err)
	}

	reader, err := GetHTMLReport(tmpDir, taskID)
	if err != nil {
		t.Fatalf("GetHTMLReport() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("Read content does not match")
	}
}

func TestGetHTMLReport_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create test-results directory but no HTML report
	resultsDir := TestResultsPath(tmpDir, taskID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := GetHTMLReport(tmpDir, taskID)
	if err == nil {
		t.Error("GetHTMLReport() should fail when report doesn't exist")
	}
}

func TestGetTrace(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create trace file
	tracesDir := TracesPath(tmpDir, taskID)
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("trace zip data")
	if err := os.WriteFile(filepath.Join(tracesDir, "trace.zip"), content, 0644); err != nil {
		t.Fatal(err)
	}

	reader, err := GetTrace(tmpDir, taskID, "trace.zip")
	if err != nil {
		t.Fatalf("GetTrace() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("Read content does not match")
	}
}

func TestGetTrace_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.zip",
		"..",
		".",
	}

	for _, filename := range invalidFilenames {
		_, err := GetTrace(tmpDir, taskID, filename)
		if err == nil {
			t.Errorf("GetTrace(%q) should fail", filename)
		}
	}
}

func TestExtractPageName(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"homepage.png", "homepage"},
		{"dashboard-1.png", "dashboard 1"},
		{"login_page.png", "login page"},
		{"screenshot-test-page.png", "test page"},
		{"Screenshot-test.png", "test"},
		{"test.png", "test"},
	}

	for _, tt := range tests {
		result := extractPageName(tt.filename)
		if result != tt.expected {
			t.Errorf("extractPageName(%q) = %q, want %q", tt.filename, result, tt.expected)
		}
	}
}

func TestInitTestResultsDir(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc-test-results", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := InitTestResultsDir(tmpDir, taskID); err != nil {
		t.Fatalf("InitTestResultsDir() error = %v", err)
	}

	// Verify directories were created
	dirs := []string{
		TestResultsPath(tmpDir, taskID),
		ScreenshotsPath(tmpDir, taskID),
		TracesPath(tmpDir, taskID),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %q was not created", dir)
		}
	}
}

func TestTestResultStatus_ProtoValues(t *testing.T) {
	// Verify proto status enum values
	statuses := []orcv1.TestResultStatus{
		orcv1.TestResultStatus_TEST_RESULT_STATUS_PASSED,
		orcv1.TestResultStatus_TEST_RESULT_STATUS_FAILED,
		orcv1.TestResultStatus_TEST_RESULT_STATUS_SKIPPED,
	}

	expected := []string{"TEST_RESULT_STATUS_PASSED", "TEST_RESULT_STATUS_FAILED", "TEST_RESULT_STATUS_SKIPPED"}

	for i, status := range statuses {
		if status.String() != expected[i] {
			t.Errorf("Status %d = %q, want %q", i, status.String(), expected[i])
		}
	}
}

func TestTestCoverage_Proto(t *testing.T) {
	coverage := &orcv1.TestCoverage{
		Percentage: 85.5,
		Lines: &orcv1.CoverageDetail{
			Total:   100,
			Covered: 85,
		},
		Branches: &orcv1.CoverageDetail{
			Total:   50,
			Covered: 40,
		},
	}

	if coverage.Percentage != 85.5 {
		t.Errorf("Percentage = %f, want 85.5", coverage.Percentage)
	}

	if coverage.Lines == nil || coverage.Lines.Covered != 85 {
		t.Error("Lines coverage not correctly set")
	}

	if coverage.Branches == nil || coverage.Branches.Covered != 40 {
		t.Error("Branches coverage not correctly set")
	}
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}
