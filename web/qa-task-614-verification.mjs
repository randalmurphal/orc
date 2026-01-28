/**
 * TASK-614: QA Verification for Initiatives Page
 *
 * Tests the 4 specific findings from previous code analysis:
 * - QA-001: Task trend indicators missing
 * - QA-002: Stat card trends not calculated
 * - QA-003: Initiative cards missing time estimates
 * - QA-004: Grid uses too many columns
 *
 * Screenshots saved to: /tmp/qa-TASK-614/
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'fs/promises';
import { existsSync } from 'fs';

const SCREENSHOT_DIR = '/tmp/qa-TASK-614';
const BASE_URL = 'http://localhost:5173';

async function ensureDir(dir) {
	if (!existsSync(dir)) {
		await mkdir(dir, { recursive: true });
	}
}

function assessFinding(id, isPresent, evidence) {
	return {
		id,
		status: isPresent ? 'STILL_PRESENT' : 'FIXED',
		evidence,
		confidence: isPresent ? 95 : 90
	};
}

async function main() {
	console.log('═══════════════════════════════════════════════════════════');
	console.log('  QA Verification: TASK-614 - Initiatives Page');
	console.log('═══════════════════════════════════════════════════════════');
	console.log('');

	await ensureDir(SCREENSHOT_DIR);

	// Check if server is accessible
	console.log(`Checking server availability at ${BASE_URL}...`);
	try {
		const response = await fetch(BASE_URL);
		if (!response.ok) {
			throw new Error(`Server returned ${response.status}`);
		}
		console.log('✓ Server is accessible\n');
	} catch (error) {
		console.error(`✗ Server not accessible: ${error.message}`);
		console.error('\nPlease start the dev server first:');
		console.error('  cd web && npm run dev\n');
		process.exit(1);
	}

	const browser = await chromium.launch({ headless: true });
	const findings = {};

	try {
		// ============================================
		// DESKTOP TESTING (1920x1080)
		// ============================================
		console.log('═══════════════════════════════════════════════════════════');
		console.log('  DESKTOP TESTING (1920x1080)');
		console.log('═══════════════════════════════════════════════════════════');
		console.log('');

		const context = await browser.newContext({
			viewport: { width: 1920, height: 1080 }
		});
		const page = await context.newPage();

		const consoleErrors = [];
		page.on('console', msg => {
			if (msg.type() === 'error') {
				consoleErrors.push(msg.text());
			}
		});

		// Navigate
		console.log('Step 1: Navigating to /initiatives...');
		await page.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'networkidle',
			timeout: 30000
		});
		await page.waitForTimeout(2000);
		console.log('        ✓ Page loaded\n');

		// Take full desktop screenshot
		console.log('Step 2: Taking desktop screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-initiatives.png`,
			fullPage: true
		});
		console.log(`        ✓ Saved to ${SCREENSHOT_DIR}/desktop-initiatives.png\n`);

		// ============================================
		// VERIFY QA-001 & QA-002: Stat Card Trends
		// ============================================
		console.log('───────────────────────────────────────────────────────────');
		console.log(' Verifying QA-001 & QA-002: Stat Card Trends');
		console.log('───────────────────────────────────────────────────────────');

		const statCards = await page.locator('.stats-row-card').all();
		console.log(`Found ${statCards.length} stat cards:\n`);

		let anyTrendFound = false;
		let totalTasksHasTrend = false;
		const statCardDetails = [];

		for (let i = 0; i < statCards.length; i++) {
			const card = statCards[i];
			const label = await card.locator('.stats-row-card-label').textContent();
			const value = await card.locator('.stats-row-card-value').textContent();
			const trendCount = await card.locator('.stats-row-card-trend').count();
			const hasTrend = trendCount > 0;

			let trendText = '';
			if (hasTrend) {
				trendText = await card.locator('.stats-row-card-trend').textContent();
				anyTrendFound = true;

				if (label?.includes('Total Tasks')) {
					totalTasksHasTrend = true;
				}
			}

			const cardInfo = {
				label: label?.trim(),
				value: value?.trim(),
				hasTrend,
				trendText: trendText?.trim()
			};

			statCardDetails.push(cardInfo);

			console.log(`  ${i + 1}. ${cardInfo.label}`);
			console.log(`     Value: ${cardInfo.value}`);
			console.log(`     Has trend: ${hasTrend ? `YES ("${cardInfo.trendText}")` : 'NO'}`);
			console.log('');
		}

		// Take closeup of stat cards
		const statsRow = page.locator('.stats-row').first();
		if (await statsRow.count() > 0) {
			const box = await statsRow.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/desktop-stat-cards-closeup.png`,
					clip: {
						x: Math.max(0, box.x - 20),
						y: Math.max(0, box.y - 20),
						width: Math.min(box.width + 40, 1920),
						height: box.height + 40
					}
				});
			}
		}

		// Assess QA-001
		findings['QA-001'] = assessFinding(
			'QA-001',
			!totalTasksHasTrend,
			totalTasksHasTrend
				? 'Total Tasks card shows trend indicator'
				: 'Total Tasks card has no trend indicator'
		);

		// Assess QA-002
		findings['QA-002'] = assessFinding(
			'QA-002',
			!anyTrendFound,
			anyTrendFound
				? `${statCardDetails.filter(c => c.hasTrend).length}/${statCardDetails.length} cards show trends`
				: 'No stat cards show trend indicators'
		);

		console.log(`Result: QA-001 = ${findings['QA-001'].status}`);
		console.log(`        ${findings['QA-001'].evidence}`);
		console.log(`Result: QA-002 = ${findings['QA-002'].status}`);
		console.log(`        ${findings['QA-002'].evidence}\n`);

		// ============================================
		// VERIFY QA-003: Initiative Card Time Estimates
		// ============================================
		console.log('───────────────────────────────────────────────────────────');
		console.log(' Verifying QA-003: Initiative Card Time Estimates');
		console.log('───────────────────────────────────────────────────────────');

		const initiativeCards = await page.locator('.initiative-card').all();
		console.log(`Found ${initiativeCards.length} initiative cards:\n`);

		let anyCardHasTimeEstimate = false;
		const initiativeCardDetails = [];

		for (let i = 0; i < Math.min(initiativeCards.length, 3); i++) {
			const card = initiativeCards[i];
			const title = await card.locator('.initiative-card-name').textContent();
			const metaItems = await card.locator('.initiative-card-meta-item').all();

			const cardInfo = {
				title: title?.trim(),
				metaItems: []
			};

			for (const meta of metaItems) {
				const text = await meta.textContent();
				cardInfo.metaItems.push(text?.trim());
			}

			// Check for time estimate
			const hasTimeEstimate = cardInfo.metaItems.some(item =>
				item && (
					/Est\.\s+\d+h?\s+remaining/i.test(item) ||
					/\d+h\s+remaining/i.test(item) ||
					item.includes('remaining')
				)
			);

			if (hasTimeEstimate) {
				anyCardHasTimeEstimate = true;
			}

			cardInfo.hasTimeEstimate = hasTimeEstimate;
			initiativeCardDetails.push(cardInfo);

			console.log(`  ${i + 1}. ${cardInfo.title}`);
			console.log(`     Meta items: ${cardInfo.metaItems.length}`);
			cardInfo.metaItems.forEach((item, idx) => {
				console.log(`       [${idx}] ${item}`);
			});
			console.log(`     Has time estimate: ${hasTimeEstimate ? 'YES' : 'NO'}`);
			console.log('');

			// Take screenshot of first card
			if (i === 0) {
				const box = await card.boundingBox();
				if (box) {
					await page.screenshot({
						path: `${SCREENSHOT_DIR}/desktop-first-initiative-card.png`,
						clip: {
							x: Math.max(0, box.x - 10),
							y: Math.max(0, box.y - 10),
							width: box.width + 20,
							height: box.height + 20
						}
					});
				}
			}
		}

		// Assess QA-003
		findings['QA-003'] = assessFinding(
			'QA-003',
			!anyCardHasTimeEstimate,
			anyCardHasTimeEstimate
				? 'Initiative cards show time estimates'
				: 'No initiative cards show time estimates'
		);

		console.log(`Result: QA-003 = ${findings['QA-003'].status}`);
		console.log(`        ${findings['QA-003'].evidence}\n`);

		// ============================================
		// VERIFY QA-004: Grid Layout Columns
		// ============================================
		console.log('───────────────────────────────────────────────────────────');
		console.log(' Verifying QA-004: Grid Layout Columns');
		console.log('───────────────────────────────────────────────────────────');

		const grid = page.locator('.initiatives-view-grid').first();
		let gridColumnCount = 0;
		let gridDetails = null;

		if (await grid.count() > 0) {
			gridDetails = await grid.evaluate(el => {
				const computed = window.getComputedStyle(el);
				const cols = computed.gridTemplateColumns;
				const columnValues = cols.split(' ').filter(c => c && c !== '0px');
				return {
					css: cols,
					columnCount: columnValues.length,
					width: el.offsetWidth
				};
			});

			gridColumnCount = gridDetails.columnCount;

			console.log(`Grid CSS: ${gridDetails.css}`);
			console.log(`Grid width: ${gridDetails.width}px`);
			console.log(`Actual columns: ${gridColumnCount}\n`);

			// Take grid screenshot
			const gridBox = await grid.boundingBox();
			if (gridBox) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/desktop-grid-layout.png`,
					clip: {
						x: 0,
						y: Math.max(0, gridBox.y - 20),
						width: 1920,
						height: Math.min(gridBox.height + 40, 1080)
					}
				});
			}
		} else {
			console.log('⚠ Grid element not found\n');
		}

		// Assess QA-004
		findings['QA-004'] = assessFinding(
			'QA-004',
			gridColumnCount > 2,
			gridColumnCount > 2
				? `Grid has ${gridColumnCount} columns (expected: 2)`
				: gridColumnCount === 2
				? 'Grid has exactly 2 columns as expected'
				: `Grid has ${gridColumnCount} columns (unexpected)`
		);

		console.log(`Result: QA-004 = ${findings['QA-004'].status}`);
		console.log(`        ${findings['QA-004'].evidence}\n`);

		// Console errors check
		console.log('───────────────────────────────────────────────────────────');
		console.log(' Console Errors');
		console.log('───────────────────────────────────────────────────────────');
		if (consoleErrors.length === 0) {
			console.log('✓ No console errors\n');
		} else {
			console.log(`✗ Found ${consoleErrors.length} console errors:\n`);
			consoleErrors.forEach((err, i) => {
				console.log(`  ${i + 1}. ${err}`);
			});
			console.log('');
		}

		await context.close();

		// ============================================
		// MOBILE TESTING (375x667)
		// ============================================
		console.log('═══════════════════════════════════════════════════════════');
		console.log('  MOBILE TESTING (375x667)');
		console.log('═══════════════════════════════════════════════════════════');
		console.log('');

		const mobileContext = await browser.newContext({
			viewport: { width: 375, height: 667 },
			isMobile: true,
			hasTouch: true,
			userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15'
		});
		const mobilePage = await mobileContext.newPage();

		console.log('Step 1: Navigating to /initiatives...');
		await mobilePage.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'networkidle',
			timeout: 30000
		});
		await mobilePage.waitForTimeout(2000);
		console.log('        ✓ Page loaded\n');

		console.log('Step 2: Taking mobile screenshot...');
		await mobilePage.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-initiatives.png`,
			fullPage: true
		});
		console.log(`        ✓ Saved to ${SCREENSHOT_DIR}/mobile-initiatives.png\n`);

		// Check mobile layout
		const mobileStatCards = await mobilePage.locator('.stats-row-card').all();
		console.log(`Mobile stat cards: ${mobileStatCards.length}`);

		const mobileInitiativeCards = await mobilePage.locator('.initiative-card').all();
		console.log(`Mobile initiative cards: ${mobileInitiativeCards.length}\n`);

		await mobileContext.close();

	} catch (error) {
		console.error('\n✗ Error during testing:', error);
		throw error;
	} finally {
		await browser.close();
	}

	// ============================================
	// SUMMARY REPORT
	// ============================================
	console.log('═══════════════════════════════════════════════════════════');
	console.log('  VERIFICATION SUMMARY');
	console.log('═══════════════════════════════════════════════════════════');
	console.log('');

	const stillPresent = Object.values(findings).filter(f => f.status === 'STILL_PRESENT').length;
	const fixed = Object.values(findings).filter(f => f.status === 'FIXED').length;

	Object.entries(findings).forEach(([id, result]) => {
		const icon = result.status === 'FIXED' ? '✓' : '✗';
		const color = result.status === 'FIXED' ? '\x1b[32m' : '\x1b[31m'; // Green or Red
		const reset = '\x1b[0m';

		console.log(`${color}${icon} ${id}: ${result.status}${reset}`);
		console.log(`   ${result.evidence}`);
		console.log(`   Confidence: ${result.confidence}%`);
		console.log('');
	});

	console.log('───────────────────────────────────────────────────────────');
	console.log(`Total: ${Object.keys(findings).length} issues verified`);
	console.log(`Fixed: ${fixed}`);
	console.log(`Still Present: ${stillPresent}`);
	console.log('───────────────────────────────────────────────────────────');
	console.log('');
	console.log(`Screenshots: ${SCREENSHOT_DIR}/`);
	console.log('');

	// Write detailed JSON report
	const report = {
		task: 'TASK-614',
		date: new Date().toISOString(),
		findings,
		summary: {
			total: Object.keys(findings).length,
			fixed,
			stillPresent
		},
		screenshots: {
			desktop: `${SCREENSHOT_DIR}/desktop-initiatives.png`,
			desktopStatCards: `${SCREENSHOT_DIR}/desktop-stat-cards-closeup.png`,
			desktopFirstCard: `${SCREENSHOT_DIR}/desktop-first-initiative-card.png`,
			desktopGrid: `${SCREENSHOT_DIR}/desktop-grid-layout.png`,
			mobile: `${SCREENSHOT_DIR}/mobile-initiatives.png`
		}
	};

	await writeFile(
		`${SCREENSHOT_DIR}/verification-report.json`,
		JSON.stringify(report, null, 2)
	);

	console.log(`✓ Detailed report: ${SCREENSHOT_DIR}/verification-report.json`);
	console.log('');

	// Exit code: 0 if all fixed, 1 if any still present
	process.exit(stillPresent > 0 ? 1 : 0);
}

main().catch(err => {
	console.error('\nFatal error:', err);
	process.exit(2);
});
