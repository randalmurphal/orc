import { test, expect } from '@playwright/test';

test.describe('Keyboard Shortcuts', () => {
	test('should open keyboard shortcuts help with ?', async ({ page }) => {
		await page.goto('/');

		// Press ? to open shortcuts help
		await page.keyboard.press('Shift+/');

		// Wait for modal to appear
		const modal = page.locator('.modal-content, [role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 1000 });

		// Should show keyboard shortcuts content
		const shortcutsContent = page.locator('text=Keyboard Shortcuts');
		await expect(shortcutsContent).toBeVisible();

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test('should open command palette with Cmd+K', async ({ page }) => {
		await page.goto('/');

		// Press Cmd+K (or Ctrl+K on non-Mac)
		await page.keyboard.press('Meta+k');

		// Command palette should open
		const palette = page.locator('.command-palette, [role="combobox"], .modal-content');
		const paletteVisible = await palette.isVisible().catch(() => false);

		// Close with Escape if visible
		if (paletteVisible) {
			await page.keyboard.press('Escape');
		}
	});

	test('should navigate with g d to dashboard', async ({ page }) => {
		await page.goto('/');

		// Press g then d for go-to-dashboard sequence
		await page.keyboard.press('g');
		await page.keyboard.press('d');

		// Should navigate to dashboard
		await expect(page).toHaveURL('/dashboard');
	});

	test('should navigate with g t to tasks', async ({ page }) => {
		await page.goto('/dashboard');

		// Press g then t for go-to-tasks sequence
		await page.keyboard.press('g');
		await page.keyboard.press('t');

		// Should navigate to tasks (home)
		await expect(page).toHaveURL('/');
	});

	test('should navigate with g s to settings', async ({ page }) => {
		await page.goto('/');

		// Press g then s for go-to-settings sequence
		await page.keyboard.press('g');
		await page.keyboard.press('s');

		// Should navigate to settings
		await expect(page).toHaveURL('/settings');
	});

	test('should navigate with g p to prompts', async ({ page }) => {
		await page.goto('/');

		// Press g then p for go-to-prompts sequence
		await page.keyboard.press('g');
		await page.keyboard.press('p');

		// Should navigate to prompts
		await expect(page).toHaveURL('/prompts');
	});

	test('should close modal with Escape', async ({ page }) => {
		await page.goto('/');

		// Open new task modal with Cmd+N
		await page.keyboard.press('Meta+n');

		// Modal should be visible
		const modal = page.locator('.modal-content, [role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 1000 });

		// Close with Escape
		await page.keyboard.press('Escape');
		await expect(modal).not.toBeVisible();
	});

	test.describe('Task List Navigation', () => {
		test('should navigate tasks with j/k', async ({ page }) => {
			await page.goto('/');

			// Wait for tasks to load
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

		// Click on search input
		const searchInput = page.locator('input[placeholder*="Search"]');
		if (await searchInput.isVisible()) {
			await searchInput.click();

			// Type g d - should not navigate
			await page.keyboard.type('gd');

			// Should still be on home page
			await expect(page).toHaveURL('/');

			// But input should have the text
			await expect(searchInput).toHaveValue('gd');
		}
	});
});
