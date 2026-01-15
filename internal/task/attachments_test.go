package task

import (
	"strings"
	"testing"
)

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
		result := DetectContentType(tt.filename)
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
			t.Errorf("DetectContentType(%q) = %q, want one of %v", tt.filename, result, tt.expectedTypes)
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
		result := IsImageContentType(tt.contentType)
		if result != tt.expected {
			t.Errorf("IsImageContentType(%q) = %v, want %v", tt.contentType, result, tt.expected)
		}
	}
}

func TestDetectContentType_AllFallbacks(t *testing.T) {
	// Test all fallback cases in DetectContentType
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
		result := DetectContentType(tt.filename)
		found := false
		for _, pfx := range tt.expectedPfx {
			if strings.HasPrefix(result, pfx) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DetectContentType(%q) = %q, want one of prefixes %v", tt.filename, result, tt.expectedPfx)
		}
	}
}
