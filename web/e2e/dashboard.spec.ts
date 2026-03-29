/**
 * Dashboard / Command Center E2E Tests
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project.
 * See global-setup.ts for sandbox creation details.
 */
import { test, expect } from './fixtures';

test.describe('Command Center', () => {
	test('should load the command center at root', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveTitle(/orc/);
		await expect(page.locator('h1')).toContainText('Command Center');
	});

	test('should display top stats cards', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('h1')).toContainText('Command Center');

		// Stats row should show Projects, Running, and Signals counts
		await expect(page.getByText('Projects')).toBeVisible();
		await expect(page.getByText('Running')).toBeVisible();
		await expect(page.getByText('Signals')).toBeVisible();
	});

	test('should display collapsible sections', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('h1')).toContainText('Command Center');

		// All five sections should render
		await expect(page.getByRole('region', { name: 'Running' })).toBeVisible();
		await expect(page.getByRole('region', { name: 'Attention' })).toBeVisible();
		await expect(page.getByRole('region', { name: 'Discussions' })).toBeVisible();
		await expect(page.getByRole('region', { name: 'Recommendations' })).toBeVisible();
		await expect(page.getByRole('region', { name: 'Recently Completed' })).toBeVisible();
	});

	test('should have New Task button', async ({ page }) => {
		await page.goto('/');
		const newTaskBtn = page.getByRole('button', { name: 'New Task' });
		await expect(newTaskBtn).toBeVisible();
	});

	test('should load within reasonable time', async ({ page }) => {
		const startTime = Date.now();
		await page.goto('/');
		await expect(page.locator('h1')).toContainText('Command Center');
		const loadTime = Date.now() - startTime;

		// Page should load within 3 seconds
		expect(loadTime).toBeLessThan(3000);
	});

	test('should display Projects section', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('h1')).toContainText('Command Center');

		// Projects section should show with heading
		await expect(page.getByRole('heading', { name: 'Projects', level: 2 })).toBeVisible();
	});
});
