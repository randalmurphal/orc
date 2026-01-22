# Specification: Add pagination to transcript API endpoint

## Problem Statement

The current `GET /api/tasks/:id/transcripts` endpoint loads ALL transcripts into memory, which causes OOM errors and slow responses for large tasks (greenfield with 50+ iterations can generate 15-30MB of transcript data per request).

## Success Criteria

- [ ] Default response limited to 50 transcripts when no `limit` parameter specified
- [ ] `limit` parameter accepted with values 1-200 (rejects values outside range with 400 error)
- [ ] Cursor-based pagination works correctly:
  - [ ] `cursor` parameter accepts transcript ID
  - [ ] Response includes `next_cursor` when more results exist
  - [ ] Response includes `prev_cursor` when viewing later pages
  - [ ] `has_more` boolean correctly indicates if more results exist
- [ ] Direction parameter works:
  - [ ] `direction=asc` (default) returns oldest first
  - [ ] `direction=desc` returns newest first
- [ ] Phase filter works: `?phase=implement` returns only implement phase transcripts
- [ ] `phases` summary included in response showing all phases with counts
- [ ] `total_count` in pagination shows total matching transcripts (uses COUNT query, excludes cursor filter)
- [ ] Query uses indexes, no full table scans (verified via EXPLAIN QUERY PLAN)
- [ ] Response time < 100ms for any page (tested with 1000+ transcripts)

## Testing Requirements

- [ ] Unit test: `TestGetTranscriptsPaginated_DefaultLimit` - verifies 50 default limit
- [ ] Unit test: `TestGetTranscriptsPaginated_CustomLimit` - verifies limit 1-200 range
- [ ] Unit test: `TestGetTranscriptsPaginated_CursorNavigation` - verifies cursor-based pagination forward and backward
- [ ] Unit test: `TestGetTranscriptsPaginated_PhaseFilter` - verifies phase filtering
- [ ] Unit test: `TestGetTranscriptsPaginated_Direction` - verifies asc/desc ordering
- [ ] Unit test: `TestGetTranscriptsPaginated_PhaseSummary` - verifies phases array in response
- [ ] Unit test: `TestGetTranscriptsPaginated_TotalCount` - verifies total_count accuracy
- [ ] Integration test: `TestTranscriptPaginationPerformance` - verifies <100ms with 500+ transcripts
- [ ] Integration test: `TestTranscriptPaginationIndexUsage` - verifies index usage via EXPLAIN

## Scope

### In Scope
- Cursor-based pagination with configurable limit
- Phase filter (single phase)
- Direction parameter (asc/desc)
- Phases summary in response
- Total count for filtered results (excludes cursor position)
- Index creation for new queries
- Handler updates for query parameter parsing
- Database layer pagination method

### Out of Scope
- `section_type` filter (TASK-399 dependency - mentioned for future)
- Full-text search integration with pagination
- Streaming/infinite scroll implementation
- Frontend changes (separate task)
- Multiple phase filter (array of phases)
- Iteration filters (transcripts don't track iteration number)

## Technical Approach

### Query Parameter Parsing

```go
type TranscriptPaginationOpts struct {
    Phase     string // Filter by phase (optional)
    Cursor    int64  // Cursor for pagination (transcript ID, 0 = start)
    Limit     int    // Max results (default: 50, max: 200)
    Direction string // 'asc' | 'desc' (default: asc)
}
```

### Database Query Strategy

Cursor-based pagination is more efficient than offset for large datasets:

```sql
-- Forward pagination (direction=asc, default)
SELECT * FROM transcripts
WHERE task_id = ?
  AND id > ?  -- cursor
  AND (phase = ? OR ? = '')  -- optional phase filter
ORDER BY id ASC
LIMIT ?

-- Backward pagination uses id < cursor with DESC order, then reverse results
```

Key efficiency considerations:
1. Use `id` as cursor (primary key, indexed) - NOT timestamp (could have duplicates)
2. Add composite index for filtered queries
3. Use COUNT(*) with same WHERE clause for total_count (separate query)
4. Phase summary uses GROUP BY query

### Response Schema

```json
{
  "transcripts": [
    {
      "id": 12345,
      "task_id": "TASK-001",
      "phase": "implement",
      "session_id": "uuid",
      "message_uuid": "uuid",
      "parent_uuid": "uuid",
      "type": "assistant",
      "role": "assistant",
      "content": "...",
      "model": "claude-3-5-sonnet-20241022",
      "input_tokens": 1000,
      "output_tokens": 500,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 200,
      "tool_calls": "[...]",
      "tool_results": "{...}",
      "timestamp": "2026-01-16T14:30:00Z"
    }
  ],
  "pagination": {
    "next_cursor": "12400",
    "prev_cursor": "12300",
    "has_more": true,
    "total_count": 523
  },
  "phases": [
    {"phase": "spec", "transcript_count": 45},
    {"phase": "implement", "transcript_count": 350},
    {"phase": "review", "transcript_count": 78},
    {"phase": "test", "transcript_count": 50}
  ]
}
```

### Files to Modify

- `internal/api/handlers_tasks_state.go`:
  - Update `handleGetTranscripts` to parse query params and call paginated method
  - Add response structure with pagination metadata
  - Parse and validate query parameters

- `internal/db/transcript.go`:
  - Add `TranscriptPaginationOpts` struct
  - Add `GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error)`
  - Add `GetPhaseSummary(taskID string) ([]PhaseSummary, error)`

- `internal/db/schema/project_022.sql` (new migration):
  - Add composite index: `CREATE INDEX idx_transcripts_task_id ON transcripts(task_id, id)`
  - This supports efficient cursor pagination with task filtering

- `internal/storage/backend.go`:
  - Add `GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error)` to interface
  - Add `GetPhaseSummary(taskID string) ([]PhaseSummary, error)` to interface

- `internal/storage/database_backend.go`:
  - Implement `GetTranscriptsPaginated` delegating to db layer
  - Implement `GetPhaseSummary` delegating to db layer

- `internal/api/server_test.go`:
  - Add pagination tests

- `internal/db/db_test.go`:
  - Add pagination query tests

## Feature Details

### User Story

As a **frontend developer using the orc API**, I want **paginated transcript results** so that **I can efficiently load and display transcripts for large tasks without memory issues or long wait times**.

### Acceptance Criteria

1. **API contract preserved**: Clients can still parse `transcripts` array from response (key name unchanged)
2. **Sensible defaults**: No parameters = first 50 transcripts, oldest first
3. **Navigation possible**: `next_cursor` allows fetching next page; `prev_cursor` allows going back
4. **Metadata available**: `phases` summary lets UI show navigation/jumping to specific phases
5. **Filtering works**: Can get just one phase's transcripts efficiently
6. **Performance**: Any paginated request completes in <100ms regardless of total transcript count
7. **Compatibility**: Old API tests continue passing (response structure is additive)

### Edge Cases

1. **Empty results**: Return empty `transcripts` array, `has_more: false`, `total_count: 0`
2. **Invalid cursor**: Return 400 error "invalid cursor format"
3. **Limit out of range**: Return 400 error "limit must be between 1 and 200"
4. **Invalid direction**: Return 400 error "direction must be 'asc' or 'desc'"
5. **Non-existent phase**: Return empty results (not error) - phase filter is best-effort
6. **Cursor beyond end**: Return empty results, `has_more: false`
