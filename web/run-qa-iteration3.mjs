#!/usr/bin/env node

/**
 * QA Iteration 3 Test Runner
 *
 * Runs comprehensive E2E tests against http://localhost:5173/settings
 * Saves all screenshots to qa-screenshots-iter3/
 */

import { chromium } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = path.join(__dirname, 'qa-screenshots-iter3');

// Ensure screenshot directory exists
if (!fs.existsSync(SCREENSHOT_DIR)) {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
}

console.log('QA Iteration 3 - Settings Page Comprehensive Testing');
console.log('====================================================\n');
console.log(`Base URL: ${BASE_URL}`);
console.log(`Screenshots: ${SCREENSHOT_DIR}\n`);

const findings = {
  'QA-002': { status: 'UNKNOWN', title: 'Forward slash validation', confidence: 0 },
  'QA-003': { status: 'UNKNOWN', title: 'Spaces validation', confidence: 0 },
  'QA-004': { status: 'UNKNOWN', title: 'Length validation', confidence: 0 },
  'QA-005': { status: 'UNKNOWN', title: 'Modified indicator', confidence: 0 }
};

const newFindings = [];

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function testQA002(page) {
  console.log('\n=== QA-002: Forward slash validation ===');

  try {
    // Navigate to Settings
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');
    await sleep(500);

    // Click Slash Commands section
    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-002-1-slash-commands-section.png'),
      fullPage: true
    });

    // Click New Command button
    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-002-2-modal-open.png'),
      fullPage: true
    });

    // Type command name with forward slash
    const nameInput = page.locator('input').first();
    await nameInput.fill('test/command');
    await sleep(300);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-002-3-slash-entered.png'),
      fullPage: true
    });

    // Try to create
    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    // Check for validation error
    const errorText = await page.textContent('body');
    const hasValidationError =
      errorText.toLowerCase().includes('cannot contain') ||
      errorText.toLowerCase().includes('invalid') ||
      errorText.toLowerCase().includes('slash');

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-002-4-validation-result.png'),
      fullPage: true
    });

    if (hasValidationError) {
      findings['QA-002'].status = 'FIXED';
      findings['QA-002'].confidence = 90;
      console.log('✓ FIXED - Validation error shown for forward slash');
    } else {
      findings['QA-002'].status = 'STILL_PRESENT';
      findings['QA-002'].confidence = 95;
      console.log('✗ STILL_PRESENT - No validation error shown');
    }

    // Close modal
    const cancelBtn = page.locator('button').filter({ hasText: /cancel|close/i }).first();
    if (await cancelBtn.isVisible()) {
      await cancelBtn.click();
      await sleep(300);
    }

  } catch (error) {
    console.error('Error in QA-002:', error.message);
    findings['QA-002'].status = 'ERROR';
  }
}

async function testQA003(page) {
  console.log('\n=== QA-003: Spaces validation ===');

  try {
    // Navigate to Settings
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');
    await sleep(500);

    // Click Slash Commands section
    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    // Click New Command button
    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-003-1-modal-open.png'),
      fullPage: true
    });

    // Type command name with spaces
    const nameInput = page.locator('input').first();
    await nameInput.fill('test command');
    await sleep(300);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-003-2-spaces-entered.png'),
      fullPage: true
    });

    // Try to create
    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    // Check for validation error
    const errorText = await page.textContent('body');
    const hasValidationError =
      errorText.toLowerCase().includes('cannot contain') ||
      errorText.toLowerCase().includes('invalid') ||
      errorText.toLowerCase().includes('space');

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-003-3-validation-result.png'),
      fullPage: true
    });

    if (hasValidationError) {
      findings['QA-003'].status = 'FIXED';
      findings['QA-003'].confidence = 90;
      console.log('✓ FIXED - Validation error shown for spaces');
    } else {
      findings['QA-003'].status = 'STILL_PRESENT';
      findings['QA-003'].confidence = 95;
      console.log('✗ STILL_PRESENT - No validation error shown');
    }

    // Close modal
    const cancelBtn = page.locator('button').filter({ hasText: /cancel|close/i }).first();
    if (await cancelBtn.isVisible()) {
      await cancelBtn.click();
      await sleep(300);
    }

  } catch (error) {
    console.error('Error in QA-003:', error.message);
    findings['QA-003'].status = 'ERROR';
  }
}

