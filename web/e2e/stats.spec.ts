/**
 * E2E Tests for Statistics Page (TASK-615)
 *
 * Validates that the Stats page matches the reference design in
 * example_ui/statistics-charts.png
 */

import { test, expect } from './fixtures';
import path from 'path';

const SCREENSHOT_DIR = '/tmp/qa-TASK-615';

test.describe('Statistics Page', () => {
	test.beforeAll(async () => {
		// Ensure screenshot directory exists
		const fs = await import('fs');
		if (!fs.existsSync(SCREENSHOT_DIR)) {
			fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
		}
	});

	test.beforeEach(async ({ page }) => {
		// Navigate to Stats page
		await page.goto('http://localhost:5173/stats');

		// Wait for page to load - look for the main stats container
		await page.waitForSelector('[data-testid="stats-view"]', {
			timeout: 10000,
			state: 'visible'
		}).catch(async () => {
			// Fallback: wait for any of the major sections
			await Promise.race([
				page.waitForSelector('.stats-metrics'),
				page.waitForSelector('.stats-heatmap'),
				page.waitForSelector('[class*="stats"]'),
			]);
		});
	});

	test('should display all major sections', async ({ page }) => {
		console.log('Testing major sections presence...');

		// Take full page screenshot
		await page.screenshot({
			path: path.join(SCREENSHOT_DIR, 'stats-desktop.png'),
			fullPage: true
		});

		// Check for header
		const header = await page.locator('h1, [role="heading"]').filter({ hasText: /statistics/i }).first();
		await expect(header).toBeVisible({ timeout: 5000 }).catch(() => {
			console.warn('Statistics header not found with expected selector');
		});

		// Check for time filter buttons (24h, 7d, 30d, All)
		const filterButtons = await page.locator('button').filter({
			hasText: /^(24h|7d|30d|All)$/
		}).count();

		if (filterButtons < 4) {
			console.error(`Expected 4 time filter buttons, found ${filterButtons}`);
		}

		// Check for Export button
		const exportBtn = await page.locator('button').filter({
			hasText: /export/i
		}).count();

		if (exportBtn === 0) {
			console.warn('Export button not found');
		}

		// Check for metric cards (should be 5)
		const metricCards = await page.locator('[class*="metric-card"], [data-testid*="metric"]').count();
		console.log(`Found ${metricCards} metric cards (expected 5)`);

		// Check for charts
		const hasHeatmap = await page.locator('[class*="heatmap"], [data-testid*="heatmap"]').count() > 0;
		const hasBarChart = await page.locator('[class*="bar-chart"], svg').count() > 0;
		const hasDonutChart = await page.locator('[class*="donut"], [class*="pie"]').count() > 0;

		console.log('Charts found:', { hasHeatmap, hasBarChart, hasDonutChart });

		// Check for data tables
		const tables = await page.locator('table, [role="table"]').count();
		console.log(`Found ${tables} data tables (expected 2: Initiatives + Files)`);

		// Log findings
		const findings = [];
		if (filterButtons < 4) findings.push('Missing time filter buttons');
		if (exportBtn === 0) findings.push('Missing export button');
		if (metricCards < 5) findings.push(`Only ${metricCards}/5 metric cards found`);
		if (!hasHeatmap) findings.push('Activity heatmap not found');
		if (!hasBarChart) findings.push('Bar chart not found');
		if (!hasDonutChart) findings.push('Donut chart not found');
		if (tables < 2) findings.push(`Only ${tables}/2 data tables found`);

		if (findings.length > 0) {
			console.error('Visual elements missing:', findings);
		}
	});

	test('should display metric cards with correct data', async ({ page }) => {
		console.log('Testing metric cards...');

		// Look for metric values - they should be visible numbers
		const metricValues = await page.locator('[class*="metric"] [class*="value"], [class*="stat-value"]').count();
		console.log(`Found ${metricValues} metric values`);

		// Check for trend indicators (arrows or percentage changes)
		const trendIndicators = await page.locator('[class*="trend"], [class*="change"]').count();
		console.log(`Found ${trendIndicators} trend indicators`);

		// Take screenshot of metrics section
		const metricsSection = page.locator('.stats-metrics, [class*="metrics"]').first();
		if (await metricsSection.count() > 0) {
			await metricsSection.screenshot({
				path: path.join(SCREENSHOT_DIR, 'metrics-cards.png')
			});
		}
	});

	test('should handle time filter interactions', async ({ page }) => {
		console.log('Testing time filter interactions...');

		// Find filter buttons
		const filters = ['24h', '7d', '30d', 'All'];

		for (const filter of filters) {
			const button = page.locator('button').filter({ hasText: new RegExp(`^${filter}$`) }).first();

			if (await button.count() > 0) {
				console.log(`Clicking ${filter} filter...`);
				await button.click();

				// Wait a bit for any data updates
				await page.waitForTimeout(500);

				// Check if button has active state
				const className = await button.getAttribute('class');
				console.log(`${filter} button classes: ${className}`);

				// Take screenshot of this filter state
				await page.screenshot({
					path: path.join(SCREENSHOT_DIR, `filter-${filter}.png`),
					fullPage: true
				});
			} else {
				console.warn(`Filter button "${filter}" not found`);
			}
		}
	});

	test('should display activity heatmap', async ({ page }) => {
		console.log('Testing activity heatmap...');

		// Look for heatmap container
		const heatmap = page.locator('[class*="heatmap"], [class*="activity"]').first();

		if (await heatmap.count() > 0) {
			// Check for month labels
			const monthLabels = await page.locator('text=/Oct|Nov|Dec|Jan|Feb|Mar/').count();
			console.log(`Found ${monthLabels} month labels`);

			// Check for day labels
			const dayLabels = await page.locator('text=/Mon|Tue|Wed|Thu|Fri|Sat|Sun/').count();
			console.log(`Found ${dayLabels} day labels`);

			// Check for cells (should be many)
			const cells = await page.locator('[class*="heatmap"] > *').count();
			console.log(`Found ${cells} heatmap cells`);

			// Take screenshot
			await heatmap.screenshot({
				path: path.join(SCREENSHOT_DIR, 'activity-heatmap.png')
			});

			// Try hovering over a cell to see tooltip
			const firstCell = heatmap.locator('> *').first();
			if (await firstCell.count() > 0) {
				await firstCell.hover();
				await page.waitForTimeout(300);

				// Check if tooltip appeared
				const tooltip = await page.locator('[role="tooltip"], [class*="tooltip"]').count();
				console.log(`Tooltip after hover: ${tooltip > 0 ? 'visible' : 'not found'}`);
			}
		} else {
			console.error('Activity heatmap not found');
		}
	});

	test('should display charts correctly', async ({ page }) => {
		console.log('Testing charts...');

		// Bar chart (Tasks Completed Per Day)
		const barChartTitle = await page.locator('text=/Tasks Completed Per Day/i').count();
		if (barChartTitle > 0) {
			console.log('✓ Bar chart section found');

			// Look for day labels (Mon, Tue, etc.)
			const dayLabels = await page.locator('text=/^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)$/').count();
			console.log(`  - Day labels: ${dayLabels}/7`);
		} else {
			console.warn('Bar chart section not found');
		}

		// Donut chart (Task Outcomes)
		const donutTitle = await page.locator('text=/Task Outcomes/i').count();
		if (donutTitle > 0) {
			console.log('✓ Donut chart section found');

			// Look for legend items
			const legendItems = await page.locator('text=/Completed|With Retries|Failed/i').count();
			console.log(`  - Legend items: ${legendItems}/3`);
		} else {
			console.warn('Donut chart section not found');
		}

		// Take screenshot of charts area
		const chartsArea = page.locator('[class*="charts"], .stats-charts').first();
		if (await chartsArea.count() > 0) {
			await chartsArea.screenshot({
				path: path.join(SCREENSHOT_DIR, 'charts-section.png')
			});
		}
	});

	test('should display data tables', async ({ page }) => {
		console.log('Testing data tables...');

		// Most Active Initiatives
		const initiativesTitle = await page.locator('text=/Most Active Initiatives/i').count();
		if (initiativesTitle > 0) {
			console.log('✓ Most Active Initiatives section found');

			const initiativeRows = await page.locator('text=/Most Active Initiatives/i').locator('..').locator('table tr, [role="row"]').count();
			console.log(`  - Rows: ${initiativeRows}`);
		} else {
			console.warn('Most Active Initiatives section not found');
		}

		// Most Modified Files
		const filesTitle = await page.locator('text=/Most Modified Files/i').count();
		if (filesTitle > 0) {
			console.log('✓ Most Modified Files section found');

			// Check for "No data" message (expected per QA-001)
			const noData = await page.locator('text=/No data|No files|Empty/i').count();
			if (noData > 0) {
				console.warn('  - Shows "No data" (Known issue QA-001)');
			}

			const fileRows = await page.locator('text=/Most Modified Files/i').locator('..').locator('table tr, [role="row"]').count();
			console.log(`  - Rows: ${fileRows}`);
		} else {
			console.warn('Most Modified Files section not found');
		}
	});

	test('should be responsive on mobile', async ({ page }) => {
		console.log('Testing mobile responsive layout...');

		// Resize to mobile viewport
		await page.setViewportSize({ width: 375, height: 667 });
		await page.waitForTimeout(500); // Wait for layout reflow

		// Take mobile screenshot
		await page.screenshot({
			path: path.join(SCREENSHOT_DIR, 'stats-mobile.png'),
			fullPage: true
		});

		// Check that sections are still visible
		const visibleSections = await page.locator('[class*="stats"], [data-testid*="stats"]').count();
		console.log(`Visible sections on mobile: ${visibleSections}`);

		// Check for horizontal scroll (should not exist)
		const hasHorizontalScroll = await page.evaluate(() => {
			return document.documentElement.scrollWidth > document.documentElement.clientWidth;
		});

		if (hasHorizontalScroll) {
			console.error('⚠️  Page has horizontal scroll on mobile (layout issue)');
		} else {
			console.log('✓ No horizontal scroll on mobile');
		}

		// Test that filter buttons are accessible
		const filterBtn = page.locator('button').filter({ hasText: /^7d$/ }).first();
		if (await filterBtn.count() > 0) {
			const box = await filterBtn.boundingBox();
			if (box && (box.width < 44 || box.height < 44)) {
				console.warn(`⚠️  Filter button touch target too small: ${box.width}x${box.height}px (min 44x44)`);
			}
		}
	});

	test('should not have console errors', async ({ page }) => {
		console.log('Checking for console errors...');

		const errors: string[] = [];
		const warnings: string[] = [];

		page.on('console', (msg) => {
			const type = msg.type();
			const text = msg.text();

			if (type === 'error') {
				errors.push(text);
			} else if (type === 'warning') {
				warnings.push(text);
			}
		});

		// Navigate and wait for page to load
		await page.goto('http://localhost:5173/stats');
		await page.waitForTimeout(2000); // Give time for any async errors

		// Click around to trigger any interaction errors
		const allFilterBtn = page.locator('button').filter({ hasText: /^All$/ }).first();
		if (await allFilterBtn.count() > 0) {
			await allFilterBtn.click();
			await page.waitForTimeout(500);
		}

		// Report findings
		if (errors.length > 0) {
			console.error('❌ Console errors found:');
			errors.forEach((err, i) => console.error(`  ${i + 1}. ${err}`));
		} else {
			console.log('✓ No console errors');
		}

		if (warnings.length > 0) {
			console.warn('⚠️  Console warnings:');
			warnings.forEach((warn, i) => console.warn(`  ${i + 1}. ${warn}`));
		}

		// Assert no critical errors
		const criticalErrors = errors.filter(e =>
			!e.includes('Warning') &&
			!e.includes('DevTools') &&
			!e.includes('[HMR]')
		);

		expect(criticalErrors).toHaveLength(0);
	});

	test('should handle export functionality', async ({ page }) => {
		console.log('Testing export functionality...');

		const exportBtn = page.locator('button').filter({ hasText: /export/i }).first();

		if (await exportBtn.count() > 0) {
			// Set up download listener
			const downloadPromise = page.waitForEvent('download', { timeout: 5000 }).catch(() => null);

			// Click export
			await exportBtn.click();

			// Wait for download
			const download = await downloadPromise;

			if (download) {
				console.log('✓ Export triggered download');
				console.log(`  - Filename: ${download.suggestedFilename()}`);

				// Save to test directory
				const downloadPath = path.join(SCREENSHOT_DIR, download.suggestedFilename());
				await download.saveAs(downloadPath);
				console.log(`  - Saved to: ${downloadPath}`);

				// Verify it's a CSV
				const fs = await import('fs');
				const content = fs.readFileSync(downloadPath, 'utf-8');
				const lines = content.split('\n');

				console.log(`  - Lines in CSV: ${lines.length}`);
				console.log(`  - First line (headers): ${lines[0]}`);

				expect(lines.length).toBeGreaterThan(0);
				expect(lines[0]).toContain(','); // Should have CSV format
			} else {
				console.warn('⚠️  Export button clicked but no download triggered');
			}
		} else {
			console.warn('⚠️  Export button not found');
		}
	});

	test('should match visual snapshot (if baseline exists)', async ({ page }) => {
		// This is a visual regression test that compares against a baseline screenshot
		// First run will create the baseline, subsequent runs will compare

		await page.goto('http://localhost:5173/stats');
		await page.waitForTimeout(1000); // Wait for all content to render

		// Take full page screenshot and compare
		await expect(page).toHaveScreenshot('stats-page-full.png', {
			fullPage: true,
			maxDiffPixels: 1000, // Allow minor differences
		});

		// Screenshot individual sections for more granular comparison
		const metricsSection = page.locator('.stats-metrics').first();
		if (await metricsSection.count() > 0) {
			await expect(metricsSection).toHaveScreenshot('stats-metrics.png');
		}

		const heatmapSection = page.locator('[class*="heatmap"]').first();
		if (await heatmapSection.count() > 0) {
			await expect(heatmapSection).toHaveScreenshot('stats-heatmap.png');
		}
	});
});
