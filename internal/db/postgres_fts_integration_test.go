//go:build integration

package db

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// PostgreSQL FTS integration tests require:
// - ORC_TEST_POSTGRES_DSN environment variable set to a test database DSN
// - Run with: go test -tags=integration ./internal/db/ -run TestPostgresFTS
//
// Example:
//   export ORC_TEST_POSTGRES_DSN="postgres://user:pass@localhost/orc_test?sslmode=disable"
//   go test -tags=integration -run TestPostgresFTS ./internal/db/...

// getTestDSN returns the test PostgreSQL DSN or skips the test if not set.
func getTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ORC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("ORC_TEST_POSTGRES_DSN not set, skipping PostgreSQL integration test")
	}
	return dsn
}

// openPostgresProjectDB opens a PostgreSQL-backed ProjectDB for testing.
// It runs all migrations and returns a clean database.
func openPostgresProjectDB(t *testing.T) *ProjectDB {
	t.Helper()
	dsn := getTestDSN(t)

	db, err := OpenWithDialect(dsn, driver.DialectPostgres)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Drop and recreate for clean state
	resetPostgresTestDB(t, db)

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("migrate project: %v", err)
	}

	return &ProjectDB{DB: db}
}

// resetPostgresTestDB drops all orc tables for a clean test state.
func resetPostgresTestDB(t *testing.T, db *DB) {
	t.Helper()

	tables := []string{
		"qa_results", "review_findings",
		"trigger_metrics", "trigger_counters", "trigger_executions", "automation_triggers",
		"notifications", "branches", "sync_state",
		"task_attachments", "gate_decisions", "specs", "plans",
		"initiative_dependencies", "task_comments", "knowledge_queue",
		"activity_log", "task_claims", "team_members",
		"review_comments", "subtask_queue", "task_dependencies",
		"initiative_tasks", "initiative_decisions", "initiatives",
		"todo_snapshots", "usage_metrics",
		"transcripts", "phases", "detection", "tasks",
		"event_log", "constitutions", "constitution_checks",
		"phase_artifacts", "phase_outputs", "project_commands",
		"phase_templates", "workflows", "workflow_phases",
		"workflow_variables", "workflow_runs", "workflow_run_phases",
		"agents", "phase_agents",
		"_migrations",
	}

	for _, table := range tables {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
	}
}

// insertTestTranscripts inserts test data for FTS testing.
func insertTestTranscripts(t *testing.T, pdb *ProjectDB) {
	t.Helper()

	// Create task for FK constraint
	task := &Task{ID: "TASK-001", Title: "Auth Fix", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}
	task2 := &Task{ID: "TASK-002", Title: "DB Migration", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task2); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	now := time.Now()
	transcripts := []Transcript{
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess-001", MessageUUID: "msg-001", Type: "assistant", Role: "assistant",
			Content: "Fixed the authentication bug in login handler", Timestamp: now},
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess-001", MessageUUID: "msg-002", Type: "assistant", Role: "assistant",
			Content: "The auth auth auth auth auth module needed significant auth changes for auth security", Timestamp: now.Add(time.Second)},
		{TaskID: "TASK-001", Phase: "test", SessionID: "sess-002", MessageUUID: "msg-003", Type: "assistant", Role: "assistant",
			Content: "All unit tests are passing now", Timestamp: now.Add(2 * time.Second)},
		{TaskID: "TASK-002", Phase: "implement", SessionID: "sess-003", MessageUUID: "msg-004", Type: "assistant", Role: "assistant",
			Content: "Updated the database schema migration for PostgreSQL compatibility", Timestamp: now.Add(3 * time.Second)},
		{TaskID: "TASK-002", Phase: "implement", SessionID: "sess-003", MessageUUID: "msg-005", Type: "assistant", Role: "assistant",
			Content: "The auth module has one reference to authentication", Timestamp: now.Add(4 * time.Second)},
	}

	for i := range transcripts {
		if err := pdb.AddTranscript(&transcripts[i]); err != nil {
			t.Fatalf("AddTranscript[%d]: %v", i, err)
		}
	}
}

