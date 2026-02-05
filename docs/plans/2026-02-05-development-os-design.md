# Development OS: Orc as a Development Operating System

**Date:** 2026-02-05
**Status:** Design
**Scope:** Full architectural vision for transforming orc from a task orchestrator into a development operating system

---

## Vision

Orc becomes the persistent memory, execution, and adaptation layer that wraps around Claude Code. Claude Code is the brain. Orc is the nervous system, the muscle memory, and the institutional knowledge.

The collaboration model:
- **Claude/Orc = Tech Lead** — Owns technical decomposition, execution, quality. Makes technical calls autonomously.
- **User = Product/Dev Manager** — Owns vision, strategic direction, business decisions. Reviews output, course-corrects.

The system supports the full lifecycle: **Think → Plan → Execute → Learn** — and the transitions between these modes are seamless, not lossy.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   INTERFACE LAYER                        │
│  Web Dashboard (threads, diff, terminal, knowledge)     │
│  CLI (power users, scripting, CI)                       │
├─────────────────────────────────────────────────────────┤
│                 INTELLIGENCE LAYER                       │
│  Orchestrator (assess/decide/act loop)                  │
│  Planning Agent · Execution Agents · Review Agents      │
│  Learning Agent · Knowledge Curator                     │
├─────────────────────────────────────────────────────────┤
│                 ORCHESTRATION LAYER                      │
│  Workflow Engine · Phase Executor (multi-type)           │
│  Gate System · Variable Resolution · Git Integration    │
│  Team Coordination (claims, heartbeats, budgets)        │
├─────────────────────────────────────────────────────────┤
│                   KNOWLEDGE LAYER                        │
│  Code Index (AST, relationships, patterns)              │
│  Task Artifacts (specs, findings, decisions, retries)   │
│  Retrieval Pipeline (multi-signal scoring)              │
│  Neo4j + Qdrant + Redis (Docker-managed)                │
├─────────────────────────────────────────────────────────┤
│                   DATA LAYER                             │
│  SQLite (solo) / PostgreSQL (team)                      │
│  Tasks, phases, workflows, users, costs, transcripts    │
└─────────────────────────────────────────────────────────┘
```

### Feedback Loop

1. User sets vision → Planning decomposes → Execution builds
2. Execution produces artifacts → Knowledge layer indexes them
3. Orchestrator queries knowledge → Makes better decisions next time
4. Patterns surface → Orchestrator fixes the process
5. Process improves → Execution quality goes up → Fewer interventions needed

---

## Component 1: Knowledge Layer

### Overview

The knowledge layer is orc's persistent memory. It combines code indexing (ported from ai-devtools-admin and graphrag repos into `internal/knowledge/`) with task artifact indexing to build a queryable graph of everything orc knows about the codebase and its history.

All code lives in the orc repo as `internal/knowledge/`. No external Go module dependencies on graphrag or ai-devtools-admin — they are reference implementations.

### Infrastructure

Docker-managed by default. `orc init` starts containers, `orc knowledge status` shows health.

```yaml
# ~/.orc/config.yaml
knowledge:
  enabled: true
  backend: docker          # "docker" (default) | "external"
  docker:
    neo4j_port: 7687
    qdrant_port: 6334
    data_dir: ~/.orc/knowledge/
  external:
    neo4j_uri: bolt://neo4j:7687
    qdrant_uri: http://qdrant:6334
    redis_uri: redis://redis:6379
  indexing:
    auto_index_on_complete: true
    embedding_model: voyage-4       # Default: good balance (requires VOYAGE_API_KEY)
    # embedding_model: voyage-4-large  # Higher quality, higher cost
    # embedding_model: voyage-4-nano   # Local, open weights, no API key needed
    # All three use 1024 dimensions — can switch without re-indexing vectors
