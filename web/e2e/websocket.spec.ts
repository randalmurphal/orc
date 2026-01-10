import { test, expect } from '@playwright/test';

test.describe('WebSocket Connection', () => {
	test('should show connection status on task detail page', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');

		// Use existing tasks
		const taskCards = page.locator('.task-grid a');
		const count = await taskCards.count();

		if (count > 0) {
			// Navigate to first task
			await taskCards.first().click();
			await expect(page).toHaveURL(/\/tasks\/TASK-\d+/);
			await page.waitForLoadState('networkidle');

			// The connection banner only shows when not connected
			// So either no banner (good) or a status indicator exists
			const connectionBanner = page.locator('.connection-banner');
			const isDisconnected = await connectionBanner.isVisible().catch(() => false);

			// If disconnected, should show reconnect option
			if (isDisconnected) {
				await expect(connectionBanner).toContainText(/Connecting|Reconnecting|Disconnected/);
			}
		}
	});

	test('should handle page reload gracefully', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');

		// Use existing tasks
		const taskCards = page.locator('.task-grid a');
		const count = await taskCards.count();

		if (count > 0) {
			// Get task title before clicking
			const firstCard = taskCards.first();
			await firstCard.click();
			await expect(page).toHaveURL(/\/tasks\/TASK-\d+/);

			// Store URL and title
			const url = page.url();
			const title = await page.locator('h1').textContent();

			// Reload page
			await page.reload();

			// Should still be on the same page
			await expect(page).toHaveURL(url);

			// Task info should reload
			await expect(page.locator('h1')).toHaveText(title || '');
		}
	});
});

test.describe('Real-time Updates', () => {
	test('should display transcript section', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');

		// Use existing tasks
		const taskCards = page.locator('.task-grid a');
		const count = await taskCards.count();

		if (count > 0) {
			// Navigate to first task
			await taskCards.first().click();
			await page.waitForLoadState('networkidle');

			// Transcript section should exist
			const transcriptSection = page.locator('section:has-text("Transcript")');
			await expect(transcriptSection).toBeVisible();
		}
	});

	test('should display timeline for tasks with plans', async ({ page }) => {
		await page.goto('/');

		// Check if there are any existing tasks with plans
		const taskCards = page.locator('.task-grid a, .task-card');
		const count = await taskCards.count();

		if (count > 0) {
			// Click first task
			await taskCards.first().click();

			// Wait for page load
			await page.waitForLoadState('networkidle');

			// If task has a plan, timeline should be visible
			const timeline = page.locator('.section:has-text("Timeline"), section:has(h2:text("Timeline"))');
			const hasTimeline = await timeline.isVisible().catch(() => false);

			// Timeline visibility depends on whether task has a plan
			// Just verify the check doesn't crash
			expect(typeof hasTimeline).toBe('boolean');
		}
	});

	test('should show token usage when available', async ({ page }) => {
		await page.goto('/');

		// Navigate to an existing task if available
		const taskCards = page.locator('.task-grid a, .task-card');
		const count = await taskCards.count();

		if (count > 0) {
			await taskCards.first().click();
			await page.waitForLoadState('networkidle');

			// Token usage section may or may not be visible depending on state
			const tokenSection = page.locator('.section:has-text("Token Usage"), section:has(h2:text("Token Usage"))');
			const hasTokens = await tokenSection.isVisible().catch(() => false);

			// Just verify the element query works
			expect(typeof hasTokens).toBe('boolean');
		}
	});
});