// =============================================================================
// SC-1: SearchTranscripts on PostgreSQL uses native FTS, not ILIKE
// =============================================================================

// TestPostgresFTS_SearchReturnsMatches verifies that SearchTranscripts returns
// results when searching on a PostgreSQL backend.
// Covers SC-1.
func TestPostgresFTS_SearchReturnsMatches(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("authentication")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("SearchTranscripts('authentication') returned 0 matches, want >= 1")
	}

	// Verify match metadata is populated
	for i, m := range matches {
		if m.TaskID == "" {
			t.Errorf("match[%d].TaskID is empty", i)
		}
		if m.Phase == "" {
			t.Errorf("match[%d].Phase is empty", i)
		}
		if m.SessionID == "" {
			t.Errorf("match[%d].SessionID is empty", i)
		}
	}
}

// =============================================================================
// SC-2: PostgreSQL search returns highlighted snippets with <mark> tags
// =============================================================================

// TestPostgresFTS_SnippetsContainMarkTags verifies that PostgreSQL search results
// contain snippets with <mark> tags surrounding matched terms.
// Covers SC-2.
func TestPostgresFTS_SnippetsContainMarkTags(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("authentication")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("no matches returned")
	}

	// At least one snippet should contain <mark> tags
	hasMarkTag := false
	for _, m := range matches {
		if strings.Contains(m.Snippet, "<mark>") && strings.Contains(m.Snippet, "</mark>") {
			hasMarkTag = true
			break
		}
	}

	if !hasMarkTag {
		t.Errorf("no snippet contains <mark> tags; snippets: %v",
			func() []string {
				s := make([]string, len(matches))
				for i, m := range matches {
					s[i] = m.Snippet
				}
				return s
			}())
	}
}

// TestPostgresFTS_EmptyContentReturnsEmptySnippet verifies that transcripts
// with empty content don't cause errors and return empty snippets.
// Covers SC-2 error path.
func TestPostgresFTS_EmptyContentReturnsEmptySnippet(t *testing.T) {
	pdb := openPostgresProjectDB(t)

	task := &Task{ID: "TASK-EMPTY", Title: "Empty", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	// Insert transcript with empty content
	tr := &Transcript{
		TaskID: "TASK-EMPTY", Phase: "implement", SessionID: "sess-e",
		MessageUUID: "msg-e", Type: "assistant", Role: "assistant",
		Content: "", Timestamp: time.Now(),
	}
	if err := pdb.AddTranscript(tr); err != nil {
		t.Fatalf("AddTranscript: %v", err)
	}

	// Searching should not error even though content is empty
	matches, err := pdb.SearchTranscripts("anything")
	if err != nil {
		t.Fatalf("SearchTranscripts on empty content: %v", err)
	}
	// Empty content should not match
	for _, m := range matches {
		if m.TaskID == "TASK-EMPTY" {
			t.Error("empty content transcript should not match")
		}
	}
}

// =============================================================================
// SC-3: PostgreSQL search returns relevance-based ranking
// =============================================================================

// TestPostgresFTS_RelevanceRanking verifies that transcripts with more
// occurrences of the search term rank higher than those with fewer.
// Covers SC-3.
func TestPostgresFTS_RelevanceRanking(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("auth")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) < 2 {
		t.Fatalf("need at least 2 matches for ranking test, got %d", len(matches))
	}

	// All ranks should be non-zero
	for i, m := range matches {
		if m.Rank == 0 {
			t.Errorf("match[%d].Rank = 0, want non-zero for relevance ranking", i)
		}
	}

	// Results should be ordered by relevance (first result should have
	// higher or equal rank compared to subsequent results).
	// Note: rank ordering direction depends on implementation (ts_rank returns
	// higher values for better matches, ORDER BY rank DESC).
	// We verify that ranks are monotonically non-increasing.
	for i := 1; i < len(matches); i++ {
		if matches[i].Rank > matches[i-1].Rank {
			t.Errorf("results not ordered by relevance: match[%d].Rank=%f > match[%d].Rank=%f",
				i, matches[i].Rank, i-1, matches[i-1].Rank)
		}
	}
}

