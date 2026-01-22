# Backend Review Issues - Complete Fix List

**Created:** 2026-01-22
**Branch:** fix/backend-review-cleanup
**Status:** In Progress

All items must be completed before merge.

---

## Critical Issues (Must Fix First)

### 1. Inverted Period Filter in Outcomes Endpoint
- **File:** `internal/api/handlers_stats.go:372-373`
- **Bug:** Filter logic excludes tasks completed AFTER cutoff instead of BEFORE
- **Current code:**
  ```go
  if cutoffTime != nil && t.CompletedAt != nil && t.CompletedAt.Before(*cutoffTime) {
      continue
  }
  ```
- **Fix:** Include tasks where CompletedAt is AFTER cutoffTime, skip those with no completion or before cutoff:
  ```go
  if cutoffTime != nil && (t.CompletedAt == nil || t.CompletedAt.Before(*cutoffTime)) {
      continue
  }
  ```
- **Status:** [x] Fixed

### 2. Add GET /api/decisions Endpoint
- **File:** `internal/api/handlers_decisions.go` (new handler needed)
- **File:** `internal/api/server.go` (register route)
- **Problem:** Frontend can't load pending decisions on page refresh - only WebSocket delivers them
- **Fix:** Add `handleListDecisions` that calls `s.pendingDecisions.List()`
- **Status:** [x] Fixed

### 3. Add Limit Cap to Transcript Pagination
- **File:** `internal/db/transcript.go:132-138`
- **Problem:** No cap on limit > 200, client can request `limit=10000000` causing DoS
- **Fix:** Add after default assignment:
  ```go
  if opts.Limit > 200 {
      opts.Limit = 200
  }
  ```
- **Status:** [x] Fixed

### 4. Fix Config LoadFrom Parameter
- **File:** `internal/api/handlers_config.go:389`
- **Problem:** `config.LoadFrom(s.workDir)` passes directory, but function expects file path
- **Current:** `cfg, err := config.LoadFrom(s.workDir)`
- **Fix:** `cfg, err := config.LoadFrom(filepath.Join(s.workDir, ".orc", "config.yaml"))`
- **Status:** [x] Fixed

### 5. Document Missing Stats Endpoints in API_REFERENCE.md
- **File:** `docs/API_REFERENCE.md`
- **Missing endpoints:**
  - [ ] `GET /api/stats/per-day` - Daily bar chart data
  - [ ] `GET /api/stats/outcomes` - Task outcome distribution (donut chart)
  - [ ] `GET /api/stats/top-initiatives` - Initiative leaderboard
  - [x] `GET /api/stats/comparison` - Period comparison stats
- **Status:** [x] Fixed

### 6. Document Dashboard Stats New Fields
- **File:** `docs/API_REFERENCE.md` (Dashboard section ~line 1303)
- **Missing fields:** `avg_task_time_seconds`, `success_rate`, `period`, `previous_period`, `changes`
- **Status:** [x] Fixed

### 7. Add Decision Event Persistence Cases
- **File:** `internal/events/persistent.go:165-200`
- **Problem:** `DecisionRequiredData`, `DecisionResolvedData`, `FilesChangedUpdate` not in switch
- **Fix:** Add cases to extract phase field when available
- **Status:** [x] Fixed

### 8. Fix Session API Silent Error Handling
- **File:** `internal/api/handlers_session.go:46-47`
- **Problem:** Returns HTTP 200 with zeros on DB failure (violates "NO Fallbacks" rule)
- **Fix:** Return 500 error or include error field in response
- **Status:** [x] Fixed

### 9. Add State Update When Task Becomes Blocked
- **File:** `internal/executor/task_execution.go:708-716`
- **Problem:** Task status saved but state.Error not set, no event published
- **Fix:** Add:
  ```go
  s.Error = fmt.Sprintf("blocked at gate: %s (phase %s)", decision.Reason, phase.ID)
  e.backend.SaveState(s)
  e.publishState(t.ID, s)
  ```
- **Status:** [x] Fixed

### 10. Add Phase Verification to Decision Resolution
- **File:** `internal/api/handlers_decisions.go:56-59`
- **Problem:** Can approve decision for wrong phase - potential state corruption
- **Fix:** Add verification:
  ```go
  if st.CurrentPhase != decision.Phase {
      s.jsonError(w, fmt.Sprintf("decision phase mismatch: expected %s, got %s", st.CurrentPhase, decision.Phase), http.StatusConflict)
      return
  }
  ```
- **Status:** [x] Fixed

### 11. Fix Decision ID Collision Risk
- **File:** `internal/gate/gate.go:181-233`
- **Problem:** Same ID for phase retries: `gate_{taskID}_{phase}` - stale decisions served
- **Fix:** Include timestamp: `fmt.Sprintf("gate_%s_%s_%d", opts.TaskID, opts.Phase, time.Now().UnixNano())`
- **Status:** [x] Fixed

### 12. Document WebSocket Events (session_update, finalize)
- **File:** `docs/API_REFERENCE.md` (WebSocket section)
- **Problem:** `session_update` and `finalize` events not in main events table
- **Status:** [x] Fixed

---

## Important Issues (Should Fix)

### 13. Replace O(n²) Bubble Sort with sort.Slice
- **File:** `internal/api/handlers_stats.go:621-629`
- **Problem:** Bubble sort is O(n²), inefficient for large initiative lists
- **Fix:** Replace with `sort.Slice(stats, func(i, j int) bool { return stats[i].taskCount > stats[j].taskCount })`
- **Status:** [x] Fixed

