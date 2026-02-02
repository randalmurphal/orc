# Templates

Embedded prompt templates for phase execution.

## Directory Structure

```
templates/
├── embed.go              # Go embed directives
├── prompts/              # ALL prompt templates
│   ├── *.md              # Phase and session prompts (22 files)
│   └── automation/*.md   # Maintenance automation templates (8 files)
├── workflows/            # Built-in workflow definitions (9 YAML files)
├── phases/               # Built-in phase template definitions (11 YAML files)
├── agents/               # Sub-agent definitions (9 built-in)
├── docs/                 # Documentation templates
├── scripts/              # Helper scripts
└── pr-body.md            # PR description template
```

## Workflows (YAML-First)

Built-in workflows are defined as YAML in `templates/workflows/` and synced to GlobalDB on startup via `workflow.CacheService`. YAML files are the source of truth; DB is a runtime cache.

| Workflow ID | Phases | Use Case |
|-------------|--------|----------|
| `implement-trivial` | implement | One-liner fixes, typos |
| `implement-small` | tiny_spec → implement → review | Bug fixes, isolated changes |
| `implement-medium` | spec → tdd_write → implement → review → docs | Standard features |
| `implement-large` | spec → tdd_write → breakdown → implement → review → docs | Complex multi-file features |
| `review` | review | Review existing changes |
| `spec` | spec | Generate spec only |
| `docs` | docs | Documentation only |
| `qa` | qa | Manual QA session |
| `qa-e2e` | qa_e2e_test ⟳ qa_e2e_fix | E2E browser testing with fix loop |

**Resolution priority:** personal (`~/.orc/workflows/`) > local (`.orc/local/workflows/`) > project (`.orc/workflows/`) > embedded (`templates/workflows/`)

**Key concepts:**
- **TDD-first**: Tests written before implementation (medium/large via tdd_write phase)
- **Test classification**: tdd_write classifies tests as solitary (mocked), sociable (real collaborators), or integration (wiring verification)
- **Integration test mandate**: New code wired into existing paths MUST have integration tests proving the wiring works
- **Review includes verification**: The review phase handles success criteria verification
- **No separate test phase**: TDD handles testing upfront
- **Composable phases**: Each phase is a reusable template in `phase_templates` table

**Review phase** (small+): Multi-agent code review with specialized reviewers.

**Note**: `finalize` is a manual command (`orc finalize TASK-XXX`), not an automatic workflow phase.
Use it to sync with target branch and resolve conflicts before merge.

## Template Variables

| Variable | Description |
|----------|-------------|
| `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}` | Task context |
| `{{TASK_CATEGORY}}` | feature/bug/refactor/etc |
| `{{PHASE}}`, `{{WEIGHT}}`, `{{ITERATION}}` | Execution context |
| `{{SPEC_CONTENT}}` | Spec phase artifact |
| `{{TDD_TESTS_CONTENT}}`, `{{TDD_TEST_PLAN}}` | TDD phase output |
| `{{BREAKDOWN_CONTENT}}` | Task breakdown output |
| `{{RETRY_ATTEMPT}}`, `{{RETRY_FROM_PHASE}}`, `{{RETRY_REASON}}` | Retry context via variable resolver (empty on first run) |
| `{{RETRY_FEEDBACK}}`, `{{RETRY_FAILED_CRITERIA}}`, `{{RETRY_MAX_ATTEMPTS}}` | Structured retry context via prompt service. `{{RETRY_CONTEXT}}` available for backward compat |
| `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}`, `{{TARGET_BRANCH}}` | Git context |
| `{{INITIATIVE_CONTEXT}}` | Initiative details |
| `{{LANGUAGE}}`, `{{HAS_FRONTEND}}`, `{{HAS_TESTS}}` | Project detection |
| `{{TEST_COMMAND}}`, `{{LINT_COMMAND}}`, `{{BUILD_COMMAND}}` | Project commands |
| `{{ERROR_PATTERNS}}` | Language-specific error handling idioms |
| `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{TEST_RESULTS}}` | UI testing |
| `{{REVIEW_ROUND}}`, `{{REVIEW_FINDINGS}}` | Review phase |
| `{{CONSTITUTION_CONTENT}}` | Project principles |

## Phase Prompts

| File | Purpose |
|------|---------|
| `classify.md` | Weight classification |
| `research.md` | Pattern research |
| `spec.md` | Technical specification with user stories and quality checklist |
| `tiny_spec.md` | Combined spec+TDD for trivial/small tasks |
| `design.md` | Create design document |
| `tdd_write.md` | Write failing tests before implementation (classifies solitary/sociable/integration, requires integration tests for wiring) |
| `breakdown.md` | Break spec into checkboxed implementation tasks |
| `implement.md` | Implementation with TDD context, must make tests pass |
| `review.md` | Multi-agent code review (6 reviewers incl. no-op detection) + success criteria verification |
| `docs.md` | Documentation (AI doc standards, hierarchical inheritance, doc type templates) |
| `qa.md` | Manual QA verification session |
| `test.md` | Test execution template |
| `finalize.md` | Branch sync, conflict resolution (manual command) |