```

### Three Data Producers, One Graph

| Producer | What It Indexes | Source |
|----------|----------------|--------|
| **Code indexer** | Source code (AST chunks, symbols, relationships, patterns) | Ported from ai-devtools-admin |
| **Artifact indexer** | Task specs, review findings, decisions, retry context | New |
| **Retrieval pipeline** | Query → multi-signal scoring → ranked results | Ported from graphrag |

### Graph Schema

**Code nodes** (from code indexer):

| Node Type | Key Properties |
|-----------|---------------|
| `:Symbol` | name, kind, file_path, start_line, end_line, signature, docstring |
| `:File` | path, repo, hash, last_indexed |
| `:Pattern` | name, canonical_file, member_count |
| `:Module` | path, description |

**Artifact nodes** (from orc):

| Node Type | Key Properties |
|-----------|---------------|
| `:Task` | id, title, category, workflow, weight |
| `:Spec` | content_hash, task_id, summary |
| `:Finding` | severity, description, file_path, line, agent_id, task_id |
| `:Decision` | content, rationale, initiative_id |
| `:Retry` | attempt, reason, from_phase, task_id |

**Relationships:**

```
Code ↔ Code:
  CALLS, IMPORTS, EXTENDS, CONTAINS, IN_FILE, FOLLOWS_PATTERN

Artifact → Code:
  TARGETS (Spec → Symbol/File)
  ABOUT (Finding → Symbol/File)
  MODIFIES (Task → Files changed)
  AFFECTS (Decision → Symbols/Patterns)

Artifact → Artifact:
  FROM_TASK, FROM_INITIATIVE, IMPLEMENTS, CAUSED_BY, SUPERSEDES
```

### When Indexing Happens

| Trigger | What Gets Indexed |
|---------|-------------------|
| `orc init` (first time) | Full codebase |
| `orc index` (manual) | Full or incremental codebase (respects file hashes) |
| Task completion | Changed files + task artifacts (spec, findings, decisions, retries, metrics) |
| `orc learn` | Nothing new — queries existing graph for patterns |

### Graceful Degradation

When knowledge layer is unavailable (disabled, Docker not running, containers unhealthy):
- `IsAvailable()` returns false
- All query methods return empty results (not errors)
- All indexing methods are no-ops
- The rest of orc works exactly as it does today
- Knowledge phase types skip automatically

### Package Structure

```
internal/knowledge/
├── infra/              # Docker container lifecycle
│   ├── manager.go      # Start/stop/health check containers
│   └── config.go       # Connection config (docker vs external)
│
├── index/              # Indexing pipeline
│   ├── code/           # Ported from ai-devtools-admin
│   │   ├── walker.go       # File discovery (.gitignore aware)
│   │   ├── parser.go       # Tree-sitter AST extraction
│   │   ├── chunker.go      # AST-aware chunking
│   │   ├── relationships.go # Import/call/extend extraction
│   │   ├── patterns.go     # Pattern detection & clustering
│   │   └── secrets.go      # Secret detection & redaction
│   │
│   ├── artifact/       # New — orc task artifact indexing
│   │   ├── spec.go
│   │   ├── findings.go
│   │   ├── decisions.go
│   │   ├── retries.go
│   │   └── metrics.go
│   │
│   └── indexer.go      # Unified indexer (orchestrates code + artifact)
│
├── store/              # Storage backends
│   ├── graph.go        # Neo4j operations
│   ├── vector.go       # Qdrant operations
│   └── cache.go        # Redis operations
│
├── retrieve/           # Query pipeline
│   ├── pipeline.go     # Composable retrieval stages
│   ├── stages.go       # Semantic, graph expansion, temporal, rerank
│   ├── presets.go      # Standard, fast, deep, graph-first, recency
│   └── scorer.go       # Multi-signal scoring
│
├── embed/              # Embedding generation
│   ├── embedder.go     # Interface
│   ├── voyage.go       # Voyage AI
│   └── sidecar.go      # Local model fallback
│
└── knowledge.go        # Top-level Service API
```

### Top-Level API

```go
type Service struct { ... }

// Infrastructure
func (s *Service) Start(ctx context.Context) error
func (s *Service) Stop(ctx context.Context) error
func (s *Service) Status(ctx context.Context) (*Health, error)
func (s *Service) IsAvailable() bool

// Indexing
func (s *Service) IndexProject(ctx context.Context, root string, opts IndexOpts) (*IndexResult, error)
func (s *Service) IndexTaskArtifacts(ctx context.Context, task *db.Task, artifacts TaskArtifacts) error
func (s *Service) Reindex(ctx context.Context, files []string) error

