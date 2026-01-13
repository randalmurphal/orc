package task

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentPath(t *testing.T) {
	result := AttachmentPath("/project", "TASK-001")
	expected := filepath.Join("/project", OrcDir, TasksDir, "TASK-001", AttachmentsDir)
	if result != expected {
		t.Errorf("AttachmentPath() = %q, want %q", result, expected)
	}
}

func TestListAttachments_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory but no attachments
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	attachments, err := ListAttachments(tmpDir, taskID)
	if err != nil {
		t.Fatalf("ListAttachments() error = %v", err)
	}

	if len(attachments) != 0 {
		t.Errorf("ListAttachments() returned %d attachments, want 0", len(attachments))
	}
}

func TestListAttachments_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachments directory with files
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	testFiles := map[string][]byte{
		"screenshot.png":  []byte("PNG data"),
		"document.pdf":    []byte("PDF data"),
		"notes.txt":       []byte("Text data"),
		"image.jpg":       []byte("JPEG data"),
		"unknown.xyz":     []byte("Unknown data"),
	}

	for name, content := range testFiles {
		if err := os.WriteFile(filepath.Join(attachmentsDir, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	attachments, err := ListAttachments(tmpDir, taskID)
	if err != nil {
		t.Fatalf("ListAttachments() error = %v", err)
	}

	if len(attachments) != len(testFiles) {
		t.Errorf("ListAttachments() returned %d attachments, want %d", len(attachments), len(testFiles))
	}

	// Check that files are present and have correct properties
	attachMap := make(map[string]Attachment)
	for _, a := range attachments {
		attachMap[a.Filename] = a
	}

	// Check PNG
	if a, ok := attachMap["screenshot.png"]; ok {
		if a.ContentType != "image/png" {
			t.Errorf("screenshot.png ContentType = %q, want %q", a.ContentType, "image/png")
		}
		if !a.IsImage {
			t.Error("screenshot.png should be marked as image")
		}
	} else {
		t.Error("screenshot.png not found in attachments")
	}

	// Check PDF
	if a, ok := attachMap["document.pdf"]; ok {
		if a.ContentType != "application/pdf" {
			t.Errorf("document.pdf ContentType = %q, want %q", a.ContentType, "application/pdf")
		}
		if a.IsImage {
			t.Error("document.pdf should not be marked as image")
		}
	} else {
		t.Error("document.pdf not found in attachments")
	}

	// Check unknown extension - may vary by platform
	if a, ok := attachMap["unknown.xyz"]; ok {
		// macOS may return "chemical/x-xyz" for .xyz files
		validTypes := []string{"application/octet-stream", "chemical/x-xyz"}
		valid := false
		for _, t := range validTypes {
			if a.ContentType == t {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("unknown.xyz ContentType = %q, want one of %v", a.ContentType, validTypes)
		}
	} else {
		t.Error("unknown.xyz not found in attachments")
	}
}

func TestSaveAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Save an attachment
	content := []byte("Test image content")
	reader := bytes.NewReader(content)

	attachment, err := SaveAttachment(tmpDir, taskID, "test.png", reader)
	if err != nil {
		t.Fatalf("SaveAttachment() error = %v", err)
	}

	if attachment.Filename != "test.png" {
		t.Errorf("Filename = %q, want %q", attachment.Filename, "test.png")
	}

	if attachment.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", attachment.Size, len(content))
	}

	if attachment.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want %q", attachment.ContentType, "image/png")
	}

	if !attachment.IsImage {
		t.Error("Should be marked as image")
	}

	// Verify file was created
	savedPath := filepath.Join(AttachmentPath(tmpDir, taskID), "test.png")
	savedContent, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if !bytes.Equal(savedContent, content) {
		t.Error("Saved content does not match original")
	}
}

func TestSaveAttachment_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.txt",
		"path\\to\\file.txt",
		"..",
		".",
	}

	for _, filename := range invalidFilenames {
		reader := bytes.NewReader([]byte("test"))
		_, err := SaveAttachment(tmpDir, taskID, filename, reader)
		if err == nil {
			t.Errorf("SaveAttachment(%q) should fail", filename)
		}
	}
}

func TestGetAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachment
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("Test file content")
	if err := os.WriteFile(filepath.Join(attachmentsDir, "test.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}

	// Get attachment
	attachment, reader, err := GetAttachment(tmpDir, taskID, "test.txt")
	if err != nil {
		t.Fatalf("GetAttachment() error = %v", err)
	}
	defer reader.Close()

	if attachment.Filename != "test.txt" {
		t.Errorf("Filename = %q, want %q", attachment.Filename, "test.txt")
	}

	if attachment.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", attachment.Size, len(content))
	}

	// Read and verify content
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("Read content does not match")
	}
}

func TestGetAttachment_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachments directory but no file
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, _, err := GetAttachment(tmpDir, taskID, "nonexistent.txt")
	if err == nil {
		t.Error("GetAttachment() should fail for nonexistent file")
	}
}

func TestGetAttachment_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.txt",
		"..",
	}

	for _, filename := range invalidFilenames {
		_, _, err := GetAttachment(tmpDir, taskID, filename)
		if err == nil {
			t.Errorf("GetAttachment(%q) should fail", filename)
		}
	}
}

func TestDeleteAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachment
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(attachmentsDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Fatal("File should exist before delete")
	}

	// Delete attachment
	if err := DeleteAttachment(tmpDir, taskID, "test.txt"); err != nil {
		t.Fatalf("DeleteAttachment() error = %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}
}

func TestDeleteAttachment_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachments directory but no file
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := DeleteAttachment(tmpDir, taskID, "nonexistent.txt")
	if err == nil {
		t.Error("DeleteAttachment() should fail for nonexistent file")
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		filename      string
		expectedTypes []string // Allow multiple valid types
	}{
		{"image.png", []string{"image/png"}},
		{"image.PNG", []string{"image/png"}},
		{"photo.jpg", []string{"image/jpeg"}},
		{"photo.jpeg", []string{"image/jpeg"}},
		{"animation.gif", []string{"image/gif"}},
		{"image.webp", []string{"image/webp"}},
		{"icon.ico", []string{"image/x-icon", "image/vnd.microsoft.icon"}},
		{"diagram.svg", []string{"image/svg+xml"}},
		{"document.pdf", []string{"application/pdf"}},
		{"notes.txt", []string{"text/plain"}},
		{"readme.md", []string{"text/markdown"}},
		{"config.json", []string{"application/json"}},
		{"config.yaml", []string{"text/yaml", "application/yaml"}},
		{"config.yml", []string{"text/yaml", "application/yaml"}},
		{"unknown.xyz", []string{"application/octet-stream", "chemical/x-xyz"}}, // macOS may return chemical/x-xyz
		{"noextension", []string{"application/octet-stream"}},
	}

	for _, tt := range tests {
		result := detectContentType(tt.filename)
		found := false
		for _, expected := range tt.expectedTypes {
			// For most standard MIME types, there may be slight variations
			// (e.g., "text/plain; charset=utf-8"), so we check prefix
			if result == expected || hasPrefix(result, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("detectContentType(%q) = %q, want one of %v", tt.filename, result, tt.expectedTypes)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestIsImageContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/svg+xml", true},
		{"application/pdf", false},
		{"text/plain", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		result := isImageContentType(tt.contentType)
		if result != tt.expected {
			t.Errorf("isImageContentType(%q) = %v, want %v", tt.contentType, result, tt.expected)
		}
	}
}

func TestListAttachments_WithSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachments directory with files and a subdirectory
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory (should be skipped)
	if err := os.MkdirAll(filepath.Join(attachmentsDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test file
	if err := os.WriteFile(filepath.Join(attachmentsDir, "test.png"), []byte("PNG data"), 0644); err != nil {
		t.Fatal(err)
	}

	attachments, err := ListAttachments(tmpDir, taskID)
	if err != nil {
		t.Fatalf("ListAttachments() error = %v", err)
	}

	// Should only have the file, not the subdirectory
	if len(attachments) != 1 {
		t.Errorf("ListAttachments() returned %d attachments, want 1 (should skip subdirectories)", len(attachments))
	}
}

func TestListAttachments_ReadDirError(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create attachments directory
	attachmentsDir := AttachmentPath(tmpDir, taskID)
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make the directory unreadable
	if err := os.Chmod(attachmentsDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(attachmentsDir, 0755) // Restore for cleanup

	_, err := ListAttachments(tmpDir, taskID)
	if err == nil {
		t.Error("ListAttachments() should fail for unreadable directory")
	}
}

func TestSaveAttachment_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Save first version
	content1 := []byte("Version 1")
	reader1 := bytes.NewReader(content1)
	_, err := SaveAttachment(tmpDir, taskID, "test.txt", reader1)
	if err != nil {
		t.Fatalf("First SaveAttachment() error = %v", err)
	}

	// Save second version (overwrite)
	content2 := []byte("Version 2 with more content")
	reader2 := bytes.NewReader(content2)
	attachment, err := SaveAttachment(tmpDir, taskID, "test.txt", reader2)
	if err != nil {
		t.Fatalf("Second SaveAttachment() error = %v", err)
	}

	// Verify the file was overwritten
	if attachment.Size != int64(len(content2)) {
		t.Errorf("Size = %d, want %d", attachment.Size, len(content2))
	}

	// Read and verify content
	savedPath := filepath.Join(AttachmentPath(tmpDir, taskID), "test.txt")
	savedContent, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}
	if !bytes.Equal(savedContent, content2) {
		t.Error("Saved content does not match second version")
	}
}

func TestSaveAttachment_EmptyFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader([]byte("test"))
	_, err := SaveAttachment(tmpDir, taskID, "", reader)
	// Empty filename should create temp file naming issues or be handled
	// The current implementation would allow it but create unusual files
	if err == nil {
		// If it succeeds, check the file was created with empty name
		files, _ := os.ReadDir(AttachmentPath(tmpDir, taskID))
		// Empty filename is technically valid, just unusual
		if len(files) == 0 {
			t.Error("No file was created")
		}
	}
}

func TestDeleteAttachment_InvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "TASK-001"

	invalidFilenames := []string{
		"../etc/passwd",
		"path/to/file.txt",
		"..",
		".",
	}

	for _, filename := range invalidFilenames {
		err := DeleteAttachment(tmpDir, taskID, filename)
		if err == nil {
			t.Errorf("DeleteAttachment(%q) should fail", filename)
		}
	}
}

func TestDetectContentType_AllFallbacks(t *testing.T) {
	// Test all fallback cases in detectContentType
	tests := []struct {
		filename    string
		expectedPfx []string // Multiple valid prefixes to allow for platform variations
	}{
		{"file.png", []string{"image/png"}},
		{"file.PNG", []string{"image/png"}}, // Case insensitive
		{"file.jpg", []string{"image/jpeg"}},
		{"file.jpeg", []string{"image/jpeg"}},
		{"file.gif", []string{"image/gif"}},
		{"file.webp", []string{"image/webp"}},
		{"file.svg", []string{"image/svg"}},
		{"file.ico", []string{"image/"}}, // x-icon or vnd.microsoft.icon
		{"file.pdf", []string{"application/pdf"}},
		{"file.txt", []string{"text/plain"}},
		{"file.md", []string{"text/"}}, // markdown or plain
		{"file.json", []string{"application/json"}},
		{"file.yaml", []string{"text/yaml", "application/yaml"}},
		{"file.yml", []string{"text/yaml", "application/yaml"}},
		{"noextension", []string{"application/octet-stream"}},
	}

	for _, tt := range tests {
		result := detectContentType(tt.filename)
		found := false
		for _, pfx := range tt.expectedPfx {
			if strings.HasPrefix(result, pfx) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("detectContentType(%q) = %q, want one of prefixes %v", tt.filename, result, tt.expectedPfx)
		}
	}
}
