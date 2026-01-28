/**
 * QA Test Script for Initiatives Page
 * Tests desktop (1920x1080) and mobile (375x667) viewports
 */

import { chromium } from '@playwright/test';
import { mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const SCREENSHOT_DIR = join(__dirname, '../qa-screenshots');
const BASE_URL = 'http://localhost:5173';

async function ensureDir(dir) {
	if (!existsSync(dir)) {
		await mkdir(dir, { recursive: true });
	}
}

async function testDesktopViewport(browser) {
	console.log('\n=== DESKTOP TESTING (1920x1080) ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	// Capture console messages
	const consoleMessages = [];
	page.on('console', msg => {
		const type = msg.type();
		if (type === 'error' || type === 'warning') {
			consoleMessages.push({ type, text: msg.text() });
		}
	});

	const results = {
		statCards: [],
		initiativeCards: [],
		gridLayout: null,
		consoleErrors: []
	};

	try {
		// Navigate to initiatives page
		console.log('1. Navigating to /initiatives...');
		await page.goto(`${BASE_URL}/initiatives`, { waitUntil: 'networkidle' });
		await page.waitForTimeout(2000); // Let animations settle

		// Take initial screenshot
		console.log('2. Taking initial screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-01-initial-fullpage.png`,
			fullPage: true
		});

		// Take viewport screenshot (what user sees first)
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-02-initial-viewport.png`,
			fullPage: false
		});

		// Check for stat cards
		console.log('3. Checking stat cards...');
		const statCards = await page.locator('.stats-row-card').all();
		console.log(`   Found ${statCards.length} stat cards`);

		for (let i = 0; i < statCards.length; i++) {
			const card = statCards[i];
			const label = await card.locator('.stats-row-card-label').textContent();
			const value = await card.locator('.stats-row-card-value').textContent();
			const hasTrend = await card.locator('.stats-row-card-trend').count() > 0;

			const cardInfo = {
				label: label?.trim(),
				value: value?.trim(),
				hasTrend
			};

			results.statCards.push(cardInfo);
			console.log(`   - ${cardInfo.label}: ${cardInfo.value} (trend: ${hasTrend ? 'YES' : 'NO'})`);
		}

		// Take screenshot of stat cards area
		const statsRow = page.locator('.stats-row').first();
		if (await statsRow.count() > 0) {
			const box = await statsRow.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/desktop-03-stat-cards-closeup.png`,
					clip: {
						x: Math.max(0, box.x - 20),
						y: Math.max(0, box.y - 20),
						width: Math.min(box.width + 40, 1920),
						height: box.height + 40
					}
				});
			}
		}

		// Check initiative cards
		console.log('4. Checking initiative cards...');
		const initiativeCards = await page.locator('.initiative-card').all();
		console.log(`   Found ${initiativeCards.length} initiative cards`);

		for (let i = 0; i < initiativeCards.length; i++) {
			const card = initiativeCards[i];
			const title = await card.locator('.initiative-card-name').textContent();
			const metaItems = await card.locator('.initiative-card-meta-item').all();

			const cardInfo = {
				title: title?.trim(),
				metaItemCount: metaItems.length,
				metaItems: []
			};

			// Extract meta item text
			for (const meta of metaItems) {
				const text = await meta.textContent();
				cardInfo.metaItems.push(text?.trim());
			}

			results.initiativeCards.push(cardInfo);
			console.log(`   - ${cardInfo.title}: ${cardInfo.metaItemCount} meta items`);
			if (cardInfo.metaItems.length > 0) {
				cardInfo.metaItems.forEach((item, idx) => {
					console.log(`     [${idx}] ${item}`);
				});
			}
		}

		// Take screenshot of first initiative card if any exist
		if (initiativeCards.length > 0) {
			const firstCard = initiativeCards[0];
			const box = await firstCard.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/desktop-04-first-initiative-card.png`,
					clip: {
						x: Math.max(0, box.x - 10),
						y: Math.max(0, box.y - 10),
						width: box.width + 20,
						height: box.height + 20
					}
				});
			}
		}

		// Check grid layout columns
		console.log('5. Checking grid layout...');
		const grid = page.locator('.initiatives-view-grid').first();
		if (await grid.count() > 0) {
			results.gridLayout = await grid.evaluate(el => {
				const computed = window.getComputedStyle(el);
				const cols = computed.gridTemplateColumns;
				return {
					gridTemplateColumns: cols,
					actualColumnCount: cols.split(' ').filter(c => c && c !== '0px').length,
					cssGridTemplateColumns: cols
				};
			});
			console.log(`   CSS: ${results.gridLayout.gridTemplateColumns}`);
			console.log(`   Actual columns: ${results.gridLayout.actualColumnCount}`);
		}

		// Take screenshot of grid layout (annotated if possible)
		const gridBox = await grid.boundingBox();
		if (gridBox) {
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/desktop-05-grid-layout.png`,
				clip: {
					x: 0,
					y: gridBox.y - 50,
					width: 1920,
					height: Math.min(gridBox.height + 100, 1080)
				}
			});
		}

		// Console errors
		console.log('\n6. Console messages:');
		results.consoleErrors = consoleMessages;
		if (consoleMessages.length === 0) {
			console.log('   No errors or warnings');
		} else {
			consoleMessages.forEach(msg => {
				console.log(`   [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

	} catch (error) {
		console.error('Error during desktop testing:', error);
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-ERROR.png`,
			fullPage: true
		});
	} finally {
		await context.close();
	}

	return results;
}

