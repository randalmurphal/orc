# Visual Comparison: Reference Design vs. Current Implementation

## Side-by-Side Analysis

### Reference Design (example_ui/agents-config.png)

```
┌─────────────────────────────────────────────────────────────┐
│ 🤖 Agents                              [+ Add Agent]         │
│ Configure Claude models and execution settings              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Active Agents                                                │
│ Currently configured Claude instances                        │
│                                                              │
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│ │ 🧠 Primary   │  │ ⚡ Quick      │  │ 🔍 Code      │       │
│ │ Coder        │  │ Tasks        │  │ Review       │       │
│ │              │  │              │  │              │       │
│ │ claude-son.. │  │ claude-hai.. │  │ claude-son.. │       │
│ │ [ACTIVE]     │  │ [IDLE]       │  │ [IDLE]       │       │
│ │              │  │              │  │              │       │
│ │ 847K tokens  │  │ 124K tokens  │  │ 256K tokens  │       │
│ │ 34 tasks     │  │ 12 tasks     │  │ 8 tasks      │       │
│ │ 94% success  │  │ 91% success  │  │ 100% success │       │
│ │              │  │              │  │              │       │
│ │ [Read/Write] │  │ [Read]       │  │ [Read]       │       │
│ │ [Bash]       │  │ [Web Search] │  │ [Git Diff]   │       │
│ │ [Web Search] │  │              │  │              │       │
│ │ [MCP]        │  │              │  │              │       │
│ └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│ Execution Settings                                           │
│ Global configuration for all agents                          │
│                                                              │
│ ┌───────────────────────┐  ┌───────────────────────┐       │
│ │ Parallel Tasks        │  │ Auto-Approve          │       │
│ │ [====●────]  2        │  │ [●─────]              │       │
│ └───────────────────────┘  └───────────────────────┘       │
│                                                              │
│ ┌───────────────────────┐  ┌───────────────────────┐       │
│ │ Default Model         │  │ Cost Limit            │       │
│ │ [claude-sonnet-4 ▼]   │  │ [==========●]  $25    │       │
│ └───────────────────────┘  └───────────────────────┘       │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│ Tool Permissions                                             │
│ Control what actions agents can perform                      │
│                                                              │
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│ │ File Read    │  │ File Write   │  │ Bash Commands│       │
│ │ [●─────]     │  │ [●─────]     │  │ [●─────]     │       │
│ └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│ │ Web Search   │  │ Git Ops      │  │ MCP Servers  │       │
│ │ [●─────]     │  │ [●─────]     │  │ [●─────]     │       │
│ └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Current Implementation (src/pages/environment/Agents.tsx)

```
┌─────────────────────────────────────────────────────────────┐
│ Agents                                                       │ (h3, not h1)
│ Sub-agent definitions for specialized Claude Code tasks     │ (wrong text)
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ [Project] [Global]  ← Scope tabs (not in design)            │
│                                                              │
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│ │ 👤 Agent 1   │  │ 👤 Agent 2   │  │ 👤 Agent 3   │       │
│ │              │  │              │  │              │       │
│ │ Description  │  │ Description  │  │ Description  │       │
│ │ text here    │  │ text here    │  │ text here    │       │
│ │              │  │              │  │              │       │
│ │ model-name   │  │ model-name   │  │ model-name   │       │
│ │ /path/to/def │  │ /path/to/def │  │ /path/to/def │       │
│ └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
│ (Click to preview agent definition modal)                   │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│ ← NO Execution Settings section                             │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
│ ← NO Tool Permissions section                               │
│                                                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Element-by-Element Comparison

### Header

| Element | Reference | Current | Status |
|---------|-----------|---------|--------|
| Title tag | `<h1>` | `<h3>` | ❌ Wrong |
| Title text | "Agents" | "Agents" | ✅ Match |
| Subtitle | "Configure Claude models and execution settings" | "Sub-agent definitions for specialized Claude Code tasks" | ❌ Wrong |
| Add button | "+ Add Agent" (top-right) | Not present | ❌ Missing |

### Active Agents Section

| Element | Reference | Current | Status |
|---------|-----------|---------|--------|
| Section heading | "Active Agents" | Not present | ❌ Missing |
| Section subtitle | "Currently configured Claude instances" | Not present | ❌ Missing |
| Agent cards | 3 cards with rich data | Simple cards with basic info | ❌ Different |
| Card: Emoji | ✅ (🧠, ⚡, 🔍) | ❌ Generic user icon | ❌ Wrong |
| Card: Name | Agent name | Agent name | ✅ Match |
| Card: Model | Full model name | Model name if present | ⚠️ Partial |
| Card: Status | Badge (ACTIVE/IDLE) | Not present | ❌ Missing |
| Card: Stats | 3 metrics (tokens, tasks, success%) | Not present | ❌ Missing |
| Card: Tool badges | Tool badges row | Not present | ❌ Missing |
| Card interaction | View details? | Click to preview | ⚠️ Different |

