// Quick check if server is accessible
import { chromium } from '@playwright/test';

const BASE_URL = 'http://localhost:5173';

async function checkServer() {
	console.log(`Checking if ${BASE_URL} is accessible...`);

	const browser = await chromium.launch({ headless: true });
	const page = await browser.newPage();

	try {
		const response = await page.goto(`${BASE_URL}/initiatives`, {
			waitUntil: 'dom contentloaded',
			timeout: 5000
		});

		console.log(`✓ Server responded with status: ${response.status()}`);
		console.log(`✓ URL: ${response.url()}`);

		await browser.close();
		return true;
	} catch (error) {
		console.log(`✗ Server not accessible: ${error.message}`);
		await browser.close();
		return false;
	}
}

checkServer().then(success => {
	process.exit(success ? 0 : 1);
}).catch(err => {
	console.error('Fatal error:', err);
	process.exit(1);
});
