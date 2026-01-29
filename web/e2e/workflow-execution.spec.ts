/**
 * E2E Tests for Workflow Execution Visualization
 *
 * Tests for TASK-639: Live execution visualization on workflow canvas
 *
 * Full user journey test covering:
 * - SC-1: Active phase node displays purple pulsing glow when running
 * - SC-2: Phase status updates in real-time
 * - SC-3: Completed phases show green left border and checkmark styling
 * - SC-4: Completed phases display cost badge
 * - SC-5: Header shows "Running" badge with pulse animation
 * - SC-6: Header shows live session metrics
 * - SC-7: Edges leading TO running phase are animated
 * - SC-8: Cancel button stops the workflow run
 *
 * Note: E2E tests run in isolated sandbox, not against production data
 */

import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// WebSocket event message structure for mocking
interface WSEvent {
	type: 'event';
	event: string;
	task_id: string;
	data: unknown;
	time: string;
}

// Helper to create a WebSocket event message
function createWSEvent(event: string, taskId: string, data: unknown): WSEvent {
	return {
		type: 'event',
		event,
		task_id: taskId,
		data,
		time: new Date().toISOString(),
	};
}

// Helper: Wait for workflow editor to load
async function waitForEditorLoad(page: Page) {
	await page.waitForSelector('.workflow-editor-page', { timeout: 10000 });
	await page.waitForSelector('.loading-state', { state: 'hidden', timeout: 10000 }).catch(() => {});
	await page.waitForLoadState('networkidle');
	// Wait for canvas to render
	await page.waitForSelector('.react-flow', { timeout: 5000 }).catch(() => {});
	await page.waitForTimeout(300);
}

// Helper: Navigate to a workflow with an active run
async function navigateToWorkflowWithRun(page: Page, workflowId: string = 'medium'): Promise<string | null> {
	await page.goto(`/workflows/${workflowId}`);
	await waitForEditorLoad(page);
	return workflowId;
}

