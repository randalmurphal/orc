# Phase 2: React - Task Detail Page with All Tabs

## Overview

Port the Task Detail page from Svelte to React 19, including all 6 tabs (Timeline, Changes, Transcript, Test Results, Attachments, Comments) and the Dependencies sidebar.

## Requirements

### Functional Requirements

1. **Task Detail Page** (`/tasks/:id`)
   - Load task data, state, plan, and transcripts
   - Tab navigation with URL persistence (`?tab=xxx`)
   - Real-time updates via WebSocket subscription
   - Connection status banner for running tasks

2. **TaskHeader Component**
   - Display: Task ID, weight badge, status indicator, category badge, initiative badge
   - Phase progress indicator (e.g., "2/6")
   - Actions: Edit (opens modal), Delete (with confirmation)
   - PR status display when PR exists

3. **DependencySidebar Component**
   - Collapsible sidebar (expand/collapse toggle)
   - Sections: Blocked by, Blocks (computed), Related, Referenced by (computed)
   - Status indicators for each dependency (completed/running/pending)
   - Add/remove blockers and related tasks inline
   - AddDependencyModal for searching/selecting tasks

4. **Timeline Tab** (default)
   - Horizontal phase execution flow
   - Phase status indicators (pending/running/completed/failed/skipped)
   - Duration display per phase
   - Token usage stats panel
   - Iteration/retry counts

5. **Changes Tab** (Diff Viewer)
   - Split/unified view toggle
   - File list with expand/collapse
   - Diff visualization with line numbers
   - Syntax highlighting by file type
   - Virtual scrolling for large diffs
   - Review comments with severity (blocker/issue/suggestion)
   - "Send to Agent" for retriggering with feedback

6. **Transcript Tab**
   - Paginated transcript history (10 per page)
   - Expandable content sections (prompt, response, retry context)
   - Token counts per turn (input/output/cached)
   - Live streaming content display
   - Export/copy functionality
   - Auto-scroll toggle

7. **Test Results Tab**
   - Test report summary (passed/failed/skipped)
   - Test suite breakdown
   - Screenshot gallery with lightbox
   - Trace viewer links
   - HTML report link

8. **Attachments Tab**
   - Drag-and-drop upload
   - Image gallery with lightbox
   - File list with metadata
   - Delete with confirmation

9. **Comments Tab**
   - General task discussion
   - Phase-scoped filtering
   - Threaded comments with replies
   - Author type indicators (human/agent/system)

### Non-Functional Requirements

- Match existing Svelte styling patterns (CSS modules)
- Use existing React patterns from web-react/
- Follow component structure conventions
- Maintain feature parity with Svelte implementation

## Technical Approach

### Component Architecture

```
pages/
  TaskDetail.tsx                    # Main page component

components/task/
  TaskHeader.tsx                    # Header with actions
  TaskEditModal.tsx                 # Edit form modal
  DependencySidebar.tsx             # Dependencies panel
  AddDependencyModal.tsx            # Task search/select
  TabNav.tsx                        # Tab navigation
  Timeline.tsx                      # Phase timeline
  TokenStats.tsx                    # Token usage panel
  PRActions.tsx                     # PR creation/status
  ExportPanel.tsx                   # Export options
  Attachments.tsx                   # Attachments tab
  TestResults.tsx                   # Test results tab

components/diff/
  DiffViewer.tsx                    # Main diff container
  DiffFile.tsx                      # File diff wrapper
  DiffHunk.tsx                      # Hunk with context
  DiffLine.tsx                      # Single line
  DiffStats.tsx                     # Summary stats
  InlineCommentThread.tsx           # Review comments
  VirtualScroller.tsx               # Performance optimization

components/transcript/
  Transcript.tsx                    # Main transcript view
  TranscriptEntry.tsx               # Single transcript file
  StreamingContent.tsx              # Live streaming display

components/comments/
  TaskCommentsPanel.tsx             # Comments container
  TaskCommentThread.tsx             # Thread with replies
  TaskCommentForm.tsx               # Comment creation
```

### State Management

Use existing stores:
- `useTaskStore` - Task data, execution state
- `useProjectStore` - Project context

Add to lib/types.ts:
- `TranscriptFile` interface
- `DiffResult`, `FileDiff`, `DiffHunk`, `DiffLine` interfaces
- `ReviewComment` interface
- `TaskComment` interface
- `Attachment` interface
- `TestResultsInfo` interface

### API Integration

