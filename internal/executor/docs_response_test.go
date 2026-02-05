package executor

import (
	"testing"
)

// TestParseDocsResponse tests parsing of docs phase output with initiative_notes.
// SC-1: Docs phase output schema includes `initiative_notes` field
// SC-5: Each note has required fields: type and content
func TestParseDocsResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		content       string
		wantStatus    string
		wantSummary   string
		wantNotes     int
		wantErr       bool
		wantErrContains string
	}{
		{
			name: "complete with initiative_notes",
			content: `{
				"status": "complete",
				"summary": "Documentation updated",
				"content": "## Summary\n\nDocs updated.",
				"initiative_notes": [
					{"type": "pattern", "content": "Use repository pattern for data access", "relevant_files": ["internal/repo/"]},
					{"type": "warning", "content": "Don't modify legacy_handler.go directly"}
				]
			}`,
			wantStatus:  "complete",
			wantSummary: "Documentation updated",
			wantNotes:   2,
			wantErr:     false,
		},
		{
			name: "complete without initiative_notes",
			content: `{
				"status": "complete",
				"summary": "Documentation updated",
				"content": "## Summary\n\nDocs updated."
			}`,
			wantStatus:  "complete",
			wantSummary: "Documentation updated",
			wantNotes:   0,
			wantErr:     false,
		},
		{
			name: "complete with empty initiative_notes",
			content: `{
				"status": "complete",
				"summary": "No new learnings",
				"content": "## Summary\n\nDocs updated.",
				"initiative_notes": []
			}`,
			wantStatus:  "complete",
			wantSummary: "No new learnings",
			wantNotes:   0,
			wantErr:     false,
		},
		{
			name: "complete with rationale",
			content: `{
				"status": "complete",
				"summary": "Documentation updated",
				"content": "## Summary",
				"initiative_notes": [
					{"type": "learning", "content": "CI requires Redis running"}
				],
				"notes_rationale": "Task established new CI dependency pattern"
			}`,
			wantStatus:  "complete",
			wantSummary: "Documentation updated",
			wantNotes:   1,
			wantErr:     false,
		},
		{
			name: "blocked status",
			content: `{
				"status": "blocked",
				"reason": "Need clarification on doc structure"
			}`,
			wantStatus:  "blocked",
			wantNotes:   0,
			wantErr:     false,
		},
		{
			name:    "invalid JSON",
			content: `not valid json`,
			wantErr: true,
		},
		{
			name: "note missing type - should error",
			content: `{
				"status": "complete",
				"summary": "Docs updated",
				"content": "content",
				"initiative_notes": [
					{"content": "Missing type field"}
				]
			}`,
			wantErr:         true,
			wantErrContains: "type",
		},
		{
			name: "note missing content - should error",
			content: `{
				"status": "complete",
				"summary": "Docs updated",
				"content": "content",
				"initiative_notes": [
					{"type": "pattern"}
				]
			}`,
			wantErr:         true,
			wantErrContains: "content",
		},
		{
			name: "invalid note type - should error",
			content: `{
				"status": "complete",
				"summary": "Docs updated",
				"content": "content",
				"initiative_notes": [
					{"type": "invalid_type", "content": "Some content"}
				]
			}`,
			wantErr:         true,
			wantErrContains: "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseDocsResponse(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDocsResponse() expected error, got nil")
				} else if tt.wantErrContains != "" {
					if !containsIgnoreCase(err.Error(), tt.wantErrContains) {
						t.Errorf("ParseDocsResponse() error = %q, want to contain %q", err.Error(), tt.wantErrContains)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDocsResponse() unexpected error: %v", err)
				return
			}

			if resp.Status != tt.wantStatus {
				t.Errorf("ParseDocsResponse() status = %v, want %v", resp.Status, tt.wantStatus)
			}
			if resp.Summary != tt.wantSummary {
				t.Errorf("ParseDocsResponse() summary = %v, want %v", resp.Summary, tt.wantSummary)
			}
			if len(resp.InitiativeNotes) != tt.wantNotes {
				t.Errorf("ParseDocsResponse() notes count = %d, want %d", len(resp.InitiativeNotes), tt.wantNotes)
			}
		})
	}
}

// TestDocsResponseNoteTypes validates allowed note types.
// SC-5: Each note has required fields including valid type
func TestDocsResponseNoteTypes(t *testing.T) {
	t.Parallel()
	validTypes := []string{"pattern", "warning", "learning", "handoff"}

	for _, noteType := range validTypes {
		t.Run(noteType, func(t *testing.T) {
			content := `{
				"status": "complete",
				"summary": "done",
				"content": "content",
				"initiative_notes": [
					{"type": "` + noteType + `", "content": "test content"}
				]
			}`
			resp, err := ParseDocsResponse(content)
			if err != nil {
				t.Errorf("ParseDocsResponse() should accept type %q, got error: %v", noteType, err)
				return
			}
			if len(resp.InitiativeNotes) != 1 {
				t.Errorf("ParseDocsResponse() should have 1 note, got %d", len(resp.InitiativeNotes))
				return
			}
			if resp.InitiativeNotes[0].Type != noteType {
				t.Errorf("ParseDocsResponse() note type = %q, want %q", resp.InitiativeNotes[0].Type, noteType)
			}
		})
	}
}

// TestDocsResponseRelevantFiles validates that relevant_files is properly parsed.
// SC-6: Optional relevant_files array is preserved when provided
func TestDocsResponseRelevantFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		content   string
		wantFiles []string
	}{
		{
			name: "with relevant_files",
			content: `{
				"status": "complete",
				"summary": "done",
				"content": "content",
				"initiative_notes": [
					{"type": "pattern", "content": "test", "relevant_files": ["internal/repo/", "pkg/utils/helper.go"]}
				]
			}`,
			wantFiles: []string{"internal/repo/", "pkg/utils/helper.go"},
		},
		{
			name: "without relevant_files",
			content: `{
				"status": "complete",
				"summary": "done",
				"content": "content",
				"initiative_notes": [
					{"type": "pattern", "content": "test"}
				]
			}`,
			wantFiles: nil,
		},
		{
			name: "empty relevant_files",
			content: `{
				"status": "complete",
				"summary": "done",
				"content": "content",
				"initiative_notes": [
					{"type": "pattern", "content": "test", "relevant_files": []}
				]
			}`,
			wantFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseDocsResponse(tt.content)
			if err != nil {
				t.Fatalf("ParseDocsResponse() error: %v", err)
			}

			if len(resp.InitiativeNotes) != 1 {
				t.Fatalf("expected 1 note, got %d", len(resp.InitiativeNotes))
			}

			note := resp.InitiativeNotes[0]
			if tt.wantFiles == nil {
				if note.RelevantFiles != nil {
					t.Errorf("expected nil relevant_files, got %v", note.RelevantFiles)
				}
			} else {
				if len(note.RelevantFiles) != len(tt.wantFiles) {
					t.Errorf("relevant_files length = %d, want %d", len(note.RelevantFiles), len(tt.wantFiles))
				}
				for i, f := range tt.wantFiles {
					if i < len(note.RelevantFiles) && note.RelevantFiles[i] != f {
						t.Errorf("relevant_files[%d] = %q, want %q", i, note.RelevantFiles[i], f)
					}
				}
			}
		})
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			(len(s) > 0 && (s[0:len(substr)] == substr || containsIgnoreCase(s[1:], substr))))
}
