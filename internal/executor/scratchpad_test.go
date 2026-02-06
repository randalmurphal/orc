// Package executor tests for scratchpad entry extraction from phase JSON output.
//
// TDD Tests for TASK-020: Phase scratchpad extraction
//
// Success Criteria Coverage:
//   - SC-3: Executor extracts scratchpad entries from phase JSON output
//
// Failure Mode Coverage:
//   - Malformed scratchpad entry (missing category/content) → skipped with warning
//   - No scratchpad field in output → zero entries, no error
//   - Empty scratchpad array → zero entries, no error
//   - Unknown category accepted
//
// Edge Cases:
//   - BDD-1: Spec phase with decision + observation entries
//   - BDD-2: Implement phase failure with blocker entry
//   - BDD-3: Phase output with no scratchpad field
package executor

import (
	"testing"
)

// ============================================================================
// SC-3: Extract scratchpad entries from phase JSON output
// ============================================================================

// TestExtractScratchpadEntries_ValidEntries verifies extraction of well-formed
// scratchpad entries from JSON phase output (BDD-1 scenario).
func TestExtractScratchpadEntries_ValidEntries(t *testing.T) {
	t.Parallel()

	output := `{
		"status": "complete",
		"summary": "Spec completed",
		"content": "# Spec content",
		"scratchpad": [
			{"category": "decision", "content": "Chose token bucket for rate limiting"},
			{"category": "observation", "content": "Existing middleware uses chi router"}
		]
	}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Category != "decision" {
		t.Errorf("entry[0].Category = %q, want %q", entries[0].Category, "decision")
	}
	if entries[0].Content != "Chose token bucket for rate limiting" {
		t.Errorf("entry[0].Content = %q, want %q", entries[0].Content, "Chose token bucket for rate limiting")
	}
	if entries[1].Category != "observation" {
		t.Errorf("entry[1].Category = %q, want %q", entries[1].Category, "observation")
	}
	if entries[1].Content != "Existing middleware uses chi router" {
		t.Errorf("entry[1].Content = %q, want %q", entries[1].Content, "Existing middleware uses chi router")
	}
}

// TestExtractScratchpadEntries_BlockerEntry verifies extraction of blocker
// entries (BDD-2 scenario).
func TestExtractScratchpadEntries_BlockerEntry(t *testing.T) {
	t.Parallel()

	output := `{
		"status": "blocked",
		"reason": "Cannot proceed without Node 18+",
		"scratchpad": [
			{"category": "blocker", "content": "Test framework requires Node 18+"}
		]
	}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries returned error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Category != "blocker" {
		t.Errorf("category = %q, want %q", entries[0].Category, "blocker")
	}
}

// TestExtractScratchpadEntries_NoScratchpadField verifies that output without
// a scratchpad field produces zero entries and no error (BDD-3 scenario).
func TestExtractScratchpadEntries_NoScratchpadField(t *testing.T) {
	t.Parallel()

	output := `{"status": "complete", "summary": "Done", "content": "Implementation complete"}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries should not error for missing scratchpad field, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for output without scratchpad, got %d", len(entries))
	}
}

// TestExtractScratchpadEntries_EmptyScratchpadArray verifies that an empty
// scratchpad array produces zero entries and no error.
func TestExtractScratchpadEntries_EmptyScratchpadArray(t *testing.T) {
	t.Parallel()

	output := `{"status": "complete", "summary": "Done", "scratchpad": []}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries should not error for empty scratchpad array, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty scratchpad array, got %d", len(entries))
	}
}

// TestExtractScratchpadEntries_MalformedEntrySkipped verifies that entries
// missing category or content are skipped (not causing an error).
func TestExtractScratchpadEntries_MalformedEntrySkipped(t *testing.T) {
	t.Parallel()

	output := `{
		"status": "complete",
		"scratchpad": [
			{"category": "decision", "content": "Valid entry"},
			{"content": "Missing category"},
			{"category": "observation"},
			{"category": "", "content": "Empty category"},
			{"category": "warning", "content": ""}
		]
	}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries should not error for malformed entries, got: %v", err)
	}

	// Only the first entry is valid (has both non-empty category AND content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry (malformed should be skipped), got %d", len(entries))
	}
	if entries[0].Content != "Valid entry" {
		t.Errorf("content = %q, want %q", entries[0].Content, "Valid entry")
	}
}

// TestExtractScratchpadEntries_UnknownCategory verifies that unknown categories
// are accepted and stored without error.
func TestExtractScratchpadEntries_UnknownCategory(t *testing.T) {
	t.Parallel()

	output := `{
		"status": "complete",
		"scratchpad": [
			{"category": "custom_insight", "content": "A custom observation type"}
		]
	}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries should accept unknown categories, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Category != "custom_insight" {
		t.Errorf("category = %q, want %q", entries[0].Category, "custom_insight")
	}
}

// TestExtractScratchpadEntries_InvalidJSON verifies that completely invalid
// JSON returns zero entries and no error (scratchpad extraction is defensive).
func TestExtractScratchpadEntries_InvalidJSON(t *testing.T) {
	t.Parallel()

	output := `not valid json at all`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries should not error for invalid JSON, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for invalid JSON, got %d", len(entries))
	}
}

// TestExtractScratchpadEntries_AllCategories verifies standard categories are all accepted.
func TestExtractScratchpadEntries_AllCategories(t *testing.T) {
	t.Parallel()

	output := `{
		"status": "complete",
		"scratchpad": [
			{"category": "observation", "content": "Observed something"},
			{"category": "decision", "content": "Decided something"},
			{"category": "blocker", "content": "Blocked by something"},
			{"category": "todo", "content": "Need to do something"},
			{"category": "warning", "content": "Watch out for something"}
		]
	}`

	entries, err := ExtractScratchpadEntries(output)
	if err != nil {
		t.Fatalf("ExtractScratchpadEntries error: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries (one per standard category), got %d", len(entries))
	}

	expectedCategories := []string{"observation", "decision", "blocker", "todo", "warning"}
	for i, cat := range expectedCategories {
		if entries[i].Category != cat {
			t.Errorf("entries[%d].Category = %q, want %q", i, entries[i].Category, cat)
		}
	}
}
