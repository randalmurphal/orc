# Quick Fix Guide - Statistics Page Issues

**For:** Developers implementing fixes for TASK-615 QA findings
**Priority:** QA-003 is BLOCKING deployment

---

## 🚨 QA-003: Most Modified Files Not Implemented [BLOCKING]

**Severity:** HIGH | **Effort:** 4-6 hours | **Priority:** P0

### Problem
```typescript
// web/src/stores/statsStore.ts:273
const topFiles: TopFile[] = []; // ← Always empty!
```

### What's Missing
1. Backend API endpoint
2. Frontend integration
3. Data display

### Backend Fix (Go)

**Create new RPC method in dashboard service:**

```protobuf
// internal/api/proto/orc/v1/dashboard.proto
message GetTopFilesRequest {
  int32 limit = 1; // default 4
  string period = 2; // "day", "week", "month", "all"
}

message TopFile {
  string path = 1;
  int32 modify_count = 2;
}

message GetTopFilesResponse {
  repeated TopFile files = 1;
}

service DashboardService {
  // ... existing methods ...
  rpc GetTopFiles(GetTopFilesRequest) returns (GetTopFilesResponse) {}
}
```

**Implement handler:**

```go
// internal/api/server.go or internal/dashboard/service.go
func (s *DashboardService) GetTopFiles(
	ctx context.Context,
	req *dashboardpb.GetTopFilesRequest,
) (*dashboardpb.GetTopFilesResponse, error) {
	// Query task_state table for file modification counts
	// Option 1: If you track file changes in task metadata
	files, err := s.storage.GetTopModifiedFiles(ctx, int(req.Limit))
	if err != nil {
		return nil, err
	}

	// Option 2: Parse git history (slower but more accurate)
	// files, err := s.git.GetTopModifiedFiles(ctx, req.Period, int(req.Limit))

	pbFiles := make([]*dashboardpb.TopFile, len(files))
	for i, f := range files {
		pbFiles[i] = &dashboardpb.TopFile{
			Path:        f.Path,
			ModifyCount: int32(f.ModifyCount),
		}
	}

	return &dashboardpb.GetTopFilesResponse{Files: pbFiles}, nil
}
```

**Database query (if using task metadata):**

```go
// internal/storage/stats.go (or wherever stats queries live)
func (s *Storage) GetTopModifiedFiles(ctx context.Context, limit int) ([]TopFile, error) {
	// This assumes file changes are tracked in task state or commits
	// Adjust query based on actual schema
	query := `
		SELECT
			filepath,
			COUNT(*) as modify_count
		FROM (
			-- Extract filepaths from task commits or metadata
			SELECT DISTINCT
				json_extract(change, '$.file') as filepath
			FROM task_commits
			WHERE json_valid(change)
			UNION ALL
			SELECT filepath FROM task_file_changes
		)
		WHERE filepath IS NOT NULL
		GROUP BY filepath
		ORDER BY modify_count DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query top files: %w", err)
	}
	defer rows.Close()

	var files []TopFile
	for rows.Next() {
		var f TopFile
		if err := rows.Scan(&f.Path, &f.ModifyCount); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, rows.Err()
}
```

### Frontend Fix (TypeScript)

**Update statsStore.ts:**

```typescript
// web/src/stores/statsStore.ts:210-227

// Add to proto imports at top
import { GetTopFilesRequestSchema } from '@/gen/orc/v1/dashboard_pb';

// In fetchStats(), line 210, add to Promise.all:
const [
	statsResponse,
	costResponse,
	dailyMetricsResponse,
	metricsResponse,
	topInitiativesResponse,
	comparisonResponse,
	topFilesResponse, // ← ADD THIS
] = await Promise.all([
	dashboardClient.getStats(createProto(GetStatsRequestSchema, {})),
	dashboardClient.getCostSummary(...),
	dashboardClient.getDailyMetrics(...),
	dashboardClient.getMetrics(...),
	dashboardClient.getTopInitiatives(...),
	dashboardClient.getComparison(...),
	dashboardClient.getTopFiles( // ← ADD THIS
		createProto(GetTopFilesRequestSchema, { limit: 4 })
	),
]);

// Replace line 273:
// OLD: const topFiles: TopFile[] = [];
// NEW:
const topFiles: TopFile[] = (topFilesResponse.files ?? []).map((file) => ({
	path: file.path,
	modifyCount: file.modifyCount,
}));
```

### Testing

```bash
# 1. Regenerate proto types
make proto

