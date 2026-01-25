/**
 * Playwright configuration for React frontend E2E testing
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

// Limit workers to prevent OOM when running parallel orc tasks.
// Each orc worker spawns Playwright which can spawn multiple browser processes.
// With unlimited workers on a 16-core machine, 3 parallel orc tasks could spawn
// 3 * 16 = 48 browser processes, exhausting memory.
const DEFAULT_WORKERS = 4;

export default defineConfig({
	testDir: path.resolve(__dirname, 'e2e'),
	globalSetup: path.resolve(__dirname, 'e2e/global-setup.ts'),
	globalTeardown: path.resolve(__dirname, 'e2e/global-teardown.ts'),
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 1, // Add 1 retry for local runs to handle flaky UI tests
	workers: process.env.CI ? 1 : DEFAULT_WORKERS,
	reporter: [
		['list'],
		['html', { outputFolder: 'playwright-report' }],
		['json', { outputFile: 'test-results/results.json' }],
	],
	/* Output directory for test artifacts (screenshots, traces) */
	outputDir: 'test-results',
	/* Snapshot/visual comparison settings */
	snapshotDir: 'e2e/__snapshots__',
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
			command: 'bun run dev',
			url: 'http://localhost:5173',
			reuseExistingServer: !process.env.CI,
			timeout: 30000,
		},
	],
});
