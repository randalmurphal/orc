package executor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// ValidNoteTypes defines the allowed note types for initiative notes.
var ValidNoteTypes = map[string]bool{
	"pattern":  true,
	"warning":  true,
	"learning": true,
	"handoff":  true,
}

// InitiativeNoteOutput represents a note extracted by the knowledge curator sub-agent.
type InitiativeNoteOutput struct {
	Type          string   `json:"type"`
	Content       string   `json:"content"`
	RelevantFiles []string `json:"relevant_files,omitempty"`
}

// DocsResponse represents the structured response from a docs phase execution.
type DocsResponse struct {
	Status          string                 `json:"status"`
	Summary         string                 `json:"summary,omitempty"`
	Reason          string                 `json:"reason,omitempty"`
	Content         string                 `json:"content,omitempty"`
	InitiativeNotes []InitiativeNoteOutput `json:"initiative_notes,omitempty"`
	NotesRationale  string                 `json:"notes_rationale,omitempty"`
}

// ParseDocsResponse parses docs phase JSON response with initiative_notes support.
// Validates that all notes have required fields (type, content) and valid type values.
func ParseDocsResponse(content string) (*DocsResponse, error) {
	var resp DocsResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid docs response JSON: %w", err)
	}

	// Validate status is one of the expected values
	switch resp.Status {
	case "complete", "blocked", "continue":
		// Valid
	default:
		return nil, fmt.Errorf("invalid docs status: %q (expected complete, blocked, or continue)", resp.Status)
	}

	// Validate each initiative note
	for i, note := range resp.InitiativeNotes {
		if strings.TrimSpace(note.Type) == "" {
			return nil, fmt.Errorf("initiative_notes[%d]: missing required field 'type'", i)
		}
		if !ValidNoteTypes[note.Type] {
			return nil, fmt.Errorf("initiative_notes[%d]: invalid type %q (expected pattern, warning, learning, or handoff)", i, note.Type)
		}
		if strings.TrimSpace(note.Content) == "" {
			return nil, fmt.Errorf("initiative_notes[%d]: missing required field 'content'", i)
		}
	}

	return &resp, nil
}

// PersistInitiativeNotes saves initiative notes extracted from docs phase output.
// Notes are only persisted when the task is part of an initiative (initiativeID non-empty).
// Each note is saved with author_type="agent" and source_phase="docs".
func PersistInitiativeNotes(backend storage.Backend, notes []InitiativeNoteOutput, taskID, initiativeID string) error {
	// Skip if no initiative (task not linked)
	if initiativeID == "" {
		return nil
	}

	// Skip if no notes to persist
	if len(notes) == 0 {
		return nil
	}

	for _, note := range notes {
		// Generate unique ID for each note
		noteID, err := backend.GetNextNoteID()
		if err != nil {
			return fmt.Errorf("generate note ID: %w", err)
		}

		dbNote := &db.InitiativeNote{
			ID:            noteID,
			InitiativeID:  initiativeID,
			AuthorType:    db.NoteAuthorAgent,
			SourceTask:    taskID,
			SourcePhase:   "docs",
			NoteType:      note.Type,
			Content:       note.Content,
			RelevantFiles: note.RelevantFiles,
			Graduated:     false, // Agent notes require graduation per DEC-003
		}

		if err := backend.SaveInitiativeNote(dbNote); err != nil {
			return fmt.Errorf("save initiative note: %w", err)
		}
	}

	return nil
}
