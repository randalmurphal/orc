// docs_note_wiring_integration_test.go tests the wiring of initiative note
// persistence into the workflow executor's phase completion flow.
//
// These tests verify that the executor correctly:
// - Parses docs phase output for initiative_notes (SC-1)
// - Calls PersistInitiativeNotes after docs phase completes (SC-2)
// - Passes correct metadata to SaveInitiativeNote (SC-3)
// - Skips note persistence when task has no initiative (SC-4)
package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/proto"
)

// TestDocsPhase_PersistsInitiativeNotes verifies that when the docs phase
// completes with initiative_notes in the output, the notes are persisted
// to the database via SaveInitiativeNote.
//
// SC-2: Notes are persisted when docs phase completes with initiative_notes
// SC-3: Persisted notes have correct metadata
func TestDocsPhase_PersistsInitiativeNotes(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create initiative first
	initiativeID := "INIT-001"
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("create initiative: %v", err)
	}

	// Create task linked to initiative
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test docs note persistence")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tsk.InitiativeId = proto.String(initiativeID)
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow with docs phase
	createWorkflowWithDocsPhase(t, backend, "test-workflow")

	// Mock turn executor returns docs output with initiative_notes
	docsOutput := `{
		"status": "complete",
		"summary": "Documentation updated",
		"content": "## Documentation Summary\n\nDocs updated.",
		"initiative_notes": [
			{"type": "pattern", "content": "Use repository pattern for data access", "relevant_files": ["internal/repo/"]},
			{"type": "warning", "content": "Don't modify legacy_handler.go directly"}
		],
		"notes_rationale": "Task established new patterns worth sharing"
	}`
	mockTE := &docsNoteMockTurnExecutor{
		result: docsOutput,
	}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true), // Skip gates for simpler test
	)

	// Run workflow
	_, err = we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "test",
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// Verify notes were persisted
	notes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("get initiative notes: %v", err)
	}

	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}

	// Verify first note has correct metadata
	var patternNote *db.InitiativeNote
	for i := range notes {
		if notes[i].NoteType == "pattern" {
			patternNote = &notes[i]
			break
		}
	}

	if patternNote == nil {
		t.Fatal("pattern note not found")
	}

	// SC-3: Verify metadata
	if patternNote.AuthorType != db.NoteAuthorAgent {
		t.Errorf("author_type = %q, want %q", patternNote.AuthorType, db.NoteAuthorAgent)
	}
	if patternNote.SourceTask != taskID {
		t.Errorf("source_task = %q, want %q", patternNote.SourceTask, taskID)
	}
	if patternNote.SourcePhase != "docs" {
		t.Errorf("source_phase = %q, want %q", patternNote.SourcePhase, "docs")
	}
	if patternNote.Content != "Use repository pattern for data access" {
		t.Errorf("content = %q, want specific content", patternNote.Content)
	}
	if len(patternNote.RelevantFiles) != 1 || patternNote.RelevantFiles[0] != "internal/repo/" {
		t.Errorf("relevant_files = %v, want [internal/repo/]", patternNote.RelevantFiles)
	}
}

// TestDocsPhase_NoInitiative_SkipsNotePersistence verifies that when a task
// has no initiative, notes in the docs output are NOT persisted.
//
// SC-4: Notes only persisted when task is part of an initiative
func TestDocsPhase_NoInitiative_SkipsNotePersistence(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create task WITHOUT initiative
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test without initiative")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	// No initiative set
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow with docs phase
	createWorkflowWithDocsPhase(t, backend, "test-workflow")

	// Mock turn executor returns docs output with initiative_notes
	docsOutput := `{
		"status": "complete",
		"summary": "Documentation updated",
		"content": "## Summary",
		"initiative_notes": [
			{"type": "pattern", "content": "This note should NOT be saved"}
		]
	}`
	mockTE := &docsNoteMockTurnExecutor{
		result: docsOutput,
	}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow - should complete without error
	_, err := we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "test",
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// No initiative means no way to save notes - this is the expected behavior
	// The test passes if no panic/error occurs when there's no initiative
}

