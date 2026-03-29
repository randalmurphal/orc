/**
 * Navigation E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 */
import { test, expect } from './fixtures';

test.describe('Navigation', () => {
	test('should navigate between main pages via top nav', async ({ page }) => {
		// Start at home (Command Center)
		await page.goto('/');
		await expect(page.locator('h1')).toContainText('Command Center');

		// Navigate to Project
		await page.getByRole('link', { name: 'Project' }).click();
		await expect(page).toHaveURL(/\/project/);

		// Navigate to Board
		await page.getByRole('link', { name: 'Board' }).click();
		await expect(page).toHaveURL(/\/board/);

		// Navigate to Inbox (Recommendations)
		await page.getByRole('link', { name: 'Inbox' }).click();
		await expect(page).toHaveURL(/\/recommendations/);

		// Navigate to Workflows
		await page.getByRole('link', { name: 'Workflows' }).click();
		await expect(page).toHaveURL(/\/workflows/);

		// Navigate to Settings
		await page.getByRole('link', { name: 'Settings' }).click();
		await expect(page).toHaveURL(/\/settings/);

		// Navigate back to Home
		await page.getByRole('link', { name: 'Home' }).click();
		await expect(page).toHaveURL(/\//);
	});

	test('should have top navigation bar', async ({ page }) => {
		await page.goto('/');

		// TopBar should have navigation links
		const nav = page.getByRole('navigation').first();
		await expect(nav).toBeVisible();

		// All primary nav links should exist
		await expect(page.getByRole('link', { name: 'Home' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Project' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Board' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Workflows' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible();
	});

	test('should highlight current page in navigation', async ({ page }) => {
		await page.goto('/board');

		// The Board nav link should be marked as active
		const boardLink = page.getByRole('link', { name: 'Board' });
		await expect(boardLink).toBeVisible();
		// Active links get aria-current or an active class
		const isCurrent = await boardLink.getAttribute('aria-current');
		const hasActiveClass = await boardLink.evaluate(el => el.classList.contains('active'));
		expect(isCurrent === 'page' || hasActiveClass).toBeTruthy();
	});

	test('should have project sidebar', async ({ page }) => {
		await page.goto('/');

		// Left sidebar should show project name and thread list
		const sidebar = page.getByRole('navigation', { name: 'Main navigation' });
		await expect(sidebar).toBeVisible();
	});
});

test.describe('Layout', () => {
	test('should have consistent header across pages', async ({ page }) => {
		const pages = ['/', '/project', '/board', '/recommendations', '/workflows'];

		for (const path of pages) {
			await page.goto(path);

			// Each page should have a main heading
			const heading = page.locator('h1');
			await expect(heading).toBeVisible();
		}
	});

	test('should be responsive', async ({ page }) => {
		// Test mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.goto('/');

		// Page should still be usable
		const heading = page.locator('h1');
		await expect(heading).toBeVisible();

		// Test tablet viewport
		await page.setViewportSize({ width: 768, height: 1024 });
		await expect(heading).toBeVisible();

		// Test desktop viewport
		await page.setViewportSize({ width: 1280, height: 800 });
		await expect(heading).toBeVisible();
	});
});
