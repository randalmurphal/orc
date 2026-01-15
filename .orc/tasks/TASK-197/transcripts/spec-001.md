# spec - Iteration 1

## Prompt

Create a specification for this task:

**Task**: Add process/resource tracking to diagnose system freezes
**Category**: {{TASK_CATEGORY}}
**Description**: ## Problem
System freezes occur across Linux, Mac, and WSL after running multiple orc tasks. Suspected cause: orphaned MCP server processes (Playwright, browsers) not being cleaned up when Claude CLI sessions end.

## Root Cause Analysis
When orc runs a task:
```
orc (Go process)
└── Claude CLI (spawned by llmkit)
    └── MCP servers (Playwright, etc)
        └── Chromium browsers
```

llmkit kills Claude CLI on session.Close(), but doesn't kill the process GROUP - leaving MCP servers and browsers as orphans that accumulate.

## Solution: Add Resource Tracking

### 1. Process Tree Tracking
- Before task: snapshot running processes
- After task: compare, log any new orphans
- Track: PID, PPID, command, memory usage

### 2. Memory Tracking  
- Log memory before/after each phase
- Alert if memory grows significantly between tasks

### 3. Child Process Cleanup (llmkit fix)
- Use process groups (Setpgid) when spawning Claude CLI
- Kill entire process group on Close(), not just parent

### 4. MCP Server Lifecycle Logging
- Log when MCP servers start/stop
- Detect orphaned MCP processes

## Files to Modify
- internal/executor/session_adapter.go - Add pre/post tracking
- internal/executor/executor.go - Add memory logging
- llmkit/claude/session/session.go - Process group kill (separate PR)

## Success Criteria
- Logs show process counts before/after tasks
- Orphaned processes are detected and logged
- Memory growth is tracked between tasks

{{INITIATIVE_CONTEXT}}

## Instructions

Create a clear, actionable specification that defines exactly what needs to be done
and how to verify it's complete.

### 1. Problem Statement
Summarize what needs to be solved in 1-2 sentences.

### 2. Success Criteria (REQUIRED)
Define specific, testable criteria as checkboxes:
- Each criterion must be verifiable (file exists, test passes, API returns X)
- No vague language ("works well", "is fast")
- Include both functional and quality criteria

### 3. Testing Requirements (REQUIRED)
Specify what tests must pass:
- [ ] Unit test: [specific test description]
- [ ] Integration test: [if applicable]
- [ ] E2E test: [if UI changes]

### 4. Scope
Define boundaries to prevent scope creep:
- **In Scope**: What will be implemented
- **Out of Scope**: What will NOT be implemented

### 5. Technical Approach
Brief plan for implementation:
- Files to modify
- Key changes in each file

### 6. Category-Specific Analysis

**If this is a BUG (category=bug):**
- Reproduction Steps: Exact steps to trigger the bug
- Current Behavior: What happens now (the bug)
- Expected Behavior: What should happen
- Root Cause: Where the bug originates (if known)
- Verification: How to confirm the fix works

**If this is a FEATURE (category=feature):**
- User Story: As a [user], I want [feature] so that [benefit]
- Acceptance Criteria: Specific conditions for feature acceptance

**If this is a REFACTOR (category=refactor):**
- Before Pattern: Current code/architecture
- After Pattern: Target code/architecture
- Risk Assessment: What could break

## Output Format

Wrap your spec in artifact tags:

<artifact>
# Specification: Add process/resource tracking to diagnose system freezes

## Problem Statement
[1-2 sentences]

## Success Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

## Testing Requirements
- [ ] [Test 1]
- [ ] [Test 2]

## Scope
### In Scope
- [Item]
### Out of Scope
- [Item]

## Technical Approach
[Brief implementation plan]

### Files to Modify
- [file]: [change]

## [Category-Specific Section]
[Include appropriate section based on category]
</artifact>

After completing the spec, commit:
```bash
git add -A
git commit -m "[orc] TASK-197: spec - completed"
```

Then output:
```
**Commit**: [SHA]
<phase_complete>true</phase_complete>
```

If blocked (requirements unclear):
```
<phase_blocked>
reason: [what's unclear]
needs: [what clarification is needed]
</phase_blocked>
```


## Response

**Commit**: 0cf5185

<phase_complete>true</phase_complete>

---
Tokens: 425855 input, 2643 output, 68573 cache_creation, 356943 cache_read
Complete: true
Blocked: false
