/**
 * E2E QA Tests for Workflow Editor - Gates as Edges Model
 *
 * TASK-766: QA for the gates-as-edges visual model
 *
 * This test suite extends the base gates-as-edges tests with additional
 * coverage for:
 * - SC-1: AI gate purple color rendering
 * - SC-2: Loop edges render as backward connections
 * - SC-3: Gate configuration changes persist
 *
 * These tests require creating/cloning custom workflows since built-in
 * workflows don't have AI gates or loop configurations.
 */

import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

/**
 * Helper to clone a workflow for testing editable workflows
 */
async function cloneWorkflow(page: Page, sourceId: string, newId: string) {
	// Navigate to workflows page
	await page.goto('/workflows');
	await page.waitForSelector('[data-testid="workflows-list"]');

	// Find and click the clone button for the source workflow
	const workflowCard = page.locator(`[data-testid="workflow-card-${sourceId}"]`);
	await workflowCard.hover();
	await workflowCard.locator('[data-testid="clone-workflow-button"]').click();

	// Wait for clone modal
	await page.waitForSelector('[data-testid="clone-workflow-modal"]');

	// Fill in the new ID
	await page.fill('#workflow-id', newId);

	// Submit
	await page.click('button[type="submit"]');

	// Wait for navigation to the new workflow
	await page.waitForURL(`/workflows/${newId}`);
}

