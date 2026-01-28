# QA Test Report - TASK-613: Agents Page

**Date**: 2026-01-28
**Status**: CRITICAL ISSUE IDENTIFIED
**Test Method**: Code analysis vs. reference design

---

## Executive Summary

**The Agents page implementation does not match the reference design.**

The current code at `/agents` displays a sub-agent configuration interface (for Claude Code task specialization), while the reference design shows a Claude model execution configuration page with agent monitoring, execution settings, and tool permissions.

**Finding**: 1 Critical issue (100% confidence)
**Status**: Implementation blocked - requires full page rebuild

---

## What Was Expected

Based on the reference design (`example_ui/agents-config.png`), the page should have:

### 1. Page Header
- ✅ h1: "Agents"
- ✅ Subtitle: "Configure Claude models and execution settings"
- ✅ Button: "+ Add Agent" (top-right)

### 2. Active Agents Section
- ✅ Heading: "Active Agents"
- ✅ Subtitle: "Currently configured Claude instances"
- ✅ **3 Agent Cards**, each showing:
  - Agent emoji + name (🧠 Primary Coder, ⚡ Quick Tasks, 🔍 Code Review)
  - Model name
  - Status badge (ACTIVE/IDLE)
  - Stats row: Token count, Tasks done, Success rate
  - Tool badges row: Enabled tools for that agent

### 3. Execution Settings Section
- ✅ Heading: "Execution Settings"
- ✅ Subtitle: "Global configuration for all agents"
- ✅ **4 Controls in 2x2 Grid**:
  1. **Parallel Tasks** - Slider (1-10)
  2. **Auto-Approve** - Toggle switch
  3. **Default Model** - Dropdown selector
  4. **Cost Limit** - Slider ($0-$100)

### 4. Tool Permissions Section
- ✅ Heading: "Tool Permissions"
- ✅ Subtitle: "Control what actions agents can perform"
- ✅ **6 Permission Toggles** (3-column grid):
  1. File Read
  2. File Write
  3. Bash Commands
  4. Web Search
  5. Git Operations
  6. MCP Servers

---

## What Was Found

Current implementation (`web/src/pages/environment/Agents.tsx`):

### Present
- ❌ h3 "Agents" (wrong heading level)
- ❌ Subtitle: "Sub-agent definitions for specialized Claude Code tasks" (wrong text)
- ❌ Project/Global scope tabs (not in design)
- ❌ Card grid showing agent definitions from config
- ❌ Preview modal for viewing agent details

### Missing
- ❌ "+ Add Agent" button
- ❌ Active Agents section
- ❌ Agent cards with stats and status
- ❌ Execution Settings section
- ❌ Tool Permissions section

---

## Impact Assessment

| Impact Area | Severity |
|------------|----------|
| Feature completeness | Critical - 0% match |
| User requirements | Not met |
| Design adherence | Not met |
| Functionality | Wrong feature built |

**This is not a bug fix situation - it's a full implementation gap.**

---

## Root Cause

The current implementation was built for a different use case:
- **Current**: Configure Claude Code sub-agents for specialized tasks
- **Required**: Configure Claude model instances with execution settings and monitoring

These are two different features. The current code should be:
1. Preserved if sub-agent config is still needed (move to different route)
2. OR completely replaced if requirements changed

---

## Recommended Actions

### Immediate (Before Further Development)
1. ✅ **Clarify requirements** with product/design team
   - Confirm reference design is the correct target
   - Determine fate of current sub-agent configuration UI

2. ✅ **Scope the work** - This is not a small fix
   - Estimated: 8-16 hours for complete implementation
   - Requires: 3-4 new components + page restructure

### Implementation Plan

If reference design is confirmed:

#### Phase 1: Component Library (2-3 hours)
- Create `AgentCard` component
  - Props: name, emoji, model, status, stats, tools
  - Layout: Card with header, stats row, badges row
  - Use existing `Badge`, `Card`, `Stat` from `components/core/`

- Create `ExecutionSettings` component
  - 2x2 grid layout (responsive)
  - Use existing `Slider`, `Toggle`, `Select` components
  - Handle state management

- Create `ToolPermissions` component
  - 3-column grid layout (responsive)
  - Use existing `Toggle` component
  - 6 permission items with icons

#### Phase 2: Page Integration (2-3 hours)
- Replace `Agents.tsx` content
- Implement proper page header with h1 and button
- Add 3 main sections with headings/subtitles
- Wire up mock data for initial testing

#### Phase 3: Backend Integration (2-4 hours)
- Define API contracts for:
  - Fetching active agents + stats
  - Updating execution settings
  - Updating tool permissions
  - Adding new agent
- Implement API calls
- Add loading/error states

#### Phase 4: Polish & Testing (2-3 hours)
- Add "+ Add Agent" modal/form
- Responsive testing (desktop + mobile)
- Interaction testing (sliders, toggles, dropdowns)
- Accessibility testing (keyboard nav, ARIA labels)
- Visual regression vs. reference design

#### Phase 5: QA Validation (1-2 hours)
- Run comprehensive test suite (scripts already created)
- Verify against reference design
- Test edge cases
- Performance check

---

## Testing Resources Created

### Automated Test Scripts
1. **`web/qa-agents-test.mjs`**
   - Comprehensive Playwright test
   - Verifies all page elements
   - Tests interactivity
   - Captures screenshots
   - Generates findings.json

2. **`web/run-qa.sh`**
   - Wrapper script
   - Checks dev server
   - Executes tests

### To Run Tests (Once Implemented)
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node qa-agents-test.mjs
```

Screenshots will be saved to `/tmp/qa-TASK-613/`

---

## Design System Alignment

Good news: Many required components already exist in `components/core/`:

| Needed | Available Component | Status |
|--------|-------------------|--------|
| Sliders | `Slider` | ✅ Ready |
| Toggles | `Toggle` | ✅ Ready |
| Dropdown | `Select` | ✅ Ready |
| Status badges | `Badge` | ✅ Ready |
| Card containers | `Card` | ✅ Ready |
| Stat displays | `Stat` | ✅ Ready |

**Only new component needed**: `AgentCard` (combines existing primitives)

---

## Files Referenced

### Current Implementation
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/src/pages/environment/Agents.tsx`

### Reference Design
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png`

### QA Artifacts
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/QA-FINDINGS-TASK-613.md` (detailed findings)
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/qa-findings.json` (structured data)
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/qa-agents-test.mjs` (test script)
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/run-qa.sh` (runner script)

---

## Next Steps

1. ✅ **Review this report** with team
2. ✅ **Confirm reference design** is the target
3. ✅ **Decide on current code**:
   - Keep and move to `/settings/sub-agents`?
   - OR replace completely?
4. ✅ **Create task breakdown** for implementation
5. ✅ **Begin Phase 1** (component library)

---

## Questions for Stakeholders

1. Is the reference design (`agents-config.png`) the correct target?
2. Should the current sub-agent configuration UI be preserved?
3. Where should agent statistics come from (mock vs. real API)?
4. Are there any design changes needed before implementation?
5. What is the priority/timeline for this work?

---

**Report Generated**: 2026-01-28
**QA Engineer**: Randy's Workstation QA Process
**Confidence Level**: 100% (Code analysis confirms mismatch)
