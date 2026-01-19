# Specification: Create TasksBarChart component

## Problem Statement

The Statistics view needs a bar chart component to visualize tasks completed per day of the week. This component displays 7 bars (Mon-Sun) with heights proportional to task counts, matching the design from example_ui/stats.html.

## Success Criteria

- [ ] Component file exists at `web/src/components/stats/TasksBarChart.tsx`
- [ ] CSS file exists at `web/src/components/stats/TasksBarChart.css`
- [ ] Renders 7 bars, one for each day of the week (Mon-Sun)
- [ ] Bar specifications match reference:
  - Max width: 32px
  - Border-radius: 4px 4px 0 0 (rounded top corners only)
  - Color: var(--primary) (purple)
  - Height scales proportionally to max value in dataset
- [ ] Day labels (Mon, Tue, Wed, Thu, Fri, Sat, Sun) display below each bar
- [ ] Chart container has 160px height
- [ ] CSS-only implementation (no external charting library)
- [ ] Hover on bar shows exact count (via Tooltip component)
- [ ] Props interface: `data: { day: string; count: number }[]`
- [ ] `npm run typecheck` exits 0
- [ ] Zero values render with minimal visible height (4px minimum)
- [ ] Bars animate on data change (height transition)
- [ ] Component supports `className` prop for customization
- [ ] Component forwards ref correctly

## Testing Requirements

- [ ] Unit test: Renders 7 bar-group elements
- [ ] Unit test: Each bar-group contains a bar and label
- [ ] Unit test: Day labels display correctly (Mon-Sun)
- [ ] Unit test: Bar heights scale proportionally (tallest bar = 100% of container height)
- [ ] Unit test: Zero values render with minimum height
- [ ] Unit test: All-zero dataset renders all bars at minimum height
- [ ] Unit test: Hover shows tooltip with exact count
- [ ] Unit test: Handles empty data array gracefully
- [ ] Unit test: Forwards ref correctly
- [ ] Unit test: Accepts and applies className prop
- [ ] Unit test: Accessibility - has appropriate aria labels

## Scope

### In Scope
- TasksBarChart component and CSS
- Unit tests for component functionality
- Tooltip integration for hover counts
- CSS transitions for data change animation
- Loading skeleton state (consistent with StatsRow pattern)

### Out of Scope
- Data fetching (handled by parent component/statsStore)
- Chart title/header (parent component responsibility)
- Log scale or value capping (simple linear scaling sufficient for typical data)
- Responsive breakpoints (chart scales naturally with flex)

## Technical Approach

Create a pure CSS bar chart following the pattern from `example_ui/stats.html:424-456`. Use flexbox layout with `align-items: flex-end` to align bars to bottom. Bar heights calculated as percentage of max value.

### Files to Create

- `web/src/components/stats/TasksBarChart.tsx`:
  - Props interface with data array and optional className/loading
  - Calculate max value from data for scaling
  - Map data to bar groups with Tooltip wrapper
  - ForwardRef pattern matching StatsRow

- `web/src/components/stats/TasksBarChart.css`:
  - `.tasks-bar-chart` container: flexbox, 160px height, gap between bars
  - `.tasks-bar-chart-group`: flex column, centered items
  - `.tasks-bar-chart-bar`: max-width 32px, top border-radius, primary color, transition
  - `.tasks-bar-chart-label`: 9px font, muted text color
  - Loading skeleton styles
  - Reduced motion media query

- `web/src/components/stats/TasksBarChart.test.tsx`:
  - Test structure matching StatsRow.test.tsx patterns
  - Coverage for rendering, scaling, edge cases, accessibility

### Height Calculation Logic

```typescript
const maxCount = Math.max(...data.map(d => d.count), 1); // min 1 to avoid division by zero
const barHeight = count === 0 ? 4 : Math.max(4, (count / maxCount) * 140); // 140px max (leaving room for label)
```

## Feature Analysis

### User Story

As a user viewing the Statistics page, I want to see a bar chart showing tasks completed per day of the week, so that I can identify patterns in my productivity.

### Acceptance Criteria

1. **Visual Accuracy**: Chart matches reference design from example_ui/Screenshot_20260116_201852.png
2. **Data Binding**: Bars reflect provided data values accurately
3. **Interactivity**: Hovering shows exact count in tooltip
4. **Animation**: Bar heights animate smoothly when data changes
5. **Edge Cases**: Zero values and empty data handled gracefully
6. **Accessibility**: Screen readers can access chart data via aria labels
7. **Type Safety**: Full TypeScript coverage, typecheck passes
