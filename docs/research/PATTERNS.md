# Patterns and Anti-Patterns

**Purpose**: Distilled wisdom from research into actionable patterns for orc.

---

## Patterns to Adopt

### 1. Filesystem as Shared Memory

**What**: Use filesystem state instead of message passing.

**Why**: 
- Persists across crashes
- Human-readable
- Git-trackable
- No serialization bugs

**Implementation**:
```
.orc/tasks/TASK-001/
├── task.yaml      # Definition
├── state.yaml     # Execution state
├── plan.yaml      # Phase sequence
└── transcripts/   # Claude logs
```

---

### 2. Completion Tags

**What**: Explicit XML tags for phase completion.

**Why**:
- Unambiguous parsing
- Works with any output format
- Easy to detect programmatically

**Implementation**:
```markdown
I've completed all the requirements.

<phase_complete>true</phase_complete>
```

**Parser**:
```go
var pattern = regexp.MustCompile(`<phase_complete>(\w+)</phase_complete>`)
```

---

### 3. Git-Native Checkpointing

**What**: Use git commits as checkpoints, branches for isolation.

**Why**:
- Battle-tested
- Familiar tooling
- Free diffing, history, rewind
- Collaboration built-in

**Implementation**:
- Branch: `orc/TASK-ID`
- Commit: `[orc] phase: status`
- Rewind: `git reset --hard <commit>`

---

### 4. Weight-Based Rigor Scaling

**What**: Task complexity determines phase sequence.

**Why**:
- Trivial tasks stay trivial
- Complex tasks get appropriate gates
- Predictable for users

**Implementation**:
| Weight | Phases |
|--------|--------|
| trivial | implement |
| small | implement → test |
| medium | spec → implement → review → test |
| large | research → spec → design → implement → review → test |

---

### 5. Ralph-Style Loops Within Phases

**What**: Iterate until completion, not fixed steps.

**Why**:
- Self-correcting
- Handles partial failures
- Matches human work patterns

**Implementation**:
```go
for i := 0; i < maxIterations; i++ {
    result := RunClaude(prompt)
    if MeetsCriteria(result) {
        return success
    }
}
return maxIterationsExceeded
```

---

### 6. Human Gates for High-Stakes Operations

**What**: Require human approval for merge, critical specs.

**Why**:
- Safety for production code
- Compliance requirements
- Human judgment for architecture

**Implementation**:
```yaml
gates:
  merge: human  # Always human by default
  spec: ai      # AI can approve specs
```

---

## Anti-Patterns to Avoid

### 1. Implicit Completion

**Problem**: Agent decides "done" without explicit signal.

**Symptom**: Infinite loops, premature termination.

**Solution**: Always require `<phase_complete>true</phase_complete>`.

---

### 2. In-Memory State

**Problem**: State lost on crash, hard to debug.

**Symptom**: "Where did my progress go?"

**Solution**: Filesystem persistence, git commits.

---

### 3. Complex Agent Protocols

**Problem**: Multi-agent message passing, custom serialization.

**Symptom**: Hard to debug, framework lock-in.

**Solution**: Simple subprocess invocation, file-based communication.

---

### 4. Unbounded Scope

**Problem**: Phase has no clear end condition.

**Symptom**: "Improve the code" runs forever.

**Solution**: Explicit, verifiable completion criteria.

---

### 5. Silent Failures

**Problem**: Errors swallowed, agent continues with bad state.

**Symptom**: Subtle bugs, corrupted output.

**Solution**: Fail loud, surface errors, require acknowledgment.

---

### 6. One-Size-Fits-All

**Problem**: Same process for typo fix and new service.

**Symptom**: Over-engineering simple tasks, under-engineering complex ones.

**Solution**: Weight classification, scaled phase sequences.

---

## Decision Matrix

| Situation | Pattern | Rationale |
|-----------|---------|-----------|
| Need to save progress | Git commit | Native, reliable |
| Need parallel execution | Git worktrees | Isolation without complexity |
| Need completion signal | XML tags | Unambiguous parsing |
| Need human oversight | Gates | Configurable checkpoints |
| Need to recover from failure | Git rewind | Simple, reliable |
| Need to scale rigor | Weight system | Match effort to complexity |

---

## Implementation Priority

| Priority | Pattern | Complexity | Impact |
|----------|---------|------------|--------|
| P0 | Completion tags | Low | Critical |
| P0 | Git checkpoints | Low | Critical |
| P0 | Filesystem state | Low | Critical |
| P1 | Weight classification | Medium | High |
| P1 | Human gates | Medium | High |
| P2 | Ralph loops | Medium | Medium |
| P2 | Worktree isolation | Medium | Medium |
