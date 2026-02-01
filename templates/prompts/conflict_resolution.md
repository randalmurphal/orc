# Conflict Resolution Task

You are resolving merge conflicts for task: {{.TaskID}} - {{.TaskTitle}}

{{if .TaskDescription}}
## Task Context

{{.TaskDescription}}
{{end}}

## Conflicted Files

{{range .ConflictFiles}}- `{{.}}`
{{end}}

## Understanding the Conflict

Before resolving, understand what happened:
- **Your changes (HEAD)**: Work done for this task
- **Upstream changes (target branch)**: Work from parallel tasks that merged first

The conflict exists because both sides modified the same lines. Your job is to preserve BOTH intentions.

## Conflict Resolution Rules

**CRITICAL - You MUST follow these rules:**

1. **NEVER remove features** - Both your changes AND upstream changes must be preserved
2. **Merge intentions, not text** - Understand what each side was trying to accomplish
3. **Prefer additive resolution** - If in doubt, keep both implementations
4. **Test after every file** - Don't batch conflict resolutions
5. **Preserve all imports** - If both sides added imports, keep all of them
6. **Check for duplicates** - If both sides added similar code, deduplicate intelligently

## Prohibited Resolutions

- **NEVER**: Just take "ours" or "theirs" without understanding
- **NEVER**: Remove upstream features to fix conflicts
- **NEVER**: Remove your features to fix conflicts
- **NEVER**: Comment out conflicting code
- **NEVER**: Leave conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) in files

## Resolution Strategy

1. **Read the entire file** to understand context, not just the conflict markers
2. **Identify what each side was adding/changing**:
   - What new functions/methods were added?
   - What existing functions were modified?
   - What imports were added?
3. **Merge intelligently**:
   - New functions from both sides: keep both
   - Same function modified differently: merge the changes
   - Import conflicts: combine all imports, deduplicate
4. **Verify the result**:
   - No syntax errors
   - All new code from both sides is present
   - No duplicate definitions

{{if .Instructions}}
## Additional Instructions

{{.Instructions}}
{{end}}

## Resolution Steps

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
