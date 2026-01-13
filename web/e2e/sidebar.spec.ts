import { test, expect } from '@playwright/test';

test.describe('Sidebar', () => {
	test.beforeEach(async ({ page }) => {
		// Clear localStorage before each test
		await page.addInitScript(() => {
			localStorage.clear();
		});
	});

	test('should be expanded by default', async ({ page }) => {
		await page.goto('/');

		const sidebar = page.locator('.sidebar');
		await expect(sidebar).toBeVisible();
		await expect(sidebar).toHaveClass(/expanded/);

		// Labels should be visible when expanded
		const taskLabel = page.locator('.nav-label:has-text("Tasks")');
		await expect(taskLabel).toBeVisible();
	});

	test('should collapse when clicking the collapse button', async ({ page }) => {
		await page.goto('/');

		const sidebar = page.locator('.sidebar');
		await expect(sidebar).toHaveClass(/expanded/);

		// Click the collapse button (in the logo section when expanded)
		const collapseBtn = page.locator('.toggle-btn');
		await expect(collapseBtn).toBeVisible();
		await collapseBtn.click();

		// Wait for transition
		await page.waitForTimeout(300);

		// Sidebar should no longer have expanded class
		await expect(sidebar).not.toHaveClass(/expanded/);

		// Labels should not be visible when collapsed
		const taskLabel = page.locator('.nav-label:has-text("Tasks")');
		await expect(taskLabel).not.toBeVisible();
	});

	test('should expand when clicking the expand button', async ({ page }) => {
		// Start with collapsed sidebar
		await page.addInitScript(() => {
			localStorage.setItem('orc-sidebar-expanded', 'false');
		});
		await page.goto('/');

		const sidebar = page.locator('.sidebar');
		await expect(sidebar).not.toHaveClass(/expanded/);

		// Click the expand button (visible when collapsed)
		const expandBtn = page.locator('.expand-btn');
		await expect(expandBtn).toBeVisible();
		await expandBtn.click();

		// Wait for transition
		await page.waitForTimeout(300);

		// Sidebar should now have expanded class
		await expect(sidebar).toHaveClass(/expanded/);
	});

	test('should toggle with keyboard shortcut Cmd+B', async ({ page }) => {
		await page.goto('/');

		const sidebar = page.locator('.sidebar');
		await expect(sidebar).toHaveClass(/expanded/);

		// Press Cmd+B (or Ctrl+B on Windows/Linux)
		await page.keyboard.press('Meta+b');

		// Wait for transition
		await page.waitForTimeout(300);

		// Sidebar should be collapsed
		await expect(sidebar).not.toHaveClass(/expanded/);

		// Press again to expand
		await page.keyboard.press('Meta+b');
		await page.waitForTimeout(300);

		// Sidebar should be expanded again
		await expect(sidebar).toHaveClass(/expanded/);
	});

	test('should persist collapsed state across page reloads', async ({ page }) => {
		await page.goto('/');

		// Collapse the sidebar
		const collapseBtn = page.locator('.toggle-btn');
		await collapseBtn.click();
		await page.waitForTimeout(300);

		// Reload the page
		await page.reload();

		// Sidebar should still be collapsed
		const sidebar = page.locator('.sidebar');
		await expect(sidebar).not.toHaveClass(/expanded/);
	});

	test('should persist expanded state across page reloads', async ({ page }) => {
		// Start collapsed
		await page.addInitScript(() => {
			localStorage.setItem('orc-sidebar-expanded', 'false');
		});
		await page.goto('/');

		// Expand the sidebar
		const expandBtn = page.locator('.expand-btn');
		await expandBtn.click();
		await page.waitForTimeout(300);

		// Reload the page
		await page.reload();

		// Sidebar should still be expanded
		const sidebar = page.locator('.sidebar');
		await expect(sidebar).toHaveClass(/expanded/);
	});

	test('main content area adjusts margin when sidebar toggles', async ({ page }) => {
		await page.goto('/');

		const mainArea = page.locator('.main-area');

		// When expanded, should have sidebar-expanded class
		await expect(mainArea).toHaveClass(/sidebar-expanded/);

		// Collapse sidebar
		const collapseBtn = page.locator('.toggle-btn');
		await collapseBtn.click();
		await page.waitForTimeout(300);

		// Main area should no longer have sidebar-expanded class
		await expect(mainArea).not.toHaveClass(/sidebar-expanded/);
	});

	test('should show keyboard hint when expanded', async ({ page }) => {
		await page.goto('/');

		// Keyboard hint should be visible when expanded
		const keyboardHint = page.locator('.keyboard-hint');
		await expect(keyboardHint).toBeVisible();
		await expect(keyboardHint).toContainText('to toggle');
	});

	test('should hide keyboard hint when collapsed', async ({ page }) => {
		await page.addInitScript(() => {
			localStorage.setItem('orc-sidebar-expanded', 'false');
		});
		await page.goto('/');

		// Keyboard hint should not be visible when collapsed
		const keyboardHint = page.locator('.keyboard-hint');
		await expect(keyboardHint).not.toBeVisible();
	});

	test('navigation links should work when expanded', async ({ page }) => {
		await page.goto('/');

		// Click on Dashboard
		const dashboardLink = page.locator('.nav-item[href="/dashboard"]');
		await dashboardLink.click();

		await expect(page).toHaveURL('/dashboard');
	});

	test('navigation links should work when collapsed (via icons)', async ({ page }) => {
		await page.addInitScript(() => {
			localStorage.setItem('orc-sidebar-expanded', 'false');
		});
		await page.goto('/');

		// Click on Dashboard icon (should have title tooltip when collapsed)
		const dashboardLink = page.locator('.nav-item[href="/dashboard"]');
		await expect(dashboardLink).toHaveAttribute('title', 'Dashboard');
		await dashboardLink.click();

		await expect(page).toHaveURL('/dashboard');
	});
});
