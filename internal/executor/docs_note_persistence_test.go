package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestPersistInitiativeNotes_Basic tests basic note persistence.
// SC-2: Notes are persisted when docs phase completes with initiative_notes
// SC-3: Persisted notes have correct metadata
func TestPersistInitiativeNotes_Basic(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "INIT-001"

	// Create an initiative first
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("failed to create initiative: %v", err)
	}

	// Sample docs response with notes
	docsOutput := `{
		"status": "complete",
		"summary": "Documentation updated",
		"content": "## Summary",
		"initiative_notes": [
			{"type": "pattern", "content": "Use repository pattern for data access", "relevant_files": ["internal/repo/"]},
			{"type": "warning", "content": "Don't modify legacy_handler.go directly"}
		]
	}`

	// Parse and persist notes
	resp, err := ParseDocsResponse(docsOutput)
	if err != nil {
		t.Fatalf("ParseDocsResponse failed: %v", err)
	}

	err = PersistInitiativeNotes(backend, resp.InitiativeNotes, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes failed: %v", err)
	}

	// Verify notes were saved
	notes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("GetInitiativeNotes failed: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}

	// Verify first note
	found := false
	for _, n := range notes {
		if n.NoteType == "pattern" && n.Content == "Use repository pattern for data access" {
			found = true
			// SC-3: Verify metadata
			if n.AuthorType != db.NoteAuthorAgent {
				t.Errorf("note author_type = %q, want %q", n.AuthorType, db.NoteAuthorAgent)
			}
			if n.SourceTask != taskID {
				t.Errorf("note source_task = %q, want %q", n.SourceTask, taskID)
			}
			if n.SourcePhase != "docs" {
				t.Errorf("note source_phase = %q, want %q", n.SourcePhase, "docs")
			}
			if len(n.RelevantFiles) != 1 || n.RelevantFiles[0] != "internal/repo/" {
				t.Errorf("note relevant_files = %v, want [internal/repo/]", n.RelevantFiles)
			}
		}
	}
	if !found {
		t.Error("pattern note not found")
	}
}

// TestPersistInitiativeNotes_EmptyNotes tests that empty notes array doesn't error.
// SC-4: Notes only persisted when task is part of an initiative
func TestPersistInitiativeNotes_EmptyNotes(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "INIT-001"

	// Create an initiative first
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("failed to create initiative: %v", err)
	}

	// Empty notes array
	err = PersistInitiativeNotes(backend, []InitiativeNoteOutput{}, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes should not error on empty notes: %v", err)
	}

	// Verify no notes were saved
	notes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("GetInitiativeNotes failed: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

// TestPersistInitiativeNotes_NilNotes tests that nil notes array doesn't error.
func TestPersistInitiativeNotes_NilNotes(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "INIT-001"

	// nil notes array (no initiative_notes in output)
	err := PersistInitiativeNotes(backend, nil, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes should not error on nil notes: %v", err)
	}
}

// TestPersistInitiativeNotes_NoInitiative tests that notes are skipped when no initiative.
// SC-4: Notes only persisted when task is part of an initiative
func TestPersistInitiativeNotes_NoInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "" // No initiative

	notes := []InitiativeNoteOutput{
		{Type: "pattern", Content: "test pattern"},
	}

	// Should not error, but should skip persistence
	err := PersistInitiativeNotes(backend, notes, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes should not error when no initiative: %v", err)
	}

	// No way to verify nothing was saved without an initiative, but it shouldn't crash
}

// TestPersistInitiativeNotes_AllNoteTypes tests all valid note types.
// SC-5: Each note has required fields: type (pattern/warning/learning/handoff), content
func TestPersistInitiativeNotes_AllNoteTypes(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "INIT-001"

	// Create an initiative first
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("failed to create initiative: %v", err)
	}

	notes := []InitiativeNoteOutput{
		{Type: "pattern", Content: "pattern content"},
		{Type: "warning", Content: "warning content"},
		{Type: "learning", Content: "learning content"},
		{Type: "handoff", Content: "handoff content"},
	}

	err = PersistInitiativeNotes(backend, notes, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes failed: %v", err)
	}

	// Verify all notes were saved with correct types
	savedNotes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("GetInitiativeNotes failed: %v", err)
	}

	if len(savedNotes) != 4 {
		t.Errorf("expected 4 notes, got %d", len(savedNotes))
	}

	typeCount := make(map[string]int)
	for _, n := range savedNotes {
		typeCount[n.NoteType]++
	}

	for _, expectedType := range []string{"pattern", "warning", "learning", "handoff"} {
		if typeCount[expectedType] != 1 {
			t.Errorf("expected 1 note of type %q, got %d", expectedType, typeCount[expectedType])
		}
	}
}

// TestPersistInitiativeNotes_RelevantFilesPreserved tests that relevant_files are preserved.
// SC-6: Optional relevant_files array is preserved when provided
func TestPersistInitiativeNotes_RelevantFilesPreserved(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskID := "TASK-001"
	initiativeID := "INIT-001"

	// Create an initiative first
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("failed to create initiative: %v", err)
	}

	notes := []InitiativeNoteOutput{
		{
			Type:          "pattern",
			Content:       "Use this pattern",
			RelevantFiles: []string{"internal/repo/", "pkg/utils/helper.go", "cmd/main.go"},
		},
	}

	err = PersistInitiativeNotes(backend, notes, taskID, initiativeID)
	if err != nil {
		t.Fatalf("PersistInitiativeNotes failed: %v", err)
	}

	savedNotes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("GetInitiativeNotes failed: %v", err)
	}

	if len(savedNotes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(savedNotes))
	}

	note := savedNotes[0]
	if len(note.RelevantFiles) != 3 {
		t.Errorf("expected 3 relevant_files, got %d", len(note.RelevantFiles))
	}

	expectedFiles := []string{"internal/repo/", "pkg/utils/helper.go", "cmd/main.go"}
	for i, f := range expectedFiles {
		if i < len(note.RelevantFiles) && note.RelevantFiles[i] != f {
			t.Errorf("relevant_files[%d] = %q, want %q", i, note.RelevantFiles[i], f)
		}
	}
}
