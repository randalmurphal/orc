# Visual Comparison Guide - Agents Page

Quick reference for comparing implementation against reference design.

## Reference Design
Location: `example_ui/agents-config.png`

## Layout Structure

```
┌─────────────────────────────────────────────────────────┐
│ Agents                              [+ Add Agent]       │ ← Header
│ Configure Claude models and execution settings          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Active Agents                                           │ ← Section 1
│ Currently configured Claude instances                   │
│                                                         │
│ ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│ │ Primary  │  │  Quick   │  │   Code   │            │ ← 3 Cards
│ │  Coder   │  │  Tasks   │  │  Review  │            │
│ │ [ACTIVE] │  │  [IDLE]  │  │  [IDLE]  │            │
│ │  Stats   │  │  Stats   │  │  Stats   │            │
│ │  Tools   │  │  Tools   │  │  Tools   │            │
│ └──────────┘  └──────────┘  └──────────┘            │
│                                                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Execution Settings                                      │ ← Section 2
│ Global configuration for all agents                     │
│                                                         │
│ ┌─────────────────────┐  ┌─────────────────────┐    │
│ │ Parallel Tasks  [2] │  │ Auto-Approve   [ON] │    │ ← Row 1
│ │ ────────●────────   │  │                     │    │
│ └─────────────────────┘  └─────────────────────┘    │
│                                                         │
│ ┌─────────────────────┐  ┌─────────────────────┐    │
│ │ Default Model       │  │ Cost Limit    [$25] │    │ ← Row 2
│ │ claude-sonnet-4-... │  │ ────────●────────   │    │
│ └─────────────────────┘  └─────────────────────┘    │
│                                                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Tool Permissions                                        │ ← Section 3
│ Control what actions agents can perform                 │
│                                                         │
│ File Read       [ON]   File Write      [ON]            │ ← Row 1
│ Bash Commands   [ON]   Web Search      [ON]            │ ← Row 2
│ Git Operations  [ON]   MCP Servers     [ON]            │ ← Row 3
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Color Scheme

### Status Badges
- **ACTIVE**: Green background (#10b981 / emerald-500)
- **IDLE**: Gray background (#6b7280 / gray-500)

### Sections
- Dark background cards (#1e293b / slate-800)
- Section dividers with subtle borders

### Interactive Elements
- **Primary Button**: Purple/violet primary color
- **Toggles**: Purple when enabled, gray when disabled
- **Sliders**: Purple track with white handle

## Typography

### Headings
- **Page Title**: ~32px, bold, white
- **Page Subtitle**: ~16px, gray-400
- **Section Titles**: ~20px, semibold, white
- **Section Subtitles**: ~14px, gray-400

### Card Content
- **Agent Name**: ~18px, semibold
- **Model**: ~12px, gray-400, monospace
- **Stats Numbers**: ~24px, bold
- **Stats Labels**: ~12px, gray-500

## Agent Cards - Expected Data

### Primary Coder
```
Name: Primary Coder
Model: claude-sonnet-4-20250514
Status: ACTIVE (green)
Stats:
  - 847K Tokens Used
  - 34 Tasks Done
  - 94% Success Rate
Tools: [File Read/Write] [Bash] [Web Search] [MCP]
```

### Quick Tasks
```
Name: Quick Tasks
Model: claude-haiku-3-5-20241022
Status: IDLE (gray)
Stats:
  - 124K Tokens Used
  - 12 Tasks Done
  - 91% Success Rate
Tools: [File Read] [Web Search]
```

### Code Review
```
Name: Code Review
Model: claude-sonnet-4-20250514
Status: IDLE (gray)
Stats:
  - 256K Tokens Used
  - 8 Tasks Done
  - 100% Success Rate
Tools: [File Read] [Git Diff]
```

## Execution Settings - Expected Values

```
Parallel Tasks: 2
  - Slider at position 2 out of 5
  - Label shows "2"

Auto-Approve: Enabled
  - Toggle in ON position
  - Purple color

Default Model: claude-sonnet-4-20250514
  - Dropdown showing this value
  - Options include Opus 4, Haiku 3.5

Cost Limit: $25
  - Slider at position showing "$25"
  - Range 0-100
```

## Tool Permissions - Expected State

All 6 toggles should be **ENABLED** (purple/ON) by default:

```
[ON] File Read          [ON] File Write
[ON] Bash Commands      [ON] Web Search
[ON] Git Operations     [ON] MCP Servers
```

## Spacing & Layout

### Sections
- Vertical spacing between sections: ~48px
- Section header margin-bottom: ~24px

### Cards
- Gap between agent cards: ~24px
- Card padding: ~24px

### Settings Grid
- 2 columns on desktop
- Gap between cards: ~24px
- Card padding: ~20px

### Tool Permissions Grid
- 3 columns on desktop (2 toggles per row)
- Gap between items: ~16px

## Mobile Breakpoints

### 375px viewport
- Stack sections vertically
- Agent cards in single column
- Execution settings in single column
- Tool permissions in single column
- **NO horizontal scrolling**

## Accessibility Checks

- [ ] All interactive elements have aria-labels
- [ ] Sections use semantic HTML (section, header, article)
- [ ] Toggles are keyboard navigable
- [ ] Sliders show values and support keyboard input
- [ ] High contrast between text and backgrounds
- [ ] Focus indicators visible on all controls

## Common Visual Issues to Check

### Agent Cards
- [ ] Cards have equal heights
- [ ] Status badges aligned consistently
- [ ] Stats are centered and aligned
- [ ] Tool badges wrap properly
- [ ] Icon colors match design (purple, blue, green, amber)

### Execution Settings
- [ ] Setting cards have equal heights
- [ ] Labels aligned left
- [ ] Controls aligned right (for toggles)
- [ ] Sliders show values clearly
- [ ] Descriptions are readable

### Tool Permissions
- [ ] Grid maintains 3-column layout on desktop
- [ ] Icons align with labels
- [ ] Toggles align to the right
- [ ] Spacing is consistent

### Overall
- [ ] No layout shift on load
- [ ] Smooth hover states
- [ ] No overlapping elements
- [ ] Consistent color usage
- [ ] Loading states show skeletons (if API is slow)

---

## Quick Desktop Test (1920x1080)

1. Open http://localhost:5173/agents
2. Check page title and "+ Add Agent" button
3. Count agent cards (should be 3)
4. Verify section headings (Active Agents, Execution Settings, Tool Permissions)
5. Check all 4 execution settings controls
6. Check all 6 tool permission toggles
7. Open browser console - should have no errors

## Quick Mobile Test (375x667)

1. Resize browser to 375px width (or use DevTools device emulation)
2. Verify no horizontal scrolling
3. Check all sections are readable
4. Test toggle interactions
5. Verify cards stack vertically

---

## Screenshot Comparison

When comparing screenshots:

1. **Overall layout**: Does structure match reference?
2. **Content**: Are all texts and values correct?
3. **Styling**: Do colors and spacing look similar?
4. **Functionality**: Can you interact with all controls?

**Note**: Minor style differences (shadows, exact spacing) are acceptable as long as the structure and content match.