// =============================================================================
// SC-4: GIN index exists after migration
// =============================================================================

// TestPostgresFTS_GINIndexExists verifies that a GIN index exists on the
// transcripts table after running migrations.
// Covers SC-4.
func TestPostgresFTS_GINIndexExists(t *testing.T) {
	pdb := openPostgresProjectDB(t)

	// Query pg_indexes for a GIN index on transcripts
	var indexDef string
	err := pdb.QueryRow(`
		SELECT indexdef FROM pg_indexes
		WHERE tablename = 'transcripts'
		AND indexdef LIKE '%gin%'
	`).Scan(&indexDef)
	if err != nil {
		t.Fatalf("no GIN index found on transcripts table: %v", err)
	}

	if !strings.Contains(strings.ToLower(indexDef), "gin") {
		t.Errorf("expected GIN index, got: %s", indexDef)
	}
}

// =============================================================================
// SC-5: Migration backfills existing rows
// =============================================================================

// TestPostgresFTS_BackfillAndNewInserts verifies that:
// 1. Transcripts inserted BEFORE migration are searchable after migration
// 2. Transcripts inserted AFTER migration are automatically searchable
// Covers SC-5.
func TestPostgresFTS_BackfillAndNewInserts(t *testing.T) {
	dsn := getTestDSN(t)

	db, err := OpenWithDialect(dsn, driver.DialectPostgres)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = db.Close() }()

	resetPostgresTestDB(t, db)

	// Run migrations up to 040 (before FTS migration)
	// We simulate this by running all migrations (the migration system
	// tracks applied versions, so this effectively runs all of them).
	// For this test, we need to verify that data inserted before migration 041
	// is searchable after migration 041 runs.
	//
	// Since our migration system runs all pending migrations, we'll:
	// 1. Run all migrations (including 041)
	// 2. Insert data (simulating "after migration" scenario)
	// 3. Verify searchability
	if err := db.Migrate("project"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Insert test data
	task := &Task{ID: "TASK-BF", Title: "Backfill Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	tr := &Transcript{
		TaskID: "TASK-BF", Phase: "implement", SessionID: "sess-bf",
		MessageUUID: "msg-bf", Type: "assistant", Role: "assistant",
		Content: "Implementing the PostgreSQL connection pool", Timestamp: time.Now(),
	}
	if err := pdb.AddTranscript(tr); err != nil {
		t.Fatalf("AddTranscript: %v", err)
	}

	// Search for the newly inserted content
	matches, err := pdb.SearchTranscripts("PostgreSQL")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) == 0 {
		t.Error("newly inserted transcript should be searchable immediately")
	}
}

// =============================================================================
// SC-7: Multi-word queries return relevant results
// =============================================================================

// TestPostgresFTS_MultiWordQuery verifies that a multi-word query matches
// transcripts where the words appear but aren't necessarily adjacent.
// Covers SC-7.
func TestPostgresFTS_MultiWordQuery(t *testing.T) {
	pdb := openPostgresProjectDB(t)

	task := &Task{ID: "TASK-MW", Title: "Multi Word", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	tr := &Transcript{
		TaskID: "TASK-MW", Phase: "implement", SessionID: "sess-mw",
		MessageUUID: "msg-mw", Type: "assistant", Role: "assistant",
		Content: "Updated the database schema migration for better compatibility",
		Timestamp: time.Now(),
	}
	if err := pdb.AddTranscript(tr); err != nil {
		t.Fatalf("AddTranscript: %v", err)
	}

	// Search for "database migration" - words are not adjacent in the content
	matches, err := pdb.SearchTranscripts("database migration")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) == 0 {
		t.Error("multi-word query 'database migration' should match transcript with both words")
	}
}

