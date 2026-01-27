/**
 * E2E Tests for TASK-556: Modal dropdown click interception bug
 *
 * This file tests that Radix Select dropdown options inside modals can be
 * clicked with the mouse. The bug was that modal backdrop (z-index: 1000)
 * intercepted pointer events before they reached the dropdown content
 * (z-index: 100).
 *
 * Success Criteria Coverage:
 * - SC-1: Radix Select dropdown content renders above modal backdrop
 * - SC-2: NewTaskModal workflow dropdown options are clickable with mouse
 * - SC-3: TaskEditModal workflow and initiative dropdowns are clickable
 * - SC-4: Existing keyboard navigation continues to work
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project.
 * See global-setup.ts for sandbox creation details.
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper to wait for board page to load
async function waitForBoardLoad(page: Page) {
	// Wait for the New Task button to be visible - this indicates the page is ready
	await expect(page.getByRole('button', { name: 'New Task' })).toBeVisible({ timeout: 15000 });
}

// Helper to wait for modal to be fully loaded
async function waitForModalLoad(page: Page) {
	await page.waitForSelector('[role="dialog"]', { timeout: 5000 });
	// Wait for any loading states to disappear
	await page.waitForSelector('.loading', { state: 'hidden', timeout: 3000 }).catch(() => {});
}

// Helper to open New Task modal
async function openNewTaskModal(page: Page) {
	const newTaskButton = page.getByRole('button', { name: 'New Task' });
	await expect(newTaskButton).toBeVisible();
	await newTaskButton.click();
	await waitForModalLoad(page);
}

// Helper to create a task and navigate to its detail page
async function createTaskAndNavigate(page: Page, title: string) {
	await openNewTaskModal(page);

	// Fill in title
	const titleInput = page.locator('#new-task-title');
	await titleInput.fill(title);

	// Create task
	const createButton = page.getByRole('button', { name: /create task/i });
	await createButton.click();

	// Wait for modal to close
	await expect(page.locator('[role="dialog"]')).not.toBeVisible({ timeout: 5000 });

	// Navigate to task detail
	await page.locator(`.task-card:has-text("${title}")`).click();
	await page.waitForURL(/\/tasks\/TASK-\d+/);
}

test.describe('TASK-556: Modal Dropdown Click Bug Fix', () => {
	test.describe('SC-1, SC-2: NewTaskModal Workflow Dropdown Mouse Clicks', () => {
		test.beforeEach(async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);
		});

		test('should click workflow dropdown option with mouse (SC-1, SC-2)', async ({ page }) => {
			await openNewTaskModal(page);

			// Open workflow dropdown
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await expect(workflowTrigger).toBeVisible();
			await workflowTrigger.click();

			// Wait for dropdown to open
			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Click on a specific workflow option (NOT keyboard navigation)
			// This is the critical test - clicking should work, not be intercepted by modal backdrop
			const largeOption = page.locator('[role="option"]:has-text("Large")');
			await expect(largeOption).toBeVisible();

			// Use force: false to ensure we're testing real click interception behavior
			// If the backdrop intercepts, this will fail with "element intercepts pointer events"
			await largeOption.click({ force: false });

			// Dropdown should close after selection
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });

			// Trigger should show selected value
			await expect(workflowTrigger).toContainText(/large/i);
		});

		test('should click Small workflow option (SC-2)', async ({ page }) => {
			await openNewTaskModal(page);

			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			// Wait for dropdown options to be visible
			const smallOption = page.locator('[role="option"]:has-text("Small")');
			await expect(smallOption).toBeVisible({ timeout: 3000 });

			// Click the option - this must not be intercepted by modal backdrop
			await smallOption.click({ force: false });

			// Verify selection succeeded
			await expect(workflowTrigger).toContainText(/small/i);
		});

		test('should click Medium workflow option (SC-2)', async ({ page }) => {
			await openNewTaskModal(page);

			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			const mediumOption = page.locator('[role="option"]:has-text("Medium")');
			await expect(mediumOption).toBeVisible({ timeout: 3000 });

			await mediumOption.click({ force: false });

			await expect(workflowTrigger).toContainText(/medium/i);
		});

		test('should click None workflow option (SC-2)', async ({ page }) => {
			await openNewTaskModal(page);

			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			const noneOption = page.locator('[role="option"]:has-text("None")');
			await expect(noneOption).toBeVisible({ timeout: 3000 });

			await noneOption.click({ force: false });

			await expect(workflowTrigger).toContainText(/none/i);
		});

		test('should allow rapid open/close dropdown clicks (edge case)', async ({ page }) => {
			await openNewTaskModal(page);

			const workflowTrigger = page.locator('[aria-label="Workflow"]');

			// Rapidly open, close, open again
			await workflowTrigger.click();
			await expect(page.locator('[role="listbox"]')).toBeVisible({ timeout: 2000 });

			// Press Escape to close dropdown (more reliable than position-based click)
			await page.keyboard.press('Escape');
			await expect(page.locator('[role="listbox"]')).not.toBeVisible({ timeout: 2000 });

			// Modal should still be open after Escape closes dropdown
			await expect(page.locator('[role="dialog"]')).toBeVisible();

			// Open again and click an option
			await workflowTrigger.click();
			const largeOption = page.locator('[role="option"]:has-text("Large")');
			await expect(largeOption).toBeVisible({ timeout: 2000 });
			await largeOption.click({ force: false });

			// Should still work
			await expect(workflowTrigger).toContainText(/large/i);
		});
	});

	test.describe('SC-3: TaskEditModal Workflow Dropdown Mouse Clicks', () => {
		test.beforeEach(async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Create a task to edit
			const taskTitle = `Edit Workflow Test ${Date.now()}`;
			await createTaskAndNavigate(page, taskTitle);
		});

		test('should click workflow dropdown option in Edit modal (SC-3)', async ({ page }) => {
			// Open edit modal - use specific aria-label to avoid matching task-cards
			const editButton = page.getByRole('button', { name: 'Edit task' });
			await editButton.click();
			await waitForModalLoad(page);

			// Click workflow dropdown trigger
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await expect(workflowTrigger).toBeVisible();
			await workflowTrigger.click();

			// Wait for dropdown to be visible
			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Click on Large workflow option
			const largeOption = page.locator('[role="option"]:has-text("Large")');
			await expect(largeOption).toBeVisible();

			// This is the critical test - click must not be intercepted by modal backdrop
			await largeOption.click({ force: false });

			// Dropdown should close
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });

			// Trigger should show new selection
			await expect(workflowTrigger).toContainText(/large/i);
		});

		test('should save changed workflow selection (SC-3)', async ({ page }) => {
			// Open edit modal - use specific aria-label to avoid matching task-cards
			const editButton = page.getByRole('button', { name: 'Edit task' });
			await editButton.click();
			await waitForModalLoad(page);

			// Change workflow
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			const largeOption = page.locator('[role="option"]:has-text("Large")');
			await expect(largeOption).toBeVisible({ timeout: 3000 });
			await largeOption.click({ force: false });

			// Save changes
			const saveButton = page.getByRole('button', { name: /save/i });
			await saveButton.click();

			// Wait for modal to close
			await expect(page.locator('[role="dialog"]')).not.toBeVisible({ timeout: 5000 });

			// Reopen edit modal to verify persistence
			await editButton.click();
			await waitForModalLoad(page);

			// Workflow should still be Large
			const workflowTriggerAfter = page.locator('[aria-label="Workflow"]');
			await expect(workflowTriggerAfter).toContainText(/large/i);
		});
	});

	test.describe('SC-3: TaskEditModal Initiative Dropdown Mouse Clicks', () => {
		test.beforeEach(async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Create a task to edit
			const taskTitle = `Edit Initiative Test ${Date.now()}`;
			await createTaskAndNavigate(page, taskTitle);
		});

		test('should click initiative dropdown option in Edit modal (SC-3)', async ({ page }) => {
			// Open edit modal - use specific aria-label to avoid matching task-cards
			const editButton = page.getByRole('button', { name: 'Edit task' });
			await editButton.click();
			await waitForModalLoad(page);

			// Click initiative dropdown trigger
			const initiativeTrigger = page.locator('[aria-label="Select initiative"]');
			await expect(initiativeTrigger).toBeVisible();
			await initiativeTrigger.click();

			// Wait for dropdown to be visible
			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Click on "None" option (always available)
			const noneOption = page.locator('[role="option"]:has-text("None")');
			await expect(noneOption).toBeVisible();

			// This click must not be intercepted by modal backdrop
			await noneOption.click({ force: false });

			// Dropdown should close
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });
		});

		test('should handle both workflow and initiative dropdowns in same modal (SC-3)', async ({ page }) => {
			// Open edit modal - use specific aria-label to avoid matching task-cards
			const editButton = page.getByRole('button', { name: 'Edit task' });
			await editButton.click();
			await waitForModalLoad(page);

			// First: change workflow using mouse click
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			let dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			const smallOption = page.locator('[role="option"]:has-text("Small")');
			await smallOption.click({ force: false });

			await expect(dropdown).not.toBeVisible({ timeout: 3000 });
			await expect(workflowTrigger).toContainText(/small/i);

			// Second: interact with initiative dropdown
			const initiativeTrigger = page.locator('[aria-label="Select initiative"]');
			await initiativeTrigger.click();

			dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			const noneOption = page.locator('[role="option"]:has-text("None")');
			await noneOption.click({ force: false });

			await expect(dropdown).not.toBeVisible({ timeout: 3000 });

			// Both selections should be maintained
			await expect(workflowTrigger).toContainText(/small/i);
		});
	});

	test.describe('SC-4: Keyboard Navigation Regression Tests', () => {
		test.beforeEach(async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);
		});

		test('should still support keyboard navigation in NewTaskModal (SC-4)', async ({ page }) => {
			await openNewTaskModal(page);

			// Focus workflow dropdown trigger
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.focus();

			// Open dropdown with Enter
			await page.keyboard.press('Enter');

			// Dropdown should open
			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Navigate with Arrow keys
			await page.keyboard.press('ArrowDown');
			await page.keyboard.press('ArrowDown');

			// Select with Enter
			await page.keyboard.press('Enter');

			// Dropdown should close
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });

			// Some workflow should be selected (not "None" after navigating down twice)
			// The exact value depends on dropdown order, but it shouldn't be the placeholder
		});

		test('should still support keyboard navigation in TaskEditModal (SC-4)', async ({ page }) => {
			// Create a task first
			const taskTitle = `Keyboard Nav Test ${Date.now()}`;
			await createTaskAndNavigate(page, taskTitle);

			// Open edit modal - use specific aria-label to avoid matching task-cards
			const editButton = page.getByRole('button', { name: 'Edit task' });
			await editButton.click();
			await waitForModalLoad(page);

			// Focus and open workflow dropdown with keyboard
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.focus();
			await page.keyboard.press('Enter');

			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Navigate and select with keyboard
			await page.keyboard.press('ArrowDown');
			await page.keyboard.press('Enter');

			// Should close after selection
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });
		});

		test('should close dropdown with Escape key (SC-4)', async ({ page }) => {
			await openNewTaskModal(page);

			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Press Escape to close
			await page.keyboard.press('Escape');

			// Dropdown should close but modal should remain open
			await expect(dropdown).not.toBeVisible({ timeout: 3000 });
			await expect(page.locator('[role="dialog"]')).toBeVisible();
		});
	});

	test.describe('Preservation Requirements', () => {
		test('modal can be closed by clicking backdrop', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			await openNewTaskModal(page);

			// Modal should be visible
			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			// Click on backdrop (outside modal content)
			const backdrop = page.locator('.modal-backdrop');
			await backdrop.click({ position: { x: 10, y: 10 } });

			// Modal should close
			await expect(modal).not.toBeVisible({ timeout: 3000 });
		});

		test('modal focus trap still works', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			await openNewTaskModal(page);

			// Tab through the modal several times
			for (let i = 0; i < 10; i++) {
				await page.keyboard.press('Tab');
			}

			// Focus should still be within the modal
			const modal = page.locator('[role="dialog"]');
			const focusedElement = page.locator(':focus');

			// The focused element should be a descendant of the modal
			// or the modal backdrop (for Radix Dialog focus management)
			const isInModal = await modal.locator(':focus').count() > 0;
			const isBackdrop = await focusedElement.evaluate(
				(el: Element) => el.classList.contains('modal-backdrop')
			);

			expect(isInModal || isBackdrop).toBe(true);
		});
	});

	test.describe('Edge Cases', () => {
		test('dropdown extends near viewport edge should still be clickable', async ({ page }) => {
			// Resize viewport to be smaller
			await page.setViewportSize({ width: 600, height: 500 });

			await page.goto('/board');
			await waitForBoardLoad(page);

			await openNewTaskModal(page);

			// Open dropdown
			const workflowTrigger = page.locator('[aria-label="Workflow"]');
			await workflowTrigger.click();

			const dropdown = page.locator('[role="listbox"]');
			await expect(dropdown).toBeVisible({ timeout: 3000 });

			// Even at viewport edge, clicking should work
			const largeOption = page.locator('[role="option"]:has-text("Large")');
			await expect(largeOption).toBeVisible();
			await largeOption.click({ force: false });

			await expect(workflowTrigger).toContainText(/large/i);
		});
	});
});