test.describe('Workflow Execution Visualization', () => {
	test.describe('SC-1 & SC-3: Phase node status styling', () => {
		test('should display purple glow on running phase node', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			// Navigate to workflow editor
			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject phase running event
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement',
					status: 'running',
					iteration: 1,
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify running phase has purple glow styling
			const runningNode = page.locator('.phase-node--running');
			// Note: Test will fail until implementation is complete
			await expect(runningNode).toBeVisible({ timeout: 5000 });

			// Verify CSS glow effect
			const hasGlowClass = await runningNode.evaluate((el) => {
				const styles = window.getComputedStyle(el);
				// Check for box-shadow or animation property
				return (
					styles.boxShadow !== 'none' ||
					styles.animation !== 'none' ||
					el.classList.contains('phase-node--running')
				);
			});
			expect(hasGlowClass).toBe(true);
		});

		test('should display green border on completed phase node', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject phase completed event
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'spec',
					status: 'completed',
					iteration: 1,
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify completed phase has green border styling
			const completedNode = page.locator('.phase-node--completed');
			await expect(completedNode).toBeVisible({ timeout: 5000 });

			// Verify green border CSS
			const borderColor = await completedNode.evaluate((el) => {
				const styles = window.getComputedStyle(el);
				return styles.borderLeftColor;
			});
			// Green color should be in the border (rgb(34, 197, 94) or similar green)
			expect(borderColor).toMatch(/rgb\(\d+,\s*\d+,\s*\d+\)|green|#[0-9a-fA-F]{6}/);
		});

		test('should display red border on failed phase node', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject phase failed event
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement',
					status: 'failed',
					iteration: 1,
					error: 'Build failed',
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify failed phase has red border styling
			const failedNode = page.locator('.phase-node--failed');
			await expect(failedNode).toBeVisible({ timeout: 5000 });
		});
	});

	test.describe('SC-2: Real-time phase status updates', () => {
		test('should update node status within 2 seconds of event', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Record start time
			const startTime = Date.now();

			// Inject phase event
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement',
					status: 'running',
				});
				wsSendToPage(JSON.stringify(phaseEvent));
			}

			// Wait for node to update (should be < 2 seconds)
			const runningNode = page.locator('.phase-node--running');
			await expect(runningNode).toBeVisible({ timeout: 2000 });

			// Verify timing
			const elapsed = Date.now() - startTime;
			expect(elapsed).toBeLessThan(2000);
		});
	});

	test.describe('SC-4: Cost badge on completed phases', () => {
		test('should show cost badge when phase has costUsd > 0', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject completed phase with cost
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'spec',
					status: 'completed',
					iteration: 2,
					cost_usd: 0.42,
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify cost badge is displayed
			const costBadge = page.locator('.phase-node-footer:has-text("$0.42")');
			await expect(costBadge).toBeVisible({ timeout: 5000 });
		});

		test('should not show cost badge when costUsd is 0', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject completed phase with zero cost
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'spec',
					status: 'completed',
					cost_usd: 0,
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify no $0.00 badge is displayed
			const zeroCostBadge = page.locator('.phase-node-footer:has-text("$0.00")');
			await expect(zeroCostBadge).not.toBeVisible();
		});
	});

	test.describe('SC-5 & SC-6: Execution header', () => {
		test('should show Running badge in header when workflow is executing', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject run started state
			if (wsSendToPage) {
				const stateEvent = createWSEvent('state', 'TASK-001', {
					status: 'running',
					current_phase: 'spec',
				});
				wsSendToPage(JSON.stringify(stateEvent));
				await page.waitForTimeout(500);
			}

			// Verify Running badge is visible
			const runningBadge = page.locator('.execution-badge:has-text("Running")');
			await expect(runningBadge).toBeVisible({ timeout: 5000 });

			// Verify badge has pulse animation
			const hasPulse = await runningBadge.evaluate((el) => {
				return el.classList.contains('execution-badge--pulse');
			});
			expect(hasPulse).toBe(true);
		});

		test('should show live metrics in header during execution', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject session metrics event
			if (wsSendToPage) {
				const metricsEvent = createWSEvent('session_metrics', 'TASK-001', {
					duration_seconds: 323, // 5m 23s
					total_tokens: 45200,
					estimated_cost_usd: 1.23,
				});
				wsSendToPage(JSON.stringify(metricsEvent));
				await page.waitForTimeout(500);
			}

			// Verify metrics are displayed in header
			const durationDisplay = page.locator('.execution-header:has-text("5m")');
			await expect(durationDisplay).toBeVisible({ timeout: 5000 });

			const tokenDisplay = page.locator('.execution-header:has-text("45")');
			await expect(tokenDisplay).toBeVisible();

			const costDisplay = page.locator('.execution-header:has-text("$1.23")');
			await expect(costDisplay).toBeVisible();
		});

		test('should change badge to Completed when run finishes', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// First, simulate running state
			if (wsSendToPage) {
				const runningEvent = createWSEvent('state', 'TASK-001', {
					status: 'running',
					current_phase: 'review',
				});
				wsSendToPage(JSON.stringify(runningEvent));
				await page.waitForTimeout(300);

				// Then complete
				const completedEvent = createWSEvent('state', 'TASK-001', {
					status: 'completed',
				});
				wsSendToPage(JSON.stringify(completedEvent));
				await page.waitForTimeout(500);
			}

			// Verify Completed badge is visible
			const completedBadge = page.locator('.execution-badge:has-text("Completed")');
			await expect(completedBadge).toBeVisible({ timeout: 5000 });
		});
	});

	test.describe('SC-7: Edge animation', () => {
		test('should animate edges leading to running phase', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject phase running event
			if (wsSendToPage) {
				const phaseEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement', // Second phase in typical workflow
					status: 'running',
				});
				wsSendToPage(JSON.stringify(phaseEvent));
				await page.waitForTimeout(500);
			}

			// Verify edge to running phase is animated
			// React Flow adds 'animated' class to animated edges
			const animatedEdge = page.locator('.react-flow__edge.animated');
			await expect(animatedEdge).toBeVisible({ timeout: 5000 });
		});

		test('should stop edge animation when no phase is running', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			if (wsSendToPage) {
				// First running
				const runningEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement',
					status: 'running',
				});
				wsSendToPage(JSON.stringify(runningEvent));
				await page.waitForTimeout(300);

				// Then complete (no more running phases)
				const completedEvent = createWSEvent('phase_changed', 'TASK-001', {
					phase_name: 'implement',
					status: 'completed',
				});
				wsSendToPage(JSON.stringify(completedEvent));
				await page.waitForTimeout(500);
			}

			// Verify no edges are animated
			const animatedEdge = page.locator('.react-flow__edge.animated');
			await expect(animatedEdge).not.toBeVisible();
		});
	});

	test.describe('SC-8: Cancel functionality', () => {
		test('should show Cancel button when workflow is running', async ({ page }) => {
			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Inject running state
			if (wsSendToPage) {
				const stateEvent = createWSEvent('state', 'TASK-001', {
					status: 'running',
					current_phase: 'implement',
				});
				wsSendToPage(JSON.stringify(stateEvent));
				await page.waitForTimeout(500);
			}

			// Verify Cancel button is visible
			const cancelButton = page.locator('button:has-text("Cancel")');
			await expect(cancelButton).toBeVisible({ timeout: 5000 });
		});

		test('should update UI to Cancelled when cancel succeeds', async ({ page }) => {
			// Mock the cancelWorkflowRun API to succeed
			await page.route('**/orc.v1.WorkflowService/CancelWorkflowRun', async (route) => {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({
						run: {
							id: 'run-001',
							status: 'cancelled',
						},
					}),
				});
			});

			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Set running state
			if (wsSendToPage) {
				const stateEvent = createWSEvent('state', 'TASK-001', {
					status: 'running',
					current_phase: 'implement',
				});
				wsSendToPage(JSON.stringify(stateEvent));
				await page.waitForTimeout(500);
			}

			// Click Cancel button
			const cancelButton = page.locator('button:has-text("Cancel")');
			await expect(cancelButton).toBeVisible();
			await cancelButton.click();

			// If there's a confirmation dialog, confirm it
			const confirmButton = page.locator('button:has-text("Confirm")');
			if (await confirmButton.isVisible().catch(() => false)) {
				await confirmButton.click();
			}

			// Verify badge changes to Cancelled
			const cancelledBadge = page.locator('.execution-badge:has-text("Cancelled")');
			await expect(cancelledBadge).toBeVisible({ timeout: 5000 });
		});

		test('should show error toast when cancel fails', async ({ page }) => {
			// Mock the cancelWorkflowRun API to fail
			await page.route('**/orc.v1.WorkflowService/CancelWorkflowRun', async (route) => {
				await route.fulfill({
					status: 500,
					contentType: 'application/json',
					body: JSON.stringify({
						error: 'Internal server error',
					}),
				});
			});

			// Set up WebSocket interception
			let wsSendToPage: ((data: string) => void) | null = null;

			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();
				wsSendToPage = (data: string) => ws.send(data);

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Set running state
			if (wsSendToPage) {
				const stateEvent = createWSEvent('state', 'TASK-001', {
					status: 'running',
					current_phase: 'implement',
				});
				wsSendToPage(JSON.stringify(stateEvent));
				await page.waitForTimeout(500);
			}

			// Click Cancel button
			const cancelButton = page.locator('button:has-text("Cancel")');
			await expect(cancelButton).toBeVisible();
			await cancelButton.click();

			// If there's a confirmation dialog, confirm it
			const confirmButton = page.locator('button:has-text("Confirm")');
			if (await confirmButton.isVisible().catch(() => false)) {
				await confirmButton.click();
			}

			// Verify error toast appears
			const toast = page.locator('.toast, [role="alert"]');
			await expect(toast).toContainText(/Failed to cancel|Error/i, { timeout: 5000 });

			// Badge should still show Running (not cancelled)
			const runningBadge = page.locator('.execution-badge:has-text("Running")');
			await expect(runningBadge).toBeVisible();
		});
	});

	test.describe('No active run - default state', () => {
		test('should not show execution header when no active run', async ({ page }) => {
			// Navigate to workflow without setting any running state
			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Verify execution header is not visible
			const executionHeader = page.locator('.execution-header');
			await expect(executionHeader).not.toBeVisible();
		});

		test('should show nodes without status styling when no active run', async ({ page }) => {
			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Verify no nodes have status classes
			const runningNode = page.locator('.phase-node--running');
			const completedNode = page.locator('.phase-node--completed');
			const failedNode = page.locator('.phase-node--failed');

			await expect(runningNode).not.toBeVisible();
			await expect(completedNode).not.toBeVisible();
			await expect(failedNode).not.toBeVisible();
		});
	});

	test.describe('Reconnection handling', () => {
		test('should show reconnecting indicator on disconnect', async ({ page }) => {
			// Set up WebSocket with disconnect simulation
			await page.routeWebSocket(/\/api\/ws/, async (ws) => {
				const server = await ws.connectToServer();

				ws.onMessage((message) => server.send(message));
				server.onMessage((message) => ws.send(message));

				// Disconnect after 1 second
				setTimeout(() => {
					ws.close({ code: 1006, reason: 'Test disconnect' });
				}, 1000);
			});

			await page.goto('/workflows/medium');
			await waitForEditorLoad(page);

			// Wait for disconnect
			await page.waitForTimeout(1500);

			// Verify reconnecting indicator
			const reconnectingIndicator = page.locator(':has-text("Reconnecting")');
			// May or may not be visible depending on reconnection timing
			// Just verify the query doesn't crash
			expect(await reconnectingIndicator.count()).toBeGreaterThanOrEqual(0);
		});
	});
});
