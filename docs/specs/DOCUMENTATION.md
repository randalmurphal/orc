# Documentation Specification

**Purpose**: Define how orc creates and maintains documentation for managed projects.

---

## Core Principles

1. **Docs follow implementation** - Agent creates/updates docs after code, with full context
2. **Structure before content** - Ensure proper doc structure exists, then populate
3. **AI-first hierarchy** - CLAUDE.md files for agents, README.md for humans
4. **Project-aware** - Different doc requirements for different project types
5. **Incremental improvement** - Create missing docs, don't just update existing

---

## Project Type Detection

Orc detects project type to determine documentation requirements:

| Signal | Project Type |
|--------|--------------|
| No existing code, new `.orc/` | **Greenfield** |
| Multiple `go.mod`/`package.json` in subdirs | **Monorepo** |
| Single `go.mod`/`package.json` at root | **Single Package** |
| Existing comprehensive docs | **Mature Project** |
| Minimal/no docs | **Undocumented** |

```yaml
# Detected in research phase, stored in task.yaml
project:
  type: monorepo
  language: go
  doc_status: partial  # none | partial | comprehensive
  existing_docs:
    - README.md
    - docs/architecture/
  missing_docs:
    - CLAUDE.md
    - CONTRIBUTING.md
```

---

## Required Documentation by Project Type

### Greenfield Projects

| Doc | Required | Purpose |
|-----|----------|---------|
| `README.md` | Yes | Project overview, quick start, usage |
| `CLAUDE.md` | Yes | AI agent instructions, codebase overview |
| `docs/architecture/OVERVIEW.md` | Yes | System design, component relationships |
| `docs/architecture/DECISIONS.md` | Yes | Key architectural decisions (or ADR format) |
| `CONTRIBUTING.md` | Recommended | How to contribute, code standards |
| `CHANGELOG.md` | Recommended | Version history |

### Monorepo Projects

| Doc | Required | Purpose |
|-----|----------|---------|
| Root `README.md` | Yes | Monorepo overview, package index |
| Root `CLAUDE.md` | Yes | Monorepo navigation, cross-cutting concerns |
| `packages/*/README.md` | Yes | Per-package documentation |
| `packages/*/CLAUDE.md` | Recommended | Per-package AI instructions |
| `docs/architecture/` | Yes | System-wide architecture |
| `docs/guides/` | Recommended | Cross-package workflows |

### Single Package Projects

| Doc | Required | Purpose |
|-----|----------|---------|
| `README.md` | Yes | Package overview, installation, usage |
| `CLAUDE.md` | Yes | AI agent instructions |
| `docs/` or inline | Optional | API docs, guides |

### Existing Undocumented Projects

Priority order for creating missing docs:
1. `CLAUDE.md` - Enables AI to work effectively
2. `README.md` - Basic project understanding
3. Architecture docs - If complex enough
4. API docs - If public interfaces exist

---

## CLAUDE.md Specification

CLAUDE.md files are optimized for AI agents. They should be:
- **Concise** - 100-200 lines max per file
- **Structured** - Tables over prose, clear headings
- **Actionable** - Commands, patterns, not philosophy
- **Hierarchical** - Root CLAUDE.md links to subdirectory ones

### Required Sections

```markdown
# Project/Package Name

## Quick Start
[3-5 commands to build/test/run]

## Structure
[Table: path → purpose]

## Key Patterns
[Code patterns used in this codebase]

## Commands
[Table: command → what it does]

## Dependencies
[External deps and their purpose]

## Docs Reference
[Links to detailed documentation]
```

### CLAUDE.md Hierarchy

```
project/
├── CLAUDE.md              # Root: overview, navigation
├── src/
│   ├── CLAUDE.md          # Source-level patterns (optional)
│   ├── auth/
│   │   └── CLAUDE.md      # Auth module specifics (if complex)
│   └── api/
│       └── CLAUDE.md      # API module specifics (if complex)
└── docs/
    └── CLAUDE.md          # Docs navigation (optional)
```

**Rule**: Create CLAUDE.md at a directory level only if:
- It's the root (always)
- The directory has >10 files or significant complexity
- There are non-obvious patterns specific to that directory

---

## Documentation Phase

### When Docs Phase Runs