**Session prompts:** `spec_session.md`, `plan_session.md`, `plan_from_spec.md`, `setup.md`

**Review rounds:** `review_round1.md`, `review_round2.md`

**Gates:** `conflict_resolution.md`

**Automation:** `automation/*.md` (8 maintenance templates)

## Built-in Agents

Agent definitions in `agents/*.md` with YAML frontmatter (name, model, tools) + prompt. Seeded to GlobalDB on startup via `workflow.SeedAgents()`.

| Agent ID | Model | Purpose | Used By |
|----------|-------|---------|---------|
| `code-reviewer` | opus | Guidelines compliance review | Review phase (parallel) |
| `code-simplifier` | opus | Complexity and simplification analysis | Review phase (parallel) |
| `comment-analyzer` | haiku | Comment quality and accuracy | Review phase (parallel) |
| `dependency-validator` | haiku | Detect missing code-level deps between initiative tasks | `on_initiative_planned` trigger |
| `over-engineering-detector` | opus | Detects scope creep and unnecessary abstractions | Review phase (parallel) |
| `pr-test-analyzer` | sonnet | Test coverage and quality analysis | Review phase (parallel) |
| `silent-failure-hunter` | opus | Error handling and silent failure detection | Review phase (parallel) |
| `spec-quality-auditor` | opus | Validates success criteria are behavioral and testable | Review phase (parallel) |
| `type-design-analyzer` | sonnet | Type system and interface design review | Review phase (parallel) |

**Trigger agents** (like `dependency-validator`) run via `WorkflowTrigger` definitions, not phase agents. See `docs/architecture/GATES.md` for trigger configuration.

## Prompt Structure Best Practices

Based on Anthropic prompting research. These patterns are applied across all phase and agent prompts.

### Section Ordering (Critical)

Prompts follow this top-to-bottom order for optimal model attention:

| Order | Section | Why First |
|-------|---------|-----------|
| 1 | **Output format** (`<output_format>`) | Model anchors on expected structure early |
| 2 | **Critical constraints** (quality gates, checklists) | Hard requirements before creative work |
| 3 | **Examples** (multishot, input→output pairs) | Concrete patterns calibrate behavior |
| 4 | **Context** (task metadata, project detection) | Ground the model in specifics |
| 5 | **Injected artifacts** (constitution, initiative, spec) | Reference material |
| 6 | **Instructions** (streamlined guidance) | Last — model has full context to interpret |

### XML Tags for Structure

Use XML tags (`<output_format>`, `<project_context>`, `<instructions>`) instead of markdown headers for machine-parsed sections. Models parse XML boundaries more reliably than `##` headers in long prompts.

### System Identity

Every phase prompt starts with a one-line identity statement: "You are a [role] working on [task type]." This anchors the model's behavior before any instructions.

### Failure Mode Priming

State the most common failure mode explicitly near the top:
- **Spec**: "Most common failure is success criteria that verify existence instead of behavior"
- **Implement**: "Most common failure is declaring completion without running verification"
- **TDD**: "Most common failure is tests that pass with empty stubs"

### Agent Prompt Patterns

Agent prompts use `<project_context>` blocks with template variables:

```markdown
<project_context>
Language: {{LANGUAGE}}
Frameworks: {{FRAMEWORKS}}
{{CONSTITUTION_CONTENT}}
</project_context>
```

Variables are rendered via `ToInlineAgentDef()` in `executor/agent_loader.go` before dispatch.

### Agent Model Tiers

| Tier | Model | When to Use | Examples |
|------|-------|-------------|---------|
| Critical | opus | Quality-sensitive analysis, complex reasoning | code-reviewer, silent-failure-hunter |
| Standard | sonnet | Structured analysis with clear rubrics | pr-test-analyzer, type-design-analyzer |
| Simple | haiku | Pattern matching, low-judgment tasks | comment-analyzer, dependency-validator |

### Completion Format

```json
{"status": "complete", "summary": "Brief description", "artifact": "...content..."}
{"status": "blocked", "reason": "Why blocked and what's needed"}
{"status": "continue", "reason": "What was done and what's next"}
```

Output constrained via `--json-schema`. See `executor/phase_response.go` for per-phase schemas.

## Artifact Output

