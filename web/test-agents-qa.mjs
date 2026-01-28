#!/usr/bin/env node

/**
 * E2E QA Test for Agents Page - TASK-613
 * Validates implementation against reference design
 */

import { chromium } from '@playwright/test';
import { writeFileSync, mkdirSync } from 'fs';
import { join } from 'path';

const SCREENSHOT_DIR = '/tmp/qa-TASK-613';
const APP_URL = 'http://localhost:5173';

const findings = [];
let findingId = 1;

function addFinding(severity, category, title, expected, actual, screenshotPath, steps, confidence = 95) {
  findings.push({
    id: `QA-${String(findingId++).padStart(3, '0')}`,
    severity,
    confidence,
    category,
    title,
    steps_to_reproduce: steps,
    expected,
    actual,
    screenshot_path: screenshotPath
  });
}

async function testAgentsPage() {
  console.log('🚀 Starting E2E QA Test for Agents Page...\n');

  // Ensure screenshot directory exists
  try {
    mkdirSync(SCREENSHOT_DIR, { recursive: true });
  } catch (err) {
    // Directory might already exist, ignore
  }

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    deviceScaleFactor: 1
  });
  const page = await context.newPage();

  // Capture console messages
  const consoleMessages = [];
  page.on('console', msg => {
    consoleMessages.push({
      type: msg.type(),
      text: msg.text()
    });
  });

  try {
    console.log('📍 Navigating to application...');
    await page.goto(APP_URL, { waitUntil: 'networkidle', timeout: 30000 });

    console.log('⏳ Waiting for React to load...');
    await page.waitForSelector('#root', { timeout: 10000 });
    await page.waitForTimeout(2000); // Let React fully render

    // Take initial screenshot
    const homePath = join(SCREENSHOT_DIR, '01-homepage.png');
    await page.screenshot({ path: homePath, fullPage: true });
    console.log(`📸 Screenshot: ${homePath}`);

    // Look for Agents navigation link
    console.log('\n🔍 Looking for Agents navigation...');
    const agentsNavExists = await page.locator('nav a[href*="agent"], nav button:has-text("Agents")').count() > 0;

    if (!agentsNavExists) {
      console.log('❌ Agents navigation not found in sidebar');
      addFinding(
        'critical',
        'functional',
        'Agents navigation link missing from sidebar',
        'Navigation sidebar should contain link/button to Agents page',
        'No Agents link found in navigation sidebar',
        homePath,
        ['Navigate to http://localhost:5173', 'Check sidebar navigation'],
        100
      );
    } else {
      // Click Agents navigation
      console.log('✅ Found Agents navigation, clicking...');
      await page.locator('nav a[href*="agent"], nav button:has-text("Agents")').first().click();
      await page.waitForTimeout(1000);

      // Take screenshot of Agents page
      const agentsDesktopPath = join(SCREENSHOT_DIR, '02-agents-desktop.png');
      await page.screenshot({ path: agentsDesktopPath, fullPage: true });
      console.log(`📸 Screenshot: ${agentsDesktopPath}`);

      // Get page title
      const pageTitle = await page.locator('h1').first().textContent();
      console.log(`📄 Page title: "${pageTitle}"`);

      console.log('\n🔍 Checking required elements...\n');

      // Check for "+ Add Agent" button
      const addAgentBtn = await page.locator('button:has-text("Add Agent")').count();
      console.log(`${addAgentBtn > 0 ? '✅' : '❌'} "+ Add Agent" button`);
      if (addAgentBtn === 0) {
        addFinding(
          'high',
          'functional',
          'Missing "+ Add Agent" button in header',
          'Header should have "+ Add Agent" button in top-right corner',
          'No button with text "Add Agent" found on page',
          agentsDesktopPath,
          ['Navigate to /agents page', 'Look for "+ Add Agent" button in header'],
          95
        );
      }

      // Check for "Active Agents" section
      const activeAgentsSection = await page.locator('text="Active Agents"').count();
      console.log(`${activeAgentsSection > 0 ? '✅' : '❌'} "Active Agents" section`);
      if (activeAgentsSection === 0) {
        addFinding(
          'critical',
          'functional',
          'Missing "Active Agents" section',
          'Page should have "Active Agents" section with "Currently configured Claude instances" subtitle and agent cards',
          'No "Active Agents" heading found on page',
          agentsDesktopPath,
          ['Navigate to /agents page', 'Look for "Active Agents" section'],
          100
        );
      }

      // Check for agent cards with stats
      const agentCards = await page.locator('[class*="agent-card"], [class*="AgentCard"]').count();
      console.log(`${agentCards >= 3 ? '✅' : '❌'} Agent cards (found: ${agentCards}, expected: 3)`);
      if (agentCards < 3) {
        addFinding(
          'high',
          'visual',
          `Insufficient agent cards displayed (found ${agentCards}, expected 3)`,
          'Should display 3 agent cards: Primary Coder, Quick Tasks, Code Review',
          `Only found ${agentCards} agent card(s) on page`,
          agentsDesktopPath,
          ['Navigate to /agents page', 'Count agent cards in Active Agents section'],
          90
        );
      }

      // Check for "Execution Settings" section
      const executionSettings = await page.locator('text="Execution Settings"').count();
      console.log(`${executionSettings > 0 ? '✅' : '❌'} "Execution Settings" section`);
      if (executionSettings === 0) {
        addFinding(
          'critical',
          'functional',
          'Missing "Execution Settings" section',
          'Page should have "Execution Settings" section with Parallel Tasks slider, Auto-Approve toggle, Default Model dropdown, and Cost Limit slider',
          'No "Execution Settings" heading found on page',
          agentsDesktopPath,
          ['Navigate to /agents page', 'Look for "Execution Settings" section'],
          100
        );
      }

      // Check for "Tool Permissions" section
      const toolPermissions = await page.locator('text="Tool Permissions"').count();
      console.log(`${toolPermissions > 0 ? '✅' : '❌'} "Tool Permissions" section`);
      if (toolPermissions === 0) {
        addFinding(
          'critical',
          'functional',
          'Missing "Tool Permissions" section',
          'Page should have "Tool Permissions" section with 6 permission toggles (File Read, File Write, Bash Commands, Web Search, Git Operations, MCP Servers)',
          'No "Tool Permissions" heading found on page',
          agentsDesktopPath,
          ['Navigate to /agents page', 'Look for "Tool Permissions" section'],
          100
        );
      }

      // Test mobile viewport
      console.log('\n📱 Testing mobile viewport (375x667)...');
      await page.setViewportSize({ width: 375, height: 667 });
      await page.waitForTimeout(500);

      const agentsMobilePath = join(SCREENSHOT_DIR, '03-agents-mobile.png');
      await page.screenshot({ path: agentsMobilePath, fullPage: true });
      console.log(`📸 Screenshot: ${agentsMobilePath}`);

      // Check if content is accessible on mobile
      const mobileOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > window.innerWidth;
      });

      if (mobileOverflow) {
        console.log('⚠️  Horizontal overflow detected on mobile');
        addFinding(
          'medium',
          'visual',
          'Horizontal overflow on mobile viewport',
          'Page content should fit within 375px width without horizontal scrolling',
          'Page content exceeds viewport width, causing horizontal scroll',
          agentsMobilePath,
          ['Navigate to /agents page', 'Resize browser to 375x667', 'Check for horizontal scrollbar'],
          85
        );
      }
    }

    // Check console errors
    console.log('\n🔍 Checking console messages...');
    const errors = consoleMessages.filter(m => m.type === 'error');
    const warnings = consoleMessages.filter(m => m.type === 'warning');

    console.log(`   Errors: ${errors.length}`);
    console.log(`   Warnings: ${warnings.length}`);

    if (errors.length > 0) {
      console.log('\n❌ Console Errors:');
      errors.forEach((err, i) => {
        console.log(`   ${i + 1}. ${err.text}`);
      });

      addFinding(
        'high',
        'functional',
        `${errors.length} JavaScript error(s) in browser console`,
        'Page should load without JavaScript errors',
        `Found ${errors.length} console error(s): ${errors.map(e => e.text).join('; ')}`,
        join(SCREENSHOT_DIR, '02-agents-desktop.png'),
        ['Navigate to /agents page', 'Open browser DevTools console', 'Check for errors'],
        90
      );
    }

  } catch (error) {
    console.error('\n💥 Test failed:', error.message);
    const errorScreenshot = join(SCREENSHOT_DIR, '99-error.png');
    try {
      await page.screenshot({ path: errorScreenshot, fullPage: true });
    } catch (e) {
      // Screenshot failed, ignore
    }
    addFinding(
      'critical',
      'functional',
      'E2E test execution failure',
      'Test should complete successfully',
      `Test crashed: ${error.message}`,
      errorScreenshot,
      ['Run E2E test script'],
      100
    );
  } finally {
    await browser.close();
  }

  // Generate report
  console.log('\n' + '='.repeat(60));
  console.log('📊 QA TEST RESULTS');
  console.log('='.repeat(60));
  console.log(`\nFindings: ${findings.length} issues (confidence >= 80)`);
  console.log(`Severity breakdown:`);

  const bySeverity = findings.reduce((acc, f) => {
    acc[f.severity] = (acc[f.severity] || 0) + 1;
    return acc;
  }, {});

  console.log(`  Critical: ${bySeverity.critical || 0}`);
  console.log(`  High: ${bySeverity.high || 0}`);
  console.log(`  Medium: ${bySeverity.medium || 0}`);
  console.log(`  Low: ${bySeverity.low || 0}`);

  if (findings.length > 0) {
    console.log('\n📋 Issues Found:\n');
    findings.forEach(f => {
      console.log(`${f.id} [${f.severity.toUpperCase()}] ${f.title}`);
      console.log(`   Expected: ${f.expected}`);
      console.log(`   Actual: ${f.actual}`);
      console.log(`   Screenshot: ${f.screenshot_path}`);
      console.log('');
    });
  }

  // Save findings JSON
  const report = {
    status: 'complete',
    summary: `Tested Agents page implementation. Found ${findings.length} issue(s) with confidence >= 80`,
    findings,
    verification: {
      scenarios_tested: 8,
      viewports_tested: ['desktop', 'mobile']
    }
  };

  const reportPath = join(SCREENSHOT_DIR, 'findings.json');
  writeFileSync(reportPath, JSON.stringify(report, null, 2));
  console.log(`💾 Full report saved: ${reportPath}`);
  console.log('\n✅ QA test complete!');

  return report;
}

// Run test
testAgentsPage().catch(err => {
  console.error('Fatal error:', err);
  process.exit(1);
});
