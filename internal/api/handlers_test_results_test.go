package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

// === Test Results API Tests ===

func TestGetTestResultsEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/test-results", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTestResultsEndpoint_NoResults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-001")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-001
title: Test Results Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-001/test-results", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var results task.TestResultsInfo
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if results.Report != nil {
		t.Error("expected no report")
	}
	if len(results.Screenshots) != 0 {
		t.Errorf("expected 0 screenshots, got %d", len(results.Screenshots))
	}
}

func TestGetTestResultsEndpoint_WithReport(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with test results
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-002")
	testResultsDir := filepath.Join(taskDir, "test-results")
	os.MkdirAll(testResultsDir, 0755)

	taskYAML := `id: TASK-TR-002
title: Test Results Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create test report with proper structure
	report := task.TestReport{
		Version:   1,
		Framework: "playwright",
		Duration:  5000,
		Summary: task.TestSummary{
			Total:   10,
			Passed:  8,
			Failed:  1,
			Skipped: 1,
		},
	}
	reportBytes, _ := json.Marshal(report)
	os.WriteFile(filepath.Join(testResultsDir, "report.json"), reportBytes, 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-002/test-results", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var results task.TestResultsInfo
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if results.Report == nil {
		t.Fatal("expected report to be present")
	}
	if results.Report.Summary.Total != 10 {
		t.Errorf("expected 10 total tests, got %d", results.Report.Summary.Total)
	}
	if results.Report.Summary.Passed != 8 {
		t.Errorf("expected 8 passed tests, got %d", results.Report.Summary.Passed)
	}
	if results.Report.Summary.Failed != 1 {
		t.Errorf("expected 1 failed test, got %d", results.Report.Summary.Failed)
	}
}

func TestListScreenshotsEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/test-results/screenshots", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListScreenshotsEndpoint_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-003")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-003
title: Screenshot Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-003/test-results/screenshots", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var screenshots []task.Screenshot
	if err := json.NewDecoder(w.Body).Decode(&screenshots); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(screenshots) != 0 {
		t.Errorf("expected 0 screenshots, got %d", len(screenshots))
	}
}

func TestListScreenshotsEndpoint_WithScreenshots(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with screenshots
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-004")
	screenshotsDir := filepath.Join(taskDir, "test-results", "screenshots")
	os.MkdirAll(screenshotsDir, 0755)

	taskYAML := `id: TASK-TR-004
title: Screenshot Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create test screenshots
	os.WriteFile(filepath.Join(screenshotsDir, "login-page.png"), []byte("PNG content"), 0644)
	os.WriteFile(filepath.Join(screenshotsDir, "dashboard.png"), []byte("PNG content 2"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-004/test-results/screenshots", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var screenshots []task.Screenshot
	if err := json.NewDecoder(w.Body).Decode(&screenshots); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestGetScreenshotEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with screenshot
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-005")
	screenshotsDir := filepath.Join(taskDir, "test-results", "screenshots")
	os.MkdirAll(screenshotsDir, 0755)

	taskYAML := `id: TASK-TR-005
title: Screenshot Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create test screenshot
	content := []byte("PNG content")
	os.WriteFile(filepath.Join(screenshotsDir, "test.png"), content, 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-005/test-results/screenshots/test.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected content-type image/png, got %s", ct)
	}

	if w.Body.String() != string(content) {
		t.Error("body content mismatch")
	}
}

func TestGetScreenshotEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-006")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-006
title: Screenshot Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-006/test-results/screenshots/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetScreenshotEndpoint_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-007")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-007
title: Screenshot Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Test with encoded path traversal attempt
	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-007/test-results/screenshots/..%2F..%2Ftask.yaml", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// After sanitization with filepath.Base, the filename becomes "task.yaml" which doesn't exist
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for path traversal attempt, got %d", w.Code)
	}
}

func TestUploadScreenshotEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-008")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-008
title: Screenshot Upload Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile("file", "screenshot.png")
	part.Write([]byte("fake PNG content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-008/test-results/screenshots", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var screenshot task.Screenshot
	if err := json.NewDecoder(w.Body).Decode(&screenshot); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if screenshot.Filename != "screenshot.png" {
		t.Errorf("expected filename screenshot.png, got %s", screenshot.Filename)
	}
}

func TestUploadScreenshotEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/test-results/screenshots", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUploadScreenshotEndpoint_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-009")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-009
title: Screenshot Upload Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-009/test-results/screenshots", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSaveTestReportEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-010")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-010
title: Test Report Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	report := task.TestReport{
		Version:   1,
		Framework: "playwright",
		Duration:  1000,
		Summary: task.TestSummary{
			Total:   5,
			Passed:  5,
			Failed:  0,
			Skipped: 0,
		},
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-010/test-results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify report was saved
	savedReport, _ := os.ReadFile(filepath.Join(taskDir, "test-results", "report.json"))
	if len(savedReport) == 0 {
		t.Error("expected report.json to be saved")
	}
}

func TestSaveTestReportEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	report := task.TestReport{Summary: task.TestSummary{Total: 5}}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/test-results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSaveTestReportEndpoint_InvalidBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-011")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-011
title: Test Report Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-011/test-results", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestInitTestResultsEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-012")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-012
title: Init Test Results Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-012/test-results/init", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "initialized" {
		t.Errorf("expected status initialized, got %s", resp["status"])
	}
	if resp["path"] == "" {
		t.Error("expected path to be returned")
	}

	// Verify directories were created
	if _, err := os.Stat(filepath.Join(taskDir, "test-results", "screenshots")); os.IsNotExist(err) {
		t.Error("expected screenshots directory to be created")
	}
	if _, err := os.Stat(filepath.Join(taskDir, "test-results", "traces")); os.IsNotExist(err) {
		t.Error("expected traces directory to be created")
	}
}

func TestInitTestResultsEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/test-results/init", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetHTMLReportEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with HTML report
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-013")
	testResultsDir := filepath.Join(taskDir, "test-results")
	os.MkdirAll(testResultsDir, 0755)

	taskYAML := `id: TASK-TR-013
title: HTML Report Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	htmlContent := "<html><body>Test Report</body></html>"
	os.WriteFile(filepath.Join(testResultsDir, "index.html"), []byte(htmlContent), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-013/test-results/report", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected content-type text/html, got %s", ct)
	}

	if w.Body.String() != htmlContent {
		t.Error("body content mismatch")
	}
}

func TestGetHTMLReportEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory without HTML report
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-014")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-014
title: HTML Report Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-014/test-results/report", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTraceEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with trace file
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-015")
	tracesDir := filepath.Join(taskDir, "test-results", "traces")
	os.MkdirAll(tracesDir, 0755)

	taskYAML := `id: TASK-TR-015
title: Trace Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	traceContent := []byte("fake zip content")
	os.WriteFile(filepath.Join(tracesDir, "trace.zip"), traceContent, 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-015/test-results/traces/trace.zip", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected content-type application/zip, got %s", ct)
	}

	if w.Body.String() != string(traceContent) {
		t.Error("body content mismatch")
	}
}

func TestGetTraceEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory without trace
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-016")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-016
title: Trace Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-016/test-results/traces/nonexistent.zip", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTraceEndpoint_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-017")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-TR-017
title: Trace Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Test with encoded path traversal attempt
	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-017/test-results/traces/..%2F..%2Ftask.yaml", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// After sanitization with filepath.Base, the filename becomes "task.yaml" which doesn't exist
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for path traversal attempt, got %d", w.Code)
	}
}

func TestGetTestResultsEndpoint_WithScreenshotsAndTraces(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with full test results
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-018")
	testResultsDir := filepath.Join(taskDir, "test-results")
	screenshotsDir := filepath.Join(testResultsDir, "screenshots")
	tracesDir := filepath.Join(testResultsDir, "traces")
	os.MkdirAll(screenshotsDir, 0755)
	os.MkdirAll(tracesDir, 0755)

	taskYAML := `id: TASK-TR-018
title: Full Test Results Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create test report with proper structure
	report := task.TestReport{
		Version:   1,
		Framework: "playwright",
		Duration:  3000,
		Summary: task.TestSummary{
			Total:  5,
			Passed: 4,
			Failed: 1,
		},
	}
	reportBytes, _ := json.Marshal(report)
	os.WriteFile(filepath.Join(testResultsDir, "report.json"), reportBytes, 0644)

	// Create screenshots
	os.WriteFile(filepath.Join(screenshotsDir, "failure-1.png"), []byte("PNG"), 0644)
	os.WriteFile(filepath.Join(screenshotsDir, "failure-2.png"), []byte("PNG"), 0644)

	// Create traces
	os.WriteFile(filepath.Join(tracesDir, "trace-1.zip"), []byte("ZIP"), 0644)

	// Create HTML report
	os.WriteFile(filepath.Join(testResultsDir, "index.html"), []byte("<html></html>"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-018/test-results", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var results task.TestResultsInfo
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if results.Report == nil {
		t.Error("expected report to be present")
	}
	if len(results.Screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(results.Screenshots))
	}
	if len(results.TraceFiles) != 1 {
		t.Errorf("expected 1 trace, got %d", len(results.TraceFiles))
	}
	if !results.HasHTMLReport {
		t.Error("expected HTML report to be present")
	}
}
