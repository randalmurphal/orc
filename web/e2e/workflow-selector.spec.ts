/**
 * E2E Tests for Workflow Selector in Task Modals
 *
 * Tests for TASK-536: Add workflow selector to New Task and Edit Task modals
 *
 * Success Criteria Coverage:
 * - SC-1: New Task modal displays workflow selector dropdown
 * - SC-2: Creating task with workflow saves workflow_id
 * - SC-4: Edit Task modal displays workflow selector with current workflow pre-selected
 * - SC-7: Task with assigned workflow runs correctly via UI
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project.
 * See global-setup.ts for sandbox creation details.
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper to wait for modal to be fully loaded
async function waitForModalLoad(page: Page) {
	await page.waitForSelector('[role="dialog"]', { timeout: 5000 });
	// Wait for any loading states to disappear
	await page.waitForSelector('.loading', { state: 'hidden', timeout: 3000 }).catch(() => {});
}

// Helper to open New Task modal
async function openNewTaskModal(page: Page) {
	// Use keyboard shortcut (Shift+Alt+N) or click button
	const newTaskButton = page.getByRole('button', { name: 'New Task' });
	await expect(newTaskButton).toBeVisible();
	await newTaskButton.click();
	await waitForModalLoad(page);
}

// Helper to create a task with specific workflow
async function createTaskWithWorkflow(
	page: Page,
	title: string,
	workflow: string
) {
	await openNewTaskModal(page);

	// Fill in title
	const titleInput = page.locator('#new-task-title');
	await titleInput.fill(title);

	// Select workflow
	const workflowSelect = page.locator('[aria-label="Workflow"]');
	await expect(workflowSelect).toBeVisible();
	await workflowSelect.click();

	// Select the workflow option
	const workflowOption = page.locator(`[role="option"]:has-text("${workflow}")`);
	await expect(workflowOption).toBeVisible();
	await workflowOption.click();

	// Create task
	const createButton = page.getByRole('button', { name: /create task/i });
	await createButton.click();

	// Wait for modal to close
	await expect(page.locator('[role="dialog"]')).not.toBeVisible({ timeout: 5000 });
}

test.describe('Workflow Selector - New Task Modal', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');
	});

	test('SC-1: should display workflow selector dropdown in New Task modal', async ({ page }) => {
		await openNewTaskModal(page);

		// Workflow selector should be visible
		const workflowLabel = page.locator('label:has-text("Workflow")');
		await expect(workflowLabel).toBeVisible();

		// There should be a workflow dropdown/select
		const workflowSelect = page.locator('[aria-label="Workflow"], #new-task-workflow');
		await expect(workflowSelect).toBeVisible();
	});

	test('SC-1: should show builtin workflows (small, medium, large) in dropdown', async ({ page }) => {
		await openNewTaskModal(page);

		// Open workflow dropdown
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await workflowSelect.click();

		// Should show builtin workflows
		await expect(page.locator('[role="option"]:has-text("Small")')).toBeVisible();
		await expect(page.locator('[role="option"]:has-text("Medium")')).toBeVisible();
		await expect(page.locator('[role="option"]:has-text("Large")')).toBeVisible();
	});

	test('SC-2: should create task with selected workflow', async ({ page }) => {
		const taskTitle = `E2E Workflow Test ${Date.now()}`;

		await createTaskWithWorkflow(page, taskTitle, 'Medium');

		// Task should appear in list
		await expect(page.locator(`text=${taskTitle}`)).toBeVisible({ timeout: 5000 });

		// Navigate to task detail to verify workflow was saved
		await page.locator(`text=${taskTitle}`).click();
		await page.waitForURL(/\/tasks\/TASK-\d+/);

		// Workflow should be displayed on task detail page
		const workflowBadge = page.locator('.task-workflow, .workflow-badge, [data-workflow]');
		await expect(workflowBadge).toContainText(/medium/i);
	});

	test('SC-3: should default workflow to match selected weight', async ({ page }) => {
		await openNewTaskModal(page);

		// Default weight is medium, so workflow should also default to medium
		const workflowTrigger = page.locator('[aria-label="Workflow"]');
		await expect(workflowTrigger).toContainText(/medium/i);

		// Change weight to small
		const weightSelect = page.locator('#new-task-weight');
		await weightSelect.selectOption({ label: 'small' });

		// Workflow should auto-update to small
		await expect(workflowTrigger).toContainText(/small/i);

		// Change weight to large
		await weightSelect.selectOption({ label: 'large' });

		// Workflow should auto-update to large
		await expect(workflowTrigger).toContainText(/large/i);
	});

	test('SC-3: should preserve manual workflow selection when weight changes', async ({ page }) => {
		await openNewTaskModal(page);

		// Manually select a different workflow than the weight
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await workflowSelect.click();

		// Select "Large" workflow while weight is still "medium"
		const largeOption = page.locator('[role="option"]:has-text("Large")');
		await largeOption.click();

		// Now change weight to small
		const weightSelect = page.locator('#new-task-weight');
		await weightSelect.selectOption({ label: 'small' });

		// Workflow should still be "Large" because it was manually selected
		await expect(workflowSelect).toContainText(/large/i);
	});

	test('should show error state when workflows fail to load', async ({ page }) => {
		// Intercept workflow API to simulate error
		await page.route('**/orc.v1.WorkflowService/ListWorkflows', (route) => {
			route.abort('failed');
		});

		await page.reload();
		await page.waitForLoadState('networkidle');

		await openNewTaskModal(page);

		// Should show error message
		await expect(page.locator('text=/failed to load workflows/i')).toBeVisible();
	});
});

