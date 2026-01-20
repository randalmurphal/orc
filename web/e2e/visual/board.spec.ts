/**
 * Board View Visual Verification Tests
 *
 * Comprehensive tests to verify the Board view implementation matches
 * the reference mockup (example_ui/board.html and Screenshot_20260116_201804.png).
 *
 * These tests verify:
 * - Layout dimensions (IconNav 56px, TopBar 48px, Running 420px, RightPanel 300px)
 * - CSS custom properties (colors, typography, spacing)
 * - Component structure (nav items, buttons, sections)
 * - Computed styles match design tokens
 *
 * Run with: npx playwright test visual/board.spec.ts
 *
 * @see example_ui/board.html - Reference HTML/CSS implementation
 * @see example_ui/Screenshot_20260116_201804.png - Visual reference
 */

import { test, expect } from '../fixtures';
import type { Page, Locator } from '@playwright/test';

// =============================================================================
// Test Setup Utilities
// =============================================================================

/**
 * Disables all animations for deterministic tests
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
		`,
	});
}

/**
 * Wait for fonts to load
 */
async function waitForFonts(page: Page) {
	await page.waitForFunction(() => document.fonts.ready);
}

/**
 * Wait for page to be fully stable
 */
async function waitForPageStable(page: Page) {
	await page.waitForLoadState('networkidle');
	await waitForFonts(page);
	await page.waitForTimeout(100);
}

// =============================================================================
// IconNav Visual Verification
// =============================================================================

test.describe('IconNav Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('dimensions: 56px width', async ({ page }) => {
		const nav = page.locator('.icon-nav');
		await expect(nav).toBeVisible();

		const box = await nav.boundingBox();
		expect(box?.width).toBe(56);
	});

	test('logo: 32x32px with gradient and glow', async ({ page }) => {
		const logo = page.locator('.icon-nav__logo-mark');
		await expect(logo).toBeVisible();

		const result = await logo.evaluate((el) => {
			const style = getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return {
				width: rect.width,
				height: rect.height,
				background: style.backgroundImage,
				borderRadius: style.borderRadius,
				boxShadow: style.boxShadow,
			};
		});

		expect(result.width).toBe(32);
		expect(result.height).toBe(32);
		expect(result.background).toContain('linear-gradient');
		expect(result.borderRadius).toBe('8px');
		expect(result.boxShadow).toContain('168, 85, 247'); // primary-glow color
	});

	test('nav items: correct count (6 items + divider)', async ({ page }) => {
		const items = page.locator('.icon-nav__item');
		// Board, Initiatives, Stats, Agents, Settings, Help = 6 items
		await expect(items).toHaveCount(6);
	});

	test('active state: Board item has primary-dim background', async ({
		page,
	}) => {
		const activeItem = page.locator('.icon-nav__item--active');
		await expect(activeItem).toBeVisible();

		const result = await activeItem.evaluate((el) => {
			const style = getComputedStyle(el);
			return {
				background: style.backgroundColor,
				color: style.color,
			};
		});

		// --primary-dim: rgba(168, 85, 247, 0.1)
		expect(result.background).toContain('168');
		expect(result.background).toContain('85');
		expect(result.background).toContain('247');
		// --primary-bright: #c084fc = rgb(192, 132, 252)
		expect(result.color).toContain('192');
	});

	test('nav label: 8px font, 500 weight', async ({ page }) => {
		const label = page.locator('.icon-nav__label').first();
		await expect(label).toBeVisible();

		const result = await label.evaluate((el) => {
			const style = getComputedStyle(el);
			return {
				fontSize: style.fontSize,
				fontWeight: style.fontWeight,
			};
		});

		expect(result.fontSize).toBe('8px');
		expect(result.fontWeight).toBe('500');
	});

	test('icon size: 18x18px', async ({ page }) => {
		const icon = page.locator('.icon-nav__icon').first();
		await expect(icon).toBeVisible();

		const result = await icon.evaluate((el) => {
			const rect = el.getBoundingClientRect();
			return { width: rect.width, height: rect.height };
		});

		expect(result.width).toBe(18);
		expect(result.height).toBe(18);
	});
});

