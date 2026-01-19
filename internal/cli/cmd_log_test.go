package cli

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestReadNewContentPartialLine(t *testing.T) {
	// Test with a partial line (no newline)
	data := bytes.NewReader([]byte("partial"))
	reader := bufio.NewReader(data)
	var partial strings.Builder

	offset := readNewContent(reader, &partial, 0)

	// Partial line should be buffered (no newline)
	if partial.String() != "partial" {
		t.Errorf("expected partial line to be buffered, got %q", partial.String())
	}
	if offset != 7 {
		t.Errorf("expected offset 7, got %d", offset)
	}
}

func TestReadNewContentCompleteLine(t *testing.T) {
	data := bytes.NewReader([]byte("complete line\n"))
	reader := bufio.NewReader(data)
	var partial strings.Builder

	offset := readNewContent(reader, &partial, 0)

	// Complete line should clear the buffer
	if partial.String() != "" {
		t.Errorf("expected partial to be empty after complete line, got %q", partial.String())
	}
	if offset != 14 {
		t.Errorf("expected offset 14, got %d", offset)
	}
}

func TestReadNewContentWithPriorPartial(t *testing.T) {
	data := bytes.NewReader([]byte("end of line\n"))
	reader := bufio.NewReader(data)
	var partial strings.Builder
	partial.WriteString("start of line - ")

	offset := readNewContent(reader, &partial, 0)

	// After complete line, partial should be cleared
	if partial.String() != "" {
		t.Errorf("expected partial to be empty, got %q", partial.String())
	}
	if offset != 12 {
		t.Errorf("expected offset 12, got %d", offset)
	}
}

func TestReadNewContentMultipleLines(t *testing.T) {
	data := bytes.NewReader([]byte("line 1\nline 2\nline 3\n"))
	reader := bufio.NewReader(data)
	var partial strings.Builder

	offset := readNewContent(reader, &partial, 0)

	// All complete lines, partial should be empty
	if partial.String() != "" {
		t.Errorf("expected partial to be empty, got %q", partial.String())
	}
	if offset != 21 {
		t.Errorf("expected offset 21, got %d", offset)
	}
}

func TestFollowFilePollingCancellation(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(tmpFile, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create a context that we'll cancel quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Capture stderr output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run polling in goroutine
	done := make(chan error, 1)
	go func() {
		done <- followFilePolling(ctx, tmpFile)
	}()

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("followFilePolling returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("followFilePolling did not complete in time")
	}

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Following") {
		t.Errorf("expected 'Following' message in output, got: %q", output)
	}
}

func TestFollowFilePollingNewContent(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(tmpFile, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run polling in goroutine
	done := make(chan error, 1)
	go func() {
		done <- followFilePolling(ctx, tmpFile)
	}()

	// Write new content after a short delay
	time.Sleep(50 * time.Millisecond)
	f, _ := os.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("new line 1\n")
	_, _ = f.WriteString("new line 2\n")
	_ = f.Close()

	// Wait for completion
	<-done

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should see the new lines (not the initial content since we seek to end)
	if !strings.Contains(output, "new line 1") {
		t.Errorf("expected 'new line 1' in output, got: %q", output)
	}
	if !strings.Contains(output, "new line 2") {
		t.Errorf("expected 'new line 2' in output, got: %q", output)
	}
	// Should NOT see initial content
	if strings.Contains(output, "initial") && !strings.Contains(output, "Following") {
		// "initial" might appear in file name in "Following" message, so check carefully
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "initial" {
				t.Errorf("should not see 'initial' content since we seek to end")
			}
		}
	}
}

func TestFollowFileTruncation(t *testing.T) {
	// Create a temp file with content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(tmpFile, []byte("lots of initial content here\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan error, 1)
	go func() {
		done <- followFilePolling(ctx, tmpFile)
	}()

	// Wait for it to start, then truncate
	time.Sleep(50 * time.Millisecond)
	_ = os.WriteFile(tmpFile, []byte("new\n"), 0o644) // This truncates

	// Give it time to detect
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-done

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should see truncation message
	if !strings.Contains(output, "truncated") {
		t.Errorf("expected truncation message in output, got: %q", output)
	}
}

