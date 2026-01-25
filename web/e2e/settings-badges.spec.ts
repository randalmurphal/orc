/**
 * Settings Badge E2E Tests - TDD for TASK-533
 *
 * Tests that Settings sidebar badges show accurate counts matching actual content.
 * These tests will FAIL until the implementation replaces mock values with API data.
 *
 * Success Criteria covered:
 * - SC-1: Slash Commands badge shows actual command count from API
 * - SC-2: MCP Servers badge shows actual server count from API
 * - SC-3: Memory badge is removed until memory API exists
 */

import { test, expect } from './fixtures';

test.describe('Settings Sidebar Badges', () => {
	test.describe('SC-1: Slash Commands badge accuracy', () => {
		test('badge count matches actual number of slash commands displayed', async ({ page }) => {
			// Navigate to settings slash commands
			await page.goto('/settings/commands');

			// Get the badge count from sidebar
			const slashCommandsNavItem = page.locator('.settings-nav-item:has-text("Slash Commands")');
			const badge = slashCommandsNavItem.locator('.settings-nav-item__badge');

			// Get the actual command count from the content area
			const content = page.locator('.settings-content');

			// Check if there are commands or "No commands" message
			const noCommandsMessage = content.locator('text=/No commands|Create a command to get started/i');
			const commandCards = content.locator('.command-card, .skill-card, [class*="command-item"]');

			const hasNoCommands = await noCommandsMessage.isVisible().catch(() => false);
			const commandCount = await commandCards.count();

			if (hasNoCommands || commandCount === 0) {
				// When no commands exist, badge should NOT be visible
				await expect(badge).not.toBeVisible();
			} else {
				// When commands exist, badge should show correct count
				await expect(badge).toBeVisible();
				const badgeText = await badge.textContent();
				expect(parseInt(badgeText ?? '0', 10)).toBe(commandCount);
			}
		});

		test('badge shows no hard-coded "5" when there are no commands', async ({ page }) => {
			// This test specifically targets the bug: badge showing "5" when content shows "No commands"
			await page.goto('/settings/commands');

			// Check if content shows empty state
			const content = page.locator('.settings-content');
			const emptyState = content.locator('text=/No commands|Create a command/i');

			if (await emptyState.isVisible()) {
				// If no commands, badge must NOT show "5" (the hard-coded mock value)
				const badge = page.locator('.settings-nav-item:has-text("Slash Commands") .settings-nav-item__badge');
				const isVisible = await badge.isVisible();

				if (isVisible) {
					const badgeText = await badge.textContent();
					expect(badgeText).not.toBe('5');
				}
			}
		});
	});

	test.describe('SC-2: MCP Servers badge accuracy', () => {
		test('badge count matches actual number of MCP servers displayed', async ({ page }) => {
			// Navigate to MCP settings
			await page.goto('/settings/mcp');

			// Get the badge count from sidebar
			const mcpNavItem = page.locator('.settings-nav-item:has-text("MCP Servers")');
			const badge = mcpNavItem.locator('.settings-nav-item__badge');

			// Get the actual MCP server count from content
			const content = page.locator('.settings-content');
			const noServersMessage = content.locator('text=/No servers|No MCP servers/i');
			const serverCards = content.locator('.mcp-server-card, .server-card, [class*="server-item"]');

			const hasNoServers = await noServersMessage.isVisible().catch(() => false);
			const serverCount = await serverCards.count();

			if (hasNoServers || serverCount === 0) {
				// When no servers, badge should NOT be visible
				await expect(badge).not.toBeVisible();
			} else {
				// When servers exist, badge should show correct count
				await expect(badge).toBeVisible();
				const badgeText = await badge.textContent();
				expect(parseInt(badgeText ?? '0', 10)).toBe(serverCount);
			}
		});

		test('badge shows no hard-coded "2" when there are no MCP servers', async ({ page }) => {
			await page.goto('/settings/mcp');

			const content = page.locator('.settings-content');
			const emptyState = content.locator('text=/No servers|No MCP/i');

			if (await emptyState.isVisible()) {
				const badge = page.locator('.settings-nav-item:has-text("MCP Servers") .settings-nav-item__badge');
				const isVisible = await badge.isVisible();

				if (isVisible) {
					const badgeText = await badge.textContent();
					expect(badgeText).not.toBe('2');
				}
			}
		});
	});

	test.describe('SC-3: Memory badge removal', () => {
		test('Memory nav item has no badge', async ({ page }) => {
			await page.goto('/settings/memory');

			// Memory nav item should exist
			const memoryNavItem = page.locator('.settings-nav-item:has-text("Memory")');
			await expect(memoryNavItem).toBeVisible();

			// But it should NOT have a badge
			const badge = memoryNavItem.locator('.settings-nav-item__badge');
			await expect(badge).not.toBeVisible();
		});

		test('Memory badge does not show hard-coded "12"', async ({ page }) => {
			await page.goto('/settings');

			const memoryNavItem = page.locator('.settings-nav-item:has-text("Memory")');
			const badge = memoryNavItem.locator('.settings-nav-item__badge');

			// Badge should not exist OR should not contain "12"
			const isVisible = await badge.isVisible().catch(() => false);
			if (isVisible) {
				const badgeText = await badge.textContent();
				expect(badgeText).not.toBe('12');
			}
		});
	});

	test.describe('Badge consistency with API', () => {
		test('all badges reflect actual data, not mock values', async ({ page }) => {
			await page.goto('/settings/commands');

			// Wait for page to fully load
			await page.waitForLoadState('networkidle');

			// Check that none of the hard-coded mock values appear incorrectly
			const allBadges = page.locator('.settings-nav-item__badge');
			const badgeCount = await allBadges.count();

			// Collect all badge values
			const badgeValues: string[] = [];
			for (let i = 0; i < badgeCount; i++) {
				const text = await allBadges.nth(i).textContent();
				if (text) badgeValues.push(text);
			}

			// If we see the exact mock values (5, 2, 12), that's suspicious
			// This is a heuristic test - if all three appear, likely still using mocks
			const hasAllMockValues =
				badgeValues.includes('5') && badgeValues.includes('2') && badgeValues.includes('12');

			expect(hasAllMockValues).toBe(false);
		});
	});
});
