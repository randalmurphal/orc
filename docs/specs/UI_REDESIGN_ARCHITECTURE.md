# ORC UI Redesign - Complete Architecture Specification

## Overview

Complete frontend rebuild based on reference mockups in `example_ui/`. This document defines the architecture, initiatives, and tasks for Opus agent execution.

**Reference Files:**
- `example_ui/board.html` - Board view (main dashboard)
- `example_ui/initiatives.html` - Initiatives overview
- `example_ui/stats.html` - Statistics/analytics
- `example_ui/agents.html` - Agent configuration
- `example_ui/settings.html` - Settings hub

**Reference Screenshots:**
- `example_ui/Screenshot_20260116_201804.png` - Board view
- `example_ui/Screenshot_20260116_201819.png` - Initiatives view
- `example_ui/Screenshot_20260116_201852.png` - Stats view
- `example_ui/Screenshot_20260116_201905.png` - Agents view
- `example_ui/Screenshot_20260116_201916.png` - Settings view

---

## Design System Specification

### Color Palette (from mockups)

```css
/* Backgrounds (dark to light) */
--bg-base: #050508;           /* Page background */
--bg-elevated: #0a0a0f;       /* Nav, header, panels */
--bg-surface: #101016;        /* Interactive surfaces */
--bg-card: #15151d;           /* Card backgrounds */
--bg-hover: #1c1c26;          /* Hover states */

/* Borders */
--border: rgba(255, 255, 255, 0.04);
--border-light: rgba(255, 255, 255, 0.08);

/* Text */
--text-primary: #eaeaef;
--text-secondary: #8e8e9a;
--text-muted: #55555f;

/* Primary (Purple) */
--primary: #a855f7;
--primary-bright: #c084fc;
--primary-dim: rgba(168, 85, 247, 0.1);
--primary-glow: rgba(168, 85, 247, 0.25);

/* Semantic Colors */
--green: #10b981;      --green-dim: rgba(16, 185, 129, 0.08);
--amber: #f59e0b;      --amber-dim: rgba(245, 158, 11, 0.08);
--red: #ef4444;        --red-dim: rgba(239, 68, 68, 0.08);
--blue: #3b82f6;       --blue-dim: rgba(59, 130, 246, 0.08);
--cyan: #06b6d4;       --cyan-dim: rgba(6, 182, 212, 0.08);
--orange: #f97316;     --orange-dim: rgba(249, 115, 22, 0.08);
```

### Typography

```css
/* Font Families */
--font-sans: 'Inter', system-ui, sans-serif;
--font-mono: 'JetBrains Mono', monospace;

/* Font Sizes */
--text-xs: 8px;    /* Labels, badges */
--text-sm: 9px;    /* Meta text, muted */
--text-base: 11px; /* Body text */
--text-md: 12px;   /* Secondary text */
--text-lg: 13px;   /* Section titles */
--text-xl: 14px;   /* Card titles */
--text-2xl: 16px;  /* Page titles */
--text-3xl: 18px;  /* Large numbers */
--text-4xl: 24px;  /* Stats */
--text-5xl: 28px;  /* Hero stats */
```

### Layout Dimensions

```css
--icon-nav-width: 56px;
--settings-sidebar-width: 240px;
--right-panel-width: 300px;
--top-bar-height: 48px;
--running-column-width: 420px;
```

### Component Patterns

| Component | Border Radius | Padding | Border |
|-----------|--------------|---------|--------|
| Card | 10-12px | 16-20px | 1px var(--border) |
| Button | 6px | 6-7px 10-12px | 1px var(--border) for ghost |
| Badge | 3-4px | 2-4px 5-8px | none |
| Input | 6px | 6px 10px | 1px var(--border) |
| Toggle | 10px | - | none |

---

## Component Architecture

### Directory Structure