func TestDetectSection(t *testing.T) {
	tests := []struct {
		line     string
		expected transcriptSection
	}{
		{"## Prompt", sectionPrompt},
		{"## Response", sectionResponse},
		{"---", sectionMetadata},
		{"# implement - Iteration 1", sectionMetadata},
		{"# spec - Iteration 3", sectionMetadata},
		{"Some regular content", sectionUnknown},
		{"", sectionUnknown},
		{"  ## Prompt  ", sectionPrompt}, // whitespace should be trimmed
		{"## PromptExtra", sectionUnknown},
		{"## Responses", sectionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := detectSection(tt.line)
			if result != tt.expected {
				t.Errorf("detectSection(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestShouldShowLine(t *testing.T) {
	tests := []struct {
		name     string
		section  transcriptSection
		opts     displayOptions
		expected bool
	}{
		// No filtering - show everything
		{"no filter, unknown section", sectionUnknown, displayOptions{}, true},
		{"no filter, prompt section", sectionPrompt, displayOptions{}, true},
		{"no filter, response section", sectionResponse, displayOptions{}, true},
		{"no filter, metadata section", sectionMetadata, displayOptions{}, true},

		// Response only
		{"response only, unknown section", sectionUnknown, displayOptions{responseOnly: true}, true},
		{"response only, prompt section", sectionPrompt, displayOptions{responseOnly: true}, false},
		{"response only, response section", sectionResponse, displayOptions{responseOnly: true}, true},
		{"response only, metadata section", sectionMetadata, displayOptions{responseOnly: true}, false},

		// Prompt only
		{"prompt only, unknown section", sectionUnknown, displayOptions{promptOnly: true}, true},
		{"prompt only, prompt section", sectionPrompt, displayOptions{promptOnly: true}, true},
		{"prompt only, response section", sectionResponse, displayOptions{promptOnly: true}, false},
		{"prompt only, metadata section", sectionMetadata, displayOptions{promptOnly: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldShowLine(tt.section, tt.opts)
			if result != tt.expected {
				t.Errorf("shouldShowLine(%v, %+v) = %v, want %v",
					tt.section, tt.opts, result, tt.expected)
			}
		})
	}
}

func TestFilterTranscriptLines(t *testing.T) {
	sampleTranscript := []string{
		"# implement - Iteration 1",
		"",
		"## Prompt",
		"",
		"This is the prompt content.",
		"More prompt content.",
		"",
		"## Response",
		"",
		"This is the response content.",
		"More response content.",
		"",
		"---",
		"Tokens: 1000 input, 500 output",
	}

	t.Run("no filtering", func(t *testing.T) {
		opts := displayOptions{}
		result := filterTranscriptLines(sampleTranscript, opts)
		if len(result) != len(sampleTranscript) {
			t.Errorf("expected %d lines, got %d", len(sampleTranscript), len(result))
		}
	})

	t.Run("response only", func(t *testing.T) {
		opts := displayOptions{responseOnly: true}
		result := filterTranscriptLines(sampleTranscript, opts)

		// Should contain response content
		found := false
		for _, line := range result {
			if strings.Contains(line, "response content") {
				found = true
			}
			// Should NOT contain prompt content
			if strings.Contains(line, "prompt content") {
				t.Error("response-only should not include prompt content")
			}
		}
		if !found {
			t.Error("response-only should include response content")
		}
	})

	t.Run("prompt only", func(t *testing.T) {
		opts := displayOptions{promptOnly: true}
		result := filterTranscriptLines(sampleTranscript, opts)

		// Should contain prompt content
		found := false
		for _, line := range result {
			if strings.Contains(line, "prompt content") {
				found = true
			}
			// Should NOT contain response content
			if strings.Contains(line, "response content") {
				t.Error("prompt-only should not include response content")
			}
		}
		if !found {
			t.Error("prompt-only should include prompt content")
		}
	})

	t.Run("with color", func(t *testing.T) {
		opts := displayOptions{useColor: true}
		result := filterTranscriptLines(sampleTranscript, opts)

		// Prompt lines should have ANSI codes
		foundColoredPrompt := false
		for _, line := range result {
			if strings.Contains(line, "prompt content") {
				if strings.Contains(line, ansiDim) && strings.Contains(line, ansiReset) {
					foundColoredPrompt = true
				}
			}
		}
		if !foundColoredPrompt {
			t.Error("expected prompt lines to have ANSI color codes")
		}

		// Response lines should NOT have dim codes
		for _, line := range result {
			if strings.Contains(line, "response content") {
				if strings.Contains(line, ansiDim) {
					t.Error("response lines should not be dimmed")
				}
			}
		}
	})
}

func TestShowFileContentWithFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-transcript.md")

	content := `# implement - Iteration 1

## Prompt

This is the prompt.

## Response

This is the response.

---
Tokens: 100 input
`
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("response only via file", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		opts := displayOptions{responseOnly: true}
		err := showFileContent(tmpFile, 0, opts)

		_ = w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("showFileContent error: %v", err)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "This is the response") {
			t.Errorf("expected response content in output, got: %q", output)
		}
		if strings.Contains(output, "This is the prompt") {
			t.Errorf("should not contain prompt content in response-only mode, got: %q", output)
		}
	})

	t.Run("with tail limit", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		opts := displayOptions{}
		err := showFileContent(tmpFile, 3, opts) // Only last 3 lines

		_ = w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("showFileContent error: %v", err)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Count lines
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > 3 {
			t.Errorf("expected at most 3 lines, got %d: %v", len(lines), lines)
		}
	})
}

func TestFollowFileWithWatcherNewContent(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(tmpFile, []byte("initial content\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("fsnotify not available: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	// Watch the directory
	if err := watcher.Add(tmpDir); err != nil {
		t.Fatalf("failed to watch directory: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run watcher in goroutine
	done := make(chan error, 1)
	go func() {
		done <- followFileWithWatcher(ctx, tmpFile, watcher)
	}()

	// Write new content after a short delay
	time.Sleep(100 * time.Millisecond)
	f, _ := os.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("watched line 1\n")
	_, _ = f.WriteString("watched line 2\n")
	_ = f.Sync()
	_ = f.Close()

	// Wait for watcher to process (fsnotify can have some delay)
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should see the new lines
	if !strings.Contains(output, "watched line 1") {
		t.Errorf("expected 'watched line 1' in output, got: %q", output)
	}
	if !strings.Contains(output, "watched line 2") {
		t.Errorf("expected 'watched line 2' in output, got: %q", output)
	}
}