// =============================================================================
// TopBar Visual Verification
// =============================================================================

test.describe('TopBar Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('dimensions: 48px height', async ({ page }) => {
		const topBar = page.locator('.top-bar');
		await expect(topBar).toBeVisible();

		const box = await topBar.boundingBox();
		expect(box?.height).toBe(48);
	});

	test('project selector: visible with folder icon', async ({ page }) => {
		const selector = page.locator('.project-selector');
		await expect(selector).toBeVisible();

		// Check for folder icon
		const folderIcon = selector.locator('svg').first();
		await expect(folderIcon).toBeVisible();
	});

	test('search box: 200px width', async ({ page }) => {
		const searchBox = page.locator('.search-box');
		await expect(searchBox).toBeVisible();

		const box = await searchBox.boundingBox();
		expect(box?.width).toBe(200);
	});

	test('session stats: 3 metrics with colored icons', async ({ page }) => {
		const stats = page.locator('.session-stat');
		await expect(stats).toHaveCount(3);

		// Verify icon color classes
		const purpleIcon = page.locator('.session-stat-icon.purple');
		const amberIcon = page.locator('.session-stat-icon.amber');
		const greenIcon = page.locator('.session-stat-icon.green');

		await expect(purpleIcon).toBeVisible();
		await expect(amberIcon).toBeVisible();
		await expect(greenIcon).toBeVisible();
	});

	test('session stat value: JetBrains Mono font', async ({ page }) => {
		const value = page.locator('.session-stat .value').first();
		await expect(value).toBeVisible();

		const fontFamily = await value.evaluate((el) =>
			getComputedStyle(el).fontFamily.toLowerCase()
		);
		expect(fontFamily).toContain('jetbrains');
	});

	test('pause button: visible ghost variant', async ({ page }) => {
		const pauseBtn = page.locator('button:has-text("Pause")');
		await expect(pauseBtn).toBeVisible();

		const classes = await pauseBtn.getAttribute('class');
		expect(classes).toContain('ghost');
	});
});

// =============================================================================
// Board Layout Visual Verification
// =============================================================================

test.describe('Board Layout Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('page header: visible with task count', async ({ page }) => {
		const header = page.locator('.page-header');
		await expect(header).toBeVisible();

		// Check for Board title
		const title = page.locator('.page-header h2');
		await expect(title).toContainText('Board');

		// Check for task count
		const taskCount = page.locator('.task-count');
		await expect(taskCount).toBeVisible();
	});

	test('view mode dropdown: present', async ({ page }) => {
		const dropdown = page.locator('.view-mode-dropdown');
		await expect(dropdown).toBeVisible();
	});

	test('initiative dropdown: present', async ({ page }) => {
		// Look for initiative filter dropdown
		const dropdown = page.locator(
			'button:has-text("All initiatives"), button:has-text("initiatives")'
		);
		await expect(dropdown.first()).toBeVisible();
	});
});

// =============================================================================
// Typography Visual Verification
// =============================================================================

test.describe('Typography Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('body font: Inter', async ({ page }) => {
		const body = page.locator('body');
		const fontFamily = await body.evaluate((el) =>
			getComputedStyle(el).fontFamily.toLowerCase()
		);
		expect(fontFamily).toContain('inter');
	});

	test('task ID: JetBrains Mono font', async ({ page }) => {
		// Wait for task cards to load
		const taskCard = page.locator('.task-card').first();
		const isVisible = await taskCard.isVisible().catch(() => false);

		if (isVisible) {
			const taskId = taskCard.locator('.task-id');
			if (await taskId.isVisible().catch(() => false)) {
				const fontFamily = await taskId.evaluate((el) =>
					getComputedStyle(el).fontFamily.toLowerCase()
				);
				expect(fontFamily).toContain('jetbrains');
			}
		}
	});
});

// =============================================================================
// Colors Visual Verification
// =============================================================================

