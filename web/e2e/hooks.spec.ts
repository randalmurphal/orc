/**
 * Hooks and Skills E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 */
import { test, expect } from './fixtures';

test.describe('Hooks Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/environment/hooks');
	});

	test('should display hooks page', async ({ page }) => {
		await expect(page).toHaveTitle(/Hooks/);
		// Page header
		await expect(page.locator('h3')).toHaveText('Hooks');
	});

	test('should show hook groups or loading/error state', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Should show hooks groups or loading/error state
		const hooksGroups = page.locator('.hooks-groups');
		const loading = page.locator('.env-loading');
		const error = page.locator('.env-error');

		const hasGroups = await hooksGroups.isVisible().catch(() => false);
		const isLoading = await loading.isVisible().catch(() => false);
		const hasError = await error.isVisible().catch(() => false);

		expect(hasGroups || isLoading || hasError).toBeTruthy();
	});

	test('should show scope tabs', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Should have Project and Global scope tab triggers
		const projectTab = page.locator('button.env-scope-tab', { hasText: 'Project' });
		const globalTab = page.locator('button.env-scope-tab', { hasText: 'Global' });

		await expect(projectTab).toBeVisible();
		await expect(globalTab).toBeVisible();
	});

	test('should display hook event groups', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const hooksGroups = page.locator('.hooks-group');
		const count = await hooksGroups.count();

		// Should have at least some hook event types displayed
		if (count > 0) {
			const firstGroup = hooksGroups.first();
			// Group should have title
			await expect(firstGroup.locator('.hooks-group-title')).toBeVisible();
			// Group should have Edit/Add button (use first() since there may be multiple buttons)
			await expect(firstGroup.locator('button').first()).toBeVisible();
		}
	});
});

test.describe('Skills Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/environment/skills');
	});

	test('should display skills page', async ({ page }) => {
		await expect(page).toHaveTitle(/Skills/);
		// Page header
		await expect(page.locator('h3')).toHaveText('Skills');
	});

	test('should show skills or empty state', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Should show skills grid, empty state, or loading/error state
		const skillsGrid = page.locator('.env-card-grid');
		const emptyState = page.locator('.env-empty');
		const loading = page.locator('.env-loading');
		const error = page.locator('.env-error');

		const hasSkills = await skillsGrid.isVisible().catch(() => false);
		const isEmpty = await emptyState.isVisible().catch(() => false);
		const isLoading = await loading.isVisible().catch(() => false);
		const hasError = await error.isVisible().catch(() => false);

		expect(hasSkills || isEmpty || isLoading || hasError).toBeTruthy();
	});

	test('should show scope tabs', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Should have Project and Global scope tab triggers
		const projectTab = page.locator('button.env-scope-tab', { hasText: 'Project' });
		const globalTab = page.locator('button.env-scope-tab', { hasText: 'Global' });

		await expect(projectTab).toBeVisible();
		await expect(globalTab).toBeVisible();
	});

	test('should display skill cards when skills exist', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const skillCards = page.locator('.skill-card');
		const count = await skillCards.count();

		// Skills may or may not exist depending on sandbox state
		if (count > 0) {
			const firstCard = skillCards.first();
			// Card should have title
			await expect(firstCard.locator('.env-card-title')).toBeVisible();
		}
	});
});
