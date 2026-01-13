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
