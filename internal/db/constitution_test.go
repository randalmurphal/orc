package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstitution(t *testing.T) {
	// Constitution is file-based, needs a real directory
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	db, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	defer func() { _ = db.Close() }()

	t.Run("SaveAndLoad", func(t *testing.T) {
		content := "# Project Principles\n\n1. No silent failures\n2. Always test"

		// Save
		err := db.SaveConstitution(content)
		if err != nil {
			t.Fatalf("SaveConstitution: %v", err)
		}

		// Verify file was created
		constitutionPath := filepath.Join(orcDir, "CONSTITUTION.md")
		if _, err := os.Stat(constitutionPath); os.IsNotExist(err) {
			t.Error("Constitution file should exist at .orc/CONSTITUTION.md")
		}

		// Load
		loaded, err := db.LoadConstitution()
		if err != nil {
			t.Fatalf("LoadConstitution: %v", err)
		}

		if loaded.Content != content {
			t.Errorf("Content mismatch: got %q, want %q", loaded.Content, content)
		}
		if loaded.Path != constitutionPath {
			t.Errorf("Path mismatch: got %q, want %q", loaded.Path, constitutionPath)
		}
		if loaded.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set from file mtime")
		}
	})

	t.Run("Update", func(t *testing.T) {
		// Update existing constitution
		newContent := "# Updated Principles\n\n1. Test everything"

		err := db.SaveConstitution(newContent)
		if err != nil {
			t.Fatalf("SaveConstitution (update): %v", err)
		}

		loaded, err := db.LoadConstitution()
		if err != nil {
			t.Fatalf("LoadConstitution: %v", err)
		}

		if loaded.Content != newContent {
			t.Errorf("Content should be updated, got %q", loaded.Content)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := db.ConstitutionExists()
		if err != nil {
			t.Fatalf("ConstitutionExists: %v", err)
		}
		if !exists {
			t.Error("Constitution should exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := db.DeleteConstitution()
		if err != nil {
			t.Fatalf("DeleteConstitution: %v", err)
		}

		exists, err := db.ConstitutionExists()
		if err != nil {
			t.Fatalf("ConstitutionExists: %v", err)
		}
		if exists {
			t.Error("Constitution should not exist after delete")
		}

		// Verify file is gone
		constitutionPath := filepath.Join(orcDir, "CONSTITUTION.md")
		if _, err := os.Stat(constitutionPath); !os.IsNotExist(err) {
			t.Error("Constitution file should be deleted")
		}

		// Load should return ErrNoConstitution
		_, err = db.LoadConstitution()
		if err != ErrNoConstitution {
			t.Errorf("LoadConstitution should return ErrNoConstitution, got %v", err)
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		// Deleting when no file exists should not error
		err := db.DeleteConstitution()
		if err != nil {
			t.Errorf("DeleteConstitution on non-existent should not error: %v", err)
		}
	})
}

func TestConstitutionCheck(t *testing.T) {
	// Constitution checks are still database-backed
	db, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a task first (constitution checks have FK constraint)
	task := &Task{
		ID:     "TASK-001",
		Title:  "Test Task",
		Status: "created",
		Weight: "small",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	t.Run("SaveAndLoadCheck", func(t *testing.T) {
		check := &ConstitutionCheck{
			TaskID:     "TASK-001",
			Phase:      "spec",
			Passed:     false,
			Violations: []string{"Missing test plan", "No success criteria"},
		}

		err := db.SaveConstitutionCheck(check)
		if err != nil {
			t.Fatalf("SaveConstitutionCheck: %v", err)
		}

		if check.ID == 0 {
			t.Error("ID should be set after save")
		}

		// Get latest check
		latest, err := db.GetLatestConstitutionCheck("TASK-001", "spec")
		if err != nil {
			t.Fatalf("GetLatestConstitutionCheck: %v", err)
		}
		if latest == nil {
			t.Fatal("Should find latest check")
		}

		if latest.Passed {
			t.Error("Check should not be passed")
		}
		if len(latest.Violations) != 2 {
			t.Errorf("Expected 2 violations, got %d", len(latest.Violations))
		}
	})

	t.Run("MultipleChecks", func(t *testing.T) {
		// Save a passing check
		check := &ConstitutionCheck{
			TaskID: "TASK-001",
			Phase:  "spec",
			Passed: true,
		}

		err := db.SaveConstitutionCheck(check)
		if err != nil {
			t.Fatalf("SaveConstitutionCheck: %v", err)
		}

		// Get all checks
		checks, err := db.GetConstitutionChecks("TASK-001")
		if err != nil {
			t.Fatalf("GetConstitutionChecks: %v", err)
		}

		if len(checks) != 2 {
			t.Errorf("Expected 2 checks, got %d", len(checks))
		}

		// Latest should be passing
		latest, _ := db.GetLatestConstitutionCheck("TASK-001", "spec")
		if !latest.Passed {
			t.Error("Latest check should be passed")
		}
	})

	t.Run("NoCheckFound", func(t *testing.T) {
		latest, err := db.GetLatestConstitutionCheck("TASK-999", "spec")
		if err != nil {
			t.Fatalf("GetLatestConstitutionCheck: %v", err)
		}
		if latest != nil {
			t.Error("Should return nil for non-existent check")
		}
	})
}
