import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'fs/promises';

// Create output directory
const OUTPUT_DIR = '/tmp/qa-TASK-613';
await mkdir(OUTPUT_DIR, { recursive: true });

const findings = [];
let findingCounter = 1;

function addFinding(severity, title, expected, actual, confidence, steps = [], category = 'functional') {
  const id = `QA-${String(findingCounter).padStart(3, '0')}`;
  findings.push({
    id,
    severity,
    confidence,
    category,
    title,
    steps_to_reproduce: steps,
    expected,
    actual,
    screenshot_path: `/tmp/qa-TASK-613/bug-${id}.png`,
    suggested_fix: null
  });
  findingCounter++;
  return id;
}

async function takeScreenshot(page, name) {
  const path = `${OUTPUT_DIR}/${name}`;
  await page.screenshot({ path, fullPage: true });
  console.log(`   📸 Screenshot: ${name}`);
  return path;
}

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1920, height: 1080 }
});
const page = await context.newPage();

// Collect console messages
const consoleMessages = [];
page.on('console', msg => consoleMessages.push({ type: msg.type(), text: msg.text() }));

try {
  console.log('\n🧪 QA Testing: Agents Page');
  console.log('=' .repeat(60));

  // Navigate to Agents page
  console.log('\n📍 Step 1: Navigation');
  await page.goto('http://localhost:5173/agents', { waitUntil: 'networkidle', timeout: 10000 });
  await page.waitForTimeout(1000);

  await takeScreenshot(page, 'agents-desktop.png');
  console.log('   ✓ Loaded /agents page');

  // Verify Header
  console.log('\n📍 Step 2: Header Verification');

  const pageTitle = await page.locator('h1').filter({ hasText: 'Agents' }).count();
  if (pageTitle === 0) {
    addFinding('high', 'Missing page title "Agents"',
      'Page should have h1 heading with text "Agents"',
      'No h1 with "Agents" found',
      90,
      ['Navigate to http://localhost:5173/agents', 'Check page header']);
    console.log('   ❌ Missing "Agents" h1 title');
  } else {
    console.log('   ✓ Found "Agents" title');
  }

  const subtitle = await page.getByText('Configure Claude models and execution settings').count();
  if (subtitle === 0) {
    addFinding('medium', 'Missing subtitle text',
      'Should display subtitle "Configure Claude models and execution settings"',
      'Subtitle not found',
      85,
      ['Navigate to /agents', 'Look below the title']);
    console.log('   ❌ Missing subtitle');
  } else {
    console.log('   ✓ Found subtitle');
  }

  const addAgentBtn = await page.getByRole('button', { name: /add agent/i }).count();
  if (addAgentBtn === 0) {
    addFinding('high', 'Missing "+ Add Agent" button',
      'Header should have "+ Add Agent" button in top-right',
      'Button not found',
      90,
      ['Navigate to /agents', 'Check top-right corner of header']);
    console.log('   ❌ Missing "+ Add Agent" button');
  } else {
    console.log('   ✓ Found "+ Add Agent" button');

    // Test button interaction
    try {
      await page.getByRole('button', { name: /add agent/i }).click();
      await page.waitForTimeout(500);

      const modal = await page.locator('[role="dialog"]').count();
      if (modal === 0) {
        addFinding('medium', '"+ Add Agent" button has no visible effect',
          'Clicking should open a modal or form',
          'No modal appeared after clicking',
          80,
          ['Navigate to /agents', 'Click "+ Add Agent" button']);
        console.log('   ⚠️  Button clicked but no modal appeared');
      } else {
        console.log('   ✓ Modal appeared on click');
        // Close modal
        const closeBtn = await page.locator('button[aria-label="Close"]').count();
        if (closeBtn > 0) {
          await page.locator('button[aria-label="Close"]').click();
          await page.waitForTimeout(300);
        }
      }
    } catch (e) {
      console.log('   ⚠️  Error testing button:', e.message);
    }
  }

  // Verify Active Agents Section
  console.log('\n📍 Step 3: Active Agents Section');

  const activeAgentsHeading = await page.locator('h2, h3').filter({ hasText: 'Active Agents' }).count();
  if (activeAgentsHeading === 0) {
    addFinding('high', 'Missing "Active Agents" section',
      'Should have "Active Agents" section heading',
      'Section heading not found',
      90,
      ['Navigate to /agents', 'Scroll down to find Active Agents section']);
    console.log('   ❌ Missing "Active Agents" heading');
  } else {
    console.log('   ✓ Found "Active Agents" heading');
  }

  const activeAgentsSubtitle = await page.getByText('Currently configured Claude instances').count();
  if (activeAgentsSubtitle === 0) {
    console.log('   ⚠️  Missing subtitle "Currently configured Claude instances"');
  } else {
    console.log('   ✓ Found section subtitle');
  }

  // Check for agent cards - looking for specific agent names from reference
  const agentNames = ['Primary Coder', 'Quick Tasks', 'Code Review'];
  let foundAgents = 0;

  for (const name of agentNames) {
    const found = await page.getByText(name, { exact: false }).count();
    if (found > 0) {
      console.log(`   ✓ Found agent: "${name}"`);
      foundAgents++;
    } else {
      console.log(`   ⚠️  Missing agent: "${name}"`);
    }
  }

  if (foundAgents === 0) {
    addFinding('critical', 'No agent cards displayed',
      'Should display 3 agent cards: Primary Coder, Quick Tasks, Code Review',
      'No agent cards found',
      95,
      ['Navigate to /agents', 'Check Active Agents section']);
  } else if (foundAgents < 3) {
    addFinding('medium', `Only ${foundAgents}/3 agent cards displayed`,
      'Should display all 3 agents from reference design',
      `Found ${foundAgents} agent(s)`,
      85,
      ['Navigate to /agents', 'Check Active Agents section']);
  }

  // Verify Execution Settings
  console.log('\n📍 Step 4: Execution Settings');

  const execSettingsHeading = await page.locator('h2, h3').filter({ hasText: 'Execution Settings' }).count();
  if (execSettingsHeading === 0) {
    addFinding('high', 'Missing "Execution Settings" section',
      'Should have "Execution Settings" section',
      'Section not found',
      90,
      ['Navigate to /agents', 'Scroll down to Execution Settings']);
    console.log('   ❌ Missing "Execution Settings" heading');
  } else {
    console.log('   ✓ Found "Execution Settings" heading');
  }

  // Check for specific settings
  const settings = [
    { label: 'Parallel Tasks', type: 'slider' },
    { label: 'Auto-Approve', type: 'toggle' },
    { label: 'Default Model', type: 'dropdown' },
    { label: 'Cost Limit', type: 'slider' }
  ];

  for (const setting of settings) {
    const found = await page.getByText(setting.label, { exact: false }).count();
    if (found === 0) {
      addFinding('high', `Missing "${setting.label}" setting`,
        `Should have "${setting.label}" ${setting.type} control`,
        'Setting not found',
        90,
        ['Navigate to /agents', 'Check Execution Settings section']);
      console.log(`   ❌ Missing "${setting.label}"`);
    } else {
      console.log(`   ✓ Found "${setting.label}"`);
    }
  }

  // Test slider interactivity
  const sliders = await page.locator('input[type="range"]').all();
  if (sliders.length > 0) {
    try {
      const firstSlider = sliders[0];
      const initialValue = await firstSlider.inputValue();
      await firstSlider.fill('5');
      await page.waitForTimeout(200);
      const newValue = await firstSlider.inputValue();

      if (initialValue !== newValue) {
        console.log('   ✓ Slider is interactive');
      } else {
        addFinding('medium', 'Slider not responding to input',
          'Slider should update value when changed',
          'Value did not change',
          80,
          ['Navigate to /agents', 'Try dragging any slider']);
      }
    } catch (e) {
      console.log('   ⚠️  Could not test slider:', e.message);
    }
  }

  // Verify Tool Permissions
  console.log('\n📍 Step 5: Tool Permissions');

  const toolPermissionsHeading = await page.locator('h2, h3').filter({ hasText: 'Tool Permissions' }).count();
  if (toolPermissionsHeading === 0) {
    addFinding('high', 'Missing "Tool Permissions" section',
      'Should have "Tool Permissions" section',
      'Section not found',
      90,
      ['Navigate to /agents', 'Scroll down to Tool Permissions']);
    console.log('   ❌ Missing "Tool Permissions" heading');
  } else {
    console.log('   ✓ Found "Tool Permissions" heading');
  }

  const permissions = [
    'File Read',
    'File Write',
    'Bash Commands',
    'Web Search',
    'Git Operations',
    'MCP Servers'
  ];

  let missingPerms = [];
  for (const perm of permissions) {
    const found = await page.getByText(perm, { exact: false }).count();
    if (found === 0) {
      missingPerms.push(perm);
      console.log(`   ❌ Missing "${perm}"`);
    } else {
      console.log(`   ✓ Found "${perm}"`);
    }
  }

  if (missingPerms.length > 0) {
    addFinding('high', 'Missing tool permission toggles',
      'Should have all 6 permission toggles',
      `Missing: ${missingPerms.join(', ')}`,
      90,
      ['Navigate to /agents', 'Check Tool Permissions section']);
  }

  // Mobile Testing
  console.log('\n📍 Step 6: Mobile Viewport Testing');
  await page.setViewportSize({ width: 375, height: 667 });
  await page.waitForTimeout(1000);

  await takeScreenshot(page, 'agents-mobile.png');
  console.log('   ✓ Captured mobile screenshot');

  const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
  if (bodyWidth > 375) {
    addFinding('high', 'Horizontal scrolling on mobile',
      'Page should fit within 375px width',
      `Page width is ${bodyWidth}px`,
      90,
      ['Resize browser to 375x667', 'Check if horizontal scroll appears']);
    console.log(`   ❌ Horizontal scroll detected (width: ${bodyWidth}px)`);
  } else {
    console.log('   ✓ No horizontal scrolling');
  }

  // Console Errors
  console.log('\n📍 Step 7: Console Messages');
  const errors = consoleMessages.filter(m => m.type === 'error');
  const warnings = consoleMessages.filter(m => m.type === 'warning');

  if (errors.length > 0) {
    console.log(`   ❌ ${errors.length} console error(s):`);
    errors.forEach(err => console.log(`      - ${err.text}`));

    addFinding('high', 'JavaScript console errors',
      'Page should load without errors',
      `${errors.length} error(s)`,
      95,
      ['Open browser DevTools', 'Navigate to /agents', 'Check Console tab']);
  } else {
    console.log('   ✓ No console errors');
  }

  if (warnings.length > 0) {
    console.log(`   ⚠️  ${warnings.length} console warning(s)`);
  }

} catch (error) {
  console.error('\n❌ Test execution failed:', error.message);
  addFinding('critical', 'Test execution error',
    'Tests should complete without errors',
    `Error: ${error.message}`,
    100,
    ['Run QA test suite']);
} finally {
  await browser.close();

  // Save findings
  await writeFile(`${OUTPUT_DIR}/findings.json`, JSON.stringify(findings, null, 2));

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
    console.log('\n✅ No issues found - page appears to match requirements!');
  }

  console.log(`\n📁 Output: ${OUTPUT_DIR}/`);
  console.log('   - agents-desktop.png');
  console.log('   - agents-mobile.png');
  console.log('   - findings.json');
}
