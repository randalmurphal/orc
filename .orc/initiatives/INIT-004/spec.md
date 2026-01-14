# INIT-004: Automated Finalize & Merge

## Vision

Post-PR automation that handles the "last mile" of task completion: syncing with main, resolving conflicts intelligently via Claude, running tests, and merging - with configurable gates for different automation profiles.

## Problem Statement

Current flow stops at PR creation:
```
implement → test → review → completed → PR created → ???
```

After PR approval, manual steps remain:
1. Sync branch with main (may have diverged)
2. Resolve any merge conflicts
3. Re-run tests to verify
4. Actually merge the PR
5. Clean up the branch

This creates friction and delays, especially when:
- Multiple PRs land while waiting for review
- Conflicts arise that require intelligent resolution
- Tests need re-running after sync

## Solution

Add a **finalize phase** that automates the merge process:

```
completed → PR approved → FINALIZE → finished
                            │
                            ├─ sync main → branch
                            ├─ resolve conflicts (Claude)
                            ├─ run tests
                            ├─ risk assessment
                            └─ merge to main
```

## Design Principles

1. **Risk-proportionate automation** - Trivial merges just happen; complex conflicts get attention
2. **No new columns** - All states stay in Done column with visual differentiation
3. **Intelligent conflict resolution** - Claude preserves features from both sides
4. **Configurable gates** - Auto mode is fully automatic; strict mode requires human approval

## Status Model

### New Statuses

| Status | Description | Column |
|--------|-------------|--------|
| `finalizing` | Running finalize process | Done (with spinner) |
| `finished` | Successfully merged to main | Done (with checkmark) |

### Status Transitions

```
completed ──┬──→ finalizing ──┬──→ finished
            │                 │
            │                 └──→ (retry on failure)
            │
            └──→ (manual trigger or auto on PR approval)
```

## Finalize Flow

### 1. Sync with Target Branch

```bash
git fetch origin main
git rebase origin/main  # or merge based on config
```

If conflicts detected → proceed to conflict resolution

### 2. Conflict Resolution (Claude)

**Critical Rules:**
- NEVER remove features from either branch
- If both branches add similar code, merge the functionality intelligently
- If Branch A's changes depend on Branch B's structure, adapt A to work with B
- Preserve all test coverage from both sides

**Process:**
1. For each conflicted file, understand intent from both sides
2. Merge intentions, not just text
3. Run tests after each resolution to verify

### 3. Run Tests

```bash
# Use configured test command or auto-detect
make test  # or npm test, go test, pytest, etc.
```

If tests fail after conflict resolution:
- Analyze failure
- Fix the issue (conflict resolution likely broke something)
- Re-run tests
- Loop until passing or max retries

### 4. Risk Assessment

After sync/conflict resolution, assess the diff:

```bash
git diff main...HEAD --stat
```

| Diff Size | Action |
|-----------|--------|
| < 50 lines (configurable) | Proceed to merge |
| ≥ threshold | Run review pass to sanity check |

### 5. Merge Decision

Based on:
- Conflict complexity (none/minor/major)
- Test results (passing/failing)
- Diff size after resolution
- Configured gate (ai/human/none)

Auto mode: AI decides whether to proceed or escalate
Safe/strict mode: Human approval required for non-trivial merges

### 6. Execute Merge

```bash
git checkout main
git merge --ff-only task-branch  # or --no-ff based on config
git push origin main
git branch -d task-branch
git push origin --delete task-branch
```

## Escalation Logic

| Situation | Action |
|-----------|--------|
| Conflicts resolved, tests pass, small diff | → Merge |
| Conflicts resolved, tests pass, large diff | → Review pass within finalize |
| Conflicts resolved, tests fail | → Fix in finalize, retry |
| Tests fail repeatedly (3+ times) | → Back to `implement` phase |
| Major conflicts with unclear intent | → Block for human |
| AI assesses high risk | → Block for human (safe/strict modes) |

## Configuration

```yaml
completion:
  action: "pr"  # Creates PR when task completes

  finalize:
    enabled: true
    auto_trigger: false  # v2: poll for PR approval

    sync:
      strategy: "rebase"  # rebase | merge

    conflict_resolution:
      enabled: true
      instructions: |
        When resolving conflicts:
        - NEVER remove features from either side
        - If both sides add similar functionality, keep both or merge intelligently
        - If one change depends on another, apply the dependency first
        - Preserve all test coverage

    risk_assessment:
      enabled: true
      re_review_threshold: 50  # Lines changed triggers review

    gates:
      pre_merge: "ai"  # ai | human | none
```

