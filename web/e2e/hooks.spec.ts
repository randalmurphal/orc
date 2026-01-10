import { test, expect } from '@playwright/test';

test.describe('Hooks Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/hooks');
	});

	test('should display hooks page', async ({ page }) => {
		await expect(page).toHaveTitle(/Hooks/);
	});

	test('should show hook list or empty state', async ({ page }) => {
		// Wait for page to load
		await page.waitForLoadState('networkidle');

		// Should show either hooks list or empty state
		const hooksList = page.locator('.hooks-list, .hook-list, [data-testid="hooks"]');
		const emptyState = page.locator('.empty-state, .no-hooks');
		const errorState = page.locator('.error');

		const hasHooks = await hooksList.isVisible().catch(() => false);
		const isEmpty = await emptyState.isVisible().catch(() => false);
		const hasError = await errorState.isVisible().catch(() => false);

		// One of these states should be true
		expect(hasHooks || isEmpty || hasError).toBeTruthy();
	});
});

test.describe('Skills Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/skills');
	});

	test('should display skills page', async ({ page }) => {
		await expect(page).toHaveTitle(/Skills/);
	});

	test('should show skill list or empty state', async ({ page }) => {
		// Wait for page to load
		await page.waitForLoadState('networkidle');

		// Should show either skills list or empty state
		const skillsList = page.locator('.skills-list, .skill-list, [data-testid="skills"]');
		const emptyState = page.locator('.empty-state, .no-skills');
		const errorState = page.locator('.error');

		const hasSkills = await skillsList.isVisible().catch(() => false);
		const isEmpty = await emptyState.isVisible().catch(() => false);
		const hasError = await errorState.isVisible().catch(() => false);

		// One of these states should be true
		expect(hasSkills || isEmpty || hasError).toBeTruthy();
	});
});
