#!/usr/bin/env node

/**
 * QA Verification Script for Agents Page (TASK-613)
 *
 * This script verifies that the previous QA issues have been fixed.
 * It requires the dev server to be running on http://localhost:5173
 */

import { chromium } from 'playwright';
import { writeFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const WORKTREE_DIR = join(__dirname, '..');
const BASE_URL = 'http://localhost:5173';

const findings = [];
const consoleMessages = [];

async function verifyAgentsPage() {
  console.log('=== Agents Page QA Verification ===\n');
  console.log('Connecting to:', BASE_URL);
  console.log('');

  const browser = await chromium.launch({ headless: true });

  try {
    // ============================================================================
    // DESKTOP TESTING (1920x1080)
    // ============================================================================
    console.log('=== DESKTOP TESTING (1920x1080) ===\n');

    const desktopContext = await browser.newContext({
      viewport: { width: 1920, height: 1080 }
    });
    const page = await desktopContext.newPage();

    // Capture console messages
    page.on('console', msg => {
      consoleMessages.push({
        type: msg.type(),
        text: msg.text(),
        location: msg.location()
      });
    });

    // Navigate to the home page first
    console.log('Navigating to', BASE_URL);
    await page.goto(BASE_URL, { waitUntil: 'networkidle', timeout: 30000 });
    await page.waitForTimeout(1500);

    // Click on Agents link in sidebar
    console.log('Clicking Agents link...');
    const agentsLink = page.locator('[href="/agents"]').first();
    await agentsLink.click({ timeout: 5000 });
    await page.waitForURL('**/agents', { timeout: 5000 });
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1500);

    // Take full desktop screenshot
    const desktopScreenshot = join(WORKTREE_DIR, 'agents-page-desktop-full.png');
    await page.screenshot({
      path: desktopScreenshot,
      fullPage: true
    });
    console.log('✓ Desktop screenshot saved');

    // ============================================================================
    // QA-001 & QA-002: Page title and subtitle
    // ============================================================================
    console.log('\n--- Testing QA-001 & QA-002: Page header ---');

    const pageTitle = await page.locator('h1, h2').first().textContent();
    console.log('Page title:', pageTitle);

    const subtitleLocator = page.locator('text=/Configure.*Claude.*models.*execution.*settings/i');
    const subtitle = await subtitleLocator.textContent().catch(() => null);
    console.log('Page subtitle:', subtitle);

    if (!subtitle || !subtitle.toLowerCase().includes('configure claude models and execution settings')) {
      findings.push({
        id: 'QA-002',
        status: 'STILL_PRESENT',
        severity: 'high',
        confidence: 95,
        title: 'Incorrect page subtitle',
        expected: 'Configure Claude models and execution settings',
        actual: subtitle || 'Not found',
        screenshot: 'agents-page-desktop-full.png'
      });
      console.log('✗ QA-002: STILL PRESENT - Subtitle incorrect or missing');
    } else {
      console.log('✓ QA-002: FIXED - Subtitle correct');
    }

    // ============================================================================
    // QA-003: "+ Add Agent" button
    // ============================================================================
    console.log('\n--- Testing QA-003: + Add Agent button ---');

    const addAgentButton = page.locator('button:has-text("Add Agent")');
    const buttonVisible = await addAgentButton.isVisible().catch(() => false);
    console.log('Add Agent button visible:', buttonVisible);

    if (!buttonVisible) {
      findings.push({
        id: 'QA-003',
        status: 'STILL_PRESENT',
        severity: 'high',
        confidence: 100,
        title: 'Missing "+ Add Agent" button in header',
        expected: '+ Add Agent button visible in page header',
        actual: 'Button not found',
        screenshot: 'agents-page-desktop-full.png'
      });
      console.log('✗ QA-003: STILL PRESENT - Button not found');
    } else {
      console.log('✓ QA-003: FIXED - Add Agent button present');
    }

    // ============================================================================
    // QA-004: "Active Agents" section with 3 agent cards
    // ============================================================================
    console.log('\n--- Testing QA-004: Active Agents section ---');

    const activeAgentsHeading = page.locator('text=/^Active Agents$/i');
    const hasActiveAgentsSection = await activeAgentsHeading.isVisible().catch(() => false);
    console.log('Active Agents heading visible:', hasActiveAgentsSection);

    if (!hasActiveAgentsSection) {
      findings.push({
        id: 'QA-004',
        status: 'STILL_PRESENT',
        severity: 'critical',
        confidence: 100,
        title: 'Missing "Active Agents" section',
        expected: 'Section with heading "Active Agents" and 3 agent cards',
        actual: 'Section not found',
        screenshot: 'agents-page-desktop-full.png'
      });
      console.log('✗ QA-004: STILL PRESENT - Active Agents section not found');
    } else {
      console.log('✓ Active Agents section found');

      // Check for the 3 specific agent cards
      const primaryCoder = await page.locator('text=/Primary Coder/i').isVisible().catch(() => false);
      const quickTasks = await page.locator('text=/Quick Tasks/i').isVisible().catch(() => false);
      const codeReview = await page.locator('text=/Code Review/i').isVisible().catch(() => false);

      console.log('  - Primary Coder:', primaryCoder ? '✓' : '✗');
      console.log('  - Quick Tasks:', quickTasks ? '✓' : '✗');
      console.log('  - Code Review:', codeReview ? '✓' : '✗');

      if (!primaryCoder || !quickTasks || !codeReview) {
        const found = [];
        if (primaryCoder) found.push('Primary Coder');
        if (quickTasks) found.push('Quick Tasks');
        if (codeReview) found.push('Code Review');

        findings.push({
          id: 'QA-004',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 95,
          title: 'Missing one or more agent cards',
          expected: 'All 3 agents: Primary Coder, Quick Tasks, Code Review',
          actual: found.length > 0 ? `Found: ${found.join(', ')}` : 'No agent cards found',
          screenshot: 'agents-page-desktop-full.png'
        });
        console.log('✗ QA-004: STILL PRESENT - Missing agent card(s)');
      } else {
        console.log('✓ QA-004: FIXED - All 3 agent cards present');
      }
    }

    // ============================================================================
    // QA-005: "Execution Settings" section with 4 controls
    // ============================================================================
    console.log('\n--- Testing QA-005: Execution Settings section ---');

    const executionSettingsHeading = page.locator('text=/^Execution Settings$/i');
    const hasExecutionSettings = await executionSettingsHeading.isVisible().catch(() => false);
    console.log('Execution Settings heading visible:', hasExecutionSettings);

    if (!hasExecutionSettings) {
      findings.push({
        id: 'QA-005',
        status: 'STILL_PRESENT',
        severity: 'critical',
        confidence: 100,
        title: 'Missing "Execution Settings" section',
        expected: 'Section with Parallel Tasks, Auto-Approve, Default Model, Cost Limit',
        actual: 'Section not found',
        screenshot: 'agents-page-desktop-full.png'
      });
      console.log('✗ QA-005: STILL PRESENT - Execution Settings section not found');
    } else {
      console.log('✓ Execution Settings section found');

      // Check for the 4 controls
      const parallelTasks = await page.locator('text=/Parallel Tasks/i').isVisible().catch(() => false);
      const autoApprove = await page.locator('text=/Auto-Approve/i').isVisible().catch(() => false);
      const defaultModel = await page.locator('text=/Default Model/i').isVisible().catch(() => false);
      const costLimit = await page.locator('text=/Cost Limit/i').isVisible().catch(() => false);

      console.log('  - Parallel Tasks:', parallelTasks ? '✓' : '✗');
      console.log('  - Auto-Approve:', autoApprove ? '✓' : '✗');
      console.log('  - Default Model:', defaultModel ? '✓' : '✗');
      console.log('  - Cost Limit:', costLimit ? '✓' : '✗');

      if (!parallelTasks || !autoApprove || !defaultModel || !costLimit) {
        const found = [];
        if (parallelTasks) found.push('Parallel Tasks');
        if (autoApprove) found.push('Auto-Approve');
        if (defaultModel) found.push('Default Model');
        if (costLimit) found.push('Cost Limit');

        findings.push({
          id: 'QA-005',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 95,
          title: 'Missing one or more execution settings controls',
          expected: 'All 4 controls: Parallel Tasks, Auto-Approve, Default Model, Cost Limit',
          actual: found.length > 0 ? `Found: ${found.join(', ')}` : 'No controls found',
          screenshot: 'agents-page-desktop-full.png'
        });
        console.log('✗ QA-005: STILL PRESENT - Missing control(s)');
      } else {
        console.log('✓ QA-005: FIXED - All execution settings controls present');
      }
    }

    // ============================================================================
    // QA-006: "Tool Permissions" section with 6 toggles
    // ============================================================================
    console.log('\n--- Testing QA-006: Tool Permissions section ---');

    const toolPermissionsHeading = page.locator('text=/^Tool Permissions$/i');
    const hasToolPermissions = await toolPermissionsHeading.isVisible().catch(() => false);
    console.log('Tool Permissions heading visible:', hasToolPermissions);

    if (!hasToolPermissions) {
      findings.push({
        id: 'QA-006',
        status: 'STILL_PRESENT',
        severity: 'critical',
        confidence: 100,
        title: 'Missing "Tool Permissions" section',
        expected: 'Section with 6 permission toggles',
        actual: 'Section not found',
        screenshot: 'agents-page-desktop-full.png'
      });
      console.log('✗ QA-006: STILL PRESENT - Tool Permissions section not found');
    } else {
      console.log('✓ Tool Permissions section found');

      // Check for the 6 permissions
      const fileRead = await page.locator('text=/^File Read$/i').isVisible().catch(() => false);
      const fileWrite = await page.locator('text=/^File Write$/i').isVisible().catch(() => false);
      const bashCommands = await page.locator('text=/Bash.*Command/i').isVisible().catch(() => false);
      const webSearch = await page.locator('text=/^Web Search$/i').isVisible().catch(() => false);
      const gitOps = await page.locator('text=/Git.*Operation/i').isVisible().catch(() => false);
      const mcpServers = await page.locator('text=/MCP.*Server/i').isVisible().catch(() => false);

      console.log('  - File Read:', fileRead ? '✓' : '✗');
      console.log('  - File Write:', fileWrite ? '✓' : '✗');
      console.log('  - Bash Commands:', bashCommands ? '✓' : '✗');
      console.log('  - Web Search:', webSearch ? '✓' : '✗');
      console.log('  - Git Operations:', gitOps ? '✓' : '✗');
      console.log('  - MCP Servers:', mcpServers ? '✓' : '✗');

      const foundCount = [fileRead, fileWrite, bashCommands, webSearch, gitOps, mcpServers].filter(Boolean).length;

      if (foundCount < 6) {
        const found = [];
        if (fileRead) found.push('File Read');
        if (fileWrite) found.push('File Write');
        if (bashCommands) found.push('Bash Commands');
        if (webSearch) found.push('Web Search');
        if (gitOps) found.push('Git Operations');
        if (mcpServers) found.push('MCP Servers');

        findings.push({
          id: 'QA-006',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 90,
          title: 'Missing one or more tool permission toggles',
          expected: '6 toggles: File Read, File Write, Bash Commands, Web Search, Git Operations, MCP Servers',
          actual: `Found ${foundCount}/6: ${found.join(', ')}`,
          screenshot: 'agents-page-desktop-full.png'
        });
        console.log(`✗ QA-006: STILL PRESENT - Found only ${foundCount}/6 permissions`);
      } else {
        console.log('✓ QA-006: FIXED - All 6 tool permissions present');
      }
    }

    await desktopContext.close();

    // ============================================================================
    // MOBILE TESTING (375x667)
    // ============================================================================
    console.log('\n=== MOBILE TESTING (375x667) ===\n');

    const mobileContext = await browser.newContext({
      viewport: { width: 375, height: 667 }
    });
    const mobilePage = await mobileContext.newPage();

    await mobilePage.goto(BASE_URL, { waitUntil: 'networkidle' });
    await mobilePage.waitForTimeout(1500);

    // Click Agents link
    const mobileAgentsLink = mobilePage.locator('[href="/agents"]').first();
    await mobileAgentsLink.click({ timeout: 5000 });
    await mobilePage.waitForURL('**/agents');
    await mobilePage.waitForTimeout(1500);

    // Take mobile screenshot
    const mobileScreenshot = join(WORKTREE_DIR, 'agents-page-mobile-full.png');
    await mobilePage.screenshot({
      path: mobileScreenshot,
      fullPage: true
    });
    console.log('✓ Mobile screenshot saved');

    // Check for horizontal scrolling
    const bodyWidth = await mobilePage.evaluate(() => document.body.scrollWidth);
    const viewportWidth = 375;
    console.log('Body width:', bodyWidth, '| Viewport width:', viewportWidth);

    if (bodyWidth > viewportWidth + 1) { // Allow 1px tolerance
      findings.push({
        id: 'QA-MOBILE-001',
        status: 'NEW_ISSUE',
        severity: 'medium',
        confidence: 100,
        title: 'Horizontal scrolling on mobile',
        expected: 'Content fits within 375px viewport',
        actual: `Body width is ${bodyWidth}px, causing horizontal scroll`,
        screenshot: 'agents-page-mobile-full.png'
      });
      console.log('✗ Horizontal scrolling detected');
    } else {
      console.log('✓ No horizontal scrolling on mobile');
    }

    await mobileContext.close();

    // ============================================================================
    // CONSOLE ERRORS
    // ============================================================================
    console.log('\n=== CONSOLE MESSAGES ===\n');

    const errors = consoleMessages.filter(m => m.type === 'error');
    const warnings = consoleMessages.filter(m => m.type === 'warning');

    console.log(`Errors: ${errors.length}, Warnings: ${warnings.length}`);

    if (errors.length > 0) {
      console.log('\nConsole Errors:');
      errors.forEach((err, i) => {
        console.log(`  ${i + 1}. ${err.text}`);
      });

      findings.push({
        id: 'QA-CONSOLE-001',
        status: 'NEW_ISSUE',
        severity: 'high',
        confidence: 100,
        title: 'Console errors detected',
        expected: 'No console errors',
        actual: `${errors.length} error(s)`,
        screenshot: 'agents-page-desktop-full.png'
      });
    }

    // ============================================================================
    // GENERATE SUMMARY
    // ============================================================================
    console.log('\n=== VERIFICATION SUMMARY ===\n');

    const originalIssues = ['QA-001', 'QA-002', 'QA-003', 'QA-004', 'QA-005', 'QA-006'];
    const stillPresent = findings.filter(f => originalIssues.includes(f.id) && f.status === 'STILL_PRESENT');
    const fixed = originalIssues.filter(id => !findings.some(f => f.id === id && f.status === 'STILL_PRESENT'));
    const newIssues = findings.filter(f => !originalIssues.includes(f.id));

    console.log(`FIXED: ${fixed.length}/${originalIssues.length}`);
    fixed.forEach(id => console.log(`  ✓ ${id}: FIXED`));

    if (stillPresent.length > 0) {
      console.log(`\nSTILL PRESENT: ${stillPresent.length}`);
      stillPresent.forEach(f => console.log(`  ✗ ${f.id}: ${f.title} [${f.severity}]`));
    }

    if (newIssues.length > 0) {
      console.log(`\nNEW ISSUES: ${newIssues.length}`);
      newIssues.forEach(f => console.log(`  ! ${f.id}: ${f.title} [${f.severity}]`));
    }

    // Save report
    const report = {
      timestamp: new Date().toISOString(),
      summary: {
        totalOriginalIssues: originalIssues.length,
        fixed: fixed.length,
        stillPresent: stillPresent.length,
        newIssues: newIssues.length
      },
      fixedIssues: fixed.map(id => ({ id, status: 'FIXED' })),
      findings: findings,
      consoleErrors: errors.slice(0, 10),
      consoleWarnings: warnings.slice(0, 10)
    };

    const reportPath = join(WORKTREE_DIR, 'verification-report.json');
    writeFileSync(reportPath, JSON.stringify(report, null, 2));
    console.log('\n✓ Report saved: verification-report.json');
    console.log('✓ Desktop screenshot: agents-page-desktop-full.png');
    console.log('✓ Mobile screenshot: agents-page-mobile-full.png');

    return { success: stillPresent.length === 0, findings, fixed, stillPresent, newIssues };

  } catch (error) {
    console.error('\n✗ Test failed:', error.message);
    throw error;
  } finally {
    await browser.close();
  }
}

// Run verification
verifyAgentsPage()
  .then(({ success }) => {
    process.exit(success ? 0 : 1);
  })
  .catch(error => {
    console.error('Fatal error:', error);
    process.exit(1);
  });