Endpoints to integrate:
```
GET  /api/tasks/:id                      # Task metadata
GET  /api/tasks/:id/state                # Execution state
GET  /api/tasks/:id/plan                 # Phase plan
GET  /api/tasks/:id/transcripts          # Transcript files
GET  /api/tasks/:id/diff                 # Git diff
GET  /api/tasks/:id/diff/stats           # Diff stats only
GET  /api/tasks/:id/diff/file/:path      # Single file diff
GET  /api/tasks/:id/dependencies         # Dependency graph
GET  /api/tasks/:id/attachments          # Attachment list
POST /api/tasks/:id/attachments          # Upload attachment
GET  /api/tasks/:id/test-results         # Test results
GET  /api/tasks/:id/comments             # Task comments
POST /api/tasks/:id/comments             # Create comment
PATCH /api/tasks/:id                     # Update task
DELETE /api/tasks/:id                    # Delete task
POST /api/tasks/:id/run                  # Start task
POST /api/tasks/:id/pause                # Pause task
POST /api/tasks/:id/resume               # Resume task
POST /api/tasks/:id/finalize             # Trigger finalize
```

### WebSocket Integration

Use existing `useWebSocket` and `useTaskSubscription` hooks:
- Subscribe to task on mount
- Handle events: state, transcript, tokens, phase, complete, finalize
- Update local state from events
- Show connection status banner when disconnected

### URL State

Tab persistence pattern:
```tsx
const [searchParams, setSearchParams] = useSearchParams();
const activeTab = searchParams.get('tab') ?? 'timeline';

const handleTabChange = (tab: TabId) => {
  setSearchParams(prev => {
    prev.set('tab', tab);
    return prev;
  }, { replace: true });
};
```

### Styling

Follow existing patterns:
- Component-scoped CSS files
- CSS custom properties from existing theme
- BEM-like class naming
- Animation patterns for running/pulsing states

## Component Breakdown

### Phase 1: Core Page Structure (TaskDetail, TaskHeader, TabNav)

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| TaskDetail.tsx | 200-250 | TaskHeader, TabNav, DependencySidebar |
| TaskHeader.tsx | 150-200 | StatusIndicator, Icon, TaskEditModal |
| TabNav.tsx | 80-100 | Icon |
| TaskEditModal.tsx | 150-180 | Modal |

### Phase 2: Timeline Tab

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| Timeline.tsx | 150-200 | StatusIndicator |
| TokenStats.tsx | 80-100 | - |

### Phase 3: Transcript Tab

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| Transcript.tsx | 200-250 | TranscriptEntry, StreamingContent |
| TranscriptEntry.tsx | 100-120 | Icon |
| StreamingContent.tsx | 60-80 | - |

### Phase 4: Changes Tab (Diff Viewer)

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| DiffViewer.tsx | 200-250 | DiffFile, DiffStats |
| DiffFile.tsx | 100-120 | DiffHunk |
| DiffHunk.tsx | 80-100 | DiffLine |
| DiffLine.tsx | 50-60 | InlineCommentThread |
| DiffStats.tsx | 40-50 | - |
| InlineCommentThread.tsx | 120-150 | - |
| VirtualScroller.tsx | 100-120 | - |

### Phase 5: Remaining Tabs

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| TestResults.tsx | 150-180 | Icon |
| Attachments.tsx | 180-220 | Icon, Modal |
| TaskCommentsPanel.tsx | 150-180 | TaskCommentThread, TaskCommentForm |
| TaskCommentThread.tsx | 80-100 | - |
| TaskCommentForm.tsx | 60-80 | - |

### Phase 6: Dependencies Sidebar

| Component | Lines Est. | Dependencies |
|-----------|------------|--------------|
| DependencySidebar.tsx | 200-250 | AddDependencyModal, StatusIndicator |
| AddDependencyModal.tsx | 150-180 | Modal, Icon |

## API Design

No new backend APIs required - all endpoints exist. Frontend uses:

