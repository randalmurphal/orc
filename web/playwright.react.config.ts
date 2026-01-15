/**
 * Playwright configuration for React app E2E testing
 *
 * This is a dual-run configuration that uses the same tests against the React app (port 5174).
 * Run with: bunx playwright test --config=playwright.react.config.ts
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
	/* Snapshot/visual comparison settings - use same baselines */
	snapshotDir: './e2e/__snapshots__',
	snapshotPathTemplate: '{snapshotDir}/{testFileDir}/{testFileName}-snapshots/{arg}{ext}',
	expect: {
		toHaveScreenshot: {
			maxDiffPixels: 1000,
			threshold: 0.2,
		},
		toMatchSnapshot: {
			maxDiffPixelRatio: 0.02,
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
			testIgnore: /visual\.spec\.ts$/,
		},
		{
			name: 'visual',
			testMatch: /visual\.spec\.ts$/,
			use: {
				...devices['Desktop Chrome'],
				viewport: { width: 1440, height: 900 },
				deviceScaleFactor: 2,
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
			command: 'cd ../web-react && bun run dev',
			url: 'http://localhost:5174',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
	],
});