async function testMobileViewport(browser) {
	console.log('\n=== MOBILE TESTING (375x667) ===\n');

	const context = await browser.newContext({
		viewport: { width: 375, height: 667 },
		isMobile: true,
		hasTouch: true
	});
	const page = await context.newPage();

	// Capture console messages
	const consoleMessages = [];
	page.on('console', msg => {
		const type = msg.type();
		if (type === 'error' || type === 'warning') {
			consoleMessages.push({ type, text: msg.text() });
		}
	});

	const results = {
		consoleErrors: []
	};

	try {
		console.log('1. Navigating to /initiatives...');
		await page.goto(`${BASE_URL}/initiatives`, { waitUntil: 'networkidle' });
		await page.waitForTimeout(2000);

		console.log('2. Taking initial screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-01-initial-fullpage.png`,
			fullPage: true
		});

		// Take viewport screenshot
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-02-initial-viewport.png`,
			fullPage: false
		});

		// Check stat cards on mobile
		console.log('3. Checking stat cards layout...');
		const statCards = await page.locator('.stats-row-card').all();
		console.log(`   Found ${statCards.length} stat cards`);

		// Check if stat cards are stacked or scrollable
		const statsRow = page.locator('.stats-row').first();
		if (await statsRow.count() > 0) {
			const statsRowStyles = await statsRow.evaluate(el => {
				const computed = window.getComputedStyle(el);
				return {
					display: computed.display,
					flexDirection: computed.flexDirection,
					overflowX: computed.overflowX,
					width: computed.width
				};
			});
			console.log(`   Stats row display: ${statsRowStyles.display}`);
			console.log(`   Stats row flex direction: ${statsRowStyles.flexDirection}`);
			console.log(`   Stats row overflow-x: ${statsRowStyles.overflowX}`);

			// Take screenshot of stats row
			const box = await statsRow.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/mobile-03-stat-cards.png`,
					clip: {
						x: 0,
						y: Math.max(0, box.y - 20),
						width: 375,
						height: Math.min(box.height + 40, 667)
					}
				});
			}
		}

		// Check initiative cards
		console.log('4. Checking initiative cards...');
		const initiativeCards = await page.locator('.initiative-card').all();
		console.log(`   Found ${initiativeCards.length} initiative cards`);

		// Check if cards are single column
		const grid = page.locator('.initiatives-view-grid').first();
		if (await grid.count() > 0) {
			const gridStyles = await grid.evaluate(el => {
				const computed = window.getComputedStyle(el);
				return {
					gridTemplateColumns: computed.gridTemplateColumns,
					gap: computed.gap
				};
			});
			console.log(`   Grid template columns: ${gridStyles.gridTemplateColumns}`);
			console.log(`   Grid gap: ${gridStyles.gap}`);
		}

		// Test header responsiveness
		console.log('5. Checking header layout...');
		const header = page.locator('.initiatives-view-header').first();
		if (await header.count() > 0) {
			const headerStyles = await header.evaluate(el => {
				const computed = window.getComputedStyle(el);
				return {
					flexDirection: computed.flexDirection,
					alignItems: computed.alignItems
				};
			});
			console.log(`   Header flex direction: ${headerStyles.flexDirection}`);
			console.log(`   Header align items: ${headerStyles.alignItems}`);

			const box = await header.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/mobile-04-header.png`,
					clip: {
						x: 0,
						y: 0,
						width: 375,
						height: box.height + 20
					}
				});
			}
		}

		// Take screenshot of first initiative card
		if (initiativeCards.length > 0) {
			const firstCard = initiativeCards[0];
			const box = await firstCard.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/mobile-05-first-card.png`,
					clip: {
						x: Math.max(0, box.x - 10),
						y: Math.max(0, box.y - 10),
						width: Math.min(box.width + 20, 375),
						height: box.height + 20
					}
				});
			}
		}

		// Console errors
		console.log('\n6. Console messages:');
		results.consoleErrors = consoleMessages;
		if (consoleMessages.length === 0) {
			console.log('   No errors or warnings');
		} else {
			consoleMessages.forEach(msg => {
				console.log(`   [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

	} catch (error) {
		console.error('Error during mobile testing:', error);
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-ERROR.png`,
			fullPage: true
		});
	} finally {
		await context.close();
	}

	return results;
}

