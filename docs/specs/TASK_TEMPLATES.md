# Task Templates

**Status**: Planning
**Priority**: P1
**Last Updated**: 2026-01-10

---

## Problem Statement

Users frequently create similar tasks:
- Bug fixes follow similar patterns
- Feature work has consistent structure
- Refactoring tasks need similar preparation

Without templates:
- Repetitive task setup
- Inconsistent task quality
- Lost knowledge from successful tasks

---

## Solution: Reusable Task Templates

Save successful task patterns as templates:
- Capture weight, prompts, context files
- Create new tasks from templates
- Share templates across projects

---

## User Experience

### Saving a Template

```bash
# After completing a successful task
$ orc template save TASK-001 --name bugfix

Template saved: bugfix

Captured:
  • Weight: small
  • Phases: implement → test
  • Custom prompts: implement, test
  • Success criteria: tests pass, no regressions

Use with:
  orc new --template bugfix "Fix the auth timeout bug"
```

### Creating from Template

```bash
$ orc new --template bugfix "Fix the database connection leak"

Using template: bugfix
Weight: small
Phases: implement → test

Task created: TASK-005
Run: orc run TASK-005
```

### Listing Templates

```bash
$ orc template list

NAME         WEIGHT    PHASES              SCOPE    DESCRIPTION
────         ──────    ──────              ─────    ───────────
bugfix       small     implement,test      project  Quick bug fix pattern
feature      medium    spec,impl,test      project  New feature with spec
refactor     medium    spec,impl,test      global   Code refactoring
migration    large     spec,impl,test,val  global   Database migration
```

### Template Details

```bash
$ orc template show bugfix

Template: bugfix
────────────────

Weight: small
Phases: implement → test
Scope:  project (only available in this project)

Custom Prompts:
  implement: .orc/templates/bugfix/implement.md
  test:      .orc/templates/bugfix/test.md

Variables:
  {{BUG_DESCRIPTION}}  - What the bug is
  {{AFFECTED_FILES}}   - Files to investigate

Context Files:
  - Must include reproduction steps
  - Should reference related tests

Usage:
  orc new --template bugfix "Fix the timeout bug"
  orc new --template bugfix "Fix the memory leak" -v BUG_DESCRIPTION="Memory not freed"
```

---

## Template Structure

### Storage Location

```
# Project templates
.orc/templates/
├── bugfix/
│   ├── template.yaml      # Template definition
│   ├── implement.md       # Custom implement prompt
│   └── test.md            # Custom test prompt
└── feature/
    ├── template.yaml
    ├── spec.md
    ├── implement.md
    └── test.md

# Global templates (shared across projects)
~/.orc/templates/
├── refactor/
│   └── template.yaml
└── migration/
    ├── template.yaml
    └── ...
```

### Template Definition

```yaml
# .orc/templates/bugfix/template.yaml
name: bugfix
description: Quick bug fix with tests
version: 1

weight: small

phases:
  - implement
  - test

# Variables that can be passed when using template
variables:
  - name: BUG_DESCRIPTION
    description: Description of the bug
    required: false
  - name: AFFECTED_FILES
    description: Files that might be affected
    required: false

# Custom prompts (override defaults)
prompts:
  implement: implement.md
  test: test.md

# Default values for task creation
defaults:
  branch_prefix: fix/

# Metadata
created_from: TASK-001
created_at: 2026-01-10T14:30:00Z
author: randy
```

### Custom Prompt Example

```markdown
<!-- .orc/templates/bugfix/implement.md -->
# Bug Fix Implementation

## Task
{{TASK_TITLE}}

## Bug Description
{{BUG_DESCRIPTION}}

## Affected Files
{{AFFECTED_FILES}}

## Instructions

1. **Locate the bug**
   - Search for the issue in the affected files
   - Understand the root cause

2. **Implement the fix**
   - Make minimal changes
   - Don't refactor unrelated code
   - Add comments if the fix isn't obvious

3. **Write/update tests**
   - Add a test that would have caught this bug
   - Ensure existing tests still pass

4. **Verify**
   - Run all related tests
   - Check for regressions

When done, output:
<phase_complete>true</phase_complete>
```

---

## CLI Commands

### template save

