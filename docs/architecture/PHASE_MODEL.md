# Phase Model

**Purpose**: Define phase types, templates, and transition rules.

---

## Phase Types

| Phase | Purpose | Produces | Commit |
|-------|---------|----------|--------|
| `classify` | Determine task weight | weight assignment | Optional |
| `research` | Investigate codebase | research.md | Yes |
| `spec` | Define requirements | spec.md | Yes |
| `design` | Architecture decisions | design.md | Yes |
| `implement` | Write code | code changes | **Yes** |
| `review` | Code review | review findings + fixes | Yes |
| `docs` | Create/update documentation | README.md, CLAUDE.md, etc. | Yes |
| `test` | Write and run tests | test results | Yes |
| `validate` | Final verification | validation report | Yes |
| `merge` | Merge to target branch | merged code | Yes (final) |

### Docs Phase Details

The `docs` phase runs **after implementation and review**, with full context of what changed:

| Weight | Docs Behavior |
|--------|---------------|
| trivial | Skip (unless missing CLAUDE.md at root) |
| small | Update affected README sections only |
| medium | Create missing docs, update affected existing |
| large | Full doc audit, create all missing, update all affected |
| greenfield | Create complete doc structure from templates |

See [DOCUMENTATION.md](../specs/DOCUMENTATION.md) for full specification.

### Phase Commit Requirement

**Every phase that produces artifacts or code changes MUST commit before marking complete.**

This is critical for:
- **Rollback capability**: Rewind to any phase start
- **Audit trail**: Track what changed when
- **Parallel safety**: Worktrees don't conflict
- **Recovery**: Resume from any checkpoint

#### Commit Message Format

```
[orc] TASK-ID: phase - status

Phase: phase-name
Status: completed|failed
Artifact: [path if applicable]
```

Example:
```
[orc] TASK-001: implement - completed

Phase: implement
Status: completed
Files changed: 5
```

---

## Phase Templates by Weight

### Trivial
```yaml
phases:
  - name: implement
    max_iterations: 3
    checkpoint: false
    gate: auto
```

### Small
```yaml
phases:
  - name: implement
    max_iterations: 5
    checkpoint: true
    gate: auto
  - name: test
    max_iterations: 3
    gate: ai
```

### Medium
```yaml
phases:
  - name: spec
    max_iterations: 3
    gate: ai
  - name: implement
    max_iterations: 10
    checkpoint_frequency: 3
    gate: auto
  - name: review
    max_iterations: 3
    gate: ai
  - name: docs
    max_iterations: 3
    gate: auto
  - name: test
    max_iterations: 3
    gate: auto
```

### Large
```yaml
phases:
  - name: research
    max_iterations: 5
    gate: auto
  - name: spec
    max_iterations: 5
    gate: human
  - name: design
    max_iterations: 3
    gate: human
  - name: implement
    max_iterations: 20
    checkpoint_frequency: 5
    gate: auto
  - name: review
    max_iterations: 5
    gate: ai
  - name: docs
    max_iterations: 5
    gate: auto
  - name: test
    max_iterations: 5
    gate: auto
  - name: validate
    max_iterations: 2
    gate: ai
```

### Greenfield
```yaml
phases:
  - name: research
    max_iterations: 10
    gate: human
  - name: spec
    max_iterations: 10
    gate: human
  - name: design
    max_iterations: 5
    gate: human
  - name: implement
    max_iterations: 30
    checkpoint_frequency: 5
    staged: true
    gate: auto
  - name: review
    max_iterations: 5
    gate: ai
  - name: docs
    max_iterations: 10
    gate: ai
  - name: test
    max_iterations: 10
    gate: auto
  - name: validate
    max_iterations: 3
    gate: human
```

---

## Phase State

```yaml
# Per-phase state in state.yaml
phases:
  implement:
    status: running        # pending | running | completed | failed | skipped
    started_at: 2026-01-10T10:45:00Z
    iterations: 3
    last_checkpoint: abc123
    artifacts:
      - path: src/auth/login.go
        action: created
      - path: src/auth/login_test.go
        action: created
```

---

## Phase Transitions

```
        ┌─────────────────────────────────────┐
        │                                     │
        ▼                                     │
    ┌───────┐     ┌───────┐     ┌───────┐    │
    │pending│────►│running│────►│complete│───┘
    └───────┘     └───┬───┘     └───────┘    (next phase)
                      │
              ┌───────┴───────┐
              ▼               ▼
          ┌──────┐       ┌──────┐
          │failed│       │skipped│
          └──────┘       └──────┘
```

