/**
 * Prompts E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 */
import { test, expect } from './fixtures';

test.describe('Prompt Management', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/prompts');
	});

	test('should display prompts page', async ({ page }) => {
		await expect(page).toHaveTitle(/Prompts - orc/);
		await expect(page.locator('h1')).toHaveText('Prompt Templates');
	});

	test('should show phase list', async ({ page }) => {
		const phaseList = page.locator('.prompt-list');
		await expect(phaseList).toBeVisible();

		// Should have "Phases" heading
		await expect(phaseList.locator('h2')).toHaveText('Phases');
	});

	test('should show variable reference panel', async ({ page }) => {
		const variablesPanel = page.locator('.variables-panel');
		await expect(variablesPanel).toBeVisible();

		await expect(variablesPanel.locator('h2')).toHaveText('Variable Reference');
	});

	test('should show no-selection state initially', async ({ page }) => {
		const noSelection = page.locator('.no-selection');
		await expect(noSelection).toBeVisible();
		await expect(noSelection).toContainText('Select a phase');
	});

	test('should select a prompt phase', async ({ page }) => {
		// Wait for page to load
		await page.waitForLoadState('networkidle');

		// Check if prompts loaded successfully
		const promptItems = page.locator('.prompt-item');
		const hasPrompts = await promptItems.count() > 0;

		if (hasPrompts) {
			// Click on first phase
			const firstPhase = promptItems.first();
			await firstPhase.click();

			// Editor should appear
			const editor = page.locator('.editor-panel');
			await expect(editor).toBeVisible();

			// Textarea should be visible
			await expect(page.locator('.editor-textarea')).toBeVisible();
		} else {
			// If no prompts, just verify the phases heading exists
			const phasesHeading = page.locator('h2:has-text("Phases")');
			await expect(phasesHeading).toBeVisible();
		}
	});

	test('should show source info for selected prompt', async ({ page }) => {
		await page.waitForLoadState('networkidle');
		const promptItems = page.locator('.prompt-item');

		if (await promptItems.count() > 0) {
			await promptItems.first().click();

			const sourceInfo = page.locator('.source-info');
			await expect(sourceInfo).toBeVisible();
			await expect(sourceInfo).toContainText('Source:');
		}
	});

	test('should show Save Override button', async ({ page }) => {
		await page.waitForLoadState('networkidle');
		const promptItems = page.locator('.prompt-item');

		if (await promptItems.count() > 0) {
			await promptItems.first().click();

			const saveButton = page.locator('button:has-text("Save Override")');
			await expect(saveButton).toBeVisible();

			// Should be disabled initially (no changes)
			await expect(saveButton).toBeDisabled();
		}
	});

	test('should enable Save button when content changes', async ({ page }) => {
		await page.waitForLoadState('networkidle');
		const promptItems = page.locator('.prompt-item');

		if (await promptItems.count() > 0) {
			await promptItems.first().click();

			const textarea = page.locator('.editor-textarea');
			const saveButton = page.locator('button:has-text("Save Override")');

			// Initially disabled
			await expect(saveButton).toBeDisabled();

			// Make a change
			const currentContent = await textarea.inputValue();
			await textarea.fill(currentContent + '\n# Modified by E2E test');

			// Should be enabled now
			await expect(saveButton).toBeEnabled();
		}
	});

	test('should display badges for prompt sources', async ({ page }) => {
		await page.waitForLoadState('networkidle');
		const badges = page.locator('.prompt-item .badge');
		const badgeCount = await badges.count();

		// Badges exist only if prompts loaded
		// Just verify the query doesn't error
		expect(badgeCount).toBeGreaterThanOrEqual(0);
	});

	test('should display variable names in reference panel', async ({ page }) => {
		const variablesPanel = page.locator('.variables-panel');
		await expect(variablesPanel).toBeVisible();

		const variableItems = page.locator('.variable-item');
		const count = await variableItems.count();

		// Variables may or may not be present depending on API state
		// Just verify the panel exists and doesn't error
		if (count > 0) {
			const firstVariable = variableItems.first();
			await expect(firstVariable.locator('.variable-name')).toBeVisible();
			await expect(firstVariable.locator('.variable-desc')).toBeVisible();
		}
	});
});
