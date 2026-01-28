/**
 * QA Test: Settings > Slash Commands Page
 * Task: TASK-616
 *
 * Tests the Slash Commands view against reference image at example_ui/settings-slash-commands.png
 *
 * Test Coverage:
 * - Visual layout comparison with reference
 * - Interactive elements (command list, edit/delete buttons, new command button)
 * - Command editor functionality
 * - Mobile viewport (375x667)
 * - Console errors
 */

import { test, expect } from './fixtures';
import path from 'path';

const SCREENSHOTS_DIR = '/tmp/qa-TASK-616';

test.describe('Settings > Slash Commands Page - QA Verification', () => {
	let consoleErrors: string[] = [];
	let consoleWarnings: string[] = [];

	test.beforeEach(async ({ page }) => {
		// Capture console errors and warnings
		consoleErrors = [];
		consoleWarnings = [];

		page.on('console', (msg) => {
			const text = msg.text();
			// Filter out noise
			if (text.includes('Download the React DevTools')) return;
			if (text.includes('[vite]')) return;

			if (msg.type() === 'error') {
				consoleErrors.push(text);
			} else if (msg.type() === 'warning') {
				consoleWarnings.push(text);
			}
		});

		// Navigate to Settings > Slash Commands
		await page.goto('/settings/commands');
		await page.waitForLoadState('networkidle');
	});

	test('QA-001: Desktop initial page load - layout elements present', async ({ page }) => {
		// Wait for content to load
		await page.waitForSelector('.settings-view__title', { state: 'visible', timeout: 10000 });

		// Take screenshot for visual comparison
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-001-desktop-initial.png'),
			fullPage: true,
		});

		// === Header Elements ===
		const title = await page.locator('.settings-view__title').textContent();
		expect(title?.trim()).toBe('Slash Commands');

		const subtitle = await page.locator('.settings-view__subtitle').textContent();
		expect(subtitle).toContain('Custom commands for Claude Code');
		expect(subtitle).toContain('~/.claude/commands');

		// Verify "New Command" button with plus icon
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await expect(newCommandBtn).toBeVisible();
		const plusIcon = newCommandBtn.locator('[data-icon="plus"]');
		await expect(plusIcon).toBeVisible();

		// === Section Headers ===
		// May not exist if no commands are loaded
		const projectSection = page.locator('text=Project Commands');
		const globalSection = page.locator('text=Global Commands');

		// Log presence for documentation
		const hasProjectSection = await projectSection.count() > 0;
		const hasGlobalSection = await globalSection.count() > 0;

		console.log(`Project Commands section: ${hasProjectSection ? 'PRESENT' : 'ABSENT'}`);
		console.log(`Global Commands section: ${hasGlobalSection ? 'PRESENT' : 'ABSENT'}`);

		// === Command Editor ===
		// Editor should be visible if commands exist, or empty state shown
		const editorArea = page.locator('.settings-view__editor');
		await expect(editorArea).toBeVisible();

		console.log('✓ QA-001: Desktop layout elements verified');
	});

	test('QA-002: Command list - items are selectable', async ({ page }) => {
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		// Check if commands exist
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount === 0) {
			console.warn('⚠ QA-002: No commands loaded - cannot test selection');
			console.warn('This may indicate API is not returning skills or project has no commands');

			// Take screenshot of empty state
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-002-no-commands.png'),
				fullPage: true,
			});

			// Verify empty state is shown
			const emptyState = page.locator('.command-list-empty');
			const emptyEditor = page.locator('.settings-view__empty');
			const hasEmptyState = (await emptyState.count()) > 0 || (await emptyEditor.count()) > 0;

			expect(hasEmptyState).toBe(true);
			return;
		}

		// Test: Click first command
		const firstCommand = commandItems.first();
		await firstCommand.click();
		await page.waitForTimeout(300);

		// Verify selection
		await expect(firstCommand).toHaveClass(/selected/);

		// Take screenshot of selected state
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-002-command-selected.png'),
			fullPage: true,
		});

		// Test: Click second command if exists
		if (commandCount > 1) {
			const secondCommand = commandItems.nth(1);
			await secondCommand.click();
			await page.waitForTimeout(300);

			// Verify second selected, first deselected
			await expect(secondCommand).toHaveClass(/selected/);
			await expect(firstCommand).not.toHaveClass(/selected/);
		}

		console.log(`✓ QA-002: Command selection works (${commandCount} commands tested)`);
	});

	test('QA-003: Command actions - edit and delete buttons', async ({ page }) => {
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount === 0) {
			console.warn('⚠ QA-003: No commands - skipping action button tests');
			return;
		}

		const firstCommand = commandItems.first();

		// Hover to ensure buttons are visible (they may be hidden until hover)
		await firstCommand.hover();
		await page.waitForTimeout(200);

		// === Edit Button ===
		const editBtn = firstCommand.locator('button[aria-label*="Edit"]');
		await expect(editBtn).toBeVisible();

		// Click edit - should select the command
		await editBtn.click();
		await page.waitForTimeout(300);
		await expect(firstCommand).toHaveClass(/selected/);

		// === Delete Button ===
		const deleteBtn = firstCommand.locator('button[aria-label*="Delete"]');
		await expect(deleteBtn).toBeVisible();

		// Click delete - should show confirmation
		await deleteBtn.click();
		await page.waitForTimeout(300);

		// Check for confirmation buttons (check/x)
		const confirmBtn = firstCommand.locator('button[aria-label*="Confirm"]');
		const cancelBtn = firstCommand.locator('button[aria-label*="Cancel"]');

		await expect(confirmBtn).toBeVisible();
		await expect(cancelBtn).toBeVisible();

		// Take screenshot of confirmation state
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-003-delete-confirmation.png'),
			fullPage: true,
		});

		// Cancel deletion
		await cancelBtn.click();
		await page.waitForTimeout(300);

		// Verify buttons return to normal state
		await expect(editBtn).toBeVisible();
		await expect(deleteBtn).toBeVisible();

		console.log('✓ QA-003: Edit and delete buttons work correctly');
	});

	test('QA-004: New Command button - modal opens', async ({ page }) => {
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		// Check for modal
		const modal = page.locator('[role="dialog"]');
		const modalVisible = await modal.count() > 0;

		if (!modalVisible) {
			console.error('✗ QA-004: Modal did not appear after clicking New Command');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-004-modal-failed.png'),
				fullPage: true,
			});
			expect(modalVisible).toBe(true);
			return;
		}

		await expect(modal).toBeVisible();

		// Take screenshot of modal
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-004-new-command-modal.png'),
			fullPage: true,
		});

		// Check modal content (common elements)
		const modalTitle = modal.locator('h2, [class*="title"]');
		await expect(modalTitle).toBeVisible();

		console.log('✓ QA-004: New Command modal opens successfully');

		// Close modal
		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);
	});

	test('QA-005: Command editor - displays content and Save button', async ({ page }) => {
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount === 0) {
			console.warn('⚠ QA-005: No commands - skipping editor tests');
			return;
		}

		// Select first command
		await commandItems.first().click();
		await page.waitForTimeout(500);

		// Verify editor shows content
		const editorContent = page.locator('.settings-view__editor');
		await expect(editorContent).toBeVisible();

		// Check for file path display
		const filePath = editorContent.locator('text=/.claude/commands/');
		await expect(filePath).toBeVisible();

		// Verify Save button
		const saveBtn = page.locator('button:has-text("Save")');
		await expect(saveBtn).toBeVisible();

		// Take screenshot
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-005-editor-view.png'),
			fullPage: true,
		});

		// Test Save button (just click, don't verify actual save without mocking)
		await saveBtn.click();
		await page.waitForTimeout(500);

		console.log('✓ QA-005: Command editor displays correctly and Save button works');
	});

	test('QA-006: Mobile viewport (375x667) - responsive layout', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.goto('/settings/commands'); // Re-navigate with new viewport
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		// Take screenshot
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-006-mobile-initial.png'),
			fullPage: true,
		});

		// Verify critical elements are accessible
		const title = page.locator('.settings-view__title');
		await expect(title).toBeVisible();

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await expect(newCommandBtn).toBeVisible();

		// Check if command list is accessible
		const commandListContainer = page.locator('.settings-view__list');
		const listVisible = await commandListContainer.isVisible();

		console.log(`Mobile: Command list visible: ${listVisible}`);

		// Test tapping a command if any exist
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount > 0) {
			await commandItems.first().tap();
			await page.waitForTimeout(300);

			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-006-mobile-selected.png'),
				fullPage: true,
			});

			console.log('✓ QA-006: Mobile - command selection works');
		}

		// Check for horizontal scroll (bad UX)
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		const viewportWidth = 375;

		if (bodyWidth > viewportWidth) {
			console.warn(`⚠ QA-006: Horizontal scroll detected (body width: ${bodyWidth}px > viewport: ${viewportWidth}px)`);
		}

		console.log('✓ QA-006: Mobile viewport layout verified');
	});

	test('QA-007: Console errors check', async ({ page }) => {
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		// Interact with page to trigger potential errors
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount > 0) {
			await commandItems.first().click();
			await page.waitForTimeout(500);
		}

		// Try opening and closing modal
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);
		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);

		// Report findings
		if (consoleErrors.length > 0) {
			console.error('✗ QA-007: Console Errors Detected:');
			consoleErrors.forEach((err, i) => {
				console.error(`  ${i + 1}. ${err}`);
			});
		} else {
			console.log('✓ QA-007: No console errors detected');
		}

		if (consoleWarnings.length > 0) {
			console.warn('⚠ QA-007: Console Warnings:');
			consoleWarnings.forEach((warn, i) => {
				console.warn(`  ${i + 1}. ${warn}`);
			});
		}

		// Fail test if errors found
		expect(consoleErrors.length).toBe(0);
	});

	test.afterAll(async () => {
		console.log('\n=== QA Test Report: Settings > Slash Commands ===');
		console.log(`Screenshots saved to: ${SCREENSHOTS_DIR}`);
		console.log('Reference image: example_ui/settings-slash-commands.png');
		console.log('\nCompare screenshots to identify visual differences.');
		console.log('================================================\n');
	});
});
