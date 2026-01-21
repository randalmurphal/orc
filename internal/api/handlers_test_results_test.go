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

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// === Test Results API Tests ===

// createTestResultsBackend creates a backend for test results tests.
func createTestResultsBackend(t *testing.T, tmpDir string) *storage.DatabaseBackend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestGetTestResultsEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/test-results", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTestResultsEndpoint_NoResults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-001", "Test Results Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-002", "Test Results Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create test results directory and report (file system artifacts)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-002")
	testResultsDir := filepath.Join(taskDir, "test-results")
	_ = os.MkdirAll(testResultsDir, 0755)

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
	_ = os.WriteFile(filepath.Join(testResultsDir, "report.json"), reportBytes, 0644)

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/test-results/screenshots", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListScreenshotsEndpoint_EmptyList(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-003", "Screenshot Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-004", "Screenshot Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create screenshots directory and files (file system artifacts)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-004")
	screenshotsDir := filepath.Join(taskDir, "test-results", "screenshots")
	_ = os.MkdirAll(screenshotsDir, 0755)
	_ = os.WriteFile(filepath.Join(screenshotsDir, "login-page.png"), []byte("PNG content"), 0644)
	_ = os.WriteFile(filepath.Join(screenshotsDir, "dashboard.png"), []byte("PNG content 2"), 0644)

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-005", "Screenshot Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create screenshot (file system artifact)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-005")
	screenshotsDir := filepath.Join(taskDir, "test-results", "screenshots")
	_ = os.MkdirAll(screenshotsDir, 0755)
	content := []byte("PNG content")
	_ = os.WriteFile(filepath.Join(screenshotsDir, "test.png"), content, 0644)

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-006", "Screenshot Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-006/test-results/screenshots/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetScreenshotEndpoint_PathTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-007", "Screenshot Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-008", "Screenshot Upload Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile("file", "screenshot.png")
	_, _ = part.Write([]byte("fake PNG content"))
	_ = writer.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.png")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/test-results/screenshots", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUploadScreenshotEndpoint_NoFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-009", "Screenshot Upload Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-TR-009/test-results/screenshots", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSaveTestReportEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-010", "Test Report Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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

	// Verify report was saved (file system artifact)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-010")
	savedReport, _ := os.ReadFile(filepath.Join(taskDir, "test-results", "report.json"))
	if len(savedReport) == 0 {
		t.Error("expected report.json to be saved")
	}
}

func TestSaveTestReportEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-011", "Test Report Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-012", "Init Test Results Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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

	// Verify directories were created (file system artifacts)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-012")
	if _, err := os.Stat(filepath.Join(taskDir, "test-results", "screenshots")); os.IsNotExist(err) {
		t.Error("expected screenshots directory to be created")
	}
	if _, err := os.Stat(filepath.Join(taskDir, "test-results", "traces")); os.IsNotExist(err) {
		t.Error("expected traces directory to be created")
	}
}

func TestInitTestResultsEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/test-results/init", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetHTMLReportEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-013", "HTML Report Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create HTML report (file system artifact)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-013")
	testResultsDir := filepath.Join(taskDir, "test-results")
	_ = os.MkdirAll(testResultsDir, 0755)
	htmlContent := "<html><body>Test Report</body></html>"
	_ = os.WriteFile(filepath.Join(testResultsDir, "index.html"), []byte(htmlContent), 0644)

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend (no HTML report)
	tsk := task.New("TASK-TR-014", "HTML Report Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-014/test-results/report", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTraceEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-015", "Trace Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create trace file (file system artifact)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-015")
	tracesDir := filepath.Join(taskDir, "test-results", "traces")
	_ = os.MkdirAll(tracesDir, 0755)
	traceContent := []byte("fake zip content")
	_ = os.WriteFile(filepath.Join(tracesDir, "trace.zip"), traceContent, 0644)

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend (no trace file)
	tsk := task.New("TASK-TR-016", "Trace Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TR-016/test-results/traces/nonexistent.zip", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTraceEndpoint_PathTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-017", "Trace Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

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
	t.Parallel()
	tmpDir := t.TempDir()
	backend := createTestResultsBackend(t, tmpDir)

	// Create task via backend
	tsk := task.New("TASK-TR-018", "Full Test Results Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Create full test results (file system artifacts)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TR-018")
	testResultsDir := filepath.Join(taskDir, "test-results")
	screenshotsDir := filepath.Join(testResultsDir, "screenshots")
	tracesDir := filepath.Join(testResultsDir, "traces")
	_ = os.MkdirAll(screenshotsDir, 0755)
	_ = os.MkdirAll(tracesDir, 0755)

	// Create test report
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
	_ = os.WriteFile(filepath.Join(testResultsDir, "report.json"), reportBytes, 0644)

	// Create screenshots
	_ = os.WriteFile(filepath.Join(screenshotsDir, "failure-1.png"), []byte("PNG"), 0644)
	_ = os.WriteFile(filepath.Join(screenshotsDir, "failure-2.png"), []byte("PNG"), 0644)

	// Create traces
	_ = os.WriteFile(filepath.Join(tracesDir, "trace-1.zip"), []byte("ZIP"), 0644)

	// Create HTML report
	_ = os.WriteFile(filepath.Join(testResultsDir, "index.html"), []byte("<html></html>"), 0644)

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
