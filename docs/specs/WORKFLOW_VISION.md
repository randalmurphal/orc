# Orc Workflow Vision

## Overview

This document outlines the ideal workflow for orc in real dev scenarios, addressing gaps in the current implementation and proposing new features.

## Core Principle

**User provides ideas and decisions. Claude provides structure and execution.**

The orchestration should keep humans in the loop for what matters (requirements, decisions, review) while automating what doesn't (implementation, testing, documentation).

---

## Real-World Scenarios

### Scenario A: New Project (Greenfield)

**Current State:**
- Single monolithic task attempts entire project
- No place for user to express overall vision
- CLAUDE.md must be written manually

**Ideal State:**
```bash
orc init                    # Bootstrap .orc/
orc vision                  # Interactive: define project goals, constraints, components
                            # Output: project-vision.md, component breakdown
orc plan                    # AI proposes task sequence with dependencies
                            # User reviews/approves
orc run --all               # Execute in dependency order
```

### Scenario B: Large Feature Addition

**Current State:**
- Large features crammed into single task
- No shared feature context across sub-tasks
- No integration with existing codebase patterns

**Ideal State:**
```bash
orc feature "Add real-time notifications"
# Interactive session:
# 1. Claude researches existing codebase (WebSockets? Events?)
# 2. User describes requirements
# 3. Claude proposes approach
# 4. User refines
# Output:
#   .orc/features/real-time-notifications/
#     spec.md          # Detailed feature spec
#     tasks.yaml       # Sub-tasks with dependencies
#     context.md       # What Claude learned about existing code

orc run --feature real-time-notifications
# Runs all sub-tasks with shared feature context
```

### Scenario C: Bug Fix / Minor Update

**Current State:** Works well with trivial/small weights.

**Improvements:**
- Auto-detect if docs need updating
- Link to issue tracker (optional integration)
- Suggest related test cases

---

## Missing Concepts

### 1. Initiatives / Features

A grouping above tasks that provides shared context:

```yaml
# .orc/initiatives/INIT-001.yaml
id: INIT-001
title: "User Authentication System"
status: active

vision: |
  A secure authentication system using JWT tokens,
  following OWASP guidelines.

decisions:
  - "Using bcrypt for password hashing"
  - "7-day token expiry, 30-day refresh"

context_files:
  - .orc/initiatives/INIT-001/research.md
  - .orc/initiatives/INIT-001/spec.md

tasks:
  - id: TASK-001
    title: "Auth data models"
    status: completed
  - id: TASK-002
    title: "Login/logout endpoints"
    depends_on: [TASK-001]
    status: pending
```

### 2. Spec Sessions (Interactive)

User-driven spec creation using the Spawner pattern from setup:

```bash
orc spec "Add user authentication"
```

Spawns interactive Claude session with:
1. Research existing codebase
2. Ask clarifying questions
3. Propose approach
4. User refines
5. Output structured spec

### 3. Artifact Persistence

**Current Gap:** `loadPriorContent()` returns empty string.

**Fix:**
```go
func loadPriorContent(taskDir string, phaseID string) string {
    path := filepath.Join(taskDir, "artifacts", phaseID+".md")
    content, err := os.ReadFile(path)
    if err != nil {
        return ""
    }
    return string(content)
}
```

This enables:
- `{{SPEC_CONTENT}}` contains actual spec in implement phase
- `{{RESEARCH_CONTENT}}` contains research findings
- `{{TDD_TESTS_CONTENT}}` contains TDD test artifacts

### 4. Task Dependencies

```yaml
# task.yaml
id: TASK-002
depends_on:
  - TASK-001  # Must complete first
context_from:
  - TASK-001: artifacts/design.md  # Import this artifact
```

### 5. Docs Phase Integration

Add docs phase to plan templates:

```yaml
# templates/plans/medium.yaml
phases:
  - id: implement
  - id: test
  - id: docs   # NEW
```

With configuration:
```yaml
# .orc/config.yaml
documentation:
  enabled: true
  update_on: [feature, api_change]
  auto_update_claudemd: true
```

### 6. Test Configuration

```yaml
# .orc/config.yaml
testing:
  required: true
  coverage_threshold: 80
  types: [unit, integration]
  skip_for_weights: [trivial]
```

---

## Proposed Commands

