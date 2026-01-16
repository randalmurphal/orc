# Knowledge Sync

You are performing automated synchronization of CLAUDE.md with recent learnings from completed tasks.

## Objective

Review recent task transcripts and update CLAUDE.md with new patterns, gotchas, and decisions discovered during development.

## Context

**Recent Tasks Completed:** {{RECENT_COMPLETED_TASKS}}

**Current CLAUDE.md:** {{CLAUDEMD_CONTENT}}

## Process

1. **Extract Learnings**
   Review transcripts from recent tasks for:
   - New patterns that should be documented
   - Gotchas or pitfalls encountered
   - Architectural decisions made
   - Error messages and their solutions
   - Configuration discoveries

2. **Categorize Findings**
   Organize learnings into CLAUDE.md sections:
   - Patterns Learned (reusable approaches)
   - Known Gotchas (problems and solutions)
   - Decisions (architectural choices with rationale)

3. **Update CLAUDE.md**
   - Add new entries to appropriate sections
   - Include source task ID for traceability
   - Keep entries concise but complete
   - Don't duplicate existing entries
   - Remove outdated or superseded information

4. **Verify Accuracy**
   - Cross-reference with actual code
   - Ensure patterns described are still valid
   - Verify gotcha solutions still apply

## Output Format

When complete, output:

```xml
<phase_complete>true</phase_complete>
```

If blocked (e.g., conflicting information found), output:

```xml
<phase_blocked>reason: [description of issue]</phase_blocked>
```

## Guidelines

- Only add genuinely useful learnings
- Keep entries scannable (tables preferred)
- Include TASK-XXX source ID for every entry
- Remove entries that are no longer accurate
- Focus on information that helps future development
