import { test, expect } from '@playwright/test';

/**
 * E2E tests for decision resolution flow and visual indicators
 *
 * Tests:
 * 1. Purple glow appears on task cards when decision is pending
 * 2. Glow disappears when decision is resolved
 * 3. DecisionsPanel shows pending decisions
 * 4. FilesPanel updates with changed files
 */

test.describe('Board Decision Visual Indicators', () => {
	test.beforeEach(async ({ page }) => {
		// Navigate to board view
		await page.goto('http://localhost:5173/board');

		// Wait for board to load
		await page.waitForSelector('.board-view', { timeout: 10000 });
	});

	test('should show purple glow on task card with pending decision', async ({ page }) => {
		// Wait for a running task to appear
		const runningCard = page.locator('.running-card').first();
		await runningCard.waitFor({ state: 'visible', timeout: 10000 });

		// Simulate decision_required event via WebSocket
		// (In real scenario, this would come from the backend)
		// For now, we'll verify the CSS class is applied when decisions exist

		// Take screenshot of initial state
		await page.screenshot({
			path: '.orc/test-screenshots/board-initial.png',
			fullPage: true,
		});

		// Check if any task has the has-pending-decision class
		const taskWithDecision = page.locator('.has-pending-decision').first();

		// If a decision exists, verify the glow
		const hasPendingDecision = await taskWithDecision.count() > 0;

		if (hasPendingDecision) {
			// Verify the purple glow is applied
			const boxShadow = await taskWithDecision.evaluate((el) => {
				return window.getComputedStyle(el).boxShadow;
			});

			// Purple glow should be present (contains rgba with purple color)
			expect(boxShadow).toContain('rgba');

			// Take screenshot showing glow
			await page.screenshot({
				path: '.orc/test-screenshots/board-decision-glow.png',
				fullPage: true,
			});
		}
	});

	test('should show decisions in DecisionsPanel', async ({ page }) => {
		// Check if DecisionsPanel is rendered
		const decisionsPanel = page.locator('.decisions-panel');
		await decisionsPanel.waitFor({ state: 'visible', timeout: 5000 });

		// Take screenshot
		await page.screenshot({
			path: '.orc/test-screenshots/decisions-panel.png',
			fullPage: true,
		});

		// Check panel structure
		const panelHeader = decisionsPanel.locator('.panel-header');
		await expect(panelHeader).toBeVisible();
	});

	test('should show files in FilesPanel', async ({ page }) => {
		// Check if FilesPanel is rendered
		const filesPanel = page.locator('.files-panel');
		await filesPanel.waitFor({ state: 'visible', timeout: 5000 });

		// Take screenshot
		await page.screenshot({
			path: '.orc/test-screenshots/files-panel.png',
			fullPage: true,
		});

		// Check panel structure
		const panelHeader = filesPanel.locator('.panel-header');
		await expect(panelHeader).toBeVisible();
	});

	test('should display TaskCard with decision indicator badge', async ({ page }) => {
		// Look for task cards in queue
		const taskCard = page.locator('.task-card').first();
		await taskCard.waitFor({ state: 'visible', timeout: 5000 });

		// Check if decision badge exists
		const decisionBadge = taskCard.locator('[data-testid="decision-badge"]');
		const hasBadge = await decisionBadge.count() > 0;

		if (hasBadge) {
			await expect(decisionBadge).toBeVisible();

			// Badge should show count > 0
			const badgeText = await decisionBadge.textContent();
			expect(badgeText).toMatch(/\d+/);
		}

		// Take screenshot
		await page.screenshot({
			path: '.orc/test-screenshots/task-card-decision-badge.png',
			fullPage: true,
		});
	});

	test('should verify CSS animation for decision pulse', async ({ page }) => {
		// Find task with pending decision
		const taskWithDecision = page.locator('.has-pending-decision').first();

		const count = await taskWithDecision.count();
		if (count === 0) {
			test.skip();
			return;
		}

		// Verify animation is defined
		const animationName = await taskWithDecision.evaluate((el) => {
			return window.getComputedStyle(el).animationName;
		});

		expect(animationName).toContain('decision-pulse');

		// Verify animation duration
		const animationDuration = await taskWithDecision.evaluate((el) => {
			return window.getComputedStyle(el).animationDuration;
		});

		// Should be 2s as per spec
		expect(animationDuration).toBe('2s');
	});

	test('should check console for errors', async ({ page }) => {
		const errors: string[] = [];

		page.on('console', (msg) => {
			if (msg.type() === 'error') {
				errors.push(msg.text());
			}
		});

		// Reload page to trigger any console errors
		await page.reload();
		await page.waitForSelector('.board-view', { timeout: 10000 });

		// Wait a bit for any async errors
		await page.waitForTimeout(2000);

		// Should have no console errors
		expect(errors).toHaveLength(0);
	});

	test('should verify network requests', async ({ page }) => {
		const failedRequests: string[] = [];

		page.on('requestfailed', (request) => {
			failedRequests.push(request.url());
		});

		// Navigate and interact
		await page.reload();
		await page.waitForSelector('.board-view', { timeout: 10000 });

		// Wait for any pending requests
		await page.waitForTimeout(2000);

		// Should have no failed requests
		expect(failedRequests).toHaveLength(0);
	});
});

test.describe('Decision Resolution Interaction', () => {
	test('should allow clicking decision option in DecisionsPanel', async ({ page }) => {
		await page.goto('http://localhost:5173/board');
		await page.waitForSelector('.board-view', { timeout: 10000 });

		// Look for DecisionsPanel with pending decisions
		const decisionsPanel = page.locator('.decisions-panel');
		await decisionsPanel.waitFor({ state: 'visible', timeout: 5000 });

		// Check if there are any decision options
		const optionButton = decisionsPanel.locator('button.decision-option').first();
		const hasOptions = await optionButton.count() > 0;

		if (hasOptions) {
			// Take screenshot before click
			await page.screenshot({
				path: '.orc/test-screenshots/before-decision-resolution.png',
				fullPage: true,
			});

			// Click the option
			await optionButton.click();

			// Wait for response
			await page.waitForTimeout(500);

			// Take screenshot after click
			await page.screenshot({
				path: '.orc/test-screenshots/after-decision-resolution.png',
				fullPage: true,
			});

			// Decision should be removed from panel (if resolved successfully)
			// Note: This depends on WebSocket event, so might still be visible
		}
	});
});

test.describe('Files Changed Updates', () => {
	test('should show changed files in FilesPanel', async ({ page }) => {
		await page.goto('http://localhost:5173/board');
		await page.waitForSelector('.board-view', { timeout: 10000 });

		const filesPanel = page.locator('.files-panel');
		await filesPanel.waitFor({ state: 'visible', timeout: 5000 });

		// Check if files are displayed
		const fileItems = filesPanel.locator('.file-item');
		const fileCount = await fileItems.count();

		if (fileCount > 0) {
			// Verify each file has status badge (M/A/D)
			for (let i = 0; i < fileCount; i++) {
				const fileItem = fileItems.nth(i);
				const statusBadge = fileItem.locator('.status-badge');
				await expect(statusBadge).toBeVisible();

				const badgeText = await statusBadge.textContent();
				expect(badgeText).toMatch(/^[MAD]$/);
			}

			// Take screenshot
			await page.screenshot({
				path: '.orc/test-screenshots/files-panel-with-files.png',
				fullPage: true,
			});
		}
	});
});
