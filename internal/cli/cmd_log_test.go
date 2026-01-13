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
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
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
	f.WriteString("new line 1\n")
	f.WriteString("new line 2\n")
	f.Close()

	// Wait for completion
	<-done

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
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
	os.WriteFile(tmpFile, []byte("new\n"), 0o644) // This truncates

	// Give it time to detect
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-done

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should see truncation message
	if !strings.Contains(output, "truncated") {
		t.Errorf("expected truncation message in output, got: %q", output)
	}
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
	defer watcher.Close()

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
	f.WriteString("watched line 1\n")
	f.WriteString("watched line 2\n")
	f.Sync()
	f.Close()

	// Wait for watcher to process (fsnotify can have some delay)
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should see the new lines
	if !strings.Contains(output, "watched line 1") {
		t.Errorf("expected 'watched line 1' in output, got: %q", output)
	}
	if !strings.Contains(output, "watched line 2") {
		t.Errorf("expected 'watched line 2' in output, got: %q", output)
	}
}
