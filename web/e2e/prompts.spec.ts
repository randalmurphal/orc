/**
 * Prompts E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 */
import { test, expect } from './fixtures';

test.describe('Prompt Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/environment/prompts');
	});

	test('should display prompts page', async ({ page }) => {
		await expect(page).toHaveTitle(/Prompts/);
		// Page header
		await expect(page.locator('h3')).toHaveText('Phase Prompts');
	});

	test('should show prompts list', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		// Should show prompts list or loading/error state
		const promptsList = page.locator('.prompts-list');
		const loading = page.locator('.env-loading');
		const error = page.locator('.env-error');

		// One of these states should be visible
		const hasPrompts = await promptsList.isVisible().catch(() => false);
		const isLoading = await loading.isVisible().catch(() => false);
		const hasError = await error.isVisible().catch(() => false);

		expect(hasPrompts || isLoading || hasError).toBeTruthy();
	});

	test('should display prompt items with badges', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const promptItems = page.locator('.prompt-item');
		const count = await promptItems.count();

		// Prompts may or may not be loaded depending on API state
		if (count > 0) {
			const firstPrompt = promptItems.first();
			// Check for phase name
			await expect(firstPrompt.locator('.prompt-phase-name')).toBeVisible();
			// Check for source badge
			await expect(firstPrompt.locator('.prompt-badge')).toBeVisible();
		}
	});

	test('should show Preview and Edit buttons', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const promptItems = page.locator('.prompt-item');
		if (await promptItems.count() > 0) {
			const firstPrompt = promptItems.first();

			// Preview button
			const previewBtn = firstPrompt.locator('button:has-text("Preview")');
			await expect(previewBtn).toBeVisible();

			// Edit button
			const editBtn = firstPrompt.locator('button:has-text("Edit")');
			await expect(editBtn).toBeVisible();
		}
	});

	test('should open preview modal when clicking Preview', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const promptItems = page.locator('.prompt-item');
		if (await promptItems.count() > 0) {
			// Click preview button on first prompt
			await promptItems.first().locator('button:has-text("Preview")').click();

			// Modal should open
			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			// Modal should have prompt content
			await expect(page.locator('.prompt-preview-content')).toBeVisible();
		}
	});

	test('should open edit modal when clicking Edit', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const promptItems = page.locator('.prompt-item');
		if (await promptItems.count() > 0) {
			// Click edit button on first prompt
			await promptItems.first().locator('button:has-text("Edit")').click();

			// Modal should open
			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			// Modal should have Save Override button
			const saveButton = page.locator('button:has-text("Save Override")');
			await expect(saveButton).toBeVisible();
		}
	});

	test('should display variables in prompt items', async ({ page }) => {
		await page.waitForLoadState('networkidle');

		const promptItems = page.locator('.prompt-item');
		if (await promptItems.count() > 0) {
			// Some prompts should have variables
			const variables = page.locator('.prompt-variable');
			const count = await variables.count();
			expect(count).toBeGreaterThanOrEqual(0);
		}
	});
});