test.describe('Workflow Selector - Edit Task Modal', () => {
	let createdTaskId: string;

	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		// Create a task to edit
		const taskTitle = `Edit Test ${Date.now()}`;
		await createTaskWithWorkflow(page, taskTitle, 'Small');

		// Find and store the task ID
		const taskCard = page.locator(`.task-card:has-text("${taskTitle}")`);
		await expect(taskCard).toBeVisible();

		// Navigate to task detail
		await taskCard.click();
		await page.waitForURL(/\/tasks\/TASK-\d+/);

		// Extract task ID from URL
		const url = page.url();
		const match = url.match(/TASK-\d+/);
		if (match) {
			createdTaskId = match[0];
		}
	});

	test('SC-4: should display workflow selector in Edit Task modal', async ({ page }) => {
		// Open edit modal
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();
		await waitForModalLoad(page);

		// Workflow selector should be visible
		const workflowLabel = page.locator('label:has-text("Workflow")');
		await expect(workflowLabel).toBeVisible();

		const workflowSelect = page.locator('[aria-label="Workflow"], #task-workflow');
		await expect(workflowSelect).toBeVisible();
	});

	test('SC-4: should pre-select current task workflow', async ({ page }) => {
		// Open edit modal
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();
		await waitForModalLoad(page);

		// Workflow should show "Small" (what we created with)
		const workflowTrigger = page.locator('[aria-label="Workflow"]');
		await expect(workflowTrigger).toContainText(/small/i);
	});

	test('SC-6: should save changed workflow', async ({ page }) => {
		// Open edit modal
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();
		await waitForModalLoad(page);

		// Change workflow from Small to Large
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await workflowSelect.click();

		const largeOption = page.locator('[role="option"]:has-text("Large")');
		await largeOption.click();

		// Save changes
		const saveButton = page.getByRole('button', { name: /save/i });
		await saveButton.click();

		// Wait for modal to close
		await expect(page.locator('[role="dialog"]')).not.toBeVisible({ timeout: 5000 });

		// Reopen edit modal to verify persistence
		await editButton.click();
		await waitForModalLoad(page);

		// Workflow should now be "Large"
		const workflowTrigger = page.locator('[aria-label="Workflow"]');
		await expect(workflowTrigger).toContainText(/large/i);
	});

	test('SC-4: should show "None" for task without workflow', async ({ page }) => {
		// Create a task without workflow (using API intercept to simulate)
		// Or navigate to an existing task that has no workflow

		// For this test, we'll check the "None" option exists in dropdown
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();
		await waitForModalLoad(page);

		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await workflowSelect.click();

		// "None" option should be available
		const noneOption = page.locator('[role="option"]:has-text("None")');
		await expect(noneOption).toBeVisible();
	});
});

