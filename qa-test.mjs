/**
 * QA Test Script for Initiatives Page
 * Tests desktop (1920x1080) and mobile (375x667) viewports
 */

import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const SCREENSHOT_DIR = join(__dirname, 'qa-screenshots');
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

	try {
		// Navigate to initiatives page
		console.log('1. Navigating to /initiatives...');
		await page.goto(`${BASE_URL}/initiatives`, { waitUntil: 'networkidle' });
		await page.waitForTimeout(1000); // Let animations settle

		// Take initial screenshot
		console.log('2. Taking initial screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-01-initial.png`,
			fullPage: true
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
			console.log(`   - ${label}: ${value} (trend: ${hasTrend ? 'YES' : 'NO'})`);
		}

		// Take screenshot of stat cards
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/desktop-02-stat-cards.png`,
			clip: { x: 0, y: 0, width: 1920, height: 300 }
		});

		// Check initiative cards
		console.log('4. Checking initiative cards...');
		const initiativeCards = await page.locator('.initiative-card').all();
		console.log(`   Found ${initiativeCards.length} initiative cards`);

		for (let i = 0; i < initiativeCards.length; i++) {
			const card = initiativeCards[i];
			const title = await card.locator('.initiative-card-name').textContent();
			const hasTimeRemaining = await card.locator('.initiative-card-meta-item').first().isVisible().catch(() => false);
			const metaItems = await card.locator('.initiative-card-meta-item').all();
			console.log(`   - ${title}: ${metaItems.length} meta items`);

			// Check if first meta item is time remaining (clock icon)
			if (metaItems.length > 0) {
				const firstMeta = await metaItems[0].textContent();
				console.log(`     First meta: ${firstMeta}`);
			}
		}

		// Take screenshot of first initiative card if any exist
		if (initiativeCards.length > 0) {
			const firstCard = initiativeCards[0];
			const box = await firstCard.boundingBox();
			if (box) {
				await page.screenshot({
					path: `${SCREENSHOT_DIR}/desktop-03-first-card.png`,
					clip: box
				});
			}
		}

		// Check grid layout columns
		console.log('5. Checking grid layout...');
		const grid = page.locator('.initiatives-view-grid');
		const gridStyles = await grid.evaluate(el => {
			const computed = window.getComputedStyle(el);
			return {
				gridTemplateColumns: computed.gridTemplateColumns,
				columnCount: computed.gridTemplateColumns.split(' ').length
			};
		});
		console.log(`   Grid template columns: ${gridStyles.gridTemplateColumns}`);
		console.log(`   Column count: ${gridStyles.columnCount}`);

		// Test empty search
		console.log('6. Testing search with no results...');
		const searchInput = page.locator('input[type="search"]');
		if (await searchInput.count() > 0) {
			await searchInput.fill('ZZZZZNONEXISTENT');
			await page.waitForTimeout(500);
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/desktop-04-empty-search.png`,
				fullPage: true
			});
			await searchInput.clear();
		} else {
			console.log('   No search input found');
		}

		// Console errors
		console.log('\n7. Console messages:');
		if (consoleMessages.length === 0) {
			console.log('   No errors or warnings');
		} else {
			consoleMessages.forEach(msg => {
				console.log(`   [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

	} finally {
		await context.close();
	}

	return consoleMessages;
}

async function testMobileViewport(browser) {
	console.log('\n=== MOBILE TESTING (375x667) ===\n');

	const context = await browser.newContext({
		viewport: { width: 375, height: 667 }
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

	try {
		console.log('1. Navigating to /initiatives...');
		await page.goto(`${BASE_URL}/initiatives`, { waitUntil: 'networkidle' });
		await page.waitForTimeout(1000);

		console.log('2. Taking initial screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-01-initial.png`,
			fullPage: true
		});

		// Check stat cards on mobile
		console.log('3. Checking stat cards layout...');
		const statCards = await page.locator('.stats-row-card').all();
		console.log(`   Found ${statCards.length} stat cards`);

		// Check if stat cards are stacked or scrollable
		const statsRow = page.locator('.stats-row');
		const statsRowStyles = await statsRow.evaluate(el => {
			const computed = window.getComputedStyle(el);
			return {
				flexDirection: computed.flexDirection,
				overflowX: computed.overflowX
			};
		});
		console.log(`   Stats row flex direction: ${statsRowStyles.flexDirection}`);
		console.log(`   Stats row overflow-x: ${statsRowStyles.overflowX}`);

		// Check initiative cards
		console.log('4. Checking initiative cards...');
		const initiativeCards = await page.locator('.initiative-card').all();
		console.log(`   Found ${initiativeCards.length} initiative cards`);

		// Check if cards are single column
		const grid = page.locator('.initiatives-view-grid');
		const gridStyles = await grid.evaluate(el => {
			const computed = window.getComputedStyle(el);
			return {
				gridTemplateColumns: computed.gridTemplateColumns
			};
		});
		console.log(`   Grid template columns: ${gridStyles.gridTemplateColumns}`);

		// Test header responsiveness
		console.log('5. Checking header layout...');
		const header = page.locator('.initiatives-view-header');
		const headerStyles = await header.evaluate(el => {
			const computed = window.getComputedStyle(el);
			return {
				flexDirection: computed.flexDirection
			};
		});
		console.log(`   Header flex direction: ${headerStyles.flexDirection}`);

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/mobile-02-header.png`,
			clip: { x: 0, y: 0, width: 375, height: 200 }
		});

		// Console errors
		console.log('\n6. Console messages:');
		if (consoleMessages.length === 0) {
			console.log('   No errors or warnings');
		} else {
			consoleMessages.forEach(msg => {
				console.log(`   [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

	} finally {
		await context.close();
	}

	return consoleMessages;
}

async function main() {
	console.log('QA Testing: Initiatives Page');
	console.log('================================');

	await ensureDir(SCREENSHOT_DIR);

	const browser = await chromium.launch();

	try {
		const desktopErrors = await testDesktopViewport(browser);
		const mobileErrors = await testMobileViewport(browser);

		console.log('\n=== SUMMARY ===\n');
		console.log(`Screenshots saved to: ${SCREENSHOT_DIR}`);
		console.log(`Desktop errors: ${desktopErrors.length}`);
		console.log(`Mobile errors: ${mobileErrors.length}`);

	} finally {
		await browser.close();
	}
}

main().catch(console.error);
