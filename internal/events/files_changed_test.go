package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFilesChangedUpdate_Serialization(t *testing.T) {
	now := time.Now().UTC()
	update := FilesChangedUpdate{
		Files: []ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			{Path: "file2.go", Status: "added", Additions: 20, Deletions: 0},
		},
		TotalAdditions: 30,
		TotalDeletions: 5,
		Timestamp:      now,
	}

	// Marshal to JSON
	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded FilesChangedUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if len(decoded.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(decoded.Files))
	}
	if decoded.TotalAdditions != 30 {
		t.Errorf("expected total additions 30, got %d", decoded.TotalAdditions)
	}
	if decoded.TotalDeletions != 5 {
		t.Errorf("expected total deletions 5, got %d", decoded.TotalDeletions)
	}
	if !decoded.Timestamp.Equal(now) {
		t.Errorf("expected timestamp %v, got %v", now, decoded.Timestamp)
	}
}

func TestChangedFile_Serialization(t *testing.T) {
	file := ChangedFile{
		Path:      "test.go",
		Status:    "modified",
		Additions: 10,
		Deletions: 5,
	}

	// Marshal to JSON
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded ChangedFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.Path != "test.go" {
		t.Errorf("expected path test.go, got %s", decoded.Path)
	}
	if decoded.Status != "modified" {
		t.Errorf("expected status modified, got %s", decoded.Status)
	}
	if decoded.Additions != 10 {
		t.Errorf("expected additions 10, got %d", decoded.Additions)
	}
	if decoded.Deletions != 5 {
		t.Errorf("expected deletions 5, got %d", decoded.Deletions)
	}
}

func TestFilesChangedUpdate_EmptyFiles(t *testing.T) {
	update := FilesChangedUpdate{
		Files:          []ChangedFile{},
		TotalAdditions: 0,
		TotalDeletions: 0,
		Timestamp:      time.Now(),
	}

	// Marshal to JSON
	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded FilesChangedUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify empty files array (not nil)
	if decoded.Files == nil {
		t.Error("expected non-nil files array")
	}
	if len(decoded.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(decoded.Files))
	}
}

func TestEventFilesChanged_Constant(t *testing.T) {
	if EventFilesChanged != "files_changed" {
		t.Errorf("expected EventFilesChanged to be 'files_changed', got %s", EventFilesChanged)
	}
}

func TestFilesChangedUpdate_InEvent(t *testing.T) {
	update := FilesChangedUpdate{
		Files: []ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
		},
		TotalAdditions: 10,
		TotalDeletions: 5,
		Timestamp:      time.Now(),
	}

	event := NewEvent(EventFilesChanged, "TASK-001", update)

	if event.Type != EventFilesChanged {
		t.Errorf("expected type %s, got %s", EventFilesChanged, event.Type)
	}
	if event.TaskID != "TASK-001" {
		t.Errorf("expected task ID TASK-001, got %s", event.TaskID)
	}

	// Verify we can type assert the data
	data, ok := event.Data.(FilesChangedUpdate)
	if !ok {
		t.Fatalf("expected data to be FilesChangedUpdate, got %T", event.Data)
	}
	if len(data.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(data.Files))
	}
}
