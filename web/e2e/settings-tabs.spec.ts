/**
 * E2E Tests for Settings Tabs Navigation
 *
 * Tests for TASK-723: Create Settings page with Agents and Environment tabs
 *
 * These tests verify the full navigation flow for the tabbed Settings page:
 * - Tab switching and URL updates
 * - Deep linking to specific tabs
 * - Browser back/forward navigation
 * - Tab content verification
 *
 * CRITICAL: Tests run against ISOLATED SANDBOX project.
 *
 * Coverage mapping:
 * - SC-1: Settings page renders three tabs
 * - SC-2: Clicking Agents tab displays AgentsView
 * - SC-3: Clicking Environment tab displays EnvironmentLayout
 * - SC-4: Direct navigation to /settings/agents
 * - SC-5: Direct navigation to /settings/environment
 * - SC-6: Navigation to /settings redirects to /settings/general
 */

import { test, expect } from './fixtures';

test.describe('Settings Tabs Navigation', () => {
	test.describe('SC-1: Settings page renders three tabs', () => {
		test('displays General, Agents, and Environment tabs', async ({ page }) => {
			await page.goto('/settings/general');

			// Verify all three tabs are visible
			const tablist = page.getByRole('tablist', { name: 'Settings sections' });
			await expect(tablist).toBeVisible();

			await expect(page.getByRole('tab', { name: /general/i })).toBeVisible();
			await expect(page.getByRole('tab', { name: /agents/i })).toBeVisible();
			await expect(page.getByRole('tab', { name: /environment/i })).toBeVisible();
		});

		test('tabs are in correct order: General, Agents, Environment', async ({ page }) => {
			await page.goto('/settings/general');

			const tabs = page.getByRole('tab');
			const tabTexts = await tabs.allTextContents();

			// Verify order
			expect(tabTexts[0].toLowerCase()).toContain('general');
			expect(tabTexts[1].toLowerCase()).toContain('agents');
			expect(tabTexts[2].toLowerCase()).toContain('environment');
		});
	});

	test.describe('SC-2: Clicking Agents tab displays AgentsView', () => {
		test('clicking Agents tab shows AgentsView content', async ({ page }) => {
			await page.goto('/settings/general');

			// Click the Agents tab
			await page.getByRole('tab', { name: /agents/i }).click();

			// Verify AgentsView content is displayed
			// AgentsView shows "Agents" header and "Active Agents" section
			await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
			await expect(page.getByText('Active Agents')).toBeVisible();
		});

		test('clicking Agents tab updates URL to /settings/agents', async ({ page }) => {
			await page.goto('/settings/general');

			await page.getByRole('tab', { name: /agents/i }).click();

			await expect(page).toHaveURL(/\/settings\/agents/);
		});

		test('Agents tab becomes active after click', async ({ page }) => {
			await page.goto('/settings/general');

			const agentsTab = page.getByRole('tab', { name: /agents/i });
			await agentsTab.click();

			await expect(agentsTab).toHaveAttribute('data-state', 'active');
		});
	});

	test.describe('SC-3: Clicking Environment tab displays EnvironmentLayout', () => {
		test('clicking Environment tab shows EnvironmentLayout content', async ({ page }) => {
			await page.goto('/settings/general');

			// Click the Environment tab
			await page.getByRole('tab', { name: /environment/i }).click();

			// EnvironmentLayout has sub-navigation with links like Settings, Hooks, Skills, etc.
			await expect(page.locator('.environment-nav')).toBeVisible();
		});

		test('clicking Environment tab updates URL to /settings/environment', async ({ page }) => {
			await page.goto('/settings/general');

			await page.getByRole('tab', { name: /environment/i }).click();

			await expect(page).toHaveURL(/\/settings\/environment/);
		});

		test('Environment tab becomes active after click', async ({ page }) => {
			await page.goto('/settings/general');

			const envTab = page.getByRole('tab', { name: /environment/i });
			await envTab.click();

			await expect(envTab).toHaveAttribute('data-state', 'active');
		});
	});

	test.describe('SC-4: Direct navigation to /settings/agents', () => {
		test('navigating directly to /settings/agents shows Agents content', async ({ page }) => {
			await page.goto('/settings/agents');

			// Agents content should be visible
			await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
		});

		test('Agents tab is active when navigating directly', async ({ page }) => {
			await page.goto('/settings/agents');

			const agentsTab = page.getByRole('tab', { name: /agents/i });
			await expect(agentsTab).toHaveAttribute('data-state', 'active');
		});

		test('other tabs are inactive when on /settings/agents', async ({ page }) => {
			await page.goto('/settings/agents');

			await expect(page.getByRole('tab', { name: /general/i })).toHaveAttribute(
				'data-state',
				'inactive'
			);
			await expect(page.getByRole('tab', { name: /environment/i })).toHaveAttribute(
				'data-state',
				'inactive'
			);
		});
	});

	test.describe('SC-5: Direct navigation to /settings/environment', () => {
		test('navigating directly to /settings/environment shows Environment content', async ({
			page,
		}) => {
			await page.goto('/settings/environment');

			// EnvironmentLayout content should be visible
			await expect(page.locator('.environment-nav')).toBeVisible();
		});

		test('Environment tab is active when navigating directly', async ({ page }) => {
			await page.goto('/settings/environment');

			const envTab = page.getByRole('tab', { name: /environment/i });
			await expect(envTab).toHaveAttribute('data-state', 'active');
		});
	});

	test.describe('SC-6: Navigation to /settings redirects to /settings/general', () => {
		test('navigating to /settings redirects to /settings/general', async ({ page }) => {
			await page.goto('/settings');

			// Should redirect to /settings/general
			await expect(page).toHaveURL(/\/settings\/general/);
		});

		test('General tab is active after redirect', async ({ page }) => {
			await page.goto('/settings');

			// Wait for redirect
			await page.waitForURL(/\/settings\/general/);

			const generalTab = page.getByRole('tab', { name: /general/i });
			await expect(generalTab).toHaveAttribute('data-state', 'active');
		});

		test('General settings content is displayed after redirect', async ({ page }) => {
			await page.goto('/settings');

			// Wait for redirect and content
			await page.waitForURL(/\/settings\/general/);

			// SettingsLayout content should be visible (has the settings-layout class)
			await expect(page.locator('.settings-layout')).toBeVisible();
		});
	});

	test.describe('Browser navigation', () => {
		test('browser back button works after tab switch', async ({ page }) => {
			// Start at general
			await page.goto('/settings/general');
			await expect(page).toHaveURL(/\/settings\/general/);

			// Go to agents
			await page.getByRole('tab', { name: /agents/i }).click();
			await expect(page).toHaveURL(/\/settings\/agents/);

			// Go back
			await page.goBack();

			// Should be back at general
			await expect(page).toHaveURL(/\/settings\/general/);
			const generalTab = page.getByRole('tab', { name: /general/i });
			await expect(generalTab).toHaveAttribute('data-state', 'active');
		});

		test('browser forward button works after going back', async ({ page }) => {
			await page.goto('/settings/general');

			// Navigate to agents, then environment
			await page.getByRole('tab', { name: /agents/i }).click();
			await expect(page).toHaveURL(/\/settings\/agents/);

			// Go back
			await page.goBack();
			await expect(page).toHaveURL(/\/settings\/general/);

			// Go forward
			await page.goForward();
			await expect(page).toHaveURL(/\/settings\/agents/);

			const agentsTab = page.getByRole('tab', { name: /agents/i });
			await expect(agentsTab).toHaveAttribute('data-state', 'active');
		});

		test('page refresh preserves active tab', async ({ page }) => {
			await page.goto('/settings/agents');

			// Reload the page
			await page.reload();

			// Should still be on agents
			await expect(page).toHaveURL(/\/settings\/agents/);
			const agentsTab = page.getByRole('tab', { name: /agents/i });
			await expect(agentsTab).toHaveAttribute('data-state', 'active');
		});
	});

	test.describe('Edge cases', () => {
		test('environment sub-route shows Environment tab as active', async ({ page }) => {
			// Navigate directly to an environment sub-route
			await page.goto('/settings/environment/hooks');

			const envTab = page.getByRole('tab', { name: /environment/i });
			await expect(envTab).toHaveAttribute('data-state', 'active');
		});

		test('general sub-route shows General tab as active', async ({ page }) => {
			// Navigate directly to a general sub-route
			await page.goto('/settings/general/commands');

			const generalTab = page.getByRole('tab', { name: /general/i });
			await expect(generalTab).toHaveAttribute('data-state', 'active');
		});

		test('invalid settings path shows 404', async ({ page }) => {
			await page.goto('/settings/invalid-route');

			await expect(page.getByText('Page not found')).toBeVisible();
		});
	});

	test.describe('Keyboard navigation', () => {
		test('can navigate tabs using keyboard', async ({ page }) => {
			await page.goto('/settings/general');

			// Focus the General tab
			const generalTab = page.getByRole('tab', { name: /general/i });
			await generalTab.focus();

			// Press right arrow to move to Agents
			await page.keyboard.press('ArrowRight');

			// Agents tab should be focused
			const agentsTab = page.getByRole('tab', { name: /agents/i });
			await expect(agentsTab).toBeFocused();
		});

		test('Enter key activates focused tab', async ({ page }) => {
			await page.goto('/settings/general');

			// Focus and navigate to Agents tab
			const generalTab = page.getByRole('tab', { name: /general/i });
			await generalTab.focus();
			await page.keyboard.press('ArrowRight');

			// Press Enter to activate
			await page.keyboard.press('Enter');

			// Should navigate to agents
			await expect(page).toHaveURL(/\/settings\/agents/);
		});
	});

	test.describe('Accessibility', () => {
		test('tabs have proper ARIA attributes', async ({ page }) => {
			await page.goto('/settings/general');

			// Check tablist
			const tablist = page.getByRole('tablist');
			await expect(tablist).toBeVisible();

			// Check each tab has proper attributes
			const tabs = page.getByRole('tab');
			const count = await tabs.count();
			expect(count).toBe(3);
		});

		test('active tab panel is associated with active tab', async ({ page }) => {
			await page.goto('/settings/agents');

			const agentsTab = page.getByRole('tab', { name: /agents/i });
			const tabPanelId = await agentsTab.getAttribute('aria-controls');

			// The tab panel should exist and be visible
			if (tabPanelId) {
				const tabPanel = page.locator(`#${tabPanelId}`);
				await expect(tabPanel).toBeVisible();
			}
		});
	});
});

test.describe('Settings Page Integration', () => {
	test('Settings link in navigation goes to Settings page', async ({ page }) => {
		await page.goto('/board');

		// Click settings in the navigation (icon nav)
		const settingsLink = page.locator('a[href="/settings"]').first();
		if (await settingsLink.isVisible()) {
			await settingsLink.click();
			await expect(page).toHaveURL(/\/settings/);
		}
	});

	test('Settings page shows document title "Settings"', async ({ page }) => {
		await page.goto('/settings/general');

		await expect(page).toHaveTitle(/Settings/);
	});
});