```bash
orc template save <task-id> [flags]

Flags:
  --name, -n      Template name (required)
  --description   Template description
  --global, -g    Save to global templates (~/.orc/templates/)

Examples:
  orc template save TASK-001 -n bugfix
  orc template save TASK-042 -n migration --global
```

### template list

```bash
orc template list [flags]

Flags:
  --global, -g    Show only global templates
  --local, -l     Show only local templates
  --json          Output as JSON

Examples:
  orc template list
  orc template list --global
```

### template show

```bash
orc template show <name>

Examples:
  orc template show bugfix
  orc template show migration
```

### template delete

```bash
orc template delete <name> [flags]

Flags:
  --global, -g    Delete from global templates

Examples:
  orc template delete bugfix
  orc template delete migration --global
```

### template import/export

```bash
# Export template for sharing
orc template export bugfix > bugfix-template.tar.gz

# Import template
orc template import bugfix-template.tar.gz
orc template import https://example.com/templates/migration.tar.gz
```

### Using templates with new

```bash
orc new [flags] <title>

Flags:
  --template, -t   Use template
  -v KEY=VALUE     Set template variable

Examples:
  orc new -t bugfix "Fix timeout bug"
  orc new -t bugfix "Fix memory leak" -v BUG_DESCRIPTION="Memory not freed after request"
  orc new -t feature "Add dark mode" -v FEATURE_SCOPE="UI only"
```

---

## API Endpoints

### List Templates

```
GET /api/templates

Response:
{
  "templates": [
    {
      "name": "bugfix",
      "description": "Quick bug fix with tests",
      "weight": "small",
      "phases": ["implement", "test"],
      "scope": "project",
      "variables": [
        { "name": "BUG_DESCRIPTION", "required": false }
      ]
    },
    // ...
  ]
}
```

### Get Template

```
GET /api/templates/:name

Response:
{
  "name": "bugfix",
  "description": "Quick bug fix with tests",
  "weight": "small",
  "phases": ["implement", "test"],
  "prompts": {
    "implement": "# Bug Fix Implementation\n\n...",
    "test": "# Bug Fix Testing\n\n..."
  },
  "variables": [...],
  "created_from": "TASK-001",
  "created_at": "2026-01-10T14:30:00Z"
}
```

### Create Task from Template

```
POST /api/tasks
{
  "title": "Fix timeout bug",
  "template": "bugfix",
  "variables": {
    "BUG_DESCRIPTION": "Request times out after 30s"
  }
}
```

### Save as Template

```
POST /api/templates
{
  "task_id": "TASK-001",
  "name": "bugfix",
  "description": "Quick bug fix with tests",
  "global": false
}
```

---

## Web UI

### Template Selector in New Task Modal

```
┌─ Create New Task ───────────────────────────────────────────┐
│                                                             │
│ Template: [bugfix ▼]                                        │
│                                                             │
│ ┌─ Template Info ─────────────────────────────────────────┐│
│ │ Weight: small                                           ││
│ │ Phases: implement → test                                ││
│ │ Custom prompts for bug fixing workflow                  ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
│ Title: [Fix the database connection leak              ]     │
│                                                             │
│ Template Variables:                                         │
│ BUG_DESCRIPTION: [Connection not closed after query   ]     │
│ AFFECTED_FILES:  [internal/db/connection.go           ]     │
│                                                             │
│                            [Cancel] [Create Task]           │
└─────────────────────────────────────────────────────────────┘
```

### Templates Page

