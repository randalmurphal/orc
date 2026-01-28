#!/usr/bin/env node
/**
 * Standalone QA Test Script for Settings > Slash Commands
 * TASK-616
 *
 * Usage: node qa-test-slash-commands.mjs
 *
 * Requirements:
 * - Dev server running at http://localhost:5173
 * - API server running at http://localhost:8080
 * - Playwright installed (npm i -D @playwright/test)
 */

import { chromium } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const BASE_URL = 'http://localhost:5173';
const SCREENSHOTS_DIR = '/tmp/qa-TASK-616';
const REFERENCE_IMAGE = 'example_ui/settings-slash-commands.png';

// Ensure screenshots directory exists
if (!fs.existsSync(SCREENSHOTS_DIR)) {
	fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
}

// QA Findings
const findings = [];
let findingId = 1;

function addFinding(severity, title, details) {
	const confidence = details.confidence || 85;
	const finding = {
		id: `QA-${String(findingId++).padStart(3, '0')}`,
		severity,
		confidence,
		category: 'functional',
		title,
		...details,
	};
	findings.push(finding);
	return finding;
}

function logFinding(finding) {
	const emoji = {
		critical: '🔴',
		high: '🟠',
		medium: '🟡',
		low: '🔵'
	}[finding.severity] || '⚪';

	console.log(`\n${emoji} ${finding.id}: ${finding.title}`);
	console.log(`   Severity: ${finding.severity} | Confidence: ${finding.confidence}%`);
	if (finding.screenshot_path) {
		console.log(`   Screenshot: ${finding.screenshot_path}`);
	}
}

