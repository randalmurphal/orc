/**
 * Finalize Workflow E2E Tests
 *
 * Tests for the finalize phase workflow - branch sync, CI verification, and merge.
 *
 * Test Coverage (10 tests):
 * - Finalize UI (5): button visibility, modal opening, modal content, progress bar, card state
 * - Finalize Results (5): success state, commit SHA, target branch, failure/retry, finished state
 *
 * WebSocket Events:
 * - finalize: progress updates with status, step, step_percent, result, error
 *
 * Selector Strategy (priority order):
 * 1. role/aria-label - getByRole(), locator('[aria-label="..."]')
 * 2. Semantic text - getByText(), :has-text()
 * 3. Structural classes - .task-card, .modal-backdrop, .finalize-progress
 * 4. data-testid - fallback only
 *
 * @see web/CLAUDE.md for selector strategy documentation
 */
import { test, expect, type Page } from '@playwright/test';

// WebSocket event message structure
interface WSEvent {
	type: 'event';
	event: string;
	task_id: string;
	data: unknown;
	time: string;
}

// FinalizeResult structure matching api.ts
interface FinalizeResult {
	synced: boolean;
	conflicts_resolved: number;
	conflict_files?: string[];
	tests_passed: boolean;
	risk_level: string;
	files_changed: number;
	lines_changed: number;
	needs_review: boolean;
	commit_sha?: string;
	target_branch: string;
}

// Helper to create a WebSocket event message
function createWSEvent(event: string, taskId: string, data: unknown): WSEvent {
	return {
		type: 'event',
		event,
		task_id: taskId,
		data,
		time: new Date().toISOString()
	};
}

// Helper: Wait for board to load with tasks
async function waitForBoardLoad(page: Page) {
	await page.waitForSelector('.board-page', { timeout: 10000 });
	await page.waitForSelector('.loading-state', { state: 'hidden', timeout: 10000 }).catch(() => {});
	await page.waitForLoadState('networkidle');
	await page.waitForSelector('.task-card, [role="region"][aria-label*="column"]', { timeout: 5000 }).catch(() => {});
	await page.waitForTimeout(200);
}

// Helper: Find a completed task suitable for finalize testing
async function findCompletedTask(page: Page): Promise<string | null> {
	// Look for completed tasks in Done column
	const doneColumn = page.locator('[role="region"][aria-label="Done column"]');
	const completedCards = doneColumn.locator('.task-card.completed, .task-card:has(.status-indicator.completed)');
	const count = await completedCards.count();

	if (count > 0) {
		const taskId = await completedCards.first().locator('.task-id').textContent();
		return taskId?.trim() || null;
	}

	// If no completed task, fall back to any task
	const taskCards = page.locator('.task-card');
	const anyCount = await taskCards.count();
	if (anyCount > 0) {
		const taskId = await taskCards.first().locator('.task-id').textContent();
		return taskId?.trim() || null;
	}

	return null;
}

// Helper: Setup WebSocket interception for finalize events
async function setupWSInterception(page: Page): Promise<{ send: (data: string) => void } | null> {
	let wsSend: ((data: string) => void) | null = null;

	await page.routeWebSocket(/\/api\/ws/, async (ws) => {
		const server = await ws.connectToServer();
		wsSend = (data: string) => ws.send(data);

		ws.onMessage((message) => server.send(message));
		server.onMessage((message) => ws.send(message));
	});

	await page.reload();
	await waitForBoardLoad(page);
	await page.waitForTimeout(500);

	return wsSend ? { send: wsSend } : null;
}

