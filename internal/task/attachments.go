// Package task provides task management for orc.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package task

import (
	"mime"
	"path/filepath"
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

// DetectContentType returns the MIME type for a filename based on extension.
func DetectContentType(filename string) string {
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

// IsImageContentType returns true if the content type is an image.
func IsImageContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}
