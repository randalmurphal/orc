import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';
import { writeFile } from 'fs/promises';

// Create output directory
await mkdir('/tmp/qa-TASK-613', { recursive: true });

const findings = [];
let findingCounter = 1;

function addFinding(severity, title, expected, actual, confidence, category = 'functional') {
  findings.push({
    id: `QA-${String(findingCounter).padStart(3, '0')}`,
    severity,
    confidence,
    category,
    title,
    expected,
    actual,
    screenshot_path: null
  });
  findingCounter++;
}

async function takeScreenshot(page, name) {
  const path = `/tmp/qa-TASK-613/${name}`;
  await page.screenshot({ path, fullPage: true });
  console.log(`Screenshot saved: ${path}`);
  return path;
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  viewport: { width: 1920, height: 1080 },
  deviceScaleFactor: 1
});
const page = await context.newPage();

// Collect console messages
const consoleMessages = [];
page.on('console', msg => {
  consoleMessages.push({
    type: msg.type(),
    text: msg.text()
  });
});

try {
  console.log('=== Starting Agents Page Testing ===\n');

  // 1. Navigate to the page
  console.log('1. Navigating to http://localhost:5173...');
  await page.goto('http://localhost:5173', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);

  // 2. Find Agents link in sidebar and navigate
  console.log('2. Looking for Agents navigation...');
  const agentsLink = page.locator('a[href="/agents"], button:has-text("Agents")').first();

  if (await agentsLink.count() > 0) {
    console.log('   Found Agents link, clicking...');
    await agentsLink.click();
    await page.waitForTimeout(1000);
  } else {
    console.log('   Direct navigation attempt to /agents...');
    await page.goto('http://localhost:5173/agents', { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000);
  }

  // Take desktop screenshot
  console.log('3. Taking desktop screenshot...');
  await takeScreenshot(page, 'agents-desktop.png');

  // 4. Verify Header Elements
  console.log('\n4. Verifying Header Elements...');

  const pageTitle = page.locator('h1:has-text("Agents")').first();
  if (await pageTitle.count() === 0) {
    addFinding('high', 'Missing page title "Agents"',
      'Page should have h1 with text "Agents"',
      'No h1 with "Agents" text found',
      90);
    console.log('   ❌ Page title "Agents" not found');
  } else {
    console.log('   ✓ Page title "Agents" found');
  }

  const subtitle = page.locator('text=Configure Claude models and execution settings').first();
  if (await subtitle.count() === 0) {
    addFinding('medium', 'Missing subtitle text',
      'Should have subtitle "Configure Claude models and execution settings"',
      'Subtitle text not found',
      85);
    console.log('   ❌ Subtitle not found');
  } else {
    console.log('   ✓ Subtitle found');
  }

  const addAgentBtn = page.locator('button:has-text("Add Agent")').first();
  if (await addAgentBtn.count() === 0) {
    addFinding('high', 'Missing "+ Add Agent" button',
      'Header should have "+ Add Agent" button',
      'Button not found',
      90);
    console.log('   ❌ "+ Add Agent" button not found');
  } else {
    console.log('   ✓ "+ Add Agent" button found');
    // Try clicking it
    try {
      await addAgentBtn.click();
      await page.waitForTimeout(500);
      console.log('   ✓ "+ Add Agent" button clickable');

      // Check if modal/dialog appeared
      const modal = page.locator('[role="dialog"], .modal').first();
      if (await modal.count() === 0) {
        addFinding('medium', '"+ Add Agent" button has no visible effect',
          'Clicking should open a modal or dialog',
          'No modal/dialog appeared after clicking',
          80);
        console.log('   ⚠ No modal appeared after clicking');
      } else {
        console.log('   ✓ Modal/dialog appeared');
        // Close modal if there's a close button
        const closeBtn = page.locator('button[aria-label="Close"], button:has-text("Cancel")').first();
        if (await closeBtn.count() > 0) {
          await closeBtn.click();
          await page.waitForTimeout(300);
        }
      }
    } catch (e) {
      console.log('   ⚠ Error clicking button:', e.message);
    }
  }

  // 5. Verify Active Agents Section
  console.log('\n5. Verifying Active Agents Section...');

  const activeAgentsTitle = page.locator('h2:has-text("Active Agents"), h3:has-text("Active Agents")').first();
  if (await activeAgentsTitle.count() === 0) {
    addFinding('high', 'Missing "Active Agents" section title',
      'Should have heading with "Active Agents"',
      'Section title not found',
      90);
    console.log('   ❌ "Active Agents" section title not found');
  } else {
    console.log('   ✓ "Active Agents" section title found');
  }

  const activeAgentsSubtitle = page.locator('text=Currently configured Claude instances').first();
  if (await activeAgentsSubtitle.count() === 0) {
    addFinding('medium', 'Missing Active Agents subtitle',
      'Should have subtitle "Currently configured Claude instances"',
      'Subtitle not found',
      85);
    console.log('   ❌ Active Agents subtitle not found');
  } else {
    console.log('   ✓ Active Agents subtitle found');
  }

  // Check for agent cards
  const agentCards = page.locator('[class*="agent-card"], [data-testid*="agent-card"], .card').all();
  const cardCount = (await agentCards).length;
  console.log(`   Found ${cardCount} agent cards`);

  if (cardCount === 0) {
    addFinding('critical', 'No agent cards displayed',
      'Should display at least 3 agent cards based on reference design',
      'No agent cards found on the page',
      95);
  } else if (cardCount < 3) {
    addFinding('medium', 'Fewer agent cards than expected',
      'Reference design shows 3 agent cards',
      `Only ${cardCount} agent cards found`,
      80);
  }

  // Check for specific agent names from reference
  const primaryCoderAgent = page.locator('text=Primary Coder').first();
  const quickTasksAgent = page.locator('text=Quick Tasks').first();
  const codeReviewAgent = page.locator('text=Code Review').first();

  if (await primaryCoderAgent.count() === 0) {
    console.log('   ⚠ "Primary Coder" agent not found');
  } else {
    console.log('   ✓ "Primary Coder" agent found');
  }

  if (await quickTasksAgent.count() === 0) {
    console.log('   ⚠ "Quick Tasks" agent not found');
  } else {
    console.log('   ✓ "Quick Tasks" agent found');
  }

  if (await codeReviewAgent.count() === 0) {
    console.log('   ⚠ "Code Review" agent not found');
  } else {
    console.log('   ✓ "Code Review" agent found');
  }

  // 6. Verify Execution Settings
  console.log('\n6. Verifying Execution Settings...');

  const execSettingsTitle = page.locator('h2:has-text("Execution Settings"), h3:has-text("Execution Settings")').first();
  if (await execSettingsTitle.count() === 0) {
    addFinding('high', 'Missing "Execution Settings" section title',
      'Should have heading with "Execution Settings"',
      'Section title not found',
      90);
    console.log('   ❌ "Execution Settings" section title not found');
  } else {
    console.log('   ✓ "Execution Settings" section title found');
  }

  // Check for settings controls
  const parallelTasksLabel = page.locator('text=Parallel Tasks').first();
  const autoApproveLabel = page.locator('text=Auto-Approve').first();
  const defaultModelLabel = page.locator('text=Default Model').first();
  const costLimitLabel = page.locator('text=Cost Limit').first();

  if (await parallelTasksLabel.count() === 0) {
    addFinding('high', 'Missing "Parallel Tasks" setting',
      'Should have "Parallel Tasks" slider setting',
      'Setting not found',
      90);
    console.log('   ❌ "Parallel Tasks" setting not found');
  } else {
    console.log('   ✓ "Parallel Tasks" setting found');

    // Try to interact with slider
    const slider = page.locator('input[type="range"]').first();
    if (await slider.count() > 0) {
      try {
        const initialValue = await slider.inputValue();
        await slider.fill('5');
        await page.waitForTimeout(300);
        const newValue = await slider.inputValue();
        if (initialValue !== newValue) {
          console.log('   ✓ Parallel Tasks slider is interactive');
        } else {
          addFinding('medium', 'Parallel Tasks slider not responsive',
            'Slider should update value when changed',
            'Slider value did not change',
            80);
        }
      } catch (e) {
        console.log('   ⚠ Could not interact with slider:', e.message);
      }
    }
  }

  if (await autoApproveLabel.count() === 0) {
    addFinding('high', 'Missing "Auto-Approve" setting',
      'Should have "Auto-Approve" toggle setting',
      'Setting not found',
      90);
    console.log('   ❌ "Auto-Approve" setting not found');
  } else {
    console.log('   ✓ "Auto-Approve" setting found');
  }

  if (await defaultModelLabel.count() === 0) {
    addFinding('high', 'Missing "Default Model" setting',
      'Should have "Default Model" dropdown setting',
      'Setting not found',
      90);
    console.log('   ❌ "Default Model" setting not found');
  } else {
    console.log('   ✓ "Default Model" setting found');
  }

  if (await costLimitLabel.count() === 0) {
    addFinding('high', 'Missing "Cost Limit" setting',
      'Should have "Cost Limit" slider setting',
      'Setting not found',
      90);
    console.log('   ❌ "Cost Limit" setting not found');
  } else {
    console.log('   ✓ "Cost Limit" setting found');
  }

  // 7. Verify Tool Permissions
  console.log('\n7. Verifying Tool Permissions...');

  const toolPermissionsTitle = page.locator('h2:has-text("Tool Permissions"), h3:has-text("Tool Permissions")').first();
  if (await toolPermissionsTitle.count() === 0) {
    addFinding('high', 'Missing "Tool Permissions" section title',
      'Should have heading with "Tool Permissions"',
      'Section title not found',
      90);
    console.log('   ❌ "Tool Permissions" section title not found');
  } else {
    console.log('   ✓ "Tool Permissions" section title found');
  }

  // Check for specific permission toggles
  const permissions = [
    'File Read',
    'File Write',
    'Bash Commands',
    'Web Search',
    'Git Operations',
    'MCP Servers'
  ];

  let missingPermissions = [];
  for (const permission of permissions) {
    const permLabel = page.locator(`text=${permission}`).first();
    if (await permLabel.count() === 0) {
      missingPermissions.push(permission);
      console.log(`   ❌ "${permission}" permission not found`);
    } else {
      console.log(`   ✓ "${permission}" permission found`);
    }
  }

  if (missingPermissions.length > 0) {
    addFinding('high', 'Missing tool permission toggles',
      'Should have all 6 permission toggles: ' + permissions.join(', '),
      `Missing: ${missingPermissions.join(', ')}`,
      90);
  }

  // 8. Mobile Testing
  console.log('\n8. Mobile Viewport Testing...');
  await page.setViewportSize({ width: 375, height: 667 });
  await page.waitForTimeout(1000);

  await takeScreenshot(page, 'agents-mobile.png');
  console.log('   ✓ Mobile screenshot captured');

  // Check if layout is responsive
  const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
  if (bodyWidth > 375) {
    addFinding('high', 'Horizontal scrolling on mobile viewport',
      'Page should fit within 375px width without horizontal scroll',
      `Page width is ${bodyWidth}px, causing horizontal scroll`,
      90);
    console.log(`   ❌ Page width (${bodyWidth}px) exceeds viewport (375px)`);
  } else {
    console.log('   ✓ No horizontal scrolling detected');
  }

  // 9. Console Errors
  console.log('\n9. Checking Console Messages...');
  const errors = consoleMessages.filter(m => m.type === 'error');
  const warnings = consoleMessages.filter(m => m.type === 'warning');

  if (errors.length > 0) {
    console.log(`   Found ${errors.length} console errors:`);
    errors.forEach(err => console.log(`     - ${err.text}`));
    addFinding('high', 'JavaScript console errors detected',
      'Page should load without console errors',
      `${errors.length} error(s): ${errors.map(e => e.text).join('; ')}`,
      95);
  } else {
    console.log('   ✓ No console errors');
  }

  if (warnings.length > 0) {
    console.log(`   Found ${warnings.length} console warnings:`);
    warnings.forEach(warn => console.log(`     - ${warn.text}`));
  }

  console.log('\n=== Testing Complete ===\n');

} catch (error) {
  console.error('Test execution error:', error);
  addFinding('critical', 'Test execution failed',
    'Tests should complete without errors',
    `Error: ${error.message}`,
    100);
} finally {
  await browser.close();

  // Write findings to JSON
  const findingsJson = JSON.stringify(findings, null, 2);
  await writeFile('/tmp/qa-TASK-613/findings.json', findingsJson);

  console.log(`\nTotal findings: ${findings.length}`);
  console.log('Findings saved to: /tmp/qa-TASK-613/findings.json');

  if (findings.length > 0) {
    console.log('\nFindings Summary:');
    findings.forEach(f => {
      console.log(`  [${f.severity.toUpperCase()}] ${f.id}: ${f.title}`);
    });
  }
}
