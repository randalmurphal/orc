/**
 * Preferences E2E Tests - CRITICAL: Tests run against ISOLATED SANDBOX project.
 *
 * Tests the Preferences page functionality including:
 * - Theme switching (dark/light)
 * - Sidebar default state
 * - Board view mode
 * - Date format selection
 * - Persistence across page refreshes
 */
import { test, expect } from './fixtures';

test.describe('Preferences Page', () => {
	test.beforeEach(async ({ page }) => {
		// Clear preference localStorage keys before each test
		await page.addInitScript(() => {
			localStorage.removeItem('orc-theme');
			localStorage.removeItem('orc-sidebar-default');
			localStorage.removeItem('orc-board-view-mode');
			localStorage.removeItem('orc-date-format');
		});
		await page.goto('/preferences');
	});

	test('should render preferences page with all sections', async ({ page }) => {
		// Page title
		await expect(page.locator('h2')).toContainText('Preferences');

		// All sections should be present
		await expect(page.getByRole('heading', { name: /appearance/i })).toBeVisible();
		await expect(page.getByRole('heading', { name: /layout/i })).toBeVisible();
		await expect(page.getByRole('heading', { name: /date & time/i })).toBeVisible();
	});

	test.describe('Theme switching', () => {
		test('should default to dark theme', async ({ page }) => {
			const darkBtn = page.getByRole('button', { name: /dark/i });
			await expect(darkBtn).toHaveAttribute('aria-pressed', 'true');

			// Document should not have data-theme attribute for dark mode
			const dataTheme = await page.evaluate(() =>
				document.documentElement.getAttribute('data-theme')
			);
			expect(dataTheme).toBeNull();
		});

		test('should switch to light theme', async ({ page }) => {
			const lightBtn = page.getByRole('button', { name: /light/i });
			await lightBtn.click();

			await expect(lightBtn).toHaveAttribute('aria-pressed', 'true');

			// Document should have data-theme="light"
			const dataTheme = await page.evaluate(() =>
				document.documentElement.getAttribute('data-theme')
			);
			expect(dataTheme).toBe('light');
		});

		test('should show visible background color change', async ({ page }) => {
			// Get initial background color
			const initialBg = await page.evaluate(() =>
				getComputedStyle(document.body).backgroundColor
			);

			// Switch to light theme
			await page.getByRole('button', { name: /light/i }).click();

			// Get new background color
			const newBg = await page.evaluate(() =>
				getComputedStyle(document.body).backgroundColor
			);

			// Background should have changed
			expect(newBg).not.toBe(initialBg);
		});

		test('theme should persist after page refresh', async ({ page }) => {
			// Switch to light theme
			await page.getByRole('button', { name: /light/i }).click();

			// Refresh the page
			await page.reload();

			// Light theme should still be selected
			const lightBtn = page.getByRole('button', { name: /light/i });
			await expect(lightBtn).toHaveAttribute('aria-pressed', 'true');

			// Document should still have data-theme="light"
			const dataTheme = await page.evaluate(() =>
				document.documentElement.getAttribute('data-theme')
			);
			expect(dataTheme).toBe('light');
		});
	});

	test.describe('Sidebar default state', () => {
		test('should default to expanded', async ({ page }) => {
			const expandedBtn = page.getByRole('button', { name: /expanded/i });
			await expect(expandedBtn).toHaveAttribute('aria-pressed', 'true');
		});

		test('should switch to collapsed', async ({ page }) => {
			const collapsedBtn = page.getByRole('button', { name: /collapsed/i });
			await collapsedBtn.click();

			await expect(collapsedBtn).toHaveAttribute('aria-pressed', 'true');

			// Verify localStorage was updated
			const stored = await page.evaluate(() =>
				localStorage.getItem('orc-sidebar-default')
			);
			expect(stored).toBe('collapsed');
		});

		test('sidebar default should persist after refresh', async ({ page }) => {
			// Set to collapsed
			await page.getByRole('button', { name: /collapsed/i }).click();

			// Refresh
			await page.reload();

			// Should still be collapsed
			const collapsedBtn = page.getByRole('button', { name: /collapsed/i });
			await expect(collapsedBtn).toHaveAttribute('aria-pressed', 'true');
		});
	});

	test.describe('Board view mode', () => {
		test('should default to flat', async ({ page }) => {
			const flatBtn = page.getByRole('button', { name: /^flat$/i });
			await expect(flatBtn).toHaveAttribute('aria-pressed', 'true');
		});

		test('should switch to swimlane', async ({ page }) => {
			const swimlaneBtn = page.getByRole('button', { name: /swimlane/i });
			await swimlaneBtn.click();

			await expect(swimlaneBtn).toHaveAttribute('aria-pressed', 'true');

			// Verify localStorage
			const stored = await page.evaluate(() =>
				localStorage.getItem('orc-board-view-mode')
			);
			expect(stored).toBe('swimlane');
		});

		test('board view mode should persist after refresh', async ({ page }) => {
			// Set to swimlane
			await page.getByRole('button', { name: /swimlane/i }).click();

			// Refresh
			await page.reload();

			// Should still be swimlane
			const swimlaneBtn = page.getByRole('button', { name: /swimlane/i });
			await expect(swimlaneBtn).toHaveAttribute('aria-pressed', 'true');
		});
	});

	test.describe('Date format', () => {
		test('should default to relative', async ({ page }) => {
			const select = page.getByRole('combobox', { name: /date format/i });
			await expect(select).toHaveValue('relative');
		});

		test('should show date format preview', async ({ page }) => {
			// Preview section should exist
			await expect(page.getByText(/preview/i)).toBeVisible();
			await expect(page.getByText(/3 hours ago/i)).toBeVisible();
			await expect(page.getByText(/5 days ago/i)).toBeVisible();
		});

		test('should switch to absolute format', async ({ page }) => {
			const select = page.getByRole('combobox', { name: /date format/i });
			await select.selectOption('absolute');

			await expect(select).toHaveValue('absolute');

			// Verify localStorage
			const stored = await page.evaluate(() =>
				localStorage.getItem('orc-date-format')
			);
			expect(stored).toBe('absolute');
		});

		test('should switch to absolute24 format', async ({ page }) => {
			const select = page.getByRole('combobox', { name: /date format/i });
			await select.selectOption('absolute24');

			await expect(select).toHaveValue('absolute24');
		});

		test('date format should persist after refresh', async ({ page }) => {
			// Set to absolute24
			const select = page.getByRole('combobox', { name: /date format/i });
			await select.selectOption('absolute24');

			// Refresh
			await page.reload();

			// Should still be absolute24
			await expect(page.getByRole('combobox', { name: /date format/i })).toHaveValue(
				'absolute24'
			);
		});

		test('preview should update when format changes', async ({ page }) => {
			// Get initial preview content
			const preview = page.locator('.preview-content');
			const initialText = await preview.textContent();

			// Change to absolute format
			const select = page.getByRole('combobox', { name: /date format/i });
			await select.selectOption('absolute');

			// Preview should have different content now
			const newText = await preview.textContent();
			expect(newText).not.toBe(initialText);
		});
	});

	test.describe('Reset to defaults', () => {
		test('should reset all preferences to defaults', async ({ page }) => {
			// Change all preferences
			await page.getByRole('button', { name: /light/i }).click();
			await page.getByRole('button', { name: /collapsed/i }).click();
			await page.getByRole('button', { name: /swimlane/i }).click();
			await page.getByRole('combobox', { name: /date format/i }).selectOption('absolute24');

			// Click reset
			await page.getByRole('button', { name: /reset to defaults/i }).click();

			// Verify all defaults are restored
			await expect(page.getByRole('button', { name: /dark/i })).toHaveAttribute(
				'aria-pressed',
				'true'
			);
			await expect(page.getByRole('button', { name: /expanded/i })).toHaveAttribute(
				'aria-pressed',
				'true'
			);
			await expect(page.getByRole('button', { name: /^flat$/i })).toHaveAttribute(
				'aria-pressed',
				'true'
			);
			await expect(page.getByRole('combobox', { name: /date format/i })).toHaveValue(
				'relative'
			);
		});

		test('reset should clear localStorage', async ({ page }) => {
			// Change all preferences
			await page.getByRole('button', { name: /light/i }).click();
			await page.getByRole('button', { name: /collapsed/i }).click();

			// Click reset
			await page.getByRole('button', { name: /reset to defaults/i }).click();

			// Verify localStorage was cleared
			const stored = await page.evaluate(() => ({
				theme: localStorage.getItem('orc-theme'),
				sidebar: localStorage.getItem('orc-sidebar-default'),
				board: localStorage.getItem('orc-board-view-mode'),
				date: localStorage.getItem('orc-date-format'),
			}));

			expect(stored.theme).toBeNull();
			expect(stored.sidebar).toBeNull();
			expect(stored.board).toBeNull();
			expect(stored.date).toBeNull();
		});

		test('reset should also reset theme on document', async ({ page }) => {
			// Switch to light theme
			await page.getByRole('button', { name: /light/i }).click();

			// Verify light theme is applied
			let dataTheme = await page.evaluate(() =>
				document.documentElement.getAttribute('data-theme')
			);
			expect(dataTheme).toBe('light');

			// Click reset
			await page.getByRole('button', { name: /reset to defaults/i }).click();

			// Document should no longer have data-theme attribute
			dataTheme = await page.evaluate(() =>
				document.documentElement.getAttribute('data-theme')
			);
			expect(dataTheme).toBeNull();
		});
	});

	test.describe('Accessibility', () => {
		test('toggle groups should have proper ARIA labels', async ({ page }) => {
			await expect(page.getByRole('group', { name: /theme selection/i })).toBeVisible();
			await expect(
				page.getByRole('group', { name: /sidebar default state/i })
			).toBeVisible();
			await expect(page.getByRole('group', { name: /board view mode/i })).toBeVisible();
		});

		test('buttons should have aria-pressed attribute', async ({ page }) => {
			const darkBtn = page.getByRole('button', { name: /dark/i });
			const lightBtn = page.getByRole('button', { name: /light/i });

			// Dark should be pressed (default)
			await expect(darkBtn).toHaveAttribute('aria-pressed', 'true');
			await expect(lightBtn).toHaveAttribute('aria-pressed', 'false');

			// After clicking light
			await lightBtn.click();
			await expect(darkBtn).toHaveAttribute('aria-pressed', 'false');
			await expect(lightBtn).toHaveAttribute('aria-pressed', 'true');
		});

		test('date format select should have accessible label', async ({ page }) => {
			const select = page.getByRole('combobox', { name: /date format/i });
			await expect(select).toBeVisible();
			await expect(select).toHaveAttribute('aria-label', 'Date format');
		});
	});
});
