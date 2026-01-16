# Testing

## Unit Tests (Vitest)

```bash
npm run test           # Run once
npm run test:watch     # Watch mode
npm run test:coverage  # Coverage report
```

Tests located in `src/**/*.test.ts(x)` and `src/integration/`.

**Integration tests** in `src/integration/`:
- `store-sync.test.ts` - Cross-store synchronization
- `websocket-events.test.ts` - WebSocket message handling

## E2E Tests (Playwright)

```bash
cd web && npx playwright test                    # Run all
cd web && npx playwright test specific.spec.ts  # Single file
cd web && npx playwright test --headed          # With browser UI
cd web && npx playwright test --ui              # Interactive mode
```

### CRITICAL: Sandbox Isolation

E2E tests run against an **ISOLATED SANDBOX PROJECT** in `/tmp`, NOT the real orc project. Tests perform real actions that modify task statuses.

**ALWAYS import from `./fixtures`, NOT from `@playwright/test`:**

```ts
// CORRECT - uses sandbox project
import { test, expect } from './fixtures';

// WRONG - will corrupt real data
import { test, expect } from '@playwright/test';
```

The fixture system (`web/e2e/fixtures.ts`) automatically:
- Sets `ORC_TEST_PROJECT` environment variable
- Points API requests to sandbox project
- Uses sandbox database and worktrees

### Test Files

| File | Coverage |
|------|----------|
| `board.spec.ts` | Board layout, columns, task cards |
| `task-list.spec.ts` | Task list filtering, navigation |
| `task-detail.spec.ts` | Task detail page, tabs, actions |
| `dashboard.spec.ts` | Dashboard stats, quick actions |
| `keyboard-shortcuts.spec.ts` | Global shortcuts, sequences |
| `mobile-responsive.spec.ts` | Responsive breakpoints |
| `finalize.spec.ts` | Finalize workflow, modal states |
| `ui-primitives.spec.ts` | Button, DropdownMenu, Select, Tabs, Tooltip |
| `radix-a11y.spec.ts` | Keyboard navigation, focus trap, ARIA |
| `axe-audit.spec.ts` | WCAG 2.1 AA compliance |

### Visual Regression Tests

Separate Playwright project for visual testing:

```bash
cd web && npx playwright test --project=visual                    # Compare against baselines
cd web && npx playwright test --project=visual --update-snapshots # Capture new baselines
```

**Configuration:**
- Viewport: 1440x900 @2x
- Animations: Disabled
- Dynamic content: Masked (timestamps, token counts)

**Baselines stored in:** `web/e2e/__snapshots__/`

Covered states: Dashboard, Board (flat/swimlane), Task Detail tabs, Modal dialogs.

### WebSocket Event Injection

Test real-time UI updates via Playwright's `routeWebSocket`:

```ts
test('updates on task event', async ({ page }) => {
  let ws: WebSocketRoute;
  await page.routeWebSocket('/api/ws', route => {
    ws = route.connectToServer();
    ws.onMessage(msg => ws.send(msg)); // Forward messages
  });

  // Inject event
  ws.send(JSON.stringify({
    event: 'task_updated',
    data: { id: 'TASK-001', status: 'completed' }
  }));

  await expect(page.locator('[data-task="TASK-001"]')).toHaveClass(/completed/);
});
```

### Test Configuration

`playwright.config.ts`:
- Workers: 4 (prevents OOM with parallel tasks)
- Base URL: `http://localhost:5173`
- Retries: 2 in CI
- Projects: chromium, firefox, webkit, visual

## Test Worker Limits

Playwright and Vitest limit parallelism to prevent OOM:
- Playwright: 4 workers
- Vitest: 4 threads

Without limits, parallel orc tasks running tests could spawn 48+ processes.
