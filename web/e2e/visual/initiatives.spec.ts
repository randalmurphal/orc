/**
 * Initiatives View Visual Verification Tests
 *
 * Verifies that the Initiatives page layout matches the reference design.
 * Reference: example_ui/initiatives-dashboard.png
 *
 * Success Criteria:
 * - SC-1: Initiative cards have minimum width of 360px
 * - SC-2: Grid produces 2-column layout at 1440px viewport
 * - SC-3: Stats row displays with 4-column grid
 * - SC-4: Visual regression screenshot baseline
 * - SC-5: Existing unit tests pass (verified separately)
 * - SC-6: Card proportions match reference (icon 40px, name 15px, bar 6px, padding 20px)
 *
 * Run with: cd web && bunx playwright test e2e/visual/initiatives.spec.ts
 */
import { test, expect } from '../fixtures';
import type { Page } from '@playwright/test';

// =============================================================================
// Test Utilities
// =============================================================================

/**
 * Disables CSS animations for deterministic screenshots
 */
async function disableAnimations(page: Page) {
	await page.addStyleTag({
		content: `
			*, *::before, *::after {
				animation-duration: 0s !important;
				animation-delay: 0s !important;
				transition-duration: 0s !important;
				transition-delay: 0s !important;
			}
			.skeleton, .shimmer, .loading-shimmer {
				animation: none !important;
				background: var(--bg-surface) !important;
			}
		`,
	});
}

/**
 * Returns locators for dynamic content that should be masked in screenshots
 */
function getDynamicContentMasks(page: Page) {
	return [
		page.locator('.stats-row-card-value'),
		page.locator('.stats-row-card-trend'),
		page.locator('.initiative-card-progress-value'),
		page.locator('.initiative-card-meta-item'),
		page.locator('.timestamp'),
		page.locator('[data-timestamp]'),
	];
}

/**
 * Waits for Initiatives page to fully load
 */
async function waitForInitiativesLoaded(page: Page) {
	await page.waitForLoadState('domcontentloaded');
	// Wait for content: initiative cards, empty state, or error
	await page
		.waitForSelector(
			'.initiatives-view-grid .initiative-card, .initiatives-view-empty, .initiatives-view-error',
			{
				state: 'visible',
				timeout: 10000,
			}
		)
		.catch(() => {});
	// Small buffer for final renders
	await page.waitForTimeout(200);
}

// =============================================================================
// Test Suite
// =============================================================================