// Retrieval
func (s *Service) Query(ctx context.Context, query string, opts QueryOpts) (*QueryResult, error)
func (s *Service) QueryForTask(ctx context.Context, task *db.Task) (*TaskContext, error)

// Learning
func (s *Service) GetFileMetrics(ctx context.Context, paths []string) ([]FileMetric, error)
func (s *Service) GetRecurringPatterns(ctx context.Context, since time.Time) ([]Pattern, error)
```

### TaskContext (What Phases Receive)

```go
type TaskContext struct {
    FileHistory    []FileInsight    // Per-file: past findings, retry rates, difficulty
    RelatedWork    []RelatedTask    // Past tasks that touched similar code
    Decisions      []Decision       // Initiative decisions affecting this area
    Patterns       []CodePattern    // Known patterns in the affected code
    Warnings       []Warning        // "This file has a 40% retry rate"
}
```

Rendered as markdown and injected as `{{KNOWLEDGE_CONTEXT}}` into phase prompts.

### Context Loading Strategy: Hybrid Injection + Tools

Context reaches Claude through two complementary mechanisms:

**1. Deterministic Injection (always present, high confidence)**

Based on known task properties (file paths, function names, initiative ID), the system performs direct graph lookups and injects results into the prompt automatically:

| Signal | Confidence | Strategy |
|--------|-----------|----------|
| Files mentioned in task description | Deterministic | Always inject past findings, retry history, difficulty score |
| Functions/types in those files | Graph traversal | Always inject call graph context, related changes |
| Initiative decisions | Deterministic | Always inject |
| Exact file path matches from past findings | Deterministic | Always inject high-severity findings |

This uses a token budget to prevent context bloat:

```go
type InjectionOpts struct {
    MaxTokens   int     // Budget for injected context (default: 8000)
    MinScore    float64 // Only include results above this relevance score
    MaxResults  int     // Hard cap on number of results
}
```

High-relevance results get full content. Medium-relevance results get summaries. Low-relevance results are omitted.

**2. MCP Tools (on-demand, Claude decides when to search deeper)**

The knowledge service exposes MCP tools that Claude Code can use during any phase or conversation:

```
search_knowledge(query, opts)     — Semantic search over the full graph
get_file_history(file_path)       — All findings, retries, decisions for a file
get_related_code(symbol_name)     — Call graph, importers, dependents
get_pattern_info(pattern_name)    — Pattern description, canonical example, members
```

The injection ensures Claude always has baseline context (prevents the "didn't check" problem). The tools let Claude go deeper when the conversation or implementation reveals unexpected needs.

---

## Component 2: Executor Abstraction

### Overview

Phases declare a `type` that determines which executor handles them. The existing `ClaudeExecutor` becomes one implementation among several.

### Phase Types

```go
type PhaseType string

const (
    PhaseTypeLLM        PhaseType = "llm"         // Current behavior (Claude Code CLI)
    PhaseTypeScript     PhaseType = "script"       // Run a shell script, capture output
    PhaseTypeAPI        PhaseType = "api"          // HTTP request, parse response
    PhaseTypeHuman      PhaseType = "human"        // Block and wait for human input
    PhaseTypeSubflow    PhaseType = "subflow"      // Execute another workflow
    PhaseTypeKnowledge  PhaseType = "knowledge"    // Query/index the knowledge graph
)
```

### Executor Interface

```go
type PhaseExecutor interface {
    Execute(ctx context.Context, phase *WorkflowPhase, vars map[string]string) (*PhaseResult, error)
}
```

A registry maps `PhaseType` → `PhaseExecutor`. The workflow executor loop stays the same — it dispatches to the right executor based on phase type.

### What Stays the Same

- Variable resolution works for all phase types
- Gates work for all phase types
- Loops work for all phase types
- Cost tracking works (each type reports its own cost model)
- Transcripts work (each type produces storable output)
- Visual workflow editor works (nodes get a type badge and type-specific inspector)

### Workflow Definition Examples

```yaml
phases:
  # Knowledge query (gather context before LLM phase)
  - id: gather-context
    type: knowledge
    knowledge:
      action: query
      query: "{{TASK_DESCRIPTION}}"
      preset: standard
      output_var: KNOWLEDGE_CONTEXT
      fallback: skip

  # LLM phase (existing behavior)
  - template: implement
    type: llm
    sequence: 2

  # Script phase (run tests, deploy, custom tooling)
  - id: run-migrations
    type: script
    script:
      command: "make migrate-up"
      workdir: "{{WORKTREE_PATH}}"
      timeout: 60s
      success_pattern: "migrations applied"
    gate_type: auto

  # API phase (call external services)
  - id: notify-deploy
    type: api
    api:
      method: POST
      url: "https://deploy.internal/api/trigger"
      body: '{"branch": "{{TASK_BRANCH}}"}'
      headers:
        Authorization: "Bearer {{env.DEPLOY_TOKEN}}"
      success_status: [200, 201]

  # Subflow phase (compose workflows)
  - id: security-audit
    type: subflow
    subflow:
      workflow_id: security-review
      inherit_vars: true
