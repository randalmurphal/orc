# Claude Code Automation Ecosystem

**Purpose**: Survey of existing tools for automating Claude Code sessions.

---

## Official Tools

### Claude Code CLI
- Native subprocess invocation
- `--print` flag for prompt injection
- JSON output format available
- Exit codes for success/failure

### Ralph Wiggum Plugin
- Location: `anthropics/claude-code/plugins/ralph-wiggum`
- Provides stop hook mechanism
- Completion detection via output parsing

---

## Community Tools

### CC-Mirror
**Purpose**: Terminal session management for multiple Claude instances.

| Feature | Description |
|---------|-------------|
| Multi-pane | Run multiple Claude sessions |
| Session sync | Share context between sessions |
| Output capture | Log all sessions |

### Claude-Flow
**Purpose**: Workflow automation with YAML definitions.

```yaml
# Example workflow
steps:
  - name: research
    prompt: "Investigate the codebase..."
  - name: implement
    prompt: "Implement the feature..."
    depends_on: [research]
```

### Claude Squad
**Purpose**: Multi-agent coordination.

| Feature | Description |
|---------|-------------|
| Agent roles | Define specialized agents |
| Communication | Agents share scratchpad |
| Coordination | Dependency-aware execution |

---

## MCP Servers

### Filesystem MCP
- File read/write operations
- Directory traversal
- Git integration

### Database MCP
- SQL query execution
- Schema inspection
- Migration support

### Custom MCP Pattern
```typescript
// Single-tool dispatcher (99% token reduction)
server.tool("dispatch", async (action, params) => {
  switch(action) {
    case "read": return readFile(params.path);
    case "write": return writeFile(params.path, params.content);
    case "search": return searchCode(params.pattern);
  }
});
```

---

## Patterns to Adopt

| Pattern | Description | Orc Integration |
|---------|-------------|-----------------|
| Prompt injection | Stable prompt + filesystem state | Phase prompts |
| Completion detection | XML tags in output | `<phase_complete>` |
| Scratchpad | Structured inter-agent communication | `.orc/tasks/*/` |
| Git checkpointing | Commits as save points | Built-in |
| Worktree isolation | Parallel execution | Supported |

---

## Anti-Patterns to Avoid

| Anti-Pattern | Problem | Alternative |
|--------------|---------|-------------|
| Complex state machines | Hard to debug | Ralph-style loops |
| In-memory state | Lost on crash | Filesystem persistence |
| Implicit completion | Infinite loops | Explicit criteria |
| Tight coupling | Hard to modify | Phase-based architecture |