async function main() {
	console.log('QA Testing: Initiatives Page');
	console.log('================================');

	await ensureDir(SCREENSHOT_DIR);

	const browser = await chromium.launch({ headless: true });

	try {
		const desktopResults = await testDesktopViewport(browser);
		const mobileResults = await testMobileViewport(browser);

		console.log('\n=== SUMMARY ===\n');
		console.log(`Screenshots saved to: ${SCREENSHOT_DIR}`);
		console.log(`\nDesktop:`);
		console.log(`  - Stat cards: ${desktopResults.statCards.length}`);
		console.log(`  - Initiative cards: ${desktopResults.initiativeCards.length}`);
		console.log(`  - Grid columns: ${desktopResults.gridLayout?.actualColumnCount || 'N/A'}`);
		console.log(`  - Console errors: ${desktopResults.consoleErrors.length}`);
		console.log(`\nMobile:`);
		console.log(`  - Console errors: ${mobileResults.consoleErrors.length}`);

		// Check for QA issues
		console.log('\n=== QA FINDINGS ===\n');

		// QA-001: Check for task trend (tasksThisWeek)
		const totalTasksCard = desktopResults.statCards.find(c => c.label === 'Total Tasks');
		if (totalTasksCard && !totalTasksCard.hasTrend) {
			console.log('QA-001 STILL_PRESENT: Total Tasks card has no trend indicator');
		}

		// QA-002: Check if any stat cards have trends
		const cardsWithTrends = desktopResults.statCards.filter(c => c.hasTrend).length;
		if (cardsWithTrends === 0) {
			console.log('QA-002 STILL_PRESENT: No stat cards show trend indicators');
		}

		// QA-003: Check initiative cards for time remaining
		for (const card of desktopResults.initiativeCards) {
			const hasTimeRemaining = card.metaItems.some(item =>
				item && (item.includes('remaining') || item.includes('Est.'))
			);
			if (!hasTimeRemaining) {
				console.log(`QA-003 STILL_PRESENT: Initiative "${card.title}" missing time remaining`);
				break; // Just report once
			}
		}

		// QA-004: Check grid column count
		if (desktopResults.gridLayout && desktopResults.gridLayout.actualColumnCount > 2) {
			console.log(`QA-004 STILL_PRESENT: Grid has ${desktopResults.gridLayout.actualColumnCount} columns (expected: 2)`);
		}

	} finally {
		await browser.close();
	}
}

main().catch(console.error);
