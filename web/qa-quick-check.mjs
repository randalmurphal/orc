import { chromium } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const SCREENSHOT_DIR = '/tmp/qa-TASK-614';
fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

async function main() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
  const page = await context.newPage();
  
  const findings = [];
  const consoleMessages = [];
  
  page.on('console', msg => consoleMessages.push({ type: msg.type(), text: msg.text() }));
  
  try {
    // Navigate with 'load' instead of 'networkidle'
    console.log('Navigating to /initiatives...');
    await page.goto('http://localhost:5173/initiatives', { waitUntil: 'load', timeout: 15000 });
    await page.waitForTimeout(2000); // Wait for any animations/renders
    
    // Take screenshot
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'initiatives-desktop.png'), fullPage: true });
    console.log('Screenshot saved');
    
    // Check QA-001 & QA-002: Stat cards trend indicators
    const statsCards = await page.locator('.stats-row-card').count();
    console.log(`Found ${statsCards} stat cards`);
    
    const trendsFound = await page.locator('.stats-row-card-trend').count();
    console.log(`Found ${trendsFound} trend indicators`);
    
    if (trendsFound === 0) {
      findings.push({
        id: 'QA-002',
        status: 'STILL_PRESENT',
        evidence: `No trend indicators found (expected 4, found 0). Stats cards exist but lack trend display.`
      });
    }
    
    // Check specific "this week" trend text
    const tasksThisWeekText = await page.locator('text=/this week/i').count();
    console.log(`Found ${tasksThisWeekText} "this week" indicators`);
    
    if (tasksThisWeekText === 0) {
      findings.push({
        id: 'QA-001',
        status: 'STILL_PRESENT',
        evidence: `No "this week" trend text found. Expected trend like "+12 this week".`
      });
    }
    
    // Check QA-003: Time estimates in initiative cards
    const initiativeCards = await page.locator('.initiative-card').count();
    console.log(`Found ${initiativeCards} initiative cards`);
    
    const timeEstimates = await page.locator('text=/est\\..*remaining/i').count();
    console.log(`Found ${timeEstimates} time estimate indicators`);
    
    if (timeEstimates === 0 && initiativeCards > 0) {
      findings.push({
        id: 'QA-003',
        status: 'STILL_PRESENT',
        evidence: `No time estimates found in ${initiativeCards} initiative cards. Expected "Est. Xh remaining" text.`
      });
    }
    
    // Check QA-004: Grid layout columns
    const gridContainer = await page.locator('.initiatives-grid').first();
    const gridStyle = await gridContainer.evaluate(el => window.getComputedStyle(el).gridTemplateColumns);
    console.log(`Grid template columns: ${gridStyle}`);
    
    const columnCount = gridStyle.split(' ').length;
    console.log(`Column count: ${columnCount}`);
    
    if (columnCount > 2) {
      findings.push({
        id: 'QA-004',
        status: 'STILL_PRESENT',
        evidence: `Grid has ${columnCount} columns on 1920px viewport (expected 2). CSS: ${gridStyle}`
      });
    }
    
    // Check for console errors
    const errors = consoleMessages.filter(m => m.type === 'error');
    if (errors.length > 0) {
      console.log('\nConsole Errors:');
      errors.forEach(e => console.log(`  - ${e.text}`));
    }
    
    console.log('\n=== FINDINGS ===');
    findings.forEach(f => {
      console.log(`\n${f.id}: ${f.status}`);
      console.log(`  ${f.evidence}`);
    });
    
    if (findings.length === 0) {
      console.log('\nAll issues appear to be FIXED!');
    }
    
  } catch (error) {
    console.error('Error:', error.message);
  } finally {
    await browser.close();
  }
}

main();
