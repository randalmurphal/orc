# Specification: Implement GET /api/events endpoint for timeline queries

## Problem Statement

The Timeline View feature requires an API endpoint to query persisted events for display. Events are already stored in the `event_log` table via `EventPublisher`, but there's no REST endpoint to query them. The endpoint must support filtering by task, initiative, time range, and event types with pagination support.

## Success Criteria

- [ ] `GET /api/events` endpoint returns paginated events matching filters
- [ ] `task_id` filter works correctly (returns only events for that task)
- [ ] `initiative_id` filter works correctly (joins with tasks table to filter by initiative)
- [ ] `since` and `until` filters work correctly (ISO8601 timestamp parsing)
- [ ] `types` filter works correctly (comma-separated event types)
- [ ] `limit` parameter defaults to 100, max 1000
- [ ] `offset` parameter defaults to 0
- [ ] Response includes `task_title` for each event (from tasks table join)
- [ ] Response includes `total` count for pagination
- [ ] Response includes `has_more` boolean for pagination
- [ ] Returns empty `events` array (not null) when no events match
- [ ] Invalid `task_id` returns empty results (not error)
- [ ] Invalid timestamp format returns 400 error with descriptive message
- [ ] `limit` out of range (< 1 or > 1000) returns 400 error

## Testing Requirements

- [ ] Unit test: `TestHandleGetEvents_NoFilters` - returns all events with default pagination
- [ ] Unit test: `TestHandleGetEvents_TaskIDFilter` - filters by task_id correctly
- [ ] Unit test: `TestHandleGetEvents_InitiativeIDFilter` - filters by initiative_id with task join
- [ ] Unit test: `TestHandleGetEvents_TimeRangeFilter` - since/until parsing and filtering
- [ ] Unit test: `TestHandleGetEvents_EventTypesFilter` - comma-separated types parsing
- [ ] Unit test: `TestHandleGetEvents_Pagination` - limit, offset, total, has_more
- [ ] Unit test: `TestHandleGetEvents_InvalidParams` - error handling for bad input
- [ ] Unit test: `TestHandleGetEvents_EmptyResults` - returns empty array not null
- [ ] Unit test: `TestQueryEventsWithTitles` - DB function returns task titles
- [ ] Unit test: `TestCountEvents` - DB function returns correct count

## Scope

### In Scope
- New `GET /api/events` endpoint with all specified filters
- Database function to query events with task title join
- Database function to count total matching events
- `initiative_id` filter via task table join
- Pagination with `total` and `has_more` fields

### Out of Scope
- WebSocket real-time event streaming (separate implementation)
- Per-task timeline endpoint (`GET /api/tasks/:id/timeline` - future task)
- Maximum date range enforcement (noted in description but not critical for MVP)
- Event aggregation or grouping (timeline UI responsibility)

## Technical Approach

### Response Type

```go
type EventResponse struct {
    ID         int64    `json:"id"`
    TaskID     string   `json:"task_id"`
    TaskTitle  string   `json:"task_title"`
    Phase      *string  `json:"phase,omitempty"`
    Iteration  *int     `json:"iteration,omitempty"`
    EventType  string   `json:"event_type"`
    Data       any      `json:"data,omitempty"`
    Source     string   `json:"source"`
    CreatedAt  string   `json:"created_at"` // ISO8601
}

type EventsListResponse struct {
    Events  []EventResponse `json:"events"`
    Total   int             `json:"total"`
    Limit   int             `json:"limit"`
    Offset  int             `json:"offset"`
    HasMore bool            `json:"has_more"`
}
```

### Database Changes

Add to `internal/db/event_log.go`:

1. **Extend `QueryEventsOptions`** with `InitiativeID string`

2. **Add `EventLogWithTitle`** struct that includes `TaskTitle`

3. **Add `QueryEventsWithTitles(opts) ([]EventLogWithTitle, error)`**
   - Joins `event_log` with `tasks` table
   - If `InitiativeID` set, adds `WHERE t.initiative_id = ?`
   - Returns events with task title populated

4. **Add `CountEvents(opts) (int, error)`**
   - Same filters as QueryEvents but returns count only
   - Uses same join logic for initiative_id filter

### API Handler

Add `internal/api/handlers_events.go`:

```go
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    taskID := r.URL.Query().Get("task_id")
    initiativeID := r.URL.Query().Get("initiative_id")
    sinceStr := r.URL.Query().Get("since")
    untilStr := r.URL.Query().Get("until")
    typesStr := r.URL.Query().Get("types")
    limitStr := r.URL.Query().Get("limit")
    offsetStr := r.URL.Query().Get("offset")

    // Parse and validate timestamps (ISO8601)
    // Parse limit (default 100, max 1000)
    // Parse offset (default 0)
    // Parse types (comma-separated)

    // Build QueryEventsOptions
    // Call QueryEventsWithTitles and CountEvents
    // Build response with has_more calculation
}
```

### Route Registration

Add to `server.go` in `registerRoutes()`:
```go
s.mux.HandleFunc("GET /api/events", cors(s.handleGetEvents))
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/db/event_log.go` | Add `InitiativeID` to options, add `EventLogWithTitle` struct, add `QueryEventsWithTitles()`, add `CountEvents()` |
| `internal/api/handlers_events.go` | New file with `handleGetEvents` handler |
| `internal/api/server.go` | Register `GET /api/events` route |
| `internal/db/event_log_test.go` | Add tests for new DB functions |
| `internal/api/handlers_events_test.go` | New file with handler tests |

## Feature-Specific Analysis

### User Story
As a user viewing the Timeline, I want to query historical events across my tasks so that I can see what happened and when.

### Acceptance Criteria

1. **Basic query works**: `GET /api/events` returns recent events
2. **Task filtering**: `GET /api/events?task_id=TASK-001` returns only that task's events
3. **Initiative filtering**: `GET /api/events?initiative_id=INIT-001` returns events for all tasks in that initiative
4. **Time filtering**: `GET /api/events?since=2026-01-20T00:00:00Z&until=2026-01-21T00:00:00Z` returns events in range
5. **Type filtering**: `GET /api/events?types=phase_completed,task_completed` returns only those types
6. **Pagination**: Response includes `total`, `has_more`, and respects `limit`/`offset`
7. **Task context**: Each event includes `task_title` for display

### Edge Cases

| Case | Behavior |
|------|----------|
| Invalid `task_id` | Returns empty `events` array, `total: 0` |
| Invalid `initiative_id` | Returns empty `events` array, `total: 0` |
| Invalid `since`/`until` format | Returns 400 error with message |
| `limit` < 1 or > 1000 | Returns 400 error with message |
| Very large date range | No enforcement (noted for future) |
| Concurrent requests | No blocking (standard Go HTTP handler) |
| Task deleted | Cascade delete removes events (existing behavior) |

### Performance Considerations

The existing indexes support efficient queries:
- `idx_event_log_task` - for task_id filter
- `idx_event_log_created` - for time range filter
- `idx_event_log_event_type` - for type filter
- `idx_event_log_timeline` - composite for timeline queries

The initiative_id filter requires a join but tasks table is small and indexed.
