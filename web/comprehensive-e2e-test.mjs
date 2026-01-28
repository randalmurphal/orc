#!/usr/bin/env node
/**
 * Comprehensive E2E Test for Settings > Slash Commands
 * TASK-616: Validate against reference image example_ui/settings-slash-commands.png
 */

import { chromium } from 'playwright';
import { mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = join(__dirname, 'qa-screenshots-detailed');
const FINDINGS = [];
let screenshotCounter = 1;
let scenariosTested = 0;

if (!existsSync(SCREENSHOT_DIR)) {
  mkdirSync(SCREENSHOT_DIR, { recursive: true });
}

function addFinding(severity, confidence, category, title, steps, expected, actual, screenshotPath, suggestedFix = '') {
  if (confidence >= 80) {
    const id = `QA-${String(FINDINGS.length + 1).padStart(3, '0')}`;
    FINDINGS.push({
      id, severity, confidence, category, title,
      steps_to_reproduce: steps,
      expected, actual,
      screenshot_path: screenshotPath || '',
      suggested_fix: suggestedFix
    });
    console.log(`\n❌ ${id}: ${title} [${severity.toUpperCase()}] (${confidence}%)`);
  }
}

async function takeScreenshot(page, name) {
  const path = join(SCREENSHOT_DIR, `${screenshotCounter}-${name}.png`);
  await page.screenshot({ path, fullPage: true });
  screenshotCounter++;
  console.log(`📸 ${path}`);
  return path;
}

async function runComprehensiveTests() {
  console.log('🚀 Comprehensive E2E Test: Settings > Slash Commands\n');

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
  const page = await context.newPage();

  const consoleMessages = [];
  page.on('console', msg => {
    consoleMessages.push({ type: msg.type(), text: msg.text() });
    if (msg.type() === 'error') console.log(`🔴 Console: ${msg.text()}`);
  });

  try {
    // ==================== TEST 1: Navigation ====================
    console.log('📋 TEST 1: Navigation to Slash Commands');
    scenariosTested++;

    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(1500);  // Wait for React hydration

    // Click Settings (try multiple selectors)
    const settingsSelectors = [
      'a[href="/settings"]',
      'nav a:has-text("Settings")',
      '[aria-label="Settings"]',
      'text="Settings"'
    ];

    let settingsClicked = false;
    for (const selector of settingsSelectors) {
      if (await page.locator(selector).count() > 0) {
        await page.locator(selector).first().click();
        settingsClicked = true;
        console.log(`✓ Clicked Settings using: ${selector}`);
        break;
      }
    }

    if (!settingsClicked) {
      addFinding('high', 90, 'functional', 'Settings navigation not found',
        ['Navigate to ' + BASE_URL], 'Settings link should be in sidebar',
        'No Settings link found', await takeScreenshot(page, 'settings-not-found'));
    }

    await page.waitForTimeout(500);

    // Click Slash Commands
    const slashCmdSelectors = [
      'text="Slash Commands"',
      'a:has-text("Slash Commands")',
      '[href*="slash"]'
    ];

    let slashCmdClicked = false;
    for (const selector of slashCmdSelectors) {
      if (await page.locator(selector).count() > 0) {
        await page.locator(selector).first().click();
        slashCmdClicked = true;
        console.log(`✓ Clicked Slash Commands using: ${selector}`);
        break;
      }
    }

    await page.waitForTimeout(1000);
    const navScreenshot = await takeScreenshot(page, 'page-loaded');

    // ==================== TEST 2: Visual Structure ====================
    console.log('\n📋 TEST 2: Visual Structure Validation');
    scenariosTested++;

    // Check page title
    const titleLocator = page.locator('h1:has-text("Slash Commands"), h2:has-text("Slash Commands")');
    if (await titleLocator.count() === 0) {
      addFinding('medium', 85, 'visual', 'Page title missing',
        ['Navigate to Settings > Slash Commands'],
        'Should display "Slash Commands" heading',
        'No heading found', navScreenshot);
    } else {
      console.log('✓ Page title "Slash Commands" found');
    }

    // Check "+ New Command" button
    const newCmdBtn = page.locator('button:has-text("New Command")');
    if (await newCmdBtn.count() === 0) {
      addFinding('medium', 85, 'functional', '+ New Command button missing',
        ['Navigate to Settings > Slash Commands'],
        'Should show "+ New Command" button',
        'Button not found', navScreenshot);
    } else {
      console.log('✓ "+ New Command" button found');
    }

    // Check sections
    const projectSection = await page.locator('text="Project Commands"').count();
    const globalSection = await page.locator('text="Global Commands"').count();

    console.log(`✓ Project Commands section: ${projectSection > 0 ? 'FOUND' : 'MISSING'}`);
    console.log(`✓ Global Commands section: ${globalSection > 0 ? 'FOUND' : 'MISSING'}`);

    // Find command cards (try multiple selectors)
    const commandCardSelectors = [
      '[class*="CommandCard"]',
      '[data-command]',
      'div:has(button[aria-label*="edit"]):has(button[aria-label*="delete"])',
      'div:has-text("/") >> visible=true'
    ];

    let commandCards = page.locator('//div[contains(@class, "command") or contains(@class, "Command")]//ancestor::div[1]');
    let commandCount = await commandCards.count();

    // Try to find cards by looking for terminal icons + command names
    if (commandCount === 0) {
      // Look for elements with terminal icons
      commandCards = page.locator('div:has(svg):has-text("/")').filter({ hasNotText: 'New Command' });
      commandCount = await commandCards.count();
    }

    console.log(`Found ${commandCount} command card(s)`);

    // ==================== TEST 3: Command Selection ====================
    console.log('\n📋 TEST 3: Command Selection & Editor Updates');
    scenariosTested++;

    if (commandCount > 0) {
      // Find all clickable command elements
      const allCommands = page.locator('div:has-text("/")').filter({ has: page.locator('svg') });
      const visibleCommands = await allCommands.all();

      if (visibleCommands.length >= 2) {
        console.log(`Testing with ${visibleCommands.length} commands`);

        // Click first command
        await visibleCommands[0].click();
        await page.waitForTimeout(500);
        const screenshot1 = await takeScreenshot(page, 'command-1-selected');

        // Get editor content (try textarea or contenteditable)
        const editorSelectors = [
          'textarea',
          '[contenteditable="true"]',
          '.monaco-editor',
          '[class*="editor"]'
        ];

        let editor = null;
        let editorContent1 = '';

        for (const sel of editorSelectors) {
          const elem = page.locator(sel).first();
          if (await elem.count() > 0) {
            editor = elem;
            editorContent1 = await elem.inputValue().catch(() =>
              elem.textContent().catch(() => '')
            );
            if (editorContent1) {
              console.log(`✓ Editor content captured (${editorContent1.length} chars)`);
              break;
            }
          }
        }

        // Click second command
        await visibleCommands[1].click();
        await page.waitForTimeout(500);
        const screenshot2 = await takeScreenshot(page, 'command-2-selected');

        if (editor) {
          const editorContent2 = await editor.inputValue().catch(() =>
            editor.textContent().catch(() => '')
          );

          console.log(`Editor content 1: ${editorContent1.substring(0, 50)}...`);
          console.log(`Editor content 2: ${editorContent2.substring(0, 50)}...`);

          if (editorContent1 === editorContent2 && editorContent1.length > 0) {
            addFinding('critical', 95, 'functional',
              'Editor does not update when switching commands',
              [
                'Navigate to Settings > Slash Commands',
                'Click first command',
                'Note editor content',
                'Click second command',
                'Compare editor content'
              ],
              'Editor should show different content for different commands',
              'Editor shows identical content for both commands',
              screenshot2,
              'Check ConfigEditor component state management - likely using stale initial state');
          } else if (editorContent2.length > 0 && editorContent1 !== editorContent2) {
            console.log('✓ Editor content updates correctly');
          }
        }
      }
    }

    // ==================== TEST 4: New Command Modal ====================
    console.log('\n📋 TEST 4: New Command Modal & Validation');
    scenariosTested++;

    if (await newCmdBtn.count() > 0) {
      await newCmdBtn.click();
      await page.waitForTimeout(500);

      const modal = page.locator('[role="dialog"], .modal, [class*="Modal"]').first();
      if (await modal.count() === 0) {
        addFinding('medium', 85, 'functional', 'New Command modal does not open',
          ['Click "+ New Command"'],
          'Should open modal dialog',
          'No modal appears', await takeScreenshot(page, 'no-modal'));
      } else {
        await takeScreenshot(page, 'modal-opened');
        console.log('✓ Modal opened');

        // Test input validation with special characters
        const nameInput = modal.locator('input[name="name"], input[placeholder*="command"]').first();
        if (await nameInput.count() > 0) {
          // Test 1: Special character "/"
          await nameInput.fill('test/command');
          await page.waitForTimeout(300);

          let errorVisible = await page.locator('.error, .text-red, [role="alert"]').count();
          if (errorVisible === 0) {
            const screenshot = await takeScreenshot(page, 'no-validation-slash');
            addFinding('high', 90, 'functional',
              'No validation for "/" in command name',
              ['Click "+ New Command"', 'Enter "test/command" in Name field'],
              'Should show validation error for special characters',
              'No error shown, accepts "/" character',
              screenshot,
              'Add input validation: /^[a-z0-9-_]+$/i');
          } else {
            console.log('✓ Validation works for "/"');
          }

          // Test 2: Spaces
          await nameInput.fill('test command');
          await page.waitForTimeout(300);

          errorVisible = await page.locator('.error, .text-red, [role="alert"]').count();
          if (errorVisible === 0) {
            await takeScreenshot(page, 'no-validation-space');
            addFinding('high', 88, 'functional',
              'No validation for spaces in command name',
              ['Click "+ New Command"', 'Enter "test command" in Name field'],
              'Should show validation error for spaces',
              'No error shown, accepts spaces',
              '',
              'Command names should be alphanumeric with hyphens/underscores only');
          }

          // Test 3: Very long name
          await nameInput.fill('a'.repeat(200));
          await page.waitForTimeout(300);

          errorVisible = await page.locator('.error, .text-red, [role="alert"]').count();
          if (errorVisible === 0) {
            await takeScreenshot(page, 'no-validation-length');
            addFinding('medium', 80, 'functional',
              'No length validation for command name',
              ['Click "+ New Command"', 'Enter 200 character name'],
              'Should limit command name length',
              'Accepts very long names without validation',
              '',
              'Add maxLength validation (e.g., 50 chars)');
          }
        }

        // Close modal
        const closeBtn = modal.locator('button:has-text("Cancel"), button:has-text("Close"), [aria-label="Close"]').first();
        if (await closeBtn.count() > 0) {
          await closeBtn.click();
          await page.waitForTimeout(300);
        }
      }
    }

    // ==================== TEST 5: Mobile Viewport ====================
    console.log('\n📋 TEST 5: Mobile Viewport (375x667)');
    scenariosTested++;

    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);
    await takeScreenshot(page, 'mobile-full-page');

    // Check for horizontal overflow
    const hasHorizontalScroll = await page.evaluate(() => {
      return document.documentElement.scrollWidth > window.innerWidth;
    });

    if (hasHorizontalScroll) {
      addFinding('medium', 85, 'visual', 'Horizontal scroll on mobile',
        ['Resize to 375x667'],
        'Page should fit without horizontal scroll',
        'Page has horizontal overflow',
        '', 'Check responsive CSS');
    } else {
      console.log('✓ No horizontal scroll on mobile');
    }

    // Check button accessibility
    if (await newCmdBtn.count() > 0) {
      const box = await newCmdBtn.boundingBox();
      if (box && (box.x + box.width > 375 || box.y < 0)) {
        addFinding('medium', 80, 'accessibility',
          'Button not fully visible on mobile',
          ['Resize to 375x667', 'Check "+ New Command" button'],
          'Button should be fully visible and accessible',
          'Button extends beyond viewport or is cut off',
          '');
      }
    }

    console.log('✓ Mobile viewport tests complete');

    // ==================== TEST 6: Console Errors ====================
    console.log('\n📋 TEST 6: Console Errors');
    scenariosTested++;

    const errors = consoleMessages.filter(m => m.type === 'error');
    const warnings = consoleMessages.filter(m => m.type === 'warning');

    console.log(`Console errors: ${errors.length}`);
    console.log(`Console warnings: ${warnings.length}`);

    if (errors.length > 0) {
      const errorText = errors.map(e => e.text).slice(0, 3).join('; ');
      addFinding('medium', 85, 'functional',
        `${errors.length} JavaScript error(s) in console`,
        ['Navigate to Settings > Slash Commands', 'Open DevTools console'],
        'No JavaScript errors',
        `Found ${errors.length} error(s): ${errorText}`,
        '',
        'Check browser console for stack traces');
    }

  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    await takeScreenshot(page, 'test-failure');
    throw error;
  } finally {
    await browser.close();
  }

  // ==================== Report ====================
  console.log('\n' + '='.repeat(70));
  console.log('📊 TEST SUMMARY');
  console.log('='.repeat(70));
  console.log(`Scenarios tested: ${scenariosTested}`);
  console.log(`Findings: ${FINDINGS.length}`);
  console.log(`Screenshots: ${screenshotCounter - 1}`);

  if (FINDINGS.length > 0) {
    console.log('\n🐛 FINDINGS:\n');
    FINDINGS.forEach(f => {
      console.log(`${f.id}: ${f.title}`);
      console.log(`  Severity: ${f.severity} | Confidence: ${f.confidence}% | Category: ${f.category}`);
      if (f.suggested_fix) console.log(`  Fix: ${f.suggested_fix}`);
      console.log();
    });
  }

  const result = {
    status: 'complete',
    summary: `Tested ${scenariosTested} scenarios across 2 viewports, found ${FINDINGS.length} issue(s)`,
    findings: FINDINGS,
    verification: {
      scenarios_tested: scenariosTested,
      viewports_tested: ['desktop (1920x1080)', 'mobile (375x667)'],
      previous_issues_verified: []
    }
  };

  console.log('='.repeat(70));
  console.log('📄 STRUCTURED OUTPUT');
  console.log('='.repeat(70));
  console.log(JSON.stringify(result, null, 2));

  return result;
}

runComprehensiveTests()
  .then(result => {
    console.log(`\n${result.findings.length === 0 ? '✅' : '⚠️'} Tests complete`);
    process.exit(result.findings.length > 0 ? 1 : 0);
  })
  .catch(error => {
    console.error('\n❌ Tests failed:', error);
    process.exit(1);
  });
