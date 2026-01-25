package cli

import (
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TestPRPersistence verifies that PR info is properly saved and loaded from the database
func TestPRPersistence(t *testing.T) {
	// Use a fixed temp dir so we can inspect the database
	tmpDir := "/tmp/orc-pr-persist-test"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origWd)
		// Don't remove tmpDir so we can inspect it
	})

	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	// Create task with PR info
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusCompleted
	tk.SetPRInfo("https://github.com/owner/repo/pull/42", 42)
	tk.PR.Status = task.PRStatusPendingReview

	t.Logf("Before save - HasPR: %v, Number: %d", tk.HasPR(), tk.PR.Number)

	// Check if pr_info column exists
	dbPath := tmpDir + "/.orc/orc.db"
	t.Logf("Database path: %s", dbPath)

	// Save task
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load task
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	t.Logf("After load - HasPR: %v", loaded.HasPR())
	if loaded.PR != nil {
		t.Logf("PR Number: %d, URL: %s, Status: %s", loaded.PR.Number, loaded.PR.URL, loaded.PR.Status)
	} else {
		t.Error("PR is nil after load!")
	}

	// Verify PR info persisted correctly
	if !loaded.HasPR() {
		t.Error("Task should have PR info after load")
	}
	if loaded.PR.Number != 42 {
		t.Errorf("Expected PR number 42, got %d", loaded.PR.Number)
	}
	if loaded.PR.URL != "https://github.com/owner/repo/pull/42" {
		t.Errorf("Expected PR URL to match, got %s", loaded.PR.URL)
	}
	if loaded.PR.Status != task.PRStatusPendingReview {
		t.Errorf("Expected PR status pending_review, got %s", loaded.PR.Status)
	}
}
