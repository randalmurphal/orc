package db

import (
	"testing"
)

func TestConstitution(t *testing.T) {
	db, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory: %v", err)
	}
	defer func() { _ = db.Close() }()

	t.Run("SaveAndLoad", func(t *testing.T) {
		c := &Constitution{
			Content: "# Project Principles\n\n1. No silent failures\n2. Always test",
			Version: "1.0.0",
		}

		// Save
		err := db.SaveConstitution(c)
		if err != nil {
			t.Fatalf("SaveConstitution: %v", err)
		}

		// Verify hash was computed
		if c.ContentHash == "" {
			t.Error("ContentHash should be set after save")
		}

		// Load
		loaded, err := db.LoadConstitution()
		if err != nil {
			t.Fatalf("LoadConstitution: %v", err)
		}

		if loaded.Content != c.Content {
			t.Errorf("Content mismatch: got %q, want %q", loaded.Content, c.Content)
		}
		if loaded.Version != c.Version {
			t.Errorf("Version mismatch: got %q, want %q", loaded.Version, c.Version)
		}
		if loaded.ContentHash != c.ContentHash {
			t.Errorf("ContentHash mismatch: got %q, want %q", loaded.ContentHash, c.ContentHash)
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		// Update existing constitution
		c := &Constitution{
			Content: "# Updated Principles\n\n1. Test everything",
			Version: "2.0.0",
		}

		err := db.SaveConstitution(c)
		if err != nil {
			t.Fatalf("SaveConstitution (update): %v", err)
		}

		loaded, err := db.LoadConstitution()
		if err != nil {
			t.Fatalf("LoadConstitution: %v", err)
		}

		if loaded.Version != "2.0.0" {
			t.Errorf("Version should be 2.0.0, got %q", loaded.Version)
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

		// Load should return ErrNoConstitution
		_, err = db.LoadConstitution()
		if err != ErrNoConstitution {
			t.Errorf("LoadConstitution should return ErrNoConstitution, got %v", err)
		}
	})
}

func TestConstitutionCheck(t *testing.T) {
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

	// First save a constitution
	c := &Constitution{
		Content: "# Rules",
		Version: "1.0.0",
	}
	if err := db.SaveConstitution(c); err != nil {
		t.Fatalf("SaveConstitution: %v", err)
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
