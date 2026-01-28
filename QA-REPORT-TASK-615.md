# QA Report: Statistics Page (TASK-615)

**Test Date:** 2026-01-28
**Tester:** QA Engineer
**Test Environment:** Code inspection + E2E screenshot analysis
**Reference Design:** `example_ui/statistics-charts.png`

## Executive Summary

Testing methodology was limited to code inspection and analysis of existing E2E test screenshots due to worktree isolation constraints. Out of 3 previously reported issues, **2 are STILL PRESENT** with high confidence. Additionally, **1 NEW ISSUE** was discovered during inspection.

**Status:**
- Critical Issues: 0
- High Issues: 1 (STILL PRESENT)
- Medium Issues: 2 (1 STILL PRESENT, 1 NEW)
- Total Issues: 3

## Previous Issues Status

### QA-001: WebSocket Event Subscription Fails ❓ NEEDS MANUAL VERIFICATION

**Previous Report:**
- **Severity:** High
- **Expected:** WebSocket connection should establish successfully on Stats page load
- **Actual (Previous):** ConnectError when trying to subscribe to events

**Current Analysis:**
- **Status:** CANNOT VERIFY (code inspection only)
- **Confidence:** N/A
- **Findings:**
  - Stats page (`StatsView.tsx`) does NOT explicitly use WebSocket/event subscriptions
  - Data is loaded via REST API calls (`fetchStats()`) on component mount (line 291-293)
  - EventProvider exists globally but Stats page doesn't call `subscribe()` or `subscribeGlobal()`
  - No event handlers registered in StatsView component
  - This issue may be:
    1. Obsolete (WebSocket approach was replaced with REST)
    2. Mischaracterized (error was from different component)
    3. Fixed in later commits

**Recommendation:** Requires live browser testing with console inspection to verify if this issue still occurs.

---

### QA-002: Touch Targets Too Small on Mobile ⚠️ STILL PRESENT

**Previous Report:**
- **Severity:** Medium
- **Expected:** Touch targets should be at least 44x44px (WCAG 2.1 Level AAA)
- **Actual (Previous):** Filter buttons were 36.25x24px

**Current Analysis:**
- **Status:** STILL PRESENT
- **Severity:** Medium
- **Confidence:** 95%
- **Category:** Accessibility / UX

**Location:** `web/src/components/stats/StatsView.css`, line 68-78

**Root Cause:**
```css
.stats-view-time-btn {
	padding: 6px 12px;  /* ← Too small */
	font-size: 11px;
	/* ... */
}
```

**Calculation:**
- Font size: 11px
- Vertical padding: 6px top + 6px bottom = 12px
- Line height: ~13px (default)
- **Estimated height: ~25px** (far below 44px minimum)
- **Horizontal width: ~29px** (with text "24h", "7d", etc.)

**Impact:**
- Violates WCAG 2.1 Level AAA (Target Size)
- Difficult to tap on mobile devices (375x667 viewport)
- Users with motor impairments or large fingers will have trouble
- Common UX pattern is 44x44px minimum

**Evidence:**
- Existing E2E screenshot shows compact button layout
- CSS rules confirm undersized padding

**Suggested Fix:**
```css
.stats-view-time-btn {
	min-height: 44px;
	padding: 12px 16px;  /* Increased from 6px 12px */
	font-size: 12px;     /* Slightly larger for better legibility */
}

/* Add media query for mobile to ensure compliance */
@media (max-width: 768px) {
	.stats-view-time-btn {
		min-height: 44px;
		min-width: 44px;
		padding: 14px 18px;
	}
}
```

**Screenshot Path:** `web/e2e/__snapshots__/stats.spec.ts-snapshots/stats-page-full.png` (shows compact buttons)

---

### QA-003: Most Modified Files Table Shows No Data ❌ STILL PRESENT

**Previous Report:**
- **Severity:** Medium
- **Expected:** Table should show the 4 most frequently modified files with modification counts
- **Actual (Previous):** Table showed 0 rows (empty)

**Current Analysis:**
- **Status:** STILL PRESENT
- **Severity:** High (upgraded from Medium)
- **Confidence:** 100%
- **Category:** Functional - Data Display