Phases that produce artifacts use `--json-schema` constrained output with an `artifact` field.

| Phase | Produces Artifact | Content |
|-------|-------------------|---------|
| spec | Yes | Technical specification |
| tiny_spec | Yes | Combined spec + TDD tests |
| research | Yes | Research findings |
| design | Yes | Design document |
| tdd_write | Yes | Test files and test plan |
| breakdown | Yes | Checkboxed implementation tasks |
| docs | Yes | Documentation summary |
| implement | No | Code changes only |
| review | No | Review findings only |
| qa | No | Manual QA results only |

Artifact content is extracted from the JSON `artifact` field by `ExtractArtifactFromOutput()`.

## Embedding & Loading Pattern

**CRITICAL**: ALL prompts MUST be loaded via `templates.Prompts.ReadFile()`. No inline prompts.

```go
//go:embed prompts/*.md
var Prompts embed.FS

// Standard loading pattern (with template execution)
tmplContent, err := templates.Prompts.ReadFile("prompts/implement.md")
if err != nil {
    return "", fmt.Errorf("read implement template: %w", err)  // NEVER return empty string
}
tmpl, err := template.New("implement").Parse(string(tmplContent))
// ... execute with data map
```

**Anti-pattern (NEVER do this):**
```go
// BAD: Inline prompt
prompt := fmt.Sprintf("Evaluate whether %s...", content)

// BAD: Silent failure
content, _ := templates.Prompts.ReadFile("prompts/foo.md")
if content == nil { return "" }  // Lost error!
```

| Package | Prompt Files |
|---------|--------------|
| `executor/conflict_resolver.go` | `conflict_resolution.md` |
| `spec/prompt.go` | `spec_session.md` |
| `plan_session/prompt.go` | `plan_session.md` |
| `planner/prompt.go` | `plan_from_spec.md` |
| `setup/prompt.go` | `setup.md` |

## Project Overrides

Projects can override prompts in `.orc/prompts/`:

```
.orc/prompts/implement.md  # Overrides default
```

Prompt service checks overrides first, falls back to embedded.

## Quality Checklist Gates

Spec phases include a quality checklist that must pass before implementation:

| Check | Requirement |
|-------|-------------|
| `all_criteria_verifiable` | Every success criterion has executable verification |
| `no_existence_only_criteria` | SC verifies behavior, not just existence |
| `p1_stories_independent` | P1 stories can ship alone |
| `scope_explicit` | In/out scope listed |
| `max_3_clarifications` | ≤3 clarifications, rest are assumptions |
| `initiative_aligned` | All initiative vision requirements captured in SC |
| `complexity_within_weight` | Scope fits weight classification (see Complexity Assessment) |

Failed checklist triggers retry with feedback.

## Complexity Assessment

Spec phases (`spec.md`, `tiny_spec.md`) include mandatory complexity assessment to catch under-weighted tasks early:

| Metric | Description |
|--------|-------------|
| `files_to_modify` | Distinct files needing changes |
| `modules_affected` | Top-level packages/directories touched |
| `integration_points` | Places where new code connects to existing paths |
| `data_model_changes` | True if adding/modifying schema, protos, or domain types |
| `cross_cutting_concerns` | True if changes span multiple layers (UI + API + storage) |

**Thresholds by weight:**

| Weight | Max Files | Max Modules | Max Integration | Data Model | Cross-cutting |
|--------|-----------|-------------|-----------------|------------|---------------|
| trivial | 1 | 1 | 0 | No | No |
| small | 3 | 2 | 1 | No | No |
| medium | 7 | 3 | 3 | Limited | Limited |
| large | 15 | 5 | 5 | Yes | Yes |

When complexity exceeds weight, spec outputs `{"status": "blocked"}` with guidance to split the task or re-run with higher weight.

## No-Op Prevention

All spec/TDD/review templates include guards against hollow implementations:

| Template | Guard | Purpose |
|----------|-------|---------|
| `spec.md` | Implementation Verification Requirements | Requires concrete file paths, function signatures, observable behaviors |
| `tiny_spec.md` | Verification Requirements | Success criteria must describe observable behavior changes |
| `tdd_write.md` | Test Quality Gates | Tests must fail before implementation, not pass with empty stubs |
| `review.md` | Reviewer 6: No-Op Detection Specialist | Verifies actual behavioral changes, flags pass-through implementations |
| `review.md` | No-Op Detection Checklist | Blockers for: unused functions, ignored params, dead columns, vacuous tests |

## Review Conditionals

Templates support round-specific content:

```markdown
{{#if REVIEW_ROUND_1}}
Content for Round 1 only
{{/if}}

{{#if REVIEW_ROUND_2}}
Content for Round 2 only
{{/if}}
```
