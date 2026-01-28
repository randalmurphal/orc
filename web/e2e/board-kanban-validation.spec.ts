/**
 * Board Kanban Validation E2E Tests
 *
 * Validates the Board page matches the reference design in
 * example_ui/board-kanban-view.png. Tests cover swimlane progress bars,
 * task position numbers, right panel sections, and overall layout structure.
 *
 * CRITICAL: These tests run against an ISOLATED SANDBOX project created by
 * global-setup.ts. Tests perform real actions (clicks) that may modify
 * task states. The sandbox ensures real production tasks are NEVER affected.
 *
 * Test Coverage (4 groups):
 * - Swimlane Progress Validation: progress bars, fill elements, meta info
 * - Task Position Numbers: sequential numbering in queue cards
 * - Right Panel Sections: 5 sections with correct titles and structure
 * - Board Layout Structure: 2-column + panel layout
 *
 * Selector Strategy (priority order):
 * 1. role/aria-label - getByRole(), locator('[aria-label="..."]')
 * 2. Semantic text - getByText(), :has-text()
 * 3. Structural classes - .swimlane, .panel-section, .board-view
 * 4. data-testid - for elements without semantic meaning
 *
 * Avoid: Framework-specific classes (.svelte-xyz), deep DOM paths
 *
 * @see web/CLAUDE.md for selector strategy documentation
 * @see example_ui/board-kanban-view.png for reference design
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper function to wait for board to load
async function waitForBoardLoad(page: Page) {
	// Wait for either the board-view or board-page class to appear
	await Promise.race([
		page.waitForSelector('.board-view', { timeout: 10000 }),
		page.waitForSelector('.board-page', { timeout: 10000 }),
	]);
	// Wait for loading skeleton to disappear
	await page.waitForSelector('.board-view--loading', { state: 'detached', timeout: 10000 }).catch(() => {});
	await page.waitForTimeout(200);
}

test.describe('Board Kanban Validation', () => {
	test.describe('Swimlane Progress Validation', () => {
		test('should display progress bars in at least one swimlane header', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			if (swimlaneCount > 0) {
				// At least one swimlane should have a visible progress bar.
				// The progressbar role is on the .swimlane-progress element itself.
				const progressBars = page.locator('.swimlane [role="progressbar"]');
				const progressCount = await progressBars.count();
				expect(progressCount).toBeGreaterThan(0);
			}
		});

		test('should have colored fill elements in progress bars', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			if (swimlaneCount > 0) {
				// At least one swimlane should have a progress fill element
				const fills = page.locator('.swimlane .swimlane-progress-fill');
				const fillCount = await fills.count();
				expect(fillCount).toBeGreaterThan(0);
			}
		});

		test('should show name and meta info in swimlane headers', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			if (swimlaneCount > 0) {
				for (let i = 0; i < swimlaneCount; i++) {
					const swimlane = swimlanes.nth(i);

					// Swimlane header should have a name
					const name = swimlane.locator('.swimlane-name');
					await expect(name).toBeVisible({ timeout: 3000 });
					const nameText = await name.textContent();
					expect(nameText).toBeTruthy();
					expect(nameText!.trim().length).toBeGreaterThan(0);
				}

				// At least one swimlane (initiative-linked) should have meta info
				const metaElements = page.locator('.swimlane .swimlane-meta');
				const metaCount = await metaElements.count();
				expect(metaCount).toBeGreaterThan(0);
			}
		});
	});

	test.describe('Task Position Numbers', () => {
		test('should display position numbers on queue task cards', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Look for task position elements in the board
			const positions = page.locator('.task-position, .task-card-position');
			const positionCount = await positions.count();

			if (positionCount > 0) {
				// Each position should contain a number
				for (let i = 0; i < positionCount; i++) {
					const position = positions.nth(i);
					const text = await position.textContent();
					expect(text).toBeTruthy();
					expect(text!.trim()).toMatch(/^\d+$/);
				}
			}
		});

		test('should have sequential position numbers starting from 1', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Check positions within each swimlane for sequential ordering
			const swimlanes = page.locator('.swimlane');
			const swimlaneCount = await swimlanes.count();

			for (let i = 0; i < swimlaneCount; i++) {
				const swimlane = swimlanes.nth(i);
				const positions = swimlane.locator('.task-position, .task-card-position');
				const positionCount = await positions.count();

				if (positionCount > 0) {
					const numbers: number[] = [];
					for (let j = 0; j < positionCount; j++) {
						const text = await positions.nth(j).textContent();
						numbers.push(parseInt(text!.trim(), 10));
					}

					// Positions should start at 1 and be sequential
					expect(numbers[0]).toBe(1);
					for (let j = 1; j < numbers.length; j++) {
						expect(numbers[j]).toBe(numbers[j - 1] + 1);
					}
				}
			}
		});
	});

	test.describe('Right Panel Sections', () => {
		test('should display all 5 panel sections with correct titles', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const commandPanel = page.locator('.board-view__panel.command-panel');
			await expect(commandPanel).toBeVisible({ timeout: 5000 });

			// All 5 expected panel section titles (capitalized in HTML source)
			const expectedTitles = ['Blocked', 'Decisions', 'Claude Config', 'Files Changed', 'Completed'];

			for (const title of expectedTitles) {
				const section = commandPanel.locator(`.panel-section:has-text("${title}")`);
				await expect(section).toBeVisible({ timeout: 3000 });
			}

			// Verify exactly 5 panel sections
			const sections = commandPanel.locator('.panel-section');
			await expect(sections).toHaveCount(5);
		});

		test('should have collapsible panel sections with header and body', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const commandPanel = page.locator('.board-view__panel.command-panel');
			await expect(commandPanel).toBeVisible({ timeout: 5000 });

			const sections = commandPanel.locator('.panel-section');
			const sectionCount = await sections.count();

			for (let i = 0; i < sectionCount; i++) {
				const section = sections.nth(i);

				// Each section should have a clickable header
				const header = section.locator('.panel-header').first();
				await expect(header).toBeVisible();
			}
		});

		test('should show Claude Config section with expected config items', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const commandPanel = page.locator('.board-view__panel.command-panel');
			await expect(commandPanel).toBeVisible({ timeout: 5000 });

			// Find the Claude Config section
			const claudeConfigSection = commandPanel.locator('.panel-section:has-text("Claude Config")');
			await expect(claudeConfigSection).toBeVisible({ timeout: 3000 });

			// Expected config items in the Claude Config section
			const expectedItems = ['Slash Commands', 'CLAUDE.md', 'MCP Servers', 'Permissions'];

			for (const item of expectedItems) {
				const configItem = claudeConfigSection.locator(`:has-text("${item}")`).first();
				await expect(configItem).toBeVisible({ timeout: 3000 });
			}
		});
	});

	test.describe('Board Layout Structure', () => {
		test('should display layout with queue and running columns', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Queue column should be visible
			const queueColumn = page.locator('.board-view__queue');
			await expect(queueColumn).toBeVisible({ timeout: 5000 });

			// Running column should be visible
			const runningColumn = page.locator('.board-view__running');
			await expect(runningColumn).toBeVisible({ timeout: 5000 });
		});

		test('should display right command panel alongside board columns', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			// Command panel should be visible (using full selector)
			const commandPanel = page.locator('.board-view__panel.command-panel');
			await expect(commandPanel).toBeVisible({ timeout: 5000 });

			// Board view container should also be present
			const boardView = page.locator('.board-view');
			await expect(boardView).toBeVisible({ timeout: 5000 });
		});

		test('should show running cards with pipeline and metadata', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const runningCards = page.locator('.running-card');
			const cardCount = await runningCards.count();

			if (cardCount > 0) {
				const firstCard = runningCards.first();

				// Running card should have a pipeline visualization
				const pipeline = firstCard.locator('.running-pipeline, .pipeline');
				await expect(pipeline).toBeVisible({ timeout: 3000 });

				// Pipeline should have step elements
				const steps = pipeline.locator('.pipeline-step');
				const stepCount = await steps.count();
				expect(stepCount).toBe(5); // Plan, Code, Test, Review, Done

				// Running card should show initiative info
				const initiative = firstCard.locator('.running-initiative');
				const hasInitiative = await initiative.isVisible().catch(() => false);

				// Running card should show current phase
				const phase = firstCard.locator('.running-phase');
				const hasPhase = await phase.isVisible().catch(() => false);

				// Running card should show elapsed time
				const time = firstCard.locator('.running-time');
				const hasTime = await time.isVisible().catch(() => false);

				// At least phase or time should be visible for running cards
				expect(hasPhase || hasTime || hasInitiative).toBeTruthy();
			}
		});

		test('should show pipeline bars with labels for each phase', async ({ page }) => {
			await page.goto('/board');
			await waitForBoardLoad(page);

			const pipelines = page.locator('.pipeline');
			const pipelineCount = await pipelines.count();

			if (pipelineCount > 0) {
				const firstPipeline = pipelines.first();

				// Pipeline should have bar elements
				const bars = firstPipeline.locator('.pipeline-bar');
				await expect(bars.first()).toBeVisible({ timeout: 3000 });

				// Pipeline should have labels
				const labels = firstPipeline.locator('.pipeline-label');
				const labelCount = await labels.count();

				if (labelCount > 0) {
					const expectedPhases = ['Plan', 'Code', 'Test', 'Review', 'Done'];
					const labelTexts: string[] = [];

					for (let i = 0; i < labelCount; i++) {
						const text = await labels.nth(i).textContent();
						if (text) labelTexts.push(text.trim());
					}

					// Verify all expected phase labels are present
					for (const phase of expectedPhases) {
						expect(labelTexts).toContain(phase);
					}
				}
			}
		});
	});
});
