# spec - Iteration 1

## Prompt

Create a specification for this large task:

**Task**: Phase 2: React - All modal components

**Description**: ## Purpose
Port all modal/overlay components.

## Modals to Port

### NewTaskModal.svelte -> NewTaskModal.tsx
- Title input
- Description textarea
- Weight selector
- Category selector
- Priority selector
- Initiative selector
- Attachment upload
- Create button

### NewInitiativeModal.svelte -> NewInitiativeModal.tsx
- Title input
- Vision textarea
- Create button

### TaskEditModal.svelte -> TaskEditModal.tsx
- Edit all task fields inline
- Save/cancel buttons

### CommandPalette.svelte -> CommandPalette.tsx
- Search input
- Command sections (Tasks, Navigation, Environment, Settings, Projects, View)
- Keyboard navigation
- Fuzzy search

### KeyboardShortcutsHelp.svelte -> KeyboardShortcutsHelp.tsx
- Shortcut list organized by category
- Triggered by ? key

### LiveTranscriptModal.svelte -> LiveTranscriptModal.tsx
- Real-time Claude output streaming
- Connection status indicator
- Token counts
- Auto-scroll with manual override

### FinalizeModal.svelte -> FinalizeModal.tsx
- Explanation text
- Start button
- Progress bar with step labels
- Success/failure results
- Merge info display

### ConfirmModal.svelte -> ConfirmModal.tsx
- Generic confirmation dialog
- Customizable message and buttons

### AddDependencyModal.svelte -> AddDependencyModal.tsx
- Task search
- Add blocker/related functionality

## Success Criteria
- All modals render and function correctly
- Focus trap works in all modals
- Escape closes modals
- Keyboard shortcuts open correct modals

Define:
1. Requirements and scope
2. Technical approach
3. Component breakdown (backend, frontend if applicable)
4. API design (if applicable)
5. Success criteria with explicit checkboxes
6. Testing strategy:
   - Unit tests
   - Integration tests
   - E2E tests (using Playwright MCP tools if frontend exists)

Include clear completion criteria:
- What code must be written
- What tests must pass
- What E2E scenarios must work
- What documentation must exist

Keep iterating until the specification is clear and complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The spec is complete with:

1. **Clear scope** - 8 modal components with complexity/priority rankings
2. **Technical approach** - React patterns, state management, API integration, WebSocket handling
3. **Component-specific requirements** - Detailed features for each modal
4. **File structure** - Clear organization
5. **API dependencies** - What exists, what's needed
6. **Type additions** - FinalizeState, FinalizeResult, TranscriptFile
7. **Success criteria** - Explicit checkboxes for functional, visual, and integration requirements
8. **Testing strategy** - Unit, integration, and E2E approaches
9. **Implementation order** - Prioritized phases
10. **Risks and mitigations** - Drag-drop, WebSocket cleanup, keyboard handling, CSS migration

<phase_complete>true</phase_complete>

---
Tokens: 1220990 input, 6164 output, 206639 cache_creation, 991334 cache_read
Complete: true
Blocked: false
