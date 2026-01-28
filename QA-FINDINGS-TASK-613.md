# QA Findings: Agents Page (TASK-613)

**Test Date**: 2026-01-28
**Tester**: QA Engineer
**Reference Design**: `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png`
**Method**: Code analysis and reference design comparison

---

## Summary

**Total Findings**: 1 Critical, Multiple High-severity issues identified
**Overall Assessment**: Page implementation does not match the reference design. The current implementation appears to be for a different feature (Claude Code sub-agent configuration) rather than the Claude model execution configuration shown in the reference.

---

## Finding QA-001: Complete Page Implementation Mismatch

**Severity**: Critical
**Confidence**: 100%
**Category**: Functional

### Title
Agents page implementation does not match reference design - wrong feature implemented

### Steps to Reproduce
1. Navigate to `http://localhost:5173/agents`
2. Compare visible elements to reference design at `example_ui/agents-config.png`

### Expected
Per the reference design, the page should display:
- **Header Section**:
  - h1 title "Agents"
  - Subtitle "Configure Claude models and execution settings"
  - "+ Add Agent" button (top-right)

- **Active Agents Section**:
  - Section heading "Active Agents"
  - Subtitle "Currently configured Claude instances"
  - 3 agent cards with:
    - Agent name with emoji (🧠 Primary Coder, ⚡ Quick Tasks, 🔍 Code Review)
    - Model name (claude-sonnet-4-20250514, claude-haiku-3-5-20241022)
    - Status badge (ACTIVE/IDLE)
    - Stats: Token count, Tasks done, Success rate
    - Tool badges (File Read/Write, Bash, Web Search, MCP, Git Diff)

- **Execution Settings Section**:
  - Section heading "Execution Settings"
  - Subtitle "Global configuration for all agents"
  - 4 controls arranged in 2x2 grid:
    1. **Parallel Tasks**: Slider (1-10) with current value display
    2. **Auto-Approve**: Toggle switch
    3. **Default Model**: Dropdown selector
    4. **Cost Limit**: Slider ($0-$100) with dollar amount display

- **Tool Permissions Section**:
  - Section heading "Tool Permissions"
  - Subtitle "Control what actions agents can perform"
  - 6 permission toggles in 3-column grid:
    1. File Read
    2. File Write
    3. Bash Commands
    4. Web Search
    5. Git Operations
    6. MCP Servers

### Actual
Current implementation shows:
- h3 title "Agents" (wrong heading level)
- Subtitle "Sub-agent definitions for specialized Claude Code tasks" (wrong text)
- NO "+ Add Agent" button
- Project/Global scope tabs (not in design)
- Simple card grid showing agent definitions
- NO Active Agents section
- NO Execution Settings section
- NO Tool Permissions section
- Preview modal for agent details (not in design)

### Impact
- Page does not fulfill requirements
- Wrong feature implemented (sub-agent definitions vs. model configuration)
- Missing all interactive configuration controls
- Missing agent status monitoring
- Missing execution settings management
- Missing tool permission controls

### Screenshot Path
N/A - Code analysis based finding

### Suggested Fix
The entire `src/pages/environment/Agents.tsx` needs to be rewritten to match the reference design. Current implementation should be moved to a different page if sub-agent configuration is still needed.

**Required Implementation**:
1. Create new components:
   - `AgentsView` - Main container
   - `AgentCard` - Individual agent display with stats and badges
   - `ExecutionSettings` - 2x2 grid of configuration controls
   - `ToolPermissions` - 3-column grid of permission toggles

2. Update page structure:
   ```tsx
   <div className="agents-page">
     <PageHeader
       title="Agents"
       subtitle="Configure Claude models and execution settings"
       action={<Button>+ Add Agent</Button>}
     />

     <Section title="Active Agents" subtitle="Currently configured Claude instances">
       <AgentCardsGrid>
         {agents.map(agent => <AgentCard key={agent.id} agent={agent} />)}
       </AgentCardsGrid>
     </Section>

     <Section title="Execution Settings" subtitle="Global configuration for all agents">
       <ExecutionSettings />
     </Section>

     <Section title="Tool Permissions" subtitle="Control what actions agents can perform">
       <ToolPermissions />
     </Section>
   </div>
   ```

3. Component specifications:
   - **AgentCard**: Display emoji, name, model, status badge, stats row (tokens/tasks/success%), tool badges
   - **ExecutionSettings**: 2x2 responsive grid with Slider (Parallel Tasks, Cost Limit), Toggle (Auto-Approve), Select (Default Model)
   - **ToolPermissions**: 3-column responsive grid with 6 Toggle components

