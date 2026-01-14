# Phase 2: React - All Modal Components

## Overview

Port all modal/overlay components from Svelte 5 to React 19, following established patterns from Modal.tsx, ProjectSwitcher.tsx, and KeyboardShortcutsHelp.tsx.

## Scope

### Modals to Port (8 components)

| Component | Svelte Path | React Path | Complexity | Priority |
|-----------|-------------|------------|------------|----------|
| NewTaskModal | `web/src/lib/components/overlays/NewTaskModal.svelte` | `web-react/src/components/overlays/NewTaskModal.tsx` | High | P0 |
| NewInitiativeModal | `web/src/lib/components/overlays/NewInitiativeModal.svelte` | `web-react/src/components/overlays/NewInitiativeModal.tsx` | Low | P0 |
| CommandPalette | `web/src/lib/components/overlays/CommandPalette.svelte` | `web-react/src/components/overlays/CommandPalette.tsx` | High | P0 |
| LiveTranscriptModal | `web/src/lib/components/overlays/LiveTranscriptModal.svelte` | `web-react/src/components/overlays/LiveTranscriptModal.tsx` | High | P1 |
| FinalizeModal | `web/src/lib/components/overlays/FinalizeModal.svelte` | `web-react/src/components/overlays/FinalizeModal.tsx` | High | P1 |
| TaskEditModal | `web/src/lib/components/task/TaskEditModal.svelte` | `web-react/src/components/overlays/TaskEditModal.tsx` | Medium | P0 |
| AddDependencyModal | `web/src/lib/components/task/AddDependencyModal.svelte` | `web-react/src/components/overlays/AddDependencyModal.tsx` | Medium | P1 |
| ConfirmModal | (new) | `web-react/src/components/overlays/ConfirmModal.tsx` | Low | P0 |

**Note:** TaskEditModal already exists at `web-react/src/components/task-detail/TaskEditModal.tsx` but needs enhancement to match the Svelte version (missing weight selection with visual styles, category selection, priority visualization). It will be moved to overlays/ for consistency.

### Out of Scope

- Transcript.svelte (embedded component, not modal)
- Changes to Modal.tsx base component
- E2E test creation (handled by separate task)

## Technical Approach

### React Patterns to Follow

1. **Base Modal Usage**
   ```tsx
   import { Modal } from '@/components/overlays/Modal';

   <Modal open={isOpen} onClose={handleClose} title="Modal Title" size="md">
     {children}
   </Modal>
   ```

2. **State Management**
   - `useState` for local form state
   - `useCallback` for handlers
   - `useEffect` for side effects (WebSocket subscriptions, data loading)
   - Zustand stores for global state (`useProjectStore`, `useTaskStore`, `useInitiativeStore`)

3. **API Integration**
   - Import from `@/lib/api`
   - Use toast from `@/stores/uiStore` for notifications

4. **Keyboard Handling**
   - Cmd/Ctrl+Enter for form submission
   - Escape handled by base Modal component
   - Arrow keys for list navigation (CommandPalette)

5. **WebSocket Integration** (for LiveTranscriptModal, FinalizeModal)
   - Use `useWebSocket()` hook from `@/hooks/useWebSocket`
   - Subscribe to task-specific events via `on('all', callback)`
   - Clean up subscriptions in useEffect cleanup

### Component-Specific Requirements

#### 1. NewTaskModal.tsx
**Features:**
- Title input (required, autofocused)
- Description textarea (optional)
- Category selector (6 radio buttons with icons: feature, bug, refactor, chore, docs, test)
- Drag-and-drop file attachments
- File type filtering (images, PDF, text, markdown, JSON, logs)
- Attachment preview with thumbnails for images
- Cmd/Ctrl+Enter to submit
- Project-aware task creation via `useCurrentProjectId()`

**API:** `createTask()` or `createProjectTask()` with multipart form data

**Events:** `CustomEvent('orc:new-task')` trigger from any page

#### 2. NewInitiativeModal.tsx
**Features:**
- Title input (required, autofocused)
- Vision textarea (optional)
- Owner initials input (optional, max 5 chars)
- Cmd/Ctrl+Enter to submit
- Auto-select created initiative

**API:** `createNewInitiative()` from initiative store

#### 3. CommandPalette.tsx
**Features:**
- Search input with icon
- Categorized commands (Tasks, Navigation, Environment, Settings, Projects, View)
- Arrow key navigation (↑/↓)
- Enter to execute selected command
- Escape to close
- Search highlights matches in label/description
- Grouped results with category headers
- Footer with keyboard hints
- No results state