# 2. Test backend endpoint
curl http://localhost:8080/orc.v1.DashboardService/GetTopFiles \
  -H "Content-Type: application/json" \
  -d '{"limit": 4}'

# Expected: {"files":[{"path":"...","modifyCount":10},...]}"

# 3. Test frontend
cd web && bun run dev
# Navigate to /stats
# Scroll to "Most Modified Files" - should show data

# 4. E2E test
cd web && bunx playwright test stats.spec.ts
```

---

## ⚠️ QA-002: Touch Targets Too Small [NON-BLOCKING]

**Severity:** MEDIUM | **Effort:** 30 min | **Priority:** P1

### Problem
```css
/* web/src/components/stats/StatsView.css:69 */
.stats-view-time-btn {
	padding: 6px 12px; /* ← Results in ~25px height */
	font-size: 11px;
}
```

### Fix

**Update CSS (simple one-liner fix):**

```css
/* web/src/components/stats/StatsView.css:68-88 */
.stats-view-time-btn {
	min-height: 44px;        /* ← ADD: WCAG compliant */
	min-width: 44px;         /* ← ADD: WCAG compliant */
	padding: 12px 16px;      /* ← CHANGE: from 6px 12px */
	font-size: 12px;         /* ← CHANGE: from 11px */
	border-radius: var(--radius-sm);
	font-weight: var(--font-medium);
	color: var(--text-muted);
	background: none;
	border: none;
	cursor: pointer;
	transition: all var(--duration-fast);

	/* Ensure content is centered */
	display: flex;
	align-items: center;
	justify-content: center;
}

/* Ensure mobile compliance */
@media (max-width: 768px) {
	.stats-view-time-btn {
		min-height: 48px;    /* ← Even larger on mobile */
		min-width: 48px;
		padding: 14px 18px;
	}
}
```

### Why This Matters
- WCAG 2.1 Level AAA requires 44×44px minimum
- Users with motor impairments need larger targets
- Mobile users (especially older adults) struggle with small buttons
- iOS Human Interface Guidelines recommend 44pt minimum

### Testing

```bash
# Visual inspection
1. Open /stats in browser
2. Resize to 375×667 (mobile)
3. Inspect time filter buttons
4. Verify dimensions >= 44×44px

# Playwright test (add to stats.spec.ts)
test('time filter buttons meet WCAG touch target size', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 667 });

  const button = page.locator('.stats-view-time-btn').first();
  const box = await button.boundingBox();

  expect(box?.width).toBeGreaterThanOrEqual(44);
  expect(box?.height).toBeGreaterThanOrEqual(44);
});
```

---

## 🔍 QA-004: Heatmap Appears Sparse [INVESTIGATE]

**Severity:** MEDIUM | **Effort:** 2-3 hours | **Priority:** P2

### Problem
Heatmap shows only 6-7 cells instead of dense grid like reference design.

### Likely Cause
**Test environment has minimal task history.** This is probably NOT a bug.

### Investigation Steps

**1. Check with production data:**
```bash
# Point to production database (read-only)
export ORC_DB_PATH=/path/to/production/orc.db
./bin/orc serve

