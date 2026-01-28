/**
 * Debug Script: Investigate Initiatives Page Timeout
 *
 * Purpose: Determine why /initiatives page is timing out
 * - Try home page first (baseline)
 * - Try different wait strategies
 * - Capture console errors
 * - Screenshot at each step
 */

import { chromium } from '@playwright/test';
import { mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const SCREENSHOT_DIR = join(__dirname, '../qa-screenshots-debug');
const BASE_URL = 'http://localhost:5173';

async function ensureDir(dir) {
	if (!existsSync(dir)) {
		await mkdir(dir, { recursive: true });
	}
}

async function testHomePage(browser) {
	console.log('\n=== STEP 1: Test Home Page (Baseline) ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	const consoleMessages = [];
	const errors = [];

	page.on('console', msg => {
		consoleMessages.push({ type: msg.type(), text: msg.text() });
	});

	page.on('pageerror', error => {
		errors.push(error.message);
	});

	try {
		console.log('Navigating to home page...');
		await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 10000 });
		console.log('✓ Home page loaded (DOMContentLoaded)');

		await page.waitForTimeout(2000);

		console.log('\nTaking screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/01-home-page.png`,
			fullPage: true
		});
		console.log('✓ Screenshot saved');

		console.log('\nConsole messages:');
		consoleMessages.forEach(msg => {
			if (msg.type === 'error' || msg.type === 'warning') {
				console.log(`  [${msg.type.toUpperCase()}] ${msg.text}`);
			}
		});

		if (errors.length > 0) {
			console.log('\nPage errors:');
			errors.forEach(err => console.log(`  ${err}`));
		}

		return { success: true, consoleMessages, errors };

	} catch (error) {
		console.error('✗ Home page failed:', error.message);
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/01-home-page-ERROR.png`,
			fullPage: true
		});
		return { success: false, error: error.message, consoleMessages, errors };
	} finally {
		await context.close();
	}
}

async function testInitiativesPageDirect(browser) {
	console.log('\n=== STEP 2: Direct Navigation to /initiatives ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	const consoleMessages = [];
	const errors = [];

	page.on('console', msg => {
		consoleMessages.push({ type: msg.type(), text: msg.text() });
	});

	page.on('pageerror', error => {
		errors.push(error.message);
	});

	try {
		console.log('Attempting direct navigation to /initiatives...');
		console.log('Wait strategy: domcontentloaded (10s timeout)');

		await page.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'domcontentloaded',
			timeout: 10000
		});
		console.log('✓ DOMContentLoaded fired');

		await page.waitForTimeout(2000);

		console.log('\nTaking screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/02-initiatives-direct.png`,
			fullPage: true
		});
		console.log('✓ Screenshot saved');

		// Check what's actually rendered
		console.log('\nChecking page state...');
		const pageState = await page.evaluate(() => {
			return {
				title: document.title,
				bodyText: document.body?.innerText?.substring(0, 200),
				hasStatsRow: !!document.querySelector('.stats-row'),
				hasInitiativeCards: !!document.querySelector('.initiative-card'),
				hasError: !!document.querySelector('[class*="error"]'),
				hasLoading: !!document.querySelector('[class*="skeleton"]')
			};
		});

		console.log('Page state:', JSON.stringify(pageState, null, 2));

		console.log('\nConsole messages:');
		const importantMessages = consoleMessages.filter(m =>
			m.type === 'error' || m.type === 'warning'
		);
		if (importantMessages.length === 0) {
			console.log('  No errors or warnings');
		} else {
			importantMessages.forEach(msg => {
				console.log(`  [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

		if (errors.length > 0) {
			console.log('\nPage errors:');
			errors.forEach(err => console.log(`  ${err}`));
		}

		return { success: true, pageState, consoleMessages, errors };

	} catch (error) {
		console.error('✗ Direct navigation failed:', error.message);

		// Try to get screenshot of error state
		try {
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/02-initiatives-direct-ERROR.png`,
				fullPage: true
			});
			console.log('Screenshot of error state saved');
		} catch (e) {
			console.log('Could not capture error screenshot');
		}

		return { success: false, error: error.message, consoleMessages, errors };
	} finally {
		await context.close();
	}
}

