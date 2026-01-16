# Bulk Task Creation for Initiatives with Inline Specs

**Status**: Proposed
**Task**: TASK-245
**Category**: Feature

## Problem Statement

When creating multiple tasks for an initiative, users must either create tasks one-by-one with `orc new` or place spec files in a directory and use `orc plan --from`. There's no way to define a cohesive set of tasks with their specs in a single, self-contained manifest file that can be version-controlled and reviewed as a unit.

## User Story

As a developer planning an initiative, I want to define multiple tasks with their specifications in a single manifest file, so that I can:
- Plan and review all related tasks together before creation
- Version control the entire task breakdown as one artifact
- Create all tasks in one operation with proper dependencies
- Skip the AI-driven spec phase for tasks that already have specifications

## Success Criteria

- [ ] New CLI command `orc initiative plan <file>` accepts a manifest YAML file
- [ ] Manifest format supports multiple tasks with inline specs
- [ ] Manifest supports task dependencies (within the manifest)
- [ ] Manifest supports task weights, categories, and priorities
- [ ] Created tasks are automatically linked to the specified initiative
- [ ] Created tasks with inline specs skip the `spec` phase (spec stored in database)
- [ ] Dry-run mode (`--dry-run`) previews tasks without creating them
- [ ] Confirmation prompt before creation (skip with `--yes`)
- [ ] Error if initiative doesn't exist (unless `--create-initiative` flag)
- [ ] Validation errors reported for malformed manifests before creation
- [ ] Tasks created in dependency order (so blocked_by references resolve correctly)

## Testing Requirements

- [ ] Unit test: Manifest parsing validates required fields
- [ ] Unit test: Manifest parsing rejects circular dependencies
- [ ] Unit test: Manifest parsing validates weight/category/priority values
- [ ] Integration test: Tasks created from manifest have specs in database
- [ ] Integration test: Tasks linked to initiative correctly
- [ ] Integration test: Task dependencies resolve to correct TASK-IDs
- [ ] E2E test: Full workflow from manifest file to runnable tasks

## Scope

### In Scope
- CLI command for bulk task creation from manifest
- YAML manifest format definition
- Task creation with inline specs
- Initiative linking
- Dependency mapping between manifest tasks
- Dry-run preview mode
- Optional initiative creation

### Out of Scope
- Web UI for manifest creation (future enhancement)
- Import from external systems (Jira, Linear, etc.)
- Manifest generation from AI (existing `orc plan` handles this)
- Editing existing tasks via manifest (this is create-only)

## Technical Approach

### Manifest Format

```yaml
# initiative-tasks.yaml
version: 1
initiative: INIT-001           # Required: target initiative ID
# OR
create_initiative:             # Optional: create new initiative
  title: "User Authentication"
  vision: "OAuth2 support for Google and GitHub"

tasks:
  - id: 1                      # Local ID for dependency references
    title: "Add OAuth2 configuration"
    weight: small
    category: feature
    priority: normal
    description: |
      Add configuration structure for OAuth2 providers.
    spec: |
      # Specification: Add OAuth2 configuration

      ## Problem Statement
      Need configuration structure for OAuth2 providers.

      ## Success Criteria
      - [ ] Config struct for OAuth2 settings
      - [ ] Environment variable support
      - [ ] Validation on startup

      ## Technical Approach
      Add to internal/config/auth.go

  - id: 2
    title: "Implement Google OAuth2"
    weight: medium
    depends_on: [1]            # References local ID above
    spec: |
      # Specification: Implement Google OAuth2
      ...

  - id: 3
    title: "Implement GitHub OAuth2"
    weight: medium
    depends_on: [1]
    spec: |
      ...

  - id: 4
    title: "Add auth middleware"
    weight: small
    depends_on: [2, 3]         # Can depend on multiple
    # No spec = will run spec phase during execution
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/cli/cmd_initiative.go` | Add `plan` subcommand |
| `internal/cli/cmd_initiative_plan.go` (new) | Implement `initiative plan` command |
| `internal/initiative/manifest.go` (new) | Manifest parsing and validation |
| `internal/initiative/manifest_test.go` (new) | Manifest parsing tests |
| `docs/specs/FILE_FORMATS.md` | Document manifest format |

### Implementation Steps

1. **Define manifest struct** (`internal/initiative/manifest.go`)
   - `Manifest` struct with version, initiative, tasks
   - `ManifestTask` struct with id, title, weight, spec, depends_on
   - Parse and validate function

2. **Add validation**
   - Required fields (title, at least one task)
   - Valid weight/category/priority values
   - No circular dependencies
   - No duplicate local IDs
   - Initiative exists (or create_initiative provided)

3. **Implement CLI command**
   - `orc initiative plan <file>`
   - `--dry-run` flag for preview
   - `--yes` flag to skip confirmation
   - `--create-initiative` to auto-create missing initiative

4. **Task creation logic**
   - Parse manifest
   - Create tasks in topological order (dependencies first)
   - Map local IDs to TASK-IDs as created
   - Store inline specs in database
   - Link tasks to initiative

5. **Add tests**
   - Unit tests for manifest parsing
   - Integration tests for task creation
   - E2E test for full workflow

## Acceptance Criteria Details

### Manifest Validation

- Invalid YAML returns parse error with line number
- Missing required fields (title) returns specific error
- Invalid enum values (weight, category) list valid options
- Circular dependencies detected and rejected

### Task Creation

- Tasks created in correct order for dependency resolution
- Inline specs stored in database `specs` table
- Tasks with specs have their plan skip the `spec` phase
- Tasks without specs will run normally (including spec phase)

### Initiative Handling

- Existing initiative: tasks added to it
- `--create-initiative`: creates new initiative if not exists
- Missing initiative without flag: error with suggestion

### Output

- Dry-run shows tasks that would be created
- Creation shows each task ID as created
- Summary shows total tasks created and initiative

## Example Workflow

```bash
# Create manifest
cat > auth-tasks.yaml << 'EOF'
version: 1
create_initiative:
  title: "User Authentication"
  vision: "OAuth2 support"
tasks:
  - id: 1
    title: "Add OAuth config"
    weight: small
    spec: |
      # Specification: Add OAuth config
      ## Success Criteria
      - [ ] Config struct exists
EOF

# Preview
orc initiative plan auth-tasks.yaml --dry-run

# Create
orc initiative plan auth-tasks.yaml

# Output:
# Created initiative: INIT-003
# Created task: TASK-045 - Add OAuth config [small]
#   Spec: stored (will skip spec phase)
#
# Summary: 1 task(s) created in INIT-003
```

## Alternatives Considered

1. **Extend `orc plan --from`** - Could add manifest support to existing command, but semantics differ (AI analysis vs user-provided specs)

2. **Add `--spec` to `orc new`** - Would require multiple commands and manual dependency wiring

3. **JSON format** - YAML is more readable for inline specs with multi-line content

## Migration Notes

This is a new feature with no migration required. Existing workflows (`orc plan --from`, `orc new`) continue to work unchanged.