test.describe('Initiatives View Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/initiatives');
		await waitForInitiativesLoaded(page);
	});

	// =========================================================================
	// SC-1: Card Minimum Width (360px)
	// =========================================================================

	test.describe('SC-1: Card minimum width enforcement', () => {
		test('initiative cards are at least 360px wide at 1440px viewport', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count > 0) {
				for (let i = 0; i < count; i++) {
					const box = await cards.nth(i).boundingBox();
					expect(box).not.toBeNull();
					if (box) {
						expect(box.width).toBeGreaterThanOrEqual(360);
					}
				}
			}
		});

		test('cards take full width when viewport is narrower than 360px', async ({
			page,
		}) => {
			// BDD-2: Mobile viewport - single column, full width
			await page.setViewportSize({ width: 320, height: 800 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count > 0) {
				const card = cards.first();
				const box = await card.boundingBox();
				expect(box).not.toBeNull();
				if (box) {
					// Card should fill available width (minus padding)
					// At 320px viewport with 16px padding on each side = 288px available
					// Card should take up most of the available width
					expect(box.width).toBeGreaterThan(250);
				}
			}
		});

		test('cards enforce 360px minimum in grid at 1024px viewport', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1024, height: 768 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count > 0) {
				const box = await cards.first().boundingBox();
				expect(box).not.toBeNull();
				if (box) {
					expect(box.width).toBeGreaterThanOrEqual(360);
				}
			}
		});
	});

	// =========================================================================
	// SC-2: 2-Column Grid Layout at 1440px
	// =========================================================================

	test.describe('SC-2: Grid column layout', () => {
		test('displays 2-column grid at 1440px viewport with 4+ initiatives', async ({
			page,
		}) => {
			// BDD-1: 1440px viewport → 2 columns
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count >= 2) {
				// Get bounding boxes of first two cards
				const box1 = await cards.nth(0).boundingBox();
				const box2 = await cards.nth(1).boundingBox();

				expect(box1).not.toBeNull();
				expect(box2).not.toBeNull();

				if (box1 && box2) {
					// Cards should be side by side (same Y position, different X)
					// Allow 2px tolerance for sub-pixel rendering
					expect(Math.abs(box1.y - box2.y)).toBeLessThan(2);
					expect(box2.x).toBeGreaterThan(box1.x + box1.width - 1);
				}
			}
		});

		test('grid gap between cards is 16px', async ({ page }) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count >= 2) {
				const box1 = await cards.nth(0).boundingBox();
				const box2 = await cards.nth(1).boundingBox();

				if (box1 && box2 && Math.abs(box1.y - box2.y) < 2) {
					// Gap = start of card 2 - end of card 1
					const gap = box2.x - (box1.x + box1.width);
					// Allow ±2px tolerance for sub-pixel rendering
					expect(gap).toBeGreaterThanOrEqual(14);
					expect(gap).toBeLessThanOrEqual(18);
				}
			}
		});

		test('displays single column at mobile viewport (480px)', async ({
			page,
		}) => {
			// BDD-2: 480px viewport → single column
			await page.setViewportSize({ width: 480, height: 800 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count >= 2) {
				const box1 = await cards.nth(0).boundingBox();
				const box2 = await cards.nth(1).boundingBox();

				expect(box1).not.toBeNull();
				expect(box2).not.toBeNull();

				if (box1 && box2) {
					// Cards should be stacked vertically (card 2 below card 1)
					expect(box2.y).toBeGreaterThan(box1.y + box1.height - 1);
				}
			}
		});
	});

	// =========================================================================
	// SC-3: Stats Row 4-Column Grid
	// =========================================================================

	test.describe('SC-3: Stats row layout', () => {
		test('displays 4 stat cards in stats row', async ({ page }) => {
			// BDD-3: Stats row renders with 4 cards
			const statsRow = page.locator('.stats-row');
			await expect(statsRow).toBeVisible();

			const statCards = statsRow.locator('.stats-row-card');
			await expect(statCards).toHaveCount(4);
		});

		test('stat cards show correct labels: Active Initiatives, Total Tasks, Completion Rate, Total Cost', async ({
			page,
		}) => {
			const labels = page.locator('.stats-row-card-label');
			const count = await labels.count();

			if (count === 4) {
				await expect(labels.nth(0)).toHaveText('Active Initiatives');
				await expect(labels.nth(1)).toHaveText('Total Tasks');
				await expect(labels.nth(2)).toHaveText('Completion Rate');
				await expect(labels.nth(3)).toHaveText('Total Cost');
			}
		});

		test('stats row uses 4-column horizontal layout at desktop', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const statCards = page.locator('.stats-row .stats-row-card');
			const count = await statCards.count();

			if (count === 4) {
				const box1 = await statCards.nth(0).boundingBox();
				const box4 = await statCards.nth(3).boundingBox();

				expect(box1).not.toBeNull();
				expect(box4).not.toBeNull();

				if (box1 && box4) {
					// All 4 cards should be on the same row (same Y)
					expect(Math.abs(box1.y - box4.y)).toBeLessThan(2);
				}
			}
		});

		test('stats row uses 2x2 grid at tablet viewport (768px)', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 768, height: 1024 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const statCards = page.locator('.stats-row .stats-row-card');
			const count = await statCards.count();

			if (count === 4) {
				const box1 = await statCards.nth(0).boundingBox();
				const box3 = await statCards.nth(2).boundingBox();

				expect(box1).not.toBeNull();
				expect(box3).not.toBeNull();

				if (box1 && box3) {
					// Card 3 should be below card 1 (2x2 layout)
					expect(box3.y).toBeGreaterThan(box1.y + box1.height - 1);
				}
			}
		});

		test('stats row uses single column at mobile viewport (480px)', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 480, height: 800 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const statCards = page.locator('.stats-row .stats-row-card');
			const count = await statCards.count();

			if (count >= 2) {
				const box1 = await statCards.nth(0).boundingBox();
				const box2 = await statCards.nth(1).boundingBox();

				expect(box1).not.toBeNull();
				expect(box2).not.toBeNull();

				if (box1 && box2) {
					// Cards should be stacked vertically
					expect(box2.y).toBeGreaterThan(box1.y + box1.height - 1);
				}
			}
		});
	});

	// =========================================================================
	// SC-4: Visual Regression Screenshots
	// =========================================================================

	test.describe('SC-4: Visual regression snapshots', () => {
		test('full initiatives page matches baseline at 1440px', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);
			await disableAnimations(page);

			await expect(page).toHaveScreenshot('initiatives-view-full.png', {
				mask: getDynamicContentMasks(page),
				fullPage: true,
				maxDiffPixels: 500,
			});
		});

		test('initiatives page at mobile viewport matches baseline', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 480, height: 800 });
			await page.reload();
			await waitForInitiativesLoaded(page);
			await disableAnimations(page);

			await expect(page).toHaveScreenshot('initiatives-view-mobile.png', {
				mask: getDynamicContentMasks(page),
				fullPage: true,
				maxDiffPixels: 500,
			});
		});
	});

	// =========================================================================
	// SC-6: Card Proportions Match Reference
	// =========================================================================

	test.describe('SC-6: Card dimension verification', () => {
		test('initiative card icon is 40x40px', async ({ page }) => {
			const icon = page.locator('.initiative-card-icon').first();
			const isVisible = await icon.isVisible().catch(() => false);

			if (isVisible) {
				const box = await icon.boundingBox();
				expect(box).not.toBeNull();
				if (box) {
					expect(box.width).toBe(40);
					expect(box.height).toBe(40);
				}
			}
		});

		test('initiative card name uses 15px font', async ({ page }) => {
			const name = page.locator('.initiative-card-name').first();
			const isVisible = await name.isVisible().catch(() => false);

			if (isVisible) {
				const fontSize = await name.evaluate(
					(el) => getComputedStyle(el).fontSize
				);
				expect(fontSize).toBe('15px');
			}
		});

		test('progress bar height is 6px', async ({ page }) => {
			const progressBar = page
				.locator('.initiative-card-progress-bar')
				.first();
			const isVisible = await progressBar.isVisible().catch(() => false);

			if (isVisible) {
				const box = await progressBar.boundingBox();
				expect(box).not.toBeNull();
				if (box) {
					expect(box.height).toBe(6);
				}
			}
		});

		test('initiative card has 20px padding', async ({ page }) => {
			const card = page.locator('.initiative-card').first();
			const isVisible = await card.isVisible().catch(() => false);

			if (isVisible) {
				const padding = await card.evaluate((el) => {
					const style = getComputedStyle(el);
					return {
						top: style.paddingTop,
						right: style.paddingRight,
						bottom: style.paddingBottom,
						left: style.paddingLeft,
					};
				});
				expect(padding.top).toBe('20px');
				expect(padding.right).toBe('20px');
				expect(padding.bottom).toBe('20px');
				expect(padding.left).toBe('20px');
			}
		});

		test('initiative card has 12px border-radius', async ({ page }) => {
			const card = page.locator('.initiative-card').first();
			const isVisible = await card.isVisible().catch(() => false);

			if (isVisible) {
				const borderRadius = await card.evaluate(
					(el) => getComputedStyle(el).borderRadius
				);
				expect(borderRadius).toBe('12px');
			}
		});

		test('stats row card has 10px border-radius', async ({ page }) => {
			const card = page.locator('.stats-row-card').first();
			const isVisible = await card.isVisible().catch(() => false);

			if (isVisible) {
				const borderRadius = await card.evaluate(
					(el) => getComputedStyle(el).borderRadius
				);
				expect(borderRadius).toBe('10px');
			}
		});

		test('stats row card value uses 28px font', async ({ page }) => {
			const value = page.locator('.stats-row-card-value').first();
			const isVisible = await value.isVisible().catch(() => false);

			if (isVisible) {
				const fontSize = await value.evaluate(
					(el) => getComputedStyle(el).fontSize
				);
				expect(fontSize).toBe('28px');
			}
		});
	});

	// =========================================================================
	// Edge Cases
	// =========================================================================

	test.describe('Edge cases', () => {
		test('single initiative card is rendered without layout issues', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			// If only one card exists, it should still respect grid constraints
			if (count === 1) {
				const box = await cards.first().boundingBox();
				expect(box).not.toBeNull();
				if (box) {
					expect(box.width).toBeGreaterThanOrEqual(360);
				}
			}
		});

		test('grid handles many initiatives without breaking layout', async ({
			page,
		}) => {
			await page.setViewportSize({ width: 1440, height: 900 });
			await page.reload();
			await waitForInitiativesLoaded(page);

			const cards = page.locator('.initiatives-view-grid .initiative-card');
			const count = await cards.count();

			if (count > 2) {
				// All cards should maintain minimum width regardless of count
				for (let i = 0; i < Math.min(count, 6); i++) {
					const box = await cards.nth(i).boundingBox();
					if (box) {
						expect(box.width).toBeGreaterThanOrEqual(360);
					}
				}
			}
		});

		test('stats row has margin below separating it from card grid', async ({
			page,
		}) => {
			const statsRow = page.locator('.stats-row');
			const grid = page.locator('.initiatives-view-grid');

			const statsVisible = await statsRow.isVisible().catch(() => false);
			const gridVisible = await grid.isVisible().catch(() => false);

			if (statsVisible && gridVisible) {
				const statsBox = await statsRow.boundingBox();
				const gridBox = await grid.boundingBox();

				if (statsBox && gridBox) {
					const spacing = gridBox.y - (statsBox.y + statsBox.height);
					// Should have at least 20px spacing (CSS: margin-bottom: 24px)
					expect(spacing).toBeGreaterThanOrEqual(20);
				}
			}
		});
	});
});
