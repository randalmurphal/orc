/**
 * Playwright configuration for running E2E tests against React app
 *
 * This config reuses the same test files but runs against the React app on port 5174
 * for dual-run validation during the migration.
 */
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 1,
	workers: process.env.CI ? 1 : undefined,
	reporter: [
		['list'],
		['html', { outputFolder: 'playwright-report-react' }],
		['json', { outputFile: 'test-results-react/results.json' }],
	],
	outputDir: 'test-results-react',
	/* Snapshot/visual comparison settings */
	snapshotDir: './e2e/__snapshots__',
	snapshotPathTemplate: '{snapshotDir}/{testFileDir}/{testFileName}-snapshots/{arg}{ext}',
	expect: {
		toHaveScreenshot: {
			maxDiffPixels: 1000, // Allow minor anti-aliasing differences
			threshold: 0.2, // 20% pixel color difference threshold
		},
		toMatchSnapshot: {
			maxDiffPixelRatio: 0.02, // 2% of pixels can differ
		},
	},
	use: {
		// React app runs on port 5174
		baseURL: 'http://localhost:5174',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
	},
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
			testIgnore: /visual\.spec\.ts$/, // Visual tests use the 'visual' project
		},
		/* Visual regression tests - single browser, consistent viewport */
		{
			name: 'visual',
			testMatch: /visual\.spec\.ts$/,
			use: {
				...devices['Desktop Chrome'],
				viewport: { width: 1440, height: 900 },
				deviceScaleFactor: 2, // @2x for retina-quality screenshots
			},
		},
	],
	webServer: [
		{
			command: 'cd .. && ./bin/orc serve',
			url: 'http://localhost:8080/api/health',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
		{
			command: 'cd ../web-react && npm run dev',
			url: 'http://localhost:5174',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
	],
});
