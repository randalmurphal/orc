package db

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// setupTranscriptTest creates an in-memory database with sample transcripts.
func setupTranscriptTest(t *testing.T) *ProjectDB {
	t.Helper()

	db, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory failed: %v", err)
	}

	// Create task first (required for foreign key constraint)
	task := &Task{
		ID:          "TASK-001",
		Title:       "Test Task",
		Description: "Test Description",
		Weight:      "medium",
		Status:      "running",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Create sample transcripts
	baseTime := time.Now().Add(-1 * time.Hour)
	transcripts := []Transcript{
		{TaskID: "TASK-001", Phase: "spec", SessionID: "sess1", MessageUUID: "msg1", Type: "assistant", Role: "assistant", Content: "Spec message 1", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime},
		{TaskID: "TASK-001", Phase: "spec", SessionID: "sess1", MessageUUID: "msg2", Type: "assistant", Role: "assistant", Content: "Spec message 2", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(1 * time.Minute)},
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess1", MessageUUID: "msg3", Type: "assistant", Role: "assistant", Content: "Implement message 1", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(2 * time.Minute)},
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess1", MessageUUID: "msg4", Type: "assistant", Role: "assistant", Content: "Implement message 2", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(3 * time.Minute)},
		{TaskID: "TASK-001", Phase: "implement", SessionID: "sess1", MessageUUID: "msg5", Type: "assistant", Role: "assistant", Content: "Implement message 3", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(4 * time.Minute)},
		{TaskID: "TASK-001", Phase: "review", SessionID: "sess1", MessageUUID: "msg6", Type: "assistant", Role: "assistant", Content: "Review message 1", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(5 * time.Minute)},
	}

	for i := range transcripts {
		if err := db.AddTranscript(&transcripts[i]); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	return db
}

func TestGetTranscriptsPaginated_DefaultLimit(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	// Create TASK-002 for this test
	task := &Task{
		ID:          "TASK-002",
		Title:       "Test Task 2",
		Description: "Test Description",
		Weight:      "medium",
		Status:      "running",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Add more transcripts to test default limit
	baseTime := time.Now()
	for i := 0; i < 100; i++ {
		transcript := Transcript{
			TaskID:      "TASK-002",
			Phase:       "implement",
			SessionID:   "sess2",
			MessageUUID: fmt.Sprintf("msg-%d", i),
			Type:        "assistant",
			Role:        "assistant",
			Content:     "Message",
			Model:       "claude-3-5-sonnet-20241022",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
		}
		if err := db.AddTranscript(&transcript); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	opts := TranscriptPaginationOpts{
		Limit: 0, // Should default to 50
	}

	transcripts, pagination, err := db.GetTranscriptsPaginated("TASK-002", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	if len(transcripts) != 50 {
		t.Errorf("Expected 50 transcripts, got %d", len(transcripts))
	}

	if !pagination.HasMore {
		t.Error("Expected HasMore to be true")
	}

	if pagination.TotalCount != 100 {
		t.Errorf("Expected TotalCount to be 100, got %d", pagination.TotalCount)
	}

	if pagination.NextCursor == nil {
		t.Error("Expected NextCursor to be set")
	}
}

func TestGetTranscriptsPaginated_CustomLimit(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	opts := TranscriptPaginationOpts{
		Limit: 3,
	}

	transcripts, pagination, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	if len(transcripts) != 3 {
		t.Errorf("Expected 3 transcripts, got %d", len(transcripts))
	}

	if !pagination.HasMore {
		t.Error("Expected HasMore to be true")
	}

	if pagination.TotalCount != 6 {
		t.Errorf("Expected TotalCount to be 6, got %d", pagination.TotalCount)
	}
}

func TestGetTranscriptsPaginated_CursorNavigation(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	// Get first page
	opts := TranscriptPaginationOpts{
		Limit: 2,
	}

	page1, pagination1, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated page 1 failed: %v", err)
	}

	if len(page1) != 2 {
		t.Fatalf("Expected 2 transcripts on page 1, got %d", len(page1))
	}

	if !pagination1.HasMore {
		t.Error("Expected HasMore to be true on page 1")
	}

	if pagination1.NextCursor == nil {
		t.Fatal("Expected NextCursor to be set on page 1")
	}

	// Get second page using cursor
	opts.Cursor = *pagination1.NextCursor
	page2, pagination2, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated page 2 failed: %v", err)
	}

	if len(page2) != 2 {
		t.Fatalf("Expected 2 transcripts on page 2, got %d", len(page2))
	}

	// Verify no overlap
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Error("Pages have overlapping transcripts")
	}

	// Verify prev cursor is set on page 2
	if pagination2.PrevCursor == nil {
		t.Error("Expected PrevCursor to be set on page 2")
	}
}

func TestGetTranscriptsPaginated_PhaseFilter(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	opts := TranscriptPaginationOpts{
		Phase: "implement",
		Limit: 100,
	}

	transcripts, pagination, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	if len(transcripts) != 3 {
		t.Errorf("Expected 3 implement phase transcripts, got %d", len(transcripts))
	}

	for _, tr := range transcripts {
		if tr.Phase != "implement" {
			t.Errorf("Expected phase 'implement', got '%s'", tr.Phase)
		}
	}

	if pagination.TotalCount != 3 {
		t.Errorf("Expected TotalCount to be 3, got %d", pagination.TotalCount)
	}
}

func TestGetTranscriptsPaginated_Direction(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	// Get ascending (default)
	optsAsc := TranscriptPaginationOpts{
		Limit:     100,
		Direction: "asc",
	}

	transcriptsAsc, _, err := db.GetTranscriptsPaginated("TASK-001", optsAsc)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated asc failed: %v", err)
	}

	// Get descending
	optsDesc := TranscriptPaginationOpts{
		Limit:     100,
		Direction: "desc",
	}

	transcriptsDesc, _, err := db.GetTranscriptsPaginated("TASK-001", optsDesc)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated desc failed: %v", err)
	}

	if len(transcriptsAsc) != len(transcriptsDesc) {
		t.Fatalf("Expected same number of transcripts, got asc=%d desc=%d", len(transcriptsAsc), len(transcriptsDesc))
	}

	// Verify order is reversed
	if transcriptsAsc[0].ID != transcriptsDesc[len(transcriptsDesc)-1].ID {
		t.Error("First transcript in asc should be last in desc")
	}

	if transcriptsAsc[len(transcriptsAsc)-1].ID != transcriptsDesc[0].ID {
		t.Error("Last transcript in asc should be first in desc")
	}
}

