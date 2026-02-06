package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// SC-1: SearchTranscripts PostgreSQL branch uses native FTS, not ILIKE
// =============================================================================

// TestSearchTranscripts_PostgresBranch_NoILIKE verifies that the PostgreSQL
// branch of SearchTranscripts does NOT use ILIKE pattern matching.
// The current implementation uses ILIKE as a fallback; the task requires replacing
// it with native tsvector/tsquery full-text search.
// Covers SC-1.
func TestSearchTranscripts_PostgresBranch_NoILIKE(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	// ILIKE should not appear anywhere in SearchTranscripts.
	// It currently only exists in the PostgreSQL branch, so if it's gone from
	// the file entirely, the PostgreSQL path has been updated.
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "ILIKE") {
			t.Errorf("transcript.go:%d still contains ILIKE: %s",
				i+1, trimmed)
		}
	}
}

// TestSearchTranscripts_PostgresBranch_UsesTsQuery verifies that the PostgreSQL
// branch of SearchTranscripts uses the @@ operator for tsvector/tsquery matching.
// Covers SC-1.
func TestSearchTranscripts_PostgresBranch_UsesTsQuery(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	contentStr := string(content)

	// The PostgreSQL branch must use the @@ operator (tsvector match)
	if !strings.Contains(contentStr, "@@") {
		t.Error("transcript.go does not contain @@ operator - PostgreSQL FTS not implemented")
	}

	// Must use plainto_tsquery or to_tsquery for query conversion
	if !strings.Contains(contentStr, "plainto_tsquery") && !strings.Contains(contentStr, "to_tsquery") {
		t.Error("transcript.go does not contain plainto_tsquery or to_tsquery - PostgreSQL FTS not implemented")
	}
}

// =============================================================================
// SC-2: PostgreSQL search returns highlighted snippets with <mark> tags
// =============================================================================

// TestSearchTranscripts_PostgresBranch_UsesHeadline verifies that the PostgreSQL
// branch of SearchTranscripts uses ts_headline for snippet generation with <mark> tags,
// matching the SQLite snippet format.
// Covers SC-2.
func TestSearchTranscripts_PostgresBranch_UsesHeadline(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	contentStr := string(content)

	// Must use ts_headline for PostgreSQL snippet generation
	if !strings.Contains(contentStr, "ts_headline") {
		t.Error("transcript.go does not contain ts_headline - PostgreSQL snippet generation not implemented")
	}

	// Snippet markers must use <mark> tags to match SQLite's snippet() format
	if !strings.Contains(contentStr, "<mark>") {
		t.Error("transcript.go does not contain <mark> tag in PostgreSQL snippet config")
	}
	if !strings.Contains(contentStr, "</mark>") {
		t.Error("transcript.go does not contain </mark> tag in PostgreSQL snippet config")
	}
}

// =============================================================================
// SC-3: PostgreSQL search returns relevance-based ranking (not hardcoded 0.0)
// =============================================================================

// TestSearchTranscripts_PostgresBranch_UsesRanking verifies that the PostgreSQL
// branch of SearchTranscripts uses ts_rank for relevance scoring instead of
// the current hardcoded 0.0.
// Covers SC-3.
func TestSearchTranscripts_PostgresBranch_UsesRanking(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	contentStr := string(content)

	// Must use ts_rank for relevance scoring
	if !strings.Contains(contentStr, "ts_rank") {
		t.Error("transcript.go does not contain ts_rank - PostgreSQL ranking not implemented")
	}

	// The hardcoded "0.0 as rank" should be removed from the PostgreSQL branch.
	// Check non-comment lines only.
	lines := strings.Split(contentStr, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "0.0 as rank") {
			t.Errorf("transcript.go:%d still contains hardcoded '0.0 as rank': %s",
				i+1, trimmed)
		}
	}
}

// =============================================================================
// SC-4: PostgreSQL migration creates GIN index
// =============================================================================

// TestPostgresFTSMigration_Exists verifies that the PostgreSQL FTS migration file
// project_041.sql exists in the schema/postgres directory.
// Covers SC-4.
func TestPostgresFTSMigration_Exists(t *testing.T) {
	t.Parallel()

	schemaDir := filepath.Join("schema", "postgres")
	path := filepath.Join(schemaDir, "project_041.sql")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("project_041.sql does not exist at %s - migration not created", path)
	}
}

// TestPostgresFTSMigration_HasGINIndex verifies that project_041.sql creates
// a GIN index on the transcripts table for full-text search.
// Covers SC-4.
func TestPostgresFTSMigration_HasGINIndex(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := string(content)

	// Must contain GIN index creation
	if !strings.Contains(strings.ToUpper(contentStr), "USING GIN") {
		t.Error("project_041.sql does not contain 'USING GIN' - GIN index not created")
	}

	// GIN index must be on the transcripts table
	if !strings.Contains(strings.ToLower(contentStr), "transcripts") {
		t.Error("project_041.sql does not reference transcripts table")
	}
}