| Transition | Condition |
|------------|-----------|
| pending → running | Previous phase complete, gate passed |
| running → complete | Completion criteria met |
| running → failed | Max iterations exceeded or unrecoverable error |
| pending → skipped | User skips phase (`orc skip --phase`) |

---

## Phase Configuration

```yaml
# Phase definition in plan.yaml
phases:
  - name: implement
    type: implement
    prompt_template: prompts/implement.md
    
    # Iteration control
    max_iterations: 10
    timeout: 600s
    
    # Completion criteria
    completion_criteria:
      - all_tests_pass
      - no_lint_errors
      
    # Checkpointing
    checkpoint: true
    checkpoint_frequency: 3
    
    # Gate
    gate: auto
    gate_criteria:
      - tests_pass
```

---

## Validate Phase: Playwright MCP E2E Testing

The `validate` phase is the **final verification** before merge. For projects with UI components, this phase **MUST** use Playwright MCP tools for comprehensive end-to-end testing.

### Why Playwright MCP for Validation

| Benefit | Description |
|---------|-------------|
| **Real Browser Testing** | Tests run in actual browser, catching issues unit tests miss |
| **Visual Verification** | Screenshot comparison, accessibility snapshots |
| **Full User Flows** | Complete journeys from login to checkout |
| **Component Isolation** | Every single UI component verified independently |
| **Cross-Browser** | Validate Chrome, Firefox, Safari compatibility |

### Validation Scope

The validate phase tests **every single component** end-to-end:

```yaml
# validate phase completion criteria
completion_criteria:
  - playwright_e2e_pass          # All E2E tests pass
  - all_components_covered       # 100% component coverage in E2E
  - accessibility_validated      # WCAG compliance via snapshots
  - visual_regression_pass       # No unexpected visual changes
```

### Playwright MCP Tools Used

| Tool | Purpose | When Used |
|------|---------|-----------|
| `browser_navigate` | Load pages/routes | Start of each test flow |
| `browser_snapshot` | Accessibility tree capture | Verify component state |
| `browser_click` | User interactions | Button clicks, navigation |
| `browser_fill_form` | Form input | Login, data entry flows |
| `browser_take_screenshot` | Visual verification | Compare against baselines |
| `browser_console_messages` | Error detection | Catch JS errors |
| `browser_network_requests` | API verification | Ensure correct API calls |

### Validation Workflow

```
1. browser_navigate → Load application
2. browser_snapshot → Capture initial state
3. For each component:
   a. Navigate to component
   b. browser_snapshot → Verify accessibility
   c. Test all interactions (click, hover, type)
   d. browser_snapshot → Verify state changes
   e. browser_take_screenshot → Visual baseline
4. browser_console_messages → Check for errors
5. browser_network_requests → Verify API calls
6. Report generation → Produce validation report
```

### Validate Phase Configuration

```yaml
# For large/greenfield tasks
validate:
  type: validate
  prompt_template: prompts/validate.md
  max_iterations: 5
  timeout: 1200s  # Longer timeout for E2E

  completion_criteria:
    - playwright_e2e_pass
    - all_components_covered
    - no_console_errors
    - no_failed_network_requests

  tools:
    - playwright_mcp  # Required for validation

  gate:
    type: human  # Human reviews validation report
    criteria:
      - validation_report_exists
      - all_flows_tested
```

### Example Validation Prompt

```markdown
# Validate Phase

Run comprehensive E2E validation using Playwright MCP tools.

## Required Validations

1. **Every UI Component**: Navigate to and test each component
2. **All User Flows**: Complete login, CRUD, logout sequences
3. **Error States**: Trigger and verify error handling
4. **Edge Cases**: Empty states, max values, special characters

## Using Playwright MCP

For each test:
1. Use `browser_snapshot` to capture accessibility tree
2. Verify all interactive elements are accessible
3. Take screenshots for visual regression baseline
4. Check console for errors after each interaction

## Completion Criteria

Output `<phase_complete>true</phase_complete>` when:
- All components have been tested via Playwright
- All user flows complete successfully
- No console errors detected
- No failed network requests
- Validation report created at artifacts/validation-report.md
```

---

## Custom Phases

Projects can define custom phases:

```yaml
# orc.yaml
custom_phases:
  security_scan:
    type: custom
    prompt_template: prompts/security-scan.md
    command: "./scripts/security-scan.sh"
    success_exit_code: 0
    
  deploy_staging:
    type: custom
    prompt_template: prompts/deploy-staging.md
    gate: human
```

Insert into plan:
```yaml
phases:
  - name: implement
  - name: security_scan    # Custom phase
  - name: review
  - name: deploy_staging   # Custom phase
```
