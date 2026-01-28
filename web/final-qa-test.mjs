#!/usr/bin/env node
/**
 * Final QA Test - Iteration 2
 * Tests if the Agents page routing has been updated to use AgentsView
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'fs/promises';
import { existsSync } from 'fs';

const OUTPUT_DIR = '/tmp/qa-TASK-613';
await mkdir(OUTPUT_DIR, { recursive: true });

const findings = [];
const verification = {
  previous_issues_verified: []
};

function addFinding(id, severity, confidence, category, title, expected, actual, steps = [], screenshotPath = null) {
  findings.push({
    id,
    severity,
    confidence,
    category,
    title,
    steps_to_reproduce: steps,
    expected,
    actual,
    screenshot_path: screenshotPath || `/tmp/qa-TASK-613/bug-${id}.png`
  });
}

async function takeScreenshot(page, name) {
  const path = `${OUTPUT_DIR}/${name}`;
  await page.screenshot({ path, fullPage: true });
  console.log(`   📸 Screenshot: ${name}`);
  return path;
}

async function checkServer() {
  try {
    const response = await fetch('http://localhost:5173');
    return response.ok;
  } catch {
    return false;
  }
}

console.log('\n🧪 QA Testing: Agents Page - Iteration 2');
console.log('=' .repeat(60));

// Check if dev server is running
console.log('\n📍 Pre-flight: Checking dev server...');
const serverRunning = await checkServer();

if (!serverRunning) {
  console.log('   ❌ Dev server is not running at http://localhost:5173');
  console.log('   Please start with: cd web && bun run dev');

  // Generate report indicating tests couldn't run
  const report = {
    status: 'blocked',
    summary: 'Cannot run tests - dev server is not running',
    findings: [{
      id: 'QA-000',
      severity: 'critical',
      confidence: 100,
      category: 'infrastructure',
      title: 'Dev server not running',
      steps_to_reproduce: ['Attempt to access http://localhost:5173'],
      expected: 'Server responds',
      actual: 'Connection refused',
      screenshot_path: null
    }],
    verification: {
      previous_issues_verified: [
        'QA-001: UNKNOWN - Cannot verify without running server'
      ]
    }
  };

  await writeFile(`${OUTPUT_DIR}/test-results.json`, JSON.stringify(report, null, 2));
  console.log(`\n📁 Report saved: ${OUTPUT_DIR}/test-results.json`);
  process.exit(1);
}

console.log('   ✅ Dev server is running\n');

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1280, height: 720 }
});
const page = await context.newPage();

const consoleMessages = [];
page.on('console', msg => consoleMessages.push({ type: msg.type(), text: msg.text() }));

try {
  // Step 1: Navigate to /agents
  console.log('📍 Step 1: Initial Navigation');
  await page.goto('http://localhost:5173/agents', { waitUntil: 'networkidle', timeout: 10000 });
  await page.waitForTimeout(1000);

  const desktopScreenshot = await takeScreenshot(page, 'desktop-initial-load.png');
  console.log('   ✅ Page loaded\n');

  // Step 2: Check if QA-001 is fixed (correct component loaded)
  console.log('📍 Step 2: Verify QA-001 Fix (Correct Component)');

  // Look for signs of OLD component (sub-agent definitions)
  const oldComponentSignals = [
    await page.getByText('Sub-agent definitions', { exact: false }).count(),
    await page.locator('.env-scope-tabs').count(), // Project/Global tabs
  ];

  // Look for signs of NEW component (AgentsView)
  const newComponentSignals = {
    correctTitle: await page.locator('h1').filter({ hasText: 'Agents' }).count(),
    correctSubtitle: await page.getByText('Configure Claude models and execution settings').count(),
    addAgentButton: await page.getByRole('button', { name: /add agent/i }).count(),
    activeAgentsSection: await page.getByText('Active Agents', { exact: false }).count(),
    executionSettingsSection: await page.getByText('Execution Settings', { exact: false }).count(),
    toolPermissionsSection: await page.getByText('Tool Permissions', { exact: false }).count()
  };

  const hasOldComponent = oldComponentSignals.some(count => count > 0);
  const hasNewComponent = Object.values(newComponentSignals).every(count => count > 0);

  if (hasOldComponent) {
    await takeScreenshot(page, 'bug-QA-004.png');
    addFinding(
      'QA-004',
      'critical',
      100,
      'functional',
      'Routing still points to old Agents component',
      'Route /agents should load AgentsView component with: h1 "Agents", subtitle "Configure Claude models and execution settings", "+ Add Agent" button, Active Agents section, Execution Settings section, Tool Permissions section',
      'Page loads old Agents component showing "Sub-agent definitions" and Project/Global tabs',
      [
        'Navigate to http://localhost:5173/agents',
        'Observe page header and content'
      ],
      `/tmp/qa-TASK-613/bug-QA-004.png`
    );

    verification.previous_issues_verified.push('QA-001: NOT FIXED - Route still uses old component');
    console.log('   ❌ QA-001 NOT FIXED: Old component still loaded');
    console.log('   ❌ Root cause: routes.tsx line 19 imports old Agents component');
    console.log('   ❌ Fix needed: Import AgentsView from @/components/agents/AgentsView');
  } else if (hasNewComponent) {
    verification.previous_issues_verified.push('QA-001: FIXED - AgentsView now loaded');
    console.log('   ✅ QA-001 FIXED: New AgentsView component is loaded');
  } else {
    await takeScreenshot(page, 'bug-QA-005.png');
    addFinding(
      'QA-005',
      'critical',
      95,
      'functional',
      'Unknown component state - neither old nor new component detected',
      'Should show either old or new component',
      'Page shows unexpected content',
      ['Navigate to http://localhost:5173/agents'],
      `/tmp/qa-TASK-613/bug-QA-005.png`
    );
    verification.previous_issues_verified.push('QA-001: UNKNOWN - Unexpected page state');
    console.log('   ⚠️  QA-001 UNKNOWN: Neither old nor new component detected');
  }

  console.log('');

  // Step 3: If new component is loaded, test functionality
  if (hasNewComponent) {
    console.log('📍 Step 3: Testing AgentsView Functionality');

    // Test header elements
    console.log('   Testing header...');
    if (newComponentSignals.correctTitle === 0) {
      addFinding('QA-006', 'high', 90, 'visual', 'Missing h1 title', 'h1 with "Agents"', 'Title not found or wrong level', ['Navigate to /agents', 'Check page header']);
    }
    if (newComponentSignals.addAgentButton === 0) {
      addFinding('QA-007', 'high', 90, 'functional', 'Missing "+ Add Agent" button', 'Button in top-right', 'Button not found', ['Navigate to /agents', 'Check top-right of header']);
    }

    // Test sections
    console.log('   Testing sections...');
    if (newComponentSignals.activeAgentsSection === 0) {
      addFinding('QA-008', 'high', 90, 'functional', 'Missing "Active Agents" section', 'Section with agent cards', 'Section not found', ['Navigate to /agents', 'Scroll to find Active Agents']);
    }
    if (newComponentSignals.executionSettingsSection === 0) {
      addFinding('QA-009', 'high', 90, 'functional', 'Missing "Execution Settings" section', 'Section with sliders and toggles', 'Section not found', ['Navigate to /agents', 'Scroll to find Execution Settings']);
    }
    if (newComponentSignals.toolPermissionsSection === 0) {
      addFinding('QA-010', 'high', 90, 'functional', 'Missing "Tool Permissions" section', 'Section with permission toggles', 'Section not found', ['Navigate to /agents', 'Scroll to find Tool Permissions']);
    }

    // Test agent cards
    const agentCards = await page.locator('.agent-card').count();
    console.log(`   Found ${agentCards} agent card(s)`);
    if (agentCards === 0) {
      addFinding('QA-011', 'medium', 85, 'functional', 'No agent cards displayed', 'Should show at least some agent cards or empty state', 'No cards found', ['Navigate to /agents', 'Check Active Agents section']);
    }

    // Test sliders
    const sliders = await page.locator('input[type="range"]').count();
    console.log(`   Found ${sliders} slider(s)`);
    if (sliders < 2) {
      addFinding('QA-012', 'high', 85, 'functional', 'Missing sliders in Execution Settings', 'Should have Parallel Tasks and Cost Limit sliders', `Only found ${sliders} slider(s)`, ['Navigate to /agents', 'Check Execution Settings section']);
    }

    // Test toggles
    const toggles = await page.locator('[role="switch"]').count();
    console.log(`   Found ${toggles} toggle(s)`);
    if (toggles < 7) { // 1 Auto-Approve + 6 Tool Permissions
      addFinding('QA-013', 'high', 85, 'functional', 'Missing toggles', 'Should have 1 Auto-Approve + 6 Tool Permissions = 7 toggles', `Only found ${toggles} toggle(s)`, ['Navigate to /agents', 'Count all toggles on page']);
    }

    console.log('   ✅ Functionality testing complete\n');
  }

  // Step 4: Mobile testing
  console.log('📍 Step 4: Mobile Viewport Testing');
  await page.setViewportSize({ width: 375, height: 667 });
  await page.waitForTimeout(1000);

  const mobileScreenshot = await takeScreenshot(page, 'mobile-overview.png');

  const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
  if (bodyWidth > 375) {
    await takeScreenshot(page, 'bug-QA-014.png');
    addFinding(
      'QA-014',
      'high',
      90,
      'responsive',
      'Horizontal scrolling on mobile',
      'Page should fit within 375px width',
      `Page width is ${bodyWidth}px`,
      ['Resize browser to 375x667', 'Check if horizontal scroll appears'],
      `/tmp/qa-TASK-613/bug-QA-014.png`
    );
    console.log(`   ❌ Horizontal scroll detected (width: ${bodyWidth}px)`);
  } else {
    console.log('   ✅ No horizontal scrolling');
  }
  console.log('');

  // Step 5: Console errors
  console.log('📍 Step 5: Console Messages');
  const errors = consoleMessages.filter(m => m.type === 'error');
  const warnings = consoleMessages.filter(m => m.type === 'warning');

  if (errors.length > 0) {
    console.log(`   ❌ ${errors.length} console error(s):`);
    errors.slice(0, 3).forEach(err => console.log(`      - ${err.text.substring(0, 100)}`));

    addFinding(
      'QA-015',
      'high',
      95,
      'functional',
      'JavaScript console errors present',
      'Page should load without errors',
      `${errors.length} console error(s): ${errors.map(e => e.text).join('; ')}`,
      ['Open browser DevTools', 'Navigate to /agents', 'Check Console tab']
    );
  } else {
    console.log('   ✅ No console errors');
  }

  if (warnings.length > 0) {
    console.log(`   ⚠️  ${warnings.length} console warning(s)`);
  }

} catch (error) {
  console.error('\n❌ Test execution failed:', error.message);
  addFinding(
    'QA-016',
    'critical',
    100,
    'infrastructure',
    'Test execution error',
    'Tests should complete without errors',
    `Error: ${error.message}\n${error.stack}`,
    ['Run QA test suite']
  );
} finally {
  await browser.close();

  // Generate final report
  const report = {
    status: findings.length === 0 ? 'complete' : 'complete',
    summary: findings.length === 0
      ? 'All tests passed! AgentsView component is properly implemented and routed.'
      : `Found ${findings.length} issue(s). ${verification.previous_issues_verified.includes('NOT FIXED') ? 'QA-001 still present - routing not updated.' : 'Issues found in implementation.'}`,
    findings,
    verification
  };

  await writeFile(`${OUTPUT_DIR}/test-results.json`, JSON.stringify(report, null, 2));

  console.log('\n' + '='.repeat(60));
  console.log(`📊 Test Complete: ${findings.length} finding(s)`);
  console.log('='.repeat(60));

  if (findings.length > 0) {
    console.log('\n🐛 Findings:');
    findings.forEach(f => {
      const emoji = {
        critical: '🔴',
        high: '🟠',
        medium: '🟡',
        low: '⚪'
      }[f.severity] || '⚪';
      console.log(`${emoji} [${f.severity.toUpperCase()}] ${f.id}: ${f.title}`);
      console.log(`   Confidence: ${f.confidence}%`);
    });
  } else {
    console.log('\n✅ No issues found - page matches requirements!');
  }

  console.log(`\n📁 Output: ${OUTPUT_DIR}/`);
  console.log('   - test-results.json');
  console.log('   - desktop-initial-load.png');
  console.log('   - mobile-overview.png');
  if (findings.length > 0) {
    console.log('   - bug-*.png (screenshots)');
  }

  console.log('\n📋 Verification Status:');
  verification.previous_issues_verified.forEach(v => console.log(`   ${v}`));
}
