# Cross-Project Resources

**Status**: Partially Superseded
**Priority**: P2
**Last Updated**: 2026-01-31

> **Note**: The Skills and Hooks sections of this spec are superseded by TASK-668, which implemented GlobalDB CRUD for both resource types. Skills and hooks are now stored in `hook_scripts` and `skills` tables in GlobalDB with full CRUD via ConfigService RPC. The filesystem-based resolution described below no longer applies to these resource types.

---

## Problem Statement

Users with multiple projects:
- Duplicate skills/templates across projects
- Can't share proven patterns
- No global defaults that apply everywhere
- Configuration repeated in each project

---

## Solution: Hierarchical Resource System

Resources exist at three levels:
1. **Global** (`~/.orc/`) - Shared across all projects
2. **User** (`~/`) - User-specific defaults
3. **Project** (`.orc/`) - Project-specific overrides

Higher specificity wins (project > user > global).

---

## Resource Types

### 1. Skills

```
~/.orc/skills/                    # Global skills (available everywhere)
├── go-testing/
│   └── SKILL.md
└── code-review/
    └── SKILL.md

.claude/skills/                   # Project skills (override or extend)
└── project-conventions/
    └── SKILL.md
```

### 2. Templates

```
~/.orc/templates/                 # Global templates
├── bugfix/
│   ├── template.yaml
│   └── implement.md
└── feature/
    └── template.yaml

.orc/templates/                   # Project templates
└── api-endpoint/
    └── template.yaml
```

### 3. Prompts

```
~/.orc/prompts/                   # Global prompt overrides
├── implement.md
└── test.md

.orc/prompts/                     # Project prompt overrides
└── implement.md                  # Overrides global
```

### 4. Scripts

```
~/.orc/scripts/                   # Global scripts
├── analyze-deps
└── security-check

.claude/scripts/                  # Project scripts
└── run-tests
```

### 5. Configuration

```yaml
# ~/.orc/config.yaml - Global defaults
profile: auto
model: claude-opus-4-5-20251101
completion:
  action: pr

# .orc/config.yaml - Project overrides
profile: safe                      # Override for this project
model: claude-sonnet              # Use faster model here
```

---

## Resolution Order

For any resource, orc checks in order:

1. **Project** (`.orc/` or `.claude/`)
2. **Global** (`~/.orc/`)
3. **Built-in** (embedded in binary)

First match wins.

### Example: Skill Resolution

```go
func ResolveSkill(name string) (*Skill, error) {
    // 1. Check project skills
    projectPath := filepath.Join(".claude/skills", name, "SKILL.md")
    if exists(projectPath) {
        return loadSkill(projectPath)
    }

    // 2. Check global skills
    globalPath := filepath.Join(os.Getenv("HOME"), ".orc/skills", name, "SKILL.md")
    if exists(globalPath) {
        return loadSkill(globalPath)
    }

    // 3. Check built-in
    return loadBuiltinSkill(name)
}
```

### Example: Prompt Resolution

```go
func ResolvePrompt(phase string) (string, Source) {
    // 1. Project override
    projectPath := filepath.Join(".orc/prompts", phase+".md")
    if exists(projectPath) {
        return readFile(projectPath), SourceProject
    }

    // 2. Global override
    globalPath := filepath.Join(os.Getenv("HOME"), ".orc/prompts", phase+".md")
    if exists(globalPath) {
        return readFile(globalPath), SourceGlobal
    }

    // 3. Built-in
    return embeddedPrompts.ReadFile(phase + ".md"), SourceBuiltin
}
```

---

## Configuration Merging

Configs are merged, not replaced:

```yaml
# ~/.orc/config.yaml (global)
profile: auto
model: claude-opus-4-5-20251101
gates:
  default_type: auto
retry:
  enabled: true
  max_retries: 2

# .orc/config.yaml (project)
profile: safe          # Overrides
gates:
  phase_overrides:
    merge: human       # Extends (merged with global gates)
# model: not specified, inherits claude-opus-4-5-20251101
# retry: not specified, inherits global

# Effective config:
# profile: safe
# model: claude-opus-4-5-20251101
# gates:
#   default_type: auto
#   phase_overrides:
#     merge: human
# retry:
#   enabled: true
#   max_retries: 2
```