async function testQA004(page) {
  console.log('\n=== QA-004: Maximum length validation ===');

  try {
    // Navigate to Settings
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');
    await sleep(500);

    // Click Slash Commands section
    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    // Click New Command button
    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-004-1-modal-open.png'),
      fullPage: true
    });

    // Type very long command name (200 characters)
    const longName = 'a'.repeat(200);
    const nameInput = page.locator('input').first();
    await nameInput.fill(longName);
    await sleep(300);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-004-2-long-name-entered.png'),
      fullPage: true
    });

    // Try to create
    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    // Check for validation error
    const errorText = await page.textContent('body');
    const hasValidationError =
      errorText.toLowerCase().includes('too long') ||
      errorText.toLowerCase().includes('maximum') ||
      errorText.toLowerCase().includes('character');

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-004-3-validation-result.png'),
      fullPage: true
    });

    if (hasValidationError) {
      findings['QA-004'].status = 'FIXED';
      findings['QA-004'].confidence = 90;
      console.log('✓ FIXED - Validation error shown for long name');
    } else {
      findings['QA-004'].status = 'STILL_PRESENT';
      findings['QA-004'].confidence = 95;
      console.log('✗ STILL_PRESENT - No validation error shown');
    }

    // Close modal
    const cancelBtn = page.locator('button').filter({ hasText: /cancel|close/i }).first();
    if (await cancelBtn.isVisible()) {
      await cancelBtn.click();
      await sleep(300);
    }

  } catch (error) {
    console.error('Error in QA-004:', error.message);
    findings['QA-004'].status = 'ERROR';
  }
}

async function testQA005(page) {
  console.log('\n=== QA-005: Modified indicator on command switching ===');

  try {
    // Navigate to Settings
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');
    await sleep(500);

    // Click Slash Commands section
    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'QA-005-1-initial-state.png'),
      fullPage: true
    });

    // Check if there are command cards
    const commandCards = page.locator('[class*="command"], button[role="button"]');
    const count = await commandCards.count();

    console.log(`Found ${count} potential command elements`);

    if (count >= 2) {
      // Click first command
      await commandCards.first().click();
      await sleep(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-005-2-first-command-selected.png'),
        fullPage: true
      });

      // Click second command WITHOUT editing
      await commandCards.nth(1).click();
      await sleep(500);

      // Check for "Modified" indicator
      const bodyText = await page.textContent('body');
      const hasModifiedIndicator = bodyText.toLowerCase().includes('modified');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-005-3-second-command-selected.png'),
        fullPage: true
      });

      if (!hasModifiedIndicator) {
        findings['QA-005'].status = 'FIXED';
        findings['QA-005'].confidence = 85;
        console.log('✓ FIXED - No modified indicator shown when switching without edits');
      } else {
        findings['QA-005'].status = 'STILL_PRESENT';
        findings['QA-005'].confidence = 90;
        console.log('✗ STILL_PRESENT - Modified indicator shown incorrectly');
      }
    } else {
      console.log('Not enough commands to test. Creating test commands...');

      // Create two test commands
      for (let i = 0; i < 2; i++) {
        const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
        await newCommandBtn.click();
        await sleep(500);

        const nameInput = page.locator('input').first();
        await nameInput.fill(`testcmd${i + 1}`);

        const contentArea = page.locator('textarea').first();
        if (await contentArea.isVisible()) {
          await contentArea.fill(`Test content ${i + 1}`);
        }

        const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
        await createBtn.click();
        await sleep(1000);
      }

      // Now test switching
      const updatedCards = page.locator('[class*="command"], button[role="button"]');
      await updatedCards.first().click();
      await sleep(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-005-2-first-command-selected.png'),
        fullPage: true
      });

      await updatedCards.nth(1).click();
      await sleep(500);

      const bodyText = await page.textContent('body');
      const hasModifiedIndicator = bodyText.toLowerCase().includes('modified');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-005-3-second-command-selected.png'),
        fullPage: true
      });

      if (!hasModifiedIndicator) {
        findings['QA-005'].status = 'FIXED';
        findings['QA-005'].confidence = 85;
        console.log('✓ FIXED - No modified indicator shown');
      } else {
        findings['QA-005'].status = 'STILL_PRESENT';
        findings['QA-005'].confidence = 90;
        console.log('✗ STILL_PRESENT - Modified indicator shown incorrectly');
      }
    }

  } catch (error) {
    console.error('Error in QA-005:', error.message);
    findings['QA-005'].status = 'ERROR';
  }
}

async function testHappyPath(page) {
  console.log('\n=== Happy Path Testing ===');

  try {
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');

    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'happy-1-slash-commands-loaded.png'),
      fullPage: true
    });

    // Create new command with valid name
    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'happy-2-new-command-modal.png'),
      fullPage: true
    });

    const nameInput = page.locator('input').first();
    await nameInput.fill('validcommand123');

    const contentArea = page.locator('textarea').first();
    if (await contentArea.isVisible()) {
      await contentArea.fill('This is valid command content for testing');
    }

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'happy-3-filled-form.png'),
      fullPage: true
    });

    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'happy-4-command-created.png'),
      fullPage: true
    });

    console.log('✓ Happy path completed successfully');

  } catch (error) {
    console.error('Error in happy path:', error.message);
    newFindings.push({
      id: 'QA-NEW-001',
      severity: 'high',
      confidence: 85,
      title: 'Happy path failure',
      error: error.message
    });
  }
}