async function testInitiativesPageLoad(browser) {
	console.log('\n=== STEP 3: Direct Navigation with "load" Wait ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	const consoleMessages = [];
	const errors = [];

	page.on('console', msg => {
		consoleMessages.push({ type: msg.type(), text: msg.text() });
	});

	page.on('pageerror', error => {
		errors.push(error.message);
	});

	try {
		console.log('Attempting navigation with "load" event (15s timeout)...');

		await page.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'load',
			timeout: 15000
		});
		console.log('✓ Page load event fired');

		await page.waitForTimeout(2000);

		console.log('\nTaking screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/03-initiatives-load.png`,
			fullPage: true
		});
		console.log('✓ Screenshot saved');

		console.log('\nConsole messages:');
		const importantMessages = consoleMessages.filter(m =>
			m.type === 'error' || m.type === 'warning'
		);
		if (importantMessages.length === 0) {
			console.log('  No errors or warnings');
		} else {
			importantMessages.forEach(msg => {
				console.log(`  [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

		return { success: true, consoleMessages, errors };

	} catch (error) {
		console.error('✗ Load wait failed:', error.message);

		try {
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/03-initiatives-load-ERROR.png`,
				fullPage: true
			});
		} catch (e) {
			console.log('Could not capture error screenshot');
		}

		return { success: false, error: error.message, consoleMessages, errors };
	} finally {
		await context.close();
	}
}

async function testInitiativesPageNetworkIdle(browser) {
	console.log('\n=== STEP 4: Direct Navigation with "networkidle" ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	const consoleMessages = [];
	const errors = [];

	page.on('console', msg => {
		consoleMessages.push({ type: msg.type(), text: msg.text() });
	});

	page.on('pageerror', error => {
		errors.push(error.message);
	});

	try {
		console.log('Attempting navigation with "networkidle" (30s timeout)...');
		console.log('This is where the original script timed out...');

		await page.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'networkidle',
			timeout: 30000
		});
		console.log('✓ Network idle achieved');

		await page.waitForTimeout(2000);

		console.log('\nTaking screenshot...');
		await page.screenshot({
			path: `${SCREENSHOT_DIR}/04-initiatives-networkidle.png`,
			fullPage: true
		});
		console.log('✓ Screenshot saved');

		console.log('\nConsole messages:');
		const importantMessages = consoleMessages.filter(m =>
			m.type === 'error' || m.type === 'warning'
		);
		if (importantMessages.length === 0) {
			console.log('  No errors or warnings');
		} else {
			importantMessages.forEach(msg => {
				console.log(`  [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

		return { success: true, consoleMessages, errors };

	} catch (error) {
		console.error('✗ Network idle wait TIMED OUT:', error.message);
		console.log('\nThis confirms the issue - page never reaches networkidle state');

		try {
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/04-initiatives-networkidle-TIMEOUT.png`,
				fullPage: true
			});
			console.log('Screenshot of timeout state saved');
		} catch (e) {
			console.log('Could not capture timeout screenshot');
		}

		return { success: false, error: error.message, consoleMessages, errors, timedOut: true };
	} finally {
		await context.close();
	}
}