| Command | Purpose | Status |
|---------|---------|--------|
| `orc init` | Bootstrap project | Exists |
| `orc setup` | Interactive project config | Exists |
| `orc vision` | Define project vision | NEW |
| `orc spec <title>` | Interactive spec session | NEW |
| `orc plan` | Decompose into tasks | NEW |
| `orc feature <name>` | vision + spec + plan combined | NEW |
| `orc new` | Create individual task | Exists |
| `orc run` | Execute task(s) | Exists |
| `orc run --feature <name>` | Run feature's tasks | NEW |
| `orc run --initiative <id>` | Run initiative's tasks | NEW |
| `orc docs update` | Trigger docs update | NEW |

---

## Implementation Phases

### Phase 1: Artifact Persistence (Foundation)

**Priority: Critical**

Without this, phases can't share context.

1. Implement `loadPriorContent()` to read artifacts from disk
2. Ensure phases actually save to `artifacts/` directory
3. Add artifact parsing for structured content (YAML frontmatter?)
4. Test end-to-end: spec → implement → test with context passing

**Files:**
- `internal/executor/template.go` - Fix loadPriorContent
- `templates/prompts/*.md` - Verify artifact save instructions

### Phase 2: Docs Integration

**Priority: High**

Auto-maintained documentation is a key value proposition.

1. Add docs phase to medium/large/greenfield plans
2. Create/verify docs.md prompt template
3. Implement `documentation` config section
4. Auto-update CLAUDE.md sections marked with `<!-- orc:auto -->`

**Files:**
- `templates/plans/*.yaml` - Add docs phase
- `templates/prompts/docs.md` - Docs prompt
- `internal/config/config.go` - Add DocumentationConfig

### Phase 3: Spec Sessions

**Priority: High**

User-in-the-loop spec creation.

1. Create `orc spec` command
2. Build spec session prompt template
3. Use Spawner pattern from setup package
4. Save output to structured format
5. Integrate with task creation

**Files:**
- `internal/cli/cmd_spec.go` - New command
- `internal/spec/` - New package
- `internal/spec/builtin/spec_session.yaml` - Prompt template

### Phase 4: Initiatives/Features

**Priority: Medium**

Grouping and dependencies for multi-task work.

1. Define initiative YAML schema
2. Add `depends_on` field to task
3. Implement dependency resolution in executor
4. Create `orc feature` command
5. Add `orc run --feature` execution

**Files:**
- `internal/initiative/` - New package
- `internal/task/task.go` - Add DependsOn field
- `internal/executor/executor.go` - Dependency checking

### Phase 5: Test Configuration

**Priority: Medium**

Configurable test requirements.

1. Add testing config section
2. Parse test output for structured failures
3. Implement coverage checking (if measurable)
4. Better retry context with specific failures

**Files:**
- `internal/config/config.go` - Add TestingConfig
- `internal/executor/completion.go` - Parse test output

---

## Spec Session Prompt Template

```markdown
# Specification Session

You are helping the user create a detailed specification for a feature or task.

## Your Role

1. **Research First**: Explore the codebase to understand existing patterns
2. **Ask Questions**: Don't assume - clarify requirements
3. **Propose Approach**: Based on research, suggest implementation
4. **Refine Together**: Iterate with user until spec is clear
5. **Structure Output**: Create formal spec document

## Process

### Step 1: Understand the Request
Ask the user:
- What problem are you solving?
- Who will use this?
- What does success look like?

### Step 2: Research Codebase
- Look at existing patterns
- Identify integration points
- Note relevant dependencies

### Step 3: Clarify Details
Ask about:
- Edge cases
- Error handling
- Performance requirements
- Security considerations

### Step 4: Propose Approach
Present options if multiple exist.
Explain tradeoffs.

### Step 5: Create Spec
Output structured specification with:
- Problem statement
- Success criteria (testable)
- Scope (in/out)
- Technical approach
- Files to modify
- Edge cases
- Open questions

Save to: `.orc/features/{name}/spec.md`
```

---

## Documentation Auto-Update

CLAUDE.md sections can be marked for auto-update:

```markdown
## API Endpoints

<!-- orc:auto:api-endpoints -->
| Method | Path | Description |
|--------|------|-------------|
| GET | /api/users | List users |
| POST | /api/users | Create user |
<!-- /orc:auto:api-endpoints -->
```

During docs phase, Claude:
1. Finds sections marked with `<!-- orc:auto:* -->`
2. Regenerates content based on current code
3. Updates in place

---

## Summary

The current orc design is task-centric, but real dev work requires:
- **Shared context** across related tasks (initiatives/features)
- **User collaboration** for specs (not just automation)
- **Automatic documentation** that grows with the project
- **Configurable testing** based on project needs
- **Artifact persistence** so phases can reference prior work

Implementing these in phases (artifact persistence → docs → spec sessions → initiatives → testing) builds the foundation for truly valuable autonomous development assistance.