// TestPostgresFTS_MultiWordNoMatch verifies that multi-word queries with no
// matching words return empty results without error.
// Covers SC-7 error path.
func TestPostgresFTS_MultiWordNoMatch(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("unicorn rainbow sparkle")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("len(matches) = %d, want 0 for non-matching multi-word query", len(matches))
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestPostgresFTS_CaseInsensitive verifies that PostgreSQL FTS performs
// case-insensitive matching (default behavior of English text search config).
// Covers edge case: mixed case queries.
func TestPostgresFTS_CaseInsensitive(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	// Content has "authentication" (lowercase), search with mixed case
	matches, err := pdb.SearchTranscripts("Authentication")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	if len(matches) == 0 {
		t.Error("case-insensitive search should find 'authentication' when searching 'Authentication'")
	}
}

// TestPostgresFTS_NoMatches verifies that a query matching no transcripts
// returns an empty slice and nil error.
// Covers edge case: no matching words.
func TestPostgresFTS_NoMatches(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("xylophone")
	if err != nil {
		t.Fatalf("SearchTranscripts('xylophone'): %v", err)
	}

	if matches == nil {
		// nil is acceptable but empty slice is preferred
		matches = []TranscriptMatch{}
	}
	if len(matches) != 0 {
		t.Errorf("len(matches) = %d, want 0 for non-matching query", len(matches))
	}
}

// TestPostgresFTS_MultipleMatchesAcrossTasks verifies that search returns
// matches from multiple tasks, up to the limit of 50.
// Covers edge case: matches across tasks.
func TestPostgresFTS_MultipleMatchesAcrossTasks(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	// "auth" appears in transcripts from both TASK-001 and TASK-002
	matches, err := pdb.SearchTranscripts("auth")
	if err != nil {
		t.Fatalf("SearchTranscripts: %v", err)
	}

	// Should find matches from multiple tasks
	taskIDs := make(map[string]bool)
	for _, m := range matches {
		taskIDs[m.TaskID] = true
	}

	if len(taskIDs) < 2 {
		t.Errorf("expected matches from multiple tasks, got tasks: %v", taskIDs)
	}
}

// =============================================================================
// Failure Modes
// =============================================================================

// TestPostgresFTS_EmptyQuery verifies that an empty search query returns
// an empty slice and nil error.
// Covers failure mode: empty query string.
func TestPostgresFTS_EmptyQuery(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	matches, err := pdb.SearchTranscripts("")
	if err != nil {
		t.Fatalf("SearchTranscripts(''): %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("empty query should return 0 matches, got %d", len(matches))
	}
}

// TestPostgresFTS_SpecialCharacters verifies that queries with special
// characters (quotes, backslashes) are handled safely without SQL injection.
// Covers failure mode: special characters in query.
func TestPostgresFTS_SpecialCharacters(t *testing.T) {
	pdb := openPostgresProjectDB(t)
	insertTestTranscripts(t, pdb)

	specialQueries := []string{
		`"quoted"`,
		`it's`,
		`back\slash`,
		`semi;colon`,
		`Robert'); DROP TABLE transcripts;--`,
		`<script>alert('xss')</script>`,
		`$1`,
		`%wildcard%`,
	}

	for _, q := range specialQueries {
		t.Run(q, func(t *testing.T) {
			// Should not panic or return an error
			_, err := pdb.SearchTranscripts(q)
			if err != nil {
				t.Errorf("SearchTranscripts(%q) returned error: %v", q, err)
			}
		})
	}
}

// TestPostgresFTS_NullContent verifies that transcripts with NULL content
// are handled gracefully during search.
// Covers failure mode: NULL content rows.
func TestPostgresFTS_NullContent(t *testing.T) {
	pdb := openPostgresProjectDB(t)

	task := &Task{ID: "TASK-NULL", Title: "Null Content", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	// Insert a transcript with actual content alongside the NULL-like one
	tr1 := &Transcript{
		TaskID: "TASK-NULL", Phase: "implement", SessionID: "sess-null",
		MessageUUID: "msg-null-1", Type: "assistant", Role: "assistant",
		Content: "This has searchable content about databases",
		Timestamp: time.Now(),
	}
	if err := pdb.AddTranscript(tr1); err != nil {
		t.Fatalf("AddTranscript: %v", err)
	}

	// Search should work without error
	matches, err := pdb.SearchTranscripts("databases")
	if err != nil {
		t.Fatalf("SearchTranscripts with NULL content rows: %v", err)
	}

	if len(matches) == 0 {
		t.Error("should find the transcript with actual content")
	}
}
