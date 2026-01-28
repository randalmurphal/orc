#!/usr/bin/env node

import { chromium } from 'playwright';
import { writeFileSync } from 'fs';

const BASE_URL = 'http://localhost:5173';
const SCREENSHOT_DIR = '/home/randy/repos/orc/.orc/worktrees/orc-TASK-613';

async function testAgentsPage() {
  const browser = await chromium.launch();
  const findings = [];
  const consoleMessages = [];

  try {
    // Desktop testing (1920x1080)
    console.log('\n=== DESKTOP TESTING (1920x1080) ===\n');
    const desktopContext = await browser.newContext({
      viewport: { width: 1920, height: 1080 }
    });
    const desktopPage = await desktopContext.newPage();

    // Capture console messages
    desktopPage.on('console', msg => {
      consoleMessages.push({
        type: msg.type(),
        text: msg.text(),
        location: msg.location()
      });
    });

    // Navigate to the Agents page
    console.log('Navigating to', BASE_URL);
    await desktopPage.goto(BASE_URL, { waitUntil: 'networkidle' });

    // Wait a bit for the page to load
    await desktopPage.waitForTimeout(2000);

    // Check if we need to click the Agents link in sidebar
    const agentsLink = await desktopPage.locator('text=Agents').first();
    if (await agentsLink.isVisible()) {
      console.log('Clicking Agents link in sidebar...');
      await agentsLink.click();
      await desktopPage.waitForTimeout(1500);
    }

    // Take full page screenshot
    await desktopPage.screenshot({
      path: `${SCREENSHOT_DIR}/agents-page-desktop-full.png`,
      fullPage: true
    });
    console.log('✓ Desktop screenshot saved: agents-page-desktop-full.png');

    // === TEST QA-001: Page shows model execution settings, not sub-agent config ===
    console.log('\n--- Testing QA-001: Correct feature implementation ---');
    const pageTitle = await desktopPage.locator('h1, [class*="text-2xl"], [class*="text-3xl"]').first().textContent();
    console.log('Page title:', pageTitle);

    // === TEST QA-002: Page subtitle ===
    console.log('\n--- Testing QA-002: Page subtitle ---');
    const subtitle = await desktopPage.locator('text=/Configure.*Claude.*models/i').first().textContent().catch(() => null);
    console.log('Page subtitle:', subtitle);

    if (!subtitle || !subtitle.includes('Configure Claude models and execution settings')) {
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
    } else {
      console.log('✓ QA-002: FIXED - Subtitle matches reference');
    }

    // === TEST QA-003: "+ Add Agent" button ===
    console.log('\n--- Testing QA-003: + Add Agent button ---');
    const addAgentButton = await desktopPage.locator('button:has-text("Add Agent"), button:has-text("+ Add Agent")').count();
    console.log('Add Agent buttons found:', addAgentButton);

    if (addAgentButton === 0) {
      findings.push({
        id: 'QA-003',
        status: 'STILL_PRESENT',
        severity: 'high',
        confidence: 100,
        title: 'Missing "+ Add Agent" button in header',
        expected: '+ Add Agent button in top-right corner',
        actual: 'Button not found',
        screenshot: 'agents-page-desktop-full.png'
      });
    } else {
      console.log('✓ QA-003: FIXED - Add Agent button present');
    }

    // === TEST QA-004: Active Agents section ===
    console.log('\n--- Testing QA-004: Active Agents section ---');
    const activeAgentsSection = await desktopPage.locator('text=/Active Agents/i').first().isVisible().catch(() => false);
    console.log('Active Agents section visible:', activeAgentsSection);

    if (!activeAgentsSection) {
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
    } else {
      console.log('✓ Active Agents section found');

      // Check for agent cards
      const agentCards = await desktopPage.locator('[class*="agent"], [class*="card"]').count();
      console.log('Agent-like cards found:', agentCards);

      // Check for specific agents
      const primaryCoder = await desktopPage.locator('text=/Primary Coder/i').isVisible().catch(() => false);
      const quickTasks = await desktopPage.locator('text=/Quick Tasks/i').isVisible().catch(() => false);
      const codeReview = await desktopPage.locator('text=/Code Review/i').isVisible().catch(() => false);

      console.log('Primary Coder found:', primaryCoder);
      console.log('Quick Tasks found:', quickTasks);
      console.log('Code Review found:', codeReview);

      if (!primaryCoder || !quickTasks || !codeReview) {
        findings.push({
          id: 'QA-004',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 95,
          title: 'Missing one or more agent cards',
          expected: 'All 3 agents: Primary Coder, Quick Tasks, Code Review',
          actual: `Found: ${primaryCoder ? 'Primary Coder' : ''} ${quickTasks ? 'Quick Tasks' : ''} ${codeReview ? 'Code Review' : ''}`.trim() || 'None',
          screenshot: 'agents-page-desktop-full.png'
        });
      } else {
        console.log('✓ QA-004: FIXED - All 3 agent cards present');
      }
    }

    // === TEST QA-005: Execution Settings section ===
    console.log('\n--- Testing QA-005: Execution Settings section ---');
    const executionSettings = await desktopPage.locator('text=/Execution Settings/i').first().isVisible().catch(() => false);
    console.log('Execution Settings section visible:', executionSettings);

    if (!executionSettings) {
      findings.push({
        id: 'QA-005',
        status: 'STILL_PRESENT',
        severity: 'critical',
        confidence: 100,
        title: 'Missing "Execution Settings" section',
        expected: 'Section with Parallel Tasks, Auto-Approve, Default Model, Cost Limit controls',
        actual: 'Section not found',
        screenshot: 'agents-page-desktop-full.png'
      });
    } else {
      console.log('✓ Execution Settings section found');

      // Check for individual controls
      const parallelTasks = await desktopPage.locator('text=/Parallel Tasks/i').isVisible().catch(() => false);
      const autoApprove = await desktopPage.locator('text=/Auto-Approve/i').isVisible().catch(() => false);
      const defaultModel = await desktopPage.locator('text=/Default Model/i').isVisible().catch(() => false);
      const costLimit = await desktopPage.locator('text=/Cost Limit/i').isVisible().catch(() => false);

      console.log('Parallel Tasks control:', parallelTasks);
      console.log('Auto-Approve control:', autoApprove);
      console.log('Default Model control:', defaultModel);
      console.log('Cost Limit control:', costLimit);

      if (!parallelTasks || !autoApprove || !defaultModel || !costLimit) {
        findings.push({
          id: 'QA-005',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 95,
          title: 'Missing one or more execution settings controls',
          expected: 'All 4 controls: Parallel Tasks, Auto-Approve, Default Model, Cost Limit',
          actual: `Found: ${parallelTasks ? 'Parallel Tasks' : ''} ${autoApprove ? 'Auto-Approve' : ''} ${defaultModel ? 'Default Model' : ''} ${costLimit ? 'Cost Limit' : ''}`.trim() || 'None',
          screenshot: 'agents-page-desktop-full.png'
        });
      } else {
        console.log('✓ QA-005: FIXED - All execution settings controls present');
      }
    }

    // === TEST QA-006: Tool Permissions section ===
    console.log('\n--- Testing QA-006: Tool Permissions section ---');
    const toolPermissions = await desktopPage.locator('text=/Tool Permissions/i').first().isVisible().catch(() => false);
    console.log('Tool Permissions section visible:', toolPermissions);

    if (!toolPermissions) {
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
    } else {
      console.log('✓ Tool Permissions section found');

      // Check for specific permissions
      const fileRead = await desktopPage.locator('text=/File Read/i').isVisible().catch(() => false);
      const fileWrite = await desktopPage.locator('text=/File Write/i').isVisible().catch(() => false);
      const bashCommands = await desktopPage.locator('text=/Bash.*Command/i').isVisible().catch(() => false);
      const webSearch = await desktopPage.locator('text=/Web Search/i').isVisible().catch(() => false);
      const gitOps = await desktopPage.locator('text=/Git.*Operation/i').isVisible().catch(() => false);
      const mcpServers = await desktopPage.locator('text=/MCP.*Server/i').isVisible().catch(() => false);

      console.log('File Read permission:', fileRead);
      console.log('File Write permission:', fileWrite);
      console.log('Bash Commands permission:', bashCommands);
      console.log('Web Search permission:', webSearch);
      console.log('Git Operations permission:', gitOps);
      console.log('MCP Servers permission:', mcpServers);

      const foundCount = [fileRead, fileWrite, bashCommands, webSearch, gitOps, mcpServers].filter(Boolean).length;

      if (foundCount < 6) {
        findings.push({
          id: 'QA-006',
          status: 'STILL_PRESENT',
          severity: 'critical',
          confidence: 90,
          title: 'Missing one or more tool permission toggles',
          expected: '6 toggles: File Read, File Write, Bash Commands, Web Search, Git Operations, MCP Servers',
          actual: `Found ${foundCount}/6 permissions`,
          screenshot: 'agents-page-desktop-full.png'
        });
      } else {
        console.log('✓ QA-006: FIXED - All 6 tool permissions present');
      }
    }

    await desktopContext.close();

    // === MOBILE TESTING (375x667) ===
    console.log('\n=== MOBILE TESTING (375x667) ===\n');
    const mobileContext = await browser.newContext({
      viewport: { width: 375, height: 667 }
    });
    const mobilePage = await mobileContext.newPage();

    await mobilePage.goto(BASE_URL, { waitUntil: 'networkidle' });
    await mobilePage.waitForTimeout(2000);

    // Check if we need to click the Agents link
    const mobileAgentsLink = await mobilePage.locator('text=Agents').first();
    if (await mobileAgentsLink.isVisible()) {
      console.log('Clicking Agents link in mobile view...');
      await mobileAgentsLink.click();
      await mobilePage.waitForTimeout(1500);
    }

    await mobilePage.screenshot({
      path: `${SCREENSHOT_DIR}/agents-page-mobile-full.png`,
      fullPage: true
    });
    console.log('✓ Mobile screenshot saved: agents-page-mobile-full.png');

    // Check for horizontal scrolling
    const bodyWidth = await mobilePage.evaluate(() => document.body.scrollWidth);
    const viewportWidth = 375;
    console.log('Body width:', bodyWidth, '| Viewport width:', viewportWidth);

    if (bodyWidth > viewportWidth) {
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
    } else {
      console.log('✓ No horizontal scrolling on mobile');
    }

    await mobileContext.close();

    // === CONSOLE ERRORS ===
    console.log('\n=== CONSOLE MESSAGES ===\n');
    const errors = consoleMessages.filter(m => m.type === 'error');
    const warnings = consoleMessages.filter(m => m.type === 'warning');

    console.log(`Found ${errors.length} errors, ${warnings.length} warnings`);

    if (errors.length > 0) {
      console.log('\nErrors:');
      errors.forEach((err, i) => {
        console.log(`${i + 1}. ${err.text}`);
      });

      findings.push({
        id: 'QA-CONSOLE-001',
        status: 'NEW_ISSUE',
        severity: 'high',
        confidence: 100,
        title: 'Console errors detected',
        expected: 'No console errors',
        actual: `${errors.length} console error(s): ${errors.map(e => e.text).join('; ')}`,
        screenshot: 'agents-page-desktop-full.png'
      });
    }

    if (warnings.length > 0) {
      console.log('\nWarnings (first 5):');
      warnings.slice(0, 5).forEach((warn, i) => {
        console.log(`${i + 1}. ${warn.text}`);
      });
    }

    // === SUMMARY ===
    console.log('\n=== VERIFICATION SUMMARY ===\n');

    const originalIssues = ['QA-001', 'QA-002', 'QA-003', 'QA-004', 'QA-005', 'QA-006'];
    const stillPresent = findings.filter(f => originalIssues.includes(f.id) && f.status === 'STILL_PRESENT');
    const fixed = originalIssues.filter(id => !findings.some(f => f.id === id && f.status === 'STILL_PRESENT'));
    const newIssues = findings.filter(f => !originalIssues.includes(f.id));

    console.log('FIXED ISSUES:', fixed.length);
    fixed.forEach(id => console.log(`  ✓ ${id}: FIXED`));

    console.log('\nSTILL PRESENT:', stillPresent.length);
    stillPresent.forEach(f => console.log(`  ✗ ${f.id}: ${f.title}`));

    console.log('\nNEW ISSUES:', newIssues.length);
    newIssues.forEach(f => console.log(`  ! ${f.id}: ${f.title} [${f.severity}]`));

    // Save findings to JSON
    const report = {
      timestamp: new Date().toISOString(),
      summary: {
        totalOriginalIssues: originalIssues.length,
        fixed: fixed.length,
        stillPresent: stillPresent.length,
        newIssues: newIssues.length
      },
      fixedIssues: fixed,
      findings: findings,
      consoleErrors: errors,
      consoleWarnings: warnings.slice(0, 10)
    };

    writeFileSync(
      `${SCREENSHOT_DIR}/verification-report.json`,
      JSON.stringify(report, null, 2)
    );
    console.log('\n✓ Report saved: verification-report.json');

  } catch (error) {
    console.error('Test failed:', error);
    throw error;
  } finally {
    await browser.close();
  }
}

testAgentsPage().catch(console.error);