async function runTest() {
	console.log('=== QA Test: Settings > Slash Commands ===\n');
	console.log(`Base URL: ${BASE_URL}`);
	console.log(`Screenshots: ${SCREENSHOTS_DIR}\n`);

	const browser = await chromium.launch({ headless: true });
	const context = await browser.newContext();
	const page = await context.newPage();

	// Console monitoring
	const consoleErrors = [];
	const consoleWarnings = [];

	page.on('console', (msg) => {
		const text = msg.text();
		if (text.includes('Download the React DevTools')) return;
		if (text.includes('[vite]')) return;

		if (msg.type() === 'error') {
			consoleErrors.push(text);
		} else if (msg.type() === 'warning') {
			consoleWarnings.push(text);
		}
	});

	try {
		// ===== TEST 1: Navigate and Load Page =====
		console.log('Test 1: Navigate to /settings/commands...');
		await page.goto(`${BASE_URL}/settings/commands`, { waitUntil: 'networkidle', timeout: 10000 });

		// Take initial screenshot
		const initialScreenshot = path.join(SCREENSHOTS_DIR, 'desktop-initial.png');
		await page.screenshot({ path: initialScreenshot, fullPage: true });
		console.log(`✓ Screenshot saved: ${initialScreenshot}`);

		// ===== TEST 2: Verify Page Structure =====
		console.log('\nTest 2: Verifying page structure...');

		// Check title
		const title = await page.locator('.settings-view__title').textContent({ timeout: 5000 }).catch(() => null);
		if (!title || !title.includes('Slash Commands')) {
			addFinding('high', 'Page title missing or incorrect', {
				expected: 'Slash Commands',
				actual: title || 'Not found',
				confidence: 95,
				screenshot_path: initialScreenshot,
				steps_to_reproduce: [
					'Navigate to http://localhost:5173/settings/commands',
					'Observe page title'
				]
			});
		}

		// Check subtitle
		const subtitle = await page.locator('.settings-view__subtitle').textContent().catch(() => null);
		if (!subtitle || !subtitle.includes('~/.claude/commands')) {
			addFinding('medium', 'Page subtitle missing or incorrect', {
				expected: 'Custom commands for Claude Code (~/.claude/commands)',
				actual: subtitle || 'Not found',
				confidence: 90,
				screenshot_path: initialScreenshot
			});
		}

		// Check New Command button
		const newCommandBtn = page.locator('button:has-text("New Command")');
		const btnVisible = await newCommandBtn.isVisible().catch(() => false);
		if (!btnVisible) {
			addFinding('high', 'New Command button not visible', {
				expected: 'Button with text "New Command" should be visible',
				actual: 'Button not found',
				confidence: 95,
				screenshot_path: initialScreenshot,
				steps_to_reproduce: [
					'Navigate to /settings/commands',
					'Look for "New Command" button in header'
				]
			});
		}

		// ===== TEST 3: Check Commands =====
		console.log('\nTest 3: Checking command list...');

		const commandItems = page.locator('.command-item');
		const commandCount = await commandItems.count();

		console.log(`Found ${commandCount} command(s)`);

		if (commandCount === 0) {
			console.warn('⚠ No commands found - this may be expected if API returns empty');

			// Check for empty state
			const emptyState = await page.locator('.command-list-empty').count();
			if (emptyState === 0) {
				addFinding('medium', 'No empty state shown when no commands exist', {
					expected: 'Empty state with message like "No commands"',
					actual: 'No empty state element found',
					confidence: 80,
					screenshot_path: initialScreenshot
				});
			}
		} else {
			// Test command selection
			console.log('\nTest 4: Testing command selection...');
			const firstCommand = commandItems.first();
			await firstCommand.click();
			await page.waitForTimeout(300);

			const selectionScreenshot = path.join(SCREENSHOTS_DIR, 'command-selected.png');
			await page.screenshot({ path: selectionScreenshot, fullPage: true });

			const hasSelectedClass = await firstCommand.evaluate((el) => el.classList.contains('selected'));
			if (!hasSelectedClass) {
				addFinding('high', 'Command selection does not apply "selected" class', {
					expected: 'Clicked command should have "selected" class',
					actual: 'No "selected" class found',
					confidence: 90,
					screenshot_path: selectionScreenshot,
					steps_to_reproduce: [
						'Navigate to /settings/commands',
						'Click on any command in the list',
						'Observe: command does not highlight/select'
					]
				});
			}

			// Test edit/delete buttons
			console.log('\nTest 5: Testing action buttons...');
			await firstCommand.hover();
			await page.waitForTimeout(200);

			const editBtn = firstCommand.locator('button[aria-label*="Edit"]');
			const deleteBtn = firstCommand.locator('button[aria-label*="Delete"]');

			const editVisible = await editBtn.isVisible().catch(() => false);
			const deleteVisible = await deleteBtn.isVisible().catch(() => false);

			if (!editVisible) {
				addFinding('medium', 'Edit button not visible on command item', {
					expected: 'Edit button should be visible on command card',
					actual: 'Edit button not found',
					confidence: 85,
					screenshot_path: selectionScreenshot
				});
			}

			if (!deleteVisible) {
				addFinding('medium', 'Delete button not visible on command item', {
					expected: 'Delete button should be visible on command card',
					actual: 'Delete button not found',
					confidence: 85,
					screenshot_path: selectionScreenshot
				});
			}

			if (deleteVisible) {
				// Test delete confirmation
				await deleteBtn.click();
				await page.waitForTimeout(300);

				const deleteConfirmScreenshot = path.join(SCREENSHOTS_DIR, 'delete-confirmation.png');
				await page.screenshot({ path: deleteConfirmScreenshot, fullPage: true });

				const confirmBtn = firstCommand.locator('button[aria-label*="Confirm"]');
				const cancelBtn = firstCommand.locator('button[aria-label*="Cancel"]');

				const confirmVisible = await confirmBtn.isVisible().catch(() => false);
				const cancelVisible = await cancelBtn.isVisible().catch(() => false);

				if (!confirmVisible || !cancelVisible) {
					addFinding('high', 'Delete confirmation buttons not shown', {
						expected: 'After clicking delete, confirm and cancel buttons should appear',
						actual: `Confirm: ${confirmVisible}, Cancel: ${cancelVisible}`,
						confidence: 90,
						screenshot_path: deleteConfirmScreenshot,
						steps_to_reproduce: [
							'Navigate to /settings/commands',
							'Click delete button on a command',
							'Observe: no confirmation UI appears'
						]
					});
				} else {
					// Cancel to restore state
					await cancelBtn.click();
					await page.waitForTimeout(300);
				}
			}

			// Test editor
			console.log('\nTest 6: Testing command editor...');
			const editorArea = page.locator('.settings-view__editor');
			const editorVisible = await editorArea.isVisible();

			if (!editorVisible) {
				addFinding('high', 'Command editor not visible when command is selected', {
					expected: 'Editor should show command content when command is selected',
					actual: 'Editor area not visible',
					confidence: 90,
					screenshot_path: selectionScreenshot
				});
			} else {
				const saveBtn = page.locator('button:has-text("Save")');
				const saveVisible = await saveBtn.isVisible().catch(() => false);

				if (!saveVisible) {
					addFinding('medium', 'Save button not visible in editor', {
						expected: 'Save button should be visible in command editor',
						actual: 'Save button not found',
						confidence: 85,
						screenshot_path: selectionScreenshot
					});
				}
			}
		}

		// ===== TEST 7: New Command Modal =====
		console.log('\nTest 7: Testing New Command modal...');
		if (btnVisible) {
			await newCommandBtn.click();
			await page.waitForTimeout(500);

			const modal = page.locator('[role="dialog"]');
			const modalVisible = await modal.isVisible().catch(() => false);

			const modalScreenshot = path.join(SCREENSHOTS_DIR, 'new-command-modal.png');
			await page.screenshot({ path: modalScreenshot, fullPage: true });

			if (!modalVisible) {
				addFinding('critical', 'New Command modal does not open', {
					expected: 'Clicking "New Command" should open a modal dialog',
					actual: 'No modal with role="dialog" found',
					confidence: 95,
					screenshot_path: modalScreenshot,
					steps_to_reproduce: [
						'Navigate to /settings/commands',
						'Click "New Command" button',
						'Observe: modal does not appear'
					],
					suggested_fix: 'Check NewCommandModal component and its visibility state'
				});
			} else {
				// Close modal
				await page.keyboard.press('Escape');
				await page.waitForTimeout(300);
			}
		}

		// ===== TEST 8: Mobile Viewport =====
		console.log('\nTest 8: Testing mobile viewport (375x667)...');
		await page.setViewportSize({ width: 375, height: 667 });
		await page.reload({ waitUntil: 'networkidle' });

		const mobileScreenshot = path.join(SCREENSHOTS_DIR, 'mobile-initial.png');
		await page.screenshot({ path: mobileScreenshot, fullPage: true });

		// Check for horizontal scroll
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		if (bodyWidth > 375) {
			addFinding('medium', 'Horizontal scroll on mobile viewport', {
				expected: 'Content should fit within 375px viewport width',
				actual: `Body width: ${bodyWidth}px (exceeds viewport)`,
				confidence: 95,
				screenshot_path: mobileScreenshot,
				steps_to_reproduce: [
					'Navigate to /settings/commands',
					'Resize viewport to 375x667',
					'Observe: horizontal scrollbar appears'
				]
			});
		}

		// Check if critical elements are still accessible
		const titleVisibleMobile = await page.locator('.settings-view__title').isVisible();
		const btnVisibleMobile = await page.locator('button:has-text("New Command")').isVisible();

		if (!titleVisibleMobile) {
			addFinding('high', 'Page title not visible on mobile', {
				expected: 'Title should be visible on mobile viewport',
				actual: 'Title element not visible',
				confidence: 90,
				screenshot_path: mobileScreenshot
			});
		}

		if (!btnVisibleMobile) {
			addFinding('high', 'New Command button not visible on mobile', {
				expected: 'Button should be accessible on mobile',
				actual: 'Button not visible on mobile viewport',
				confidence: 90,
				screenshot_path: mobileScreenshot
			});
		}

		// ===== TEST 9: Console Errors =====
		console.log('\nTest 9: Checking console errors...');
		if (consoleErrors.length > 0) {
			consoleErrors.forEach((error, i) => {
				addFinding('high', `Console error detected: ${error.substring(0, 100)}`, {
					expected: 'No console errors during normal operation',
					actual: error,
					confidence: 95,
					steps_to_reproduce: [
						'Open browser console',
						'Navigate to /settings/commands',
						'Interact with page',
						'Observe console error'
					]
				});
			});
		}

		if (consoleWarnings.length > 0) {
			console.warn(`⚠ ${consoleWarnings.length} console warning(s) detected (not reported as findings)`);
		}

	} catch (error) {
		console.error('\n❌ Test execution failed:', error.message);
		addFinding('critical', 'Test execution failed', {
			expected: 'Tests should run to completion',
			actual: error.message,
			confidence: 100,
			steps_to_reproduce: [
				'Ensure dev server is running at http://localhost:5173',
				'Ensure API server is running at http://localhost:8080',
				'Run: node qa-test-slash-commands.mjs'
			]
		});
	} finally {
		await browser.close();
	}

	// ===== REPORT FINDINGS =====
	console.log('\n\n=== QA FINDINGS REPORT ===\n');

	if (findings.length === 0) {
		console.log('✓ No issues found! Page matches reference expectations.');
	} else {
		console.log(`Found ${findings.length} issue(s) (confidence >= 80):\n`);

		// Sort by severity
		const severityOrder = { critical: 0, high: 1, medium: 2, low: 3 };
		findings.sort((a, b) => severityOrder[a.severity] - severityOrder[b.severity]);

		findings.forEach((finding) => {
			logFinding(finding);
		});

		// Write JSON report
		const reportPath = path.join(SCREENSHOTS_DIR, 'qa-report.json');
		fs.writeFileSync(reportPath, JSON.stringify(findings, null, 2));
		console.log(`\n📄 Full report: ${reportPath}`);
	}

	console.log('\n=== END REPORT ===\n');
	console.log(`Screenshots saved to: ${SCREENSHOTS_DIR}`);
	console.log(`Reference image: ${REFERENCE_IMAGE}`);

	// Exit with error code if critical/high issues found
	const criticalOrHigh = findings.filter((f) => f.severity === 'critical' || f.severity === 'high');
	if (criticalOrHigh.length > 0) {
		process.exit(1);
	}
}

// Run the test
runTest().catch((err) => {
	console.error('Fatal error:', err);
	process.exit(1);
});