func TestGetPhaseSummary(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	summaries, err := db.GetPhaseSummary("TASK-001")
	if err != nil {
		t.Fatalf("GetPhaseSummary failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("Expected 3 phases, got %d", len(summaries))
	}

	// Verify counts
	expected := map[string]int{
		"spec":      2,
		"implement": 3,
		"review":    1,
	}

	for _, s := range summaries {
		expectedCount, ok := expected[s.Phase]
		if !ok {
			t.Errorf("Unexpected phase: %s", s.Phase)
			continue
		}
		if s.TranscriptCount != expectedCount {
			t.Errorf("Phase %s: expected count %d, got %d", s.Phase, expectedCount, s.TranscriptCount)
		}
	}
}

func TestGetTranscriptsPaginated_EmptyResults(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	opts := TranscriptPaginationOpts{
		Phase: "nonexistent",
		Limit: 50,
	}

	transcripts, pagination, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	if len(transcripts) != 0 {
		t.Errorf("Expected 0 transcripts, got %d", len(transcripts))
	}

	if pagination.HasMore {
		t.Error("Expected HasMore to be false")
	}

	if pagination.TotalCount != 0 {
		t.Errorf("Expected TotalCount to be 0, got %d", pagination.TotalCount)
	}
}

func TestGetTranscriptsPaginated_LimitBounds(t *testing.T) {
	t.Parallel()
	db := setupTranscriptTest(t)
	defer func() { _ = db.Close() }()

	// Test limit > 200 gets capped
	opts := TranscriptPaginationOpts{
		Limit: 300,
	}

	transcripts, _, err := db.GetTranscriptsPaginated("TASK-001", opts)
	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	// Should return all 6 since limit was capped to 200 (which is > 6)
	if len(transcripts) != 6 {
		t.Errorf("Expected 6 transcripts, got %d", len(transcripts))
	}
}

func TestGetTranscriptsPaginated_Performance(t *testing.T) {
	t.Parallel()

	db, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create task first
	task := &Task{
		ID:          "TASK-PERF",
		Title:       "Performance Test Task",
		Description: "Test Description",
		Weight:      "medium",
		Status:      "running",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Create 1000 transcripts to test performance
	baseTime := time.Now()
	const numTranscripts = 1000

	for i := 0; i < numTranscripts; i++ {
		transcript := Transcript{
			TaskID:      "TASK-PERF",
			Phase:       "implement",
			SessionID:   "sess-perf",
			MessageUUID: fmt.Sprintf("msg-perf-%d", i),
			Type:        "assistant",
			Role:        "assistant",
			Content:     "Message content",
			Model:       "claude-3-5-sonnet-20241022",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := db.AddTranscript(&transcript); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	// Test pagination performance
	opts := TranscriptPaginationOpts{
		Limit: 50,
	}

	start := time.Now()
	transcripts, pagination, err := db.GetTranscriptsPaginated("TASK-PERF", opts)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetTranscriptsPaginated failed: %v", err)
	}

	if len(transcripts) != 50 {
		t.Errorf("Expected 50 transcripts, got %d", len(transcripts))
	}

	if pagination.TotalCount != numTranscripts {
		t.Errorf("Expected TotalCount to be %d, got %d", numTranscripts, pagination.TotalCount)
	}

	// Performance requirement: < 100ms
	if elapsed > 100*time.Millisecond {
		t.Errorf("Query took too long: %v (expected < 100ms)", elapsed)
	}

	t.Logf("Pagination query for %d transcripts took %v", numTranscripts, elapsed)
}

func TestGetTranscriptsPaginated_IndexUsage(t *testing.T) {
	t.Parallel()

	db, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create task first
	task := &Task{
		ID:          "TASK-INDEX",
		Title:       "Index Test Task",
		Description: "Test Description",
		Weight:      "medium",
		Status:      "running",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Add some transcripts
	baseTime := time.Now()
	for i := 0; i < 100; i++ {
		transcript := Transcript{
			TaskID:      "TASK-INDEX",
			Phase:       "implement",
			SessionID:   "sess-idx",
			MessageUUID: fmt.Sprintf("msg-idx-%d", i),
			Type:        "assistant",
			Role:        "assistant",
			Content:     "Message",
			Model:       "claude-3-5-sonnet-20241022",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := db.AddTranscript(&transcript); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	// Use EXPLAIN QUERY PLAN to verify index usage
	query := `
		EXPLAIN QUERY PLAN
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE task_id = ? AND id > ?
		ORDER BY id ASC
		LIMIT ?
	`

	rows, err := db.Query(query, "TASK-INDEX", 0, 50)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	// Read the query plan
	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		plan = append(plan, detail)
	}

	// Verify that an index is used
	foundIndex := false
	for _, detail := range plan {
		t.Logf("Query plan: %s", detail)
		// SQLite query plans contain "USING INDEX" or "USING COVERING INDEX" when an index is used
		// Also check for our specific indexes
		if strings.Contains(detail, "USING INDEX") ||
			strings.Contains(detail, "USING COVERING INDEX") ||
			strings.Contains(detail, "idx_transcripts_task_id") ||
			strings.Contains(detail, "idx_transcripts_task") {
			foundIndex = true
		}
	}

	if !foundIndex {
		t.Errorf("Expected query to use an index, but query plan shows: %v", plan)
	}
}
