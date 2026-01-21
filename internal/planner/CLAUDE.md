# Planner Package

Spec-to-task planning functionality. Reads specification documents and uses Claude to generate task breakdowns.

## Overview

This package handles the `orc plan` command - reading spec files, generating planning prompts, parsing Claude's response, and creating tasks with dependencies.

## Key Types

| Type | Purpose |
|------|---------|
| `Planner` | Main coordinator for planning |
| `SpecFile` | Loaded specification file |
| `SpecLoader` | Loads spec files from directory |
| `ProposedTask` | Task proposed by Claude |
| `TaskBreakdown` | Parsed task breakdown |
| `CreationResult` | Created task record |

## File Structure

| File | Purpose |
|------|---------|
| `planner.go` | Main planner logic |
| `spec_loader.go` | Load spec files from directory |
| `parser.go` | Parse XML task breakdown |
| `prompt.go` | Generate planning prompt |
| `templates/plan_from_spec.md` | Planning prompt template |

## Workflow

```
1. Load spec files (SpecLoader)
2. Generate prompt (prompt.go + template)
3. Run Claude (--print mode)
4. Parse response (parser.go)
5. Validate dependencies
6. Create tasks (task package)
7. Link to initiative (optional)
```

## Usage

```go
p := planner.New(planner.Options{
    SpecDir:  ".spec",
    WorkDir:  "/path/to/project",
    Model:    "claude-sonnet-4-20250514",
})

// Load specs
files, err := p.LoadSpecs()

// Generate prompt
prompt, err := p.GeneratePrompt(files)

// Run Claude
response, err := p.RunClaude(ctx, prompt)

// Parse and create tasks
breakdown, err := p.ParseResponse(response)
results, err := p.CreateTasks(breakdown)
```

## JSON Output Format

Claude outputs tasks as JSON:

```json
{
  "summary": "Brief analysis summary",
  "tasks": [
    {
      "id": 1,
      "title": "Task title",
      "description": "What this task does...",
      "weight": "small",
      "depends_on": []
    },
    {
      "id": 2,
      "title": "Second task",
      "description": "Depends on first task",
      "weight": "medium",
      "depends_on": [1]
    }
  ]
}
```

## Dependency Validation

- All dependencies must reference existing tasks
- No forward references (task N can only depend on tasks 1 to N-1)
- No circular dependencies

## Testing

```bash
go test ./internal/planner/... -v
```