// TestPostgresFTSMigration_IsIdempotent verifies that the migration uses
// IF NOT EXISTS for index and column additions.
// Covers SC-4 (idempotent migration requirement).
func TestPostgresFTSMigration_IsIdempotent(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := strings.ToUpper(string(content))

	// Index creation should use IF NOT EXISTS
	if strings.Contains(contentStr, "CREATE INDEX") && !strings.Contains(contentStr, "IF NOT EXISTS") {
		t.Error("project_041.sql CREATE INDEX should use IF NOT EXISTS for idempotency")
	}
}

// TestPostgresFTSMigration_NoSQLiteisms verifies the FTS migration doesn't
// contain SQLite-specific syntax.
// Covers SC-4 (dialect correctness).
func TestPostgresFTSMigration_NoSQLiteisms(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := string(content)

	sqliteisms := []struct {
		pattern string
		desc    string
	}{
		{"datetime('now')", "use NOW() instead"},
		{"AUTOINCREMENT", "use SERIAL instead"},
		{"strftime(", "use PostgreSQL date functions"},
		{"USING fts5", "use tsvector instead"},
		{"CREATE VIRTUAL TABLE", "not supported in PostgreSQL"},
	}

	for _, s := range sqliteisms {
		if strings.Contains(contentStr, s.pattern) {
			t.Errorf("project_041.sql contains SQLite-ism %q: %s", s.pattern, s.desc)
		}
	}
}

// =============================================================================
// SC-5: Migration backfills search index for existing transcript rows
// =============================================================================

// TestPostgresFTSMigration_BackfillsExistingRows verifies that project_041.sql
// includes an UPDATE statement to populate the tsvector column for pre-existing
// transcript rows.
// Covers SC-5.
func TestPostgresFTSMigration_BackfillsExistingRows(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := strings.ToLower(string(content))

	// Must contain an UPDATE to backfill existing rows
	if !strings.Contains(contentStr, "update") || !strings.Contains(contentStr, "to_tsvector") {
		t.Error("project_041.sql does not contain UPDATE with to_tsvector - existing rows not backfilled")
	}
}

// TestPostgresFTSMigration_AutoIndexesNewRows verifies that project_041.sql
// includes a trigger or generated column to automatically index new transcript
// rows inserted after the migration.
// Covers SC-5.
func TestPostgresFTSMigration_AutoIndexesNewRows(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := strings.ToLower(string(content))

	// Must have either a trigger or a generated column for auto-indexing.
	// A trigger uses CREATE TRIGGER + CREATE FUNCTION.
	// A generated column uses GENERATED ALWAYS AS.
	hasTrigger := strings.Contains(contentStr, "create trigger") ||
		strings.Contains(contentStr, "create or replace function")
	hasGenerated := strings.Contains(contentStr, "generated always as")

	if !hasTrigger && !hasGenerated {
		t.Error("project_041.sql has no trigger or generated column for auto-indexing new rows")
	}
}

// TestPostgresFTSMigration_HandleNullContent verifies that the migration
// handles NULL or empty content gracefully (using COALESCE or similar).
// Covers SC-5 failure mode: empty content rows don't cause migration failure.
func TestPostgresFTSMigration_HandleNullContent(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := strings.ToLower(string(content))

	// Must use COALESCE to handle NULL content gracefully
	if !strings.Contains(contentStr, "coalesce") {
		t.Error("project_041.sql does not use COALESCE - NULL content may cause migration failure")
	}
}

// TestPostgresFTSMigration_HasTsvectorColumn verifies that project_041.sql
// adds a tsvector column to the transcripts table.
// Covers SC-4 (tsvector infrastructure).
func TestPostgresFTSMigration_HasTsvectorColumn(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("schema", "postgres", "project_041.sql"))
	if err != nil {
		t.Fatalf("read project_041.sql: %v", err)
	}

	contentStr := strings.ToLower(string(content))

	if !strings.Contains(contentStr, "tsvector") {
		t.Error("project_041.sql does not contain tsvector type - FTS column not added")
	}
}

// =============================================================================
// SC-6: SQLite FTS behavior remains unchanged
// =============================================================================