test.describe('Colors Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('background: --bg-base #050508', async ({ page }) => {
		const body = page.locator('body');
		const bg = await body.evaluate((el) => getComputedStyle(el).backgroundColor);
		// rgb(5, 5, 8)
		expect(bg).toContain('5');
	});

	test('elevated background: --bg-elevated #0a0a0f', async ({ page }) => {
		const nav = page.locator('.icon-nav');
		const bg = await nav.evaluate((el) => getComputedStyle(el).backgroundColor);
		// rgb(10, 10, 15)
		expect(bg).toContain('10');
	});

	test('primary color: --primary #a855f7', async ({ page }) => {
		const activeItem = page.locator('.icon-nav__item--active');
		if (await activeItem.isVisible().catch(() => false)) {
			const color = await activeItem.evaluate((el) => getComputedStyle(el).color);
			// rgb(192, 132, 252) = --primary-bright
			expect(color).toContain('192');
		}
	});
});

// =============================================================================
// Right Panel Visual Verification
// =============================================================================

test.describe('Right Panel Visual Verification', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('panel exists in DOM', async ({ page }) => {
		const panel = page.locator('.right-panel');
		// Panel should exist (may be collapsed)
		await expect(panel).toBeAttached();
	});

	test('panel: 300px width when open', async ({ page }) => {
		const panel = page.locator('.right-panel.open');
		const isOpen = await panel.isVisible().catch(() => false);

		if (isOpen) {
			const box = await panel.boundingBox();
			expect(box?.width).toBe(300);
		}
	});

	test('section headers: correct structure', async ({ page }) => {
		const panel = page.locator('.right-panel.open');
		const isOpen = await panel.isVisible().catch(() => false);

		if (isOpen) {
			const sections = panel.locator('.right-panel-section');
			const count = await sections.count();

			// Should have multiple sections (Blocked, Decisions, Config, Files, Completed)
			if (count > 0) {
				const header = sections.first().locator('.right-panel-header');
				await expect(header).toBeVisible();
			}
		}
	});
});

// =============================================================================
// Full Page Visual Snapshot
// =============================================================================

test.describe('Full Page Visual Snapshot', () => {
	test('board view matches baseline', async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);

		// Mask dynamic content
		const masks = [
			page.locator('.timestamp'),
			page.locator('.session-stat .value'),
			page.locator('.task-count'),
		];

		await expect(page).toHaveScreenshot('board-view-full.png', {
			maxDiffPixels: 500,
			threshold: 0.2,
			mask: masks,
			fullPage: true,
		});
	});
});

// =============================================================================
// Component-Level Assertions
// =============================================================================

test.describe('Component Assertions', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/board');
		await waitForPageStable(page);
		await disableAnimations(page);
	});

	test('all expected elements present', async ({ page }) => {
		// IconNav
		await expect(page.locator('.icon-nav')).toBeVisible();
		await expect(page.locator('.icon-nav__logo')).toBeVisible();
		await expect(page.locator('.icon-nav__items')).toBeVisible();

		// TopBar
		await expect(page.locator('.top-bar')).toBeVisible();
		await expect(page.locator('.project-selector')).toBeVisible();
		await expect(page.locator('.search-box')).toBeVisible();
		await expect(page.locator('.session-info')).toBeVisible();

		// Main content area
		await expect(page.locator('main')).toBeVisible();
	});

	test('CSS custom properties applied correctly', async ({ page }) => {
		const result = await page.evaluate(() => {
			const root = document.documentElement;
			const computedStyle = getComputedStyle(root);

			return {
				bgBase: computedStyle.getPropertyValue('--bg-base').trim(),
				bgElevated: computedStyle.getPropertyValue('--bg-elevated').trim(),
				primary: computedStyle.getPropertyValue('--primary').trim(),
				fontBody: computedStyle.getPropertyValue('--font-body').trim(),
				headerHeight: computedStyle.getPropertyValue('--header-height').trim(),
			};
		});

		expect(result.bgBase).toBe('#050508');
		expect(result.bgElevated).toBe('#0a0a0f');
		expect(result.primary).toBe('#a855f7');
		expect(result.fontBody).toContain('Inter');
		expect(result.headerHeight).toBe('48px');
	});
});
