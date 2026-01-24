/**
 * IconNav Timeline Navigation E2E Tests
 * 
 * Tests added as part of TASK-398 to verify Timeline nav item works correctly.
 * These tests verify:
 * - SC-1: Timeline nav item is visible in IconNav
 * - SC-2: Clicking Timeline navigates to /timeline
 * - SC-3: Timeline page renders correctly (TimeRangeSelector visible)
 */

import { test, expect } from './fixtures';

test.describe('IconNav - Timeline Navigation (TASK-398)', () => {
	test('should display Timeline nav item in IconNav', async ({ page }) => {
		await page.goto('/');
		
		// Wait for IconNav to render
		const iconNav = page.locator('.icon-nav');
		await expect(iconNav).toBeVisible();
		
		// Timeline link should be visible with correct aria-label
		const timelineLink = page.locator('a[aria-label="Activity timeline"]');
		await expect(timelineLink).toBeVisible();
		
		// Timeline label should be visible
		const timelineLabel = page.locator('.icon-nav__label').filter({ hasText: 'Timeline' });
		await expect(timelineLabel).toBeVisible();
	});

	test('should navigate to /timeline when Timeline is clicked', async ({ page }) => {
		await page.goto('/board');
		
		// Click on Timeline link
		const timelineLink = page.locator('a[aria-label="Activity timeline"]');
		await expect(timelineLink).toBeVisible();
		await timelineLink.click();
		
		// Should navigate to /timeline
		await expect(page).toHaveURL('/timeline');
	});

	test('should show active state on Timeline when at /timeline', async ({ page }) => {
		await page.goto('/timeline');
		
		// Wait for page to load
		await page.waitForSelector('.timeline-page', { timeout: 10000 }).catch(() => {
			// Timeline page might have a different class, check for h1
		});
		
		// Timeline link should have active class
		const timelineLink = page.locator('a[aria-label="Activity timeline"]');
		await expect(timelineLink).toHaveClass(/icon-nav__item--active/);
		
		// Should have aria-current="page"
		await expect(timelineLink).toHaveAttribute('aria-current', 'page');
	});

	test('should render TimeRangeSelector on timeline page', async ({ page }) => {
		await page.goto('/timeline');

		// Wait for timeline page to load
		await page.waitForTimeout(500);

		// TimeRangeSelector uses tabs, not buttons (role="tab")
		const todayTab = page.getByRole('tab', { name: /today/i });
		await expect(todayTab).toBeVisible({ timeout: 10000 });

		// Other time range tabs should also be present
		await expect(page.getByRole('tab', { name: /this week/i })).toBeVisible();
	});

	test('should render events or empty state on timeline page', async ({ page }) => {
		await page.goto('/timeline');
		
		// Wait for page to load
		await page.waitForTimeout(1000);
		
		// Should have either events or empty state
		const hasEvents = await page.locator('.timeline-event').count() > 0;
		const hasEmptyState = await page.locator('.timeline-empty').isVisible().catch(() => false);
		const hasNoMoreMessage = await page.locator('.timeline-no-more').isVisible().catch(() => false);
		
		// At least one of these should be true
		expect(hasEvents || hasEmptyState || hasNoMoreMessage).toBe(true);
	});

	test('should maintain Timeline active state after filter interaction', async ({ page }) => {
		await page.goto('/timeline');

		// Wait for page to load
		await page.waitForTimeout(500);

		// Click a time range tab (TimeRangeSelector uses role="tab")
		const thisWeekTab = page.getByRole('tab', { name: /this week/i });
		if (await thisWeekTab.isVisible()) {
			await thisWeekTab.click();
			await page.waitForTimeout(300);
		}

		// Timeline link should still be active
		const timelineLink = page.locator('a[aria-label="Activity timeline"]');
		await expect(timelineLink).toHaveClass(/icon-nav__item--active/);
	});
});