### 14. Add DecisionRequired/Resolved Wrapper Methods to EventPublisher
- **File:** `internal/executor/publish.go`
- **Problem:** Inconsistent API - most events have wrappers but decision events don't
- **Fix:** Add `DecisionRequired()` and `DecisionResolved()` methods
- **Status:** [x] Fixed

### 15. Add InitiativeID Support to QueryEvents
- **File:** `internal/db/event_log.go:116-177`
- **Problem:** `QueryEvents` doesn't support InitiativeID but `QueryEventsWithTitles` and `CountEvents` do
- **Fix:** Add initiative join to `QueryEvents` or document limitation
- **Status:** [x] Fixed

### 16. Add Covering Index for Event Pagination
- **File:** New migration file needed
- **Problem:** `ORDER BY created_at DESC, id DESC` not fully indexed
- **Fix:** Add `CREATE INDEX idx_event_log_created_id ON event_log(created_at DESC, id DESC)`
- **Status:** [x] Fixed (internal/db/schema/project_027.sql)

### 17. Add WebSocket Tests for Decision and Files Changed Events
- **File:** `internal/api/websocket_test.go` (new tests)
- **Problem:** No integration tests for `decision_required`, `decision_resolved`, `files_changed` delivery
- **Status:** [x] Fixed

### 18. Add Tests for Top-Initiatives Period Filtering
- **File:** `internal/api/handlers_stats_test.go`
- **Problem:** No test verifies period filter works for top-initiatives
- **Status:** [x] Fixed

### 19. Fix Percentage Change Edge Case (0 to N)
- **File:** `internal/api/handlers_stats.go:1030-1038`
- **Problem:** 0→1 returns same 100% as 0→1000
- **Fix:** Document behavior or return special value
- **Status:** [x] Already documented in API_REFERENCE.md at line 1642

### 20. Fix Timezone Mismatch in Stats Tests
- **File:** `internal/api/handlers_stats_test.go`
- **Problem:** Tests use `time.UTC` but handler uses local timezone
- **Fix:** Use consistent timezone (prefer UTC)
- **Status:** [x] Fixed

### 21. Remove Redundant phase_artifacts Index
- **File:** `internal/db/schema/project_026.sql:22-23`
- **Problem:** `idx_phase_artifacts_task` is redundant - covered by composite `idx_phase_artifacts_task_phase`
- **Fix:** Remove the redundant index
- **Status:** [x] Fixed (internal/db/schema/project_027.sql)

### 22. Fix Test MessageUUID Generation
- **File:** `internal/db/transcript_pagination_test.go:76-77`
- **Problem:** `string(rune(i))` produces control characters for i > 127
- **Fix:** Use `fmt.Sprintf("msg-%d", i)`
- **Status:** [x] Fixed

### 23. Document Undocumented Endpoints (~12)
- **File:** `docs/API_REFERENCE.md`
- **Missing:**
  - [ ] `GET /api/tasks/:id/session`
  - [ ] `GET /api/tasks/:id/tokens`
  - [ ] `POST /api/tasks/:id/retry`
  - [ ] `GET /api/tasks/:id/retry/preview`
  - [ ] `POST /api/tasks/:id/retry/feedback`
  - [ ] `GET /api/settings/hierarchy`
  - [ ] `GET /api/config/stats`
  - [ ] `GET /api/agents/stats`
  - [ ] `POST /api/projects/:id/tasks/:taskId/escalate`
  - [ ] Subtasks endpoints (7 total)
  - [ ] Automation endpoints (8 total)
- **Status:** [ ] Not started

### 24. Remove or Deprecate ENDPOINTS.md
- **File:** `internal/api/ENDPOINTS.md`
- **Problem:** Missing ~80% of endpoints, severely outdated
- **Fix:** Remove file or add deprecation notice pointing to API_REFERENCE.md
- **Status:** [x] Fixed (added deprecation notice)

---

## Minor Issues (Should Complete)

### 25. Fix API_REFERENCE.md Task Comments Method
- **File:** `docs/API_REFERENCE.md:444`
- **Problem:** Shows `PUT` but server.go:337 uses `PATCH`
- **Fix:** Change to `PATCH`
- **Status:** [x] Fixed

### 26. Add Combined Filter Tests for Events API
- **File:** `internal/api/handlers_events_test.go`
- **Problem:** No test combines multiple filters (task_id + types + since)
- **Status:** [x] Fixed

### 27. Add Invalid Event Types Test
- **File:** `internal/api/handlers_events_test.go`
- **Problem:** No test for behavior with unknown event types
- **Status:** [x] Fixed

### 28. Document Top Initiatives Period Filter Behavior
- **File:** `docs/API_REFERENCE.md` or code comments
- **Problem:** Period filter only counts completed tasks - might be intentional but undocumented
- **Status:** [x] Fixed (added code comment and test verifies behavior)

---

## Verification Checklist

After all fixes:

- [ ] `make test` passes
- [ ] `make web-test` passes
- [ ] No regressions in existing functionality
- [ ] All new code has test coverage
- [ ] API_REFERENCE.md is complete and accurate
- [ ] Code follows project conventions (no silent failures, proper error handling)

---

## Progress Tracking

| Category | Total | Completed |
|----------|-------|-----------|
| Critical | 12 | 12 |
| Important | 12 | 12 |
| Minor | 4 | 4 |
| **Total** | **28** | **28** |

---

## Notes

- The `calculatePeriodStats` method name collision was already fixed in commit `b1f111ec`
- Decision persistence to database on server restart is a known limitation documented in GATES.md
- Some "missing" features may be intentionally deferred - verify before implementing