```typescript
// lib/api.ts additions
export async function getTask(id: string): Promise<Task>;
export async function getTaskState(id: string): Promise<TaskState>;
export async function getTaskPlan(id: string): Promise<Plan>;
export async function getTranscripts(id: string): Promise<TranscriptFile[]>;
export async function getTaskDiff(id: string, options?: { files?: boolean; base?: string }): Promise<DiffResult>;
export async function getTaskDiffStats(id: string): Promise<DiffStats>;
export async function getTaskFileDiff(id: string, path: string): Promise<FileDiff>;
export async function getTaskDependencies(id: string): Promise<DependencyGraph>;
export async function addBlocker(taskId: string, blockerId: string): Promise<Task>;
export async function removeBlocker(taskId: string, blockerId: string): Promise<Task>;
export async function addRelated(taskId: string, relatedId: string): Promise<Task>;
export async function removeRelated(taskId: string, relatedId: string): Promise<Task>;
export async function getAttachments(id: string): Promise<Attachment[]>;
export async function uploadAttachment(id: string, file: File): Promise<Attachment>;
export async function deleteAttachment(id: string, filename: string): Promise<void>;
export async function getTestResults(id: string): Promise<TestResultsInfo>;
export async function getTaskComments(id: string, options?: { author_type?: string; phase?: string }): Promise<TaskComment[]>;
export async function createTaskComment(id: string, data: CreateCommentInput): Promise<TaskComment>;
export async function updateTaskComment(taskId: string, commentId: string, data: UpdateCommentInput): Promise<TaskComment>;
export async function deleteTaskComment(taskId: string, commentId: string): Promise<void>;
export async function updateTask(id: string, updates: TaskUpdateInput): Promise<Task>;
export async function deleteTask(id: string): Promise<void>;
export async function runTask(id: string, options?: { force?: boolean }): Promise<RunTaskResponse>;
export async function pauseTask(id: string): Promise<void>;
export async function resumeTask(id: string): Promise<void>;
export async function triggerFinalize(id: string): Promise<FinalizeResponse>;
export async function getFinalizeStatus(id: string): Promise<FinalizeStatus>;
```

## Success Criteria

### Must Have

- [ ] TaskDetail page loads and displays task data
- [ ] Tab navigation works with URL persistence
- [ ] Timeline tab shows phase progression with status
- [ ] Changes tab displays git diff with file list
- [ ] Transcript tab shows paginated history
- [ ] Test Results tab shows test summary
- [ ] Attachments tab lists files with upload capability
- [ ] Comments tab shows threaded discussions
- [ ] DependencySidebar shows all 4 relationship types
- [ ] WebSocket subscription updates UI in real-time
- [ ] Task actions (run/pause/resume/delete) work
- [ ] Edit modal updates task fields
- [ ] Connection status banner appears when disconnected

### Should Have

- [ ] Diff viewer split/unified toggle
- [ ] Virtual scrolling for large diffs
- [ ] Live streaming content in Transcript tab
- [ ] Screenshot lightbox in Test Results
- [ ] Drag-drop upload in Attachments
- [ ] Review comment "Send to Agent" functionality

### Nice to Have

- [ ] Keyboard navigation within diff viewer
- [ ] Syntax highlighting in diff viewer
- [ ] Comment filtering by phase
- [ ] Export functionality

## Testing Strategy

### Unit Tests (Vitest)

| Component | Test Coverage |
|-----------|---------------|
| TaskHeader | Actions trigger callbacks, badge rendering |
| TabNav | Tab selection, keyboard navigation, ARIA |
| Timeline | Phase status display, duration formatting |
| TokenStats | Token count formatting, cache display |
| DiffViewer | View mode toggle, file expansion |
| DiffLine | Line type styling, click handlers |
| Transcript | Pagination, content expansion |
| DependencySidebar | Section rendering, add/remove handlers |

### Integration Tests

1. **TaskDetail data flow**
   - Task data loads from API
   - State updates from WebSocket
   - Tab content lazy loads

2. **Form interactions**
   - Edit modal saves changes
   - Delete confirms and navigates
   - Attachment upload works

3. **WebSocket integration**
   - Subscribes on mount
   - Handles disconnect/reconnect
   - Updates task state from events

### E2E Tests (Playwright)

