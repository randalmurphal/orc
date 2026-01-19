# Specification: Create Pipeline component for task phase visualization

## Problem Statement

The Board view needs a horizontal Pipeline component to visualize task execution phases (Plan, Code, Test, Review, Done) with distinct visual states for completed, active, pending, and failed phases.

## Success Criteria

- [x] Pipeline component exists at `web/src/components/board/Pipeline.tsx`
- [x] Shows configurable phases (default: Plan, Code, Test, Review, Done)
- [x] Each phase displays: progress bar (4px height) + label below
- [x] Phase states implemented:
  - [x] Completed: solid bar in `var(--green)`, checkmark icon, green label
  - [x] Active: animated progress bar in `var(--primary)`, pulsing animation, bright label
  - [x] Pending: empty bar in `var(--overlay-light)`, muted label
  - [x] Failed: solid bar in `var(--red)`, X icon, red label
- [x] Active phase shows percentage completion when `progress` prop provided
- [x] Animation: smooth transitions, pulse animation on active phase
- [x] Horizontal layout with equal spacing (`flex: 1` per step, `gap: 3px`)
- [x] `npm run typecheck` exits 0
- [x] All unit tests pass (27 tests)

## Testing Requirements

- [x] Unit tests: `web/src/components/board/Pipeline.test.tsx` (27 tests)
  - [x] Renders all phases with correct labels
  - [x] Completed phases show green bar and checkmark icon
  - [x] Active phase shows primary color and animation class
  - [x] Progress percentage displayed when provided
  - [x] Pending phases show muted styling
  - [x] Failed phase shows red bar and X icon
  - [x] Case-insensitive phase matching
  - [x] Compact variant hides labels
  - [x] Accessibility: `role="progressbar"`, `aria-valuenow`, `aria-valuemax`, `aria-valuetext`
  - [x] Ref forwarding works correctly
  - [x] Custom className preserved
  - [x] HTML attributes passed through

## Scope

### In Scope

- Pipeline component with TypeScript interface
- CSS styling matching mockup specifications
- All phase states (pending, active, completed, failed)
- Progress percentage display for active phase
- Compact variant for mobile/constrained layouts
- Full accessibility support (ARIA attributes)
- Unit test coverage

### Out of Scope

- WebSocket integration (handled by parent RunningCard component)
- Phase change announcements for screen readers (deferred)
- E2E tests (visual testing in Playwright)

## Technical Approach

### Implementation Complete

The Pipeline component is fully implemented with:

1. **TypeScript Interface** (`PipelineProps`):
   - `phases: string[]` - Array of phase names
   - `currentPhase: string` - Currently active phase
   - `completedPhases: string[]` - List of completed phases
   - `failedPhase?: string` - Optional failed phase
   - `progress?: number` - 0-100 for current phase progress
   - `size?: "compact" | "default"` - Size variant

2. **Internal Phase State Computation**:
   - `computePhaseStates()` - Single-pass computation of phase statuses
   - Case-insensitive phase matching
   - Returns both phase states and completed count

3. **Accessibility**:
   - `role="progressbar"` on container
   - `aria-valuenow` = completed phase count
   - `aria-valuemax` = total phase count
   - `aria-valuetext` describing current state

4. **CSS Styling** (matches mockup exactly):
   - `.pipeline { display: flex; gap: 3px; }`
   - `.pipeline-bar { height: 4px; border-radius: 2px; }`
   - `@keyframes pipeline-progress-grow` for active animation
   - Compact variant hides labels

### Files

- `web/src/components/board/Pipeline.tsx` - Component implementation
- `web/src/components/board/Pipeline.css` - Styles
- `web/src/components/board/Pipeline.test.tsx` - 27 unit tests

## Feature Details

### User Story

As a user viewing the Board, I want to see a visual pipeline showing which phase each running task is in, so that I can quickly understand task progress at a glance.

### Acceptance Criteria

- [x] Pipeline renders horizontally with 5 equal-width columns
- [x] Each column has a 4px progress bar and 8px uppercase label
- [x] Completed phases show solid green bar with checkmark
- [x] Active phase shows animated purple bar (pulsing 50%-70% width)
- [x] Active phase with explicit progress shows static width (no animation)
- [x] Pending phases show empty bars with muted labels
- [x] Failed phase shows solid red bar with X icon
- [x] Compact mode hides labels for mobile views
- [x] Component accepts ref forwarding and custom className
