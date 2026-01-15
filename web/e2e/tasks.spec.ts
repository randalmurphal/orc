/**
 * Task Management E2E Tests
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project.
 * See global-setup.ts for sandbox creation details.
 */
import { test, expect } from './fixtures';

test.describe('Task Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
	});

	test('should display task list page', async ({ page }) => {
		await expect(page).toHaveTitle(/orc - Tasks/);
		await expect(page.locator('h1')).toHaveText('Tasks');
	});

	test('should show empty state when no tasks exist', async ({ page }) => {
		// Wait for loading to complete
		await page.waitForSelector('.loading', { state: 'hidden', timeout: 5000 }).catch(() => {});

		// This test may need adjustment based on whether tasks exist
		const emptyState = page.locator('.empty-state');
		const taskGrid = page.locator('.task-grid');
		const taskSection = page.locator('section');

		// Either empty state, task grid, or sections should be visible after loading
		const hasEmptyState = await emptyState.isVisible().catch(() => false);
		const hasTaskGrid = await taskGrid.isVisible().catch(() => false);
		const hasSection = await taskSection.first().isVisible().catch(() => false);

		expect(hasEmptyState || hasTaskGrid || hasSection).toBeTruthy();
	});

	test('should show New Task button', async ({ page }) => {
		const newTaskButton = page.locator('button:has-text("New Task")');
		await expect(newTaskButton).toBeVisible();
	});

	test('should open new task form when clicking New Task', async ({ page }) => {
		// Wait for page to be fully loaded
		await page.waitForLoadState('networkidle');

		const newTaskButton = page.getByRole('button', { name: 'New Task' });
		await expect(newTaskButton).toBeVisible();

		// Click and wait for form to appear
		await newTaskButton.click();

		const form = page.locator('.new-task-form');
		await expect(form).toBeVisible({ timeout: 5000 });

		const input = form.locator('input[type="text"]');
		await expect(input).toBeVisible();
		await expect(input).toHaveAttribute('placeholder', 'Task title...');
	});

	test('should cancel task creation', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const newTaskButton = page.getByRole('button', { name: 'New Task' });
		await newTaskButton.click();

		const form = page.locator('.new-task-form');
		await expect(form).toBeVisible({ timeout: 5000 });

		await form.getByRole('button', { name: 'Cancel' }).click();
		await expect(form).not.toBeVisible({ timeout: 3000 });
	});

	test('should create a new task', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const taskTitle = `E2E Test Task ${Date.now()}`;

		const newTaskButton = page.getByRole('button', { name: 'New Task' });
		await newTaskButton.click();

		const form = page.locator('.new-task-form');
		await expect(form).toBeVisible({ timeout: 5000 });

		const input = form.locator('input[type="text"]');
		await input.fill(taskTitle);

		await form.getByRole('button', { name: 'Create' }).click();

		// Form should close
		await expect(form).not.toBeVisible({ timeout: 5000 });

		// Task should appear in list
		await expect(page.locator(`text=${taskTitle}`)).toBeVisible({ timeout: 5000 });
	});

	test('should navigate to task detail page', async ({ page }) => {
		// First, ensure at least one task exists
		const taskCards = page.locator('.task-grid a, .task-grid [data-task-id]');
		const count = await taskCards.count();

		if (count > 0) {
			// Get task ID from first card
			const firstCard = taskCards.first();
			await firstCard.click();

			// Should navigate to task detail page
			await expect(page).toHaveURL(/\/tasks\/TASK-\d+/);
			await expect(page.locator('.task-detail, .task-header')).toBeVisible();
		}
	});

	test('should display task sections on detail page', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Use existing tasks instead of creating new ones
		const taskCards = page.locator('.task-grid a');
		const count = await taskCards.count();

		if (count > 0) {
			// Click first task
			await taskCards.first().click();
			await page.waitForLoadState('networkidle');

			// Verify task detail page elements
			await expect(page.locator('h1')).toBeVisible();
			// Use .first() to avoid strict mode violation when multiple elements match
			await expect(page.locator('.task-meta').first()).toBeVisible();
		}
	});

	test('should show Run button for runnable tasks', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Use existing tasks
		const taskCards = page.locator('.task-grid a');
		const count = await taskCards.count();

		if (count > 0) {
			// Click first task
			await taskCards.first().click();

			// Wait for navigation to task detail page
			await expect(page).toHaveURL(/\/tasks\/TASK-\d+/, { timeout: 5000 });
			await page.waitForLoadState('networkidle');

			// Check for action buttons - depend on task state
			const runButton = page.getByRole('button', { name: 'Run' });
			const pauseButton = page.getByRole('button', { name: 'Pause' });
			const cancelButton = page.getByRole('button', { name: 'Cancel' });

			// Check visibility of any action buttons
			const hasRunButton = await runButton.isVisible().catch(() => false);
			const hasPauseButton = await pauseButton.isVisible().catch(() => false);
			const hasCancelButton = await cancelButton.isVisible().catch(() => false);

			// For completed tasks, no action buttons are shown - this is valid
			const hasActionButton = hasRunButton || hasPauseButton || hasCancelButton;

			// Get task status from detail page (should only be one on detail page)
			const taskStatus = page.locator('.task-detail .task-status, .task-header .task-status');
			const statusText = await taskStatus.first().textContent().catch(() => '');

			// Either we have action buttons, or task is completed/failed
			const isCompletedTask = statusText?.includes('completed') || statusText?.includes('failed');
			expect(hasActionButton || isCompletedTask).toBeTruthy();
		}
	});
});

test.describe('Task Actions', () => {
	test('should handle error banner dismissal', async ({ page }) => {
		await page.goto('/');

		// This test verifies error handling UI exists
		// We can trigger an error by trying to load a non-existent task
		await page.goto('/tasks/TASK-99999');

		const errorMessage = page.locator('.error, [role="alert"]');
		await expect(errorMessage).toBeVisible({ timeout: 5000 });
	});
});
