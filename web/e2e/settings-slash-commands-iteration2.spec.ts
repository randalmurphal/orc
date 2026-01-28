/**
 * QA Test: Settings > Slash Commands Page - ITERATION 2
 * Task: TASK-616
 *
 * This test verifies fixes for bugs found in Iteration 1:
 * - QA-001 (CRITICAL): Editor content doesn't update when switching commands
 * - QA-002 (HIGH): No validation for forward slash (/) in command names
 * - QA-003 (HIGH): No validation for spaces in command names
 * - QA-004 (MEDIUM): No max length validation
 *
 * Also includes comprehensive edge case and mobile testing.
 */

import { test, expect } from './fixtures';
import path from 'node:path';

const SCREENSHOTS_DIR = '/tmp/qa-TASK-616-iteration2';

// Ensure screenshot directory exists
import fs from 'node:fs';
if (!fs.existsSync(SCREENSHOTS_DIR)) {
	fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
}

test.describe('Settings > Slash Commands - Bug Verification (Iteration 2)', () => {
	let consoleErrors: string[] = [];
	let consoleWarnings: string[] = [];

	test.beforeEach(async ({ page }) => {
		// Capture console messages
		consoleErrors = [];
		consoleWarnings = [];

		page.on('console', (msg) => {
			const text = msg.text();
			// Filter out noise
			if (text.includes('Download the React DevTools')) return;
			if (text.includes('[vite]')) return;
			if (text.includes('HMR')) return;

			if (msg.type() === 'error') {
				consoleErrors.push(text);
			} else if (msg.type() === 'warning') {
				consoleWarnings.push(text);
			}
		});

		// Navigate to Settings > Slash Commands
		await page.goto('/settings/commands');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('.settings-view__title', { state: 'visible', timeout: 10000 });
	});

	// ========================================================================
	// ITERATION 1 BUG VERIFICATION
	// ========================================================================

	test('QA-001 VERIFICATION: Editor content SHOULD update when switching commands', async ({ page }) => {
		// This test verifies the fix for QA-616-001 (Critical)
		// Previous behavior: Editor content stayed the same when switching commands
		// Expected behavior: Editor content updates to show the selected command's content

		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount < 2) {
			console.warn('⚠ QA-001: Need at least 2 commands to test switching - SKIPPED');
			return;
		}

		// Step 1: Select first command and note its content
		const firstCommand = commandItems.first();
		const firstCommandName = await firstCommand.locator('[class*="name"]').first().textContent();
		await firstCommand.click();
		await page.waitForTimeout(500);

		const editorContent = page.locator('.settings-view__editor textarea, .settings-view__editor [contenteditable]');
		const firstContent = await editorContent.inputValue().catch(() =>
			editorContent.textContent()
		);

		console.log(`First command: ${firstCommandName?.trim()}`);
		console.log(`First content length: ${firstContent?.length} characters`);

		// Take screenshot of first command selected
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-001-first-command-selected.png'),
			fullPage: true,
		});

		// Step 2: Select second command
		const secondCommand = commandItems.nth(1);
		const secondCommandName = await secondCommand.locator('[class*="name"]').first().textContent();
		await secondCommand.click();
		await page.waitForTimeout(500);

		const secondContent = await editorContent.inputValue().catch(() =>
			editorContent.textContent()
		);

		console.log(`Second command: ${secondCommandName?.trim()}`);
		console.log(`Second content length: ${secondContent?.length} characters`);

		// Take screenshot of second command selected
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-001-second-command-selected.png'),
			fullPage: true,
		});

		// VERIFICATION: Content should be different
		if (firstContent === secondContent) {
			console.error('✗ QA-001 FAILED: Editor content DID NOT UPDATE when switching commands');
			console.error(`  First command: ${firstCommandName?.trim()}`);
			console.error(`  Second command: ${secondCommandName?.trim()}`);
			console.error(`  Both showed same content (${firstContent?.length} chars)`);

			// Take failure screenshot
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-001-BUG-STILL-PRESENT.png'),
				fullPage: true,
			});

			expect(firstContent).not.toBe(secondContent);
		} else {
			console.log('✓ QA-001 PASSED: Editor content correctly updated when switching commands');
		}
	});

	test('QA-002 VERIFICATION: Should validate forward slash (/) in command names', async ({ page }) => {
		// This test verifies the fix for QA-616-002 (High)
		// Previous behavior: Accepted "test/command" without error
		// Expected behavior: Show validation error

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		// Wait for modal
		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible();

		// Enter command name with forward slash
		const nameInput = modal.locator('input[type="text"]').first();
		await nameInput.fill('test/command');
		await page.waitForTimeout(300);

		// Take screenshot of input with slash
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-002-slash-in-name.png'),
			fullPage: true,
		});

		// Try to create - should show validation error
		const createBtn = modal.locator('button:has-text("Create")');
		await createBtn.click();
		await page.waitForTimeout(500);

		// Check for error message
		const errorMessage = modal.locator('[class*="error"], [role="alert"]');
		const hasError = await errorMessage.count() > 0;

		// Also check if modal is still open (didn't create)
		const modalStillOpen = await modal.isVisible();

		if (!hasError && !modalStillOpen) {
			console.error('✗ QA-002 FAILED: Accepted slash (/) in command name without validation');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-002-BUG-STILL-PRESENT.png'),
				fullPage: true,
			});
			expect(hasError).toBe(true);
		} else {
			console.log('✓ QA-002 PASSED: Validation correctly rejects slash (/) in command names');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-002-FIXED-validation-error.png'),
				fullPage: true,
			});
		}

		// Close modal
		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);
	});

	test('QA-003 VERIFICATION: Should validate spaces in command names', async ({ page }) => {
		// This test verifies the fix for QA-616-002 (High)
		// Previous behavior: Accepted "test command" without error
		// Expected behavior: Show validation error

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible();

		// Enter command name with space
		const nameInput = modal.locator('input[type="text"]').first();
		await nameInput.fill('test command');
		await page.waitForTimeout(300);

		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-003-space-in-name.png'),
			fullPage: true,
		});

		// Try to create
		const createBtn = modal.locator('button:has-text("Create")');
		await createBtn.click();
		await page.waitForTimeout(500);

		const errorMessage = modal.locator('[class*="error"], [role="alert"]');
		const hasError = await errorMessage.count() > 0;
		const modalStillOpen = await modal.isVisible();

		if (!hasError && !modalStillOpen) {
			console.error('✗ QA-003 FAILED: Accepted space in command name without validation');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-003-BUG-STILL-PRESENT.png'),
				fullPage: true,
			});
			expect(hasError).toBe(true);
		} else {
			console.log('✓ QA-003 PASSED: Validation correctly rejects spaces in command names');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-003-FIXED-validation-error.png'),
				fullPage: true,
			});
		}

		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);
	});

	test('QA-004 VERIFICATION: Should validate max length (200 chars)', async ({ page }) => {
		// This test verifies the fix for QA-616-002 (High)
		// Previous behavior: Accepted 1000+ character names
		// Expected behavior: Show max length error

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible();

		// Enter very long command name (200 characters)
		const longName = 'a'.repeat(200);
		const nameInput = modal.locator('input[type="text"]').first();
		await nameInput.fill(longName);
		await page.waitForTimeout(300);

		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'qa-004-long-name.png'),
			fullPage: true,
		});

		// Try to create
		const createBtn = modal.locator('button:has-text("Create")');
		await createBtn.click();
		await page.waitForTimeout(500);

		const errorMessage = modal.locator('[class*="error"], [role="alert"]');
		const hasError = await errorMessage.count() > 0;
		const modalStillOpen = await modal.isVisible();

		if (!hasError && !modalStillOpen) {
			console.error('✗ QA-004 FAILED: Accepted 200+ character name without validation');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-004-BUG-STILL-PRESENT.png'),
				fullPage: true,
			});
			expect(hasError).toBe(true);
		} else {
			console.log('✓ QA-004 PASSED: Validation correctly rejects long command names');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'qa-004-FIXED-validation-error.png'),
				fullPage: true,
			});
		}

		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);
	});

	// ========================================================================
	// COMPREHENSIVE EDGE CASE TESTING
	// ========================================================================

	test('EDGE CASE: Special characters in command names', async ({ page }) => {
		const specialChars = ['@', '#', '$', '%', '^', '&', '*', '(', ')', '=', '+', '..', '../'];
		const results: { char: string; accepted: boolean }[] = [];

		for (const char of specialChars) {
			const newCommandBtn = page.locator('button:has-text("New Command")');
			await newCommandBtn.click();
			await page.waitForTimeout(300);

			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			const nameInput = modal.locator('input[type="text"]').first();
			await nameInput.fill(`test${char}command`);
			await page.waitForTimeout(200);

			const createBtn = modal.locator('button:has-text("Create")');
			await createBtn.click();
			await page.waitForTimeout(300);

			const errorMessage = modal.locator('[class*="error"], [role="alert"]');
			const hasError = await errorMessage.count() > 0;
			const modalStillOpen = await modal.isVisible();

			const accepted = !hasError && !modalStillOpen;
			results.push({ char, accepted });

			if (modalStillOpen) {
				await page.keyboard.press('Escape');
				await page.waitForTimeout(200);
			}
		}

		// Take summary screenshot
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'edge-special-chars-summary.png'),
			fullPage: true,
		});

		console.log('\nSpecial Character Validation Results:');
		console.table(results);

		// Validation expectation: ONLY alphanumeric, hyphen, underscore should be allowed
		const shouldReject = ['@', '#', '$', '%', '^', '&', '*', '(', ')', '=', '+', '..', '../'];
		const wronglyAccepted = results.filter(r => shouldReject.includes(r.char) && r.accepted);

		if (wronglyAccepted.length > 0) {
			console.error('✗ EDGE CASE FAILED: Accepted invalid special characters:');
			console.error(wronglyAccepted.map(r => r.char).join(', '));
		} else {
			console.log('✓ EDGE CASE PASSED: All special characters correctly validated');
		}
	});

	test('EDGE CASE: Unicode and emoji in command names', async ({ page }) => {
		const testCases = [
			{ name: 'test日本語', label: 'Japanese characters' },
			{ name: 'test🚀💻', label: 'Emoji' },
			{ name: 'testעברית', label: 'Hebrew (RTL)' },
			{ name: 'test中文', label: 'Chinese' },
		];

		for (const testCase of testCases) {
			const newCommandBtn = page.locator('button:has-text("New Command")');
			await newCommandBtn.click();
			await page.waitForTimeout(300);

			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			const nameInput = modal.locator('input[type="text"]').first();
			await nameInput.fill(testCase.name);
			await page.waitForTimeout(200);

			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, `edge-unicode-${testCase.label.replace(/\s/g, '-')}.png`),
				fullPage: true,
			});

			const createBtn = modal.locator('button:has-text("Create")');
			await createBtn.click();
			await page.waitForTimeout(300);

			const errorMessage = modal.locator('[class*="error"], [role="alert"]');
			const hasError = await errorMessage.count() > 0;
			const modalStillOpen = await modal.isVisible();

			console.log(`Unicode test "${testCase.label}": ${hasError || modalStillOpen ? 'REJECTED' : 'ACCEPTED'}`);

			if (modalStillOpen) {
				await page.keyboard.press('Escape');
				await page.waitForTimeout(200);
			}
		}
	});

	test('EDGE CASE: Empty and whitespace-only names', async ({ page }) => {
		const testCases = [
			{ value: '', label: 'Empty string' },
			{ value: '   ', label: 'Whitespace only' },
			{ value: '\t\n', label: 'Tabs and newlines' },
		];

		for (const testCase of testCases) {
			const newCommandBtn = page.locator('button:has-text("New Command")');
			await newCommandBtn.click();
			await page.waitForTimeout(300);

			const modal = page.locator('[role="dialog"]');
			await expect(modal).toBeVisible();

			const nameInput = modal.locator('input[type="text"]').first();
			await nameInput.fill(testCase.value);
			await page.waitForTimeout(200);

			const createBtn = modal.locator('button:has-text("Create")');
			await createBtn.click();
			await page.waitForTimeout(300);

			const errorMessage = modal.locator('[class*="error"], [role="alert"]');
			const hasError = await errorMessage.count() > 0;
			const modalStillOpen = await modal.isVisible();

			if (!hasError && !modalStillOpen) {
				console.error(`✗ EDGE CASE FAILED: Accepted ${testCase.label}`);
			} else {
				console.log(`✓ EDGE CASE PASSED: Rejected ${testCase.label}`);
			}

			if (modalStillOpen) {
				await page.keyboard.press('Escape');
				await page.waitForTimeout(200);
			}
		}
	});

	test('EDGE CASE: Rapid command switching', async ({ page }) => {
		// Test for race conditions when rapidly switching between commands
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount < 3) {
			console.warn('⚠ EDGE CASE: Need at least 3 commands to test rapid switching - SKIPPED');
			return;
		}

		// Rapidly click through commands
		for (let i = 0; i < Math.min(commandCount, 5); i++) {
			await commandItems.nth(i).click();
			await page.waitForTimeout(100); // Very short delay to simulate rapid clicking
		}

		await page.waitForTimeout(500);

		// Take screenshot after rapid switching
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'edge-rapid-switching.png'),
			fullPage: true,
		});

		// Verify no console errors occurred
		if (consoleErrors.length > 0) {
			console.error('✗ EDGE CASE FAILED: Rapid switching caused console errors:');
			consoleErrors.forEach(err => console.error(`  - ${err}`));
		} else {
			console.log('✓ EDGE CASE PASSED: No errors during rapid command switching');
		}
	});

	test('EDGE CASE: Double-click submit button', async ({ page }) => {
		// Test for duplicate submissions
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);

		const modal = page.locator('[role="dialog"]');
		await expect(modal).toBeVisible();

		const nameInput = modal.locator('input[type="text"]').first();
		await nameInput.fill('valid-test-command');
		await page.waitForTimeout(200);

		// Get initial command count
		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);

		const initialCount = await page.locator('.command-item').count();

		// Re-open modal
		await newCommandBtn.click();
		await page.waitForTimeout(300);
		await modal.waitFor({ state: 'visible' });

		const nameInput2 = modal.locator('input[type="text"]').first();
		await nameInput2.fill('valid-test-command-2');

		// Double-click create button
		const createBtn = modal.locator('button:has-text("Create")');
		await createBtn.dblclick();
		await page.waitForTimeout(1000);

		// Check command count - should only increase by 1, not 2
		const finalCount = await page.locator('.command-item').count();
		const created = finalCount - initialCount;

		if (created > 1) {
			console.error('✗ EDGE CASE FAILED: Double-click created multiple commands');
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'edge-double-click-BUG.png'),
				fullPage: true,
			});
		} else {
			console.log('✓ EDGE CASE PASSED: Double-click protection works');
		}
	});

	// ========================================================================
	// MOBILE VIEWPORT TESTING
	// ========================================================================

	test('MOBILE: 375x667 viewport - layout and usability', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.goto('/settings/commands');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		// Take initial screenshot
		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'mobile-initial.png'),
			fullPage: true,
		});

		// Check for horizontal scroll (bad UX)
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		const viewportWidth = 375;

		if (bodyWidth > viewportWidth) {
			console.error(`✗ MOBILE FAILED: Horizontal scroll detected (${bodyWidth}px > ${viewportWidth}px)`);
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'mobile-horizontal-scroll-BUG.png'),
				fullPage: true,
			});
		} else {
			console.log('✓ MOBILE PASSED: No horizontal scroll');
		}

		// Verify critical elements are visible and tappable
		const title = page.locator('.settings-view__title');
		await expect(title).toBeVisible();

		const newCommandBtn = page.locator('button:has-text("New Command")');
		await expect(newCommandBtn).toBeVisible();

		// Check button size (should be at least 44x44 for touch)
		const btnBox = await newCommandBtn.boundingBox();
		if (btnBox && (btnBox.height < 44 || btnBox.width < 44)) {
			console.warn(`⚠ MOBILE WARNING: New Command button too small (${btnBox.width}x${btnBox.height}px)`);
		}

		// Test tapping a command
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount > 0) {
			await commandItems.first().tap();
			await page.waitForTimeout(500);

			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'mobile-command-selected.png'),
				fullPage: true,
			});

			// Check if editor is visible on mobile
			const editor = page.locator('.settings-view__editor');
			const editorVisible = await editor.isVisible();

			if (editorVisible) {
				console.log('✓ MOBILE: Editor visible after selecting command');
			} else {
				console.warn('⚠ MOBILE: Editor not visible on mobile - may need to scroll');
			}
		}

		// Test opening modal on mobile
		await newCommandBtn.tap();
		await page.waitForTimeout(500);

		const modal = page.locator('[role="dialog"]');
		const modalVisible = await modal.isVisible();

		if (modalVisible) {
			await page.screenshot({
				path: path.join(SCREENSHOTS_DIR, 'mobile-modal-open.png'),
				fullPage: true,
			});

			// Check modal doesn't overflow viewport
			const modalBox = await modal.boundingBox();
			if (modalBox && modalBox.width > viewportWidth) {
				console.error('✗ MOBILE FAILED: Modal wider than viewport');
			} else {
				console.log('✓ MOBILE PASSED: Modal fits viewport');
			}

			await page.keyboard.press('Escape');
		}
	});

	// ========================================================================
	// ERROR HANDLING AND STATE MANAGEMENT
	// ========================================================================

	test('STATE: Browser refresh during edit', async ({ page }) => {
		// Test what happens if user refreshes while editing
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount === 0) {
			console.warn('⚠ STATE TEST: No commands - SKIPPED');
			return;
		}

		// Select a command and edit content
		await commandItems.first().click();
		await page.waitForTimeout(500);

		const editor = page.locator('.settings-view__editor textarea, .settings-view__editor [contenteditable]');

		// Type some text
		await editor.focus();
		await page.keyboard.type('\n\n# TEST EDIT - DO NOT SAVE\nThis is a test edit.');
		await page.waitForTimeout(300);

		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'state-before-refresh.png'),
			fullPage: true,
		});

		// Refresh the page
		await page.reload();
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('.settings-view__title', { state: 'visible' });

		await page.screenshot({
			path: path.join(SCREENSHOTS_DIR, 'state-after-refresh.png'),
			fullPage: true,
		});

		// Verify: Changes should be lost (expected behavior for unsaved edits)
		// But page should still be functional
		await expect(page.locator('.settings-view__title')).toBeVisible();

		console.log('✓ STATE: Page recovers gracefully after refresh with unsaved changes');
	});

	test('STATE: Navigation without saving', async ({ page }) => {
		// Test navigating away from settings without saving
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount === 0) {
			console.warn('⚠ STATE TEST: No commands - SKIPPED');
			return;
		}

		// Select a command and edit
		await commandItems.first().click();
		await page.waitForTimeout(500);

		const editor = page.locator('.settings-view__editor textarea, .settings-view__editor [contenteditable]');
		await editor.focus();
		await page.keyboard.type('\n# UNSAVED EDIT TEST');
		await page.waitForTimeout(300);

		// Navigate to a different settings page
		const claudemdLink = page.locator('text=CLAUDE.md').first();
		if (await claudemdLink.count() > 0) {
			await claudemdLink.click();
			await page.waitForTimeout(500);

			// Check if warning dialog appeared
			const dialog = page.locator('[role="dialog"], [role="alertdialog"]');
			const hasWarning = await dialog.count() > 0;

			if (hasWarning) {
				console.log('✓ STATE: Warning shown when navigating with unsaved changes');
				await page.screenshot({
					path: path.join(SCREENSHOTS_DIR, 'state-navigation-warning.png'),
					fullPage: true,
				});
			} else {
				console.warn('⚠ STATE: No warning when navigating with unsaved changes');
			}
		}
	});

	// ========================================================================
	// CONSOLE ERROR CHECK
	// ========================================================================

	test('CONSOLE: No JavaScript errors during normal use', async ({ page }) => {
		// Interact with page to trigger potential errors
		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		if (commandCount > 0) {
			// Click a few commands
			await commandItems.first().click();
			await page.waitForTimeout(500);

			if (commandCount > 1) {
				await commandItems.nth(1).click();
				await page.waitForTimeout(500);
			}
		}

		// Open and close modal
		const newCommandBtn = page.locator('button:has-text("New Command")');
		await newCommandBtn.click();
		await page.waitForTimeout(500);
		await page.keyboard.press('Escape');
		await page.waitForTimeout(300);

		// Try an edit action if possible
		if (commandCount > 0) {
			const editBtn = commandItems.first().locator('button[aria-label*="Edit"]');
			if (await editBtn.count() > 0) {
				await editBtn.click();
				await page.waitForTimeout(300);
			}
		}

		// Report console findings
		console.log('\n=== Console Error Report ===');

		if (consoleErrors.length > 0) {
			console.error('✗ CONSOLE ERRORS DETECTED:');
			consoleErrors.forEach((err, i) => {
				console.error(`  ${i + 1}. ${err}`);
			});
		} else {
			console.log('✓ No console errors');
		}

		if (consoleWarnings.length > 0) {
			console.warn('⚠ CONSOLE WARNINGS:');
			consoleWarnings.forEach((warn, i) => {
				console.warn(`  ${i + 1}. ${warn}`);
			});
		} else {
			console.log('✓ No console warnings');
		}

		console.log('=============================\n');

		// Fail test if errors found
		expect(consoleErrors.length).toBe(0);
	});

	test.afterAll(async () => {
		console.log('\n╔═══════════════════════════════════════════════════════════╗');
		console.log('║ QA Test Report: Settings > Slash Commands - Iteration 2  ║');
		console.log('╚═══════════════════════════════════════════════════════════╝');
		console.log(`\n📸 Screenshots: ${SCREENSHOTS_DIR}`);
		console.log(`🎯 Reference: example_ui/settings-slash-commands.png`);
		console.log('\n📋 Bug Verification Tests:');
		console.log('   - QA-001: Editor content updates when switching commands');
		console.log('   - QA-002: Validates forward slash (/) in names');
		console.log('   - QA-003: Validates spaces in names');
		console.log('   - QA-004: Validates max length');
		console.log('\n🔬 Edge Cases:');
		console.log('   - Special characters validation');
		console.log('   - Unicode and emoji handling');
		console.log('   - Empty/whitespace validation');
		console.log('   - Rapid command switching');
		console.log('   - Double-click protection');
		console.log('\n📱 Mobile Testing:');
		console.log('   - 375x667 viewport layout');
		console.log('   - Touch target sizes');
		console.log('   - Modal responsiveness');
		console.log('\n🔄 State Management:');
		console.log('   - Browser refresh behavior');
		console.log('   - Navigation without saving');
		console.log('\n🐛 Console Monitoring:');
		console.log('   - JavaScript errors');
		console.log('   - Warnings');
		console.log('\n═══════════════════════════════════════════════════════════\n');
	});
});