### Merge Logic

```go
func MergeConfigs(global, project *Config) *Config {
    result := *global // Start with global

    // Override top-level fields if set in project
    if project.Profile != "" {
        result.Profile = project.Profile
    }
    if project.Model != "" {
        result.Model = project.Model
    }

    // Merge nested structs
    result.Gates = mergeGates(global.Gates, project.Gates)
    result.Retry = mergeRetry(global.Retry, project.Retry)
    result.Completion = mergeCompletion(global.Completion, project.Completion)

    return &result
}
```

---

## CLI Commands

### Managing Global Resources

```bash
# List all skills (shows source)
$ orc skill list
NAME               SOURCE    DESCRIPTION
────               ──────    ───────────
go-testing         global    Go test patterns
code-review        global    Code review standards
project-style      project   Project-specific style

# Copy skill to global
$ orc skill promote project-style
Copied project-style to ~/.orc/skills/

# Copy template to global
$ orc template promote api-endpoint --global
Copied api-endpoint to ~/.orc/templates/

# List prompts with source
$ orc prompt list
PHASE       SOURCE    PATH
─────       ──────    ────
implement   project   .orc/prompts/implement.md
test        global    ~/.orc/prompts/test.md
spec        builtin   (embedded)
```

### Global Config

```bash
# Set global defaults
$ orc config --global profile auto
$ orc config --global model claude-opus-4-5-20251101

# View effective config
$ orc config --show-merged

# View config sources
$ orc config --show-sources
profile: safe (project)
model: claude-opus-4-5-20251101 (global)
gates.default_type: auto (global)
gates.phase_overrides.merge: human (project)
```

---

## Directory Structure

### Global Directory

```
~/.orc/
├── config.yaml           # Global defaults
├── projects.yaml         # Project registry
├── pricing.yaml          # Model pricing
├── usage.yaml            # Historical usage
├── skills/               # Global skills
│   ├── go-testing/
│   └── code-review/
├── templates/            # Global templates
│   ├── bugfix/
│   └── feature/
├── prompts/              # Global prompt overrides
│   ├── implement.md
│   └── test.md
└── scripts/              # Global scripts
    └── security-check
```

### Project Directory

```
project/
├── .orc/
│   ├── config.yaml       # Project config (merges with global)
│   ├── tasks/            # Task storage
│   ├── templates/        # Project templates
│   └── prompts/          # Project prompt overrides
├── .claude/
│   ├── settings.json     # Claude Code settings
│   ├── skills/           # Project skills
│   └── scripts/          # Project scripts
└── CLAUDE.md             # Project instructions
```

---

## Resource Sharing

### Export Resources

```bash
# Export skill as tarball
$ orc skill export go-testing -o go-testing.tar.gz

# Export template
$ orc template export bugfix -o bugfix.tar.gz

# Export full configuration
$ orc config export -o my-config.tar.gz
Exported:
  - config.yaml
  - skills/ (3 skills)
  - templates/ (2 templates)
  - prompts/ (1 override)
```

### Import Resources

```bash
# Import skill from file
$ orc skill import go-testing.tar.gz --global

# Import from URL
$ orc skill import https://example.com/skills/api-design.tar.gz

# Import configuration bundle
$ orc config import my-config.tar.gz
```

### Sync Across Machines

```bash
# Initialize from remote
$ orc init --from https://github.com/user/orc-config

# Or clone config repo
$ git clone https://github.com/user/orc-config ~/.orc

# Then symlink or copy to new machine
```

---

## API Endpoints

### List Resources with Sources

```
GET /api/skills
{
  "skills": [
    {"name": "go-testing", "source": "global", "path": "~/.orc/skills/go-testing"},
    {"name": "project-style", "source": "project", "path": ".claude/skills/project-style"}
  ]
}

GET /api/templates
{
  "templates": [
    {"name": "bugfix", "source": "global"},
    {"name": "api-endpoint", "source": "project"}
  ]
}

GET /api/config?show_sources=true
{
  "config": { ... },
  "sources": {
    "profile": "project",
    "model": "global",
    "gates.default_type": "global"
  }
}
```

### Promote to Global

```
POST /api/skills/:name/promote
POST /api/templates/:name/promote
```

---

## Web UI

### Resource List with Sources

