/**
 * QA Iteration 2 - Bug Verification Test
 *
 * Verifies whether bugs QA-001 through QA-004 from Iteration 1 are now fixed.
 * Run with: bunx playwright test qa-iteration2-verification.spec.ts --headed
 */

import { test, expect } from '@playwright/test';

const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = '/tmp/qa-TASK-616-iteration2';

test.describe('QA Iteration 2: Bug Verification', () => {
	test.beforeEach(async ({ page }) => {
		// Navigate to settings page
		await page.goto(`${BASE_URL}/settings`);

		// Click on "Slash Commands" if it's a separate section
		// (adjust selector based on actual UI)
		const slashCommandsLink = page.locator('text=Slash Commands');
		if (await slashCommandsLink.isVisible({ timeout: 2000 }).catch(() => false)) {
			await slashCommandsLink.click();
		}

		// Wait for page to load
		await page.waitForLoadState('networkidle');
	});

	test('QA-001: Verify editor content updates when switching commands', async ({ page }) => {
		// Wait for command list to load
		await page.waitForSelector('[data-testid="config-editor"]', { timeout: 10000 });

		// Get all command cards (adjust selector as needed)
		const commandCards = page.locator('.command-item');
		const count = await commandCards.count();

		if (count < 2) {
			console.log('SKIP: Need at least 2 commands to test switching');
			test.skip();
			return;
		}

		// Click first command
		await commandCards.nth(0).click();
		await page.waitForTimeout(500); // Let editor update

		// Get editor content
		const editorTextarea = page.locator('[data-testid="config-editor-textarea"]');
		const firstContent = await editorTextarea.inputValue();

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/qa-001-first-command.png`,
			fullPage: true
		});

		// Click second command (different from first)
		await commandCards.nth(1).click();
		await page.waitForTimeout(500); // Let editor update

		// Get new editor content
		const secondContent = await editorTextarea.inputValue();

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/qa-001-second-command.png`,
			fullPage: true
		});

		// Verify content changed
		if (firstContent === secondContent) {
			console.error('BUG STILL PRESENT: Editor content did not update');
			expect(firstContent).not.toBe(secondContent);
		} else {
			console.log('FIXED: Editor content updated correctly');
		}
	});

	test('QA-002: Verify forward slash (/) validation in command names', async ({ page }) => {
		// Click "New Command" button
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();

		// Wait for modal to open
		await page.waitForSelector('[data-testid="modal"]', { timeout: 5000 });

		// Find name input
		const nameInput = page.locator('#new-command-name');
		await nameInput.fill('test/command');

		// Try to create
		const createBtn = page.locator('button:has-text("Create")');
		await createBtn.click();

		await page.waitForTimeout(1000); // Wait for validation

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/qa-002-slash-validation.png`,
			fullPage: true
		});

		// Check for error message
		const errorToast = page.locator('.toast-error, .error-message, text=/invalid.*character/i');
		const hasError = await errorToast.isVisible({ timeout: 2000 }).catch(() => false);

		if (!hasError) {
			console.error('BUG STILL PRESENT: No validation error for forward slash');
			expect(hasError).toBe(true);
		} else {
			console.log('FIXED: Forward slash validation working');
		}

		// Close modal if still open
		const cancelBtn = page.locator('button:has-text("Cancel")');
		if (await cancelBtn.isVisible().catch(() => false)) {
			await cancelBtn.click();
		}
	});

	test('QA-003: Verify space validation in command names', async ({ page }) => {
		// Click "New Command" button
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();

		// Wait for modal
		await page.waitForSelector('[data-testid="modal"]', { timeout: 5000 });

		// Fill name with space
		const nameInput = page.locator('#new-command-name');
		await nameInput.fill('test command');

		// Try to create
		const createBtn = page.locator('button:has-text("Create")');
		await createBtn.click();

		await page.waitForTimeout(1000);

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/qa-003-space-validation.png`,
			fullPage: true
		});

		// Check for error
		const errorToast = page.locator('.toast-error, .error-message, text=/space|invalid/i');
		const hasError = await errorToast.isVisible({ timeout: 2000 }).catch(() => false);

		if (!hasError) {
			console.error('BUG STILL PRESENT: No validation error for spaces');
			expect(hasError).toBe(true);
		} else {
			console.log('FIXED: Space validation working');
		}

		// Close modal
		const cancelBtn = page.locator('button:has-text("Cancel")');
		if (await cancelBtn.isVisible().catch(() => false)) {
			await cancelBtn.click();
		}
	});

	test('QA-004: Verify max length validation', async ({ page }) => {
		// Click "New Command" button
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();

		// Wait for modal
		await page.waitForSelector('[data-testid="modal"]', { timeout: 5000 });

		// Fill with 200 characters
		const longName = 'a'.repeat(200);
		const nameInput = page.locator('#new-command-name');
		await nameInput.fill(longName);

		// Try to create
		const createBtn = page.locator('button:has-text("Create")');
		await createBtn.click();

		await page.waitForTimeout(1000);

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/qa-004-length-validation.png`,
			fullPage: true
		});

		// Check for error or if input was truncated
		const actualValue = await nameInput.inputValue();
		const errorToast = page.locator('.toast-error, .error-message, text=/too long|max.*length/i');
		const hasError = await errorToast.isVisible({ timeout: 2000 }).catch(() => false);
		const wasTruncated = actualValue.length < longName.length;

		if (!hasError && !wasTruncated) {
			console.error('BUG STILL PRESENT: No max length validation');
			expect(hasError || wasTruncated).toBe(true);
		} else {
			console.log('FIXED: Max length validation working');
		}

		// Close modal
		const cancelBtn = page.locator('button:has-text("Cancel")');
		if (await cancelBtn.isVisible().catch(() => false)) {
			await cancelBtn.click();
		}
	});

	test('Additional: Test mobile viewport', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });

		await page.goto(`${BASE_URL}/settings`);

		// Click Slash Commands if separate
		const slashCommandsLink = page.locator('text=Slash Commands');
		if (await slashCommandsLink.isVisible({ timeout: 2000 }).catch(() => false)) {
			await slashCommandsLink.click();
		}

		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(500);

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-viewport-375x667.png`,
			fullPage: true
		});

		console.log('Mobile viewport screenshot captured');
	});
});

test.describe('Console Errors', () => {
	test('Check for JavaScript console errors', async ({ page }) => {
		const errors: string[] = [];

		page.on('console', (msg) => {
			if (msg.type() === 'error') {
				errors.push(msg.text());
			}
		});

		page.on('pageerror', (error) => {
			errors.push(error.message);
		});

		await page.goto(`${BASE_URL}/settings`);
		await page.waitForLoadState('networkidle');

		if (errors.length > 0) {
			console.error('Console errors detected:', errors);
		} else {
			console.log('No console errors');
		}

		expect(errors).toHaveLength(0);
	});
});
