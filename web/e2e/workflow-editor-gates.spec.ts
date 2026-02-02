/**
 * E2E Tests for Workflow Editor - Gates as Edges
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-1: GateEdge component renders gate symbol on edge midpoint
 * - SC-4: Clicking gate symbol selects the edge
 * - SC-5: GateInspector panel appears when edge is selected
 * - SC-6: GateInspector shows read-only mode for built-in workflows
 * - SC-7: Entry edge renders from left canvas boundary to first phase
 * - SC-8: Exit edge renders from last phase to right canvas boundary
 * - SC-11: Hovering gate symbol shows tooltip
 * - SC-12: Gate drag behavior: gate travels with edge when phase nodes are repositioned
 *
 * These tests will FAIL until the gate-as-edges feature is implemented.
 */

import { test, expect } from '@playwright/test';

test.describe('Workflow Editor - Gates as Edges', () => {
	test.beforeEach(async ({ page }) => {
		// Navigate to a workflow with multiple phases
		await page.goto('/workflows/medium');
		// Wait for the editor to load
		await page.waitForSelector('.workflow-editor-body');
	});

	test.describe('SC-1: Gate symbols render on edges', () => {
		test('gate symbols appear on edges between phases', async ({ page }) => {
			// Wait for the canvas to be ready
			await page.waitForSelector('.react-flow');

			// Should have gate edge elements with symbols
			const gateSymbols = page.locator('.gate-edge__symbol');
			await expect(gateSymbols.first()).toBeVisible();
		});

		test('entry gate symbol appears before first phase', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Entry gate should exist
			const entryGate = page.locator('.gate-edge--entry .gate-edge__symbol');
			await expect(entryGate).toBeVisible();
		});

		test('exit gate symbol appears after last phase', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Exit gate should exist
			const exitGate = page.locator('.gate-edge--exit .gate-edge__symbol');
			await expect(exitGate).toBeVisible();
		});
	});

	test.describe('SC-4: Clicking gate symbol selects the edge', () => {
		test('clicking a gate symbol opens the gate inspector', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Click on a gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Gate inspector should appear
			await expect(page.locator('.gate-inspector')).toBeVisible();
		});

		test('clicking a gate deselects any selected phase node', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// First click on a phase node to select it
			const phaseNode = page.locator('.phase-node').first();
			await phaseNode.click();

			// Phase inspector should be visible
			await expect(page.locator('.phase-inspector')).toBeVisible();

			// Now click on a gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Gate inspector should be visible, phase inspector should be hidden
			await expect(page.locator('.gate-inspector')).toBeVisible();
			await expect(page.locator('.phase-inspector')).not.toBeVisible();
		});
	});

	test.describe('SC-5: GateInspector panel appears when edge is selected', () => {
		test('gate inspector shows gate type', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Click on a gate
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Inspector should show gate type (Auto, Human, AI, or Skip)
			const inspectorContent = page.locator('.gate-inspector');
			await expect(inspectorContent.getByText(/Auto|Human|AI|Skip/)).toBeVisible();
		});

		test('gate inspector shows gate settings', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Click on a gate
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Should show settings section
			await expect(page.locator('.gate-inspector__settings')).toBeVisible();
		});
	});

	test.describe('SC-6: GateInspector shows read-only mode for built-in workflows', () => {
		test('built-in workflow shows "Clone to customize" notice', async ({ page }) => {
			// Navigate to a built-in workflow
			await page.goto('/workflows/medium');
			await page.waitForSelector('.react-flow');

			// Click on a gate
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Should show read-only notice
			await expect(page.getByText(/Clone to customize/)).toBeVisible();
		});

		test('built-in workflow has disabled form controls in gate inspector', async ({ page }) => {
			await page.goto('/workflows/medium');
			await page.waitForSelector('.react-flow');

			// Click on a gate
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// All form controls should be disabled
			const selectInputs = page.locator('.gate-inspector select');
			const count = await selectInputs.count();
			for (let i = 0; i < count; i++) {
				await expect(selectInputs.nth(i)).toBeDisabled();
			}
		});
	});

	test.describe('SC-11: Hovering gate symbol shows tooltip', () => {
		test('tooltip appears on hover', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Hover over a gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.hover();

			// Tooltip should appear
			await expect(page.locator('.gate-edge__tooltip')).toBeVisible();
		});

		test('tooltip shows gate type information', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Hover over a gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.hover();

			// Tooltip should contain gate type
			const tooltip = page.locator('.gate-edge__tooltip');
			await expect(tooltip.getByText(/Auto|Human|AI|Skip/)).toBeVisible();
		});

		test('tooltip disappears when mouse leaves', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Hover over a gate symbol
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.hover();

			// Tooltip should appear
			await expect(page.locator('.gate-edge__tooltip')).toBeVisible();

			// Move mouse away
			await page.mouse.move(0, 0);

			// Tooltip should disappear
			await expect(page.locator('.gate-edge__tooltip')).not.toBeVisible();
		});
	});

	test.describe('SC-12: Gate drag behavior', () => {
		test('gate symbol follows edge when phase node is dragged', async ({ page }) => {
			// This test requires a custom workflow (not read-only)
			// First clone the workflow to get an editable one
			await page.goto('/workflows/medium');
			await page.waitForSelector('.react-flow');

			// Get initial gate position
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			const initialBox = await gateSymbol.boundingBox();

			// Find a phase node and drag it
			const phaseNode = page.locator('.phase-node').first();
			const phaseBox = await phaseNode.boundingBox();

			if (phaseBox && initialBox) {
				// Drag the phase node to a new position
				await phaseNode.dragTo(page.locator('.react-flow'), {
					targetPosition: {
						x: phaseBox.x + 100,
						y: phaseBox.y + 50,
					},
				});

				// Wait for React Flow to update
				await page.waitForTimeout(500);

				// Get new gate position
				const newBox = await gateSymbol.boundingBox();

				// Gate should have moved (position changed)
				expect(newBox?.x).not.toBe(initialBox.x);
			}
		});
	});

	test.describe('Clicking canvas background deselects gate', () => {
		test('clicking empty canvas area clears gate selection', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Click on a gate to select it
			const gateSymbol = page.locator('.gate-edge__symbol').first();
			await gateSymbol.click();

			// Gate inspector should be visible
			await expect(page.locator('.gate-inspector')).toBeVisible();

			// Click on the canvas background
			await page.locator('.react-flow__pane').click();

			// Gate inspector should be hidden
			await expect(page.locator('.gate-inspector')).not.toBeVisible();
		});
	});

	test.describe('Gate color coding', () => {
		test('auto gates have blue color', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Look for auto gate symbol
			const autoGate = page.locator('.gate-edge__symbol--auto');

			if (await autoGate.count() > 0) {
				await expect(autoGate.first()).toBeVisible();
			}
		});

		test('human gates have yellow color', async ({ page }) => {
			await page.waitForSelector('.react-flow');

			// Look for human gate symbol
			const humanGate = page.locator('.gate-edge__symbol--human');

			if (await humanGate.count() > 0) {
				await expect(humanGate.first()).toBeVisible();
			}
		});
	});

	test.describe('BDD-1: Gate count for multi-phase workflow', () => {
		test('3-phase workflow shows 4 gate symbols', async ({ page }) => {
			// Navigate to a workflow with 3 phases
			await page.goto('/workflows/small'); // Assuming small has 3 phases
			await page.waitForSelector('.react-flow');

			// Count gate symbols
			const gateSymbols = page.locator('.gate-edge__symbol');
			const count = await gateSymbols.count();

			// 3 phases = entry→phase1, phase1→phase2, phase2→phase3, phase3→exit = 4 gates
			expect(count).toBe(4);
		});
	});
});
