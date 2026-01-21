package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// === Attachments API Tests ===

func TestListAttachmentsEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/attachments", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListAttachmentsEndpoint_EmptyList(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-001", "Attachment Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-001/attachments", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var attachments []task.Attachment
	if err := json.NewDecoder(w.Body).Decode(&attachments); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(attachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(attachments))
	}
}

func TestListAttachmentsEndpoint_WithAttachments(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-002", "Attachment Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create test attachments through backend (stored in database)
	if _, err := backend.SaveAttachment("TASK-ATT-002", "screenshot.png", "image/png", []byte("PNG content")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}
	if _, err := backend.SaveAttachment("TASK-ATT-002", "notes.txt", "text/plain", []byte("Some notes")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-002/attachments", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var attachments []task.Attachment
	if err := json.NewDecoder(w.Body).Decode(&attachments); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(attachments))
	}
}

func TestUploadAttachmentEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	_, _ = part.Write([]byte("test content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUploadAttachmentEndpoint_NoFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-003", "Upload Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Request without file
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-ATT-003/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUploadAttachmentEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-004", "Upload Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with explicit content type header
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="screenshot.png"`)
	h.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(h)
	_, _ = part.Write([]byte("PNG test content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-ATT-004/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var attachment task.Attachment
	if err := json.NewDecoder(w.Body).Decode(&attachment); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if attachment.Filename != "screenshot.png" {
		t.Errorf("expected filename 'screenshot.png', got %q", attachment.Filename)
	}

	if attachment.ContentType != "image/png" {
		t.Errorf("expected content type 'image/png', got %q", attachment.ContentType)
	}

	if !attachment.IsImage {
		t.Error("expected IsImage to be true")
	}

	// Verify attachment can be retrieved via API
	req = httptest.NewRequest("GET", "/api/tasks/TASK-ATT-004/attachments/screenshot.png", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected attachment to be retrievable (200), got %d", w.Code)
	}
}

func TestUploadAttachmentEndpoint_WithCustomFilename(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-005", "Upload Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with custom filename
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("filename", "custom-name.png")
	part, _ := writer.CreateFormFile("file", "original.png")
	_, _ = part.Write([]byte("PNG test content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-ATT-005/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var attachment task.Attachment
	_ = json.NewDecoder(w.Body).Decode(&attachment)

	if attachment.Filename != "custom-name.png" {
		t.Errorf("expected filename 'custom-name.png', got %q", attachment.Filename)
	}
}

func TestGetAttachmentEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/attachments/test.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetAttachmentEndpoint_AttachmentNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-006", "Get Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	// No attachments saved - testing 404 response
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-006/attachments/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetAttachmentEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-007", "Get Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create attachment through backend (stored in database)
	testContent := []byte("PNG test content here")
	if _, err := backend.SaveAttachment("TASK-ATT-007", "screenshot.png", "image/png", testContent); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-007/attachments/screenshot.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check content type
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected content type 'image/png', got %q", w.Header().Get("Content-Type"))
	}

	// Check content disposition (inline for images)
	if w.Header().Get("Content-Disposition") != `inline; filename="screenshot.png"` {
		t.Errorf("expected inline content disposition, got %q", w.Header().Get("Content-Disposition"))
	}

	// Check body
	if !bytes.Equal(w.Body.Bytes(), testContent) {
		t.Error("response body does not match original content")
	}
}

func TestGetAttachmentEndpoint_NonImageAttachment(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-008", "Get Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create non-image attachment through backend
	if _, err := backend.SaveAttachment("TASK-ATT-008", "document.pdf", "application/pdf", []byte("PDF content")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-008/attachments/document.pdf", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check content disposition (attachment for non-images)
	if w.Header().Get("Content-Disposition") != `attachment; filename="document.pdf"` {
		t.Errorf("expected attachment content disposition, got %q", w.Header().Get("Content-Disposition"))
	}
}

func TestDeleteAttachmentEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/NONEXISTENT/attachments/test.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteAttachmentEndpoint_AttachmentNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-009", "Delete Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	// No attachments saved - testing 404 response
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-ATT-009/attachments/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteAttachmentEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-ATT-010", "Delete Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create attachment through backend
	if _, err := backend.SaveAttachment("TASK-ATT-010", "to-delete.png", "image/png", []byte("PNG content")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-ATT-010/attachments/to-delete.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify attachment was deleted by trying to get it
	req = httptest.NewRequest("GET", "/api/tasks/TASK-ATT-010/attachments/to-delete.png", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected attachment to be deleted (404), got %d", w.Code)
	}
}

// === Security Edge Cases ===

func TestGetAttachmentEndpoint_PathTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-SEC-001", "Security Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a safe attachment via backend
	if _, err := backend.SaveAttachment("TASK-SEC-001", "safe.txt", "text/plain", []byte("Safe content")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	pathTraversalAttempts := []string{
		"../../../etc/passwd",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"....//....//etc/passwd",
		"..\\..\\..\\etc\\passwd",
		"..%5C..%5C..%5Cetc%5Cpasswd",
		"../task.yaml",
		"..%2ftask.yaml",
	}

	for _, attempt := range pathTraversalAttempts {
		req := httptest.NewRequest("GET", "/api/tasks/TASK-SEC-001/attachments/"+attempt, nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		// Should be either 400 (bad request) or 404 (not found) but NOT 200
		if w.Code == http.StatusOK {
			t.Errorf("path traversal attempt %q should not succeed with status 200", attempt)
		}
	}
}

func TestUploadAttachmentEndpoint_PathTraversalInFilename(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-SEC-002", "Security Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	maliciousFilenames := []string{
		"../../../etc/malicious.txt",
		"..%2F..%2Fetc%2Fmalicious.txt",
		"../task.yaml",
	}

	for _, maliciousName := range maliciousFilenames {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("filename", maliciousName)
		part, _ := writer.CreateFormFile("file", "innocent.txt")
		_, _ = part.Write([]byte("Malicious content"))
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/api/tasks/TASK-SEC-002/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		// The handler should sanitize the filename using filepath.Base
		// So it should succeed but write to a safe location
		if w.Code == http.StatusCreated {
			var attachment task.Attachment
			_ = json.NewDecoder(w.Body).Decode(&attachment)

			// Filename should be sanitized (no path components)
			if attachment.Filename != filepath.Base(maliciousName) {
				// If it's not exactly the base, check it doesn't have path separators
				if filepath.Dir(attachment.Filename) != "." && attachment.Filename != filepath.Base(maliciousName) {
					t.Errorf("filename %q was not properly sanitized, got %q", maliciousName, attachment.Filename)
				}
			}

			// Verify file was NOT created outside attachments directory
			// Check parent directories don't have the file
			badPath := filepath.Join(tmpDir, ".orc", "tasks", "malicious.txt")
			if _, err := os.Stat(badPath); !os.IsNotExist(err) {
				t.Errorf("malicious file was created at %q", badPath)
			}
		}
	}
}

func TestDeleteAttachmentEndpoint_PathTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-SEC-003", "Security Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a safe attachment via backend
	if _, err := backend.SaveAttachment("TASK-SEC-003", "safe.txt", "text/plain", []byte("Safe content")); err != nil {
		t.Fatalf("failed to save attachment: %v", err)
	}

	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Path traversal attempts should fail because:
	// 1. filepath.Base strips path components
	// 2. The resulting filename doesn't exist in the database
	pathTraversalAttempts := []string{
		"../sensitive.txt",
		"..%2Fsensitive.txt",
		"../task.yaml",
	}

	for _, attempt := range pathTraversalAttempts {
		req := httptest.NewRequest("DELETE", "/api/tasks/TASK-SEC-003/attachments/"+attempt, nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		// The handler uses filepath.Base which strips path traversal
		// So "../sensitive.txt" becomes "sensitive.txt" which doesn't exist -> 404
		if w.Code == http.StatusNoContent {
			t.Errorf("path traversal delete attempt %q should not succeed", attempt)
		}
	}

	// Verify safe.txt still exists after all attempts
	req := httptest.NewRequest("GET", "/api/tasks/TASK-SEC-003/attachments/safe.txt", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("safe attachment should still exist after path traversal attempts")
	}
}

func TestUploadAttachmentEndpoint_EmptyFilename(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-SEC-004", "Security Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with empty filename override
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("filename", "")
	part, _ := writer.CreateFormFile("file", "") // Empty original filename too
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-SEC-004/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should reject empty filename
	if w.Code == http.StatusCreated {
		t.Error("should reject empty filename")
	}
}

func TestUploadAttachmentEndpoint_DotFilename(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-SEC-005", "Security Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with "." as filename
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("filename", ".")
	part, _ := writer.CreateFormFile("file", "original.txt")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-SEC-005/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should reject "." as filename
	if w.Code == http.StatusCreated {
		t.Error("should reject '.' as filename")
	}
}

// === Task Creation with Attachments ===

func TestCreateTaskEndpoint_WithAttachments(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with task data and attachments
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("title", "Test Task with Attachments")
	_ = writer.WriteField("description", "Task description")
	_ = writer.WriteField("category", "bug")

	// Add two attachments
	part1, _ := writer.CreateFormFile("attachments", "screenshot1.png")
	_, _ = part1.Write([]byte("PNG content 1"))
	part2, _ := writer.CreateFormFile("attachments", "screenshot2.png")
	_, _ = part2.Write([]byte("PNG content 2"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var createdTask task.Task
	if err := json.NewDecoder(w.Body).Decode(&createdTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdTask.Title != "Test Task with Attachments" {
		t.Errorf("expected title 'Test Task with Attachments', got %q", createdTask.Title)
	}

	if createdTask.Category != task.CategoryBug {
		t.Errorf("expected category 'bug', got %q", createdTask.Category)
	}

	// Verify attachments were saved by listing them via API
	req = httptest.NewRequest("GET", "/api/tasks/"+createdTask.ID+"/attachments", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to list attachments: %s", w.Body.String())
	}

	var attachments []task.Attachment
	if err := json.NewDecoder(w.Body).Decode(&attachments); err != nil {
		t.Fatalf("failed to decode attachments: %v", err)
	}

	if len(attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(attachments))
	}

	// Check filenames
	filenames := make(map[string]bool)
	for _, a := range attachments {
		filenames[a.Filename] = true
	}

	if !filenames["screenshot1.png"] {
		t.Error("expected screenshot1.png to be saved")
	}
	if !filenames["screenshot2.png"] {
		t.Error("expected screenshot2.png to be saved")
	}
}

func TestCreateTaskEndpoint_MultipartWithoutAttachments(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form without attachments (just task data)
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("title", "Task Without Attachments")
	_ = writer.WriteField("description", "Just a regular task")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var createdTask task.Task
	if err := json.NewDecoder(w.Body).Decode(&createdTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdTask.Title != "Task Without Attachments" {
		t.Errorf("expected title 'Task Without Attachments', got %q", createdTask.Title)
	}
}

func TestCreateTaskEndpoint_JSONStillWorks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create task with JSON (backward compatible)
	reqBody := `{"title": "JSON Task", "description": "Created via JSON", "category": "feature"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var createdTask task.Task
	if err := json.NewDecoder(w.Body).Decode(&createdTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdTask.Title != "JSON Task" {
		t.Errorf("expected title 'JSON Task', got %q", createdTask.Title)
	}

	if createdTask.Category != task.CategoryFeature {
		t.Errorf("expected category 'feature', got %q", createdTask.Category)
	}
}

func TestCreateTaskEndpoint_MultipartMissingTitle(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form without title
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("description", "Task without title")
	part, _ := writer.CreateFormFile("attachments", "screenshot.png")
	_, _ = part.Write([]byte("PNG content"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}
