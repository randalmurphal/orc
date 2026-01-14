import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 1, // Add 1 retry for local runs to handle flaky UI tests
	workers: process.env.CI ? 1 : undefined,
	reporter: 'html',
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
		baseURL: 'http://localhost:5173',
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
			command: 'npm run dev',
			url: 'http://localhost:5173',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
	],
});
