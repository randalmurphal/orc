package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// Tests for initiative notes command (list notes)
// =============================================================================

func TestInitiativeNotesCommand_Structure(t *testing.T) {
	cmd := newInitiativeNotesCmd()

	// Verify command structure
	if cmd.Use != "notes <initiative-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "notes <initiative-id>")
	}

	// Verify flags exist
	if cmd.Flag("type") == nil {
		t.Error("missing --type flag")
	}
}

func TestInitiativeNotesCommand_RequiresArg(t *testing.T) {
	cmd := newInitiativeNotesCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"INIT-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"INIT-001", "extra"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestInitiativeNotesListEmpty(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative without notes
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "No notes found") {
		t.Errorf("expected 'No notes found' message, got: %s", output)
	}
}

func TestInitiativeNotesListWithNotes(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create notes
	note1 := &db.InitiativeNote{
		ID:           "NOTE-001",
		InitiativeID: "INIT-001",
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     db.NoteTypePattern,
		Content:      "Use factory pattern",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note1); err != nil {
		t.Fatalf("save note1: %v", err)
	}

	note2 := &db.InitiativeNote{
		ID:           "NOTE-002",
		InitiativeID: "INIT-001",
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     db.NoteTypeWarning,
		Content:      "Avoid deprecated API",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note2); err != nil {
		t.Fatalf("save note2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify both notes are displayed
	if !strings.Contains(output, "NOTE-001") {
		t.Errorf("expected NOTE-001 in output, got: %s", output)
	}
	if !strings.Contains(output, "NOTE-002") {
		t.Errorf("expected NOTE-002 in output, got: %s", output)
	}
	if !strings.Contains(output, "Use factory pattern") {
		t.Errorf("expected note content in output, got: %s", output)
	}
	if !strings.Contains(output, "2 total") {
		t.Errorf("expected '2 total' in output, got: %s", output)
	}
}

func TestInitiativeNotesListFilterByType(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create notes of different types
	note1 := &db.InitiativeNote{
		ID:           "NOTE-001",
		InitiativeID: "INIT-001",
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     db.NoteTypePattern,
		Content:      "Pattern note",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note1); err != nil {
		t.Fatalf("save note1: %v", err)
	}

	note2 := &db.InitiativeNote{
		ID:           "NOTE-002",
		InitiativeID: "INIT-001",
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     db.NoteTypeWarning,
		Content:      "Warning note",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note2); err != nil {
		t.Fatalf("save note2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-001", "--type", "warning"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should only show warning note
	if strings.Contains(output, "Pattern note") {
		t.Errorf("pattern note should be filtered out, got: %s", output)
	}
	if !strings.Contains(output, "Warning note") {
		t.Errorf("expected warning note in output, got: %s", output)
	}
}

func TestInitiativeNotesInvalidType(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-001", "--type", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid note type") {
		t.Errorf("error should mention 'invalid note type', got: %v", err)
	}
}

func TestInitiativeNotesNonexistentInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Close backend before running command
	_ = backend.Close()

	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-999"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent initiative")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// =============================================================================
// Tests for initiative note add command
// =============================================================================

func TestInitiativeNoteAddCommand_Structure(t *testing.T) {
	cmd := newInitiativeNoteAddCmd()

	// Verify command structure
	if cmd.Use != "add <initiative-id> <content>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "add <initiative-id> <content>")
	}

	// Verify flags exist
	if cmd.Flag("type") == nil {
		t.Error("missing --type flag")
	}
}

func TestInitiativeNoteAddCommand_RequiresArgs(t *testing.T) {
	cmd := newInitiativeNoteAddCmd()

	// Should require exactly two arguments
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"INIT-001"}); err == nil {
		t.Error("expected error for one arg")
	}
	if err := cmd.Args(cmd, []string{"INIT-001", "content"}); err != nil {
		t.Errorf("unexpected error for two args: %v", err)
	}
}

