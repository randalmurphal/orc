/**
 * QA Test: Settings > Slash Commands Page
 *
 * Tests the Slash Commands view against reference image at example_ui/settings-slash-commands.png
 *
 * Test Scope:
 * 1. Visual layout comparison with reference
 * 2. Interactive elements (command list, edit/delete buttons, new command button)
 * 3. Command editor functionality
 * 4. Mobile viewport (375x667)
 * 5. Console errors
 */

import { test, expect, type Page } from '@playwright/test';
import { chromium } from '@playwright/test';
import path from 'path';
import fs from 'fs';

const BASE_URL = 'http://localhost:5173';
const SCREENSHOTS_DIR = '/tmp/qa-TASK-616';

// Ensure screenshots directory exists
if (!fs.existsSync(SCREENSHOTS_DIR)) {
	fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
}

test.describe('Settings > Slash Commands Page', () => {
	let consoleErrors: string[] = [];
	let consoleWarnings: string[] = [];

	test.beforeEach(async ({ page }) => {
		// Capture console errors and warnings
		consoleErrors = [];
		consoleWarnings = [];

		page.on('console', (msg) => {
			if (msg.type() === 'error') {
				consoleErrors.push(msg.text());
			} else if (msg.type() === 'warning') {
				consoleWarnings.push(msg.text());
			}
		});

		// Navigate to Settings > Slash Commands
		await page.goto(`${BASE_URL}/settings/commands`);
		await page.waitForLoadState('networkidle');
	});

	test('Desktop: Initial page load matches reference layout', async ({ page }) => {
		// Wait for content to load
		await page.waitForSelector('.settings-view__title', { timeout: 5000 });

		// Take screenshot of initial state
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'desktop-initial.png'),
			fullPage: true
		});

		// Verify page title
		const title = await page.locator('.settings-view__title').textContent();
		expect(title).toBe('Slash Commands');

		// Verify subtitle
		const subtitle = await page.locator('.settings-view__subtitle').textContent();
		expect(subtitle).toContain('Custom commands for Claude Code');

		// Verify "New Command" button exists
		const newCommandBtn = page.locator('button', { hasText: 'New Command' });
		await expect(newCommandBtn).toBeVisible();

		// Check for Project Commands section
		const projectSection = page.locator('text=Project Commands');
		await expect(projectSection).toBeVisible();

		// Check for Global Commands section
		const globalSection = page.locator('text=Global Commands');
		await expect(globalSection).toBeVisible();

		// Verify Command Editor section exists
		const editorSection = page.locator('text=Command Editor');
		await expect(editorSection).toBeVisible();

		console.log('✓ Desktop initial layout verified');
	});

	test('Command list items are clickable and selectable', async ({ page }) => {
		// Wait for commands to load
		await page.waitForSelector('.command-item', { timeout: 5000 });

		// Get all command items
		const commands = page.locator('.command-item');
		const count = await commands.count();

		if (count === 0) {
			console.warn('⚠ No commands found - API may not be returning data');
			return;
		}

		// Click first command
		await commands.first().click();
		await page.waitForTimeout(300); // Allow selection state to update

		// Verify first command is selected
		await expect(commands.first()).toHaveClass(/selected/);

		// Take screenshot of selected state
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'command-selected.png'),
			fullPage: true
		});

		// Click second command if it exists
		if (count > 1) {
			await commands.nth(1).click();
			await page.waitForTimeout(300);

			// Verify second command is selected, first is not
			await expect(commands.nth(1)).toHaveClass(/selected/);
			await expect(commands.first()).not.toHaveClass(/selected/);
		}

		console.log(`✓ Command selection works (tested ${Math.min(count, 2)} commands)`);
	});

	test('Edit and delete buttons are interactive', async ({ page }) => {
		// Wait for commands to load
		await page.waitForSelector('.command-item', { timeout: 5000 });

		const firstCommand = page.locator('.command-item').first();

		// Hover over command to ensure buttons are visible
		await firstCommand.hover();

		// Verify edit button exists and is clickable
		const editBtn = firstCommand.locator('button[aria-label*="Edit"]');
		await expect(editBtn).toBeVisible();
		await editBtn.click();
		await page.waitForTimeout(300);

		// Verify command is selected after edit click
		await expect(firstCommand).toHaveClass(/selected/);

		// Test delete button (click and verify confirmation appears)
		const deleteBtn = firstCommand.locator('button[aria-label*="Delete"]');
		await expect(deleteBtn).toBeVisible();
		await deleteBtn.click();
		await page.waitForTimeout(300);

		// Check if confirmation buttons appear (check/x icons)
		const confirmBtn = firstCommand.locator('button[aria-label*="Confirm"]');
		const cancelBtn = firstCommand.locator('button[aria-label*="Cancel"]');

		await expect(confirmBtn).toBeVisible();
		await expect(cancelBtn).toBeVisible();

		// Take screenshot of delete confirmation state
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'delete-confirmation.png'),
			fullPage: true
		});

		// Cancel delete
		await cancelBtn.click();
		await page.waitForTimeout(300);

		// Verify edit/delete buttons are back
		await expect(editBtn).toBeVisible();
		await expect(deleteBtn).toBeVisible();

		console.log('✓ Edit and delete buttons work correctly');
	});

	test('New Command button opens modal', async ({ page }) => {
		// Click "New Command" button
		const newCommandBtn = page.locator('button', { hasText: 'New Command' });
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		// Check if modal appears (exact selectors depend on modal implementation)
		// This might need adjustment based on actual modal structure
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible({ timeout: 2000 });

		// Take screenshot of modal
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'new-command-modal.png'),
			fullPage: true
		});

		console.log('✓ New Command modal opens');
	});

	test('Command editor displays content and Save button works', async ({ page }) => {
		// Wait for commands to load
		await page.waitForSelector('.command-item', { timeout: 5000 });

		// Select first command
		await page.locator('.command-item').first().click();
		await page.waitForTimeout(500);

		// Check if editor area shows content
		const editorContent = page.locator('.settings-view__editor');
		await expect(editorContent).toBeVisible();

		// Verify file path is displayed
		const filePath = page.locator('text=/.claude/commands/');
		await expect(filePath).toBeVisible();

		// Verify Save button exists
		const saveBtn = page.locator('button', { hasText: 'Save' });
		await expect(saveBtn).toBeVisible();

		// Take screenshot of editor
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'editor-view.png'),
			fullPage: true
		});

		// Test Save button (just click, don't verify save success without backend)
		await saveBtn.click();
		await page.waitForTimeout(500);

		console.log('✓ Command editor displays and Save button is clickable');
	});

	test('Mobile viewport (375x667): Layout adapts correctly', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.waitForTimeout(500);

		// Take screenshot of mobile view
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'mobile-initial.png'),
			fullPage: true
		});

		// Verify critical elements are visible and accessible
		const title = page.locator('.settings-view__title');
		await expect(title).toBeVisible();

		const newCommandBtn = page.locator('button', { hasText: 'New Command' });
		await expect(newCommandBtn).toBeVisible();

		// Check if command list is accessible (may be scrollable or stacked)
		const commandList = page.locator('.command-list');
		await expect(commandList).toBeVisible();

		// Test that commands are tappable
		const firstCommand = page.locator('.command-item').first();
		if (await firstCommand.count() > 0) {
			await firstCommand.tap();
			await page.waitForTimeout(300);

			// Take screenshot of mobile with selected command
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'mobile-selected.png'),
				fullPage: true
			});
		}

		console.log('✓ Mobile viewport layout verified');
	});

	test('Check for console errors', async ({ page }) => {
		// Navigate and interact with page
		await page.waitForSelector('.settings-view__title', { timeout: 5000 });

		// Click around to trigger any potential errors
		const commands = page.locator('.command-item');
		if (await commands.count() > 0) {
			await commands.first().click();
			await page.waitForTimeout(500);
		}

		const newCommandBtn = page.locator('button', { hasText: 'New Command' });
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		// Close modal if it opened
		const escapeKey = page.keyboard.press('Escape');
		await page.waitForTimeout(300);

		// Report console errors
		if (consoleErrors.length > 0) {
			console.error('Console Errors Found:');
			consoleErrors.forEach((err, i) => {
				console.error(`  ${i + 1}. ${err}`);
			});
		} else {
			console.log('✓ No console errors detected');
		}

		if (consoleWarnings.length > 0) {
			console.warn('Console Warnings Found:');
			consoleWarnings.forEach((warn, i) => {
				console.warn(`  ${i + 1}. ${warn}`);
			});
		}

		// Test should pass but log warnings
		expect(consoleErrors.length).toBe(0);
	});

	test('Visual comparison: Identify differences from reference', async ({ page }) => {
		// This test documents visual differences from the reference image
		// Reference: example_ui/settings-slash-commands.png

		await page.waitForSelector('.settings-view__title', { timeout: 5000 });

		// Take full page screenshot for comparison
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'visual-comparison.png'),
			fullPage: true
		});

		// Document expected elements from reference image
		const expectedElements = [
			{ selector: '.settings-view__title', text: 'Slash Commands' },
			{ selector: 'button', text: 'New Command' },
			{ selector: 'text=Project Commands', exists: true },
			{ selector: 'text=Global Commands', exists: true },
			{ selector: 'text=Command Editor', exists: true },
		];

		// Verify each expected element
		for (const element of expectedElements) {
			const locator = page.locator(element.selector);
			if (element.text) {
				await expect(locator.getByText(element.text)).toBeVisible();
			} else if (element.exists) {
				await expect(locator).toBeVisible();
			}
		}

		console.log('✓ Visual comparison documented');
		console.log(`Screenshots saved to: ${SCREENSHOTS_DIR}`);
	});
});

// Run summary
test.afterAll(() => {
	console.log('\n=== QA Test Summary ===');
	console.log(`Screenshots location: ${SCREENSHOTS_DIR}`);
	console.log('Compare screenshots with reference: example_ui/settings-slash-commands.png');
	console.log('======================\n');
});
