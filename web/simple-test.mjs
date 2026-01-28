// Simple connectivity test
import { chromium } from '@playwright/test';

async function test() {
	console.log('Testing connection to http://localhost:5173/initiatives...');

	const browser = await chromium.launch({ headless: true });
	const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
	const page = await context.newPage();

	try {
		const response = await page.goto('http://localhost:5173/initiatives', {
			waitUntil: 'networkidle',
			timeout: 10000
		});

		console.log(`Status: ${response.status()}`);
		console.log(`URL: ${response.url()}`);

		const title = await page.title();
		console.log(`Page title: ${title}`);

		console.log('\nConnection successful!');
	} catch (error) {
		console.error('Connection failed:', error.message);
		process.exit(1);
	} finally {
		await browser.close();
	}
}

test();
