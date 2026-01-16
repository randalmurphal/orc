# Specification: Truncate task card descriptions on board to max 2-3 lines

## Problem Statement
Task cards on the Board page display full descriptions which can be excessively long, especially when containing markdown content with multiple paragraphs. The current CSS has `line-clamp: 2` but it's being undermined by `white-space: pre-wrap` which preserves newlines from markdown, causing cards to grow unbounded.

## Success Criteria
- [ ] Task card descriptions are truncated to exactly 3 lines maximum
- [ ] Ellipsis (...) appears at truncation point
- [ ] Full description is visible on hover via Tooltip component
- [ ] Card heights are consistent within the same column (no tall cards breaking layout)
- [ ] Markdown formatting (newlines, headers, lists) in descriptions is normalized to plain text for display
- [ ] Existing non-markdown descriptions continue to work correctly

## Testing Requirements
- [ ] Unit test: TaskCard renders description with line-clamp when description exceeds 3 lines
- [ ] Unit test: Tooltip shows full description text on hover
- [ ] E2E test: Board page shows truncated descriptions with consistent card heights
- [ ] E2E test: Hovering over description shows full content in tooltip

## Scope

### In Scope
- Fix `white-space: pre-wrap` conflict with `line-clamp` in TaskCard.css
- Increase line-clamp from 2 to 3 lines for better context
- Add Tooltip component around description to show full text on hover
- Normalize description text (strip markdown formatting) for card display

### Out of Scope
- Changing task list page (it doesn't show descriptions)
- Adding "expand" click functionality (tooltip on hover is sufficient)
- Rendering markdown as rich text on cards (too complex for card preview)
- Adding description preview to other components

## Technical Approach

### Files to Modify
- `web/src/components/board/TaskCard.tsx`: Wrap description in Tooltip, add text normalization utility
- `web/src/components/board/TaskCard.css`: Fix white-space conflict, update line-clamp to 3

### Implementation Details

1. **CSS Fix** (TaskCard.css):
   - Change `white-space: pre-wrap` to `white-space: normal` to allow proper line-clamp
   - Update `-webkit-line-clamp` from 2 to 3 lines
   - Add `word-break: break-word` to handle long words

2. **Tooltip Integration** (TaskCard.tsx):
   - Import and use existing Tooltip component (already in project)
   - Wrap `.task-description` paragraph in Tooltip
   - Pass full description as tooltip content

3. **Text Normalization**:
   - Create simple utility to strip markdown formatting:
     - Replace multiple newlines with single space
     - Strip heading markers (#, ##, etc.)
     - Strip list markers (-, *, 1.)
     - Strip bold/italic markers (**, __, *, _)
   - Apply before displaying in card (not in tooltip)

## Feature Analysis

### User Story
As a user viewing the task board, I want task descriptions to be truncated to a consistent height so that the board layout is usable and I can quickly scan tasks without scrolling through long descriptions.

### Acceptance Criteria
- [ ] Cards with long descriptions (e.g., TASK-220, TASK-218 mentioned in bug report) show max 3 lines
- [ ] Hovering over a truncated description reveals the full text
- [ ] Board columns display cards in a consistent, scannable layout
- [ ] Markdown in descriptions (headers, lists, bold) doesn't break the truncation
