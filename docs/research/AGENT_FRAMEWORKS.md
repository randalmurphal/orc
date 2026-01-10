# Agent Orchestration Frameworks

**Purpose**: Survey of general agent frameworks and patterns applicable to orc.

---

## Framework Comparison

| Framework | Language | Key Feature | Complexity |
|-----------|----------|-------------|------------|
| LangGraph | Python | Graph-based workflows | High |
| CrewAI | Python | Role-based agents | Medium |
| AutoGen | Python | Multi-agent conversations | High |
| Swarm | Python | Lightweight handoffs | Low |
| OpenAI Agents SDK | Python | Official OpenAI | Medium |

---

## LangGraph

**Pattern**: Stateful graph execution with checkpointing.

```python
from langgraph.graph import StateGraph

graph = StateGraph(State)
graph.add_node("research", research_node)
graph.add_node("implement", implement_node)
graph.add_edge("research", "implement")
```

**Relevant for orc**:
- Graph-based phase transitions ✓
- Built-in checkpointing ✓
- State persistence ✓

**Not applicable**:
- Python-specific
- Requires LangChain ecosystem

---

## CrewAI

**Pattern**: Agents with roles, goals, and backstories.

```python
researcher = Agent(
    role="Senior Researcher",
    goal="Find relevant information",
    backstory="Expert at analyzing codebases"
)
```

**Relevant for orc**:
- Role specialization (review vs implement)
- Task delegation patterns

**Not applicable**:
- Heavy abstraction
- Python-specific

---

## OpenAI Swarm

**Pattern**: Lightweight agent handoffs.

```python
def transfer_to_reviewer():
    return reviewer_agent

implementer = Agent(
    functions=[implement, transfer_to_reviewer]
)
```

**Relevant for orc**:
- Simple handoff model ✓
- Minimal overhead ✓
- Function-based actions ✓

**Best fit for orc's phase transitions.**

---

## Mixture of Agents (MoA)

**Pattern**: Multiple agents propose, one aggregates.

```
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Agent 1 │  │ Agent 2 │  │ Agent 3 │
└────┬────┘  └────┬────┘  └────┬────┘
     │            │            │
     └────────────┼────────────┘
                  │
                  ▼
           ┌───────────┐
           │Aggregator │
           └───────────┘
```

**Relevant for orc**:
- Review phase (multiple reviewers)
- Spec validation (multiple perspectives)

---

## Patterns to Adopt

### 1. Graph-Based Transitions
Phases form a DAG with conditional edges.

### 2. Lightweight Handoffs
Keep inter-phase communication minimal.

### 3. Filesystem as State
No complex serialization - YAML files.

### 4. Checkpointing
Git commits, not database snapshots.

### 5. Human-in-the-Loop
Gates for critical transitions.

---

## Anti-Patterns

| Pattern | Problem | Orc Alternative |
|---------|---------|-----------------|
| Complex agent protocols | Hard to debug | Simple prompts |
| In-memory state | Lost on crash | File persistence |
| Implicit handoffs | Unpredictable | Explicit phases |
| Framework lock-in | Portability | Shell + subprocess |

---

## Orc's Position

Orc takes the **simplest viable approach**:

| Aspect | Complex Frameworks | Orc |
|--------|-------------------|-----|
| State | Custom serialization | YAML files |
| Checkpoints | Database | Git commits |
| Transitions | Graph engine | Sequential phases |
| Agents | Multi-agent protocols | Single Claude session |
| Recovery | Framework-specific | Git rewind |

**Philosophy**: Complexity should come from the task, not the tool.