```
web/src/
├── components/
│   ├── core/                 # Atomic design primitives
│   │   ├── Badge.tsx         # Status badges, count badges
│   │   ├── Button.tsx        # Primary, ghost, icon variants
│   │   ├── Card.tsx          # Base card with variants
│   │   ├── Icon.tsx          # Lucide icon wrapper
│   │   ├── Input.tsx         # Text input, search box
│   │   ├── Progress.tsx      # Progress bars
│   │   ├── Select.tsx        # Dropdown select
│   │   ├── Slider.tsx        # Range slider
│   │   ├── Stat.tsx          # Stat card (value + label + trend)
│   │   ├── Toggle.tsx        # On/off toggle
│   │   └── Tooltip.tsx       # Hover tooltip
│   │
│   ├── layout/               # App shell components
│   │   ├── AppShell.tsx      # Main grid layout
│   │   ├── IconNav.tsx       # 56px icon sidebar
│   │   ├── TopBar.tsx        # Header with session info
│   │   └── RightPanel.tsx    # 300px collapsible panel
│   │
│   ├── board/                # Board view components
│   │   ├── BoardView.tsx     # Main board container
│   │   ├── QueueColumn.tsx   # Queue with swimlanes
│   │   ├── RunningColumn.tsx # Active tasks with pipeline
│   │   ├── Swimlane.tsx      # Collapsible initiative group
│   │   ├── TaskCard.tsx      # Compact task card
│   │   ├── RunningCard.tsx   # Expanded running task
│   │   ├── Pipeline.tsx      # 5-phase progress visualization
│   │   ├── BlockedPanel.tsx  # Blocked tasks section
│   │   ├── DecisionsPanel.tsx # Pending decisions section
│   │   ├── ConfigPanel.tsx   # Claude config links
│   │   ├── FilesPanel.tsx    # Files changed section
│   │   └── CompletedPanel.tsx # Completed summary
│   │
│   ├── initiatives/          # Initiatives view
│   │   ├── InitiativesView.tsx
│   │   ├── InitiativeCard.tsx
│   │   └── StatsRow.tsx
│   │
│   ├── stats/                # Stats view
│   │   ├── StatsView.tsx
│   │   ├── ActivityHeatmap.tsx
│   │   ├── TasksBarChart.tsx
│   │   ├── OutcomesDonut.tsx
│   │   └── LeaderboardTable.tsx
│   │
│   ├── agents/               # Agents view
│   │   ├── AgentsView.tsx
│   │   ├── AgentCard.tsx
│   │   ├── ExecutionSettings.tsx
│   │   └── ToolPermissions.tsx
│   │
│   └── settings/             # Settings view
│       ├── SettingsLayout.tsx
│       ├── SettingsSidebar.tsx
│       ├── CommandList.tsx
│       ├── CommandEditor.tsx
│       └── ConfigEditor.tsx
│
├── pages/                    # Route pages
│   ├── BoardPage.tsx
│   ├── InitiativesPage.tsx
│   ├── StatsPage.tsx
│   ├── AgentsPage.tsx
│   └── SettingsPage.tsx
│
├── stores/                   # Zustand stores
│   ├── sessionStore.ts       # NEW: Session metrics
│   ├── statsStore.ts         # NEW: Analytics data
│   └── (existing stores)
│
└── styles/
    └── tokens.css            # Design tokens (replace existing)
```

---

## API Requirements

### Existing Endpoints (sufficient)
- Tasks: CRUD, state, transcripts, tokens, diffs
- Initiatives: CRUD, tasks, dependency graphs
- Dashboard stats: `/api/dashboard/stats`
- Config: `/api/config`
- Agents: `/api/agents`

### New Endpoints Needed

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/session` | GET | Current session metrics (time, tokens, cost) |
| `/api/stats/activity` | GET | Activity heatmap data (16 weeks) |
| `/api/stats/outcomes` | GET | Task outcomes (completed/retry/failed) |
| `/api/stats/per-day` | GET | Tasks per day for bar chart |
| `/api/stats/top-initiatives` | GET | Most active initiatives |
| `/api/stats/top-files` | GET | Most modified files |

---

## Routes

```typescript
const routes = [
  { path: '/', element: <Navigate to="/board" /> },
  { path: '/board', element: <BoardPage /> },
  { path: '/initiatives', element: <InitiativesPage /> },
  { path: '/initiatives/:id', element: <InitiativeDetailPage /> },
  { path: '/stats', element: <StatsPage /> },
  { path: '/agents', element: <AgentsPage /> },
  { path: '/settings/*', element: <SettingsPage /> },
  { path: '/tasks/:id', element: <TaskDetailPage /> },  // Keep existing
];
```

---

## WebSocket Events

### Existing (preserve)
- `task_created`, `task_updated`, `task_deleted`
- `state`, `phase`, `transcript`, `tokens`
- `activity`, `heartbeat`, `finalize`

### New Events Needed
- `session_update` - Session metrics changed
- `decision_required` - Agent needs user input

---
