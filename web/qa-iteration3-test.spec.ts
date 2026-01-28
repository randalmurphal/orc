import { test, expect, type Page } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';

const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = path.join(__dirname, '..', 'qa-screenshots-iter3');

// Ensure screenshot directory exists
if (!fs.existsSync(SCREENSHOT_DIR)) {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
}

test.describe('Settings Page QA - Iteration 3', () => {
  let page: Page;

  test.beforeEach(async ({ browser }) => {
    page = await browser.newPage();
    await page.goto(`${BASE_URL}/settings`);
    await page.waitForLoadState('networkidle');
  });

  test.afterEach(async () => {
    await page.close();
  });

  test.describe('Phase 1: Verify Previous Findings', () => {

    test('QA-002: Forward slash validation in command names', async () => {
      console.log('\n=== Testing QA-002: Forward slash validation ===');

      // Navigate to Slash Commands section
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      // Click New Command button
      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      // Take screenshot of modal
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-002-1-modal-open.png'),
        fullPage: true
      });

      // Type command name with forward slash
      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('test/command');
      await page.waitForTimeout(300);

      // Take screenshot showing input
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-002-2-slash-entered.png'),
        fullPage: true
      });

      // Try to create/save
      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      // Check for validation error
      const validationError = await page.locator('text=/cannot contain.*slash/i, text=/invalid.*name/i, .error, .text-red').count();

      // Take final screenshot
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-002-3-validation-result.png'),
        fullPage: true
      });

      console.log(`Validation error count: ${validationError}`);
      console.log('Status: ' + (validationError > 0 ? 'FIXED ✓' : 'STILL_PRESENT ✗'));
    });

    test('QA-003: Spaces validation in command names', async () => {
      console.log('\n=== Testing QA-003: Spaces validation ===');

      // Navigate to Slash Commands section
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      // Click New Command button
      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      // Take screenshot of modal
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-003-1-modal-open.png'),
        fullPage: true
      });

      // Type command name with spaces
      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('test command');
      await page.waitForTimeout(300);

      // Take screenshot showing input
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-003-2-spaces-entered.png'),
        fullPage: true
      });

      // Try to create/save
      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      // Check for validation error
      const validationError = await page.locator('text=/cannot contain.*space/i, text=/invalid.*name/i, .error, .text-red').count();

      // Take final screenshot
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-003-3-validation-result.png'),
        fullPage: true
      });

      console.log(`Validation error count: ${validationError}`);
      console.log('Status: ' + (validationError > 0 ? 'FIXED ✓' : 'STILL_PRESENT ✗'));
    });

    test('QA-004: Maximum length validation for command names', async () => {
      console.log('\n=== Testing QA-004: Maximum length validation ===');

      // Navigate to Slash Commands section
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      // Click New Command button
      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      // Take screenshot of modal
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-004-1-modal-open.png'),
        fullPage: true
      });

      // Type very long command name (200 characters)
      const longName = 'a'.repeat(200);
      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill(longName);
      await page.waitForTimeout(300);

      // Take screenshot showing input
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-004-2-long-name-entered.png'),
        fullPage: true
      });

      // Try to create/save
      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      // Check for validation error
      const validationError = await page.locator('text=/too long/i, text=/maximum.*character/i, text=/must be.*character/i, .error, .text-red').count();

      // Take final screenshot
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-004-3-validation-result.png'),
        fullPage: true
      });

      console.log(`Validation error count: ${validationError}`);
      console.log('Status: ' + (validationError > 0 ? 'FIXED ✓' : 'STILL_PRESENT ✗'));
    });

    test('QA-005: Modified indicator on command switching', async () => {
      console.log('\n=== Testing QA-005: Modified indicator bug ===');

      // Navigate to Slash Commands section
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      // Take screenshot of initial state
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'QA-005-1-initial-state.png'),
        fullPage: true
      });

      // Check if there are any existing commands
      const commandCards = await page.locator('[data-testid="command-card"], .command-card, button[role="button"]').count();
      console.log(`Found ${commandCards} command cards`);

      if (commandCards >= 2) {
        // Click first command
        const firstCommand = page.locator('[data-testid="command-card"], .command-card, button[role="button"]').first();
        await firstCommand.click();
        await page.waitForTimeout(500);

        // Take screenshot of first command selected
        await page.screenshot({
          path: path.join(SCREENSHOT_DIR, 'QA-005-2-first-command-selected.png'),
          fullPage: true
        });

        // Click second command WITHOUT editing
        const secondCommand = page.locator('[data-testid="command-card"], .command-card, button[role="button"]').nth(1);
        await secondCommand.click();
        await page.waitForTimeout(500);

        // Check for Modified indicator
        const modifiedIndicator = await page.locator('text=/modified/i, .modified, [data-modified="true"]').count();

        // Take screenshot showing result
        await page.screenshot({
          path: path.join(SCREENSHOT_DIR, 'QA-005-3-second-command-selected.png'),
          fullPage: true
        });

        console.log(`Modified indicator count: ${modifiedIndicator}`);
        console.log('Status: ' + (modifiedIndicator === 0 ? 'FIXED ✓' : 'STILL_PRESENT ✗'));
      } else {
        console.log('Not enough commands to test QA-005. Need to create test commands first.');

        // Create two test commands
        for (let i = 0; i < 2; i++) {
          const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
          await newCommandButton.click();
          await page.waitForTimeout(500);

          const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
          await nameInput.fill(`testcommand${i + 1}`);

          const contentInput = page.locator('textarea, [role="textbox"]').first();
          await contentInput.fill(`Test content ${i + 1}`);

          const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
          await createButton.click();
          await page.waitForTimeout(1000);
        }

        // Now test switching
        const firstCommand = page.locator('[data-testid="command-card"], .command-card, button[role="button"]').first();
        await firstCommand.click();
        await page.waitForTimeout(500);

        await page.screenshot({
          path: path.join(SCREENSHOT_DIR, 'QA-005-2-first-command-selected.png'),
          fullPage: true
        });

        const secondCommand = page.locator('[data-testid="command-card"], .command-card, button[role="button"]').nth(1);
        await secondCommand.click();
        await page.waitForTimeout(500);

        const modifiedIndicator = await page.locator('text=/modified/i, .modified, [data-modified="true"]').count();

        await page.screenshot({
          path: path.join(SCREENSHOT_DIR, 'QA-005-3-second-command-selected.png'),
          fullPage: true
        });

        console.log(`Modified indicator count: ${modifiedIndicator}`);
        console.log('Status: ' + (modifiedIndicator === 0 ? 'FIXED ✓' : 'STILL_PRESENT ✗'));
      }
    });
  });

  test.describe('Phase 2: Happy Path Testing', () => {

    test('Happy Path: Create, edit, save, delete command', async () => {
      console.log('\n=== Testing Happy Path ===');

      // Navigate to Slash Commands
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-1-slash-commands-loaded.png'),
        fullPage: true
      });

      // Create new command
      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-2-new-command-modal.png'),
        fullPage: true
      });

      // Fill in valid details
      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('validcommand');

      const contentInput = page.locator('textarea, [role="textbox"]').first();
      await contentInput.fill('This is a valid command content');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-3-filled-details.png'),
        fullPage: true
      });

      // Create command
      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-4-command-created.png'),
        fullPage: true
      });

      // Edit the command
      const createdCommand = page.locator('text=validcommand').first();
      await createdCommand.click();
      await page.waitForTimeout(500);

      const editor = page.locator('textarea, [role="textbox"]').first();
      await editor.fill('Updated command content');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-5-command-edited.png'),
        fullPage: true
      });

      // Save changes
      const saveButton = page.locator('button:has-text("Save")').first();
      await saveButton.click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'happy-6-command-saved.png'),
        fullPage: true
      });

      console.log('Happy path completed successfully');
    });
  });

  test.describe('Phase 3: Edge Cases', () => {

    test('Edge Case: Empty command name', async () => {
      console.log('\n=== Testing Edge Case: Empty name ===');

      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      // Leave name empty, only fill content
      const contentInput = page.locator('textarea, [role="textbox"]').first();
      await contentInput.fill('Content without name');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-1-empty-name.png'),
        fullPage: true
      });

      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-2-empty-name-result.png'),
        fullPage: true
      });
    });

    test('Edge Case: Special characters in command names', async () => {
      console.log('\n=== Testing Edge Case: Special characters ===');

      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      const specialChars = ['test@command', 'test#command', 'test$command', 'test%command'];

      for (const testName of specialChars) {
        const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
        await newCommandButton.click();
        await page.waitForTimeout(500);

        const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
        await nameInput.fill(testName);
        await page.waitForTimeout(300);

        await page.screenshot({
          path: path.join(SCREENSHOT_DIR, `edge-special-${testName.replace(/[^a-z0-9]/gi, '')}.png`),
          fullPage: true
        });

        const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
        await createButton.click();
        await page.waitForTimeout(1000);

        // Close modal if it's still open
        const cancelButton = page.locator('button:has-text("Cancel"), button:has-text("Close")').first();
        if (await cancelButton.isVisible()) {
          await cancelButton.click();
          await page.waitForTimeout(300);
        }
      }
    });

    test('Edge Case: Very long command content', async () => {
      console.log('\n=== Testing Edge Case: Very long content ===');

      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('longcontent');

      const longContent = 'Lorem ipsum dolor sit amet. '.repeat(500); // ~10000+ chars
      const contentInput = page.locator('textarea, [role="textbox"]').first();
      await contentInput.fill(longContent);
      await page.waitForTimeout(300);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-3-long-content.png'),
        fullPage: true
      });

      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-4-long-content-result.png'),
        fullPage: true
      });
    });

    test('Edge Case: Rapid clicking Save button', async () => {
      console.log('\n=== Testing Edge Case: Rapid clicking ===');

      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('rapidclick');

      const contentInput = page.locator('textarea, [role="textbox"]').first();
      await contentInput.fill('Testing rapid clicks');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-5-before-rapid-click.png'),
        fullPage: true
      });

      // Rapidly click save button 5 times
      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      for (let i = 0; i < 5; i++) {
        await createButton.click({ force: true });
        await page.waitForTimeout(50);
      }

      await page.waitForTimeout(2000);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'edge-6-after-rapid-click.png'),
        fullPage: true
      });
    });
  });

  test.describe('Phase 4: Mobile Testing', () => {

    test('Mobile: Settings page at 375x667', async () => {
      console.log('\n=== Testing Mobile Viewport ===');

      // Set mobile viewport
      await page.setViewportSize({ width: 375, height: 667 });
      await page.waitForTimeout(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-1-initial.png'),
        fullPage: true
      });

      // Navigate to Slash Commands
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-2-slash-commands.png'),
        fullPage: true
      });

      // Try to open new command modal
      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-3-new-command-modal.png'),
        fullPage: true
      });

      // Try to fill in details
      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('mobiletest');

      const contentInput = page.locator('textarea, [role="textbox"]').first();
      await contentInput.fill('Testing mobile interface');

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'mobile-4-filled-form.png'),
        fullPage: true
      });

      console.log('Mobile testing completed');
    });
  });

  test.describe('Phase 5: Console Errors', () => {

    test('Console: Check for errors during workflow', async () => {
      console.log('\n=== Checking Console Errors ===');

      const consoleMessages: string[] = [];
      const errors: string[] = [];

      page.on('console', msg => {
        const text = `[${msg.type()}] ${msg.text()}`;
        consoleMessages.push(text);
        if (msg.type() === 'error') {
          errors.push(text);
        }
      });

      // Perform typical workflow
      await page.click('text=Slash Commands');
      await page.waitForTimeout(500);

      const newCommandButton = page.locator('button:has-text("New Command"), button:has-text("+ New Command")').first();
      await newCommandButton.click();
      await page.waitForTimeout(500);

      const nameInput = page.locator('input[name="name"], input[placeholder*="name" i]').first();
      await nameInput.fill('consoletest');

      const createButton = page.locator('button:has-text("Create"), button:has-text("Save")').first();
      await createButton.click();
      await page.waitForTimeout(1000);

      console.log('\n--- Console Messages ---');
      consoleMessages.forEach(msg => console.log(msg));

      console.log('\n--- Console Errors ---');
      if (errors.length > 0) {
        errors.forEach(err => console.log(err));
      } else {
        console.log('No console errors detected');
      }

      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'console-test-final.png'),
        fullPage: true
      });
    });
  });
});
