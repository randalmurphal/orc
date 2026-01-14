# implement - Iteration 1

## Prompt

Complete this task:

**Task**: Add --priority flag to orc new command

**Description**: ## Problem
`orc new --priority critical` and `orc edit --priority critical` both fail with 'unknown flag: --priority'

Priority field exists in task.yaml but can't be set via CLI.

## Solution
Add `--priority` / `-p` flag to both `orc new` and `orc edit` commands accepting: critical, high, normal, low

## Files to Modify
- internal/cli/cmd_new.go
- internal/cli/cmd_edit.go



Implement the fix/change directly. Run any affected tests.
Keep iterating until the implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

The implementation is complete. Let me provide a summary:

## Summary

Added `--priority` / `-p` flag to both `orc new` and `orc edit` commands:

**cmd_new.go changes:**
- Added `--priority` / `-p` flag accepting: `critical`, `high`, `normal`, `low`
- Flag is parsed and validated with `task.IsValidPriority()`
- Priority is set on the task if provided (defaults to `normal` if not specified)
- Output now shows the priority when a task is created

**cmd_edit.go changes:**
- Added `--priority` / `-p` flag accepting: `critical`, `high`, `normal`, `low`
- Flag is parsed and validated with `task.IsValidPriority()`
- Priority changes are tracked and committed like other fields
- Output shows old → new priority when changed

**Usage examples:**
```bash
orc new "Urgent fix" --priority critical
orc new "Important feature" -p high
orc edit TASK-001 --priority critical
orc edit TASK-001 -p low
```

All CLI and task tests pass.

<phase_complete>true</phase_complete>

---
Tokens: 1998778 input, 7847 output, 52987 cache_creation, 1945756 cache_read
Complete: true
Blocked: false
