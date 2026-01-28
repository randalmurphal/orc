#!/usr/bin/env node
/**
 * E2E Test for Settings > Slash Commands Page
 * Task: TASK-616
 *
 * This script performs comprehensive E2E testing against the reference image
 * at example_ui/settings-slash-commands.png
 */

import { chromium } from 'playwright';
import { readFileSync, mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Configuration
const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = join(__dirname, 'qa-screenshots');
const FINDINGS = [];
let SCREENSHOT_COUNTER = 1;
let SCENARIOS_TESTED = 0;

// Ensure screenshot directory exists
if (!existsSync(SCREENSHOT_DIR)) {
  mkdirSync(SCREENSHOT_DIR, { recursive: true });
}

/**
 * Add a finding to the report
 */
function addFinding(severity, confidence, category, title, steps, expected, actual, screenshotPath, suggestedFix = '') {
  if (confidence >= 80) {
    const id = `QA-${String(FINDINGS.length + 1).padStart(3, '0')}`;
    FINDINGS.push({
      id,
      severity,
      confidence,
      category,
      title,
      steps_to_reproduce: steps,
      expected,
      actual,
      screenshot_path: screenshotPath || '',
      suggested_fix: suggestedFix
    });
    console.log(`\n❌ ${id}: ${title} [${severity.toUpperCase()}] (confidence: ${confidence}%)`);
  }
}

/**
 * Take a screenshot
 */
async function takeScreenshot(page, name) {
  const path = join(SCREENSHOT_DIR, `${SCREENSHOT_COUNTER}-${name}.png`);
  await page.screenshot({ path, fullPage: true });
  SCREENSHOT_COUNTER++;
  console.log(`📸 Screenshot saved: ${path}`);
  return path;
}

/**
 * Main test execution
 */
async function runTests() {
  console.log('🚀 Starting E2E Tests for Settings > Slash Commands\n');
  console.log(`Base URL: ${BASE_URL}`);
  console.log(`Screenshot directory: ${SCREENSHOT_DIR}\n`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 }
  });
  const page = await context.newPage();

  // Collect console messages
  const consoleMessages = [];
  page.on('console', msg => {
    consoleMessages.push({ type: msg.type(), text: msg.text() });
    if (msg.type() === 'error') {
      console.log(`🔴 Console Error: ${msg.text()}`);
    }
  });

  try {
    // ============================================
    // TEST 1: Navigation & Initial Load
    // ============================================
    console.log('📋 TEST 1: Navigation & Initial Load');
    SCENARIOS_TESTED++;

    await page.goto(BASE_URL, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000);

    // Navigate to Settings
    const settingsButton = page.locator('[data-testid="nav-settings"], a[href="/settings"]').first();
    if (await settingsButton.count() === 0) {
      await addFinding(
        'high',
        90,
        'functional',
        'Settings navigation not found',
        ['Navigate to ' + BASE_URL],
        'Settings link should be visible in sidebar',
        'No Settings link found with expected selectors',
        await takeScreenshot(page, 'settings-not-found')
      );
    } else {
      await settingsButton.click();
      await page.waitForTimeout(500);

      // Click Slash Commands submenu
      const slashCommandsLink = page.locator('text="Slash Commands"').first();
      if (await slashCommandsLink.count() > 0) {
        await slashCommandsLink.click();
        await page.waitForTimeout(500);
        await takeScreenshot(page, 'settings-slash-commands-loaded');
        console.log('✅ Successfully navigated to Slash Commands');
      } else {
        await addFinding(
          'high',
          90,
          'functional',
          'Slash Commands submenu not found',
          ['Navigate to /settings'],
          '"Slash Commands" submenu item should be visible',
          'Slash Commands link not found',
          await takeScreenshot(page, 'slash-commands-not-found')
        );
      }
    }

    // ============================================
    // TEST 2: Visual Structure Validation
    // ============================================
    console.log('\n📋 TEST 2: Visual Structure Validation');
    SCENARIOS_TESTED++;

    // Check page title
    const pageTitle = await page.locator('h1, h2').filter({ hasText: 'Slash Commands' }).count();
    if (pageTitle === 0) {
      await addFinding(
        'medium',
        85,
        'visual',
        'Page title "Slash Commands" not found',
        ['Navigate to Settings > Slash Commands'],
        'Should display "Slash Commands" as page title',
        'Title not found on page',
        await takeScreenshot(page, 'missing-title')
      );
    }

    // Check "+ New Command" button
    const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")');
    if (await newCommandButton.count() === 0) {
      await addFinding(
        'medium',
        85,
        'functional',
        '"+ New Command" button not found',
        ['Navigate to Settings > Slash Commands'],
        'Should display "+ New Command" button in top right',
        'Button not found on page',
        await takeScreenshot(page, 'missing-new-command-button')
      );
    }

    // Check Project Commands section
    const projectCommandsHeading = await page.locator('text="Project Commands"').count();
    if (projectCommandsHeading === 0) {
      await addFinding(
        'medium',
        85,
        'visual',
        '"Project Commands" section not found',
        ['Navigate to Settings > Slash Commands'],
        'Should display "Project Commands" section',
        'Section heading not found',
        await takeScreenshot(page, 'missing-project-commands')
      );
    }

    // Check Global Commands section
    const globalCommandsHeading = await page.locator('text="Global Commands"').count();
    if (globalCommandsHeading === 0) {
      await addFinding(
        'medium',
        85,
        'visual',
        '"Global Commands" section not found',
        ['Navigate to Settings > Slash Commands'],
        'Should display "Global Commands" section',
        'Section heading not found',
        await takeScreenshot(page, 'missing-global-commands')
      );
    }

    // Check for command cards
    const commandCards = page.locator('[data-testid*="command-card"], .command-card, [class*="CommandCard"]');
    const commandCount = await commandCards.count();
    console.log(`Found ${commandCount} command cards`);

    if (commandCount === 0) {
      await addFinding(
        'high',
        90,
        'functional',
        'No command cards displayed',
        ['Navigate to Settings > Slash Commands'],
        'Should display at least one command card',
        'No command cards found on page',
        await takeScreenshot(page, 'no-command-cards')
      );
    }

    // Check for command editor
    const commandEditor = page.locator('textarea, .monaco-editor, [class*="editor"]').first();
    if (await commandEditor.count() === 0) {
      await addFinding(
        'medium',
        85,
        'visual',
        'Command editor not visible',
        ['Navigate to Settings > Slash Commands'],
        'Should display command editor at bottom of page',
        'No editor element found',
        await takeScreenshot(page, 'missing-editor')
      );
    }

    // ============================================
    // TEST 3: Command Selection Interaction
    // ============================================
    console.log('\n📋 TEST 3: Command Selection Interaction');
    SCENARIOS_TESTED++;

    if (commandCount > 0) {
      // Click first command
      await commandCards.first().click();
      await page.waitForTimeout(500);
      const screenshot1 = await takeScreenshot(page, 'command-1-selected');

      // Check if editor content changed
      const editorContent1 = await commandEditor.inputValue().catch(() =>
        commandEditor.textContent().catch(() => '')
      );

      if (commandCount > 1) {
        // Click second command
        await commandCards.nth(1).click();
        await page.waitForTimeout(500);
        const screenshot2 = await takeScreenshot(page, 'command-2-selected');

        // Check if editor content changed
        const editorContent2 = await commandEditor.inputValue().catch(() =>
          commandEditor.textContent().catch(() => '')
        );

        if (editorContent1 === editorContent2 && editorContent1 !== '') {
          await addFinding(
            'critical',
            95,
            'functional',
            'Editor content does not update when switching commands',
            [
              'Navigate to Settings > Slash Commands',
              'Click first command card',
              'Note editor content',
              'Click second command card',
              'Check editor content'
            ],
            'Editor should display different content for each command',
            'Editor content remains the same when clicking different commands',
            screenshot2,
            'Check ConfigEditor component - may be using stale state'
          );
        }
      }
    }

    // ============================================
    // TEST 4: New Command Button
    // ============================================
    console.log('\n📋 TEST 4: New Command Button Interaction');
    SCENARIOS_TESTED++;

    if (await newCommandButton.count() > 0) {
      await newCommandButton.click();
      await page.waitForTimeout(500);

      // Check if modal/dialog opened
      const modal = page.locator('[role="dialog"], .modal, [class*="Modal"], [class*="Dialog"]');
      if (await modal.count() === 0) {
        await addFinding(
          'medium',
          85,
          'functional',
          'New Command button does not open modal/dialog',
          [
            'Navigate to Settings > Slash Commands',
            'Click "+ New Command" button'
          ],
          'Should open a modal/dialog for creating new command',
          'No modal or dialog appears after clicking button',
          await takeScreenshot(page, 'new-command-no-modal')
        );
      } else {
        await takeScreenshot(page, 'new-command-modal-open');
        console.log('✅ New Command modal opened');

        // Close modal
        const closeButton = modal.locator('button:has-text("Cancel"), button:has-text("Close"), [aria-label="Close"]');
        if (await closeButton.count() > 0) {
          await closeButton.first().click();
          await page.waitForTimeout(300);
        }
      }
    }

    // ============================================
    // TEST 5: Input Validation
    // ============================================
    console.log('\n📋 TEST 5: Input Validation');
    SCENARIOS_TESTED++;

    // Try to create command with invalid name
    if (await newCommandButton.count() > 0) {
      await newCommandButton.click();
      await page.waitForTimeout(500);

      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      if (await nameInput.count() > 0) {
        // Test with special characters
        await nameInput.fill('test/command');
        await page.waitForTimeout(300);
        const screenshot = await takeScreenshot(page, 'invalid-name-with-slash');

        // Look for validation error
        const errorMessage = await page.locator('.error, .invalid, [class*="error"]').count();
        if (errorMessage === 0) {
          await addFinding(
            'high',
            90,
            'functional',
            'No validation for special characters in command name',
            [
              'Click "+ New Command"',
              'Enter "test/command" in name field'
            ],
            'Should show validation error for special characters in command name',
            'No validation error displayed, accepts invalid characters',
            screenshot,
            'Add input validation to prevent special characters, spaces, and path traversal'
          );
        }

        // Close modal
        const closeButton = page.locator('[role="dialog"] button:has-text("Cancel"), [role="dialog"] button:has-text("Close")');
        if (await closeButton.count() > 0) {
          await closeButton.first().click();
          await page.waitForTimeout(300);
        }
      }
    }

    // ============================================
    // TEST 6: Mobile Viewport
    // ============================================
    console.log('\n📋 TEST 6: Mobile Viewport Testing');
    SCENARIOS_TESTED++;

    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);
    await takeScreenshot(page, 'mobile-viewport');

    // Check if layout is responsive
    const bodyOverflow = await page.evaluate(() => {
      return document.body.scrollWidth > window.innerWidth;
    });

    if (bodyOverflow) {
      await addFinding(
        'medium',
        85,
        'visual',
        'Horizontal scroll on mobile viewport',
        [
          'Resize browser to 375x667',
          'Check for horizontal scrolling'
        ],
        'Page should fit within mobile viewport without horizontal scroll',
        'Page has horizontal overflow on mobile',
        await takeScreenshot(page, 'mobile-horizontal-scroll')
      );
    }

    // Check if buttons are accessible on mobile
    if (await newCommandButton.count() > 0) {
      const buttonBox = await newCommandButton.first().boundingBox();
      if (buttonBox && buttonBox.width > 375) {
        await addFinding(
          'medium',
          80,
          'visual',
          'New Command button too wide for mobile',
          ['Resize to 375x667', 'Check "+ New Command" button'],
          'Button should fit within mobile viewport',
          'Button extends beyond viewport width',
          await takeScreenshot(page, 'button-too-wide')
        );
      }
    }

    console.log('✅ Mobile viewport tests complete');

    // Restore desktop viewport
    await page.setViewportSize({ width: 1920, height: 1080 });

    // ============================================
    // TEST 7: Console Errors
    // ============================================
    console.log('\n📋 TEST 7: Console Error Check');

    const errors = consoleMessages.filter(msg => msg.type === 'error');
    const warnings = consoleMessages.filter(msg => msg.type === 'warning');

    console.log(`Console Errors: ${errors.length}`);
    console.log(`Console Warnings: ${warnings.length}`);

    if (errors.length > 0) {
      const errorText = errors.map(e => e.text).join('; ');
      await addFinding(
        'medium',
        85,
        'functional',
        `${errors.length} JavaScript console error(s) detected`,
        ['Navigate to Settings > Slash Commands', 'Open browser console'],
        'No JavaScript errors in console',
        `Console shows ${errors.length} error(s): ${errorText}`,
        '',
        'Check browser console for stack traces'
      );
    }

  } catch (error) {
    console.error('\n❌ Test execution failed:', error.message);
    await takeScreenshot(page, 'test-failure');
    throw error;
  } finally {
    await browser.close();
  }

  // ============================================
  // Generate Report
  // ============================================
  console.log('\n' + '='.repeat(60));
  console.log('📊 TEST SUMMARY');
  console.log('='.repeat(60));
  console.log(`Scenarios Tested: ${SCENARIOS_TESTED}`);
  console.log(`Viewports Tested: desktop (1920x1080), mobile (375x667)`);
  console.log(`Total Findings: ${FINDINGS.length}`);
  console.log(`Screenshots Captured: ${SCREENSHOT_COUNTER - 1}`);

  if (FINDINGS.length > 0) {
    console.log('\n📋 FINDINGS:');
    FINDINGS.forEach(finding => {
      console.log(`\n${finding.id}: ${finding.title}`);
      console.log(`  Severity: ${finding.severity}`);
      console.log(`  Confidence: ${finding.confidence}%`);
      console.log(`  Category: ${finding.category}`);
    });
  }

  // Output structured result
  const result = {
    status: 'complete',
    summary: `Tested ${SCENARIOS_TESTED} scenarios across 2 viewports, found ${FINDINGS.length} issue(s)`,
    findings: FINDINGS,
    verification: {
      scenarios_tested: SCENARIOS_TESTED,
      viewports_tested: ['desktop (1920x1080)', 'mobile (375x667)'],
      previous_issues_verified: []
    }
  };

  console.log('\n' + '='.repeat(60));
  console.log('📄 STRUCTURED OUTPUT');
  console.log('='.repeat(60));
  console.log(JSON.stringify(result, null, 2));

  return result;
}

// Run tests
runTests()
  .then(result => {
    console.log('\n✅ E2E Tests Complete');
    process.exit(result.findings.length > 0 ? 1 : 0);
  })
  .catch(error => {
    console.error('\n❌ E2E Tests Failed:', error);
    process.exit(1);
  });
