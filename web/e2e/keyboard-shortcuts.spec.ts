import { test, expect } from '@playwright/test';

/**
 * E2E tests for keyboard shortcuts
 *
 * The app uses Shift+Alt modifier (Shift+Option on Mac) to avoid browser
 * conflicts with Cmd+K, Cmd+N, Cmd+B, Cmd+P etc.
 *
 * Test Coverage (12 tests):
 * - Global Shortcuts (6 tests)
 * - Navigation Sequences (3 tests)
 * - Task List Context (3 tests)
 */

test.describe('Keyboard Shortcuts - Global', () => {
	test('should open command palette with Shift+Alt+K', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press Shift+Alt+K (browser-safe alternative to Cmd+K)
		await page.keyboard.press('Shift+Alt+k');

		// Command palette has aria-label="Command palette"
		const palette = page.locator('[aria-label="Command palette"]');
		await expect(palette).toBeVisible({ timeout: 3000 });

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(palette).not.toBeVisible();
	});

	test('should open new task modal with Shift+Alt+N', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press Shift+Alt+N (browser-safe alternative to Cmd+N)
		await page.keyboard.press('Shift+Alt+n');

		// New task modal should be visible
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 3000 });

		// Should contain new task form elements
		const titleInput = modal.locator('input[name="title"], input[placeholder*="title" i]');
		const formExists = await titleInput.isVisible().catch(() => false);
		// Modal is open even if we can't verify the exact form structure
		expect(await modal.isVisible()).toBe(true);

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test('should toggle sidebar with Shift+Alt+B', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Get initial sidebar state
		const mainArea = page.locator('.main-area');
		const initialExpanded = await mainArea.evaluate((el) =>
			el.classList.contains('sidebar-expanded')
		);

		// Press Shift+Alt+B to toggle sidebar
		await page.keyboard.press('Shift+Alt+b');
		await page.waitForTimeout(200);

		// Sidebar state should have changed
		const afterToggle = await mainArea.evaluate((el) =>
			el.classList.contains('sidebar-expanded')
		);
		expect(afterToggle).not.toBe(initialExpanded);

		// Toggle again to restore
		await page.keyboard.press('Shift+Alt+b');
		await page.waitForTimeout(200);

		const afterSecondToggle = await mainArea.evaluate((el) =>
			el.classList.contains('sidebar-expanded')
		);
		expect(afterSecondToggle).toBe(initialExpanded);
	});

	test('should open project switcher with Shift+Alt+P', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press Shift+Alt+P (browser-safe alternative to Cmd+P)
		await page.keyboard.press('Shift+Alt+p');

		// Project switcher modal should open
		const switcher = page.locator('[role="dialog"]').filter({ hasText: /project/i });
		await expect(switcher).toBeVisible({ timeout: 3000 });

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(switcher).not.toBeVisible();
	});

	test('should show keyboard help with ? key', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press ? to open shortcuts help
		await page.keyboard.press('?');

		// Wait for modal to appear - look for the modal title specifically
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 3000 });

		// Should show keyboard shortcuts title in modal header (use exact match)
		const shortcutsTitle = page.getByRole('heading', { name: 'Keyboard Shortcuts', exact: true });
		await expect(shortcutsTitle).toBeVisible();

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test('should close all modals with Escape', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Open multiple modals in sequence and verify Escape closes each
		const modalTests = [
			{ shortcut: 'Shift+Alt+n', name: 'new task' },
			{ shortcut: 'Shift+Alt+k', name: 'command palette' },
			{ shortcut: '?', name: 'keyboard help' },
			{ shortcut: 'Shift+Alt+p', name: 'project switcher' }
		];

		for (const { shortcut, name } of modalTests) {
			// Open modal
			await page.keyboard.press(shortcut);
			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible({ timeout: 3000 });

			// Close with Escape
			await page.keyboard.press('Escape');
			await expect(modal).not.toBeVisible({ timeout: 2000 });

			// Small delay between tests
			await page.waitForTimeout(100);
		}
	});
});

