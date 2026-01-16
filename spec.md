# Specification: Critical: New Task modal doesn't open - button click and keyboard shortcut both fail

## Problem Statement

The New Task modal never appears because the modal component doesn't exist - only TODO placeholder comments exist in `AppLayout.tsx` where `NewTaskModal` should be rendered. The state management for `showNewTaskModal` works correctly, but no modal component is actually rendered.

## Success Criteria

- [ ] NewTaskModal component exists at `web/src/components/overlays/NewTaskModal.tsx`
- [ ] NewTaskModal is exported from `web/src/components/overlays/index.ts`
- [ ] NewTaskModal is rendered in `AppLayout.tsx` with `open={showNewTaskModal}` prop
- [ ] Clicking "New Task" button in Header opens the modal
- [ ] Keyboard shortcut `Shift+Alt+N` opens the modal
- [ ] Modal can be closed via Escape key, close button, or clicking backdrop
- [ ] Form contains required field: Title (text input)
- [ ] Form contains optional fields: Description (textarea), Weight (select), Category (select)
- [ ] Submitting form calls `createProjectTask` API and creates a task
- [ ] After successful creation, modal closes and task appears in task list (via WebSocket refresh)
- [ ] Form validation prevents submission with empty title
- [ ] Error messages display when API call fails
- [ ] Loading state shown during submission

## Testing Requirements

- [ ] Unit test: NewTaskModal renders with all form fields
- [ ] Unit test: Form validation rejects empty title
- [ ] Unit test: Form submission calls onSubmit with correct data
- [ ] Unit test: Modal closes on successful submission
- [ ] E2E test: Click "New Task" button → modal opens → fill form → submit → task created

## Scope

### In Scope
- Create NewTaskModal component using existing Modal primitive (Radix Dialog)
- Wire up modal rendering in AppLayout.tsx
- Form with title (required), description, weight dropdown, category dropdown
- Form submission calling `createProjectTask` API
- Basic form validation and error handling
- Loading state during submission

### Out of Scope
- File attachment uploads (can be added later)
- Initiative assignment during creation (use edit after)
- Priority assignment during creation (use edit after)
- Advanced validation (title length limits, etc.)
- Draft/autosave functionality

## Technical Approach

### Files to Create
- `web/src/components/overlays/NewTaskModal.tsx`: Modal component with form
- `web/src/components/overlays/NewTaskModal.css`: Styles for the form

### Files to Modify
- `web/src/components/overlays/index.ts`: Export NewTaskModal
- `web/src/components/layout/AppLayout.tsx`: Render NewTaskModal with state

### Component Structure
```tsx
// NewTaskModal.tsx
interface NewTaskModalProps {
  open: boolean;
  onClose: () => void;
  projectId: string | null;
  onSuccess?: (task: Task) => void;
}

// Uses Modal component from ./Modal.tsx (Radix Dialog)
// Form state managed with useState
// Submission calls createProjectTask from @/lib/api
```

### Form Fields
| Field | Type | Required | Options |
|-------|------|----------|---------|
| title | text input | Yes | - |
| description | textarea | No | - |
| weight | select | No | trivial, small, medium, large, greenfield |
| category | select | No | feature, bug, refactor, chore, docs, test |

### API Integration
Uses existing `createProjectTask(projectId, title, description, weight, category)` from `@/lib/api.ts`

## Bug Analysis

### Reproduction Steps
1. Open the web UI at `http://localhost:8080` (or `:5173` in dev mode)
2. Select a project
3. Click the "New Task" button in the header, OR press `Shift+Alt+N`
4. Observe: Nothing happens (no modal appears)

### Current Behavior
- `setShowNewTaskModal(true)` is called (state updates correctly)
- No modal renders because the component doesn't exist
- Button shows active state briefly but no dialog appears

### Expected Behavior
- Modal should open with a form to create a new task
- User fills in title (required) and optionally description, weight, category
- Clicking "Create" submits the form and creates the task
- Modal closes and new task appears in the task list

### Root Cause
Located at `web/src/components/layout/AppLayout.tsx:83-84`:
```tsx
{/* TODO: NewTaskModal will be implemented in a future task */}
{/* TODO: CommandPalette will be implemented in a future task */}
```

The TODO comments indicate the modal was never implemented - only the state management exists.

### Verification
After fix:
1. Click "New Task" button → modal opens
2. Press `Shift+Alt+N` → modal opens
3. Fill in title "Test task" → click Create → task created, modal closes
4. New task visible in task list
