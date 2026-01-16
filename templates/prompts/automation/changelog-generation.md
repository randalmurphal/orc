# Changelog Generation

You are generating a changelog from recently completed tasks.

## Objective

Create or update the project changelog with entries from recently completed tasks, following conventional changelog format.

## Context

**Recent Tasks Completed:** {{RECENT_COMPLETED_TASKS}}

**Current Changelog:** {{CHANGELOG_CONTENT}}

## Process

1. **Categorize Changes**
   Group tasks by type:
   - **Added** - New features
   - **Changed** - Changes to existing functionality
   - **Deprecated** - Features marked for removal
   - **Removed** - Removed features
   - **Fixed** - Bug fixes
   - **Security** - Security-related changes

2. **Write Entries**
   For each task:
   - Write clear, user-facing description
   - Reference task ID for traceability
   - Include PR/MR number if available
   - Note breaking changes prominently

3. **Update Changelog**
   - Add new section with current date if needed
   - Insert entries under appropriate categories
   - Keep existing entries unchanged
   - Follow Keep a Changelog format

4. **Format Check**
   - Verify markdown formatting
   - Check for consistent entry style
   - Ensure dates are correct

## Output Format

When complete, output:

```xml
<phase_complete>true</phase_complete>
```

If unable to categorize certain tasks, output:

```xml
<phase_blocked>reason: [description of unclear tasks]</phase_blocked>
```

## Guidelines

- Write for end users, not developers
- Keep entries concise but informative
- Always note breaking changes at top
- Use present tense for entries
- Include issue/PR references where available
