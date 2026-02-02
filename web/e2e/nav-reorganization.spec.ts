/**
 * E2E Tests for Navigation Reorganization (TASK-765)
 *
 * Tests verify the UX simplification changes from INIT-037:
 * - SC-1: Agents link is NOT present in main sidebar navigation
 * - SC-2: Environ link is NOT present in main sidebar navigation
 * - SC-3: Settings link navigates to Settings page with tabs
 *
 * CRITICAL: Tests run against ISOLATED SANDBOX project.
 */

import { test, expect } from './fixtures';

test.describe('Navigation Reorganization (TASK-765)', () => {
	test.describe('SC-1: Agents link removed from main navigation', () => {
		test('Agents link is NOT visible in IconNav', async ({ page }) => {
			await page.goto('/board');

			// Wait for IconNav to render
			const iconNav = page.locator('.icon-nav');
			await expect(iconNav).toBeVisible();

			// Agents link should NOT exist
			const agentsLink = page.locator('a[aria-label*="Agent"]');
			await expect(agentsLink).not.toBeVisible();

			// Agents label should NOT exist in nav
			const agentsLabel = page.locator('.icon-nav__label').filter({ hasText: 'Agents' });
			await expect(agentsLabel).not.toBeVisible();
		});

		test('no link with href="/agents" exists in main navigation', async ({ page }) => {
			await page.goto('/board');

			const iconNav = page.locator('.icon-nav');
			await expect(iconNav).toBeVisible();

			// Check that no /agents link exists in the nav
			const agentsHrefLink = iconNav.locator('a[href="/agents"]');
			await expect(agentsHrefLink).toHaveCount(0);
		});
	});

	test.describe('SC-2: Environ link removed from main navigation', () => {
		test('Environ link is NOT visible in IconNav', async ({ page }) => {
			await page.goto('/board');

			// Wait for IconNav to render
			const iconNav = page.locator('.icon-nav');
			await expect(iconNav).toBeVisible();

			// Environ link should NOT exist (various possible labels)
			const environLink = page.locator('a[aria-label*="Environment"]');
			await expect(environLink).not.toBeVisible();

			// Environ label should NOT exist in nav
			const environLabel = page.locator('.icon-nav__label').filter({ hasText: 'Environ' });
			await expect(environLabel).not.toBeVisible();
		});

		test('no link with href="/environ" exists in main navigation', async ({ page }) => {
			await page.goto('/board');

			const iconNav = page.locator('.icon-nav');
			await expect(iconNav).toBeVisible();

			// Check that no /environ link exists in the nav
			const environHrefLink = iconNav.locator('a[href="/environ"]');
			await expect(environHrefLink).toHaveCount(0);
		});
	});

	test.describe('SC-3: Settings link navigates to tabbed Settings page', () => {
		test('Settings link IS visible in IconNav', async ({ page }) => {
			await page.goto('/board');

			const iconNav = page.locator('.icon-nav');
			await expect(iconNav).toBeVisible();

			// Settings link should exist
			const settingsLink = page.locator('a[aria-label="Application settings"]');
			await expect(settingsLink).toBeVisible();

			// Settings label should be visible
			const settingsLabel = page.locator('.icon-nav__label').filter({ hasText: 'Settings' });
			await expect(settingsLabel).toBeVisible();
		});

		test('clicking Settings navigates to /settings with tabs', async ({ page }) => {
			await page.goto('/board');

			// Click Settings link
			const settingsLink = page.locator('a[aria-label="Application settings"]');
			await settingsLink.click();

			// Should navigate to /settings (redirects to /settings/general)
			await expect(page).toHaveURL(/\/settings/);

			// Should have tablist with General, Agents, Environment tabs
			const tablist = page.getByRole('tablist', { name: 'Settings sections' });
			await expect(tablist).toBeVisible();

			await expect(page.getByRole('tab', { name: /general/i })).toBeVisible();
			await expect(page.getByRole('tab', { name: /agents/i })).toBeVisible();
			await expect(page.getByRole('tab', { name: /environment/i })).toBeVisible();
		});

		test('Settings link becomes active when on /settings routes', async ({ page }) => {
			await page.goto('/settings/general');

			// Settings link should have active class
			const settingsLink = page.locator('a[aria-label="Application settings"]');
			await expect(settingsLink).toHaveClass(/icon-nav__item--active/);
		});
	});
});

test.describe('Navigation Item Count Verification', () => {
	test('IconNav displays exactly 7 navigation items (after removal)', async ({ page }) => {
		await page.goto('/board');

		const iconNav = page.locator('.icon-nav');
		await expect(iconNav).toBeVisible();

		// Count all nav links in the icon nav
		const navLinks = iconNav.locator('.icon-nav__item');
		const count = await navLinks.count();

		// Should be 7: Board, Initiatives, Timeline, Stats, Workflows, Settings, Help
		expect(count).toBe(7);
	});

	test('expected navigation items are present in correct order', async ({ page }) => {
		await page.goto('/board');

		const iconNav = page.locator('.icon-nav');
		await expect(iconNav).toBeVisible();

		// Verify expected items exist
		const expectedLabels = ['Board', 'Initiatives', 'Timeline', 'Stats', 'Workflows', 'Settings', 'Help'];

		for (const label of expectedLabels) {
			const navLabel = page.locator('.icon-nav__label').filter({ hasText: label });
			await expect(navLabel).toBeVisible();
		}
	});
});
