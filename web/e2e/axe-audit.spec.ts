/**
 * Accessibility Audit E2E Tests
 *
 * Uses axe-core/playwright to verify WCAG 2.1 Level AA compliance across all major pages.
 * Critical and serious violations fail tests; moderate and minor are logged as warnings.
 *
 * Test Coverage (8 tests):
 * - Dashboard page
 * - Board page (flat view)
 * - Board page (swimlane view)
 * - Task list page
 * - Task detail page
 * - Initiative detail page
 * - New task modal
 * - Command palette
 *
 * @see https://www.deque.com/axe/core-documentation/api-documentation/
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Helper: Run accessibility audit and return results
async function runAccessibilityAudit(page: Page, context?: string) {
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21aa']) // WCAG 2.1 Level AA
		.analyze();

	// Log warnings for minor/moderate violations
	const minorViolations = results.violations.filter(
		(v) => v.impact === 'minor' || v.impact === 'moderate'
	);
	if (minorViolations.length > 0) {
		console.log(`\n[A11y Warnings${context ? ` - ${context}` : ''}]:`);
		minorViolations.forEach((v) => {
			console.log(`  - ${v.impact?.toUpperCase()}: ${v.id} - ${v.description}`);
			console.log(`    Help: ${v.helpUrl}`);
			v.nodes.slice(0, 3).forEach((node) => {
				console.log(`    → ${node.html.substring(0, 100)}...`);
			});
		});
	}

	// Return critical/serious violations for test assertion
	const criticalViolations = results.violations.filter(
		(v) => v.impact === 'critical' || v.impact === 'serious'
	);

	return { criticalViolations, allViolations: results.violations };
}

// Helper: Format violation details for test failure message
function formatViolations(
	violations: Awaited<ReturnType<typeof runAccessibilityAudit>>['criticalViolations']
): string {
	return violations
		.map((v) => {
			const nodes = v.nodes.slice(0, 5).map((n) => `    → ${n.html.substring(0, 120)}`);
			return `\n  [${v.impact?.toUpperCase()}] ${v.id}: ${v.description}\n  Help: ${v.helpUrl}\n${nodes.join('\n')}`;
		})
		.join('\n');
}

// Helper: Wait for page to be stable
async function waitForPageStable(page: Page) {
	await page.waitForLoadState('networkidle');
	// Wait for any loading spinners to disappear
	await page.waitForSelector('.loading-state', { state: 'hidden', timeout: 10000 }).catch(() => {});
	await page.waitForSelector('.loading-spinner', { state: 'hidden', timeout: 5000 }).catch(() => {});
	// Small buffer for animations
	await page.waitForTimeout(200);
}

// Helper: Navigate to task detail page
async function navigateToTaskDetail(page: Page): Promise<string | null> {
	await page.goto('/board');
	await waitForPageStable(page);

	const taskCards = page.locator('.task-card');
	const count = await taskCards.count();

	if (count === 0) {
		return null;
	}

	await taskCards.first().click();
	await page.waitForURL(/\/tasks\/TASK-\d+/, { timeout: 5000 });
	await waitForPageStable(page);

	const match = page.url().match(/TASK-\d+/);
	return match?.[0] || null;
}

// Helper: Navigate to an existing initiative detail page
async function navigateToInitiativeDetail(page: Page): Promise<string | null> {
	// First, try to get initiatives from the API to find an existing one
	await page.goto('/');
	await waitForPageStable(page);

	// Try to expand sidebar and find an initiative link
	const sidebar = page.locator('.sidebar');
	const isExpanded = await sidebar.evaluate((el) => el.classList.contains('expanded')).catch(() => false);
	if (!isExpanded) {
		const toggleBtn = page.locator('.toggle-btn');
		if (await toggleBtn.isVisible().catch(() => false)) {
			await toggleBtn.click();
			await page.waitForTimeout(300);
		}
	}

	// Expand initiatives section if collapsed
	const initiativesHeader = page.locator('.section-header.clickable:has-text("Initiatives")');
	if (await initiativesHeader.isVisible().catch(() => false)) {
		const initiativeList = page.locator('.initiative-list');
		if (!(await initiativeList.isVisible().catch(() => false))) {
			await initiativesHeader.click();
			await page.waitForTimeout(200);
		}
	}

	// Look for an existing initiative link
	const initiativeItems = page.locator('.initiative-item[href*="initiatives/INIT-"]');
	const count = await initiativeItems.count().catch(() => 0);

	if (count > 0) {
		const href = await initiativeItems.first().getAttribute('href');
		const match = href?.match(/INIT-\d+/);
		if (match) {
			await page.goto(`/initiatives/${match[0]}`);
			await waitForPageStable(page);
			return match[0];
		}
	}

	// No existing initiative found - skip the test
	return null;
}

test.describe('Accessibility Audit', () => {
	test.describe('Page Audits', () => {
		test('Dashboard page should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/dashboard');
			await waitForPageStable(page);

			const { criticalViolations } = await runAccessibilityAudit(page, 'Dashboard');

			expect(
				criticalViolations,
				`Dashboard has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});

		test('Board page (flat view) should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/board');
			await waitForPageStable(page);

			// Ensure we're in flat view
			const viewModeDropdown = page.locator('.view-mode-dropdown');
			const triggerText = viewModeDropdown.locator('.trigger-text');
			const currentView = await triggerText.textContent();

			if (currentView !== 'Flat') {
				// Clear localStorage and reload to get flat view
				await page.evaluate(() => localStorage.removeItem('orc-board-view-mode'));
				await page.reload();
				await waitForPageStable(page);
			}

			const { criticalViolations } = await runAccessibilityAudit(page, 'Board (Flat)');

			expect(
				criticalViolations,
				`Board (Flat) has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});

		test('Board page (swimlane view) should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/board');
			await waitForPageStable(page);

			// Switch to swimlane view
			const viewModeDropdown = page.locator('.view-mode-dropdown');
			const trigger = viewModeDropdown.locator('.dropdown-trigger');
			await trigger.click();

			const dropdownMenu = viewModeDropdown.locator('.dropdown-menu[role="listbox"]');
			await expect(dropdownMenu).toBeVisible({ timeout: 3000 });

			const swimlaneOption = viewModeDropdown.locator('.dropdown-item:has-text("By Initiative")');
			await swimlaneOption.click();
			await expect(dropdownMenu).not.toBeVisible({ timeout: 3000 });

			// Wait for swimlane view to render
			const swimlaneView = page.locator('.swimlane-view');
			await expect(swimlaneView).toBeVisible({ timeout: 5000 });
			await page.waitForTimeout(200);

			const { criticalViolations } = await runAccessibilityAudit(page, 'Board (Swimlane)');

			expect(
				criticalViolations,
				`Board (Swimlane) has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});

		test('Task list page should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/');
			await waitForPageStable(page);

			const { criticalViolations } = await runAccessibilityAudit(page, 'Task List');

			expect(
				criticalViolations,
				`Task List has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});

		test('Task detail page should have no critical/serious accessibility violations', async ({
			page
		}) => {
			const taskId = await navigateToTaskDetail(page);
			test.skip(!taskId, 'No tasks available for testing');

			const { criticalViolations } = await runAccessibilityAudit(page, 'Task Detail');

			expect(
				criticalViolations,
				`Task Detail has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});

		test('Initiative detail page should have no critical/serious accessibility violations', async ({
			page
		}) => {
			const initiativeId = await navigateToInitiativeDetail(page);
			test.skip(!initiativeId, 'No initiatives available for testing');

			const { criticalViolations } = await runAccessibilityAudit(page, 'Initiative Detail');

			expect(
				criticalViolations,
				`Initiative Detail has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);
		});
	});

	test.describe('Modal Audits', () => {
		test('New task modal should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/');
			await waitForPageStable(page);

			// Open new task modal with keyboard shortcut
			await page.keyboard.press('Shift+Alt+n');

			// Wait for modal to be visible
			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible({ timeout: 3000 });
			await page.waitForTimeout(200);

			const { criticalViolations } = await runAccessibilityAudit(page, 'New Task Modal');

			expect(
				criticalViolations,
				`New Task Modal has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);

			// Close modal
			await page.keyboard.press('Escape');
		});

		test('Command palette should have no critical/serious accessibility violations', async ({
			page
		}) => {
			await page.goto('/');
			await waitForPageStable(page);

			// Open command palette with keyboard shortcut
			await page.keyboard.press('Shift+Alt+k');

			// Wait for command palette
			const palette = page.locator('[aria-label="Command palette"]');
			await expect(palette).toBeVisible({ timeout: 3000 });
			await page.waitForTimeout(200);

			const { criticalViolations } = await runAccessibilityAudit(page, 'Command Palette');

			expect(
				criticalViolations,
				`Command Palette has ${criticalViolations.length} critical/serious violations:${formatViolations(criticalViolations)}`
			).toHaveLength(0);

			// Close palette
			await page.keyboard.press('Escape');
		});
	});
});