**Commands to implement:**
- Tasks: New Task (⇧⌥N)
- Navigation: Dashboard, Tasks, Board, Environment
- Environment: Skills, Hooks, Agents, Tools, MCP, Prompts, Scripts, Automation, Knowledge, Documentation
- Settings: Preferences
- Projects: Switch Project (⇧⌥P)
- View: Toggle Sidebar (⇧⌥B)

**Events:** `CustomEvent('orc:command-palette')` trigger, `goto()` → `useNavigate()`

#### 4. LiveTranscriptModal.tsx
**Features:**
- Header: task ID, status badge (colored), phase badge, connection status indicator
- Token counts display (input/output/cached)
- Embedded Transcript component (reuse from task-detail)
- Full view button → navigate to `/tasks/:id?tab=transcript`
- Loading/error states with retry
- Real-time streaming via WebSocket

**WebSocket Events:**
- `transcript`: chunk (streaming) and response (complete)
- `state`: task state updates
- `tokens`: incremental usage updates
- `phase`/`complete`: trigger reload

**API:** `getTranscripts()`, `getTaskState()` (project-aware variants)

#### 5. FinalizeModal.tsx
**Features:**
- Header: task ID, finalize status badge, connection indicator
- Progress section: step label, percentage, progress bar (animated)
- Result section (completed): commit SHA, target branch, files changed, conflicts resolved, tests status, risk level (colored)
- Error section (failed): error message, retry button
- Not started section: explanation text
- Footer: Close button, Start/Retry Finalize button

**States:** `not_started`, `pending`, `running`, `completed`, `failed`

**WebSocket Events:** `finalize` with status/step/progress/result/error

**API:** `triggerFinalize()`, `getFinalizeStatus()`

#### 6. TaskEditModal.tsx (Enhancement)
**Current:** Basic form with title, description, weight/priority/category/queue dropdowns
**Add:**
- Weight visual selection (5 radio buttons with color-coded borders)
- Category visual selection (6 options with icons)
- Priority visual selection (4 options with colored indicators)
- Queue toggle buttons (Active/Backlog)
- Change detection (`hasChanges` derived state)
- Keyboard hint (Cmd/Ctrl+Enter)

#### 7. AddDependencyModal.tsx
**Features:**
- Search box with icon
- Task list filtered by search query
- Exclude current task and already-selected dependencies
- Status icons (✓ completed, ● running, ○ pending)
- Status labels
- Single selection → calls `onSelect(taskId)`
- Loading/error/empty states

**Props:** `type: 'blocker' | 'related'`, `currentTaskId`, `existingBlockers`, `existingRelated`

**API:** `listTasks()` (handles both array and paginated response)

#### 8. ConfirmModal.tsx (New)
**Features:**
- Generic confirmation dialog
- Customizable title, message, confirm button text
- Danger variant for destructive actions
- Cancel/Confirm buttons
- Loading state during async confirm

**Props:**
```tsx
interface ConfirmModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
  title: string;
  message: string | ReactNode;
  confirmText?: string; // default: "Confirm"
  cancelText?: string;  // default: "Cancel"
  variant?: 'default' | 'danger';
  loading?: boolean;
}
```

## File Structure

```
web-react/src/components/overlays/
├── Modal.tsx                    # (existing)
├── Modal.css                    # (existing)
├── KeyboardShortcutsHelp.tsx    # (existing)
├── ProjectSwitcher.tsx          # (existing)
├── ProjectSwitcher.css          # (existing)
├── NewTaskModal.tsx             # NEW
├── NewTaskModal.css             # NEW
├── NewInitiativeModal.tsx       # NEW
├── NewInitiativeModal.css       # NEW
├── CommandPalette.tsx           # NEW
├── CommandPalette.css           # NEW
├── LiveTranscriptModal.tsx      # NEW
├── LiveTranscriptModal.css      # NEW
├── FinalizeModal.tsx            # NEW
├── FinalizeModal.css            # NEW
├── TaskEditModal.tsx            # MOVED from task-detail/
├── TaskEditModal.css            # MOVED from task-detail/
├── AddDependencyModal.tsx       # NEW
├── AddDependencyModal.css       # NEW
├── ConfirmModal.tsx             # NEW
├── ConfirmModal.css             # NEW
└── index.ts                     # UPDATE exports
```

## API Dependencies

### Existing (lib/api.ts)
- `createTask(title, description, weight?, category?, attachments?)`
- `createProjectTask(projectId, title, description, weight?, category?, attachments?)`
- `updateTask(id, updates)`
- `listTasks()` → returns `Task[]` or `PaginatedTasks`
- `getTranscripts(taskId)` / `getProjectTranscripts(projectId, taskId)`
- `getTaskState(taskId)` / `getProjectTaskState(projectId, taskId)`
- `triggerFinalize(taskId)`
- `getFinalizeStatus(taskId)`