```

---

## Component 3: Planning Agent

### Overview

Bridges "we decided to build X" and "here are the tasks that implement X." Planning is itself a workflow — a sequence of phases, not a single LLM call.

### Planning Workflow

```yaml
id: plan
phases:
  - id: gather-knowledge
    type: knowledge
    sequence: 0
    knowledge:
      action: query
      query: "{{INITIATIVE_VISION}}"
      preset: standard
      limit: 50
      output_var: CODEBASE_CONTEXT

  - id: analyze-scope
    type: llm
    sequence: 1
    # Outputs: affected systems, risk areas, open questions

  - id: human-decisions
    type: human
    sequence: 2
    # Present analysis and open questions to user
    # User answers get recorded as initiative decisions

  - id: decompose
    type: llm
    sequence: 3
    # Generate task manifest with tasks, deps, workflows

  - id: validate-plan
    type: llm
    sequence: 4
    gate_type: ai
    # Dependency validator checks the manifest
```

### The Human-Decisions Phase

The planning agent identifies what it doesn't know and asks. "Should we use row-level security or schema-based isolation?" Decisions get recorded on the initiative and flow into every downstream task.

### Replanning Trigger

When a completed task produces unexpected findings (review found architectural issues, implementation revealed missing dependencies), the initiative flags remaining tasks for re-evaluation. Uses the existing `on_task_completed` trigger mechanism.

---

## Component 4: Learning Loop

### Three Levels

**Level 1: Artifact Indexing (Automatic)**

Post-completion hook indexes task artifacts into the knowledge graph. No user action required.

| Artifact | What Gets Indexed |
|----------|-------------------|
| Spec | Content, target files/functions |
| Code changes | AST-chunked diffs, linked to existing code nodes |
| Review findings | Severity, description, code locations, reviewer agent |
| Retry context | Attempt number, reason, triggering phase |
| Metrics | Duration, cost, token usage per phase |

**Level 2: Context Injection (Automatic)**

Knowledge phase types query the graph before LLM phases and inject `{{KNOWLEDGE_CONTEXT}}`:

```markdown
## Relevant History

### Files You'll Likely Modify
- `internal/executor/workflow_executor.go` — Modified in 12 previous tasks.
  Retry rate: 40%. Common issue: variable resolution order.

### Related Decisions
- INIT-012: "Use schema-constrained output for all phase completions"

### Patterns
- Phase executor changes typically require corresponding test updates
```

**Level 3: Learning Reports (User-Initiated)**

```bash
orc learn                        # Analyze recent tasks
orc learn --since 2026-01-01     # Specific time range
orc learn --initiative INIT-041  # Scope to initiative
```

Generates reports on workflow effectiveness, review agent accuracy, recurring issues, and cost patterns. The user decides what's actionable.

---

## Component 5: Orchestrator

### Overview

The orchestrator is a structured, knowledge-informed loop that replaces the current skill-based tech lead approach. It runs as `orc orchestrate` and follows a repeating assess → decide → act cycle.

### Priority Stack

Every cycle, the orchestrator evaluates actions in priority order:

```
PRIORITY 1: Verify completed work
  Read diff, check success criteria, query knowledge graph
  for known issues in affected files.
  If bad → reopen or create fix task

PRIORITY 2: Fix systemic issues
  Query knowledge graph for recurring findings/retries
  If pattern found → update constitution, adjust prompts,
  change workflow assignments