### Code Reference
- Current file: `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/src/pages/environment/Agents.tsx`
- Lines 96-236: Entire page implementation

---

## Detailed Component Analysis

### Missing Components

#### 1. Active Agents Section (High Priority)
**Status**: Not implemented
**Requirements**:
- Section heading + subtitle
- Grid layout for agent cards
- Each card needs:
  - Emoji + name display
  - Model identifier
  - Status badge (color-coded: green=ACTIVE, gray=IDLE)
  - Stats row with 3 metrics
  - Tool badges row (filtered by agent's enabled tools)

#### 2. Execution Settings (High Priority)
**Status**: Not implemented
**Requirements**:
- 2x2 responsive grid (mobile: 1 column)
- **Parallel Tasks Slider**:
  - Range: 1-10
  - Display current value next to label
  - Keyboard accessible (arrow keys)
- **Auto-Approve Toggle**:
  - On/Off state
  - Subtitle: "Automatically approve safe operations without prompting"
- **Default Model Dropdown**:
  - Options: Available Claude models
  - Current value visible
  - Subtitle: "Model to use for new tasks"
- **Cost Limit Slider**:
  - Range: $0-$100
  - Display formatted dollar amount
  - Subtitle: "Daily spending limit before pause"

#### 3. Tool Permissions (High Priority)
**Status**: Not implemented
**Requirements**:
- 3-column grid (mobile: 1 column)
- 6 toggle switches with icons and labels
- Each toggle:
  - Icon representing the tool
  - Tool name
  - On/Off state
  - Visual feedback on toggle

### Missing Header Elements

**"+ Add Agent" Button**:
- Position: Top-right of page header
- Styling: Primary button with "+" icon
- Behavior: Opens modal/dialog for adding new agent
- Currently: Not implemented

---

## Testing Artifacts

### Test Scripts Created
The following test scripts were created for future execution:

1. **`web/qa-agents-test.mjs`**: Comprehensive Playwright test
   - Navigates to /agents page
   - Verifies all page elements
   - Tests interactivity (sliders, toggles)
   - Captures desktop + mobile screenshots
   - Checks console for errors
   - Generates findings.json

2. **`web/run-qa.sh`**: Bash wrapper script
   - Checks if dev server is running
   - Executes Playwright test
   - Outputs results

### To Run Tests
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node qa-agents-test.mjs
```

**Note**: Tests require dev server running at `http://localhost:5173`

---

## Recommendations

### Immediate Actions
1. **Halt current implementation** - Wrong feature is being built
2. **Clarify requirements** - Confirm reference design is correct
3. **Create component architecture** - Design the 3 main sections before coding
4. **Implement in phases**:
   - Phase 1: Page header + Active Agents cards (mock data)
   - Phase 2: Execution Settings controls
   - Phase 3: Tool Permissions grid
   - Phase 4: Wire up to backend API
   - Phase 5: Add Agent modal/form

### Design System Considerations
- Reuse existing components from `components/core/`:
  - `Slider` - For Parallel Tasks and Cost Limit
  - `Toggle` - For Auto-Approve and Tool Permissions
  - `Select` - For Default Model dropdown
  - `Badge` - For status indicators
  - `Card` - For agent card containers
  - `Stat` - For agent stats display

### Testing Requirements
Once implementation is corrected:
1. **Visual Regression**: Compare screenshots to reference design
2. **Interaction Testing**:
   - Test all sliders respond to drag/click/keyboard
   - Test all toggles switch state
   - Test dropdown opens and selects
   - Test "+ Add Agent" opens modal
3. **Responsive Testing**:
   - Desktop: 1920x1080
   - Mobile: 375x667
   - Verify grid layouts stack correctly
4. **Accessibility Testing**:
   - Keyboard navigation through all controls
   - ARIA labels for toggles and sliders
   - Focus indicators visible

---

## Conclusion

The current Agents page implementation is fundamentally misaligned with the reference design. Rather than small bug fixes, this requires a complete reimplementation of the page following the design specification. The current code should be preserved if sub-agent configuration is still a needed feature, but moved to a different route (e.g., `/settings/sub-agents`).

**Estimated Effort**: Full page rebuild required (~8-16 hours for complete implementation with tests)

**Priority**: Critical - Blocks any meaningful QA of the Agents feature

---

**Test Scripts Location**:
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/qa-agents-test.mjs`
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web/run-qa.sh`

**Reference Design**:
- `/home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png`
