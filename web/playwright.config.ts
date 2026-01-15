import { defineConfig, devices } from '@playwright/test';

/**
 * CRITICAL: E2E tests run against an ISOLATED SANDBOX project, NOT the real orc project.
 *
 * Tests perform real actions (drag-drop, clicks, API calls) that modify task statuses.
 * Running against production data WILL corrupt real task states.
 *
 * The sandbox is created by global-setup.ts and cleaned up by global-teardown.ts.
 * Tests should import from './e2e/fixtures' instead of '@playwright/test' to ensure
 * the sandbox project is selected automatically.
 *
 * See web/e2e/global-setup.ts for sandbox creation details.
 */
export default defineConfig({
	testDir: './e2e',
	globalSetup: './e2e/global-setup.ts',
	globalTeardown: './e2e/global-teardown.ts',
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
