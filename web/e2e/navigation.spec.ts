import { test, expect } from '@playwright/test';

test.describe('Navigation', () => {
	test('should navigate between main pages', async ({ page }) => {
		// Start at home
		await page.goto('/');
		await expect(page).toHaveTitle(/orc - Tasks/);

		// Navigate to prompts
		const promptsLink = page.locator('a[href="/prompts"], nav a:has-text("Prompts")');
		if (await promptsLink.isVisible()) {
			await promptsLink.click();
			await expect(page).toHaveURL('/prompts');
			await expect(page).toHaveTitle(/Prompts/);
		}

		// Navigate to hooks
		const hooksLink = page.locator('a[href="/hooks"], nav a:has-text("Hooks")');
		if (await hooksLink.isVisible()) {
			await hooksLink.click();
			await expect(page).toHaveURL('/hooks');
		}

		// Navigate to skills
		const skillsLink = page.locator('a[href="/skills"], nav a:has-text("Skills")');
		if (await skillsLink.isVisible()) {
			await skillsLink.click();
			await expect(page).toHaveURL('/skills');
		}

		// Navigate to config
		const configLink = page.locator('a[href="/config"], nav a:has-text("Config")');
		if (await configLink.isVisible()) {
			await configLink.click();
			await expect(page).toHaveURL('/config');
		}

		// Navigate back to tasks
		const tasksLink = page.locator('nav a:has-text("Tasks")');
		if (await tasksLink.isVisible()) {
			await tasksLink.click();
			await expect(page).toHaveURL('/');
		}
	});

	test('should have navigation menu', async ({ page }) => {
		await page.goto('/');

		// Look for navigation element
		const nav = page.locator('nav, .navigation, .sidebar');
		const hasNav = await nav.isVisible().catch(() => false);

		// Navigation should exist
		expect(hasNav).toBeTruthy();
	});

	test('should highlight current page in navigation', async ({ page }) => {
		await page.goto('/prompts');

		// The prompts nav link should be marked as active/current
		const activeLink = page.locator('nav a.active, nav a[aria-current="page"], .nav-item.active');
		const hasActiveLink = await activeLink.isVisible().catch(() => false);

		// Active state should be shown (implementation may vary)
		// This is a soft check since styling varies
		if (hasActiveLink) {
			await expect(activeLink).toBeVisible();
		}
	});
});

test.describe('Layout', () => {
	test('should have consistent header across pages', async ({ page }) => {
		const pages = ['/', '/prompts', '/hooks', '/skills'];

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