```
┌─ Skills ────────────────────────────────────────────────────┐
│                                                             │
│ ┌─ Global ────────────────────────────────────────────────┐│
│ │ go-testing         Go test patterns and table tests     ││
│ │ code-review        Code review standards                ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
│ ┌─ Project ───────────────────────────────────────────────┐│
│ │ project-style      Project-specific code style          ││
│ │                                        [↑ Promote]      ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Config Source Indicators

```
┌─ Configuration ─────────────────────────────────────────────┐
│                                                             │
│ Profile:  safe            [project]                         │
│ Model:    claude-opus-4-5 [global]                          │
│                                                             │
│ Gates:                                                      │
│   Default: auto           [global]                          │
│   Merge:   human          [project]                         │
│                                                             │
│ [Edit Project Config]  [Edit Global Config]                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for resource resolution code
- 100% coverage for config merge logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestResolveSkill_ProjectWins` | Project skill overrides global |
| `TestResolveSkill_GlobalFallback` | Falls back to global when no project |
| `TestResolveSkill_BuiltinFallback` | Falls back to builtin when neither exists |
| `TestResolvePrompt_AllLevels` | Same resolution for prompts |
| `TestMergeConfigs_TopLevel` | Project profile overrides global |
| `TestMergeConfigs_Nested` | Nested structs merge correctly |
| `TestMergeConfigs_NilHandling` | Nil project fields don't clobber global |
| `TestMergeConfigs_DeepMerge` | gates.phase_overrides merges not replaces |
| `TestResourceSource_Tracking` | Source field correctly identifies origin |
| `TestExportSkill_Tarball` | Creates valid tarball with SKILL.md |
| `TestImportSkill_Tarball` | Extracts to correct location |
| `TestExportTemplate_Tarball` | Templates export correctly |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestGlobalDirCreation` | ~/.orc/ created on first run |
| `TestProjectRegistry_AddRemove` | Projects added/removed from registry |
| `TestSkillPromote_CopiesCorrectly` | Promote copies all files |
| `TestConfigMerge_FromFiles` | Actual YAML files merge correctly |
| `TestCrossProjectSkillAccess` | Task in project A uses global skill |
| `TestCLISkillList` | `orc skill list` shows sources |
| `TestCLISkillPromote` | `orc skill promote` works |
| `TestCLIConfigShowMerged` | `orc config --show-merged` |
| `TestCLIConfigShowSources` | `orc config --show-sources` |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_skill_list_shows_sources` | `browser_navigate`, `browser_snapshot` | Skills page shows source badges |
| `test_config_sources_visible` | `browser_snapshot` | Config shows where values come from |
| `test_promote_skill_button` | `browser_click` | Promote button visible for project skills |
| `test_promote_skill_action` | `browser_click`, `browser_snapshot` | Skill moves to global |
| `test_global_skill_badge` | `browser_snapshot` | Global skills have "Global" badge |
| `test_project_skill_badge` | `browser_snapshot` | Project skills have "Project" badge |
| `test_edit_global_config` | `browser_click`, `browser_type` | Can edit ~/.orc/ config |
| `test_edit_project_config` | `browser_click`, `browser_type` | Can edit .orc/ config |
| `test_import_export_roundtrip` | API calls | Export then import produces same resource |

### CLI Tests

| Test | Description |
|------|-------------|
| `test_skill_list_output` | NAME/SOURCE/DESCRIPTION columns |
| `test_skill_promote_command` | Copies to ~/.orc/skills/ |
| `test_template_promote_command` | Copies to ~/.orc/templates/ |
| `test_config_merge_display` | Shows merged config |
| `test_config_sources_display` | Shows origin of each value |

### Test Fixtures
- Sample global ~/.orc/ directory structure
- Sample project .orc/ with overrides
- Skills at different levels (project, global, builtin)
- Conflicting config values for merge testing

---

## Success Criteria

- [ ] Global skills available in all projects
- [ ] Project skills override global
- [ ] Config merges correctly
- [ ] CLI shows resource sources
- [ ] Promote command works
- [ ] Export/import works
- [ ] Web UI shows sources
- [ ] Built-in resources work as fallback
- [ ] No conflicts between levels
- [ ] 80%+ test coverage on resource code
- [ ] All E2E tests pass