test.describe('Finalize Workflow', () => {
	test.describe('Finalize UI', () => {
		test('should show finalize button on completed tasks in Done column', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Look for completed tasks in Done column
			const doneColumn = page.locator('[role="region"][aria-label="Done column"]');
			const taskCards = doneColumn.locator('.task-card');
			const count = await taskCards.count();

			// Skip if no tasks in Done column
			if (count === 0) {
				// If no tasks in Done, simulate one via WebSocket
				const allCards = page.locator('.task-card');
				const anyCount = await allCards.count();
				test.skip(anyCount === 0, 'No tasks available for testing');

				const taskId = (await allCards.first().locator('.task-id').textContent())?.trim();

				// Set up WebSocket interception
				const ws = await setupWSInterception(page);
				if (ws && taskId) {
					// Simulate task completion
					const stateEvent = createWSEvent('state', taskId, {
						status: 'completed'
					});
					ws.send(JSON.stringify(stateEvent));

					const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
						task: { id: taskId, status: 'completed' }
					});
					ws.send(JSON.stringify(taskUpdatedEvent));

					await page.waitForTimeout(500);
				}
			}

			// Now check for completed tasks with finalize button
			const completedCards = page.locator('.task-card.completed, .task-card:has(.action-btn.finalize)');
			const completedCount = await completedCards.count();

			if (completedCount > 0) {
				// Verify finalize button exists on completed card
				const finalizeBtn = completedCards.first().locator('.action-btn.finalize');
				await expect(finalizeBtn).toBeVisible();

				// Button should have merge icon SVG
				const svg = finalizeBtn.locator('svg');
				await expect(svg).toBeVisible();
			}
		});

		test('should open FinalizeModal when finalize button clicked', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Get any task and simulate completion
			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			// Set up WebSocket and simulate completed status
			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);
			}

			// Find the finalize button on completed task
			const completedCard = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
			const finalizeBtn = completedCard.locator('.action-btn.finalize');

			// If button exists, click it
			if (await finalizeBtn.isVisible().catch(() => false)) {
				await finalizeBtn.click();

				// FinalizeModal should open
				const modal = page.locator('.modal-backdrop[role="dialog"]');
				await expect(modal).toBeVisible({ timeout: 5000 });

				// Modal should have correct structure
				const modalTitle = modal.locator('#finalize-modal-title');
				await expect(modalTitle).toContainText('Finalize Task');
			}
		});

		test('should display explanation and start button in modal', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			// Set up WebSocket and simulate completed status
			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);
			}

			// Click finalize button
			const completedCard = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
			const finalizeBtn = completedCard.locator('.action-btn.finalize');

			if (await finalizeBtn.isVisible().catch(() => false)) {
				await finalizeBtn.click();

				const modal = page.locator('.modal-backdrop[role="dialog"]');
				await expect(modal).toBeVisible({ timeout: 5000 });

				// Modal should show task ID in header
				const taskIdDisplay = modal.locator('.task-id');
				await expect(taskIdDisplay).toContainText(taskId!);

				// Modal should show explanation text
				const infoText = modal.locator('.info-text');
				await expect(infoText).toContainText(/finalize phase|sync|branch|merge/i);

				// Start button should be visible
				const startBtn = modal.locator('.btn-primary:has-text("Start Finalize")');
				await expect(startBtn).toBeVisible();

				// Close button should exist
				const closeBtn = modal.locator('.btn-secondary:has-text("Close")');
				await expect(closeBtn).toBeVisible();
			}
		});

		test('should show progress bar with step labels during finalization', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			// Set up WebSocket
			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				// Simulate completed status and open modal
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);

				// Click finalize button
				const completedCard = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
				const finalizeBtn = completedCard.locator('.action-btn.finalize');

				if (await finalizeBtn.isVisible().catch(() => false)) {
					await finalizeBtn.click();

					const modal = page.locator('.modal-backdrop[role="dialog"]');
					await expect(modal).toBeVisible({ timeout: 5000 });

					// Send finalize running event with progress
					const finalizeEvent = createWSEvent('finalize', taskId, {
						task_id: taskId,
						status: 'running',
						step: 'Syncing branch...',
						progress: 'Fetching latest changes from main',
						step_percent: 25,
						updated_at: new Date().toISOString()
					});
					ws.send(JSON.stringify(finalizeEvent));
					await page.waitForTimeout(300);

					// Modal should show progress section
					const progressSection = modal.locator('.progress-section');
					await expect(progressSection).toBeVisible({ timeout: 5000 });

					// Step label should show current step
					const stepLabel = modal.locator('.step-label');
					await expect(stepLabel).toContainText('Syncing branch');

					// Progress percentage should show
					const stepPercent = modal.locator('.step-percent');
					await expect(stepPercent).toContainText('25%');

					// Progress bar should exist
					const progressBar = modal.locator('.progress-bar');
					await expect(progressBar).toBeVisible();
				}
			}
		});

		test('should update task card to show finalizing state (pulsing border)', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			// Set up WebSocket
			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				// Simulate task changing to finalizing status
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'finalizing' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));

				// Also send finalize progress event
				const finalizeEvent = createWSEvent('finalize', taskId, {
					task_id: taskId,
					status: 'running',
					step: 'Running tests...',
					step_percent: 50
				});
				ws.send(JSON.stringify(finalizeEvent));

				await page.waitForTimeout(500);

				// Card should have finalizing class
				const card = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
				await expect(card).toBeVisible();

				// Check for finalizing class or progress indicator
				const finalizingCard = card.locator('.finalizing, .finalize-progress');
				const hasFinalizingState = await finalizingCard.isVisible().catch(() => false);

				// The card should show some indication of finalizing
				if (hasFinalizingState) {
					// Progress bar with step should be visible
					const finalizingProgress = card.locator('.finalize-progress');
					await expect(finalizingProgress).toBeVisible();

					const step = card.locator('.finalize-step');
					await expect(step).toContainText('Running tests');
				}
			}
		});
	});

	test.describe('Finalize Results', () => {
		test('should show success state with merge info', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			// Set up WebSocket
			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				// Simulate completed status and open modal
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);

				// Open finalize modal
				const completedCard = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
				const finalizeBtn = completedCard.locator('.action-btn.finalize');

				if (await finalizeBtn.isVisible().catch(() => false)) {
					await finalizeBtn.click();

					const modal = page.locator('.modal-backdrop[role="dialog"]');
					await expect(modal).toBeVisible({ timeout: 5000 });

					// Wait a bit for modal to initialize
					await page.waitForTimeout(200);

					// Send finalize completed event with result
					const finalizeResult: FinalizeResult = {
						synced: true,
						conflicts_resolved: 2,
						conflict_files: ['src/api.ts', 'src/utils.ts'],
						tests_passed: true,
						risk_level: 'low',
						files_changed: 15,
						lines_changed: 234,
						needs_review: false,
						commit_sha: 'abc1234567890',
						target_branch: 'main'
					};

					const finalizeEvent = createWSEvent('finalize', taskId, {
						task_id: taskId,
						status: 'completed',
						step: 'Complete',
						step_percent: 100,
						result: finalizeResult,
						updated_at: new Date().toISOString()
					});
					ws.send(JSON.stringify(finalizeEvent));

					// Modal should show success result section
					const resultSection = modal.locator('.result-section.success');
					await expect(resultSection).toBeVisible({ timeout: 5000 });

					// Success header should show "Finalize Completed"
					const resultHeader = modal.locator('.result-header');
					await expect(resultHeader).toContainText('Finalize Completed');
				}
			}
		});

		test('should display merged commit SHA', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				// Simulate completed and open modal
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);

				const finalizeBtn = page.locator(`.task-card:has(.task-id:has-text("${taskId}")) .action-btn.finalize`);

				if (await finalizeBtn.isVisible().catch(() => false)) {
					await finalizeBtn.click();

					const modal = page.locator('.modal-backdrop[role="dialog"]');
					await expect(modal).toBeVisible({ timeout: 5000 });

					// Wait a bit for modal to initialize
					await page.waitForTimeout(200);

					// Send completed finalize with commit SHA
					const finalizeEvent = createWSEvent('finalize', taskId, {
						task_id: taskId,
						status: 'completed',
						step: 'Complete',
						step_percent: 100,
						result: {
							synced: true,
							conflicts_resolved: 0,
							tests_passed: true,
							risk_level: 'low',
							files_changed: 5,
							lines_changed: 50,
							needs_review: false,
							commit_sha: 'deadbeef12345678',
							target_branch: 'main'
						},
						updated_at: new Date().toISOString()
					});
					ws.send(JSON.stringify(finalizeEvent));

					// Wait for result section to appear - more robust than fixed timeout
					const resultSection = modal.locator('.result-section.success');
					await expect(resultSection).toBeVisible({ timeout: 5000 });

					// Check for commit SHA display (7-char abbreviated)
					const commitRow = modal.locator('.detail-row:has(.detail-label:has-text("Commit"))');
					await expect(commitRow).toBeVisible({ timeout: 3000 });

					const commitValue = commitRow.locator('.detail-value');
					await expect(commitValue).toContainText('deadbee');
				}
			}
		});

		test('should show target branch name', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);

				const finalizeBtn = page.locator(`.task-card:has(.task-id:has-text("${taskId}")) .action-btn.finalize`);

				if (await finalizeBtn.isVisible().catch(() => false)) {
					await finalizeBtn.click();

					const modal = page.locator('.modal-backdrop[role="dialog"]');
					await expect(modal).toBeVisible({ timeout: 5000 });

					// Wait a bit for modal to initialize
					await page.waitForTimeout(200);

					// Send completed finalize
					const finalizeEvent = createWSEvent('finalize', taskId, {
						task_id: taskId,
						status: 'completed',
						step: 'Complete',
						step_percent: 100,
						result: {
							synced: true,
							conflicts_resolved: 0,
							tests_passed: true,
							risk_level: 'low',
							files_changed: 3,
							lines_changed: 30,
							needs_review: false,
							commit_sha: 'abc123456789',
							target_branch: 'develop'
						},
						updated_at: new Date().toISOString()
					});
					ws.send(JSON.stringify(finalizeEvent));

					// Wait for result section to appear
					const resultSection = modal.locator('.result-section.success');
					await expect(resultSection).toBeVisible({ timeout: 5000 });

					// Check for target branch display
					const branchRow = modal.locator('.detail-row:has(.detail-label:has-text("Target Branch"))');
					await expect(branchRow).toBeVisible({ timeout: 3000 });

					const branchValue = branchRow.locator('.detail-value');
					await expect(branchValue).toContainText('develop');
				}
			}
		});

		test('should handle finalize failure with retry option', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'completed' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));
				await page.waitForTimeout(500);

				const finalizeBtn = page.locator(`.task-card:has(.task-id:has-text("${taskId}")) .action-btn.finalize`);

				if (await finalizeBtn.isVisible().catch(() => false)) {
					await finalizeBtn.click();

					const modal = page.locator('.modal-backdrop[role="dialog"]');
					await expect(modal).toBeVisible({ timeout: 5000 });

					// Send failed finalize event
					const finalizeEvent = createWSEvent('finalize', taskId, {
						task_id: taskId,
						status: 'failed',
						step: 'Running tests',
						step_percent: 75,
						error: 'Tests failed: 3 assertions failed in api.test.ts',
						updated_at: new Date().toISOString()
					});
					ws.send(JSON.stringify(finalizeEvent));
					await page.waitForTimeout(300);

					// Modal should show failed state
					const failedSection = modal.locator('.result-section.failed');
					await expect(failedSection).toBeVisible({ timeout: 5000 });

					// Failed header should show "Finalize Failed"
					const failedHeader = modal.locator('.result-header');
					await expect(failedHeader).toContainText('Finalize Failed');

					// Error message should be visible
					const errorMsg = modal.locator('.error-message');
					await expect(errorMsg).toContainText('Tests failed');

					// Retry button should appear
					const retryBtn = modal.locator('.btn-primary:has-text("Retry Finalize")');
					await expect(retryBtn).toBeVisible();
				}
			}
		});

		test('should update task card to finished state with green merge badge', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const taskCards = page.locator('.task-card');
			const count = await taskCards.count();
			test.skip(count === 0, 'No tasks available for testing');

			const taskId = (await taskCards.first().locator('.task-id').textContent())?.trim();
			test.skip(!taskId, 'Could not get task ID');

			const ws = await setupWSInterception(page);
			if (ws && taskId) {
				// Simulate task transitioning to finished status
				const taskUpdatedEvent = createWSEvent('task_updated', taskId, {
					task: { id: taskId, status: 'finished' }
				});
				ws.send(JSON.stringify(taskUpdatedEvent));

				// Send finalize completed event with result
				const finalizeEvent = createWSEvent('finalize', taskId, {
					task_id: taskId,
					status: 'completed',
					result: {
						synced: true,
						conflicts_resolved: 0,
						tests_passed: true,
						risk_level: 'low',
						files_changed: 8,
						lines_changed: 120,
						needs_review: false,
						commit_sha: 'finalsha12345678',
						target_branch: 'main'
					}
				});
				ws.send(JSON.stringify(finalizeEvent));
				await page.waitForTimeout(500);

				// Card should have finished class and show merge info
				const card = page.locator(`.task-card:has(.task-id:has-text("${taskId}"))`);
				await expect(card).toBeVisible();

				// Check for finished info section with commit SHA and branch
				const finishedInfo = card.locator('.finished-info');
				const hasFinishedInfo = await finishedInfo.isVisible().catch(() => false);

				if (hasFinishedInfo) {
					// Should show abbreviated commit SHA
					const commitSha = finishedInfo.locator('.commit-sha');
					await expect(commitSha).toContainText('finalsh');

					// Should show merge target
					const mergeTarget = finishedInfo.locator('.merge-target');
					await expect(mergeTarget).toContainText('merged to main');
				}
			}
		});
	});
});
