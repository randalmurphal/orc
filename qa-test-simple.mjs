#!/usr/bin/env node

/**
 * Simple QA test for Agents page
 * Runs basic navigation and screenshot capture
 */

import { exec } from 'child_process';
import { promisify } from 'util';
import { existsSync, mkdirSync } from 'fs';

const execAsync = promisify(exec);

const OUTPUT_DIR = '/tmp/qa-TASK-613';

// Ensure output directory exists
if (!existsSync(OUTPUT_DIR)) {
  mkdirSync(OUTPUT_DIR, { recursive: true });
}

console.log('QA Test: Agents Page');
console.log('===================\n');

// Check if server is responding
try {
  console.log('1. Checking if dev server is running...');
  const { stdout } = await execAsync('curl -s -o /dev/null -w "%{http_code}" http://localhost:5173');
  if (stdout.trim() === '200') {
    console.log('   ✓ Dev server responding at http://localhost:5173\n');
  } else {
    console.log(`   ⚠ Server returned status ${stdout.trim()}\n`);
  }
} catch (error) {
  console.log('   ❌ Dev server not accessible');
  console.log('   Error:', error.message);
  process.exit(1);
}

// Use playwright CLI to capture screenshots
console.log('2. Capturing screenshots with Playwright...');
console.log('   Note: Running from web directory where Playwright is installed\n');

try {
  // Desktop screenshot
  console.log('   Taking desktop screenshot (1920x1080)...');
  await execAsync(
    `cd web && npx playwright screenshot ` +
    `--browser chromium ` +
    `--viewport-size=1920,1080 ` +
    `--full-page ` +
    `http://localhost:5173/agents ` +
    `${OUTPUT_DIR}/agents-desktop.png`,
    { cwd: '/home/randy/repos/orc/.orc/worktrees/orc-TASK-613' }
  );
  console.log(`   ✓ Saved: ${OUTPUT_DIR}/agents-desktop.png`);

  // Mobile screenshot
  console.log('   Taking mobile screenshot (375x667)...');
  await execAsync(
    `cd web && npx playwright screenshot ` +
    `--browser chromium ` +
    `--viewport-size=375,667 ` +
    `--full-page ` +
    `http://localhost:5173/agents ` +
    `${OUTPUT_DIR}/agents-mobile.png`,
    { cwd: '/home/randy/repos/orc/.orc/worktrees/orc-TASK-613' }
  );
  console.log(`   ✓ Saved: ${OUTPUT_DIR}/agents-mobile.png\n`);

} catch (error) {
  console.error('   ❌ Screenshot capture failed:');
  console.error('   ', error.message);
  if (error.stderr) {
    console.error('   ', error.stderr);
  }
  process.exit(1);
}

console.log('===================');
console.log('✓ Screenshots captured successfully');
console.log(`\nOutput directory: ${OUTPUT_DIR}/`);
console.log('  - agents-desktop.png (1920x1080)');
console.log('  - agents-mobile.png (375x667)');
console.log('\nNext: Manual review required to compare against reference design');
console.log('Reference: /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/example_ui/agents-config.png');
