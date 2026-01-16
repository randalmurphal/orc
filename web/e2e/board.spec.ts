/**
 * Board Page E2E Tests
 *
 * Framework-agnostic tests for the Board/Kanban page.
 * These tests define BEHAVIOR, not implementation, to work on both
 * Svelte (current) and React (future migration) implementations.
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project created by
 * global-setup.ts. Tests perform real actions (clicks) that may modify
 * task states. The sandbox ensures real production tasks are NEVER affected.
 *
 * Test Coverage (13 tests):
 * - Board Rendering (4): columns, headers, task cards, counts
 * - View Mode Toggle (5): flat/swimlane views, persistence, filtering
 * - Swimlane View (4): grouping, collapse/expand, persistence, unassigned
 *
 * Selector Strategy (priority order):
 * 1. role/aria-label - getByRole(), locator('[aria-label="..."]')
 * 2. Semantic text - getByText(), :has-text()
 * 3. Structural classes - .task-card, .column, .swimlane
 * 4. data-testid - for elements without semantic meaning
 *
 * Avoid: Framework-specific classes (.svelte-xyz), deep DOM paths
 *
 * @see web/CLAUDE.md for selector strategy documentation
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper function to wait for board to load
async function waitForBoardLoad(page: Page) {
	// Wait for the board page to render
	await page.waitForSelector('.board-page', { timeout: 10000 });
	// Wait for loading state to disappear
	await page.waitForSelector('.loading-state', { state: 'hidden', timeout: 10000 }).catch(() => {});
	// Give a small buffer for any animations
	await page.waitForTimeout(100);
}

// Helper to get all column headers
async function getColumnHeaders(page: Page): Promise<string[]> {
	const headers = await page.locator('[role="region"][aria-label*="column"] .column-header h2').allTextContents();
	return headers;
}

// Helper to clear localStorage for test isolation
async function clearBoardStorage(page: Page) {
	await page.evaluate(() => {
		localStorage.removeItem('orc-board-view-mode');
		localStorage.removeItem('orc-collapsed-swimlanes');
		localStorage.removeItem('orc-show-backlog');
	});
}

// Helper to switch to swimlane view with retry for flaky dropdown
async function switchToSwimlaneView(page: Page) {
	const viewModeDropdown = page.locator('.view-mode-dropdown');
	const trigger = viewModeDropdown.locator('.dropdown-trigger');
	await expect(trigger).toBeVisible({ timeout: 5000 });

	// Radix Select portals the content to document.body, so look globally
	// Use role="listbox" which is the accessible role Radix adds
	const dropdownMenu = page.locator('[role="listbox"]');
	const swimlaneOption = page.locator('[role="option"]:has-text("By Initiative")');

	// Retry loop for flaky dropdown - sometimes first click doesn't register
	for (let attempt = 0; attempt < 3; attempt++) {
		await trigger.click();

		// Wait a bit for dropdown animation
		await page.waitForTimeout(150);

		// Check if dropdown opened
		const isOpen = await dropdownMenu.isVisible().catch(() => false);
		if (isOpen) {
			break;
		}
	}

	await expect(dropdownMenu).toBeVisible({ timeout: 3000 });

	// Click on "By Initiative" option
	await expect(swimlaneOption).toBeVisible({ timeout: 3000 });
	await swimlaneOption.click();

	// Wait for dropdown to close
	await expect(dropdownMenu).not.toBeVisible({ timeout: 3000 });

	// Verify swimlane view is visible and stable
	const swimlaneView = page.locator('.swimlane-view');
	await expect(swimlaneView).toBeVisible({ timeout: 5000 });

	// Wait for swimlane content to render
	await page.waitForTimeout(100);
}

test.describe('Board Page', () => {
	test.describe('Board Rendering', () => {
		test('should display board page with all 6 columns (Queued, Spec, Implement, Test, Review, Done)', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Check for all 6 columns by their aria-label
			const expectedColumns = ['Queued', 'Spec', 'Implement', 'Test', 'Review', 'Done'];

			for (const columnName of expectedColumns) {
				const column = page.locator(`[role="region"][aria-label="${columnName} column"]`);
				await expect(column).toBeVisible();
			}

			// Verify we have exactly 6 columns
			const allColumns = page.locator('[role="region"][aria-label*="column"]');
			await expect(allColumns).toHaveCount(6);
		});

		test('should show correct column headers and task counts', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Check each column has a header with title and count
			const columns = page.locator('[role="region"][aria-label*="column"]');
			const count = await columns.count();

			for (let i = 0; i < count; i++) {
				const column = columns.nth(i);

				// Each column should have an h2 header
				const header = column.locator('.column-header h2');
				await expect(header).toBeVisible();
				const headerText = await header.textContent();
				expect(headerText).toBeTruthy();

				// Each column should have a count badge
				const countBadge = column.locator('.column-header .count');
				await expect(countBadge).toBeVisible();

				// Count should be a number
				const countText = await countBadge.textContent();
				expect(countText).toMatch(/^\d+$/);
			}
		});

		test('should render task cards in correct columns based on status/phase', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Get all task cards in the board
			const taskCards = page.locator('.task-card');
			const cardCount = await taskCards.count();

			if (cardCount > 0) {
				// For each task card, verify it's in a column (not the board itself)
				for (let i = 0; i < Math.min(cardCount, 5); i++) {
					const card = taskCards.nth(i);
					// Card should be inside a column region (use aria-label*="column" to exclude the board)
					const parentColumn = card.locator('xpath=ancestor::div[@role="region"][contains(@aria-label, "column")]');
					await expect(parentColumn).toBeVisible();
				}

				// Verify at least one task has a task ID
				const firstCard = taskCards.first();
				const taskId = firstCard.locator('.task-id');
				await expect(taskId).toBeVisible();
				const idText = await taskId.textContent();
				expect(idText).toMatch(/TASK-\d+/);
			}
		});

		test('should show task count in header', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// The page header should show task count
			const header = page.locator('.page-header');
			await expect(header).toBeVisible();

			// Look for the task count element
			const taskCount = page.locator('.task-count');
			await expect(taskCount).toBeVisible();

			// Should contain "tasks" text
			const countText = await taskCount.textContent();
			expect(countText).toMatch(/\d+\s+tasks?/);
		});
	});

	test.describe('View Mode Toggle', () => {
		test('should default to flat view mode', async ({ page }) => {
			// Clear any stored view mode
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// The view mode dropdown should show "Flat" by default
			const viewModeDropdown = page.locator('.view-mode-dropdown');
			await expect(viewModeDropdown).toBeVisible();

			const triggerText = viewModeDropdown.locator('.trigger-text');
			await expect(triggerText).toHaveText('Flat');

			// The board should be in flat mode (not swimlane)
			const board = page.locator('.board');
			await expect(board).toBeVisible();

			// Swimlane view should NOT be visible
			const swimlaneView = page.locator('.swimlane-view');
			await expect(swimlaneView).not.toBeVisible();
		});

		test('should switch to swimlane view when selected', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Open view mode dropdown
			const viewModeDropdown = page.locator('.view-mode-dropdown');
			const trigger = viewModeDropdown.locator('.dropdown-trigger');
			await expect(trigger).toBeVisible();

			// Click and wait for dropdown to open
			// Radix Select portals content to document.body, use role="listbox"
			await trigger.click();
			const dropdownMenu = page.locator('[role="listbox"]');
			await expect(dropdownMenu).toBeVisible({ timeout: 3000 });

			// Click on "By Initiative" option (Radix uses role="option")
			const swimlaneOption = page.locator('[role="option"]:has-text("By Initiative")');
			await expect(swimlaneOption).toBeVisible();
			await swimlaneOption.click();

			// Wait for view to change and dropdown to close
			await expect(dropdownMenu).not.toBeVisible({ timeout: 2000 });

			// Verify swimlane view is now visible
			const swimlaneView = page.locator('.board.swimlane-view');
			await expect(swimlaneView).toBeVisible({ timeout: 3000 });

			// Flat view class should NOT be present
			const flatView = page.locator('.board.flat-view');
			await expect(flatView).not.toBeVisible();

			// Dropdown should now show "By Initiative"
			const triggerText = viewModeDropdown.locator('.trigger-text');
			await expect(triggerText).toHaveText('By Initiative');
		});

		test('should persist view mode in localStorage', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Switch to swimlane view using helper
			await switchToSwimlaneView(page);

			// Verify localStorage was updated
			const storedMode = await page.evaluate(() => localStorage.getItem('orc-board-view-mode'));
			expect(storedMode).toBe('swimlane');

			// Reload the page
			await page.reload();
			await waitForBoardLoad(page);

			// View should still be swimlane after reload (with increased timeout for stability)
			const swimlaneView = page.locator('.swimlane-view');
			await expect(swimlaneView).toBeVisible({ timeout: 5000 });

			// Dropdown should still show "By Initiative"
			const viewModeDropdown = page.locator('.view-mode-dropdown');
			const triggerText = viewModeDropdown.locator('.trigger-text');
			await expect(triggerText).toHaveText('By Initiative');
		});

		test('should disable swimlane toggle when initiative filter active', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// First check if there are any initiatives to filter by
			const initiativeDropdown = page.locator('.initiative-dropdown, [data-testid="initiative-dropdown"]');

			// Check if there are initiatives available
			const hasInitiatives = await initiativeDropdown.isVisible().catch(() => false);

			if (hasInitiatives) {
				// Click to open initiative filter
				await initiativeDropdown.click();
				await page.waitForTimeout(100);

				// Look for any initiative option (not "All initiatives")
				const initiativeOptions = page.locator('.dropdown-item').filter({
					hasNot: page.locator(':has-text("All initiatives")')
				});

				const optionCount = await initiativeOptions.count();

				if (optionCount > 0) {
					// Select an initiative
					await initiativeOptions.first().click();
					await page.waitForTimeout(200);

					// View mode dropdown should be disabled (wrapped in .view-mode-disabled)
					const viewModeDisabled = page.locator('.view-mode-disabled');
					await expect(viewModeDisabled).toBeVisible();
				}
			}
		});

		test('should show initiative banner when filtering', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Find and click initiative filter
			const initiativeDropdown = page.locator('.initiative-dropdown, [data-testid="initiative-dropdown"]');
			const hasInitiatives = await initiativeDropdown.isVisible().catch(() => false);

			if (hasInitiatives) {
				await initiativeDropdown.click();
				await page.waitForTimeout(100);

				// Look for "Unassigned" option which should always exist
				const unassignedOption = page.locator('.dropdown-item:has-text("Unassigned")');
				const hasUnassigned = await unassignedOption.isVisible().catch(() => false);

				if (hasUnassigned) {
					await unassignedOption.click();
					await page.waitForTimeout(200);

					// Initiative banner should appear
					const banner = page.locator('.initiative-banner');
					await expect(banner).toBeVisible();

					// Banner should have clear filter button
					const clearBtn = banner.locator('.banner-clear');
					await expect(clearBtn).toBeVisible();
					await expect(clearBtn).toHaveText('Clear filter');
				}
			}
		});
	});

	test.describe('Swimlane View', () => {
		test('should group tasks by initiative in swimlane view', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Switch to swimlane view using helper
			await switchToSwimlaneView(page);

			// Should have swimlane headers for columns
			const swimlaneHeaders = page.locator('.swimlane-headers');
			await expect(swimlaneHeaders).toBeVisible();

			// Should have at least one swimlane (Unassigned always exists if there are tasks)
			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			// If there are tasks, there should be at least one swimlane
			// Use the header task count (specific to page header, not swimlane counts)
			const headerTaskCount = page.locator('.page-header .task-count');
			const taskCountText = await headerTaskCount.textContent();
			const numTasks = parseInt(taskCountText?.match(/\d+/)?.[0] || '0');

			if (numTasks > 0) {
				expect(swimlaneCount).toBeGreaterThan(0);
			}
		});

		test('should collapse/expand swimlanes', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Switch to swimlane view using helper
			await switchToSwimlaneView(page);

			// Find swimlanes
			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			if (swimlaneCount > 0) {
				const firstSwimlane = swimlanes.first();

				// Initially expanded - content should be visible
				const swimlaneContent = firstSwimlane.locator('.swimlane-content');
				await expect(swimlaneContent).toBeVisible({ timeout: 3000 });

				// Click header to collapse
				const header = firstSwimlane.locator('.swimlane-header');
				await header.click();

				// Content should now be hidden (wait for animation)
				await expect(swimlaneContent).not.toBeVisible({ timeout: 3000 });

				// Swimlane should have collapsed class
				await expect(firstSwimlane).toHaveClass(/collapsed/, { timeout: 2000 });

				// Click again to expand
				await header.click();

				// Content should be visible again (wait for animation)
				await expect(swimlaneContent).toBeVisible({ timeout: 3000 });
				await expect(firstSwimlane).not.toHaveClass(/collapsed/, { timeout: 2000 });
			}
		});

		test('should persist collapsed state in localStorage', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Switch to swimlane view using helper
			await switchToSwimlaneView(page);

			// Find swimlanes
			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			if (swimlaneCount > 0) {
				const firstSwimlane = swimlanes.first();

				// Collapse it
				const header = firstSwimlane.locator('.swimlane-header');
				await header.click();

				// Verify collapsed (wait for animation)
				await expect(firstSwimlane).toHaveClass(/collapsed/, { timeout: 3000 });

				// Check localStorage
				const storedState = await page.evaluate(() =>
					localStorage.getItem('orc-collapsed-swimlanes')
				);
				expect(storedState).toBeTruthy();

				// Reload the page
				await page.reload();
				await waitForBoardLoad(page);

				// Should still be in swimlane view and collapsed state should persist
				const swimlaneView = page.locator('.swimlane-view');
				await expect(swimlaneView).toBeVisible();

				const swimlanesAfterReload = page.locator('.swimlane');
				const firstSwimlaneAfterReload = swimlanesAfterReload.first();

				// Should still be collapsed
				await expect(firstSwimlaneAfterReload).toHaveClass(/collapsed/);
			}
		});

		test('should show Unassigned swimlane for tasks without initiative', async ({ page }) => {
			await page.goto('/board');
			await clearBoardStorage(page);
			await page.reload();
			await waitForBoardLoad(page);

			// Switch to swimlane view using helper
			await switchToSwimlaneView(page);

			// Look for Unassigned swimlane
			const unassignedSwimlane = page.locator('.swimlane:has(.swimlane-title:has-text("Unassigned"))');

			// If there are tasks without initiatives, Unassigned swimlane should exist
			// Use the header task count (specific to page header, not swimlane counts)
			const headerTaskCount = page.locator('.page-header .task-count');
			const taskCountText = await headerTaskCount.textContent();
			const numTasks = parseInt(taskCountText?.match(/\d+/)?.[0] || '0');

			if (numTasks > 0) {
				// Check if Unassigned swimlane exists
				const hasUnassigned = await unassignedSwimlane.isVisible().catch(() => false);

				// Either there are unassigned tasks (swimlane visible) or all tasks have initiatives
				// Both are valid states
				if (hasUnassigned) {
					// Verify it has the expected structure
					const title = unassignedSwimlane.locator('.swimlane-title');
					await expect(title).toHaveText('Unassigned');

					// Should have task count badge in the swimlane header
					const taskCountBadge = unassignedSwimlane.locator('.swimlane-header .task-count');
					await expect(taskCountBadge).toBeVisible();
				}
			}
		});
	});
});