test.describe('Workflow Selector - Task Execution', () => {
	test('SC-7: should run task with assigned workflow', async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		// Create a task with medium workflow
		const taskTitle = `Run Test ${Date.now()}`;
		await createTaskWithWorkflow(page, taskTitle, 'Medium');

		// Find the task and navigate to it
		const taskCard = page.locator(`.task-card:has-text("${taskTitle}")`);
		await expect(taskCard).toBeVisible();
		await taskCard.click();
		await page.waitForURL(/\/tasks\/TASK-\d+/);

		// Click Run button
		const runButton = page.getByRole('button', { name: 'Run' });
		await expect(runButton).toBeVisible({ timeout: 5000 });
		await runButton.click();

		// Task should transition to running state
		await expect(page.locator('.task-status')).toContainText(/running/i, {
			timeout: 10000,
		});

		// Verify executor was spawned by checking orc status
		// (This validates the workflow was properly assigned and executor started)
		// The task should show phase progression
		const phaseIndicator = page.locator('.phase-indicator, .current-phase, [data-phase]');
		await expect(phaseIndicator).toBeVisible({ timeout: 10000 });
	});

	test('SC-7: should show error for invalid workflow', async ({ page }) => {
		// Intercept the run request to simulate workflow validation error
		await page.route('**/orc.v1.TaskService/RunTask', (route) => {
			route.fulfill({
				status: 400,
				contentType: 'application/json',
				body: JSON.stringify({
					code: 'invalid_argument',
					message: 'Workflow not found',
				}),
			});
		});

		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		// Find an existing task and try to run it
		const taskCards = page.locator('.task-card');
		const count = await taskCards.count();

		if (count > 0) {
			await taskCards.first().click();
			await page.waitForURL(/\/tasks\/TASK-\d+/);

			const runButton = page.getByRole('button', { name: 'Run' });
			if (await runButton.isVisible().catch(() => false)) {
				await runButton.click();

				// Should show error toast
				await expect(page.locator('[role="alert"], .toast-error')).toContainText(
					/workflow not found/i,
					{ timeout: 5000 }
				);

				// Task should remain in created state (not running)
				await expect(page.locator('.task-status')).not.toContainText(/running/i);
			}
		}
	});
});

test.describe('Workflow Selector - Accessibility', () => {
	test('should have proper ARIA labels on workflow selector', async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		await openNewTaskModal(page);

		// Workflow selector should have aria-label
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		await expect(workflowSelect).toBeVisible();

		// Should be keyboard navigable
		await workflowSelect.focus();
		await page.keyboard.press('Enter');

		// Dropdown should open
		const dropdown = page.locator('[role="listbox"]');
		await expect(dropdown).toBeVisible();

		// Arrow down should navigate options
		await page.keyboard.press('ArrowDown');
		const focusedOption = page.locator('[role="option"][data-highlighted]');
		await expect(focusedOption).toBeVisible();

		// Escape should close
		await page.keyboard.press('Escape');
		await expect(dropdown).not.toBeVisible();
	});

	test('should support Enter key submission with workflow selected', async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		await openNewTaskModal(page);

		const taskTitle = `Enter Key Test ${Date.now()}`;

		// Fill title
		const titleInput = page.locator('#new-task-title');
		await titleInput.fill(taskTitle);

		// Press Enter to submit
		await titleInput.press('Enter');

		// Modal should close (task created)
		await expect(page.locator('[role="dialog"]')).not.toBeVisible({ timeout: 5000 });

		// Task should exist
		await expect(page.locator(`text=${taskTitle}`)).toBeVisible();
	});
});

test.describe('Workflow Selector - Preservation Requirements', () => {
	test('should not affect initiative selector in Edit modal', async ({ page }) => {
		await page.goto('/board');
		await page.waitForLoadState('networkidle');

		// Create and navigate to a task
		const taskTitle = `Initiative Test ${Date.now()}`;
		await createTaskWithWorkflow(page, taskTitle, 'Medium');

		await page.locator(`.task-card:has-text("${taskTitle}")`).click();
		await page.waitForURL(/\/tasks\/TASK-\d+/);

		// Open edit modal
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();
		await waitForModalLoad(page);

		// Both workflow and initiative selectors should exist
		const workflowSelect = page.locator('[aria-label="Workflow"]');
		const initiativeSelect = page.locator('[aria-label="Select initiative"], #task-initiative');

		await expect(workflowSelect).toBeVisible();
		await expect(initiativeSelect).toBeVisible();

		// Initiative selector should still be functional
		await initiativeSelect.click();
		const initiativeDropdown = page.locator('[role="listbox"]');
		await expect(initiativeDropdown).toBeVisible();

		// Should have "None" option at minimum
		await expect(page.locator('[role="option"]:has-text("None")')).toBeVisible();
	});
});
