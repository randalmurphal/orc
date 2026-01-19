# Specification: Create ActivityHeatmap Component (GitHub-style)

## Intent

Provide a visual representation of task activity over time in a GitHub-style contribution heatmap. This enables users to quickly understand their task completion patterns, identify productive periods, and track engagement trends over the past 16 weeks.

## Success Criteria

- [ ] `ActivityHeatmap` component created at `web/src/components/stats/ActivityHeatmap.tsx`
- [ ] CSS styles created at `web/src/components/stats/ActivityHeatmap.css`
- [ ] Grid displays 16 weeks x 7 days (112 cells) with weeks as columns
- [ ] Day labels (Mon, Wed, Fri) visible on left side
- [ ] Month labels displayed on top, aligned to week columns
- [ ] Color intensity reflects task count using 5 distinct levels (0-4)
- [ ] Tooltip appears on hover showing date and task count
- [ ] Legend displays "Less" to "More" with intensity scale
- [ ] Click handler calls `onDayClick` with date and count
- [ ] Skeleton loading state while data fetches
- [ ] `npm run typecheck` exits with code 0
- [ ] Unit tests pass: `npm run test -- ActivityHeatmap`
- [ ] Component renders correctly in Stats page

## Testing Requirements

### Unit Tests (`web/src/components/stats/ActivityHeatmap.test.tsx`)

- [ ] Renders 112 cells (16 weeks × 7 days)
- [ ] Applies correct level class (level-0 through level-4) based on count
- [ ] Shows day labels for Mon, Wed, Fri
- [ ] Displays month labels correctly
- [ ] Shows tooltip with date and count on hover
- [ ] Calls `onDayClick` when cell is clicked with correct parameters
- [ ] Handles empty data array (shows all level-0)
- [ ] Handles missing days in data (fills with level-0)
- [ ] Does not render future dates as clickable
- [ ] Renders skeleton state when `loading={true}`
- [ ] Applies custom className when provided
- [ ] Has correct ARIA attributes for accessibility
- [ ] Keyboard navigation works with arrow keys

### E2E Tests (extend `web/e2e/dashboard.spec.ts` or new `stats.spec.ts`)

- [ ] Heatmap is visible on stats page
- [ ] Hovering over cell shows tooltip
- [ ] Clicking cell triggers navigation/filter (if implemented)
- [ ] Legend is visible with correct labels
- [ ] Responsive: shows 8 weeks below 768px viewport
- [ ] Responsive: shows 4 weeks below 480px viewport

## Scope

### In Scope

- `ActivityHeatmap` React component with TypeScript interfaces
- CSS styling matching the example_ui mockup (purple color scheme)
- Date grid calculation (16 weeks aligned to Sunday start)
- Level threshold calculation from task counts
- Tooltip component integration (using existing `Tooltip` from `@/components/ui/Tooltip`)
- Loading/skeleton state
- Accessibility (ARIA labels, keyboard navigation)
- Responsive behavior (reduced weeks on smaller screens)
- Unit tests with Vitest
- E2E test coverage

### Out of Scope

- API endpoint implementation (`GET /api/stats/activity`) - assumed to exist or be mocked
- Data caching logic (handled by parent component)
- Stats page integration (separate task)
- Summary statistics below heatmap (separate component)

## Technical Approach

### Component Architecture

```
ActivityHeatmap/
├── ActivityHeatmap.tsx    # Main component
├── ActivityHeatmap.css    # Styles
└── ActivityHeatmap.test.tsx # Tests
```

### TypeScript Interfaces

```typescript
// Props interface
interface ActivityHeatmapProps {
  data: ActivityData[];
  weeks?: number;           // default 16
  loading?: boolean;        // show skeleton
  onDayClick?: (date: string, count: number) => void;
  className?: string;
}

// Activity data from API
interface ActivityData {
  date: string;             // "2026-01-16" format (ISO date)
  count: number;            // task count for that day
}

// Internal cell representation
interface HeatmapCell {
  date: string;
  count: number;
  level: 0 | 1 | 2 | 3 | 4;
  dayOfWeek: number;        // 0=Sunday, 6=Saturday
  weekIndex: number;        // 0 to weeks-1
  isFuture: boolean;
}

// Level thresholds
const LEVEL_THRESHOLDS = {
  0: 0,      // No activity
  1: 1,      // 1-3 tasks
  2: 4,      // 4-6 tasks
  3: 7,      // 7-9 tasks
  4: 10,     // 10+ tasks
};
```

### Grid Calculation Algorithm

```typescript
function buildHeatmapGrid(data: ActivityData[], weeks: number): HeatmapCell[][] {
  const today = new Date();
  // Align start to Sunday of (weeks - 1) weeks ago
  const startDate = startOfWeek(subWeeks(today, weeks - 1), { weekStartsOn: 0 });

  // Create lookup map for O(1) access
  const dataMap = new Map(data.map(d => [d.date, d.count]));

  // Build grid: array of weeks, each week is array of 7 days
  const grid: HeatmapCell[][] = [];

  for (let w = 0; w < weeks; w++) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < 7; d++) {
      const date = addDays(startDate, w * 7 + d);
      const dateStr = format(date, 'yyyy-MM-dd');
      const count = dataMap.get(dateStr) || 0;
      const isFuture = isAfter(date, today);

      week.push({
        date: dateStr,
        count: isFuture ? 0 : count,
        level: isFuture ? 0 : getLevel(count),
        dayOfWeek: d,
        weekIndex: w,
        isFuture,
      });
    }
    grid.push(week);
  }
  return grid;
}

function getLevel(count: number): 0 | 1 | 2 | 3 | 4 {
  if (count === 0) return 0;
  if (count <= 3) return 1;
  if (count <= 6) return 2;
  if (count <= 9) return 3;
  return 4;
}
```

