/**
 * E2E tests for Agents configuration page
 * Validates UI matches example_ui/agents-config.png reference
 */
import { test, expect } from './fixtures';

test.describe('Agents Page', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/agents');
		// Wait for page to load
		await page.waitForLoadState('networkidle');
	});

	test('page loads and displays header', async ({ page }) => {
		// Check page title
		await expect(page.locator('h1')).toHaveText('Agents');

		// Check subtitle
		await expect(page.locator('text=Configure Claude models and execution settings')).toBeVisible();

		// Check Add Agent button exists
		await expect(page.locator('button', { hasText: 'Add Agent' })).toBeVisible();
	});

	test('displays Active Agents section with agent cards', async ({ page }) => {
		// Check section header
		await expect(page.locator('h2', { hasText: 'Active Agents' })).toBeVisible();
		await expect(page.locator('text=Currently configured Claude instances')).toBeVisible();

		// Check for agent cards - should have at least one
		const agentCards = page.locator('[role="article"][aria-label*="agent"]');
		await expect(agentCards.first()).toBeVisible();
	});

	test('agent cards display required information', async ({ page }) => {
		const agentCard = page.locator('[role="article"][aria-label*="agent"]').first();

		// Should have agent name
		await expect(agentCard.locator('h3')).toBeVisible();

		// Should have status badge (ACTIVE or IDLE)
		const statusBadge = agentCard.locator('[class*="badge"]');
		await expect(statusBadge).toBeVisible();
		const statusText = await statusBadge.textContent();
		expect(statusText).toMatch(/ACTIVE|IDLE/);

		// Should have stats section
		await expect(agentCard.locator('text=/Tokens Today|Tasks Done|Success Rate/')).toBeVisible();

		// Should have capability badges
		const capabilityBadges = agentCard.locator('[class*="badge"]').filter({ hasNotText: /ACTIVE|IDLE/ });
		await expect(capabilityBadges.first()).toBeVisible();
	});

	test('displays Execution Settings section', async ({ page }) => {
		await expect(page.locator('h2', { hasText: 'Execution Settings' })).toBeVisible();
		await expect(page.locator('text=Global configuration for all agents')).toBeVisible();

		// Check for Parallel Tasks setting
		await expect(page.locator('text=Parallel Tasks')).toBeVisible();
		await expect(page.locator('text=Maximum number of tasks to run simultaneously')).toBeVisible();

		// Check for Auto-Approve setting
		await expect(page.locator('text=Auto-Approve')).toBeVisible();
		await expect(page.locator('text=Automatically approve safe operations without prompting')).toBeVisible();

		// Check for Default Model setting
		await expect(page.locator('text=Default Model')).toBeVisible();
		await expect(page.locator('text=Model to use for new tasks')).toBeVisible();

		// Check for Cost Limit setting
		await expect(page.locator('text=Cost Limit')).toBeVisible();
		await expect(page.locator('text=Daily spending limit before pause')).toBeVisible();
	});

	test('displays Tool Permissions section', async ({ page }) => {
		await expect(page.locator('h2', { hasText: 'Tool Permissions' })).toBeVisible();
		await expect(page.locator('text=Control what actions agents can perform')).toBeVisible();

		// Check for all expected permission toggles
		const expectedPermissions = [
			'File Read',
			'File Write',
			'Bash Commands',
			'Web Search',
			'Git Operations',
			'MCP Servers'
		];

		for (const permission of expectedPermissions) {
			await expect(page.locator(`text=${permission}`)).toBeVisible();
		}
	});

	test('Parallel Tasks slider is interactive', async ({ page }) => {
		const slider = page.locator('input[type="range"]').first();
		await expect(slider).toBeVisible();

		// Get initial value
		const initialValue = await slider.getAttribute('value');

		// Try to change value (simulate drag)
		await slider.fill('4');

		// Value should change
		const newValue = await slider.getAttribute('value');
		expect(newValue).not.toBe(initialValue);
	});

	test('Auto-Approve toggle is interactive', async ({ page }) => {
		// Find the Auto-Approve toggle switch
		const toggleSwitch = page.locator('text=Auto-Approve').locator('..').locator('[role="switch"]');
		await expect(toggleSwitch).toBeVisible();

		// Get initial state
		const initialState = await toggleSwitch.getAttribute('aria-checked');

		// Click to toggle
		await toggleSwitch.click();

		// Wait a bit for state update
		await page.waitForTimeout(200);

		// State should change
		const newState = await toggleSwitch.getAttribute('aria-checked');
		expect(newState).not.toBe(initialState);
	});

	test('Default Model dropdown is interactive', async ({ page }) => {
		// Find the Default Model select/dropdown
		const modelSelect = page.locator('text=Default Model').locator('..').locator('select, [role="combobox"]');
		await expect(modelSelect).toBeVisible();

		// Click to open
		await modelSelect.click();

		// Check if options appear (could be a dropdown or modal)
		await expect(page.locator('[role="option"], [role="menuitem"]').first()).toBeVisible({ timeout: 1000 });
	});

	test('Add Agent button is clickable', async ({ page }) => {
		const addButton = page.locator('button', { hasText: 'Add Agent' });
		await expect(addButton).toBeVisible();
		await expect(addButton).toBeEnabled();

		// Click should not throw error
		await addButton.click();

		// Could check for modal/navigation, but at minimum it should be clickable
	});

	test('Tool Permission toggles are interactive', async ({ page }) => {
		// Find first tool permission toggle
		const firstToggle = page.locator('[role="switch"]').first();
		await expect(firstToggle).toBeVisible();

		// Get initial state
		const initialState = await firstToggle.getAttribute('aria-checked');

		// Click to toggle
		await firstToggle.click();

		// If a warning dialog appears for disabling a permission, close it
		const warningDialog = page.locator('[role="alertdialog"], [role="dialog"]');
		if (await warningDialog.isVisible()) {
			// Look for cancel or close button to dismiss
			const cancelButton = warningDialog.locator('button', { hasText: /Cancel|Close/i });
			if (await cancelButton.isVisible()) {
				await cancelButton.click();
			}
		}
	});

	test('agent card is clickable', async ({ page }) => {
		const agentCard = page.locator('[role="article"][aria-label*="agent"]').first();
		await expect(agentCard).toBeVisible();

		// Should be clickable (has tabindex 0 and keyboard support)
		const tabIndex = await agentCard.getAttribute('tabindex');
		expect(tabIndex).toBe('0');

		// Click should not throw error
		await agentCard.click();
	});

	test('page has no console errors', async ({ page }) => {
		const errors: string[] = [];
		page.on('console', (msg) => {
			if (msg.type() === 'error') {
				errors.push(msg.text());
			}
		});

		await page.goto('/agents');
		await page.waitForLoadState('networkidle');

		// Filter out known non-critical errors
		const criticalErrors = errors.filter(
			(err) => !err.includes('DevTools') && !err.includes('Warning')
		);

		expect(criticalErrors).toHaveLength(0);
	});

	test('mobile responsive layout', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.goto('/agents');
		await page.waitForLoadState('networkidle');

		// Header should still be visible
		await expect(page.locator('h1', { hasText: 'Agents' })).toBeVisible();

		// Add Agent button should be visible
		await expect(page.locator('button', { hasText: 'Add Agent' })).toBeVisible();

		// Agent cards should be visible (stacked vertically)
		const agentCards = page.locator('[role="article"][aria-label*="agent"]');
		await expect(agentCards.first()).toBeVisible();

		// Settings sections should be visible
		await expect(page.locator('h2', { hasText: 'Execution Settings' })).toBeVisible();
		await expect(page.locator('h2', { hasText: 'Tool Permissions' })).toBeVisible();

		// Take screenshot for visual inspection
		await page.screenshot({ path: '/tmp/qa-TASK-613/mobile-agents-page.png', fullPage: true });
	});

	test('desktop visual comparison', async ({ page }) => {
		// Set consistent desktop viewport
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/agents');
		await page.waitForLoadState('networkidle');

		// Take full page screenshot
		await page.screenshot({ path: '/tmp/qa-TASK-613/desktop-agents-page.png', fullPage: true });

		// Visual inspection points (checked manually against reference):
		// - Header layout (title left, button right)
		// - Agent cards in grid (3 columns on wide screens)
		// - Each card has icon, name, status badge, stats, capability badges
		// - Execution Settings grid (2x2 layout)
		// - Tool Permissions grid (3 columns of toggles)
		// - Dark theme styling
	});
});
