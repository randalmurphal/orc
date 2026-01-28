import { chromium } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = '/tmp/qa-TASK-614';
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

async function safeCount(page, selector) {
  try {
    return await page.locator(selector).count({ timeout: 5000 });
  } catch {
    return 0;
  }
}

async function safeEvaluate(page, selector, fn) {
  try {
    const element = await page.locator(selector).first();
    return await element.evaluate(fn, { timeout: 5000 });
  } catch {
    return null;
  }
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
  const page = await context.newPage();

  const findings = [];
  const consoleMessages = [];

  page.on('console', msg => consoleMessages.push({ type: msg.type(), text: msg.text() }));

  try {
    console.log('Navigating to /initiatives...');
    await page.goto('http://localhost:5173/initiatives', { waitUntil: 'load', timeout: 15000 });
    await page.waitForTimeout(2000);

    // Take full page screenshot
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'initiatives-desktop.png'), fullPage: true });
    console.log('✓ Screenshot saved');

    // QA-001 & QA-002: Check stat cards and trends
    const statsCards = await safeCount(page, '.stats-row-card');
    console.log(`\nStats Cards: ${statsCards} found`);

    const trendElements = await safeCount(page, '.stats-row-card-trend');
    console.log(`Trend indicators: ${trendElements} found`);

    const thisWeekTexts = await safeCount(page, 'text=/\\+.*this week/i');
    console.log(`"this week" texts: ${thisWeekTexts} found`);

    // Take screenshot of stats section
    const statsSection = page.locator('.stats-row').first();
    if (await statsSection.count() > 0) {
      await statsSection.screenshot({ path: path.join(SCREENSHOT_DIR, 'stats-section.png') });
    }

    if (trendElements === 0) {
      findings.push({
        id: 'QA-002',
        status: 'STILL_PRESENT',
        severity: 'medium',
        confidence: 95,
        category: 'functional',
        title: 'Stat cards missing trend indicators',
        evidence: `Found ${statsCards} stat cards but 0 trend indicator elements (.stats-row-card-trend). Expected 4 trends showing weekly changes.`,
        screenshot: 'stats-section.png'
      });
    }

    if (thisWeekTexts === 0) {
      findings.push({
        id: 'QA-001',
        status: 'STILL_PRESENT',
        severity: 'medium',
        confidence: 90,
        category: 'functional',
        title: 'Task trend "this week" text missing',
        evidence: `No "+X this week" text found in stat cards (searched for pattern /\\+.*this week/i).`,
        screenshot: 'stats-section.png'
      });
    }

    // QA-003: Check initiative card time estimates
    const initiativeCards = await safeCount(page, '.initiative-card');
    console.log(`\nInitiative Cards: ${initiativeCards} found`);

    const timeEstimates = await safeCount(page, 'text=/est\\..*remaining/i');
    console.log(`Time estimates: ${timeEstimates} found`);

    // Take screenshot of first initiative card
    const firstCard = page.locator('.initiative-card').first();
    if (await firstCard.count() > 0) {
      await firstCard.screenshot({ path: path.join(SCREENSHOT_DIR, 'initiative-card.png') });
    }

    if (timeEstimates < initiativeCards && initiativeCards > 0) {
      findings.push({
        id: 'QA-003',
        status: 'STILL_PRESENT',
        severity: 'medium',
        confidence: 85,
        category: 'functional',
        title: 'Initiative cards missing estimated time remaining',
        evidence: `Found ${initiativeCards} initiative cards but only ${timeEstimates} have time estimates. Expected "Est. Xh remaining" on all cards.`,
        screenshot: 'initiative-card.png'
      });
    }

    // QA-004: Check grid layout
    console.log('\nChecking grid layout...');

    // Try multiple possible selectors
    const gridSelectors = ['.initiatives-grid', '.initiatives-view-initiatives', '[class*="grid"]'];
    let gridStyle = null;

    for (const selector of gridSelectors) {
      const count = await safeCount(page, selector);
      if (count > 0) {
        gridStyle = await safeEvaluate(page, selector, el => window.getComputedStyle(el).gridTemplateColumns);
        if (gridStyle) {
          console.log(`Grid found with selector "${selector}"`);
          console.log(`Grid template columns: ${gridStyle}`);
          break;
        }
      }
    }

    if (gridStyle) {
      const columnCount = gridStyle.split(' ').filter(c => c.includes('fr') || c.includes('px')).length;
      console.log(`Detected ${columnCount} columns`);

      if (columnCount > 2) {
        findings.push({
          id: 'QA-004',
          status: 'STILL_PRESENT',
          severity: 'low',
          confidence: 90,
          category: 'visual',
          title: 'Grid layout uses too many columns',
          evidence: `Grid has ${columnCount} columns on 1920px viewport (expected 2 per reference design). CSS: ${gridStyle}`,
          screenshot: 'initiatives-desktop.png'
        });
      } else {
        console.log('✓ Grid layout appears correct (2 columns)');
      }
    } else {
      console.log('⚠ Could not detect grid layout');
    }

    // Mobile test
    console.log('\nTesting mobile viewport (375x667)...');
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(1000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'initiatives-mobile.png'), fullPage: true });
    console.log('✓ Mobile screenshot saved');

    // Check console errors
    const errors = consoleMessages.filter(m => m.type === 'error');
    if (errors.length > 0) {
      console.log('\n⚠ Console Errors Found:');
      errors.forEach(e => console.log(`  - ${e.text}`));
    }

    // Summary
    console.log('\n' + '='.repeat(60));
    console.log('VERIFICATION RESULTS');
    console.log('='.repeat(60));

    if (findings.length === 0) {
      console.log('\n✓ ALL ISSUES FIXED - No findings to report\n');
    } else {
      console.log(`\n${findings.length} issue(s) found:\n`);
      findings.forEach(f => {
        console.log(`${f.id}: ${f.status}`);
        console.log(`  ${f.title}`);
        console.log(`  ${f.evidence}`);
        console.log('');
      });
    }

    console.log(`Screenshots saved to: ${SCREENSHOT_DIR}/`);

    // Write JSON report
    const qa001Fixed = !findings.find(f => f.id === 'QA-001');
    const qa002Fixed = !findings.find(f => f.id === 'QA-002');
    const qa003Fixed = !findings.find(f => f.id === 'QA-003');
    const qa004Fixed = !findings.find(f => f.id === 'QA-004');

    const report = {
      status: 'complete',
      summary: `Tested Initiatives page: ${findings.length} of 4 previous issues still present`,
      findings: findings.map(f => ({
        id: f.id,
        severity: f.severity,
        confidence: f.confidence,
        category: f.category,
        title: f.title,
        steps_to_reproduce: [
          'Navigate to http://localhost:5173/initiatives',
          'Observe the issue described in evidence'
        ],
        expected: getExpected(f.id),
        actual: f.evidence,
        screenshot_path: path.join(SCREENSHOT_DIR, f.screenshot)
      })),
      verification: {
        scenarios_tested: 4,
        viewports_tested: ['desktop', 'mobile'],
        previous_issues_verified: [
          `QA-001: ${qa001Fixed ? 'FIXED' : 'STILL_PRESENT'}`,
          `QA-002: ${qa002Fixed ? 'FIXED' : 'STILL_PRESENT'}`,
          `QA-003: ${qa003Fixed ? 'FIXED' : 'STILL_PRESENT'}`,
          `QA-004: ${qa004Fixed ? 'FIXED' : 'STILL_PRESENT'}`
        ]
      }
    };

    fs.writeFileSync(path.join(SCREENSHOT_DIR, 'qa-report.json'), JSON.stringify(report, null, 2));
    console.log('\n✓ Report saved to qa-report.json\n');

  } catch (error) {
    console.error('\n✗ Test Error:', error.message);
    console.error(error.stack);
  } finally {
    await browser.close();
  }
}

function getExpected(id) {
  const expectations = {
    'QA-001': 'Total Tasks stat card shows trend like "+12 this week" with up arrow',
    'QA-002': 'All 4 stat cards show trend indicators (green/red arrows with change text)',
    'QA-003': 'Initiative cards show estimated time remaining like "Est. 8h remaining" with clock icon',
    'QA-004': 'Grid layout uses exactly 2 columns on desktop viewport (1920px)'
  };
  return expectations[id] || 'See reference design';
}

main();
