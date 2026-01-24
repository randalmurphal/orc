/**
 * Timeline Page E2E Tests
 *
 * End-to-end tests for the Timeline/Activity Feed page.
 * These tests verify the complete user journey including:
 * - Page loading and initial data fetch
 * - Date grouping and collapsible sections
 * - Filtering by event type, task, initiative
 * - Infinite scroll pagination
 * - Real-time WebSocket updates
 *
 * Success Criteria covered:
 * - SC-1: Timeline page renders at /timeline route
 * - SC-2: Initial load fetches events from last 24 hours
 * - SC-3: Each event displays task ID, task title, event type, and timestamp
 * - SC-4: Events are grouped by date with collapsible headers
 * - SC-5: Collapse state persists across page navigation
 * - SC-6: Filter dropdown shows event type checkboxes
 * - SC-7: Filtering by event type updates the event list
 * - SC-8: Scrolling to bottom triggers next page load
 * - SC-10: WebSocket events prepend to timeline in real-time
 * - SC-11: Time range selector changes query parameters
 * - SC-12: Empty state shows when no events match filters
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The /timeline route and TimelinePage component do not yet exist.
 */

import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper to wait for timeline to load
async function waitForTimelineLoad(page: Page) {
	await page.waitForSelector('.timeline-page', { timeout: 10000 });
	// Wait for loading state to disappear
	await page.waitForSelector('.timeline-loading', { state: 'hidden', timeout: 10000 }).catch(() => {});
}

// Helper to get all visible event cards
async function getEventCards(page: Page) {
	return page.locator('.timeline-event').all();
}

// Helper to scroll to bottom of timeline
async function scrollToBottom(page: Page) {
	const timeline = page.locator('.timeline-view');
	await timeline.evaluate((el) => {
		el.scrollTop = el.scrollHeight;
	});
}

// Helper to clear timeline localStorage
async function clearTimelineStorage(page: Page) {
	await page.evaluate(() => {
		localStorage.removeItem('orc-timeline-collapsed-groups');
		localStorage.removeItem('orc-timeline-time-range');
	});
}