// TestProjectDB_TranscriptSearch_SQLiteRegressionGuard is a regression test
// verifying that SQLite FTS5 search behavior is unchanged after the PostgreSQL
// FTS implementation. This test is expected to PASS with the current code since
// it tests existing SQLite behavior.
// Covers SC-6.
func TestProjectDB_TranscriptSearch_SQLiteRegressionGuard(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create task for FK constraint
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	now := time.Now()
	transcripts := []Transcript{
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess-001", MessageUUID: "msg-001", Type: "assistant", Role: "assistant", Content: "Fixed the authentication bug in login handler", Timestamp: now},
		{TaskID: "TASK-001", Phase: "test", SessionID: "sess-002", MessageUUID: "msg-002", Type: "assistant", Role: "assistant", Content: "All unit tests are passing now", Timestamp: now.Add(time.Second)},
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess-001", MessageUUID: "msg-003", Type: "assistant", Role: "assistant", Content: "Updated the database schema for migrations", Timestamp: now.Add(2 * time.Second)},
	}

	for i := range transcripts {
		if err := pdb.AddTranscript(&transcripts[i]); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	// Test 1: Basic search returns matches
	matches, err := pdb.SearchTranscripts("authentication")
	if err != nil {
		t.Fatalf("SearchTranscripts('authentication') failed: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("len(matches) for 'authentication' = %d, want 1", len(matches))
	}

	// Test 2: Verify match metadata
	if len(matches) > 0 {
		m := matches[0]
		if m.TaskID != "TASK-001" {
			t.Errorf("match.TaskID = %q, want TASK-001", m.TaskID)
		}
		if m.Phase != "implement" {
			t.Errorf("match.Phase = %q, want implement", m.Phase)
		}
		if m.SessionID != "sess-001" {
			t.Errorf("match.SessionID = %q, want sess-001", m.SessionID)
		}
		// SQLite FTS5 snippet should contain <mark> tags
		if !strings.Contains(m.Snippet, "<mark>") {
			t.Errorf("match.Snippet should contain <mark> tags, got %q", m.Snippet)
		}
		// Rank should be non-zero for SQLite FTS5
		if m.Rank == 0 {
			t.Errorf("match.Rank = 0, want non-zero for FTS5 ranking")
		}
	}

	// Test 3: Another search to verify different terms
	matches2, err := pdb.SearchTranscripts("tests")
	if err != nil {
		t.Fatalf("SearchTranscripts('tests') failed: %v", err)
	}
	if len(matches2) != 1 {
		t.Errorf("len(matches) for 'tests' = %d, want 1", len(matches2))
	}

	// Test 4: No matches returns empty slice, not nil error
	matches3, err := pdb.SearchTranscripts("nonexistent_term_xyz")
	if err != nil {
		t.Fatalf("SearchTranscripts('nonexistent') failed: %v", err)
	}
	if len(matches3) != 0 {
		t.Errorf("len(matches) for nonexistent = %d, want 0", len(matches3))
	}

	// Test 5: Search for term that appears in multiple transcripts
	matches4, err := pdb.SearchTranscripts("database")
	if err != nil {
		t.Fatalf("SearchTranscripts('database') failed: %v", err)
	}
	if len(matches4) != 1 {
		t.Errorf("len(matches) for 'database' = %d, want 1", len(matches4))
	}
}

// =============================================================================
// SC-7: Multi-word queries (source verification)
// =============================================================================

// TestSearchTranscripts_PostgresBranch_MultiWordQuery verifies that the PostgreSQL
// branch handles multi-word queries by using plainto_tsquery (which automatically
// handles multiple words by ANDing them).
// Covers SC-7.
func TestSearchTranscripts_PostgresBranch_MultiWordQuery(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	contentStr := string(content)

	// plainto_tsquery automatically handles multi-word queries by
	// converting "word1 word2" to "word1 & word2" (AND).
	// websearch_to_tsquery would also work.
	// The key is that raw user input is NOT passed directly to to_tsquery
	// which would fail on unescaped special characters.
	hasPlainTo := strings.Contains(contentStr, "plainto_tsquery")
	hasWebSearch := strings.Contains(contentStr, "websearch_to_tsquery")

	if !hasPlainTo && !hasWebSearch {
		t.Error("transcript.go should use plainto_tsquery or websearch_to_tsquery for safe multi-word query handling")
	}
}

// =============================================================================
// Failure Mode: SUBSTRING removed from PostgreSQL branch
// =============================================================================

// TestSearchTranscripts_PostgresBranch_NoSubstring verifies that the PostgreSQL
// branch no longer uses the crude SUBSTRING-based snippet extraction.
// Covers SC-2 (no crude snippets).
func TestSearchTranscripts_PostgresBranch_NoSubstring(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("transcript.go")
	if err != nil {
		t.Fatalf("read transcript.go: %v", err)
	}

	// SUBSTRING is only used in the current PostgreSQL fallback.
	// After implementation, it should be replaced by ts_headline.
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "SUBSTRING(content") {
			t.Errorf("transcript.go:%d still contains SUBSTRING snippet extraction: %s",
				i+1, trimmed)
		}
	}
}
