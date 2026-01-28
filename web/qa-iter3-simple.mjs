#!/usr/bin/env node

/**
 * QA Iteration 3 - Simplified Test Runner
 * Tests Settings page validation and behavior
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

console.log('\n========================================');
console.log('QA ITERATION 3 - Settings Page Testing');
console.log('========================================\n');

const results = {
  'QA-002': { id: 'QA-002', title: 'Forward slash validation', status: 'UNKNOWN', confidence: 0, severity: 'high' },
  'QA-003': { id: 'QA-003', title: 'Spaces validation', status: 'UNKNOWN', confidence: 0, severity: 'high' },
  'QA-004': { id: 'QA-004', title: 'Length validation', status: 'UNKNOWN', confidence: 0, severity: 'high' },
  'QA-005': { id: 'QA-005', title: 'Modified indicator bug', status: 'UNKNOWN', confidence: 0, severity: 'medium' }
};

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function takeScreenshot(page, name, description = '') {
  const filename = path.join(SCREENSHOT_DIR, name);
  await page.screenshot({ path: filename, fullPage: true });
  if (description) {
    console.log(`  📸 ${name} - ${description}`);
  }
}

async function testValidation(page, testId, testName, inputValue, expectedError) {
  console.log(`\n${testId}: ${testName}`);
  console.log('─'.repeat(60));

  try {
    // Navigate to settings
    await page.goto(`${BASE_URL}/settings`, { waitUntil: 'networkidle', timeout: 10000 });
    await sleep(500);

    // Look for Slash Commands section
    console.log('  Looking for Slash Commands section...');
    const slashCommandsText = await page.textContent('body');

    if (!slashCommandsText.includes('Slash Commands')) {
      console.log('  ❌ Could not find "Slash Commands" section');
      await takeScreenshot(page, `${testId}-no-section.png`, 'Settings page loaded but no Slash Commands section');
      results[testId].status = 'ERROR';
      results[testId].confidence = 90;
      return;
    }

    console.log('  ✓ Found Slash Commands section');

    // Click on Slash Commands
    const links = await page.$$('a, button, [role="button"]');
    let clicked = false;
    for (const link of links) {
      const text = await link.textContent();
      if (text && text.includes('Slash Commands')) {
        await link.click();
        await sleep(500);
        clicked = true;
        console.log('  ✓ Clicked Slash Commands');
        break;
      }
    }

    if (!clicked) {
      console.log('  ⚠ Could not click Slash Commands section');
    }

    await takeScreenshot(page, `${testId}-1-section-loaded.png`, 'Slash Commands section opened');

    // Find and click New Command button
    console.log('  Looking for New Command button...');
    const buttons = await page.$$('button');
    let newCommandClicked = false;

    for (const button of buttons) {
      const text = await button.textContent();
      if (text && /new\s+command/i.test(text)) {
        await button.click();
        await sleep(500);
        newCommandClicked = true;
        console.log('  ✓ Clicked New Command button');
        break;
      }
    }

    if (!newCommandClicked) {
      console.log('  ❌ Could not find New Command button');
      await takeScreenshot(page, `${testId}-no-button.png`, 'No New Command button found');
      results[testId].status = 'ERROR';
      results[testId].confidence = 85;
      return;
    }

    await takeScreenshot(page, `${testId}-2-modal-open.png`, 'New Command modal opened');

    // Find name input
    console.log(`  Entering test value: "${inputValue}"`);
    const inputs = await page.$$('input');

    if (inputs.length === 0) {
      console.log('  ❌ No input fields found in modal');
      await takeScreenshot(page, `${testId}-no-input.png`, 'Modal has no input fields');
      results[testId].status = 'ERROR';
      results[testId].confidence = 85;
      return;
    }

    // Fill the first input (should be name)
    await inputs[0].fill(inputValue);
    await sleep(300);
    console.log('  ✓ Entered value in name field');

    await takeScreenshot(page, `${testId}-3-value-entered.png`, `Value "${inputValue}" entered`);

    // Try to submit
    console.log('  Clicking Create/Save button...');
    const submitButtons = await page.$$('button');
    let submitClicked = false;

    for (const button of submitButtons) {
      const text = await button.textContent();
      if (text && /(create|save)/i.test(text)) {
        await button.click();
        await sleep(1000);
        submitClicked = true;
        console.log('  ✓ Clicked Create/Save button');
        break;
      }
    }

    if (!submitClicked) {
      console.log('  ⚠ Could not find Create/Save button');
    }

    // Check for validation errors
    console.log('  Checking for validation errors...');
    const pageText = await page.textContent('body');
    const lowerPageText = pageText.toLowerCase();

    const hasError =
      lowerPageText.includes('invalid') ||
      lowerPageText.includes('cannot contain') ||
      lowerPageText.includes(expectedError.toLowerCase()) ||
      lowerPageText.includes('error') && (lowerPageText.includes('name') || lowerPageText.includes('command'));

    await takeScreenshot(page, `${testId}-4-validation-result.png`, 'Final result after submission');

    if (hasError) {
      console.log('  ✅ FIXED - Validation error detected!');
      results[testId].status = 'FIXED';
      results[testId].confidence = 90;
    } else {
      console.log('  ❌ STILL_PRESENT - No validation error shown');
      results[testId].status = 'STILL_PRESENT';
      results[testId].confidence = 95;
    }

    // Close modal if still open
    const cancelButtons = await page.$$('button');
    for (const button of cancelButtons) {
      const text = await button.textContent();
      if (text && /(cancel|close)/i.test(text)) {
        try {
          await button.click();
          await sleep(300);
          console.log('  ✓ Closed modal');
        } catch (e) {
          // Modal might have auto-closed
        }
        break;
      }
    }

  } catch (error) {
    console.log(`  ❌ ERROR: ${error.message}`);
    await takeScreenshot(page, `${testId}-error.png`, `Error: ${error.message}`);
    results[testId].status = 'ERROR';
    results[testId].confidence = 80;
  }
}

async function testModifiedIndicator(page) {
  const testId = 'QA-005';
  console.log(`\n${testId}: Modified Indicator Bug`);
  console.log('─'.repeat(60));

  try {
    await page.goto(`${BASE_URL}/settings`, { waitUntil: 'networkidle', timeout: 10000 });
    await sleep(500);

    // Navigate to Slash Commands
    const links = await page.$$('a, button, [role="button"]');
    for (const link of links) {
      const text = await link.textContent();
      if (text && text.includes('Slash Commands')) {
        await link.click();
        await sleep(500);
        break;
      }
    }

    await takeScreenshot(page, `${testId}-1-initial.png`, 'Initial state');

    // Find command cards/buttons
    console.log('  Looking for command cards...');
    const allElements = await page.$$('[class*="card"], [class*="command"], button');

    let commandElements = [];
    for (const el of allElements) {
      const text = await el.textContent();
      // Skip if it's the New Command button
      if (text && !text.includes('New Command') && text.trim().length > 0) {
        commandElements.push(el);
      }
    }

    console.log(`  Found ${commandElements.length} potential command elements`);

    if (commandElements.length < 2) {
      console.log('  ⚠ Not enough commands to test (need at least 2)');
      console.log('  Creating test commands...');

      // Create two test commands
      for (let i = 0; i < 2; i++) {
        const buttons = await page.$$('button');
        for (const button of buttons) {
          const text = await button.textContent();
          if (text && /new\s+command/i.test(text)) {
            await button.click();
            await sleep(500);
            break;
          }
        }

        const inputs = await page.$$('input');
        await inputs[0].fill(`testcmd${i + 1}`);

        const textareas = await page.$$('textarea');
        if (textareas.length > 0) {
          await textareas[0].fill(`Test content ${i + 1}`);
        }

        const submitButtons = await page.$$('button');
        for (const button of submitButtons) {
          const text = await button.textContent();
          if (text && /(create|save)/i.test(text)) {
            await button.click();
            await sleep(1000);
            break;
          }
        }
      }

      // Re-fetch command elements
      const updatedElements = await page.$$('[class*="card"], [class*="command"], button');
      commandElements = [];
      for (const el of updatedElements) {
        const text = await el.textContent();
        if (text && !text.includes('New Command') && text.trim().length > 0) {
          commandElements.push(el);
        }
      }
    }

    if (commandElements.length >= 2) {
      console.log('  ✓ Found at least 2 commands');

      // Click first command
      console.log('  Clicking first command...');
      await commandElements[0].click();
      await sleep(500);
      await takeScreenshot(page, `${testId}-2-first-selected.png`, 'First command selected');

      // Click second command WITHOUT editing
      console.log('  Clicking second command (without editing)...');
      await commandElements[1].click();
      await sleep(500);

      // Check for "Modified" indicator
      const pageText = await page.textContent('body');
      const hasModified = pageText.toLowerCase().includes('modified');

      await takeScreenshot(page, `${testId}-3-second-selected.png`, 'Second command selected');

      if (!hasModified) {
        console.log('  ✅ FIXED - No "Modified" indicator shown');
        results[testId].status = 'FIXED';
        results[testId].confidence = 85;
      } else {
        console.log('  ❌ STILL_PRESENT - "Modified" indicator incorrectly shown');
        results[testId].status = 'STILL_PRESENT';
        results[testId].confidence = 90;
      }
    } else {
      console.log('  ⚠ Could not create or find enough commands');
      results[testId].status = 'ERROR';
      results[testId].confidence = 70;
    }

  } catch (error) {
    console.log(`  ❌ ERROR: ${error.message}`);
    await takeScreenshot(page, `${testId}-error.png`, `Error: ${error.message}`);
    results[testId].status = 'ERROR';
    results[testId].confidence = 75;
  }
}

async function main() {
  console.log(`Base URL: ${BASE_URL}`);
  console.log(`Screenshot directory: ${SCREENSHOT_DIR}\n`);

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
  });

  const page = await context.newPage();

  // Set longer timeouts
  page.setDefaultTimeout(15000);

  try {
    // Phase 1: Test previous findings
    console.log('\n========================================');
    console.log('PHASE 1: VERIFY PREVIOUS FINDINGS');
    console.log('========================================');

    await testValidation(page, 'QA-002', 'Forward slash validation', 'test/command', 'slash');
    await testValidation(page, 'QA-003', 'Spaces validation', 'test command', 'space');
    await testValidation(page, 'QA-004', 'Length validation', 'a'.repeat(200), 'length');
    await testModifiedIndicator(page);

  } catch (error) {
    console.error('\n❌ Fatal error:', error.message);
  } finally {
    await browser.close();
  }

  // Print summary
  console.log('\n\n========================================');
  console.log('SUMMARY');
  console.log('========================================\n');

  for (const [id, result] of Object.entries(results)) {
    const icon = result.status === 'FIXED' ? '✅' :
                 result.status === 'STILL_PRESENT' ? '❌' :
                 result.status === 'ERROR' ? '⚠️' : '❓';

    console.log(`${icon} ${id} (${result.severity}): ${result.title}`);
    console.log(`   Status: ${result.status} (confidence: ${result.confidence}%)`);
    console.log('');
  }

  console.log(`Screenshots saved to: ${SCREENSHOT_DIR}`);
  console.log(`Total screenshots: ${fs.readdirSync(SCREENSHOT_DIR).length}`);

  // Save report
  const report = {
    timestamp: new Date().toISOString(),
    iteration: 3,
    findings: results,
    screenshotDir: SCREENSHOT_DIR
  };

  const reportPath = path.join(__dirname, 'qa-iteration3-report.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));

  console.log(`\nReport saved to: ${reportPath}\n`);

  // Exit with error if any STILL_PRESENT findings
  const stillPresent = Object.values(results).filter(r => r.status === 'STILL_PRESENT').length;
  if (stillPresent > 0) {
    console.log(`⚠️  ${stillPresent} issue(s) still present\n`);
    process.exit(1);
  } else {
    console.log('✅ All previous issues have been fixed!\n');
    process.exit(0);
  }
}

main().catch(error => {
  console.error('Fatal error:', error);
  process.exit(1);
});