### Execution Settings Section

| Element | Reference | Current | Status |
|---------|-----------|---------|--------|
| Entire section | ✅ Present | ❌ Not present | ❌ MISSING |
| Parallel Tasks slider | ✅ | ❌ | ❌ MISSING |
| Auto-Approve toggle | ✅ | ❌ | ❌ MISSING |
| Default Model dropdown | ✅ | ❌ | ❌ MISSING |
| Cost Limit slider | ✅ | ❌ | ❌ MISSING |

### Tool Permissions Section

| Element | Reference | Current | Status |
|---------|-----------|---------|--------|
| Entire section | ✅ Present | ❌ Not present | ❌ MISSING |
| File Read toggle | ✅ | ❌ | ❌ MISSING |
| File Write toggle | ✅ | ❌ | ❌ MISSING |
| Bash Commands toggle | ✅ | ❌ | ❌ MISSING |
| Web Search toggle | ✅ | ❌ | ❌ MISSING |
| Git Operations toggle | ✅ | ❌ | ❌ MISSING |
| MCP Servers toggle | ✅ | ❌ | ❌ MISSING |

### Extra Elements (Not in Design)

| Element | In Reference? | In Current? | Notes |
|---------|---------------|-------------|-------|
| Project/Global tabs | ❌ No | ✅ Yes | Remove or move |
| Preview modal | ❌ No | ✅ Yes | Remove or repurpose |

---

## Scoring

### Implementation Completeness

| Section | Expected Elements | Implemented | % Complete |
|---------|-------------------|-------------|------------|
| Header | 3 (title, subtitle, button) | 2 partial | 33% |
| Active Agents | 8 per card × 3 cards = 24 | 3 partial | 12% |
| Execution Settings | 4 controls | 0 | 0% |
| Tool Permissions | 6 toggles | 0 | 0% |
| **TOTAL** | **31 elements** | **~4** | **~13%** |

### Match Score

- **Visual Design**: 10% match
- **Functionality**: 0% match (different features)
- **Content**: 20% match (same page name only)
- **Interactivity**: 0% match (no settings/permissions)

**Overall Match**: ~8%

---

## Critical Missing Features

### 1. Agent Monitoring (HIGH PRIORITY)
- ❌ Real-time status (ACTIVE/IDLE/ERROR)
- ❌ Usage statistics (tokens, tasks, success rate)
- ❌ Tool capability visualization
- ❌ Model identification

### 2. Execution Control (HIGH PRIORITY)
- ❌ Parallel task limiting
- ❌ Auto-approve configuration
- ❌ Default model selection
- ❌ Cost limiting

### 3. Permission Management (HIGH PRIORITY)
- ❌ Tool-level permission toggles
- ❌ Global security controls
- ❌ Audit trail (implied)

---

## What Needs to Happen

### Delete/Replace
```diff
- Current Agents.tsx (lines 96-236)
- Project/Global scope tabs
- Agent preview modal
- Sub-agent card layout
```

### Add/Create
```diff
+ Page header with h1 + subtitle + button
+ Active Agents section
+   └─ 3 AgentCard components
+       ├─ Emoji + name
+       ├─ Model identifier
+       ├─ Status badge
+       ├─ Stats row (3 metrics)
+       └─ Tool badges row
+ Execution Settings section
+   └─ 2x2 grid of controls
+       ├─ Parallel Tasks slider (1-10)
+       ├─ Auto-Approve toggle
+       ├─ Default Model dropdown
+       └─ Cost Limit slider ($0-$100)
+ Tool Permissions section
+   └─ 3-column grid
+       ├─ File Read toggle
+       ├─ File Write toggle
+       ├─ Bash Commands toggle
+       ├─ Web Search toggle
+       ├─ Git Operations toggle
+       └─ MCP Servers toggle
```

---

## Visual Diff Summary

```diff
--- Current Implementation
+++ Reference Design

- h3 title
+ h1 title

- "Sub-agent definitions for specialized Claude Code tasks"
+ "Configure Claude models and execution settings"

- [Project] [Global] tabs
+ (no tabs)

- Simple agent cards (name, description, model, path)
+ Rich agent cards (emoji, name, model, status, stats, tool badges)

+ [+ Add Agent] button

+ Active Agents section heading + subtitle

+ Execution Settings section
+   Parallel Tasks slider
+   Auto-Approve toggle
+   Default Model dropdown
+   Cost Limit slider

+ Tool Permissions section
+   6 permission toggles in 3-column grid

- Preview modal
+ (no modal - or repurpose for Add Agent)
```

---

## Conclusion

**Visual match**: ~8%
**Functional match**: 0% (different features)
**Implementation status**: Critical mismatch

The current page implements a completely different feature than what's shown in the reference design. This is not a bug - it's an architectural mismatch that requires a full rebuild.

---

**Reference Image**: `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png`
**Current Code**: `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/src/pages/environment/Agents.tsx`
**Analysis Date**: 2026-01-28
