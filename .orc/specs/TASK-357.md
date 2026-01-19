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

### E2E Tests (extend `web/e2e/stats.spec.ts`)

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
- Tooltip integration using Radix `Tooltip` from `@/components/ui/Tooltip`
- Loading/skeleton state
- Accessibility (ARIA labels, keyboard navigation)
- Responsive behavior (reduced weeks on smaller screens)
- Unit tests with Vitest
- E2E test coverage

### Out of Scope

- API endpoint implementation (`GET /api/stats/activity`) - data comes from `statsStore.activityData`
- Data caching logic (handled by statsStore)
- Stats page integration (separate task)
- Summary statistics below heatmap (separate component)

## Technical Approach

### Component Architecture

```
web/src/components/stats/
├── ActivityHeatmap.tsx       # Main component
├── ActivityHeatmap.css       # Styles
├── ActivityHeatmap.test.tsx  # Unit tests
└── index.ts                  # Barrel export
```

### TypeScript Interfaces

```typescript
// Props interface
export interface ActivityHeatmapProps {
  data: ActivityData[];
  weeks?: number;           // default 16
  loading?: boolean;        // show skeleton
  onDayClick?: (date: string, count: number) => void;
  className?: string;
}

// Activity data (matches statsStore.activityData Map entries)
export interface ActivityData {
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

### Grid Calculation Algorithm (Vanilla JS - No External Date Library)

```typescript
/**
 * Build heatmap grid using vanilla JavaScript Date operations.
 * No external date library required.
 */