PRIORITY 3: Unblock stuck work
  Check blocked tasks, pending decisions, stale claims
  Quick wins that keep momentum

PRIORITY 4: Launch new work
  Reason about impact, not FIFO:
  - Dependency chains (unblocks most downstream work)
  - Risk (knowledge graph says area is hard → heavier workflow)
  - Initiative priority
  Pre-flight: verify spec quality before launching

PRIORITY 5: Proactive improvement
  Learning report analysis, prompt refinement,
  workflow optimization
```

### Knowledge Integration

| Priority | Knowledge Query | Purpose |
|----------|----------------|---------|
| Verify | Past findings for files changed in completed task | Know what to look for |
| Systemic | Recurring findings in last N tasks | Detect patterns |
| Unblock | Why similar tasks got stuck before | Faster diagnosis |
| Launch | Difficulty profile for files task will touch | Right-size workflow |
| Improve | Review agent signal-to-noise ratios | Data-driven optimization |

### Decision Log

Each cycle produces a structured summary:

```
ORCHESTRATOR CYCLE 14

ASSESSED:
  - TASK-045 completed: Verified (diff reviewed, 4/4 criteria met)
  - TASK-046 completed: ISSUE FOUND
    → "migration reversible" criterion not met
    → Created TASK-048

SYSTEMIC:
  - Pattern: 3/5 tasks had "unwrapped errors" in gate/ package
  - Action: Updated constitution

LAUNCHED:
  - TASK-047 (upgraded small → medium, 45% retry rate on affected files)

ESCALATING:
  - TASK-049 proposes new DB table, 2 similar tables exist.
    Recommendation: consolidate into cost_log.
```

### Autonomy Configuration

```yaml
orchestrator:
  autonomy: guided    # "guided" | "autonomous" | "supervised"
  # guided: escalate constitution changes, workflow adjustments, >5 file changes
  # autonomous: only escalate strategic decisions and novel situations
  # supervised: escalate everything (early trust-building phase)
