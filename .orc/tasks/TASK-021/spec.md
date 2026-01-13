# TASK-021: Auto-detect Frontend Projects and Require Playwright Testing

## Overview

Enhance project detection to identify frontend projects and flag tasks that need UI testing.

## Detection Signals

### Project-Level (during `orc init`)

| Signal | Confidence |
|--------|------------|
| `web/`, `frontend/`, `client/` directories | High |
| `package.json` with react/vue/svelte/angular | High |
| `src/components/`, `src/pages/` | Medium |
| `.svelte`, `.vue`, `.tsx` files | Medium |
| `vite.config.*`, `next.config.*` | High |

### Task-Level (during `orc new`)

Scan task title/description for UI keywords:
- `button`, `form`, `page`, `modal`, `dialog`
- `UI`, `frontend`, `component`, `view`
- `click`, `input`, `submit`, `display`
- `layout`, `style`, `CSS`, `responsive`

## Storage

### Project Detection (existing)

```yaml
# .orc/detection.yaml (or SQLite)
has_frontend: true
frontend_framework: svelte
frontend_dir: web/
```

### Task Metadata

```yaml
# .orc/tasks/TASK-XXX/task.yaml
id: TASK-XXX
title: Add user profile page
testing_requirements:
  unit: true
  integration: true
  e2e: true           # Auto-set if has_frontend && UI keywords
  visual: true        # Screenshots required
ui_affected: true     # Explicit flag
```

## Implementation

### 1. Enhance `internal/detect/detector.go`

```go
type Detection struct {
    // ... existing ...
    HasFrontend      bool   `yaml:"has_frontend"`
    FrontendFramework string `yaml:"frontend_framework"`
    FrontendDir      string `yaml:"frontend_dir"`
}

func (d *Detector) DetectFrontend() *FrontendInfo
```

### 2. Enhance `internal/task/task.go`

```go
type TestingRequirements struct {
    Unit        bool `yaml:"unit"`
    Integration bool `yaml:"integration"`
    E2E         bool `yaml:"e2e"`
    Visual      bool `yaml:"visual"`
}

type Task struct {
    // ... existing ...
    TestingRequirements TestingRequirements `yaml:"testing_requirements,omitempty"`
    UIAffected          bool                `yaml:"ui_affected,omitempty"`
}
```

### 3. Enhance `orc new` Command

In task creation flow:
1. Load project detection
2. If `has_frontend` && title/description has UI keywords:
   - Set `ui_affected: true`
   - Set `testing_requirements.e2e: true`
   - Set `testing_requirements.visual: true`
3. Log: "Task flagged for UI testing based on frontend project + keywords"

### 4. Update `orc init` Recommendation

Already partially done - ensure Playwright MCP plugin is recommended.

## UI Keywords Regex

```go
var uiKeywords = regexp.MustCompile(`(?i)\b(button|form|page|modal|dialog|ui|frontend|component|view|click|input|submit|display|layout|style|css|responsive|menu|nav|header|footer|sidebar|card|table|list|grid)\b`)
```

## Testing

- Unit test keyword detection
- Test detection for various project structures
- E2E test task creation with UI flagging
