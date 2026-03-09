import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

async function disableAnimations(page: Page) {
	await page.addStyleTag({
		content: `
			*, *::before, *::after {
				animation-duration: 0s !important;
				animation-delay: 0s !important;
				transition-duration: 0s !important;
				transition-delay: 0s !important;
				scroll-behavior: auto !important;
			}
		`,
	});
}

async function openBoard(page: Page) {
	await page.goto('/board');
	await page.waitForLoadState('networkidle');
	await expect(page.getByRole('region', { name: 'Task board' })).toBeVisible();
	await expect(page.getByRole('region', { name: 'Queue column' })).toBeVisible();
	await expect(page.getByRole('region', { name: 'Running tasks column' })).toBeVisible();
	await page.waitForTimeout(150);
}

function getBoardCommandPanel(page: Page) {
	return page.locator('.board-view__panel.command-panel');
}

function getFirstQueueTaskCard(page: Page) {
	return page.getByRole('region', { name: 'Queue column' }).locator('.task-card').first();
}

test.describe('Board Kanban Validation', () => {
	test('should render two-column layout', async ({ page }) => {
		await openBoard(page);

		await expect(page.locator('.board-view')).toBeVisible();
		await expect(page.getByRole('region', { name: 'Queue column' })).toContainText('Queue');
		await expect(page.getByRole('region', { name: 'Running tasks column' })).toContainText('Running');
		await expect(page.locator('.queue-column-count')).toBeVisible();
	});

	test('should show swimlanes with progress', async ({ page }) => {
		await openBoard(page);

		const swimlanes = page.locator('.swimlane');
		const swimlaneCount = await swimlanes.count();
		expect(swimlaneCount).toBeGreaterThan(0);

		const initiativeSwimlane = page.locator('.swimlane:has(.swimlane-meta)').first();
		await expect(initiativeSwimlane.locator('.swimlane-header')).toBeVisible();
		await expect(initiativeSwimlane.locator('.swimlane-name')).toBeVisible();
		await expect(initiativeSwimlane.locator('.swimlane-count')).toBeVisible();
		await expect(initiativeSwimlane.locator('.swimlane-progress')).toBeVisible();
		await expect(initiativeSwimlane.locator('.swimlane-progress-fill')).toBeVisible();
	});

	test('should render running card pipeline or empty state', async ({ page }) => {
		await openBoard(page);

		const runningCards = page.locator('.running-card');
		const runningCardCount = await runningCards.count();

		if (runningCardCount > 0) {
			const pipeline = runningCards.first().locator('.pipeline');
			await expect(pipeline).toBeVisible();
			for (const label of ['Plan', 'Code', 'Test', 'Review', 'Done']) {
				await expect(pipeline).toContainText(label);
			}
			return;
		}

		const runningColumn = page.getByRole('region', { name: 'Running tasks column' });
		await expect(runningColumn).toContainText('No running tasks');
		await expect(runningColumn).toContainText('orc run');
	});

	test('should render right panel sections', async ({ page }) => {
		await openBoard(page);

		const panel = getBoardCommandPanel(page);
		await expect(panel).toBeVisible();
		await expect(panel.locator('.panel-section')).toHaveCount(5);

		for (const sectionTitle of ['Blocked', 'Decisions', 'Claude Config', 'Files Changed', 'Completed']) {
			await expect(panel.locator('.panel-section').filter({ hasText: sectionTitle }).first()).toBeVisible();
		}
	});

	test('should render queue task card structure', async ({ page }) => {
		await openBoard(page);

		const firstTaskCard = getFirstQueueTaskCard(page);
		await expect(firstTaskCard).toBeVisible();
		await expect(firstTaskCard.locator('.task-card-position')).toBeVisible();
		await expect(firstTaskCard.locator('.task-card-id')).toHaveText(/TASK-\d+/);
		await expect(firstTaskCard.locator('.task-card-title')).toBeVisible();
		await expect(firstTaskCard.locator('.task-card-category')).toBeVisible();
		await expect(firstTaskCard.locator('.task-card-priority')).toBeVisible();
	});

	test('should capture board kanban visual snapshot', async ({ page }) => {
		await openBoard(page);
		await disableAnimations(page);

		const panel = getBoardCommandPanel(page);
		await expect(panel).toBeVisible();

		await expect(page).toHaveScreenshot('board-kanban-full.png', {
			fullPage: true,
			mask: [
				page.locator('.project-sidebar button').first(),
			],
		});
	});
});