| Task Weight | Docs Phase |
|-------------|------------|
| trivial | Skip (unless missing CLAUDE.md) |
| small | Minimal (update affected README sections) |
| medium | Standard (update affected docs, create missing) |
| large | Comprehensive (full doc audit and update) |
| greenfield | Full (create all required docs) |

### Docs Phase Workflow

```
1. AUDIT
   - Scan for existing docs in affected directories
   - Check CLAUDE.md files for accuracy
   - Identify missing required docs

2. CREATE (if missing)
   - Generate missing required docs from templates
   - Populate with content from implementation context

3. UPDATE (if exists)
   - Update sections affected by code changes
   - Ensure examples still work
   - Update command references

4. VALIDATE
   - All code blocks are syntactically valid
   - All file references exist
   - No TODO/FIXME in docs
   - CLAUDE.md under 200 lines
```

### Docs Phase Prompt Context

The docs phase receives full context from implementation:
- Files changed (paths and summaries)
- New public APIs/exports
- Changed behavior
- New dependencies
- Architecture decisions made

---

## Doc Templates

### README.md Template

```markdown
# {Project Name}

{One-line description}

## Features

- Feature 1
- Feature 2

## Quick Start

\`\`\`bash
# Install
{install command}

# Run
{run command}
\`\`\`

## Usage

{Basic usage examples}

## Documentation

- [Architecture](docs/architecture/)
- [API Reference](docs/api/)
- [Contributing](CONTRIBUTING.md)

## License

{License}
```

### CLAUDE.md Template

```markdown
# {Project/Package Name}

## Quick Start

\`\`\`bash
{build command}
{test command}
{run command}
\`\`\`

## Structure

| Path | Purpose |
|------|---------|
| `src/` | Source code |
| `tests/` | Test files |

## Key Patterns

{Pattern 1}: {Brief description}
\`\`\`{lang}
{code example}
\`\`\`

## Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build the project |
| `make test` | Run tests |

## Dependencies

| Dependency | Purpose |
|------------|---------|
| {dep1} | {why} |

## Docs Reference

| Topic | Path |
|-------|------|
| Architecture | `docs/architecture/` |
```

---

## Doc Validation Criteria

The validate phase checks documentation:

| Check | Criteria |
|-------|----------|
| CLAUDE.md exists | Root CLAUDE.md present |
| CLAUDE.md current | Matches actual structure |
| CLAUDE.md concise | Under 200 lines |
| README.md exists | Root README.md present |
| README.md complete | Has quick start, usage sections |
| Code blocks valid | Syntax highlighting works |
| Links valid | Internal doc links resolve |
| Examples work | Code examples are runnable |

---

## Configuration

Projects can customize doc requirements in `orc.yaml`:

```yaml
documentation:
  # Override project type detection
  type: monorepo

  # Required docs (beyond defaults)
  required:
    - CONTRIBUTING.md
    - SECURITY.md

  # Skip certain docs
  skip:
    - CHANGELOG.md

  # CLAUDE.md settings
  claude_md:
    max_lines: 150
    required_sections:
      - Quick Start
      - Structure
      - Commands

  # Custom templates
  templates:
    readme: .orc/templates/README.md.tmpl
    claude: .orc/templates/CLAUDE.md.tmpl
```

---

## Integration with Phases

### Research Phase
- Detect project type
- Inventory existing documentation
- Note doc gaps in research.md

### Implementation Phase
- Focus on code (no doc writing)
- Track changes for doc phase context

### Review Phase
- Check if implementation has doc implications
- Flag API changes that need doc updates

### Docs Phase (NEW - after review, before test)
- Create missing required docs
- Update affected existing docs
- Ensure CLAUDE.md accuracy

### Validate Phase
- Run doc validation checks
- Verify examples work
- Check link integrity

---

## Example: Task Creates Missing Docs

```yaml
# Task: Add user authentication
# Project: Undocumented Go API

# Research phase detects:
project:
  type: single_package
  doc_status: none
  missing_docs:
    - README.md
    - CLAUDE.md

# After implementation, docs phase:
# 1. Creates README.md from template + implementation context
# 2. Creates CLAUDE.md with:
#    - Project structure (from file analysis)
#    - Commands (from Makefile/scripts)
#    - Key patterns (from code analysis)
#    - New auth module documentation
# 3. Both docs include the new auth feature

# Result: Project goes from undocumented to properly documented
```
