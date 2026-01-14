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

// === Attachments API Tests ===

func TestListAttachmentsEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/attachments", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListAttachmentsEndpoint_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-001")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-ATT-001
title: Attachment Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task directory with attachments
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-002")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-002
title: Attachment Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create test attachments
	os.WriteFile(filepath.Join(attachmentsDir, "screenshot.png"), []byte("PNG content"), 0644)
	os.WriteFile(filepath.Join(attachmentsDir, "notes.txt"), []byte("Some notes"), 0644)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write([]byte("test content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUploadAttachmentEndpoint_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-003")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-ATT-003
title: Upload Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Request without file
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-ATT-003/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUploadAttachmentEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-004")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-ATT-004
title: Upload Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "screenshot.png")
	part.Write([]byte("PNG test content"))
	writer.Close()

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

	// Verify file was created
	attachmentPath := filepath.Join(taskDir, "attachments", "screenshot.png")
	if _, err := os.Stat(attachmentPath); os.IsNotExist(err) {
		t.Error("expected attachment file to be created")
	}
}

func TestUploadAttachmentEndpoint_WithCustomFilename(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-005")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-ATT-005
title: Upload Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with custom filename
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", "custom-name.png")
	part, _ := writer.CreateFormFile("file", "original.png")
	part.Write([]byte("PNG test content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks/TASK-ATT-005/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var attachment task.Attachment
	json.NewDecoder(w.Body).Decode(&attachment)

	if attachment.Filename != "custom-name.png" {
		t.Errorf("expected filename 'custom-name.png', got %q", attachment.Filename)
	}
}

func TestGetAttachmentEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/attachments/test.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetAttachmentEndpoint_AttachmentNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-006")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-006
title: Get Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-ATT-006/attachments/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetAttachmentEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with attachment
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-007")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-007
title: Get Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	testContent := []byte("PNG test content here")
	os.WriteFile(filepath.Join(attachmentsDir, "screenshot.png"), testContent, 0644)

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
	tmpDir := t.TempDir()

	// Create task directory with non-image attachment
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-008")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-008
title: Get Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)
	os.WriteFile(filepath.Join(attachmentsDir, "document.pdf"), []byte("PDF content"), 0644)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/NONEXISTENT/attachments/test.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteAttachmentEndpoint_AttachmentNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-009")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-009
title: Delete Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-ATT-009/attachments/nonexistent.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteAttachmentEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory with attachment
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ATT-010")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-ATT-010
title: Delete Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	attachmentPath := filepath.Join(attachmentsDir, "to-delete.png")
	os.WriteFile(attachmentPath, []byte("PNG content"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-ATT-010/attachments/to-delete.png", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was deleted
	if _, err := os.Stat(attachmentPath); !os.IsNotExist(err) {
		t.Error("expected attachment file to be deleted")
	}
}

// === Security Edge Cases ===

func TestGetAttachmentEndpoint_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SEC-001")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-SEC-001
title: Security Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create a file in attachments to make sure it exists
	os.WriteFile(filepath.Join(attachmentsDir, "safe.txt"), []byte("Safe content"), 0644)

	// Also create a "sensitive" file outside the attachments directory
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SEC-002")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-SEC-002
title: Security Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	maliciousFilenames := []string{
		"../../../etc/malicious.txt",
		"..%2F..%2Fetc%2Fmalicious.txt",
		"../task.yaml",
	}

	for _, maliciousName := range maliciousFilenames {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("filename", maliciousName)
		part, _ := writer.CreateFormFile("file", "innocent.txt")
		part.Write([]byte("Malicious content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/tasks/TASK-SEC-002/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		// The handler should sanitize the filename using filepath.Base
		// So it should succeed but write to a safe location
		if w.Code == http.StatusCreated {
			var attachment task.Attachment
			json.NewDecoder(w.Body).Decode(&attachment)

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
	tmpDir := t.TempDir()

	// Create task directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SEC-003")
	attachmentsDir := filepath.Join(taskDir, "attachments")
	os.MkdirAll(attachmentsDir, 0755)

	taskYAML := `id: TASK-SEC-003
title: Security Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create a file we want to protect
	sensitiveFile := filepath.Join(taskDir, "sensitive.txt")
	os.WriteFile(sensitiveFile, []byte("Sensitive data"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	pathTraversalAttempts := []string{
		"../sensitive.txt",
		"..%2Fsensitive.txt",
		"../task.yaml",
	}

	for _, attempt := range pathTraversalAttempts {
		req := httptest.NewRequest("DELETE", "/api/tasks/TASK-SEC-003/attachments/"+attempt, nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		// The handler uses filepath.Base which should strip path traversal
		// So it should fail with 404 (file not found in attachments dir)
		if w.Code == http.StatusNoContent {
			t.Errorf("path traversal delete attempt %q should not succeed", attempt)
		}

		// Verify sensitive file still exists
		if _, err := os.Stat(sensitiveFile); os.IsNotExist(err) {
			t.Errorf("sensitive file was deleted by path traversal attempt %q", attempt)
		}
	}
}

func TestUploadAttachmentEndpoint_EmptyFilename(t *testing.T) {
	tmpDir := t.TempDir()

	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SEC-004")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-SEC-004
title: Security Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with empty filename override
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", "")
	part, _ := writer.CreateFormFile("file", "") // Empty original filename too
	part.Write([]byte("content"))
	writer.Close()

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
	tmpDir := t.TempDir()

	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SEC-005")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-SEC-005
title: Security Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with "." as filename
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", ".")
	part, _ := writer.CreateFormFile("file", "original.txt")
	part.Write([]byte("content"))
	writer.Close()

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
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form with task data and attachments
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("title", "Test Task with Attachments")
	writer.WriteField("description", "Task description")
	writer.WriteField("category", "bug")

	// Add two attachments
	part1, _ := writer.CreateFormFile("attachments", "screenshot1.png")
	part1.Write([]byte("PNG content 1"))
	part2, _ := writer.CreateFormFile("attachments", "screenshot2.png")
	part2.Write([]byte("PNG content 2"))
	writer.Close()

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

	// Verify attachments were saved
	attachmentsDir := filepath.Join(tmpDir, ".orc", "tasks", createdTask.ID, "attachments")
	files, err := os.ReadDir(attachmentsDir)
	if err != nil {
		t.Fatalf("failed to read attachments dir: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(files))
	}

	// Check filenames
	filenames := make(map[string]bool)
	for _, f := range files {
		filenames[f.Name()] = true
	}

	if !filenames["screenshot1.png"] {
		t.Error("expected screenshot1.png to be saved")
	}
	if !filenames["screenshot2.png"] {
		t.Error("expected screenshot2.png to be saved")
	}
}

func TestCreateTaskEndpoint_MultipartWithoutAttachments(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form without attachments (just task data)
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("title", "Task Without Attachments")
	writer.WriteField("description", "Just a regular task")
	writer.Close()

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
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Create multipart form without title
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("description", "Task without title")
	part, _ := writer.CreateFormFile("attachments", "screenshot.png")
	part.Write([]byte("PNG content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}
