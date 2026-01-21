# Conflict Resolution Task

You are resolving merge conflicts for task: {{.TaskID}} - {{.TaskTitle}}

## Conflicted Files

{{range .ConflictFiles}}- `{{.}}`
{{end}}

## Conflict Resolution Rules

**CRITICAL - You MUST follow these rules:**

1. **NEVER remove features** - Both your changes AND upstream changes must be preserved
2. **Merge intentions, not text** - Understand what each side was trying to accomplish
3. **Prefer additive resolution** - If in doubt, keep both implementations
4. **Test after every file** - Don't batch conflict resolutions

## Prohibited Resolutions

- **NEVER**: Just take "ours" or "theirs" without understanding
- **NEVER**: Remove upstream features to fix conflicts
- **NEVER**: Remove your features to fix conflicts
- **NEVER**: Comment out conflicting code

{{if .Instructions}}
## Additional Instructions

{{.Instructions}}
{{end}}

## Instructions

1. For each conflicted file, read and understand both sides of the conflict
2. Resolve the conflict by merging both changes appropriately
3. Stage the resolved file with `git add <file>`
4. After all files are resolved, output ONLY this JSON:
```json
{"status": "complete", "summary": "Resolved X conflicts in files A, B, C"}
```

If you cannot resolve a conflict, output ONLY this JSON:
```json
{"status": "blocked", "reason": "[explanation]"}
```