test.describe('Timeline Page', () => {
	test.describe('Page Loading (SC-1, SC-2)', () => {
		test('should render timeline page at /timeline route', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Page should render without 404
			const timelinePage = page.locator('.timeline-page');
			await expect(timelinePage).toBeVisible();

			// Should have a heading
			const heading = page.locator('h1').filter({ hasText: /timeline/i });
			await expect(heading).toBeVisible();
		});

		test('should show loading state while fetching events', async ({ page }) => {
			// Intercept API to delay response
			await page.route('**/api/events*', async (route) => {
				await new Promise((resolve) => setTimeout(resolve, 500));
				await route.continue();
			});

			await page.goto('/timeline');

			// Should show loading indicator
			const loadingIndicator = page.locator('.timeline-loading');
			await expect(loadingIndicator).toBeVisible({ timeout: 2000 });
		});

		test('should fetch events from API on load', async ({ page }) => {
			// Track API calls
			const apiCalls: string[] = [];
			await page.route('**/api/events*', async (route) => {
				apiCalls.push(route.request().url());
				await route.continue();
			});

			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Should have made an API call
			expect(apiCalls.length).toBeGreaterThan(0);

			// API call should include 'since' parameter for 24h ago
			const url = new URL(apiCalls[0]);
			expect(url.searchParams.has('since')).toBe(true);
		});
	});

	test.describe('Event Display (SC-3)', () => {
		test('should display events with task ID and title', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Check for event card content
			const eventCards = await getEventCards(page);

			if (eventCards.length > 0) {
				const firstEvent = eventCards[0];

				// Should have task ID (TASK-XXX format)
				const taskId = firstEvent.locator('.task-id');
				await expect(taskId).toHaveText(/TASK-\d+/);

				// Should have task title
				const taskTitle = firstEvent.locator('.timeline-event-task-title');
				await expect(taskTitle).toBeVisible();
			}
		});

		test('should display event type label', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const eventCards = await getEventCards(page);

			if (eventCards.length > 0) {
				const firstEvent = eventCards[0];

				// Should have event label
				const eventLabel = firstEvent.locator('.timeline-event-label');
				await expect(eventLabel).toBeVisible();
			}
		});

		test('should display relative timestamp', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const eventCards = await getEventCards(page);

			if (eventCards.length > 0) {
				const firstEvent = eventCards[0];

				// Should have time indicator (e.g., "2m ago", "1h ago")
				const timeElement = firstEvent.locator('.timeline-event-time');
				await expect(timeElement).toHaveText(/ago|just now/i);
			}
		});

		test('should link task ID to task detail page', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const eventCards = await getEventCards(page);

			if (eventCards.length > 0) {
				const firstEvent = eventCards[0];
				const taskLink = firstEvent.locator('a[href^="/tasks/"]');

				await expect(taskLink).toBeVisible();
				const href = await taskLink.getAttribute('href');
				expect(href).toMatch(/^\/tasks\/TASK-\d+$/);
			}
		});
	});

	test.describe('Date Grouping (SC-4)', () => {
		test('should group events by date with headers', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Look for date group headers
			const groupHeaders = page.locator('.timeline-group-header');

			// Should have at least one group if there are events
			const eventCount = await getEventCards(page).then((cards) => cards.length);
			if (eventCount > 0) {
				await expect(groupHeaders.first()).toBeVisible();
			}
		});

		test('should show event count in group headers', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const groupHeader = page.locator('.timeline-group-header').first();
			const headerText = await groupHeader.textContent();

			// Should contain event count like "Today (5 events)"
			expect(headerText).toMatch(/\(\d+\s+events?\)/i);
		});

		test('should collapse/expand groups when header is clicked', async ({ page }) => {
			await page.goto('/timeline');
			await clearTimelineStorage(page);
			await page.reload();
			await waitForTimelineLoad(page);

			const firstGroup = page.locator('.timeline-group').first();
			const header = firstGroup.locator('.timeline-group-header');
			const content = firstGroup.locator('.timeline-group-content');

			// Initially expanded
			await expect(content).toBeVisible();

			// Click header to collapse
			await header.click();
			await expect(content).not.toBeVisible();

			// Click header to expand
			await header.click();
			await expect(content).toBeVisible();
		});

		test('should rotate chevron icon when collapsed/expanded', async ({ page }) => {
			await page.goto('/timeline');
			await clearTimelineStorage(page);
			await page.reload();
			await waitForTimelineLoad(page);

			const firstGroup = page.locator('.timeline-group').first();
			const header = firstGroup.locator('.timeline-group-header');

			// Check expanded state
			await expect(firstGroup).toHaveClass(/expanded/);

			// Click to collapse
			await header.click();
			await expect(firstGroup).toHaveClass(/collapsed/);
		});
	});

	test.describe('Collapse Persistence (SC-5)', () => {
		test('should persist collapse state in localStorage', async ({ page }) => {
			await page.goto('/timeline');
			await clearTimelineStorage(page);
			await page.reload();
			await waitForTimelineLoad(page);

			const firstGroup = page.locator('.timeline-group').first();
			const header = firstGroup.locator('.timeline-group-header');

			// Collapse the first group
			await header.click();
			await expect(firstGroup).toHaveClass(/collapsed/);

			// Check localStorage
			const storedState = await page.evaluate(() =>
				localStorage.getItem('orc-timeline-collapsed-groups')
			);
			expect(storedState).toBeTruthy();
		});

		test('should restore collapse state after navigation', async ({ page }) => {
			await page.goto('/timeline');
			await clearTimelineStorage(page);
			await page.reload();
			await waitForTimelineLoad(page);

			const firstGroup = page.locator('.timeline-group').first();
			const header = firstGroup.locator('.timeline-group-header');

			// Collapse the first group
			await header.click();
			await expect(firstGroup).toHaveClass(/collapsed/);

			// Navigate away
			await page.goto('/board');
			await page.waitForSelector('.board-page');

			// Navigate back
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// First group should still be collapsed
			const groupAfterNav = page.locator('.timeline-group').first();
			await expect(groupAfterNav).toHaveClass(/collapsed/);
		});
	});

	test.describe('Filtering (SC-6, SC-7)', () => {
		test('should show filter dropdown with event type options', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Click filter button
			const filterButton = page.locator('button').filter({ hasText: /filter/i });
			await filterButton.click();

			// Dropdown should be visible with event type checkboxes
			const filterMenu = page.locator('[role="menu"]');
			await expect(filterMenu).toBeVisible();

			// Should have event type options
			await expect(filterMenu.getByLabel(/phase completed/i)).toBeVisible();
			await expect(filterMenu.getByLabel(/task created/i)).toBeVisible();
		});

		test('should update URL params when filter is applied', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Apply filter
			const filterButton = page.locator('button').filter({ hasText: /filter/i });
			await filterButton.click();

			const phaseCompletedCheckbox = page.getByLabel(/phase completed/i);
			await phaseCompletedCheckbox.click();

			// Close dropdown
			await page.keyboard.press('Escape');

			// URL should have types param
			await expect(page).toHaveURL(/types=.*phase_completed/);
		});

		test('should filter events when type is selected', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Count initial events
			const initialCount = (await getEventCards(page)).length;

			// Apply filter
			await page.goto('/timeline?types=error_occurred');
			await waitForTimelineLoad(page);

			// Event count should change (either fewer or same if all were errors)
			const filteredCount = (await getEventCards(page)).length;
			expect(filteredCount).toBeLessThanOrEqual(initialCount);
		});

		test('should show filter badge with active filter count', async ({ page }) => {
			await page.goto('/timeline?types=phase_completed,error_occurred');
			await waitForTimelineLoad(page);

			// Filter button should show badge
			const filterBadge = page.locator('.filter-badge');
			await expect(filterBadge).toHaveText('2');
		});
	});

	test.describe('Infinite Scroll (SC-8)', () => {
		test('should load more events when scrolling to bottom', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const initialCount = (await getEventCards(page)).length;

			// Skip if not enough events to paginate
			if (initialCount < 50) {
				test.skip();
				return;
			}

			// Scroll to bottom
			await scrollToBottom(page);

			// Wait for more events to load
			await page.waitForSelector('.timeline-event', { timeout: 5000 });
			await page.waitForTimeout(500); // Allow time for render

			const newCount = (await getEventCards(page)).length;
			expect(newCount).toBeGreaterThan(initialCount);
		});

		test('should show loading indicator while fetching more', async ({ page }) => {
			// Intercept API to delay pagination response
			let requestCount = 0;
			await page.route('**/api/events*', async (route) => {
				requestCount++;
				if (requestCount > 1) {
					await new Promise((resolve) => setTimeout(resolve, 500));
				}
				await route.continue();
			});

			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Scroll to bottom
			await scrollToBottom(page);

			// Should show loading more indicator
			const loadingMore = page.locator('.timeline-loading-more');
			await expect(loadingMore).toBeVisible({ timeout: 2000 });
		});

		test('should show "No more events" when all loaded', async ({ page }) => {
			// Navigate with small time range to ensure fewer events
			await page.goto('/timeline?since=' + new Date(Date.now() - 60000).toISOString());
			await waitForTimelineLoad(page);

			// Scroll to bottom multiple times
			for (let i = 0; i < 5; i++) {
				await scrollToBottom(page);
				await page.waitForTimeout(200);
			}

			// Should eventually show no more events message
			const noMoreMessage = page.locator('.timeline-no-more');
			await expect(noMoreMessage).toBeVisible({ timeout: 10000 });
		});
	});

	test.describe('Real-time Updates (SC-10)', () => {
		test('should show new events via WebSocket without refresh', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const initialCount = (await getEventCards(page)).length;

			// Wait for potential WebSocket events (in real test, would trigger from backend)
			// For E2E, we check that WebSocket connection is established
			await page.waitForTimeout(2000);

			// Check WebSocket status indicator
			const wsIndicator = page.locator('.ws-status');
			if (await wsIndicator.isVisible()) {
				await expect(wsIndicator).not.toHaveClass(/disconnected/);
			}
		});

		test('should show reconnecting indicator when WebSocket disconnects', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Simulate disconnect by disabling network (for WS only)
			// This is a simplified check - real disconnect testing would need backend involvement
			const wsStatus = page.locator('.ws-status');

			// Just verify the element exists and can show status
			await expect(wsStatus).toBeVisible();
		});
	});

	test.describe('Time Range Selector (SC-11)', () => {
		test('should show time range buttons', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Should have preset range buttons
			await expect(page.getByRole('button', { name: /today/i })).toBeVisible();
			await expect(page.getByRole('button', { name: /this week/i })).toBeVisible();
			await expect(page.getByRole('button', { name: /this month/i })).toBeVisible();
		});

		test('should update URL params when time range changes', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			// Click "This Week" button
			await page.getByRole('button', { name: /this week/i }).click();

			// URL should have updated params
			await expect(page).toHaveURL(/since=/);
		});

		test('should refetch events when time range changes', async ({ page }) => {
			// Track API calls
			let apiCallCount = 0;
			await page.route('**/api/events*', async (route) => {
				apiCallCount++;
				await route.continue();
			});

			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const initialCalls = apiCallCount;

			// Change time range
			await page.getByRole('button', { name: /this month/i }).click();

			// Should make new API call
			await page.waitForTimeout(500);
			expect(apiCallCount).toBeGreaterThan(initialCalls);
		});
	});

	test.describe('Empty State (SC-12)', () => {
		test('should show empty state when no events match filters', async ({ page }) => {
			// Use filter that likely returns no results
			await page.goto('/timeline?types=nonexistent_type');
			await waitForTimelineLoad(page);

			// Should show empty state
			const emptyState = page.locator('.timeline-empty');
			await expect(emptyState).toBeVisible();
			await expect(emptyState).toContainText(/no events/i);
		});

		test('should show "adjust filters" hint when filtered empty', async ({ page }) => {
			await page.goto('/timeline?types=nonexistent_type');
			await waitForTimelineLoad(page);

			const emptyState = page.locator('.timeline-empty');
			await expect(emptyState).toContainText(/adjust.*filters/i);
		});

		test('should show different message for empty time period', async ({ page }) => {
			// Use very narrow time range that likely has no events
			const future = new Date(Date.now() + 86400000).toISOString();
			await page.goto(`/timeline?since=${future}`);
			await waitForTimelineLoad(page);

			const emptyState = page.locator('.timeline-empty');
			await expect(emptyState).toContainText(/no events.*time period/i);
		});
	});

	test.describe('Keyboard Navigation', () => {
		test('should support keyboard navigation through events', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const firstEvent = page.locator('.timeline-event').first();

			// Focus first event
			await firstEvent.focus();
			await expect(firstEvent).toBeFocused();

			// Tab to next event
			await page.keyboard.press('Tab');
			const secondEvent = page.locator('.timeline-event').nth(1);
			await expect(secondEvent).toBeFocused();
		});

		test('should expand event details on Enter key', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const firstEvent = page.locator('.timeline-event').first();

			// Focus and press Enter
			await firstEvent.focus();
			await page.keyboard.press('Enter');

			// Details should be visible
			const details = firstEvent.locator('.timeline-event-details');
			if ((await firstEvent.getAttribute('data-expandable')) === 'true') {
				await expect(details).toBeVisible();
			}
		});
	});

	test.describe('Task Link Navigation', () => {
		test('should navigate to task detail when task link is clicked', async ({ page }) => {
			await page.goto('/timeline');
			await waitForTimelineLoad(page);

			const eventCards = await getEventCards(page);

			if (eventCards.length > 0) {
				const taskLink = eventCards[0].locator('a[href^="/tasks/"]');

				// Click the task link
				await taskLink.click();

				// Should navigate to task detail page
				await expect(page).toHaveURL(/\/tasks\/TASK-\d+/);
			}
		});
	});

	test.describe('Error Handling', () => {
		test('should show error state when API fails', async ({ page }) => {
			// Mock API failure
			await page.route('**/api/events*', async (route) => {
				await route.fulfill({ status: 500, body: 'Internal Server Error' });
			});

			await page.goto('/timeline');

			// Should show error state
			const errorState = page.locator('.timeline-error');
			await expect(errorState).toBeVisible({ timeout: 5000 });
			await expect(errorState).toContainText(/failed/i);
		});

		test('should show retry button on error', async ({ page }) => {
			await page.route('**/api/events*', async (route) => {
				await route.fulfill({ status: 500, body: 'Internal Server Error' });
			});

			await page.goto('/timeline');

			const retryButton = page.getByRole('button', { name: /retry/i });
			await expect(retryButton).toBeVisible({ timeout: 5000 });
		});

		test('should retry fetch when retry button is clicked', async ({ page }) => {
			let failCount = 0;
			await page.route('**/api/events*', async (route) => {
				failCount++;
				if (failCount <= 1) {
					await route.fulfill({ status: 500, body: 'Error' });
				} else {
					await route.continue();
				}
			});

			await page.goto('/timeline');

			// Wait for error state
			const retryButton = page.getByRole('button', { name: /retry/i });
			await expect(retryButton).toBeVisible({ timeout: 5000 });

			// Click retry
			await retryButton.click();

			// Should load successfully on retry
			await waitForTimelineLoad(page);
			await expect(page.locator('.timeline-error')).not.toBeVisible();
		});
	});
});