# Open /stats
# Does heatmap look dense now?
```

**2. Verify date range calculation:**
```typescript
// web/src/stores/statsStore.ts:193-207
// For 'all' period, should fetch 365 days
// For '30d', should fetch 30 days
// Check if dailyMetrics.days has correct count
console.log('Days fetched:', dailyMetricsResponse.stats?.days.length);
```

**3. Check ActivityHeatmap rendering:**
```typescript
// web/src/components/stats/ActivityHeatmap.tsx
// Verify:
// - Cells are being created for all dates
// - CSS isn't hiding cells
// - Date range spans 16 weeks (TASK-609)
```

**4. Mock dense data test:**
```typescript
// Add to stats.spec.ts
test('heatmap renders dense data correctly', async ({ page }) => {
	// Inject mock store with dense activity data
	await page.evaluate(() => {
		const { useStatsStore } = window as any;
		const store = useStatsStore.getState();

		// Generate 112 days of activity (16 weeks)
		const activityData = new Map();
		for (let i = 0; i < 112; i++) {
			const date = new Date();
			date.setDate(date.getDate() - i);
			activityData.set(
				date.toISOString().split('T')[0],
				Math.floor(Math.random() * 20) + 1
			);
		}

		store.activityData = activityData;
	});

	await page.waitForTimeout(500);

	// Count heatmap cells
	const cellCount = await page.locator('.activity-heatmap-cell').count();
	expect(cellCount).toBeGreaterThan(80); // Should have ~112 cells
});
```

### If It's Actually a Bug

**Check these common issues:**

1. **Off-by-one date calculation:**
   ```typescript
   // Ensure inclusive date range
   for (let i = 0; i <= 112; i++) { // ← Note <=
   ```

2. **CSS hiding cells:**
   ```css
   /* Ensure cells are visible */
   .activity-heatmap-cell {
     min-width: 10px;
     min-height: 10px;
     opacity: 1; /* Not accidentally hidden */
   }
   ```

3. **Data transformation bug:**
   ```typescript
   // Verify no data is lost in generateActivityData()
   console.log('tasksPerDay count:', tasksPerDay.length);
   console.log('activityData size:', activityData.size);
   // Should be equal
   ```

---

## Testing Checklist

### Before Submitting Fix

- [ ] **QA-003:** Backend endpoint returns data
- [ ] **QA-003:** Frontend displays data in table
- [ ] **QA-003:** Table shows correct format (file path + count)
- [ ] **QA-003:** No console errors
- [ ] **QA-002:** Touch targets measure >= 44×44px on mobile
- [ ] **QA-002:** Buttons still look good visually
- [ ] **QA-002:** No layout breaking on desktop
- [ ] All existing E2E tests pass
- [ ] No TypeScript errors
- [ ] No console warnings
- [ ] Export CSV still works

### E2E Test Commands

```bash
cd web

# Run all stats tests
bunx playwright test stats.spec.ts

# Run specific test
bunx playwright test stats.spec.ts -g "should display data tables"

# Debug mode (headed browser)
bunx playwright test stats.spec.ts --headed --debug

# Mobile viewport
bunx playwright test stats.spec.ts --device="iPhone SE"

# Update snapshots (if visual changed)
bunx playwright test stats.spec.ts --update-snapshots
```

---

## Files to Modify

### QA-003 (Most Modified Files)

**Backend:**
```
internal/api/proto/orc/v1/dashboard.proto  [ADD method]
internal/api/server.go                      [ADD handler]
internal/storage/stats.go                   [ADD query method]
```

**Frontend:**
```
web/src/stores/statsStore.ts               [MODIFY line 210, 273]
web/src/gen/orc/v1/dashboard_pb.ts         [REGENERATE]
```

### QA-002 (Touch Targets)

```
web/src/components/stats/StatsView.css     [MODIFY line 68-88]
```

### QA-004 (Heatmap Investigation)

```
web/src/components/stats/ActivityHeatmap.tsx  [VERIFY]
web/src/stores/statsStore.ts                  [VERIFY line 146-154]
```

---

## Common Gotchas

### QA-003
- ❌ Don't forget to regenerate proto types: `make proto`
- ❌ Don't use placeholder data - users will see it
- ❌ Don't forget error handling (empty results, API down)
- ✅ Handle case where git history doesn't exist
- ✅ Cache results (API might be slow)

### QA-002
- ❌ Don't just set `height: 44px` - use `min-height`
- ❌ Don't break desktop layout (test on 1280px too)
- ❌ Don't forget `:hover` and `:active` states
- ✅ Test with real touch device if possible
- ✅ Run accessibility audit (axe DevTools)

### QA-004
- ❌ Don't fix if it's just test data sparsity
- ❌ Don't break existing functionality
- ✅ Verify with production data first
- ✅ Add test to prevent regression

---

## Need Help?

**Review full QA reports:**
- `QA-REPORT-TASK-615.md` - Detailed technical analysis
- `QA-EXECUTIVE-SUMMARY.md` - High-level overview
- `QA-FINDINGS.json` - Machine-readable results

**Questions?**
- Check `/web/CLAUDE.md` for component architecture
- Check `/CLAUDE.md` for project conventions
- Review existing similar implementations (e.g., InitiativesTable)

---

**Priority Order:**
1. 🚨 Fix QA-003 (blocks deployment)
2. ⚠️ Fix QA-002 (accessibility issue)
3. 🔍 Investigate QA-004 (may not be a bug)

**Good luck!** 🚀
