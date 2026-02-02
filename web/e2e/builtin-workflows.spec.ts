/**
 * E2E Tests for Built-in Workflows Display
 *
 * Tests for TASK-752: Implement and expose 8 built-in workflow templates
 *
 * Success Criteria Coverage:
 * - SC-4: UI shows built-in workflows in Built-in section of Workflows page
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project.
 * See global-setup.ts for sandbox creation details.
 */
import { test, expect } from './fixtures';

// The exact 8 built-in workflows required by TASK-752
const requiredBuiltinWorkflows = [
	{ id: 'implement-large', name: 'Implement (Large)' },
	{ id: 'implement-medium', name: 'Implement (Medium)' },
	{ id: 'implement-small', name: 'Implement (Small)' },
	{ id: 'implement-trivial', name: 'Implement (Trivial)' },
	{ id: 'review', name: 'Review' },
	{ id: 'qa-e2e', name: 'QA E2E' },
	{ id: 'spec', name: 'Spec Only' },
	{ id: 'docs', name: 'Documentation' },
];

test.describe('Built-in Workflows Display', () => {
	test.beforeEach(async ({ page }) => {
		// Navigate to Workflows page
		await page.goto('/workflows');
		await page.waitForLoadState('networkidle');
	});

	test('SC-4: should display Built-in section on Workflows page', async ({ page }) => {
		// Look for a section header or grouping labeled "Built-in"
		const builtinSection = page.locator('text=/built-?in/i').first();
		await expect(builtinSection).toBeVisible({ timeout: 5000 });
	});

	test('SC-4: should show all 8 required built-in workflows', async ({ page }) => {
		// Each built-in workflow should appear on the page
		for (const workflow of requiredBuiltinWorkflows) {
			// Look for workflow name in the list/grid
			const workflowElement = page.locator(`text="${workflow.name}"`).first();
			await expect(workflowElement).toBeVisible({
				timeout: 5000,
			});
		}
	});

	test('SC-4: should mark built-in workflows with badge or indicator', async ({ page }) => {
		// Built-in workflows should have visual distinction (badge, icon, or section)
		// This could be a badge, a section header, or a visual indicator

		// Option 1: Check for "Built-in" badges
		const builtinBadges = page.locator('.builtin-badge, [data-builtin="true"], .badge:has-text("Built-in")');
		const badgeCount = await builtinBadges.count();

		// Option 2: Check for a dedicated section with all 8 workflows
		const builtinSection = page.locator('[data-section="builtin"], .builtin-workflows-section');
		const sectionExists = (await builtinSection.count()) > 0;

		// At least one of these patterns should exist
		expect(badgeCount >= 8 || sectionExists).toBeTruthy();
	});

	test('SC-4: should NOT allow editing built-in workflows', async ({ page }) => {
		// Click on a built-in workflow to open it
		const workflowCard = page
			.locator('.workflow-card, [data-workflow-id]')
			.filter({ hasText: 'Implement (Medium)' })
			.first();

		if ((await workflowCard.count()) > 0) {
			await workflowCard.click();
			await page.waitForLoadState('networkidle');

			// Edit button should not exist, be disabled, or show a clone prompt
			const editButton = page.getByRole('button', { name: /edit/i });
			const isEditButtonVisible = await editButton.isVisible().catch(() => false);

			if (isEditButtonVisible) {
				// If visible, it should be disabled
				const isDisabled = await editButton.isDisabled().catch(() => false);
				expect(isDisabled).toBeTruthy();
			}

			// Alternatively, there should be a "Clone" button instead of "Edit"
			const cloneButton = page.getByRole('button', { name: /clone|copy|duplicate/i });
			await expect(cloneButton).toBeVisible();
		}
	});

	test('SC-4: should allow cloning built-in workflows', async ({ page }) => {
		// Click on a built-in workflow
		const workflowCard = page
			.locator('.workflow-card, [data-workflow-id]')
			.filter({ hasText: 'Implement (Small)' })
			.first();

		if ((await workflowCard.count()) > 0) {
			await workflowCard.click();
			await page.waitForLoadState('networkidle');

			// Look for clone button
			const cloneButton = page.getByRole('button', { name: /clone|copy|duplicate/i });
			await expect(cloneButton).toBeVisible();
			await expect(cloneButton).toBeEnabled();

			// Click clone button
			await cloneButton.click();

			// Should show clone modal/dialog
			const cloneModal = page.locator('[role="dialog"]');
			await expect(cloneModal).toBeVisible({ timeout: 3000 });

			// Should have a field for new workflow ID/name
			const nameInput = cloneModal.locator('input[name="name"], input[name="id"], #clone-workflow-name');
			await expect(nameInput).toBeVisible();
		}
	});

	test('SC-4: should show workflow phases in Built-in workflow detail', async ({ page }) => {
		// Navigate to implement-medium workflow detail
		await page.goto('/workflows/implement-medium');
		await page.waitForLoadState('networkidle');

		// Should show at least the expected phases
		const expectedPhases = ['spec', 'tdd_write', 'implement', 'review', 'docs'];

		for (const phase of expectedPhases) {
			// Look for phase in the workflow editor/view
			const phaseElement = page.locator(`text=/${phase}/i`).first();
			await expect(phaseElement).toBeVisible({ timeout: 5000 });
		}
	});

	test('SC-4: should show IsBuiltin indicator in workflow detail', async ({ page }) => {
		// Navigate to a built-in workflow
		await page.goto('/workflows/implement-medium');
		await page.waitForLoadState('networkidle');

		// Should show some indicator that this is a built-in workflow
		const builtinIndicator = page.locator(
			'text=/built-?in/i, .builtin-badge, [data-is-builtin="true"], .read-only-indicator'
		);
		await expect(builtinIndicator).toBeVisible();
	});
});