func TestInitiativeNoteAddSuccess(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeNoteAddCmd()
	cmd.SetArgs([]string{"INIT-001", "Test note content", "--type", "pattern"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output shows note was added
	if !strings.Contains(output, "Note added") {
		t.Errorf("expected 'Note added' message, got: %s", output)
	}
	if !strings.Contains(output, "pattern") {
		t.Errorf("expected 'pattern' type in output, got: %s", output)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify note was saved
	notes, err := backend.GetInitiativeNotes("INIT-001")
	if err != nil {
		t.Fatalf("get notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Content != "Test note content" {
		t.Errorf("note content = %q, want %q", notes[0].Content, "Test note content")
	}
	if notes[0].NoteType != db.NoteTypePattern {
		t.Errorf("note type = %q, want %q", notes[0].NoteType, db.NoteTypePattern)
	}
	if notes[0].AuthorType != db.NoteAuthorHuman {
		t.Errorf("note author type = %q, want %q", notes[0].AuthorType, db.NoteAuthorHuman)
	}
}

func TestInitiativeNoteAddInvalidType(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	cmd := newInitiativeNoteAddCmd()
	cmd.SetArgs([]string{"INIT-001", "content", "--type", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid note type") {
		t.Errorf("error should mention 'invalid note type', got: %v", err)
	}
}

func TestInitiativeNoteAddNonexistentInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Close backend before running command
	_ = backend.Close()

	cmd := newInitiativeNoteAddCmd()
	cmd.SetArgs([]string{"INIT-999", "content", "--type", "pattern"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent initiative")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestInitiativeNoteAddAllTypes(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test all valid types
	types := []string{"pattern", "warning", "learning", "handoff"}
	for _, noteType := range types {
		cmd := newInitiativeNoteAddCmd()
		cmd.SetArgs([]string{"INIT-001", "Content for " + noteType, "--type", noteType})
		if err := cmd.Execute(); err != nil {
			t.Errorf("failed to add %s note: %v", noteType, err)
		}
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	notes, err := backend.GetInitiativeNotes("INIT-001")
	if err != nil {
		t.Fatalf("get notes: %v", err)
	}
	if len(notes) != 4 {
		t.Errorf("expected 4 notes, got %d", len(notes))
	}
}

// =============================================================================
// Tests for initiative note delete command
// =============================================================================

func TestInitiativeNoteDeleteCommand_Structure(t *testing.T) {
	cmd := newInitiativeNoteDeleteCmd()

	// Verify command structure
	if cmd.Use != "delete <note-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "delete <note-id>")
	}

	// Verify flags exist
	if cmd.Flag("force") == nil {
		t.Error("missing --force flag")
	}
}

func TestInitiativeNoteDeleteCommand_RequiresArg(t *testing.T) {
	cmd := newInitiativeNoteDeleteCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"NOTE-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
}

func TestInitiativeNoteDeleteSuccess(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative and note
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	note := &db.InitiativeNote{
		ID:           "NOTE-001",
		InitiativeID: "INIT-001",
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     db.NoteTypePattern,
		Content:      "Test note",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note); err != nil {
		t.Fatalf("save note: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run delete with --force to skip confirmation
	cmd := newInitiativeNoteDeleteCmd()
	cmd.SetArgs([]string{"NOTE-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify note was deleted
	deletedNote, err := backend.GetInitiativeNote("NOTE-001")
	if err != nil {
		t.Fatalf("get note error: %v", err)
	}
	if deletedNote != nil {
		t.Error("note should have been deleted")
	}
}

func TestInitiativeNoteDeleteNonexistent(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Close backend before running command
	_ = backend.Close()

	cmd := newInitiativeNoteDeleteCmd()
	cmd.SetArgs([]string{"NOTE-999", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent note")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// =============================================================================
// Tests for show --notes flag
// =============================================================================

func TestShowCommandWithNotesFlag(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task
	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(tk, "INIT-001")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create note linked to the task
	note := &db.InitiativeNote{
		ID:           "NOTE-001",
		InitiativeID: "INIT-001",
		Author:       "agent",
		AuthorType:   db.NoteAuthorAgent,
		SourceTask:   "TASK-001",
		SourcePhase:  "docs",
		NoteType:     db.NoteTypeLearning,
		Content:      "Key insight from implementation",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveInitiativeNote(note); err != nil {
		t.Fatalf("save note: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-001", "--notes"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify notes section is displayed
	if !strings.Contains(output, "Task Notes") {
		t.Errorf("expected 'Task Notes' section, got: %s", output)
	}
	if !strings.Contains(output, "NOTE-001") {
		t.Errorf("expected NOTE-001 in output, got: %s", output)
	}
	if !strings.Contains(output, "Key insight") {
		t.Errorf("expected note content in output, got: %s", output)
	}
}

func TestShowCommandNotesEmpty(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task without any notes
	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-001", "--notes"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify empty notes message is displayed
	if !strings.Contains(output, "Task Notes") {
		t.Errorf("expected 'Task Notes' section, got: %s", output)
	}
	if !strings.Contains(output, "No notes generated") {
		t.Errorf("expected 'No notes generated' message, got: %s", output)
	}
}

// =============================================================================
// Tests for initiative show with notes display
// =============================================================================

func TestInitiativeShowDisplaysNotesCount(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create notes
	for i := 0; i < 3; i++ {
		note := &db.InitiativeNote{
			ID:           "NOTE-" + filepath.Base(tmpDir) + "-" + string(rune('1'+i)),
			InitiativeID: "INIT-001",
			Author:       "human",
			AuthorType:   db.NoteAuthorHuman,
			NoteType:     db.NoteTypePattern,
			Content:      "Test note content",
			CreatedAt:    time.Now(),
		}
		if err := backend.SaveInitiativeNote(note); err != nil {
			t.Fatalf("save note %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Use notes list command to verify count
	cmd := newInitiativeNotesCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify notes count in output
	if !strings.Contains(output, "3 total") {
		t.Errorf("expected '3 total' in output, got: %s", output)
	}
}
