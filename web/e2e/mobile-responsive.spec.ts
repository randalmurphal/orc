/**
 * Mobile Responsive E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 *
 * Tests the mobile responsive design with hamburger menu functionality.
 * All tests use mobile viewport (375px width) unless otherwise specified.
 */
import { test, expect } from './fixtures';

// Mobile viewport matching iPhone SE
const MOBILE_VIEWPORT = { width: 375, height: 667 };
// Desktop viewport
const DESKTOP_VIEWPORT = { width: 1024, height: 768 };
// Tablet viewport
const TABLET_VIEWPORT = { width: 768, height: 1024 };

test.describe('Mobile Responsive Design', () => {
	test.describe('Mobile Viewport (375px)', () => {
		test.beforeEach(async ({ page }) => {
			await page.setViewportSize(MOBILE_VIEWPORT);
		});

		test('hamburger menu button should be visible on mobile', async ({ page }) => {
			await page.goto('/');

			// Hamburger button should be visible
			const hamburgerBtn = page.locator('.mobile-menu-btn');
			await expect(hamburgerBtn).toBeVisible();
			await expect(hamburgerBtn).toHaveAttribute('aria-label', 'Toggle navigation menu');
		});

		test('sidebar should be hidden by default on mobile', async ({ page }) => {
			await page.goto('/');
			await page.evaluate(() => localStorage.clear());
			await page.reload();

			const sidebar = page.locator('.sidebar');
			await expect(sidebar).toBeVisible(); // sidebar element exists
			// Sidebar should be translated off-screen (not have mobile-open class)
			await expect(sidebar).not.toHaveClass(/mobile-open/);
		});

		test('sidebar should open when hamburger button is clicked', async ({ page }) => {
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			const sidebar = page.locator('.sidebar');

			// Click hamburger to open
			await hamburgerBtn.click();
			await page.waitForTimeout(200);

			// Sidebar should have mobile-open class
			await expect(sidebar).toHaveClass(/mobile-open/);
		});

		test('sidebar should close when clicking outside (on backdrop area)', async ({ page }) => {
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			const sidebar = page.locator('.sidebar');

			// Open sidebar
			await hamburgerBtn.click();
			await page.waitForTimeout(200);
			await expect(sidebar).toHaveClass(/mobile-open/);

			// Backdrop should be visible
			const backdrop = page.locator('.mobile-backdrop');
			await expect(backdrop).toBeVisible();

			// Click on the right side of the screen (outside the sidebar)
			// The sidebar is 220px wide on mobile, so click at x=300
			await page.mouse.click(300, 300);
			await page.waitForTimeout(200);

			// Sidebar should be closed
			await expect(sidebar).not.toHaveClass(/mobile-open/);
		});

		test('sidebar should close when clicking a navigation link', async ({ page }) => {
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			const sidebar = page.locator('.sidebar');

			// Open sidebar
			await hamburgerBtn.click();
			await page.waitForTimeout(200);
			await expect(sidebar).toHaveClass(/mobile-open/);

			// Click a navigation link
			const dashboardLink = page.locator('.nav-item[href="/dashboard"]');
			await dashboardLink.click();

			// Wait for navigation and animation
			await page.waitForTimeout(300);

			// Should navigate to dashboard
			await expect(page).toHaveURL('/dashboard');

			// Sidebar should be closed
			await expect(sidebar).not.toHaveClass(/mobile-open/);
		});

		test('sidebar should close when pressing Escape key', async ({ page }) => {
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			const sidebar = page.locator('.sidebar');

			// Open sidebar
			await hamburgerBtn.click();
			await page.waitForTimeout(200);
			await expect(sidebar).toHaveClass(/mobile-open/);

			// Press Escape
			await page.keyboard.press('Escape');
			await page.waitForTimeout(200);

			// Sidebar should be closed
			await expect(sidebar).not.toHaveClass(/mobile-open/);
		});

		test('main content should use full width on mobile', async ({ page }) => {
			await page.goto('/');

			const appMain = page.locator('.app-main');

			// On mobile, margin-left should be 0 (not offset for sidebar)
			const marginLeft = await appMain.evaluate((el) => {
				return window.getComputedStyle(el).marginLeft;
			});

			expect(marginLeft).toBe('0px');
		});

		test('project button should be hidden on mobile', async ({ page }) => {
			await page.goto('/');

			const projectBtn = page.locator('.project-btn');
			await expect(projectBtn).not.toBeVisible();
		});

		test('command hint should be hidden on mobile', async ({ page }) => {
			await page.goto('/');

			const cmdHint = page.locator('.cmd-hint');
			await expect(cmdHint).not.toBeVisible();
		});

		test('page should render correctly at 375px (iPhone SE)', async ({ page }) => {
			await page.goto('/');

			// Header should be visible
			const header = page.locator('.header');
			await expect(header).toBeVisible();

			// Main content should be visible
			const appContent = page.locator('.app-content');
			await expect(appContent).toBeVisible();

			// No horizontal scrollbar should appear
			const hasHorizontalScroll = await page.evaluate(() => {
				return document.documentElement.scrollWidth > document.documentElement.clientWidth;
			});
			expect(hasHorizontalScroll).toBe(false);
		});

		test('page should render correctly at 414px (iPhone Plus)', async ({ page }) => {
			await page.setViewportSize({ width: 414, height: 896 });
			await page.goto('/');

			// Header should be visible
			const header = page.locator('.header');
			await expect(header).toBeVisible();

			// Main content should be visible
			const appContent = page.locator('.app-content');
			await expect(appContent).toBeVisible();

			// No horizontal scrollbar should appear
			const hasHorizontalScroll = await page.evaluate(() => {
				return document.documentElement.scrollWidth > document.documentElement.clientWidth;
			});
			expect(hasHorizontalScroll).toBe(false);
		});
	});

	test.describe('Desktop Viewport (1024px)', () => {
		test.beforeEach(async ({ page }) => {
			await page.setViewportSize(DESKTOP_VIEWPORT);
		});

		test('hamburger menu should NOT be visible on desktop', async ({ page }) => {
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			// Check that hamburger button is hidden (display: none) on desktop
			await expect(hamburgerBtn).toBeHidden();
		});

		test('sidebar should be visible on desktop', async ({ page }) => {
			await page.goto('/');
			await page.evaluate(() => localStorage.clear());
			await page.reload();

			const sidebar = page.locator('.sidebar');
			await expect(sidebar).toBeVisible();
			await expect(sidebar).toHaveClass(/expanded/);
		});

		test('sidebar collapse/expand should work on desktop', async ({ page }) => {
			await page.goto('/');
			await page.evaluate(() => localStorage.clear());
			await page.reload();

			const sidebar = page.locator('.sidebar');
			const toggleBtn = page.locator('.toggle-btn');

			// Initially expanded
			await expect(sidebar).toHaveClass(/expanded/);

			// Click to collapse
			await toggleBtn.click();
			await page.waitForTimeout(300);

			// Should be collapsed
			await expect(sidebar).not.toHaveClass(/expanded/);

			// Click to expand again
			await toggleBtn.click();
			await page.waitForTimeout(300);

			// Should be expanded
			await expect(sidebar).toHaveClass(/expanded/);
		});

		test('project button should be visible on desktop', async ({ page }) => {
			await page.goto('/');

			const projectBtn = page.locator('.project-btn');
			await expect(projectBtn).toBeVisible();
		});
	});

	test.describe('Breakpoint Transition (768px)', () => {
		test('at exactly 768px, hamburger should NOT be visible', async ({ page }) => {
			await page.setViewportSize(TABLET_VIEWPORT);
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			await expect(hamburgerBtn).toBeHidden();

			// Sidebar should be visible
			const sidebar = page.locator('.sidebar');
			await expect(sidebar).toBeVisible();
		});

		test('at 767px, hamburger should be visible', async ({ page }) => {
			await page.setViewportSize({ width: 767, height: 1024 });
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			await expect(hamburgerBtn).toBeVisible();
		});

		test('transition from mobile to desktop should be smooth', async ({ page }) => {
			// Start at mobile
			await page.setViewportSize(MOBILE_VIEWPORT);
			await page.goto('/');

			const sidebar = page.locator('.sidebar');
			const hamburgerBtn = page.locator('.mobile-menu-btn');

			// Mobile: hamburger visible
			await expect(hamburgerBtn).toBeVisible();

			// Resize to desktop
			await page.setViewportSize(DESKTOP_VIEWPORT);
			await page.waitForTimeout(300); // Wait for any CSS transitions

			// Desktop: hamburger hidden, sidebar visible
			await expect(hamburgerBtn).toBeHidden();
			await expect(sidebar).toBeVisible();
		});
	});

	test.describe('Mobile Menu State', () => {
		test('mobile menu state should NOT persist to localStorage', async ({ page }) => {
			await page.setViewportSize(MOBILE_VIEWPORT);
			await page.goto('/');

			const hamburgerBtn = page.locator('.mobile-menu-btn');
			const sidebar = page.locator('.sidebar');

			// Open mobile menu
			await hamburgerBtn.click();
			await page.waitForTimeout(200);
			await expect(sidebar).toHaveClass(/mobile-open/);

			// Reload page
			await page.reload();
			await page.waitForTimeout(200);

			// Mobile menu should be closed (not persisted)
			await expect(sidebar).not.toHaveClass(/mobile-open/);
		});

		test('desktop sidebar state should persist across reloads', async ({ page }) => {
			await page.setViewportSize(DESKTOP_VIEWPORT);
			await page.goto('/');
			await page.evaluate(() => localStorage.clear());
			await page.reload();

			const sidebar = page.locator('.sidebar');
			const toggleBtn = page.locator('.toggle-btn');

			// Collapse sidebar
			await toggleBtn.click();
			await page.waitForTimeout(300);
			await expect(sidebar).not.toHaveClass(/expanded/);

			// Reload
			await page.reload();

			// Should still be collapsed
			await expect(sidebar).not.toHaveClass(/expanded/);
		});
	});
});