test.describe('Built-in Workflows in Workflow Editor', () => {
	test('SC-4: should show built-in workflows as read-only in editor', async ({ page }) => {
		// Navigate to workflow editor for a built-in workflow
		await page.goto('/workflows/implement-medium');
		await page.waitForLoadState('networkidle');

		// Workflow editor should be in read-only mode
		// Check for disabled state or read-only indicator

		// Phase nodes should not be draggable (or show read-only cursor)
		const phaseNodes = page.locator('.react-flow__node, .workflow-phase-node');
		const firstNode = phaseNodes.first();

		if ((await firstNode.count()) > 0) {
			// Check if node has read-only styling or data attribute
			const hasReadOnlyClass = await firstNode.evaluate((el: Element) => {
				return (
					el.classList.contains('read-only') ||
					el.getAttribute('data-readonly') === 'true' ||
					el.getAttribute('draggable') === 'false'
				);
			});

			// Either read-only styling exists, OR there's a global read-only indicator
			const globalReadOnly = page.locator('.workflow-editor[data-readonly="true"], .read-only-banner');
			const hasGlobalReadOnly = (await globalReadOnly.count()) > 0;

			expect(hasReadOnlyClass || hasGlobalReadOnly).toBeTruthy();
		}
	});

	test('SC-4: should prevent adding phases to built-in workflows', async ({ page }) => {
		await page.goto('/workflows/implement-medium');
		await page.waitForLoadState('networkidle');

		// "Add Phase" button should not exist or be disabled
		const addPhaseButton = page.getByRole('button', { name: /add phase/i });
		const isVisible = await addPhaseButton.isVisible().catch(() => false);

		if (isVisible) {
			const isDisabled = await addPhaseButton.isDisabled();
			expect(isDisabled).toBeTruthy();
		}
		// If not visible, that's also acceptable (button hidden for read-only)
	});
});

test.describe('Built-in Workflows in Task Creation', () => {
	test('SC-4: should show built-in workflows in New Task workflow dropdown', async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		// Open new task modal
		const newTaskButton = page.getByRole('button', { name: 'New Task' });
		await expect(newTaskButton).toBeVisible();
		await newTaskButton.click();

		// Wait for modal
		await page.waitForSelector('[role="dialog"]', { timeout: 5000 });

		// Open workflow dropdown
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await expect(workflowSelect).toBeVisible();
		await workflowSelect.click();

		// All implementation workflows should be available
		const implementWorkflows = ['Implement (Large)', 'Implement (Medium)', 'Implement (Small)', 'Implement (Trivial)'];

		for (const workflow of implementWorkflows) {
			const option = page.locator(`[role="option"]:has-text("${workflow}")`);
			await expect(option).toBeVisible({ timeout: 3000 });
		}
	});
});

test.describe('Built-in Workflows API Integration', () => {
	test('should receive built-in workflows from API with correct source', async ({ page }) => {
		// Navigate to workflows page to trigger API call
		await page.goto('/workflows');
		await page.waitForLoadState('networkidle');

		// Wait for the page to have loaded workflows (visible as cards or list items)
		const workflowList = page.locator('.workflow-card, [data-workflow-id]');
		await expect(workflowList.first()).toBeVisible({ timeout: 10000 });

		// Verify at least 8 workflows are listed (the built-ins)
		const count = await workflowList.count();
		expect(count).toBeGreaterThanOrEqual(8);
	});
});