```typescript
// Task detail navigation and tabs
test('navigates to task detail and switches tabs', async ({ page }) => {
  await page.goto('/tasks/TASK-001');
  await expect(page.getByRole('heading', { name: /TASK-001/ })).toBeVisible();

  // Check all tabs accessible
  await page.getByRole('tab', { name: 'Changes' }).click();
  await expect(page.url()).toContain('tab=changes');

  await page.getByRole('tab', { name: 'Transcript' }).click();
  await expect(page.url()).toContain('tab=transcript');
});

// Task actions
test('runs and pauses task', async ({ page }) => {
  await page.goto('/tasks/TASK-001');
  await page.getByRole('button', { name: 'Run' }).click();
  await expect(page.getByText('running')).toBeVisible();

  await page.getByRole('button', { name: 'Pause' }).click();
  await expect(page.getByText('paused')).toBeVisible();
});

// Dependencies sidebar
test('adds and removes blocker', async ({ page }) => {
  await page.goto('/tasks/TASK-003');
  await page.getByRole('button', { name: 'Add blocker' }).click();
  await page.getByRole('textbox').fill('TASK-001');
  await page.getByRole('option', { name: /TASK-001/ }).click();
  await expect(page.getByText('TASK-001')).toBeVisible();
});

// Edit modal
test('edits task via modal', async ({ page }) => {
  await page.goto('/tasks/TASK-001');
  await page.getByRole('button', { name: 'Edit' }).click();
  await page.getByLabel('Title').fill('Updated Title');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByRole('heading', { name: /Updated Title/ })).toBeVisible();
});

// Diff viewer
test('toggles diff view mode', async ({ page }) => {
  await page.goto('/tasks/TASK-001?tab=changes');
  await page.getByRole('button', { name: 'Unified' }).click();
  await expect(page.locator('.diff-viewer.unified')).toBeVisible();

  await page.getByRole('button', { name: 'Split' }).click();
  await expect(page.locator('.diff-viewer.split')).toBeVisible();
});

// Attachments
test('uploads attachment', async ({ page }) => {
  await page.goto('/tasks/TASK-001?tab=attachments');
  const fileInput = page.locator('input[type="file"]');
  await fileInput.setInputFiles({
    name: 'test.png',
    mimeType: 'image/png',
    buffer: Buffer.from('fake image data')
  });
  await expect(page.getByText('test.png')).toBeVisible();
});
```

### Test File Mapping

| Component | Test File |
|-----------|-----------|
| TaskDetail.tsx | TaskDetail.test.tsx |
| TaskHeader.tsx | TaskHeader.test.tsx |
| TabNav.tsx | TabNav.test.tsx |
| Timeline.tsx | Timeline.test.tsx |
| DiffViewer.tsx | DiffViewer.test.tsx |
| Transcript.tsx | Transcript.test.tsx |
| DependencySidebar.tsx | DependencySidebar.test.tsx |
| Attachments.tsx | Attachments.test.tsx |
| TestResults.tsx | TestResults.test.tsx |
| TaskCommentsPanel.tsx | TaskCommentsPanel.test.tsx |

## Documentation Requirements

1. Update `web-react/CLAUDE.md`:
   - Add TaskDetail page section
   - Document all new components
   - Add API functions to lib/api.ts section
   - Update component mapping table

2. Component JSDoc headers:
   - Purpose and features
   - Props interface
   - Usage examples

## Implementation Order

1. **Core structure** (3-4 hours)
   - TaskDetail.tsx (page shell)
   - TaskHeader.tsx
   - TabNav.tsx
   - TaskEditModal.tsx

2. **Timeline Tab** (2 hours)
   - Timeline.tsx
   - TokenStats.tsx

3. **Transcript Tab** (2-3 hours)
   - Transcript.tsx
   - TranscriptEntry.tsx
   - StreamingContent.tsx

4. **Changes Tab** (4-5 hours)
   - DiffViewer.tsx
   - DiffFile.tsx
   - DiffHunk.tsx
   - DiffLine.tsx
   - DiffStats.tsx
   - InlineCommentThread.tsx
   - VirtualScroller.tsx

5. **Remaining Tabs** (3-4 hours)
   - TestResults.tsx
   - Attachments.tsx
   - TaskCommentsPanel.tsx
   - TaskCommentThread.tsx
   - TaskCommentForm.tsx

6. **Dependencies Sidebar** (2-3 hours)
   - DependencySidebar.tsx
   - AddDependencyModal.tsx

7. **Testing & Polish** (3-4 hours)
   - Unit tests
   - E2E tests
   - Documentation updates

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Diff viewer performance with large diffs | Implement virtual scrolling, lazy-load file hunks |
| WebSocket reconnection complexity | Use existing hooks, tested patterns |
| Tab state desync with URL | Use searchParams as source of truth |
| Streaming content accumulation | Clear buffer on phase/iteration change |

## Dependencies

- Existing: Modal, Icon, StatusIndicator, toast
- API client functions (to be added)
- WebSocket hooks (useWebSocket, useTaskSubscription)
- CSS custom properties (existing theme)

## Out of Scope

- PR creation flow (separate task)
- Finalize modal (already exists in board)
- Live transcript modal (already exists)
- Task creation (separate task)

---

**Estimated Total Effort:** 20-25 hours

**Files to Create:** ~25 component files + ~10 test files + CSS files

**Lines of Code:** ~3000-4000 (components) + ~1500 (tests)