### Profile Overrides

| Profile | Conflict Resolution | Risk Assessment | Pre-merge Gate |
|---------|---------------------|-----------------|----------------|
| `auto` | Claude (default) | AI decides | AI decides |
| `fast` | Claude | Skip | None |
| `safe` | Claude | Always re-review | Human |
| `strict` | Human | Always re-review | Human |

## UI Design

### Done Column States

```
┌────────────────────────────────────────┐
│ TASK-042 • completed                   │  ← Awaiting finalize
│ PR #127 (approved ✓)                   │
│              [Finalize]                │
├────────────────────────────────────────┤
│ TASK-041 • finalizing ◐                │  ← In progress
│ Syncing with main...                   │
├────────────────────────────────────────┤
│ TASK-040 • finished ✓                  │  ← Complete
│ Merged to main @ abc1234               │
└────────────────────────────────────────┘
```

### Visual Indicators

| Status | Card Style |
|--------|------------|
| `completed` | Standard card + "Finalize" button if PR approved |
| `finalizing` | Pulsing border + progress text |
| `finished` | Green checkmark + merge commit info |

### FinalizeModal

When finalize is running, show modal with:
- Current step (syncing, resolving conflicts, testing, merging)
- Live output/logs
- Token usage (if Claude involved)
- Abort button
- Error details if failed

## CLI

```bash
# Manual finalize
orc finalize TASK-042

# With options
orc finalize TASK-042 --force        # Skip risk assessment
orc finalize TASK-042 --gate human   # Override gate config
orc finalize TASK-042 --dry-run      # Show what would happen
```

## API

### POST /api/tasks/{id}/finalize

Trigger finalize for a task.

**Request:**
```json
{
  "force": false,
  "gate_override": "ai"
}
```

**Response:**
```json
{
  "status": "started",
  "task_id": "TASK-042"
}
```

Progress broadcast via WebSocket `finalize` events.

### GET /api/tasks/{id}/finalize/status

Get current finalize status.

**Response:**
```json
{
  "status": "running",
  "step": "resolving_conflicts",
  "progress": {
    "files_resolved": 2,
    "files_remaining": 1
  }
}
```

## Task Breakdown

### Foundation (Parallel)

| Task | Description | Weight |
|------|-------------|--------|
| TASK-086 | Add `finalizing` and `finished` statuses | small |
| TASK-087 | Create finalize phase prompt template | medium |
| TASK-088 | Add finalize configuration options | small |

### Core (Depends on Foundation)

| Task | Description | Weight | Depends On |
|------|-------------|--------|------------|
| TASK-089 | Implement finalize executor logic | large | 086, 087, 088 |

### Extensions (Depends on Core)

| Task | Description | Weight | Depends On |
|------|-------------|--------|------------|
| TASK-090 | Add PR status detection | medium | 089 |
| TASK-091 | Auto-trigger finalize in auto mode | medium | 089, 090 |
| TASK-092 | Add orc finalize CLI command | small | 089 |
| TASK-093 | Add finalize API endpoint | small | 089 |
| TASK-094 | Add finalize UI components | medium | 093 |

## Dependency Graph

```
TASK-086 (statuses)    ──┐
TASK-087 (prompt)      ──┼──→ TASK-089 (executor) ──┬──→ TASK-090 (PR detection) ──→ TASK-091 (auto)
TASK-088 (config)      ──┘                          ├──→ TASK-092 (CLI)
                                                    └──→ TASK-093 (API) ──→ TASK-094 (UI)
```

## Open Questions

1. **PR approval detection** - Poll GitHub/GitLab API vs webhook? Polling is simpler but adds latency.
2. **Conflict resolution prompting** - How much context to give Claude? Full file diffs or just conflict markers?
3. **Test command detection** - Auto-detect from Makefile/package.json or require explicit config?

## Success Criteria

- [ ] Completed tasks can be finalized with one click
- [ ] Conflicts are resolved intelligently (features preserved from both sides)
- [ ] Tests run automatically after sync/resolution
- [ ] Risk assessment triggers re-review when appropriate
- [ ] Auto mode handles everything without human intervention
- [ ] Safe/strict modes provide appropriate gates
- [ ] UI clearly shows finalize progress and status