function buildHeatmapGrid(data: ActivityData[], weeks: number): HeatmapCell[][] {
  const today = new Date();
  today.setHours(0, 0, 0, 0); // Normalize to midnight

  // Calculate start date: (weeks - 1) weeks ago, aligned to Sunday
  const startDate = new Date(today);
  startDate.setDate(startDate.getDate() - (weeks - 1) * 7);
  // Align to Sunday (day 0)
  const dayOffset = startDate.getDay();
  startDate.setDate(startDate.getDate() - dayOffset);

  // Create lookup map for O(1) access
  const dataMap = new Map(data.map(d => [d.date, d.count]));

  // Build grid: array of weeks, each week is array of 7 days
  const grid: HeatmapCell[][] = [];

  for (let w = 0; w < weeks; w++) {
    const week: HeatmapCell[] = [];
    for (let d = 0; d < 7; d++) {
      const date = new Date(startDate);
      date.setDate(startDate.getDate() + w * 7 + d);
      const dateStr = formatDateISO(date); // "YYYY-MM-DD"
      const isFuture = date > today;
      const count = isFuture ? 0 : (dataMap.get(dateStr) || 0);

      week.push({
        date: dateStr,
        count,
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

/** Format date as ISO string "YYYY-MM-DD" */
function formatDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/** Get intensity level from task count */
function getLevel(count: number): 0 | 1 | 2 | 3 | 4 {
  if (count === 0) return 0;
  if (count <= 3) return 1;
  if (count <= 6) return 2;
  if (count <= 9) return 3;
  return 4;
}

/** Format date for display "Jan 16, 2026" */
function formatDateDisplay(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

/** Format date for ARIA label "January 16, 2026" */
function formatDateAria(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });
}

/** Get month abbreviation from date */
function getMonthAbbrev(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString('en-US', { month: 'short' });
}
```

### Month Labels Calculation

```typescript
/**
 * Calculate month labels and their positions.
 * Returns array of { month: string, startWeek: number, span: number }
 */
function calculateMonthLabels(grid: HeatmapCell[][]): MonthLabel[] {
  const labels: MonthLabel[] = [];
  let currentMonth: string | null = null;
  let currentStartWeek = 0;

  for (let w = 0; w < grid.length; w++) {
    // Use the Sunday (first day) of each week to determine month
    const weekMonth = getMonthAbbrev(grid[w][0].date);

    if (weekMonth !== currentMonth) {
      if (currentMonth !== null) {
        labels.push({
          month: currentMonth,
          startWeek: currentStartWeek,
          span: w - currentStartWeek,
        });
      }
      currentMonth = weekMonth;
      currentStartWeek = w;
    }
  }

  // Add final month
  if (currentMonth !== null) {
    labels.push({
      month: currentMonth,
      startWeek: currentStartWeek,
      span: grid.length - currentStartWeek,
    });
  }

  return labels;
}
```

### CSS Design (from stats.html mockup - Purple Theme)

```css
/* =============================================================================
   ActivityHeatmap Component Styles
   GitHub-style activity heatmap with purple color scheme
   ============================================================================= */

/* Container */
.heatmap-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
}

.heatmap-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.heatmap-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

/* Main container */
.heatmap-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* Month labels row */
.heatmap-months {
  display: flex;
  gap: 2px;
  margin-left: 28px; /* align with grid after day labels */
  margin-bottom: 6px;
}

.heatmap-month {
  font-size: 9px;
  color: var(--text-muted);
  /* Width based on number of weeks spanned */
}

/* Grid layout: day labels + cells */
.heatmap-body {
  display: flex;
  gap: 12px;
}

/* Day labels column */
.heatmap-days {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-top: 2px;
}

.heatmap-day-label {
  font-size: 9px;
  color: var(--text-muted);
  height: 10px;
  line-height: 10px;
  width: 24px;
}

/* Grid: 7 rows (days), columns auto-flow (weeks) */
.heatmap-grid {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.heatmap-week {
  display: flex;
  gap: 2px;
}

/* Individual cells */
.heatmap-cell {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background: var(--bg-surface);
  cursor: pointer;
  transition: all 0.15s;
}

.heatmap-cell:hover {
  outline: 1px solid var(--text-muted);
  outline-offset: 1px;
}

.heatmap-cell:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}

.heatmap-cell.future {
  cursor: default;
  opacity: 0.3;
}

.heatmap-cell.future:hover {
  outline: none;
}

/* Intensity levels - Purple color scheme (matching mockup) */
.heatmap-cell.level-0 { background: var(--bg-surface); }
.heatmap-cell.level-1 { background: rgba(168, 85, 247, 0.2); }
.heatmap-cell.level-2 { background: rgba(168, 85, 247, 0.4); }
.heatmap-cell.level-3 { background: rgba(168, 85, 247, 0.6); }
.heatmap-cell.level-4 { background: var(--primary); }

/* Legend */
.heatmap-legend {
  display: flex;
  align-items: center;
  gap: 6px;
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

/* Skeleton loading state */
.heatmap-skeleton .heatmap-cell {
  background: linear-gradient(
    90deg,
    var(--bg-surface) 25%,
    var(--bg-hover) 50%,
    var(--bg-surface) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
  pointer-events: none;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* =============================================================================
   RESPONSIVE BREAKPOINTS
   ============================================================================= */

/* Tablet: 8 weeks */
@media (max-width: 768px) {
  .heatmap-container[data-responsive="true"] .heatmap-grid {
    /* Hide cells beyond 8 weeks */
  }
}

/* Mobile: 4 weeks */
@media (max-width: 480px) {
  .heatmap-container[data-responsive="true"] .heatmap-grid {
    /* Hide cells beyond 4 weeks */
  }
}

/* =============================================================================
   REDUCED MOTION
   ============================================================================= */

@media (prefers-reduced-motion: reduce) {
  .heatmap-cell {
    transition: none;
  }

  .heatmap-skeleton .heatmap-cell {
    animation: none;
    background: var(--bg-surface);
  }
}
```

### Component Structure

```tsx
import { useMemo, useCallback, useRef, type KeyboardEvent } from 'react';
import { Tooltip } from '@/components/ui/Tooltip';
import './ActivityHeatmap.css';

export function ActivityHeatmap({
  data,
  weeks = 16,
  loading = false,
  onDayClick,
  className = '',
}: ActivityHeatmapProps) {
  const gridRef = useRef<HTMLDivElement>(null);

  // Memoize grid calculation
  const grid = useMemo(
    () => buildHeatmapGrid(data, weeks),
    [data, weeks]
  );

  // Memoize month labels
  const monthLabels = useMemo(
    () => calculateMonthLabels(grid),
    [grid]
  );

  // Keyboard navigation handler
  const handleKeyDown = useCallback(
    (e: KeyboardEvent, cell: HeatmapCell, rowIndex: number, colIndex: number) => {
      if (cell.isFuture) return;

      let nextRow = rowIndex;
      let nextCol = colIndex;

      switch (e.key) {
        case 'ArrowUp':
          nextRow = Math.max(0, rowIndex - 1);
          e.preventDefault();
          break;
        case 'ArrowDown':
          nextRow = Math.min(6, rowIndex + 1);
          e.preventDefault();
          break;
        case 'ArrowLeft':
          nextCol = Math.max(0, colIndex - 1);
          e.preventDefault();
          break;
        case 'ArrowRight':
          nextCol = Math.min(weeks - 1, colIndex + 1);
          e.preventDefault();
          break;
        case 'Enter':
        case ' ':
          onDayClick?.(cell.date, cell.count);
          e.preventDefault();
          return;
        default:
          return;
      }

      // Focus next cell
      const nextCell = gridRef.current?.querySelector(
        `[data-week="${nextCol}"][data-day="${nextRow}"]`
      ) as HTMLElement | null;
      nextCell?.focus();
    },
    [weeks, onDayClick]
  );

  const containerClasses = [
    'heatmap-container',
    loading ? 'heatmap-skeleton' : '',
    className,
  ].filter(Boolean).join(' ');

  // Day labels: show Mon, Wed, Fri (indices 1, 3, 5)
  const dayLabels = ['', 'Mon', '', 'Wed', '', 'Fri', ''];

  return (
    <div
      className={containerClasses}
      role="img"
      aria-label={`Activity heatmap showing task completion over the last ${weeks} weeks`}
      data-responsive="true"
    >
      {/* Month labels */}
      <div className="heatmap-months" aria-hidden="true">
        {monthLabels.map((label, i) => (
          <span
            key={`${label.month}-${i}`}
            className="heatmap-month"
            style={{ width: `${label.span * 12 + (label.span - 1) * 2}px` }}
          >
            {label.month}
          </span>
        ))}
      </div>

      {/* Body: day labels + grid */}
      <div className="heatmap-body">
        {/* Day labels */}
        <div className="heatmap-days" aria-hidden="true">
          {dayLabels.map((label, i) => (
            <div key={i} className="heatmap-day-label">
              {label}
            </div>
          ))}
        </div>

        {/* Grid */}
        <div className="heatmap-grid" ref={gridRef} role="presentation">
          {/* Render rows (days) containing cells (weeks) */}
          {[0, 1, 2, 3, 4, 5, 6].map((dayIndex) => (
            <div key={dayIndex} className="heatmap-week">
              {grid.map((week, weekIndex) => {
                const cell = week[dayIndex];
                return (
                  <Tooltip
                    key={cell.date}
                    content={
                      <span>
                        <strong>{cell.count} tasks</strong> on {formatDateDisplay(cell.date)}
                      </span>
                    }
                    disabled={loading || cell.isFuture}
                    side="top"
                  >
                    <div
                      className={`heatmap-cell level-${cell.level}${cell.isFuture ? ' future' : ''}`}
                      data-week={weekIndex}
                      data-day={dayIndex}
                      tabIndex={cell.isFuture || loading ? -1 : 0}
                      aria-label={`${cell.count} tasks on ${formatDateAria(cell.date)}`}
                      onClick={() => !cell.isFuture && !loading && onDayClick?.(cell.date, cell.count)}
                      onKeyDown={(e) => handleKeyDown(e, cell, dayIndex, weekIndex)}
                    />
                  </Tooltip>
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Legend */}
      <div className="heatmap-legend">
        <span>Less</span>
        <div className="heatmap-legend-scale">
          <div className="heatmap-legend-cell level-0" />
          <div className="heatmap-legend-cell level-1" />
          <div className="heatmap-legend-cell level-2" />
          <div className="heatmap-legend-cell level-3" />
          <div className="heatmap-legend-cell level-4" />
        </div>
        <span>More</span>
      </div>
    </div>
  );
}
```

### Accessibility

- `role="img"` on container with descriptive `aria-label`
- Each cell has `aria-label` with task count and date
- `tabIndex={0}` on non-future cells for keyboard navigation
- Arrow keys navigate between cells
- Enter/Space triggers click action
- Focus ring visible on keyboard navigation
- Screen reader announces cell content on focus

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `ArrowUp` | Move to cell above |
| `ArrowDown` | Move to cell below |
| `ArrowLeft` | Move to previous week |
| `ArrowRight` | Move to next week |
| `Enter` / `Space` | Click cell (triggers `onDayClick`) |
| `Tab` | Move to next interactive element |

### Integration with statsStore

```tsx
// In parent component (e.g., StatsPage or ActivitySection):
import { useActivityData, useStatsLoading } from '@/stores/statsStore';

function StatsActivitySection() {
  const activityData = useActivityData();
  const loading = useStatsLoading();

  // Convert Map to array for ActivityHeatmap
  const data = useMemo(() => {
    return Array.from(activityData.entries()).map(([date, count]) => ({
      date,
      count,
    }));
  }, [activityData]);

  return (
    <ActivityHeatmap
      data={data}
      loading={loading}
      onDayClick={(date, count) => {
        console.log(`Clicked ${date} with ${count} tasks`);
        // Could filter tasks by date or navigate
      }}
    />
  );
}
```

### Dependencies

- `@radix-ui/react-tooltip` (already installed) - for tooltips
- No external date library required - uses vanilla JavaScript Date

### Files to Create

| File | Action | Purpose |
|------|--------|---------|
| `web/src/components/stats/ActivityHeatmap.tsx` | Create | Main component |
| `web/src/components/stats/ActivityHeatmap.css` | Create | Component styles |
| `web/src/components/stats/ActivityHeatmap.test.tsx` | Create | Unit tests |
| `web/src/components/stats/index.ts` | Create | Barrel export |

### Implementation Steps

1. Create `web/src/components/stats/` directory
2. Implement date utility functions (no external library)
3. Implement `buildHeatmapGrid()` function
4. Implement `calculateMonthLabels()` function
5. Create `ActivityHeatmap` component with:
   - Grid rendering using CSS flexbox
   - Day labels (blank, Mon, blank, Wed, blank, Fri, blank)
   - Month labels above grid with calculated widths
   - Cell hover tooltips using Radix `Tooltip`
   - Click handler for cells
   - Keyboard navigation
   - Loading skeleton state
6. Add CSS styles matching mockup (purple scheme)
7. Create barrel export `index.ts`
8. Write unit tests
9. Verify `npm run typecheck` passes

### Edge Cases

1. **Empty data array**: Render all cells as level-0
2. **Sparse data**: Fill missing dates with count=0
3. **Future dates**: Render as level-0, not clickable, reduced opacity
4. **Timezone handling**: Parse dates with `T00:00:00` suffix to avoid UTC shift
5. **First week partial**: Start aligns to Sunday, may include days before period
6. **Month labels positioning**: Calculate width based on weeks spanned
7. **Responsive weeks**: Use CSS to hide overflow weeks at breakpoints