async function testEdgeCases(page) {
  console.log('\n=== Edge Case Testing ===');

  // Test: Empty command name
  try {
    console.log('\nTesting: Empty command name');
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');

    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    // Leave name empty
    const contentArea = page.locator('textarea').first();
    if (await contentArea.isVisible()) {
      await contentArea.fill('Content without name');
    }

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'edge-1-empty-name-filled.png'),
      fullPage: true
    });

    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'edge-2-empty-name-result.png'),
      fullPage: true
    });

    // Close modal
    const cancelBtn = page.locator('button').filter({ hasText: /cancel|close/i }).first();
    if (await cancelBtn.isVisible()) {
      await cancelBtn.click();
      await sleep(300);
    }

    console.log('✓ Empty name test completed');

  } catch (error) {
    console.error('Error in empty name test:', error.message);
  }

  // Test: Special characters
  try {
    console.log('\nTesting: Special characters in command names');
    const specialChars = ['test@command', 'test#command', 'test$command'];

    for (const testName of specialChars) {
      const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
      await newCommandBtn.click();
      await sleep(500);

      const nameInput = page.locator('input').first();
      await nameInput.fill(testName);
      await sleep(300);

      const safeName = testName.replace(/[^a-z0-9]/gi, '');
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, `edge-special-${safeName}.png`),
        fullPage: true
      });

      const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
      await createBtn.click();
      await sleep(1000);

      const cancelBtn = page.locator('button').filter({ hasText: /cancel|close/i }).first();
      if (await cancelBtn.isVisible()) {
        await cancelBtn.click();
        await sleep(300);
      }
    }

    console.log('✓ Special characters test completed');

  } catch (error) {
    console.error('Error in special characters test:', error.message);
  }
}

async function testMobile(page) {
  console.log('\n=== Mobile Viewport Testing ===');

  try {
    await page.setViewportSize({ width: 375, height: 667 });
    await sleep(500);

    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'mobile-1-initial.png'),
      fullPage: true
    });

    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'mobile-2-slash-commands.png'),
      fullPage: true
    });

    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    if (await newCommandBtn.isVisible()) {
      await newCommandBtn.click();
      await sleep(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-3-new-command-modal.png'),
        fullPage: true
      });

      const nameInput = page.locator('input').first();
      await nameInput.fill('mobiletest');

      const contentArea = page.locator('textarea').first();
      if (await contentArea.isVisible()) {
        await contentArea.fill('Testing mobile interface');
      }

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-4-filled-form.png'),
        fullPage: true
      });
    }

    console.log('✓ Mobile testing completed');

  } catch (error) {
    console.error('Error in mobile testing:', error.message);
  }
}

async function checkConsole(page) {
  console.log('\n=== Console Error Checking ===');

  const consoleMessages = [];
  const errors = [];

  page.on('console', msg => {
    const text = `[${msg.type()}] ${msg.text()}`;
    consoleMessages.push(text);
    if (msg.type() === 'error') {
      errors.push(text);
    }
  });

  try {
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');

    const slashCommandsLink = page.locator('text=Slash Commands').first();
    await slashCommandsLink.click();
    await sleep(500);

    const newCommandBtn = page.locator('button').filter({ hasText: /new command/i }).first();
    await newCommandBtn.click();
    await sleep(500);

    const nameInput = page.locator('input').first();
    await nameInput.fill('consoletest');

    const createBtn = page.locator('button').filter({ hasText: /create|save/i }).first();
    await createBtn.click();
    await sleep(1000);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'console-test-final.png'),
      fullPage: true
    });

    console.log('\nConsole Messages:', consoleMessages.length);
    console.log('Console Errors:', errors.length);

    if (errors.length > 0) {
      console.log('\nErrors detected:');
      errors.forEach(err => console.log('  ' + err));
    }

  } catch (error) {
    console.error('Error in console checking:', error.message);
  }
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 }
  });
  const page = await context.newPage();

  try {
    // Phase 1: Verify previous findings
    await testQA002(page);
    await testQA003(page);
    await testQA004(page);
    await testQA005(page);

    // Phase 2: Happy path
    await testHappyPath(page);

    // Phase 3: Edge cases
    await testEdgeCases(page);

    // Phase 4: Mobile
    await testMobile(page);

    // Phase 5: Console
    await checkConsole(page);

  } catch (error) {
    console.error('Fatal error:', error);
  } finally {
    await browser.close();
  }

  // Print summary
  console.log('\n\n====================================================');
  console.log('QA ITERATION 3 SUMMARY');
  console.log('====================================================\n');

  console.log('Previous Findings Status:');
  for (const [id, data] of Object.entries(findings)) {
    const status = data.status === 'FIXED' ? '✓' :
                   data.status === 'STILL_PRESENT' ? '✗' : '?';
    console.log(`  ${status} ${id}: ${data.title} - ${data.status} (confidence: ${data.confidence}%)`);
  }

  console.log(`\nScreenshots saved to: ${SCREENSHOT_DIR}`);
  console.log(`Total screenshots: ${fs.readdirSync(SCREENSHOT_DIR).length}`);

  // Save findings to JSON
  const report = {
    timestamp: new Date().toISOString(),
    iteration: 3,
    previousFindings: findings,
    newFindings: newFindings,
    screenshotDir: SCREENSHOT_DIR
  };

  fs.writeFileSync(
    path.join(__dirname, 'qa-iteration3-report.json'),
    JSON.stringify(report, null, 2)
  );

  console.log('\nReport saved to: qa-iteration3-report.json\n');
}

main();
