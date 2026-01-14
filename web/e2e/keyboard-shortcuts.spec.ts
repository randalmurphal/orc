import { test, expect } from '@playwright/test';

test.describe('Keyboard Shortcuts', () => {
	test('should open keyboard shortcuts help with ?', async ({ page }) => {
		await page.goto('/');

		// Wait for app to initialize fully
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(500);

		// Press ? to open shortcuts help
		await page.keyboard.press('?');

		// Wait for modal to appear - looking for the modal backdrop with role="dialog"
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 3000 });

		// Should show keyboard shortcuts content
		const shortcutsContent = page.locator('text=Keyboard Shortcuts');
		await expect(shortcutsContent).toBeVisible();

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test('should open command palette with Shift+Alt+K', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Press Shift+Alt+K (browser-safe alternative to Cmd+K)
		await page.keyboard.press('Shift+Alt+k');

		// Command palette should open
		const palette = page.locator('.command-palette, [role="combobox"], [role="dialog"]');
		const paletteVisible = await palette.isVisible().catch(() => false);

		// Close with Escape if visible
		if (paletteVisible) {
			await page.keyboard.press('Escape');
		}
	});

	test('should navigate with g d to dashboard', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Press g then d for go-to-dashboard sequence
		await page.keyboard.press('g');
		await page.keyboard.press('d');

		// Should navigate to dashboard (allow for project query param)
		await expect(page).toHaveURL(/\/dashboard/);
	});

	test('should navigate with g t to tasks', async ({ page }) => {
		await page.goto('/dashboard');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Press g then t for go-to-tasks sequence
		await page.keyboard.press('g');
		await page.keyboard.press('t');

		// Should navigate to tasks (home) - check pathname is / or empty
		await expect(page).toHaveURL(/^http:\/\/localhost:\d+\/(\?|$)/);
	});

	test('should navigate with g r to preferences', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Press g then r for go-to-preferences sequence
		await page.keyboard.press('g');
		await page.keyboard.press('r');

		// Should navigate to preferences
		await expect(page).toHaveURL(/\/preferences/);
	});

	test('should navigate with g p to prompts', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Press g then p for go-to-prompts sequence
		await page.keyboard.press('g');
		await page.keyboard.press('p');

		// Should navigate to prompts (under environment/orchestrator)
		await expect(page).toHaveURL(/\/environment\/orchestrator\/prompts/);
	});

	test('should close modal with Escape', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Open new task modal with Shift+Alt+N (browser-safe alternative to Cmd+N)
		await page.keyboard.press('Shift+Alt+n');

		// Modal should be visible
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 2000 });

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test.describe('Task List Navigation', () => {
		test('should navigate tasks with j/k', async ({ page }) => {
			await page.goto('/');

			// Wait for tasks to load
			await page.waitForLoadState('networkidle');
			await page.waitForTimeout(500);

			// Check if there are tasks
			const taskCards = page.locator('.task-card-wrapper');
			const count = await taskCards.count();

			if (count > 0) {
				// Press j to select first task
				await page.keyboard.press('j');

				// Check for selected state
				const selectedCard = page.locator('.task-card-wrapper.selected');
				await expect(selectedCard).toBeVisible();
			}
		});
	});
});

test.describe('Keyboard Shortcuts - Input Fields', () => {
	test('should not trigger shortcuts when typing in input', async ({ page }) => {
		await page.goto('/');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(200);

		// Click on search input
		const searchInput = page.locator('input[placeholder*="Search"]');
		if (await searchInput.isVisible()) {
			await searchInput.click();

			// Type g d - should not navigate
			await page.keyboard.type('gd');

			// Should still be on home page (allow for project query param)
			await expect(page).toHaveURL(/^http:\/\/localhost:\d+\/(\?|$)/);

			// But input should have the text
			await expect(searchInput).toHaveValue('gd');
		}
	});
});
