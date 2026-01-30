# Workflow Editor Redesign Specification

## Overview

This document outlines the comprehensive redesign of the workflow editor to match the mockup at `example_ui/example_workflows/workflows_editor.png`.

## Design Direction

**Aesthetic**: Clean, professional, data-rich but not overwhelming. Dark theme with subtle purple accents. Emphasis on clarity and usability over technical details.

**Key Principles**:
1. **User-focused information** - Show what matters to users, not implementation details
2. **Clear visual hierarchy** - Important elements stand out, secondary info is subtle
3. **Consistent interactions** - Predictable, smooth animations
4. **Mode-aware editing** - Clear distinction between viewing and editing

---

## Component Changes

### 1. Remove Start/End Nodes

**Current**: Canvas shows `Start` → phases → `End` nodes
**Target**: Canvas shows only phase nodes, first-to-last flow

**Changes**:
- `layoutWorkflow.ts`: Remove start/end node creation
- `layoutWorkflow.ts`: Update edge creation to connect phases directly
- `WorkflowCanvas.tsx`: Remove startEnd node type handling
- Delete `StartEndNode.tsx` and `StartEndNode.css`

---

### 2. Fix Minimap

**Current**: Renders as gray/white rectangle
**Target**: Shows actual workflow layout with status colors

**Changes**:
- Ensure MiniMap receives proper node data
- Add custom styling for minimap container
- Use `nodeStrokeColor` and `maskColor` props
- Add border and background styling

---

### 3. Header with Mode Tabs

**Current**: Simple breadcrumb + clone button
**Target**: Rich header with workflow name, status, mode tabs

**New Structure**:
```
┌─────────────────────────────────────────────────────────────┐
│ Workflows / Workflow Name                                   │
├─────────────────────────────────────────────────────────────┤
│ [Workflow Name]                    [Overview] [Editing]     │
│ workflow-id                        [Status Badge]   [Clone] │
│ Description text...                ⏱ 0s  🔤 0  💵 $0.00    │
└─────────────────────────────────────────────────────────────┘
```

**Components**:
- `WorkflowHeader.tsx` - New component for header area
- Mode tabs control which view is shown
- Stats display (time, tokens, cost) when relevant

---

### 4. Redesigned Phase Nodes

**Current**: Shows index, name, template ID, executor, iterations, cost
**Target**: Clean card showing name, type badge, description hint, colored accent

**New PhaseNode Structure**:
```
┌──────────────────────────────┐
│ ▌ Phase Name                 │
│   phase_id                   │
│   ─────────────────────────  │
│   [AI] Brief description...  │
│   📄 4 files                 │
└──────────────────────────────┘
```

**Visual Features**:
- Colored left border based on phase category (spec=blue, implement=green, review=orange, docs=purple)
- Type badge (AI for automated, Human for gated)
- Optional file/artifact count
- Clean, minimal design
- Status states (running=glow, completed=green accent, failed=red accent)

---

### 5. Redesigned Right Panel (PhaseInspector)

**Current**: Tabs for Prompt/Variables/Settings
**Target**: Sections for Phase Input, Prompt, Completion Criteria, Available Variables

**New Structure**:
```
┌─────────────────────────────────┐
│ [Phase Name] Phase              │
│ phase_id                        │
│ [Built-in]                      │
├─────────────────────────────────┤
│ ▼ Phase Input                   │
│   Template variable preview     │
│   showing interpolated values   │
├─────────────────────────────────┤
│ ▼ Prompt                        │
│   Actual prompt content with    │
│   {{VARIABLE}} highlighting     │
├─────────────────────────────────┤
│ ▼ Completion Criteria           │
│   • Criterion 1                 │
│   • Criterion 2                 │
├─────────────────────────────────┤
│ ▼ Available Variables           │
│   [Phase Outputs]               │
│   {{SPEC}} {{TESTS}} {{REVIEW}} │
│   [Task Context]                │
│   {{TASK_ID}} {{TITLE}}         │
└─────────────────────────────────┘
```

**Key Changes**:
- Collapsible sections instead of tabs
- Show prompt content directly (not "No prompt configured")
- Variable badges are clickable (copy to clipboard)
- Better visual grouping

---

### 6. Consolidated Controls

**Current**: 3 control areas (React Flow Controls, MiniMap, CanvasToolbar)
**Target**: Single clean toolbar, minimap in corner

**Changes**:
- Remove React Flow's built-in `<Controls />` component
- Keep `CanvasToolbar` as sole control surface
- Style minimap to blend naturally in corner
- Add keyboard shortcuts hint

---

### 7. Edit/Save State Management

**Current**: Changes auto-save without indication
**Target**: Explicit dirty state with save/cancel options

**New Behavior**:
- Track changes in Zustand store
- Show "Unsaved changes" indicator when dirty
- "Save" and "Discard" buttons appear
- Confirm before navigating away with unsaved changes

---

### 8. Visual Polish

**Typography**:
- Phase names: `font-semibold`, `--text-lg`
- Phase IDs: `font-mono`, `--text-sm`, `--text-muted`
- Section headers: `font-medium`, `--text-base`, uppercase, letter-spacing

**Spacing**:
- Consistent `--space-3` for section padding
- `--space-2` for internal element gaps
- `--space-4` between major sections

**Colors**:
- Phase categories:
  - Specification: `--blue`
  - Implementation: `--green`
  - Quality/Review: `--orange`
  - Documentation: `--primary` (purple)
  - Other: `--text-muted`

**Animations**:
- Node hover: subtle scale (1.01) + shadow
- Selection: smooth border color transition
- Section expand/collapse: height animation
- Edge animations: dotted flow for active edges

---

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `layoutWorkflow.ts` | Modify | Remove start/end nodes, update edges |
| `StartEndNode.tsx` | Delete | No longer needed |
| `StartEndNode.css` | Delete | No longer needed |
| `nodes/index.ts` | Modify | Remove startEnd type |
| `PhaseNode.tsx` | Rewrite | New design with category colors |
| `PhaseNode.css` | Rewrite | New styling |
| `WorkflowCanvas.tsx` | Modify | Remove Controls, fix minimap |
| `WorkflowCanvas.css` | Modify | Minimap styling |
| `WorkflowEditorPage.tsx` | Modify | Add header component |
| `WorkflowEditorPage.css` | Modify | Header styling |
| `WorkflowHeader.tsx` | Create | New header component |
| `WorkflowHeader.css` | Create | Header styling |
| `PhaseInspector.tsx` | Rewrite | Section-based layout |
| `PhaseInspector.css` | Rewrite | New styling |
| `workflowEditorStore.ts` | Modify | Add dirty state tracking |

---

## Implementation Order

1. Remove Start/End nodes (foundational change)
2. Fix minimap (quick win, builds confidence)
3. Redesign PhaseNode (core visual improvement)
4. Add header with mode tabs
5. Redesign PhaseInspector
6. Clean up controls
7. Add edit/save state
8. Visual polish pass