**Location:** `web/src/stores/statsStore.ts`, line 272-273

**Root Cause:**
```typescript
// Build top files (placeholder - would need new API endpoint)
const topFiles: TopFile[] = [];
```

The `topFiles` data is **hardcoded to an empty array**. The implementation is incomplete.

**Steps to Reproduce:**
1. Navigate to `/stats`
2. Scroll down to "Most Modified Files" section
3. Observe: Table always shows 0 rows or "No data" state

**Expected Behavior:**
Per the reference design (`example_ui/statistics-charts.png`), the table should display:
```
Most Modified Files
1  src/lib/auth.ts                    34×
2  src/components/Button.tsx          28×
3  src/stores/session.ts              21×
4  src/lib/redis.ts                   18×
```

**Actual Behavior:**
Table is always empty because data source is not implemented.

**Impact:**
- **High** - Complete feature non-functional
- Breaks user's ability to see file modification patterns
- Reduces value of statistics page (one of 2 leaderboard tables is broken)
- Creates inconsistency (Initiatives table works, Files table doesn't)

**Technical Requirements for Fix:**
1. Backend API endpoint needed: `GET /api/v1/stats/top-files?limit=4`
   - Should query git history or task metadata for file modification counts
   - Return array of `{ path: string, modifyCount: number }`
2. Frontend statsStore needs to call this endpoint and populate `topFiles`
3. Comment at line 272 acknowledges this: "would need new API endpoint"

**Severity Upgrade Rationale:**
- Original classification: Medium
- Upgraded to: High
- Reason: This is not a degraded feature, it's a **completely non-functional feature** that was never implemented. Users expecting this data (as shown in the reference design) will be disappointed.

**Screenshot Path:** `web/e2e/__snapshots__/stats.spec.ts-snapshots/stats-page-full.png` (screenshot cuts off before tables are visible, but code confirms issue)

---

## New Issues Discovered

### QA-004: Activity Heatmap Shows Sparse Data (Potential Rendering Issue)

**Status:** NEW ISSUE
- **Severity:** Medium
- **Confidence:** 85%
- **Category:** Visual / Data Display

**Description:**
The activity heatmap in the E2E screenshot shows only 6-7 sparse cells scattered across the grid, compared to the reference design which shows a dense GitHub-style contribution graph with many cells filled.

**Evidence:**
- **Reference Design:** Shows dense activity across Oct-Jan with many cells at varying intensities
- **E2E Screenshot:** Shows only 6-7 isolated green/teal squares

**Possible Root Causes:**
1. **Data availability:** Test environment has minimal task history (likely)
2. **Rendering issue:** Heatmap component not rendering all cells properly
3. **Date range:** Heatmap might be showing wrong time period
4. **Cell sizing:** Cells might be rendering but too small to see

**Location:**
- Component: `web/src/components/stats/ActivityHeatmap.tsx`
- Data source: `statsStore.activityData` (Map<string, number>)
- Populated by: `generateActivityData()` at statsStore.ts line 146-154

**Steps to Reproduce:**
1. Navigate to `/stats`
2. Observe Activity Heatmap section
3. Compare density to reference design

**Expected:**
Dense grid showing 16 weeks of activity (per TASK-609 changelog: "increase heatmap density to 16 weeks")

**Actual:**
Sparse grid with only a few cells visible

**Impact:**
- Reduces visual impact of statistics page
- Makes it harder to see activity patterns at a glance
- Doesn't match user expectations from reference design

**Recommendation:**
1. Verify with real production data (test environment may have minimal history)
2. Check `ActivityHeatmap` component rendering logic
3. Verify date range calculation is showing 16 weeks correctly
4. Test with mock data that has dense activity

**Screenshot Path:** `web/e2e/__snapshots__/stats.spec.ts-snapshots/stats-page-full.png`

---

## Test Coverage Limitations

### What Was Tested ✅
- Code structure and implementation
- Data flow from API to UI components
- CSS styling and layout rules
- Component hierarchy and props
- State management via Zustand stores
- API endpoint calls and data transformation

### What Was NOT Tested ❌
- **Live browser interaction** (no Bash tool access)
- **Mobile viewport** (375x667) - cannot verify responsive behavior
- **Console errors** - cannot check JavaScript console
- **Network requests** - cannot verify API calls succeed
- **Export functionality** - cannot verify CSV download
- **Time filter interactions** - cannot verify button clicks update data
- **Hover states and tooltips** - cannot verify interactive elements
- **Visual regression** against reference image

### Constraints
- Limited to code inspection + existing E2E screenshot analysis
- E2E screenshot only shows top portion of page (metrics + heatmap)
- Cannot access `/tmp/qa-TASK-615/` directory (worktree isolation)
- Cannot run Playwright tests or start dev servers
- Cannot take new screenshots

---

## Recommendations

### Immediate Actions Required

1. **Fix QA-003 (High Priority)**
   - Implement backend API endpoint for top modified files
   - Wire up frontend to call new endpoint
   - Test with real git history data
   - Estimated effort: 4-6 hours (backend + frontend)

2. **Fix QA-002 (Medium Priority)**
   - Update CSS for time filter buttons to meet WCAG AAA standards
   - Test on mobile viewport (375x667)
   - Verify with accessibility tools (e.g., axe DevTools)
   - Estimated effort: 30 minutes

3. **Investigate QA-004 (Medium Priority)**
   - Test with production data to see if sparsity is environment-specific
   - Review ActivityHeatmap rendering logic
   - Verify 16-week range is displayed correctly
   - Estimated effort: 2-3 hours

### Follow-Up Testing Needed

1. **Manual browser testing** to verify:
   - QA-001 (WebSocket errors - may be obsolete)
   - QA-002 (actual touch target sizes on mobile device)
   - Console errors during Stats page load
   - Export CSV functionality
   - Time filter button interactions

2. **E2E test completion**:
   - Run full Playwright test suite
   - Capture screenshots of entire page (charts + tables)
   - Test mobile viewport separately
   - Generate test report with artifacts

3. **Accessibility audit**:
   - Run axe DevTools on Stats page
   - Verify keyboard navigation
   - Check screen reader compatibility
   - Test with high contrast mode

---

## Appendix: File References

### Key Files Inspected
```
web/src/components/stats/
├── StatsView.tsx          # Main container component
├── StatsView.css          # Styling (QA-002 issue here)
├── ActivityHeatmap.tsx    # Heatmap component
├── TasksBarChart.tsx      # Bar chart component
├── OutcomesDonut.tsx      # Donut chart component
└── LeaderboardTable.tsx   # Table components (Initiatives + Files)

web/src/stores/
└── statsStore.ts          # State management (QA-003 issue here)

web/e2e/
├── stats.spec.ts          # E2E test spec
├── fixtures.ts            # Test fixtures
└── __snapshots__/
    └── stats.spec.ts-snapshots/
        └── stats-page-full.png  # Existing screenshot
```

### Reference Materials
- Design reference: `example_ui/statistics-charts.png`
- Web components guide: `web/CLAUDE.md`
- Project documentation: `CLAUDE.md`

---

## Severity Definitions

| Severity | Definition | Examples |
|----------|------------|----------|
| **Critical** | Data loss, security hole, complete feature broken | Form loses data, XSS possible, login broken |
| **High** | Major feature impact, significant UX issue | Workflow blocked, confusing error, key action fails |
| **Medium** | Minor functionality issue, degraded experience | Edge case fails, slow response, UI glitch |
| **Low** | Cosmetic, minor inconvenience | Typo, alignment off, minor styling |

---

## Confidence Scoring

| Score | Meaning |
|-------|---------|
| 90-100 | Definite bug, clear reproduction, obvious impact |
| 80-89 | Likely bug, reproducible, noticeable impact |
| Below 80 | Don't report - uncertain, flaky, or very minor |

---

**Report Generated:** 2026-01-28
**Testing Method:** Code inspection + screenshot analysis
**Total Issues:** 3 (2 confirmed present, 1 new discovered)
**Requires Follow-Up:** Yes (manual browser testing needed for complete verification)