```

### Stateless by Design

The orchestrator's context comes from orc's database and the knowledge graph, not from conversation history. A new `orc orchestrate` session picks up exactly where the last one left off.

---

## Component 6: Web UI — The Development OS Dashboard

### Design Philosophy

The web UI becomes a self-contained development environment. The user never needs to leave to another tool. Inspired by OpenAI Codex app's command center pattern but differentiated by the knowledge graph, orchestrator intelligence, and decision pipeline.

All Claude interactions use the Claude Code CLI harness — discussion mode and agentic mode are the same executor with different configurations.

### Layout

```
┌──────────┬─────────────────────────────────┬──────────────────────┐
│          │                                  │                      │
│ PROJECT  │       MAIN CONTENT               │   CONTEXT PANEL      │
│ SIDEBAR  │       (current page view)        │                      │
│          │                                  │   Slides between:    │
│ Projects │                                  │   • Discussion       │
│ Threads  │                                  │   • Diff review      │
│ Agents   │                                  │   • Terminal         │
│          │                                  │   • Knowledge        │
│          │                                  │   • Task detail      │
│          │                                  │                      │
├──────────┴─────────────────────────────────┴──────────────────────┤
│  ▸ Terminal (Cmd+J)                                                │
└────────────────────────────────────────────────────────────────────┘
```

### Navigation

```
Home | Board | Knowledge | Workflows | Settings
```

### Home: Command Center

The default landing page. Situational awareness, not a task list.

**Orchestrator panel** — Latest decision log. Structured summary of what the tech lead assessed, decided, and did. Inline "Decision Needed" cards with approve/reject/discuss actions.

**Active work** — Compact cards for running and blocked tasks with phase progress bars.

**Recent** — Completed and reopened tasks, with orchestrator verification status.

### Thread Model

Every conversation is a thread. Threads are:
- **Persistent** — Stored in DB, survive browser refresh
- **Contextual** — Linked to task, initiative, file, or freeform
- **Indexable** — Decisions captured into knowledge graph
- **Resumable** — Pick up where you left off
- **Concurrent** — Multiple threads active, visible in sidebar

Threads appear in the left sidebar. Click to switch. Active threads show status (running/idle/done).

### Context Panel — Two Modes

**Discussion mode** (default):
- Claude Code CLI with focused system prompt (knowledge context, task context)
- No structured output constraint
- Streaming output via WebSocket
- "Record Decision" button captures outcomes to initiative
- Inline approve/reject for orchestrator escalations

**Agentic mode** (toggle):
- Claude Code CLI with full tool access, worktree-scoped
- File editing, test running, git operations
- Diff pane updates in real-time as agent makes changes
- Commit/PR actions available inline

Transition is seamless — discussing an approach, toggle to agentic to prototype, see diff, discuss result, record decision.

### Knowledge Page

New top-level page for exploring what orc knows:

- **Search** — Natural language queries over the entire knowledge graph
- **Results** — Code nodes, findings, decisions, patterns — all mixed and ranked
- **Insights** — Hot files (highest retry rates), recurring patterns, recent constitution updates
- **Discuss action** on every result — opens a thread with that context pre-loaded

### Changes to Existing Pages

**Board** — Gains orchestrator status indicator, "Discuss" action on task cards, knowledge-enriched tooltips.

**Task detail** — Gains knowledge context tab, "Discuss" button, orchestrator verification result.

**Workflow editor** — Gains knowledge/script/API/human/subflow node types with distinct visuals and type-specific inspector panels.

### Terminal Drawer

Cmd+J toggles a terminal scoped to current project/worktree. Knows about orc — `orc status`, `orc show TASK-049` work directly.

---

## Migration Path

Each phase delivers standalone value. If you stop after any phase, the system is still better than before.

### Phase 1: Knowledge Infrastructure

**Build:**
- `internal/knowledge/infra/` — Docker management for Neo4j, Qdrant, Redis
- `internal/knowledge/store/` — Graph, vector, cache store implementations
- `internal/knowledge/embed/` — Voyage AI + sidecar embedder
- Config: `knowledge:` section with docker/external modes
- CLI: `orc knowledge start/stop/status`
- Graceful degradation when unavailable

**Enables:** Infrastructure exists, orc can manage it. Nothing uses it yet.

### Phase 2: Code Indexing

**Build:**
- `internal/knowledge/index/code/` — Port walker, parser, chunker, relationships, patterns, secrets from ai-devtools-admin
- Go AST support (from graphrag's native implementation)
- CLI: `orc index` for manual indexing
- `orc init` integration (offer to index on first setup)

**Enables:** Codebase is indexed. Validates infrastructure works end-to-end.

### Phase 3: Retrieval Pipeline

**Build:**
- `internal/knowledge/retrieve/` — Port pipeline, stages, presets, scorer from graphrag
- `Service.Query()` and `QueryForTask()` methods
- CLI: `orc knowledge query "how does gate evaluation work?"`

**Enables:** Knowledge graph is queryable from CLI. Validates retrieval before wiring into phases.

### Phase 4: Phase Integration

**Build:**
- `PhaseExecutor` interface extracted from current `ClaudeExecutor`
- `KnowledgePhaseExecutor` for `type: knowledge` phases
- `{{KNOWLEDGE_CONTEXT}}` variable injection
- Built-in workflows updated with optional knowledge gather phases
- Post-completion hook for task artifact indexing
- `internal/knowledge/index/artifact/` — Spec, findings, decisions, retries

**Enables:** Tasks get enriched with relevant history. Artifacts indexed on completion. Feedback loop closed.

### Phase 5: Executor Abstraction

**Build:**
- `ScriptPhaseExecutor`, `APIPhaseExecutor`, `HumanPhaseExecutor`, `SubflowPhaseExecutor`
- Phase type field in workflow definitions and database
- Visual workflow editor updates for node types
- Phase inspectors per type in web UI

**Enables:** Workflows include non-LLM steps. Deploy scripts, API calls, human decisions, nested workflows.

### Phase 6: Orchestrator

**Build:**
- `orc orchestrate` command
- Assess/decide/act loop with knowledge integration
- Post-completion verification workflow
- Systemic pattern detection queries
- Priority reasoning logic
- Decision log storage
- Autonomy configuration
- Updated orc-dev skill

**Enables:** Full tech lead experience. Validates work, detects patterns, fixes processes, escalates intelligently.

### Phase 7: Planning Agent

**Build:**
- `plan` workflow (knowledge → analyze → human decisions → decompose → validate)
- Planning agent prompts
- Integration with existing `orc initiative plan` manifest system
- Replanning triggers on task completion

**Enables:** User provides vision, orc decomposes into task plan with right decisions surfaced.

### Phase 8: Web UI — Development OS Dashboard

**Build:**
- Home command center page
- Thread model with persistent sessions
- Context panel with discussion/agentic modes (both via Claude Code CLI harness)
- Knowledge page with search and insights
- Terminal drawer
- Layout refactor (project sidebar, thread list)
- Workflow editor updates (new node types)

**Enables:** Self-contained development environment. Never need to leave the browser.

### Dependency Graph

```
Phase 1 (infra) → Phase 2 (code index) → Phase 3 (retrieval)
                                                │
                                                ▼
                                          Phase 4 (phase integration)
                                                │
                                ┌───────────────┼───────────────┐
                                ▼               ▼               ▼
                          Phase 5           Phase 6         Phase 7
                       (executor)      (orchestrator)    (planning)
                                                │
                                                ▼
                                          Phase 8 (web UI)
