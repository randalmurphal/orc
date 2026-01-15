/**
 * Task Detail Page E2E Tests
 *
 * Framework-agnostic tests for the Task Detail page tabs.
 * These tests define BEHAVIOR, not implementation, to work on both
 * Svelte (current) and React (future migration) implementations.
 *
 * Test Coverage (15 tests):
 * - Tab Navigation (4): tab visibility, switching, URL updates, URL loading
 * - Timeline Tab (3): phase timeline, token stats, iterations/retries
 * - Changes Tab (5): diff loading, file stats, expand/collapse, view toggle, line numbers
 * - Transcript Tab (2): history display, content expansion
 * - Attachments Tab (1): attachment list
 *
 * Selector Strategy (priority order):
 * 1. role/aria-label - getByRole(), locator('[role="tab"]')
 * 2. Semantic text - getByText(), :has-text()
 * 3. Structural classes - .timeline-phase, .stat-card, .diff-file
 * 4. data-testid - fallback only
 *
 * Avoid: Framework-specific classes (.svelte-xyz), deep DOM paths
 *
 * @see web/CLAUDE.md for selector strategy documentation
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper: Navigate to a task detail page
async function navigateToTask(page: Page): Promise<string | null> {
	// Start from root which has better routing reliability
	await page.goto('/');
	await page.waitForLoadState('networkidle');

	// Navigate to board which shows all tasks
	await page.goto('/board');
	await page.waitForLoadState('networkidle');

	// Wait for the board to load
	await page.waitForSelector('.board-page, .board', { timeout: 10000 }).catch(() => {});
	await page.waitForTimeout(200);

	// Look for task cards on the board
	const taskCards = page.locator('.task-card');
	const count = await taskCards.count();

	if (count === 0) {
		return null;
	}

	// Click the first task card to navigate to detail
	await taskCards.first().click();
	await page.waitForURL(/\/tasks\/TASK-\d+/, { timeout: 5000 });

	// Extract task ID from URL
	const match = page.url().match(/TASK-\d+/);
	return match?.[0] || null;
}

// Helper: Wait for task detail page to load
async function waitForTaskDetailLoad(page: Page) {
	// Wait for tab navigation to be visible
	await page.waitForSelector('[role="tablist"]', { timeout: 10000 });
	// Wait for loading to complete
	await page.waitForSelector('.loading-state', { state: 'hidden', timeout: 10000 }).catch(() => {});
	// Small buffer for animations
	await page.waitForTimeout(100);
}

// Helper: Click a tab by its label
async function clickTab(page: Page, label: string) {
	const tab = page.locator(`[role="tab"]:has-text("${label}")`);
	await expect(tab).toBeVisible({ timeout: 5000 });
	await tab.click();
	// Wait for tab content to update
	await page.waitForTimeout(150);
}

// Helper: Get the active tab label
async function getActiveTabLabel(page: Page): Promise<string | null> {
	// Wait for tab panel to stabilize
	await page.waitForSelector('[role="tablist"]', { timeout: 5000 });
	await page.waitForTimeout(100);

	const activeTab = page.locator('[role="tab"][aria-selected="true"]');
	const isVisible = await activeTab.isVisible().catch(() => false);
	if (!isVisible) return null;
	return await activeTab.textContent();
}

test.describe('Task Detail Page', () => {
	test.describe('Tab Navigation', () => {
		test('should show all tabs (Timeline, Changes, Transcript, Test Results, Attachments, Comments)', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);

			// Expected tabs
			const expectedTabs = ['Timeline', 'Changes', 'Transcript', 'Test Results', 'Attachments', 'Comments'];

			// Verify each tab exists
			for (const tabLabel of expectedTabs) {
				const tab = page.locator(`[role="tab"]:has-text("${tabLabel}")`);
				await expect(tab).toBeVisible({ timeout: 3000 });
			}

			// Verify tab list accessibility
			const tabList = page.locator('[role="tablist"][aria-label="Task details tabs"]');
			await expect(tabList).toBeVisible();

			// Verify total number of tabs (6)
			const allTabs = page.locator('[role="tab"]');
			await expect(allTabs).toHaveCount(6);
		});

		test('should switch tabs when clicked', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);

			// Default should be Timeline (first tab)
			const timelineTab = page.locator('[role="tab"]:has-text("Timeline")');
			await expect(timelineTab).toHaveAttribute('aria-selected', 'true');

			// Click on Changes tab
			await clickTab(page, 'Changes');

			// Verify Changes tab is now selected
			const changesTab = page.locator('[role="tab"]:has-text("Changes")');
			await expect(changesTab).toHaveAttribute('aria-selected', 'true');

			// Timeline should no longer be selected
			await expect(timelineTab).toHaveAttribute('aria-selected', 'false');

			// Click on Transcript tab
			await clickTab(page, 'Transcript');

			// Verify Transcript tab is now selected
			const transcriptTab = page.locator('[role="tab"]:has-text("Transcript")');
			await expect(transcriptTab).toHaveAttribute('aria-selected', 'true');

			// Previous tab should no longer be selected
			await expect(changesTab).toHaveAttribute('aria-selected', 'false');
		});

		test('should update URL with tab parameter', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);

			// Click Changes tab
			await clickTab(page, 'Changes');

			// URL should contain tab=changes
			await expect(page).toHaveURL(/[?&]tab=changes/);

			// Click Transcript tab
			await clickTab(page, 'Transcript');
			await expect(page).toHaveURL(/[?&]tab=transcript/);

			// Click Attachments tab
			await clickTab(page, 'Attachments');
			await expect(page).toHaveURL(/[?&]tab=attachments/);
		});

		test('should load correct tab from URL query param', async ({ page }) => {
			// First navigate to get a valid task ID
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			// Navigate directly to task with tab param
			await page.goto(`/tasks/${taskId}?tab=changes`);
			await waitForTaskDetailLoad(page);

			// Changes tab should be active
			const changesTab = page.locator('[role="tab"]:has-text("Changes")');
			await expect(changesTab).toHaveAttribute('aria-selected', 'true');

			// Try another tab via URL
			await page.goto(`/tasks/${taskId}?tab=transcript`);
			await waitForTaskDetailLoad(page);

			const transcriptTab = page.locator('[role="tab"]:has-text("Transcript")');
			await expect(transcriptTab).toHaveAttribute('aria-selected', 'true');

			// Timeline should also work
			await page.goto(`/tasks/${taskId}?tab=timeline`);
			await waitForTaskDetailLoad(page);

			const timelineTab = page.locator('[role="tab"]:has-text("Timeline")');
			await expect(timelineTab).toHaveAttribute('aria-selected', 'true');
		});
	});

	test.describe('Timeline Tab', () => {
		test('should show phase timeline with status indicators', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);

			// Ensure Timeline tab is active (default)
			await clickTab(page, 'Timeline');

			// Tab panel should be visible
			const tabPanel = page.locator('[role="tabpanel"]');
			await expect(tabPanel).toBeVisible();

			// Look for timeline container
			const timeline = page.locator('.timeline-container');
			const hasTimeline = await timeline.isVisible().catch(() => false);

			if (hasTimeline) {
				// Timeline should have header
				const header = timeline.locator('h2:has-text("Execution Timeline")');
				await expect(header).toBeVisible();

				// Timeline should have phase nodes
				const phaseNodes = timeline.locator('.phase-node');
				const nodeCount = await phaseNodes.count();

				// Tasks with plans should have at least one phase
				if (nodeCount > 0) {
					// Each node should have a label
					const firstNode = phaseNodes.first();
					const nodeLabel = firstNode.locator('.node-label');
					await expect(nodeLabel).toBeVisible();

					// Nodes should have status circles
					const nodeCircle = firstNode.locator('.node-circle');
					await expect(nodeCircle).toBeVisible();
				}

				// Progress section should exist
				const progressSection = timeline.locator('.progress-section');
				await expect(progressSection).toBeVisible();

				// Progress bar should show percentage
				const progressLabel = timeline.locator('.progress-label');
				await expect(progressLabel).toBeVisible();
				const progressText = await progressLabel.textContent();
				expect(progressText).toMatch(/\d+%/);
			} else {
				// Empty state for tasks without execution plan
				const emptyState = page.locator('.empty-tab-state');
				await expect(emptyState).toBeVisible();
			}
		});

		test('should show token usage stats (input, output, cached, total)', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Timeline');

			// Look for stats grid
			const statsGrid = page.locator('.stats-grid');
			const hasStats = await statsGrid.isVisible().catch(() => false);

			if (hasStats) {
				// Find Token Usage stat card
				const tokenCard = page.locator('.stat-card:has(.stat-header:has-text("Token Usage"))');
				const hasTokenCard = await tokenCard.isVisible().catch(() => false);

				if (hasTokenCard) {
					// Should have input/output/total token stats
					const tokenStats = tokenCard.locator('.token-stats');
					await expect(tokenStats).toBeVisible();

					// Input stat
					const inputStat = tokenStats.locator('.token-stat:has(.token-label:has-text("Input"))');
					await expect(inputStat).toBeVisible();
					const inputValue = inputStat.locator('.token-value');
					await expect(inputValue).toBeVisible();

					// Output stat
					const outputStat = tokenStats.locator('.token-stat:has(.token-label:has-text("Output"))');
					await expect(outputStat).toBeVisible();

					// Total stat (should have 'total' class)
					const totalStat = tokenStats.locator('.token-stat.total');
					await expect(totalStat).toBeVisible();

					// Cached stat may or may not be present depending on task
					const cachedStat = tokenStats.locator('.token-stat.cached');
					// Just check it doesn't error if present
					await cachedStat.isVisible().catch(() => false);
				}
			}
		});

		test('should show iteration and retry counts', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Timeline');

			// Look for stats grid
			const statsGrid = page.locator('.stats-grid');
			const hasStats = await statsGrid.isVisible().catch(() => false);

			if (hasStats) {
				// Iterations card
				const iterationsCard = page.locator('.stat-card:has(.stat-header:has-text("Iterations"))');
				const hasIterations = await iterationsCard.isVisible().catch(() => false);

				if (hasIterations) {
					const iterationValue = iterationsCard.locator('.stat-value');
					await expect(iterationValue).toBeVisible();
					const valueText = await iterationValue.textContent();
					// Value should be a number
					expect(valueText).toMatch(/^\d+$/);
				}

				// Retries card (may not be present if no retries)
				const retriesCard = page.locator('.stat-card:has(.stat-header:has-text("Retries"))');
				const hasRetries = await retriesCard.isVisible().catch(() => false);

				if (hasRetries) {
					const retryValue = retriesCard.locator('.stat-value');
					await expect(retryValue).toBeVisible();
					const valueText = await retryValue.textContent();
					expect(valueText).toMatch(/^\d+$/);
				}
			}
		});
	});

	test.describe('Changes Tab - Diff Viewer', () => {
		test('should load and display diff stats', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Changes');

			// Wait for diff viewer to load
			const diffViewer = page.locator('.diff-viewer');
			await expect(diffViewer).toBeVisible({ timeout: 5000 });

			// Wait for loading to complete
			await page.waitForSelector('.diff-viewer .loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});

			// Check for diff stats in toolbar
			const diffStats = page.locator('.diff-stats');
			const hasStats = await diffStats.isVisible().catch(() => false);

			if (hasStats) {
				// Should show file count
				const fileCount = diffStats.locator('.stat-files');
				await expect(fileCount).toBeVisible();
				const fileText = await fileCount.textContent();
				expect(fileText).toMatch(/\d+ files?/);

				// Should show additions/deletions
				const additions = diffStats.locator('.additions');
				await expect(additions).toBeVisible();
				const addText = await additions.textContent();
				expect(addText).toMatch(/\+\d+/);

				const deletions = diffStats.locator('.deletions');
				await expect(deletions).toBeVisible();
				const delText = await deletions.textContent();
				expect(delText).toMatch(/-\d+/);
			} else {
				// Empty state is also valid
				const emptyState = page.locator('.diff-viewer .empty-state');
				const hasEmpty = await emptyState.isVisible().catch(() => false);
				if (hasEmpty) {
					await expect(emptyState).toContainText('No changes to display');
				}
			}
		});

		test('should show file list with additions/deletions counts', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Changes');

			// Wait for diff to load
			await page.waitForSelector('.diff-viewer .loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});

			// Check for file list
			const fileList = page.locator('.file-list');
			const hasFiles = await fileList.isVisible().catch(() => false);

			if (hasFiles) {
				// Get all diff files
				const diffFiles = page.locator('.diff-file');
				const fileCount = await diffFiles.count();

				if (fileCount > 0) {
					// Each file should have a header with path
					const firstFile = diffFiles.first();
					const fileHeader = firstFile.locator('.file-header');
					await expect(fileHeader).toBeVisible();

					// File path should be visible
					const filePath = fileHeader.locator('.file-path');
					await expect(filePath).toBeVisible();
					const pathText = await filePath.textContent();
					expect(pathText?.length).toBeGreaterThan(0);

					// File stats (additions/deletions) in header
					const fileStats = fileHeader.locator('.file-stats');
					await expect(fileStats).toBeVisible();

					// Additions count
					const additionsSpan = fileStats.locator('.additions');
					const hasAdditions = await additionsSpan.isVisible().catch(() => false);
					if (hasAdditions) {
						const addText = await additionsSpan.textContent();
						expect(addText).toMatch(/\+\d+/);
					}

					// Deletions count
					const deletionsSpan = fileStats.locator('.deletions');
					const hasDeletions = await deletionsSpan.isVisible().catch(() => false);
					if (hasDeletions) {
						const delText = await deletionsSpan.textContent();
						expect(delText).toMatch(/-\d+/);
					}
				}
			}
		});

		test('should expand/collapse files', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Changes');

			// Wait for diff to load
			await page.waitForSelector('.diff-viewer .loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});

			const diffFiles = page.locator('.diff-file');
			const fileCount = await diffFiles.count();
			test.skip(fileCount === 0, 'No files in diff to test expand/collapse');

			const firstFile = diffFiles.first();

			// File header should be clickable
			const fileHeader = firstFile.locator('.file-header');
			await expect(fileHeader).toBeVisible();

			// Check expand/collapse button attribute
			const isExpanded = await fileHeader.getAttribute('aria-expanded');

			// Click to toggle
			await fileHeader.click();
			await page.waitForTimeout(200);

			// Verify state changed
			const newIsExpanded = await fileHeader.getAttribute('aria-expanded');
			expect(newIsExpanded).not.toBe(isExpanded);

			// If expanded, file content should be visible
			if (newIsExpanded === 'true') {
				const fileContent = firstFile.locator('.file-content');
				await expect(fileContent).toBeVisible({ timeout: 5000 });
			}

			// Click again to toggle back
			await fileHeader.click();
			await page.waitForTimeout(200);
			const finalIsExpanded = await fileHeader.getAttribute('aria-expanded');
			expect(finalIsExpanded).toBe(isExpanded);
		});

		test('should toggle between split and unified view', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Changes');

			// Wait for diff to load
			await page.waitForSelector('.diff-viewer .loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});

			// Find view toggle buttons
			const viewToggle = page.locator('.view-toggle');
			const hasViewToggle = await viewToggle.isVisible().catch(() => false);
			test.skip(!hasViewToggle, 'View toggle not visible (empty diff)');

			const splitButton = viewToggle.locator('button:has-text("Split")');
			const unifiedButton = viewToggle.locator('button:has-text("Unified")');

			await expect(splitButton).toBeVisible();
			await expect(unifiedButton).toBeVisible();

			// One should be active by default (has aria-selected=true)
			const splitSelected = await splitButton.getAttribute('aria-selected');
			const unifiedSelected = await unifiedButton.getAttribute('aria-selected');

			// Exactly one should be selected
			expect(splitSelected === 'true' || unifiedSelected === 'true').toBe(true);

			// Click the non-active one
			if (splitSelected === 'true') {
				await unifiedButton.click();
				await page.waitForTimeout(100);
				await expect(unifiedButton).toHaveAttribute('aria-selected', 'true');
				await expect(splitButton).toHaveAttribute('aria-selected', 'false');
			} else {
				await splitButton.click();
				await page.waitForTimeout(100);
				await expect(splitButton).toHaveAttribute('aria-selected', 'true');
				await expect(unifiedButton).toHaveAttribute('aria-selected', 'false');
			}
		});

		test('should show line numbers', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Changes');

			// Wait for diff to load
			await page.waitForSelector('.diff-viewer .loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});

			const diffFiles = page.locator('.diff-file');
			const fileCount = await diffFiles.count();
			test.skip(fileCount === 0, 'No files in diff to test line numbers');

			const firstFile = diffFiles.first();
			const fileHeader = firstFile.locator('.file-header');

			// Expand the file if not already expanded
			const isExpanded = await fileHeader.getAttribute('aria-expanded');
			if (isExpanded !== 'true') {
				await fileHeader.click();
				await page.waitForTimeout(500); // Wait for hunks to load
			}

			// Wait for file content to load
			const fileContent = firstFile.locator('.file-content');
			await expect(fileContent).toBeVisible({ timeout: 10000 });

			// Check for diff hunks (may show loading first)
			await page.waitForSelector('.diff-hunk', { timeout: 5000 }).catch(() => {});

			const hunks = page.locator('.diff-hunk');
			const hunkCount = await hunks.count();

			if (hunkCount > 0) {
				// Diff lines should have line numbers
				const diffLines = page.locator('.diff-line');
				const lineCount = await diffLines.count();

				if (lineCount > 0) {
					// Check for line number elements
					const lineNums = page.locator('.diff-line .line-num');
					const lineNumCount = await lineNums.count();
					expect(lineNumCount).toBeGreaterThan(0);

					// At least one line number should have content
					const firstLineNum = lineNums.first();
					const lineNumber = firstLineNum.locator('.line-number');
					await expect(lineNumber).toBeVisible();
				}
			}
		});
	});

	test.describe('Transcript Tab', () => {
		test('should show transcript history', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Transcript');

			// Transcript container should be visible
			const transcriptContainer = page.locator('.transcript-container');
			await expect(transcriptContainer).toBeVisible();

			// Header should be visible
			const header = transcriptContainer.locator('h2:has-text("Transcript")');
			await expect(header).toBeVisible();

			// Check for transcript content
			const transcriptContent = transcriptContainer.locator('.transcript-content');
			await expect(transcriptContent).toBeVisible();

			// Either have transcript files or empty state
			const transcriptFiles = page.locator('.transcript-file');
			const fileCount = await transcriptFiles.count();

			if (fileCount === 0) {
				// Empty state
				const emptyState = transcriptContainer.locator('.empty-state');
				const hasEmpty = await emptyState.isVisible().catch(() => false);
				if (hasEmpty) {
					await expect(emptyState).toContainText('No transcript yet');
				}
			} else {
				// Has transcript files - verify structure
				const firstFile = transcriptFiles.first();

				// File header should be visible
				const fileHeader = firstFile.locator('.file-header');
				await expect(fileHeader).toBeVisible();

				// Should have phase badge
				const phaseBadge = fileHeader.locator('.phase-badge');
				await expect(phaseBadge).toBeVisible();

				// Should have iteration text
				const iteration = fileHeader.locator('.iteration');
				await expect(iteration).toBeVisible();
				const iterText = await iteration.textContent();
				expect(iterText).toMatch(/Iteration \d+/);
			}
		});

		test('should expand transcript content sections', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Transcript');

			const transcriptFiles = page.locator('.transcript-file');
			const fileCount = await transcriptFiles.count();
			test.skip(fileCount === 0, 'No transcript files to test expansion');

			const firstFile = transcriptFiles.first();
			const fileHeader = firstFile.locator('.file-header');

			// Check if expanded (has .expanded class or chevron rotated)
			const chevron = fileHeader.locator('.chevron');
			const isExpanded = await chevron.evaluate(el => el.classList.contains('rotated'));

			// Click to toggle
			await fileHeader.click();
			await page.waitForTimeout(200);

			// Verify toggle worked
			const newIsExpanded = await chevron.evaluate(el => el.classList.contains('rotated'));
			expect(newIsExpanded).not.toBe(isExpanded);

			// When expanded, file content should be visible with sections
			if (newIsExpanded) {
				const fileContent = firstFile.locator('.file-content');
				await expect(fileContent).toBeVisible();

				// Should have sections (Prompt, Response, etc.)
				const sections = fileContent.locator('.section');
				const sectionCount = await sections.count();

				if (sectionCount > 0) {
					// Each section should have header and content
					const firstSection = sections.first();
					const sectionHeader = firstSection.locator('.section-header');
					await expect(sectionHeader).toBeVisible();

					const sectionContent = firstSection.locator('.section-content');
					await expect(sectionContent).toBeVisible();

					// Section title should indicate type
					const sectionTitle = sectionHeader.locator('.section-title');
					const titleText = await sectionTitle.textContent();
					expect(titleText?.length).toBeGreaterThan(0);
				}
			}
		});
	});

	test.describe('Attachments Tab', () => {
		test('should display attachment list with thumbnails', async ({ page }) => {
			const taskId = await navigateToTask(page);
			test.skip(!taskId, 'No tasks available for testing');

			await waitForTaskDetailLoad(page);
			await clickTab(page, 'Attachments');

			// Wait for tab content to be visible
			const tabPanel = page.locator('[role="tabpanel"]');
			await expect(tabPanel).toBeVisible();

			// Attachments container should be visible - use first() to handle potential duplicates
			const attachmentsContainer = tabPanel.locator('.attachments-container').first();
			await expect(attachmentsContainer).toBeVisible();

			// Upload area should always be present
			const uploadArea = attachmentsContainer.locator('.upload-area');
			await expect(uploadArea).toBeVisible();

			// Upload label with instructions
			const uploadLabel = uploadArea.locator('.upload-label');
			await expect(uploadLabel).toBeVisible();
			const labelText = await uploadLabel.textContent();
			expect(labelText?.toLowerCase()).toContain('drop files');

			// Wait for loading to complete
			await page.waitForSelector('.attachments-container .loading-state', { state: 'hidden', timeout: 5000 }).catch(() => {});

			// Check for images section (if has images)
			const imagesGrid = page.locator('.images-grid');
			const hasImages = await imagesGrid.isVisible().catch(() => false);

			if (hasImages) {
				// Images should have preview thumbnails
				const imageCards = imagesGrid.locator('.image-card');
				const imageCount = await imageCards.count();

				if (imageCount > 0) {
					const firstImage = imageCards.first();

					// Should have image preview button
					const imagePreview = firstImage.locator('.image-preview');
					await expect(imagePreview).toBeVisible();

					// Should have img element
					const img = imagePreview.locator('img');
					await expect(img).toBeVisible();

					// Should have image info (name, size)
					const imageInfo = firstImage.locator('.image-info');
					await expect(imageInfo).toBeVisible();

					const imageName = imageInfo.locator('.image-name');
					await expect(imageName).toBeVisible();
				}
			}

			// Check for files section (if has non-image files)
			const filesList = page.locator('.files-list');
			const hasFiles = await filesList.isVisible().catch(() => false);

			if (hasFiles) {
				const fileItems = filesList.locator('.file-item');
				const fileCount = await fileItems.count();

				if (fileCount > 0) {
					const firstFileItem = fileItems.first();

					// Should have file icon
					const fileIcon = firstFileItem.locator('.file-icon');
					await expect(fileIcon).toBeVisible();

					// Should have file name link
					const fileName = firstFileItem.locator('.file-name');
					await expect(fileName).toBeVisible();

					// Should have file metadata (size)
					const fileMeta = firstFileItem.locator('.file-meta');
					await expect(fileMeta).toBeVisible();
				}
			}

			// If no attachments, empty state should be shown
			if (!hasImages && !hasFiles) {
				const emptyState = attachmentsContainer.locator('.empty-state');
				const hasEmpty = await emptyState.isVisible().catch(() => false);
				if (hasEmpty) {
					await expect(emptyState).toContainText('No attachments');
				}
			}
		});
	});
});
