package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world")

	err := AtomicWriteFile(path, content, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	// Verify file exists and has correct content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}

	// Verify permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("permissions mismatch: got %o, want %o", info.Mode().Perm(), 0644)
	}
}

func TestAtomicWriteFile_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "test.txt")
	content := []byte("nested content")

	err := AtomicWriteFile(path, content, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}
}

func TestAtomicWriteFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Write initial content
	if err := AtomicWriteFile(path, []byte("initial"), 0644); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Overwrite
	if err := AtomicWriteFile(path, []byte("updated"), 0644); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "updated" {
		t.Errorf("content mismatch: got %q, want %q", data, "updated")
	}
}

func TestAtomicWriteFile_NoTempFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	// Check no temp files remain
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "test.txt" {
			t.Errorf("unexpected file in directory: %s", entry.Name())
		}
	}
}

func TestAtomicWriteFileString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello string"

	err := AtomicWriteFileString(path, content, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFileString failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}
}
