# Specification: Add --status flag to orc edit command

## Problem Statement
Users cannot change task status via CLI without running the task or manually editing YAML files. This makes administrative corrections (like marking already-fixed tasks as completed) unnecessarily difficult.

## Success Criteria
- [ ] `orc edit TASK-XXX --status completed` command works
- [ ] `orc edit TASK-XXX --status planned` command works (and similar for other valid statuses)
- [ ] Invalid status values are rejected with clear error message listing valid options
- [ ] Running tasks cannot have their status changed (existing check already blocks this)
- [ ] Status change triggers auto-commit like other edit changes
- [ ] Status change is displayed in command output
- [ ] `--status` flag appears in `orc edit --help` with valid options listed

## Testing Requirements
- [ ] Unit test: `TestEditCommand_StatusFlag` - verify flag exists with shorthand
- [ ] Unit test: `TestEditCommand_StatusValidation` - verify invalid status values are rejected
- [ ] Unit test: `TestEditCommand_StatusChange` - verify valid status changes are persisted
- [ ] Integration test: verify `orc edit TASK-XXX --status completed` updates task.yaml

## Scope

### In Scope
- Add `--status` / `-s` flag to edit command
- Validate status values against `task.ValidStatuses()`
- Persist status change to task.yaml
- Trigger auto-commit for status changes
- Display status change in output

### Out of Scope
- Complex state machine transition validation (e.g., "can't go from planned to running without actually running")
  - Rationale: This is an administrative correction tool. The user knows what they're doing.
  - The existing check blocking edits to running tasks is sufficient protection.
- Automatic state.yaml updates when status changes (status in task.yaml vs state.yaml are separate concepts)
- Timestamp updates (started_at, completed_at) - user should use `orc run` for proper execution

## Technical Approach

The implementation follows the existing pattern for other edit flags (priority, weight, etc.):

1. Add `--status` string flag with `-s` shorthand
2. Read the flag value in RunE
3. Validate against `task.IsValidStatus()`
4. Update `t.Status` if different from current
5. Add "status" to changes slice
6. Display old/new status in output

### Files to Modify

- `internal/cli/cmd_edit.go`:
  - Add flag definition: `cmd.Flags().StringP("status", "s", "", "...")`
  - Add flag parsing: `newStatus, _ := cmd.Flags().GetString("status")`
  - Add validation: `if !task.IsValidStatus(...)` with error listing valid options
  - Add update logic: `if t.Status != s { t.Status = s; changes = append(...) }`
  - Add output case: `case "status": fmt.Printf("   Status: %s -> %s\n", ...)`
  - Update Long description to document the flag

- `internal/cli/cmd_edit_test.go`:
  - Add `TestEditCommand_StatusFlag` to verify flag existence
  - Add `TestEditCommand_StatusValidation` with invalid input
  - Add `TestEditCommand_StatusChange` with actual status update

## Feature Analysis

### User Story
As an orc user, I want to change task status via CLI so that I can make administrative corrections without editing YAML files directly.

### Acceptance Criteria
1. `orc edit TASK-001 --status completed` marks task as completed
2. `orc edit TASK-001 -s planned` marks task as planned (shorthand works)
3. `orc edit TASK-001 --status invalid` returns error: `invalid status "invalid" - valid options: created, classifying, planned, running, paused, blocked, finalizing, completed, finished, failed`
4. Cannot change status of running task (existing guard already handles this)
5. Change is committed to git with message including "status" in change list