### Store Functions
- `createNewInitiative()` from `@/stores/initiativeStore`
- `selectInitiative()` from `@/stores/initiativeStore`
- `addTask()` from `@/stores/taskStore`
- `toast.success/error/warning()` from `@/stores/uiStore`

## Type Additions (lib/types.ts)

```typescript
// May need to add/verify these types exist
interface FinalizeState {
  task_id: string;
  status: 'not_started' | 'pending' | 'running' | 'completed' | 'failed';
  step?: string;
  progress?: string;
  step_percent?: number;
  updated_at?: string;
  result?: FinalizeResult;
  error?: string;
}

interface FinalizeResult {
  commit_sha: string;
  target_branch: string;
  files_changed: number;
  conflicts_resolved: number;
  tests_passed: boolean;
  risk_level: 'low' | 'medium' | 'high';
}

interface TranscriptFile {
  phase: string;
  iteration: number;
  path: string;
  size: number;
  modified: string;
  content?: string;
}
```

## Success Criteria

### Functional Requirements
- [ ] All 8 modals render correctly with proper styling
- [ ] Focus trap works in all modals (Tab cycles within modal)
- [ ] Escape closes modals (via base Modal)
- [ ] Backdrop click closes modals
- [ ] Form validation works (required fields)
- [ ] API calls succeed with proper error handling
- [ ] Toast notifications display on success/error
- [ ] Keyboard shortcuts work (Cmd/Ctrl+Enter for forms)
- [ ] CommandPalette arrow navigation works
- [ ] WebSocket subscriptions connect and receive events
- [ ] File drag-drop works in NewTaskModal

### Visual Parity
- [ ] Modal sizes match Svelte (sm: 400px, md: 560px, lg: 720px, xl: 900px)
- [ ] Category/weight/priority visual selectors match Svelte styling
- [ ] Progress bars animate correctly
- [ ] Status badges use correct colors
- [ ] Connection status indicators display correctly

### Integration
- [ ] NewTaskModal triggers from Shift+Alt+N shortcut
- [ ] CommandPalette triggers from Shift+Alt+K shortcut
- [ ] Navigation commands work (goto → useNavigate)
- [ ] Project-aware API calls use correct project context
- [ ] WebSocket events update UI in real-time

## Testing Strategy

### Unit Tests (Vitest + Testing Library)

**Per modal:**
1. Renders with correct props
2. Form validation (required fields)
3. Submit handler called with correct data
4. Cancel closes modal
5. Error display on API failure
6. Loading states during async operations

**CommandPalette specific:**
- Keyboard navigation (up/down/enter/escape)
- Search filtering
- Category grouping

**LiveTranscriptModal/FinalizeModal specific:**
- WebSocket event handling (mock WebSocket)
- State updates from events
- Connection status display

### Integration Tests

1. **Form submission flow:**
   - Fill form → submit → API call → store update → modal closes → toast shown

2. **WebSocket integration:**
   - Open modal → subscribe → receive events → UI updates

### E2E Tests (Playwright)

Handled by separate E2E task, but modal tests should cover:
- Modal open/close via triggers
- Form submission
- Keyboard shortcuts
- Navigation after actions

## Implementation Order

### Phase 1: Core Modals (P0)
1. ConfirmModal (simple, dependency for others)
2. NewInitiativeModal (simple form)
3. TaskEditModal enhancement (builds on existing)
4. NewTaskModal (complex but core)
5. CommandPalette (complex but critical)

### Phase 2: WebSocket Modals (P1)
6. AddDependencyModal (simple list)
7. FinalizeModal (WebSocket)
8. LiveTranscriptModal (WebSocket + streaming)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Drag-drop file upload complexity | Use established pattern from Svelte, test thoroughly |
| WebSocket subscription cleanup | Use useEffect cleanup, test memory leaks |
| CommandPalette keyboard handling | Use ref for focus management, prevent default appropriately |
| Large CSS migration | Keep CSS modular, copy from Svelte with minimal changes |

## Notes

- Transcript.svelte component reuse: LiveTranscriptModal uses an embedded Transcript component. The React equivalent exists at `components/task-detail/TranscriptTab.tsx` - may need to extract a shared `Transcript` component or use TranscriptTab's content section.

- CSS variable consistency: All modals use CSS variables from the shared design system. No hardcoded colors.

- Platform detection: Use `@/lib/platform.ts` for `isMac()` and `getModifier()` utilities.