### CSS Design (from mockup)

```css
/* Container */
.heatmap-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* Grid: 7 rows (days), columns flow horizontally (weeks) */
.heatmap-grid {
  display: grid;
  grid-template-rows: repeat(7, 1fr);
  grid-auto-flow: column;
  gap: 3px;
}

/* Individual cells */
.heatmap-cell {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  cursor: pointer;
  transition: transform 0.1s ease;
}

.heatmap-cell:hover {
  transform: scale(1.2);
}

.heatmap-cell.future {
  cursor: default;
  opacity: 0.3;
}

/* Intensity levels - using purple (primary) color scheme */
.heatmap-cell.level-0 { background: var(--bg-surface); }
.heatmap-cell.level-1 { background: rgba(168, 85, 247, 0.2); }
.heatmap-cell.level-2 { background: rgba(168, 85, 247, 0.4); }
.heatmap-cell.level-3 { background: rgba(168, 85, 247, 0.6); }
.heatmap-cell.level-4 { background: var(--primary); }

/* Day labels */
.heatmap-day-labels {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding-top: 2px;
}

.heatmap-day-label {
  font-size: 9px;
  color: var(--text-muted);
  height: 12px;
  line-height: 12px;
  width: 24px;
}

/* Month labels */
.heatmap-month-labels {
  display: flex;
  gap: 2px;
  margin-left: 28px; /* align with grid after day labels */
  margin-bottom: 6px;
}

.heatmap-month-label {
  font-size: 9px;
  color: var(--text-muted);
  /* Width spans ~4 weeks */
}

/* Legend */
.heatmap-legend {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: var(--text-muted);
}

.heatmap-legend-scale {
  display: flex;
  gap: 2px;
}

.heatmap-legend-cell {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}
```

### Responsive Breakpoints

```css
/* Tablet: 8 weeks */
@media (max-width: 768px) {
  .heatmap-container[data-responsive="true"] {
    --heatmap-weeks: 8;
  }
}

/* Mobile: 4 weeks */
@media (max-width: 480px) {
  .heatmap-container[data-responsive="true"] {
    --heatmap-weeks: 4;
  }
}
```

### Accessibility

```tsx
<div
  role="img"
  aria-label="Activity heatmap showing task completion over the last 16 weeks"
  className="heatmap-container"
>
  {/* Grid */}
  <div className="heatmap-grid" role="presentation">
    {cells.map(cell => (
      <div
        key={cell.date}
        className={`heatmap-cell level-${cell.level}`}
        aria-label={`${cell.count} tasks on ${formatDateForAria(cell.date)}`}
        tabIndex={cell.isFuture ? -1 : 0}
        onClick={() => !cell.isFuture && onDayClick?.(cell.date, cell.count)}
        onKeyDown={(e) => handleKeyNavigation(e, cell)}
      />
    ))}
  </div>
</div>
```

### Keyboard Navigation

- Arrow keys navigate between cells
- Enter/Space triggers click on focused cell
- Focus wraps at grid boundaries

### Dependencies

- `date-fns` for date manipulation (already in project)
- `@/components/ui/Tooltip` for hover tooltips

### Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `web/src/components/stats/ActivityHeatmap.tsx` | Create | Main component |
| `web/src/components/stats/ActivityHeatmap.css` | Create | Component styles |
| `web/src/components/stats/ActivityHeatmap.test.tsx` | Create | Unit tests |
| `web/src/components/stats/index.ts` | Create | Barrel export |

### Implementation Steps

1. Create `web/src/components/stats/` directory structure
2. Implement `buildHeatmapGrid()` utility function
3. Implement `ActivityHeatmap` component with:
   - Grid rendering with CSS grid
   - Day labels (Sun hidden, Mon/Wed/Fri visible)
   - Month labels above grid
   - Cell hover tooltips using `Tooltip` component
   - Click handler
   - Loading skeleton state
4. Add CSS styles matching mockup design
5. Implement accessibility (ARIA, keyboard nav)
6. Add responsive behavior via CSS media queries
7. Write unit tests covering all scenarios
8. Verify typecheck passes

### Edge Cases to Handle

1. **Empty data array**: Render all cells as level-0
2. **Sparse data**: Fill missing dates with count=0
3. **Future dates**: Render as level-0, not clickable, reduced opacity
4. **Timezone handling**: Use UTC dates for consistency with API
5. **First week partial**: Start week may not begin on the exact day
6. **Month labels positioning**: Calculate which weeks each month spans

### Date Library Usage

Using `date-fns` functions:
- `startOfWeek(date, { weekStartsOn: 0 })` - Get Sunday of week
- `subWeeks(date, n)` - Go back n weeks
- `addDays(date, n)` - Add days
- `format(date, 'yyyy-MM-dd')` - ISO date string
- `format(date, 'MMM')` - Month abbreviation
- `isAfter(date1, date2)` - Compare dates
- `getDay(date)` - Day of week (0-6)