async function testNavigationFromHome(browser) {
	console.log('\n=== STEP 5: Navigation from Home Page ===\n');

	const context = await browser.newContext({
		viewport: { width: 1920, height: 1080 }
	});
	const page = await context.newPage();

	const consoleMessages = [];
	const errors = [];

	page.on('console', msg => {
		consoleMessages.push({ type: msg.type(), text: msg.text() });
	});

	page.on('pageerror', error => {
		errors.push(error.message);
	});

	try {
		console.log('Loading home page first...');
		await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 10000 });
		console.log('✓ Home page loaded');

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/05-before-navigation.png`,
			fullPage: true
		});

		console.log('\nLooking for Initiatives link...');
		const initiativesLink = page.locator('a[href="/initiatives"]').first();
		const linkExists = await initiativesLink.count() > 0;

		if (!linkExists) {
			console.log('✗ No Initiatives link found in navigation');
			return { success: false, error: 'No navigation link found' };
		}

		console.log('✓ Found Initiatives link, clicking...');
		await initiativesLink.click();

		console.log('Waiting for navigation (domcontentloaded, 10s)...');
		await page.waitForURL('**/initiatives', { timeout: 10000 });
		console.log('✓ Navigation complete');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `${SCREENSHOT_DIR}/05-after-navigation.png`,
			fullPage: true
		});
		console.log('✓ Screenshot saved');

		console.log('\nConsole messages:');
		const importantMessages = consoleMessages.filter(m =>
			m.type === 'error' || m.type === 'warning'
		);
		if (importantMessages.length === 0) {
			console.log('  No errors or warnings');
		} else {
			importantMessages.forEach(msg => {
				console.log(`  [${msg.type.toUpperCase()}] ${msg.text}`);
			});
		}

		return { success: true, consoleMessages, errors };

	} catch (error) {
		console.error('✗ Navigation from home failed:', error.message);

		try {
			await page.screenshot({
				path: `${SCREENSHOT_DIR}/05-navigation-ERROR.png`,
				fullPage: true
			});
		} catch (e) {
			console.log('Could not capture error screenshot');
		}

		return { success: false, error: error.message, consoleMessages, errors };
	} finally {
		await context.close();
	}
}

async function main() {
	console.log('╔═══════════════════════════════════════════════════════╗');
	console.log('║  DEBUG: Initiatives Page Timeout Investigation        ║');
	console.log('╚═══════════════════════════════════════════════════════╝');

	await ensureDir(SCREENSHOT_DIR);

	const browser = await chromium.launch({
		headless: true,
		timeout: 60000
	});

	const results = {
		homePage: null,
		directDomContentLoaded: null,
		directLoad: null,
		directNetworkIdle: null,
		navigationFromHome: null
	};

	try {
		results.homePage = await testHomePage(browser);
		results.directDomContentLoaded = await testInitiativesPageDirect(browser);
		results.directLoad = await testInitiativesPageLoad(browser);
		results.directNetworkIdle = await testInitiativesPageNetworkIdle(browser);
		results.navigationFromHome = await testNavigationFromHome(browser);

		console.log('\n╔═══════════════════════════════════════════════════════╗');
		console.log('║  INVESTIGATION SUMMARY                                ║');
		console.log('╚═══════════════════════════════════════════════════════╝');

		console.log('\nTest Results:');
		console.log(`  1. Home page:                    ${results.homePage?.success ? '✓ PASS' : '✗ FAIL'}`);
		console.log(`  2. /initiatives (domcontentloaded): ${results.directDomContentLoaded?.success ? '✓ PASS' : '✗ FAIL'}`);
		console.log(`  3. /initiatives (load):          ${results.directLoad?.success ? '✓ PASS' : '✗ FAIL'}`);
		console.log(`  4. /initiatives (networkidle):   ${results.directNetworkIdle?.success ? '✓ PASS' : '✗ FAIL'}`);
		console.log(`  5. Navigate from home:           ${results.navigationFromHome?.success ? '✓ PASS' : '✗ FAIL'}`);

		console.log(`\nScreenshots saved to: ${SCREENSHOT_DIR}`);

		// Determine root cause
		console.log('\n╔═══════════════════════════════════════════════════════╗');
		console.log('║  ROOT CAUSE ANALYSIS                                  ║');
		console.log('╚═══════════════════════════════════════════════════════╝\n');

		if (!results.homePage?.success) {
			console.log('CRITICAL: Home page won\'t load - server issue');
		} else if (!results.directDomContentLoaded?.success) {
			console.log('CRITICAL: /initiatives route broken - page won\'t load at all');
		} else if (!results.directLoad?.success) {
			console.log('HIGH: Page loads DOM but "load" event never fires - resource issue');
		} else if (!results.directNetworkIdle?.success && results.directNetworkIdle?.timedOut) {
			console.log('CONFIRMED: Page loads but network never idles');
			console.log('CAUSE: Likely WebSocket or long-polling connection');
			console.log('IMPACT: Tests using "networkidle" will timeout');
			console.log('\nRECOMMENDATION: Use "domcontentloaded" or "load" wait strategy');
			console.log('WebSocket connections prevent networkidle, this is expected behavior');
		} else {
			console.log('All tests passed - no timeout issue detected');
		}

		// Check for console errors
		const allErrors = [
			...(results.homePage?.errors || []),
			...(results.directDomContentLoaded?.errors || []),
			...(results.directLoad?.errors || []),
			...(results.directNetworkIdle?.errors || []),
			...(results.navigationFromHome?.errors || [])
		];

		if (allErrors.length > 0) {
			console.log('\n⚠️  JavaScript Errors Detected:');
			[...new Set(allErrors)].forEach(err => console.log(`  - ${err}`));
		}

	} finally {
		await browser.close();
	}

	console.log('\n✓ Investigation complete');
}

main().catch(console.error);
