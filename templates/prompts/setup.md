# orc Setup Prompt Template

You are configuring a project for orc, a task orchestrator that runs Claude Code in phases (spec → implement → test → validate).

## Project Detection

| Property | Value |
|----------|-------|
| **Project** | {{.ProjectName}} |
| **Path** | {{.ProjectPath}} |
| **Language** | {{if .Language}}{{.Language}}{{else}}unknown{{end}} |
| **Size** | {{.ProjectSize}} |
{{if .Frameworks}}| **Frameworks** | {{range $i, $f := .Frameworks}}{{if $i}}, {{end}}{{$f}}{{end}} |
{{end}}{{if .BuildTools}}| **Build Tools** | {{range $i, $t := .BuildTools}}{{if $i}}, {{end}}{{$t}}{{end}} |
{{end}}{{if .HasTests}}| **Has Tests** | yes |
{{end}}{{if .TestCommand}}| **Test Command** | `{{.TestCommand}}` |
{{end}}{{if .LintCommand}}| **Lint Command** | `{{.LintCommand}}` |
{{end}}

{{if .ExistingClaudeMD}}
## Existing CLAUDE.md

```markdown
{{.ExistingClaudeMD}}
```

ADD an orc section to this file. Do not replace existing content.
{{end}}

## Your Instructions

1. **Explore the project** - Understand structure, patterns, conventions
2. **Update CLAUDE.md** - Add orc-specific section with task weights and phase preferences
3. **Configure .orc/config.yaml** - Set test commands, review settings, automation profile
4. **Ask the user** what areas they want to focus on (especially for {{.ProjectSize}} projects)

### Configuration to Set

In `.orc/config.yaml`, configure based on what you discover:
- `test.unit`, `test.integration`, `test.e2e` - actual commands for this project
- `review.rounds` - 1 for simple projects, 2 for complex
- `automation.profile` - auto/fast/safe/strict based on user preference
- `worktree.enabled` - true for isolated task branches

### CLAUDE.md orc Section

Add a section documenting:
- Task weight examples specific to this codebase
- Phase preferences (which phases to skip/require)
- Quality gates if the project has specific requirements

## When Complete

End with a **Next Steps** summary:

1. **What you configured** - Brief table of changes made
2. **What still needs setup** - Any gaps (missing test commands, unclear patterns)
3. **First task suggestions** - 2-3 concrete tasks based on project state, like:
   - Incomplete features you noticed
   - Missing tests for existing code
   - Documentation gaps
4. **How to proceed**:
   ```bash
   # CLI commands
   orc new "your task description"
   orc run TASK-001
   orc serve  # web UI at localhost:8080
   ```

   Or use the orc plugin commands in Claude Code:
   - `/orc:init` - Start a new task interactively
   - `/orc:status` - Show current task status
   - `/orc:continue` - Resume work on current task
   - `/orc:review` - Start code review
   - `/orc:qa` - Run QA session
   - `/orc:propose` - Propose a sub-task

Be specific. Reference actual files and patterns you found. No generic advice.