```
┌─ Templates ─────────────────────────────────────────────────┐
│                                                             │
│ [+ Create Template]  [Import]                               │
│                                                             │
│ ┌─ Project Templates ─────────────────────────────────────┐│
│ │                                                         ││
│ │ bugfix                                          [small] ││
│ │ Quick bug fix with tests                                ││
│ │ implement → test                                        ││
│ │                               [View] [Edit] [Delete]    ││
│ │                                                         ││
│ │ feature                                        [medium] ││
│ │ New feature with specification                          ││
│ │ spec → implement → test                                 ││
│ │                               [View] [Edit] [Delete]    ││
│ │                                                         ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
│ ┌─ Global Templates ──────────────────────────────────────┐│
│ │                                                         ││
│ │ refactor                                       [medium] ││
│ │ Code refactoring pattern                                ││
│ │ spec → implement → test                                 ││
│ │                               [View] [Edit] [Delete]    ││
│ │                                                         ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Built-in Templates

Orc ships with these default templates:

### bugfix

```yaml
name: bugfix
weight: small
phases: [implement, test]
description: Quick bug fix with regression test
```

### feature

```yaml
name: feature
weight: medium
phases: [spec, implement, test]
description: New feature with specification phase
```

### refactor

```yaml
name: refactor
weight: medium
phases: [spec, implement, test]
description: Code refactoring with before/after comparison
```

### migration

```yaml
name: migration
weight: large
phases: [spec, implement, test, validate]
description: Database or data migration with rollback plan
```

### spike

```yaml
name: spike
weight: small
phases: [research]
description: Time-boxed investigation, no implementation
```

---

## Implementation Notes

### Template Resolution Order

1. Project templates (`.orc/templates/`)
2. Global templates (`~/.orc/templates/`)
3. Built-in templates (embedded in binary)

### Variable Substitution

Variables are substituted in:
- Task title (optional)
- Task description
- Custom prompt templates
- Success criteria

```go
func renderTemplate(content string, vars map[string]string) string {
    for key, value := range vars {
        placeholder := fmt.Sprintf("{{%s}}", key)
        content = strings.ReplaceAll(content, placeholder, value)
    }
    return content
}
```

### Saving from Task

When saving a template from a task:
1. Extract weight from task
2. Extract phases from plan
3. Copy custom prompts if any exist
4. Record success criteria
5. Optionally include context files

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for template code
- 100% coverage for variable substitution logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestRenderTemplate_SimpleVariable` | Substitutes `{{VAR}}` correctly |
| `TestRenderTemplate_MultipleVariables` | Multiple variables substitute |
| `TestRenderTemplate_MissingVariable` | Handles missing variable gracefully |
| `TestRenderTemplate_NoVariables` | Returns unchanged content |
| `TestParseTemplateYAML` | Parses template.yaml correctly |
| `TestParseTemplateYAML_InvalidYAML` | Handles parse errors |
| `TestTemplateResolutionOrder` | Project > global > built-in |
| `TestValidateTemplateName` | Alphanumeric and dashes only |
| `TestTemplateVariableValidation` | Required vs optional handling |
| `TestSaveTemplateFromTask` | Extracts weight, phases, prompts |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestCLITemplateSave` | `orc template save TASK-001 -n foo` |
| `TestCLITemplateList` | `orc template list` |
| `TestCLITemplateShow` | `orc template show foo` |
| `TestCLITemplateDelete` | `orc template delete foo` |
| `TestCLINewWithTemplate` | `orc new -t foo "title"` |
| `TestCLINewWithTemplateVariables` | `-v KEY=VALUE` substitution |
| `TestCLITemplateExport` | Exports tar.gz |
| `TestCLITemplateImport` | Imports tar.gz |
| `TestAPIListTemplates` | `GET /api/templates` |
| `TestAPIGetTemplate` | `GET /api/templates/:name` |
| `TestAPICreateTaskFromTemplate` | `POST /api/tasks` with template |
| `TestAPISaveAsTemplate` | `POST /api/templates` |
| `TestGlobalVsProjectTemplates` | `--global` flag works |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_template_dropdown_in_modal` | `browser_click`, `browser_snapshot` | New task modal shows template dropdown |
| `test_template_populates_fields` | `browser_click`, `browser_snapshot` | Selecting template shows weight/phases |
| `test_template_variables_form` | `browser_snapshot` | Variable inputs appear for template |
| `test_templates_page_list` | `browser_navigate`, `browser_snapshot` | Templates page lists all templates |
| `test_template_delete_confirmation` | `browser_click`, `browser_snapshot` | Delete requires confirmation |
| `test_create_template_from_task` | `browser_click` | Save as template from task detail |
| `test_template_info_preview` | `browser_snapshot` | Preview shows phases and description |

### Test Fixtures
- Sample template.yaml files
- Sample task for template creation
- Mock prompts for template testing

---

## Success Criteria

- [ ] `orc template save` captures task patterns
- [ ] `orc new --template` creates tasks from templates
- [ ] Templates support custom prompts
- [ ] Variables work in prompts and descriptions
- [ ] Global templates shared across projects
- [ ] Built-in templates available by default
- [ ] Web UI shows template selector
- [ ] Templates can be exported/imported
- [ ] 80%+ test coverage on template code
- [ ] All E2E tests pass
