/**
 * QA Verification Script for TASK-614
 * Verifies 4 specific previous findings on Initiatives page
 * Saves screenshots to /tmp/qa-TASK-614/
 */

import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';
import { existsSync } from 'fs';

const SCREENSHOT_DIR = '/tmp/qa-TASK-614';
const BASE_URL = 'http://localhost:5173';

async function ensureDir(dir) {
	if (!existsSync(dir)) {
		await mkdir(dir, { recursive: true });
	}
}

async function verifyFindings() {
	console.log('QA Verification: Initiatives Page');
	console.log('===================================\n');

	await ensureDir(SCREENSHOT_DIR);

	const browser = await chromium.launch({ headless: true });
	const findings = {
		'QA-001': { status: 'UNKNOWN', evidence: '' },
		'QA-002': { status: 'UNKNOWN', evidence: '' },
		'QA-003': { status: 'UNKNOWN', evidence: '' },
		'QA-004': { status: 'UNKNOWN', evidence: '' }
	};

	// ============================================
	// DESKTOP VIEWPORT TESTING (1920x1080)
	// ============================================
	console.log('=== DESKTOP TESTING (1920x1080) ===\n');

	const desktopContext = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const desktopPage = await desktopContext.newPage();

	const consoleErrors = [];
	desktopPage.on('console', msg => {
		if (msg.type() === 'error') {
			consoleErrors.push(msg.text());
		}
	});

	try {
		// Navigate to initiatives page
		console.log('1. Navigating to /initiatives...');
		await desktopPage.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'networkidle',
			timeout: 30000
		});
		await desktopPage.waitForTimeout(2000); // Let content load

		// Take full page screenshot
		console.log('2. Taking desktop screenshot...');
		await desktopPage.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-initiatives.png`,
			fullPage: true
		});
		console.log(`   ✓ Saved to ${SCREENSHOT_DIR}/desktop-initiatives.png\n`);

		// ============================================
		// QA-001 & QA-002: Check stat cards for trends
		// ============================================
		console.log('3. Verifying QA-001 & QA-002: Stat card trends...');

		const statCards = await desktopPage.locator('.stats-row-card').all();
		console.log(`   Found ${statCards.length} stat cards`);

		let anyTrendFound = false;
		let tasksCardHasTrend = false;

		for (let i = 0; i < statCards.length; i++) {
			const card = statCards[i];
			const label = await card.locator('.stats-row-card-label').textContent();
			const value = await card.locator('.stats-row-card-value').textContent();

			// Check for trend element
			const trendElements = await card.locator('.stats-row-card-trend').count();
			const hasTrend = trendElements > 0;

			let trendText = '';
			if (hasTrend) {
				trendText = await card.locator('.stats-row-card-trend').textContent();
				anyTrendFound = true;

				if (label.includes('Total Tasks')) {
					tasksCardHasTrend = true;
				}
			}

			console.log(`   - ${label}: ${value}`);
			console.log(`     Trend element: ${hasTrend ? 'YES' : 'NO'}${trendText ? ` ("${trendText}")` : ''}`);
		}

		// Take screenshot of stat cards area
		await desktopPage.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-stat-cards-closeup.png`,
			clip: { x: 0, y: 60, width: 1920, height: 200 }
		});
		console.log(`   ✓ Saved closeup to ${SCREENSHOT_DIR}/desktop-stat-cards-closeup.png`);

		// Determine QA-001 status
		if (!tasksCardHasTrend) {
			findings['QA-001'].status = 'STILL_PRESENT';
			findings['QA-001'].evidence = 'Total Tasks card has no trend indicator visible';
		} else {
			findings['QA-001'].status = 'FIXED';
			findings['QA-001'].evidence = 'Total Tasks card shows trend indicator';
		}

		// Determine QA-002 status
		if (!anyTrendFound) {
			findings['QA-002'].status = 'STILL_PRESENT';
			findings['QA-002'].evidence = 'No stat cards show trend indicators';
		} else if (statCards.length > 0 && anyTrendFound && statCards.length > 1) {
			// Check if ALL cards have trends
			const allHaveTrends = statCards.length === (await Promise.all(
				statCards.map(c => c.locator('.stats-row-card-trend').count())
			)).filter(count => count > 0).length;

			if (allHaveTrends) {
				findings['QA-002'].status = 'FIXED';
				findings['QA-002'].evidence = 'All stat cards show trend indicators';
			} else {
				findings['QA-002'].status = 'PARTIALLY_FIXED';
				findings['QA-002'].evidence = 'Some stat cards show trends, but not all';
			}
		}

		console.log(`   Result: QA-001 = ${findings['QA-001'].status}`);
		console.log(`   Result: QA-002 = ${findings['QA-002'].status}\n`);

		// ============================================
		// QA-003: Check initiative cards for time estimates
		// ============================================
		console.log('4. Verifying QA-003: Initiative card time estimates...');

		const initiativeCards = await desktopPage.locator('.initiative-card').all();
		console.log(`   Found ${initiativeCards.length} initiative cards`);

		let anyCardHasTimeEstimate = false;

		for (let i = 0; i < Math.min(initiativeCards.length, 3); i++) {
			const card = initiativeCards[i];
			const title = await card.locator('.initiative-card-name').textContent();

			// Look for time remaining text (e.g., "Est. 8h remaining")
			const cardText = await card.textContent();
			const hasTimeEstimate = /Est\.\s+\d+h?\s+remaining/i.test(cardText) ||
			                        /\d+h\s+remaining/i.test(cardText);

			// Also check for clock icon in meta items
			const metaItems = await card.locator('.initiative-card-meta-item').all();
			console.log(`   - "${title.trim()}"`);
			console.log(`     Meta items: ${metaItems.length}`);
			console.log(`     Has time estimate: ${hasTimeEstimate ? 'YES' : 'NO'}`);

			if (hasTimeEstimate) {
				anyCardHasTimeEstimate = true;
			}

			// Take screenshot of first card
			if (i === 0) {
				const box = await card.boundingBox();
				if (box) {
					await desktopPage.screenshot({
						path: `${SCREENSHOT_DIR}/desktop-first-initiative-card.png`,
						clip: box
					});
					console.log(`   ✓ Saved first card to ${SCREENSHOT_DIR}/desktop-first-initiative-card.png`);
				}
			}
		}

		// Determine QA-003 status
		if (!anyCardHasTimeEstimate) {
			findings['QA-003'].status = 'STILL_PRESENT';
			findings['QA-003'].evidence = 'No initiative cards show time estimates';
		} else {
			findings['QA-003'].status = 'FIXED';
			findings['QA-003'].evidence = 'Initiative cards show time estimates';
		}

		console.log(`   Result: QA-003 = ${findings['QA-003'].status}\n`);

		// ============================================
		// QA-004: Check grid layout columns
		// ============================================
		console.log('5. Verifying QA-004: Grid layout columns...');

		const grid = desktopPage.locator('.initiatives-view-grid');
		const gridExists = await grid.count() > 0;

		if (gridExists) {
			const gridStyles = await grid.evaluate(el => {
				const computed = window.getComputedStyle(el);
				return {
					gridTemplateColumns: computed.gridTemplateColumns,
					width: el.offsetWidth
				};
			});

			console.log(`   Grid template columns: ${gridStyles.gridTemplateColumns}`);
			console.log(`   Grid width: ${gridStyles.width}px`);

			// Count actual columns by parsing grid-template-columns
			const columnValues = gridStyles.gridTemplateColumns.split(' ');
			const columnCount = columnValues.length;
			console.log(`   Actual column count: ${columnCount}`);

			// QA-004 expects exactly 2 columns on 1920px screen
			if (columnCount === 2) {
				findings['QA-004'].status = 'FIXED';
				findings['QA-004'].evidence = `Grid uses ${columnCount} columns (expected 2)`;
			} else if (columnCount > 2) {
				findings['QA-004'].status = 'STILL_PRESENT';
				findings['QA-004'].evidence = `Grid uses ${columnCount} columns instead of 2`;
			} else {
				findings['QA-004'].status = 'UNKNOWN';
				findings['QA-004'].evidence = `Grid uses ${columnCount} columns (unexpected)`;
			}
		} else {
			findings['QA-004'].status = 'UNKNOWN';
			findings['QA-004'].evidence = 'Grid element not found';
		}

		console.log(`   Result: QA-004 = ${findings['QA-004'].status}\n`);

		// ============================================
		// Console errors
		// ============================================
		console.log('6. Console errors:');
		if (consoleErrors.length === 0) {
			console.log('   ✓ No console errors\n');
		} else {
			console.log(`   ✗ Found ${consoleErrors.length} console errors:`);
			consoleErrors.forEach((err, i) => {
				console.log(`     ${i + 1}. ${err}`);
			});
			console.log('');
		}

	} catch (error) {
		console.error('Error during desktop testing:', error.message);
	} finally {
		await desktopContext.close();
	}

	// ============================================
	// MOBILE VIEWPORT TESTING (375x667)
	// ============================================
	console.log('\n=== MOBILE TESTING (375x667) ===\n');

	const mobileContext = await browser.newContext({
		viewport: { width: 375, height: 667 },
		userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15'
	});
	const mobilePage = await mobileContext.newPage();

	try {
		console.log('1. Navigating to /initiatives...');
		await mobilePage.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'networkidle',
			timeout: 30000
		});
		await mobilePage.waitForTimeout(2000);

		console.log('2. Taking mobile screenshot...');
		await mobilePage.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-initiatives.png`,
			fullPage: true
		});
		console.log(`   ✓ Saved to ${SCREENSHOT_DIR}/mobile-initiatives.png\n`);

		// Check mobile layout
		console.log('3. Checking mobile layout...');

		const statCards = await mobilePage.locator('.stats-row-card').all();
		console.log(`   Stat cards: ${statCards.length}`);

		const statsRow = mobilePage.locator('.stats-row');
		if (await statsRow.count() > 0) {
			const statsStyles = await statsRow.evaluate(el => {
				const computed = window.getComputedStyle(el);
				return {
					display: computed.display,
					gridTemplateColumns: computed.gridTemplateColumns
				};
			});
			console.log(`   Stats row display: ${statsStyles.display}`);
			console.log(`   Stats row columns: ${statsStyles.gridTemplateColumns}`);
		}

		const initiativeCards = await mobilePage.locator('.initiative-card').all();
		console.log(`   Initiative cards: ${initiativeCards.length}\n`);

	} catch (error) {
		console.error('Error during mobile testing:', error.message);
	} finally {
		await mobileContext.close();
	}

	await browser.close();

	// ============================================
	// SUMMARY
	// ============================================
	console.log('\n=== VERIFICATION SUMMARY ===\n');

	Object.entries(findings).forEach(([id, result]) => {
		const icon = result.status === 'FIXED' ? '✓' :
		             result.status === 'STILL_PRESENT' ? '✗' :
		             result.status === 'PARTIALLY_FIXED' ? '~' : '?';
		console.log(`${icon} ${id}: ${result.status}`);
		console.log(`   ${result.evidence}`);
	});

	console.log(`\nScreenshots saved to: ${SCREENSHOT_DIR}/`);
	console.log('Console errors:', consoleErrors.length === 0 ? 'None' : consoleErrors.length);

	return findings;
}

verifyFindings().catch(err => {
	console.error('Fatal error:', err);
	process.exit(1);
});