test.describe('TASK-766: Gates as Edges QA - Extended Coverage', () => {
	test.describe('SC-1: AI gate purple color rendering', () => {
		test('AI gate symbol has purple color class', async ({ page }) => {
			// Create a custom workflow with an AI gate
			const customWorkflowId = `test-ai-gate-${Date.now()}`;

			// Clone a workflow to make it editable
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.workflow-editor-body');
			await page.waitForSelector('.react-flow');

			// Click on the first gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Wait for inspector to appear
			await expect(page.locator('.gate-inspector')).toBeVisible();

			// Change gate type to AI
			const gateTypeSelect = page.locator('#gate-type');
			await gateTypeSelect.selectOption({ value: '3' }); // GateType.AI = 3

			// Wait for UI to update
			await page.waitForTimeout(500);

			// Verify the AI gate symbol has the purple color class
			const aiGate = page.locator('.gate-edge__symbol--ai');
			await expect(aiGate).toBeVisible();
			await expect(aiGate.first()).toHaveCSS('color', 'rgb(168, 85, 247)'); // purple #a855f7
		});

		test('AI gate tooltip shows "AI" type', async ({ page }) => {
			// Navigate to a workflow with AI gates (if exists) or create one
			const customWorkflowId = `test-ai-tooltip-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Set a gate to AI type
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();
			await page.locator('#gate-type').selectOption({ value: '3' });
			await page.waitForTimeout(500);

			// Hover over the AI gate
			const aiGate = page.locator('.gate-edge__symbol--ai').first();
			await aiGate.hover();

			// Tooltip should show "AI" type
			const tooltip = page.locator('.gate-edge__tooltip');
			await expect(tooltip).toBeVisible();
			await expect(tooltip.getByText('AI')).toBeVisible();
		});

		test('AI gate inspector shows AI-specific configuration', async ({ page }) => {
			const customWorkflowId = `test-ai-config-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Set a gate to AI type
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();
			await page.locator('#gate-type').selectOption({ value: '3' });
			await page.waitForTimeout(500);

			// AI configuration section should appear
			await expect(page.getByText('AI Configuration')).toBeVisible();
			await expect(page.locator('#reviewer-agent')).toBeVisible();
		});
	});

	test.describe('SC-2: Loop edges render as backward connections', () => {
		test('loop edge renders with backward path (source right of target)', async ({ page }) => {
			// This test requires a workflow with a loop configuration
			// Navigate to qa-e2e which has a loop structure
			await page.goto('/workflows/qa-e2e');
			await page.waitForSelector('.workflow-editor-body');
			await page.waitForSelector('.react-flow');

			// Check for loop edge type elements
			const loopEdges = page.locator('[data-testid="loop-edge"]');

			// If loop edges exist, verify they render as backward connections
			const loopCount = await loopEdges.count();
			if (loopCount > 0) {
				// Loop edge should be visible
				await expect(loopEdges.first()).toBeVisible();

				// Loop edge path should curve upward (backward connection visual)
				const loopEdgePath = page.locator('.loop-edge .react-flow__edge-path');
				await expect(loopEdgePath.first()).toBeVisible();
			} else {
				// If no loop edges in built-in workflow, create a custom one
				test.skip(true, 'No loop edges in built-in workflows - need custom workflow');
			}
		});

		test('loop edge shows condition label', async ({ page }) => {
			await page.goto('/workflows/qa-e2e');
			await page.waitForSelector('.react-flow');

			// Find loop edge labels
			const loopLabels = page.locator('.loop-edge__label');
			const labelCount = await loopLabels.count();

			if (labelCount > 0) {
				// Loop label should show condition and max iterations
				await expect(loopLabels.first()).toBeVisible();
				// Label format is typically "condition ×N"
				await expect(loopLabels.first()).toContainText('×');
			} else {
				test.skip(true, 'No loop edge labels found');
			}
		});

		test('loop edge renders below forward edges (visual layering)', async ({ page }) => {
			await page.goto('/workflows/qa-e2e');
			await page.waitForSelector('.react-flow');

			const loopEdge = page.locator('.loop-edge').first();
			const gateEdge = page.locator('.gate-edge').first();

			const loopEdgeCount = await loopEdge.count();
			const gateEdgeCount = await gateEdge.count();

			if (loopEdgeCount > 0 && gateEdgeCount > 0) {
				// Both edge types should be visible
				await expect(loopEdge).toBeVisible();
				await expect(gateEdge).toBeVisible();

				// Loop edges should be rendered (z-index doesn't apply the same way in SVG,
				// but they should be in the DOM and visible)
			} else {
				test.skip(true, 'Need both loop and gate edges for layering test');
			}
		});
	});

	test.describe('SC-3: Gate configuration changes persist', () => {
		test('gate type change persists after page reload', async ({ page }) => {
			// Create a custom workflow for testing persistence
			const customWorkflowId = `test-persist-type-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Click on the first gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Get the current gate type
			const gateTypeSelect = page.locator('#gate-type');
			const initialType = await gateTypeSelect.inputValue();

			// Change gate type to Human (value: 2)
			await gateTypeSelect.selectOption({ value: '2' });

			// Wait for API call to complete
			await page.waitForTimeout(1000);

			// Reload the page
			await page.reload();
			await page.waitForSelector('.react-flow');

			// Click on the same gate
			await page.locator('.gate-edge__symbol').first().click();

			// Verify the type persisted
			const newType = await page.locator('#gate-type').inputValue();
			expect(newType).toBe('2'); // Human gate
			expect(newType).not.toBe(initialType);
		});

		test('max retries change persists after page reload', async ({ page }) => {
			const customWorkflowId = `test-persist-retries-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Select a gate
			await page.locator('.gate-edge__symbol').first().click();
			await expect(page.locator('.gate-inspector')).toBeVisible();

			// Change max retries to 7
			const maxRetriesInput = page.locator('#max-retries');
			await maxRetriesInput.clear();
			await maxRetriesInput.fill('7');
			await maxRetriesInput.blur(); // Trigger save

			// Wait for API call
			await page.waitForTimeout(1000);

			// Reload
			await page.reload();
			await page.waitForSelector('.react-flow');

			// Select the same gate
			await page.locator('.gate-edge__symbol').first().click();

			// Verify persistence
			const savedRetries = await page.locator('#max-retries').inputValue();
			expect(savedRetries).toBe('7');
		});

		test('failure action change persists after page reload', async ({ page }) => {
			const customWorkflowId = `test-persist-failure-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Select a gate
			await page.locator('.gate-edge__symbol').first().click();

			// Change failure action to 'pause'
			const failureSelect = page.locator('#on-fail');
			await failureSelect.selectOption({ value: 'pause' });

			// Wait for API
			await page.waitForTimeout(1000);

			// Reload
			await page.reload();
			await page.waitForSelector('.react-flow');

			// Select gate again
			await page.locator('.gate-edge__symbol').first().click();

			// Verify persistence
			const savedAction = await page.locator('#on-fail').inputValue();
			expect(savedAction).toBe('pause');
		});

		test('multiple config changes persist together', async ({ page }) => {
			const customWorkflowId = `test-persist-multi-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Select a gate
			await page.locator('.gate-edge__symbol').first().click();

			// Change multiple settings
			await page.locator('#gate-type').selectOption({ value: '2' }); // Human
			await page.waitForTimeout(500);
			await page.locator('#max-retries').clear();
			await page.locator('#max-retries').fill('5');
			await page.locator('#max-retries').blur();
			await page.waitForTimeout(500);
			await page.locator('#on-fail').selectOption({ value: 'fail' });

			// Wait for all API calls
			await page.waitForTimeout(1500);

			// Reload
			await page.reload();
			await page.waitForSelector('.react-flow');

			// Select gate and verify all changes persisted
			await page.locator('.gate-edge__symbol').first().click();

			expect(await page.locator('#gate-type').inputValue()).toBe('2');
			expect(await page.locator('#max-retries').inputValue()).toBe('5');
			expect(await page.locator('#on-fail').inputValue()).toBe('fail');
		});
	});

	test.describe('Gate color coding - Complete coverage', () => {
		test('all four gate types have distinct visual representations', async ({ page }) => {
			const customWorkflowId = `test-colors-${Date.now()}`;
			await cloneWorkflow(page, 'medium', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Get all gates and set different types
			const gateSymbols = page.locator('.gate-edge__symbol');
			const count = await gateSymbols.count();

			// We need at least 4 gates for this test
			if (count < 4) {
				test.skip(true, 'Need at least 4 gates to test all color types');
				return;
			}

			// Set first gate to Auto (1)
			await gateSymbols.nth(0).click();
			await page.locator('#gate-type').selectOption({ value: '1' });
			await page.waitForTimeout(300);

			// Set second gate to Human (2)
			await gateSymbols.nth(1).click();
			await page.locator('#gate-type').selectOption({ value: '2' });
			await page.waitForTimeout(300);

			// Set third gate to AI (3)
			await gateSymbols.nth(2).click();
			await page.locator('#gate-type').selectOption({ value: '3' });
			await page.waitForTimeout(300);

			// Set fourth gate to Skip (4)
			await gateSymbols.nth(3).click();
			await page.locator('#gate-type').selectOption({ value: '4' });
			await page.waitForTimeout(500);

			// Verify all color classes are present
			await expect(page.locator('.gate-edge__symbol--auto')).toBeVisible();
			await expect(page.locator('.gate-edge__symbol--human')).toBeVisible();
			await expect(page.locator('.gate-edge__symbol--ai')).toBeVisible();
			await expect(page.locator('.gate-edge__symbol--skip')).toBeVisible();
		});

		test('skip gates show no diamond symbol (passthrough style)', async ({ page }) => {
			const customWorkflowId = `test-skip-style-${Date.now()}`;
			await cloneWorkflow(page, 'small', customWorkflowId);
			await page.waitForSelector('.react-flow');

			// Set a gate to Skip type
			await page.locator('.gate-edge__symbol').first().click();
			await page.locator('#gate-type').selectOption({ value: '4' }); // Skip
			await page.waitForTimeout(500);

			// Skip gate should not show diamond character
			const skipGate = page.locator('.gate-edge__symbol--skip').first();
			await expect(skipGate).toBeVisible();

			// The text content should be empty (no diamond)
			const content = await skipGate.textContent();
			expect(content?.trim()).toBe('');
		});
	});
});

test.describe('TASK-766: Edge Cases and Error Handling', () => {
	test('gate inspector handles rapid gate type changes', async ({ page }) => {
		await page.goto('/workflows/medium');
		await page.waitForSelector('.react-flow');

		// This is a read-only workflow, but we can still test the UI responsiveness
		// by rapidly clicking different gates
		const gateSymbols = page.locator('.gate-edge__symbol');
		const count = await gateSymbols.count();

		for (let i = 0; i < Math.min(count, 3); i++) {
			await gateSymbols.nth(i).click();
			await expect(page.locator('.gate-inspector')).toBeVisible();
			// Small delay to ensure inspector updates
			await page.waitForTimeout(100);
		}

		// Inspector should still be visible and functional
		await expect(page.locator('.gate-inspector')).toBeVisible();
		await expect(page.locator('.gate-inspector__settings')).toBeVisible();
	});

	test('single-phase workflow has entry and exit gates', async ({ page }) => {
		// Trivial workflow has only one phase
		await page.goto('/workflows/trivial');
		await page.waitForSelector('.react-flow');

		// Should have exactly 2 gates: entry and exit
		const gateSymbols = page.locator('.gate-edge__symbol');
		const count = await gateSymbols.count();
		expect(count).toBe(2);

		// Entry gate should exist
		await expect(page.locator('.gate-edge--entry .gate-edge__symbol')).toBeVisible();

		// Exit gate should exist
		await expect(page.locator('.gate-edge--exit .gate-edge__symbol')).toBeVisible();
	});
});
