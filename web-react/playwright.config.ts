/**
 * Playwright configuration for React app E2E testing
 *
 * This config reuses the tests from web/e2e/ but runs against the React app on port 5174.
 * This enables dual-run validation to verify feature parity during migration.
 *
 * CRITICAL: E2E tests run against an ISOLATED SANDBOX project, NOT the real orc project.
 * Tests perform real actions (drag-drop, clicks, API calls) that modify task statuses.
 * Running against production data WILL corrupt real task states.
 *
 * The sandbox is created by global-setup.ts and cleaned up by global-teardown.ts.
 */
import { defineConfig, devices } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default defineConfig({
	// Point to the shared e2e tests in web/
	testDir: path.resolve(__dirname, '../web/e2e'),
	globalSetup: path.resolve(__dirname, '../web/e2e/global-setup.ts'),
	globalTeardown: path.resolve(__dirname, '../web/e2e/global-teardown.ts'),
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 1, // Add 1 retry for local runs to handle flaky UI tests
	workers: process.env.CI ? 1 : undefined,
	reporter: [
		['list'],
		['html', { outputFolder: 'playwright-report' }],
		['json', { outputFile: 'test-results/results.json' }],
	],
	/* Output directory for test artifacts (screenshots, traces) */
	outputDir: 'test-results',
	/* Snapshot/visual comparison settings */
	snapshotDir: '../web/e2e/__snapshots__',
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
			command: 'npm run dev',
			url: 'http://localhost:5174',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
	],
});
