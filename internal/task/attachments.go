// Package task provides task management for orc.
package task

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// AttachmentsDir is the subdirectory for task attachments
	AttachmentsDir = "attachments"
)

// Attachment represents a file attached to a task.
type Attachment struct {
	// Filename is the name of the file
	Filename string `json:"filename"`

	// Size is the file size in bytes
	Size int64 `json:"size"`

	// ContentType is the MIME type of the file
	ContentType string `json:"content_type"`

	// CreatedAt is when the attachment was added
	CreatedAt time.Time `json:"created_at"`

	// IsImage returns true if the file is an image
	IsImage bool `json:"is_image"`
}

// AttachmentPath returns the full path to the attachments directory for a task.
func AttachmentPath(projectDir, taskID string) string {
	return filepath.Join(projectDir, OrcDir, TasksDir, taskID, AttachmentsDir)
}

// ListAttachments returns all attachments for a task.
func ListAttachments(projectDir, taskID string) ([]Attachment, error) {
	attachmentsDir := AttachmentPath(projectDir, taskID)

	entries, err := os.ReadDir(attachmentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Attachment{}, nil
		}
		return nil, fmt.Errorf("read attachments directory: %w", err)
	}

	var attachments []Attachment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filename := entry.Name()
		contentType := detectContentType(filename)

		attachments = append(attachments, Attachment{
			Filename:    filename,
			Size:        info.Size(),
			ContentType: contentType,
			CreatedAt:   info.ModTime(),
			IsImage:     isImageContentType(contentType),
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(attachments, func(i, j int) bool {
		return attachments[i].CreatedAt.After(attachments[j].CreatedAt)
	})

	return attachments, nil
}

// GetAttachment returns a specific attachment's metadata and reader.
func GetAttachment(projectDir, taskID, filename string) (*Attachment, io.ReadCloser, error) {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return nil, nil, fmt.Errorf("invalid filename")
	}

	attachmentPath := filepath.Join(AttachmentPath(projectDir, taskID), filename)

	file, err := os.Open(attachmentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("attachment not found")
		}
		return nil, nil, fmt.Errorf("open attachment: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("stat attachment: %w", err)
	}

	contentType := detectContentType(filename)

	attachment := &Attachment{
		Filename:    filename,
		Size:        info.Size(),
		ContentType: contentType,
		CreatedAt:   info.ModTime(),
		IsImage:     isImageContentType(contentType),
	}

	return attachment, file, nil
}

// SaveAttachment saves a new attachment to a task.
func SaveAttachment(projectDir, taskID, filename string, reader io.Reader) (*Attachment, error) {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return nil, fmt.Errorf("invalid filename")
	}

	attachmentsDir := AttachmentPath(projectDir, taskID)

	// Create attachments directory if it doesn't exist
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return nil, fmt.Errorf("create attachments directory: %w", err)
	}

	attachmentPath := filepath.Join(attachmentsDir, filename)

	// Write to a temp file first for atomic write
	tmpFile, err := os.CreateTemp(attachmentsDir, ".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Copy content to temp file
	size, err := io.Copy(tmpFile, reader)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("write attachment: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Rename to final location (atomic on POSIX)
	if err := os.Rename(tmpPath, attachmentPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("save attachment: %w", err)
	}

	contentType := detectContentType(filename)

	return &Attachment{
		Filename:    filename,
		Size:        size,
		ContentType: contentType,
		CreatedAt:   time.Now(),
		IsImage:     isImageContentType(contentType),
	}, nil
}

// DeleteAttachment removes an attachment from a task.
func DeleteAttachment(projectDir, taskID, filename string) error {
	// Validate filename to prevent directory traversal
	if strings.ContainsAny(filename, "/\\") || filename == ".." || filename == "." {
		return fmt.Errorf("invalid filename")
	}

	attachmentPath := filepath.Join(AttachmentPath(projectDir, taskID), filename)

	if err := os.Remove(attachmentPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("attachment not found")
		}
		return fmt.Errorf("delete attachment: %w", err)
	}

	return nil
}

// detectContentType returns the MIME type for a filename based on extension.
func detectContentType(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}

	// Try standard mime type detection
	contentType := mime.TypeByExtension(ext)
	if contentType != "" {
		return contentType
	}

	// Fallback for common extensions
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	default:
		return "application/octet-stream"
	}
}

// isImageContentType returns true if the content type is an image.
func isImageContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}