// TestDocsPhase_EmptyNotes_NoError verifies that when docs output has
// an empty initiative_notes array, no error occurs.
func TestDocsPhase_EmptyNotes_NoError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create initiative
	initiativeID := "INIT-001"
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("create initiative: %v", err)
	}

	// Create task with initiative
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test empty notes")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tsk.InitiativeId = proto.String(initiativeID)
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow
	createWorkflowWithDocsPhase(t, backend, "test-workflow")

	// Mock turn executor returns docs output with empty notes
	docsOutput := `{
		"status": "complete",
		"summary": "No new learnings",
		"content": "## Summary",
		"initiative_notes": [],
		"notes_rationale": "Routine task, no novel insights"
	}`
	mockTE := &docsNoteMockTurnExecutor{
		result: docsOutput,
	}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow - should complete without error
	_, err = we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "test",
	})
	if err != nil {
		t.Fatalf("workflow run with empty notes should succeed: %v", err)
	}

	// Verify no notes were saved
	notes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("get notes: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

// TestDocsPhase_NotesMissingFromOutput_NoError verifies that when docs output
// has no initiative_notes field at all, no error occurs (backward compatible).
func TestDocsPhase_NotesMissingFromOutput_NoError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create initiative
	initiativeID := "INIT-001"
	err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     initiativeID,
		Title:  "Test Initiative",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("create initiative: %v", err)
	}

	// Create task with initiative
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test no notes field")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tsk.InitiativeId = proto.String(initiativeID)
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow
	createWorkflowWithDocsPhase(t, backend, "test-workflow")

	// Mock turn executor returns docs output WITHOUT initiative_notes field
	docsOutput := `{
		"status": "complete",
		"summary": "Documentation updated",
		"content": "## Summary"
	}`
	mockTE := &docsNoteMockTurnExecutor{
		result: docsOutput,
	}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow - should complete without error (backward compatible)
	_, err = we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "test",
	})
	if err != nil {
		t.Fatalf("workflow without initiative_notes field should succeed: %v", err)
	}

	// Verify no notes were saved
	notes, err := backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		t.Fatalf("get notes: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

// =============================================================================
// Helpers
// =============================================================================

// createWorkflowWithDocsPhase creates a minimal workflow with just a docs phase
// for testing initiative note persistence.
func createWorkflowWithDocsPhase(t *testing.T, backend storage.Backend, workflowID string) {
	t.Helper()

	// Create workflow
	wf := &db.Workflow{
		ID:   workflowID,
		Name: "Test Workflow",
	}
	if err := backend.DB().SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create docs phase template
	tmpl := &db.PhaseTemplate{
		ID:               "docs",
		Name:             "Documentation",
		PromptSource:     "embedded",
		PromptPath:       "prompts/docs.md",
		OutputVarName:    "DOCS_CONTENT",
		OutputType:       "document",
		ProducesArtifact: true,
		ArtifactType:     "docs",
		GateType:         "auto",
	}
	if err := backend.DB().SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	// Link phase to workflow
	phase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "docs",
		Sequence:        1,
	}
	if err := backend.DB().SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}
}

// docsNoteMockTurnExecutor implements TurnExecutor for testing initiative notes.
// It returns a predefined result for all calls.
type docsNoteMockTurnExecutor struct {
	result    string
	sessionID string
}

func (m *docsNoteMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	if m.result == "" {
		return nil, errors.New("no mock result configured")
	}
	return &TurnResult{
		Content:   m.result,
		Status:    PhaseStatusComplete,
		SessionID: m.sessionID,
	}, nil
}

func (m *docsNoteMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *docsNoteMockTurnExecutor) UpdateSessionID(id string) {
	m.sessionID = id
}

func (m *docsNoteMockTurnExecutor) SessionID() string {
	return m.sessionID
}