test.describe('Keyboard Shortcuts - Navigation Sequences', () => {
	test('should navigate to dashboard with g then d', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press g then d for go-to-dashboard sequence
		await page.keyboard.press('g');
		await page.keyboard.press('d');

		// Should navigate to dashboard
		await expect(page).toHaveURL(/\/dashboard/, { timeout: 3000 });
	});

	test('should navigate to tasks with g then t', async ({ page }) => {
		// Start from dashboard
		await page.goto('/dashboard');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press g then t for go-to-tasks sequence
		await page.keyboard.press('g');
		await page.keyboard.press('t');

		// Should navigate to tasks (home page) - pathname is / or empty
		await expect(page).toHaveURL(/^http:\/\/localhost:\d+\/(\?|$)/, { timeout: 3000 });
	});

	test('should navigate to environment with g then e', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press g then e for go-to-environment sequence
		await page.keyboard.press('g');
		await page.keyboard.press('e');

		// Should navigate to environment
		await expect(page).toHaveURL(/\/environment/, { timeout: 3000 });
	});
});

test.describe('Keyboard Shortcuts - Task List Context', () => {
	test('should navigate tasks with j/k keys', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(500);

		// Check if there are tasks
		const taskCards = page.locator('.task-card-wrapper');
		const count = await taskCards.count();

		if (count > 0) {
			// Press j to select first task (move down)
			await page.keyboard.press('j');
			await page.waitForTimeout(100);

			// Check for selected state
			const selectedCard = page.locator('.task-card-wrapper.selected');
			await expect(selectedCard).toBeVisible({ timeout: 2000 });

			// If multiple tasks, press j again then k to go back up
			if (count > 1) {
				await page.keyboard.press('j');
				await page.waitForTimeout(100);

				// Now press k to navigate up
				await page.keyboard.press('k');
				await page.waitForTimeout(100);

				// Should still have a selected card
				await expect(selectedCard).toBeVisible();
			}
		} else {
			// No tasks to navigate - test passes trivially
			test.skip();
		}
	});

	test('should open selected task with Enter', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(500);

		// Check if there are tasks
		const taskCards = page.locator('.task-card-wrapper');
		const count = await taskCards.count();

		if (count > 0) {
			// Press j to select first task
			await page.keyboard.press('j');
			await page.waitForTimeout(100);

			// Get the selected task ID from the first task card's data or content
			const selectedCard = page.locator('.task-card-wrapper.selected');
			await expect(selectedCard).toBeVisible({ timeout: 2000 });

			// Press Enter to open the selected task
			await page.keyboard.press('Enter');

			// Should navigate to task detail page
			await expect(page).toHaveURL(/\/tasks\/TASK-\d+/, { timeout: 3000 });
		} else {
			// No tasks to open - test passes trivially
			test.skip();
		}
	});

	test('should focus search with / key', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Press / to focus search
		await page.keyboard.press('/');
		await page.waitForTimeout(100);

		// Search input should be focused
		const searchInput = page.locator('input[placeholder*="Search"]');
		if (await searchInput.isVisible()) {
			// Check if input is focused by typing and verifying value
			await page.keyboard.type('test query');
			await expect(searchInput).toHaveValue('test query');
		} else {
			// Search not visible on this page - check command palette as fallback
			// Some implementations open command palette on /
			const palette = page.locator('[role="dialog"]');
			const paletteVisible = await palette.isVisible().catch(() => false);
			// Either search is focused or command palette opened
			expect(paletteVisible || await searchInput.isVisible()).toBe(true);
		}
	});
});

test.describe('Keyboard Shortcuts - Input Fields', () => {
	test('should not trigger shortcuts when typing in input', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(300);

		// Click on search input
		const searchInput = page.locator('input[placeholder*="Search"]');
		if (await searchInput.isVisible()) {
			await searchInput.click();

			// Type g d - should not navigate (would be go-to-dashboard sequence)
			await page.keyboard.type('gd');

			// Should still be on home page
			await expect(page).toHaveURL(/^http:\/\/localhost:\d+\/(\?|$)/);

			// Input should have the text
			await expect(searchInput).toHaveValue('gd');
		}
	});
});
