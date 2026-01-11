package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashContent(t *testing.T) {
	content := "Hello, World!"
	hash1 := HashContent(content)
	hash2 := HashContent(content)

	if hash1 != hash2 {
		t.Errorf("HashContent() not deterministic: %s != %s", hash1, hash2)
	}

	differentHash := HashContent("Different content")
	if hash1 == differentHash {
		t.Error("HashContent() returned same hash for different content")
	}

	if len(hash1) != 64 {
		t.Errorf("HashContent() returned unexpected length: %d", len(hash1))
	}
}

func TestSaveSpecTo(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task-001")

	specContent := "# Test Spec\n\nThis is a test specification."

	// Save spec
	err := SaveSpecTo(taskDir, specContent, "test")
	if err != nil {
		t.Fatalf("SaveSpecTo() error: %v", err)
	}

	// Verify files exist
	specPath := filepath.Join(taskDir, "spec.md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("Spec file was not created")
	}

	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("Spec metadata file was not created")
	}

	// Verify content
	data, _ := os.ReadFile(specPath)
	if string(data) != specContent {
		t.Errorf("Spec content mismatch: got %q, want %q", string(data), specContent)
	}
}

func TestLoadSpecFrom(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task-001")

	specContent := "# Load Test Spec"
	SaveSpecTo(taskDir, specContent, "test-source")

	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Fatalf("LoadSpecFrom() error: %v", err)
	}
	if spec == nil {
		t.Fatal("LoadSpecFrom() returned nil")
	}

	if spec.Content != specContent {
		t.Errorf("Spec content = %q, want %q", spec.Content, specContent)
	}

	if spec.Metadata.Source != "test-source" {
		t.Errorf("Spec source = %q, want %q", spec.Metadata.Source, "test-source")
	}

	expectedHash := HashContent(specContent)
	if spec.Metadata.Hash != expectedHash {
		t.Errorf("Spec hash = %q, want %q", spec.Metadata.Hash, expectedHash)
	}
}

func TestLoadSpecFromNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "nonexistent")

	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Errorf("LoadSpecFrom() for non-existent should return nil, not error: %v", err)
	}
	if spec != nil {
		t.Error("LoadSpecFrom() for non-existent should return nil")
	}
}

func TestSpecChangedLogic(t *testing.T) {
	// Test the hash comparison logic directly
	content1 := "Original content"
	content2 := "Different content"

	hash1 := HashContent(content1)
	hash2 := HashContent(content2)

	if hash1 == hash2 {
		t.Error("Different content should have different hashes")
	}

	// Same content should have same hash
	hash1Again := HashContent(content1)
	if hash1 != hash1Again {
		t.Error("Same content should have same hash")
	}
}

func TestSaveAndLoadSpecRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "roundtrip-task")

	specContent := `# Feature Spec

## Overview
This is a comprehensive test spec.

## Requirements
- Requirement 1
- Requirement 2

## Technical Details
Some technical details here.
`

	// Save
	err := SaveSpecTo(taskDir, specContent, "generated")
	if err != nil {
		t.Fatalf("SaveSpecTo() error: %v", err)
	}

	// Load
	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Fatalf("LoadSpecFrom() error: %v", err)
	}

	// Verify roundtrip
	if spec.Content != specContent {
		t.Errorf("Content changed during roundtrip")
	}
	if spec.Metadata.Source != "generated" {
		t.Errorf("Source = %q, want 'generated'", spec.Metadata.Source)
	}
	if spec.Metadata.LastSyncedAt.IsZero() {
		t.Error("LastSyncedAt should be set")
	}
}