```

Phases 5, 6, 7 are independent of each other (can be built in parallel).
Phase 8 depends on Phase 6 (orchestrator panel) but can start earlier for non-orchestrator features.

**Recommended order within 5/6/7:** Phase 6 (orchestrator) first — biggest quality gap today. Phase 7 (planning) second — highest leverage for new work. Phase 5 (executor) third — enables future growth.

---

## What Doesn't Change

| Current Feature | Status |
|----------------|--------|
| SQLite/PostgreSQL dual-mode database | Unchanged (INIT-041 continues) |
| Task model, workflows, phases | Unchanged (extended with phase types) |
| Git worktree isolation | Unchanged |
| Multi-agent review system | Unchanged |
| Gate system (auto/human/AI/skip) | Unchanged |
| Variable resolution pipeline | Unchanged (extended with knowledge vars) |
| Constitution system | Unchanged (orchestrator may suggest updates) |
| Initiative system | Unchanged (planning agent produces manifests) |
| CLI commands | Unchanged (new commands added) |
| Export/import system | Unchanged |
| Jira integration | Unchanged |

---

## Resolved Design Decisions

1. **Embedding model** — Use Voyage-4 suite exclusively. `voyage-4` as default (requires `VOYAGE_API_KEY`), `voyage-4-large` for higher quality, `voyage-4-nano` for local/offline use (open weights, no API key). All three share 1024 dimensions, so switching models doesn't require re-indexing vectors.

2. **Neo4j/Qdrant resource usage** — Accept the infrastructure weight (Neo4j ~2-4GB, Qdrant ~500MB). Document 8GB RAM minimum when knowledge layer is enabled. Consider a Qdrant-only "lite" mode as a future optimization if resource usage becomes a barrier, but don't build it upfront — the graph relationships are core to the value proposition.

3. **Incremental indexing granularity** — File-level hashing for v1 (matches ai-devtools-admin's proven approach). Chunking is fast (tree-sitter microseconds per file), embedding is the expensive step and only applies to changed chunks. Revisit chunk-level hashing only if profiling shows it's needed.

4. **Thread storage model** — Threads stored in ProjectDB (project-scoped). `threads` and `thread_messages` tables with optional foreign keys to `tasks` and `initiatives`. When a decision is recorded from a thread, it writes to `initiative_decisions` (existing) and indexes into the knowledge graph with `FROM_THREAD` relationship. Conversations are ephemeral process; decisions are persistent knowledge.

5. **Orchestrator frequency** — Three triggers: event-driven (task completes/fails/blocks — primary), periodic safety net (every 15 min while tasks are running — catches stale/stuck state), and on-demand (`orc orchestrate` or UI button). All configurable.

6. **Context loading strategy** — Hybrid injection + MCP tools. High-confidence deterministic context (file-path matches, initiative decisions, graph traversals from known files) injected automatically with a token budget (default 8K tokens). Knowledge graph also exposed as MCP tools (`search_knowledge`, `get_file_history`, `get_related_code`, `get_pattern_info`) for Claude to search deeper on demand. Injection prevents "didn't check" problem; tools enable depth when needed.
