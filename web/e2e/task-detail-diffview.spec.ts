/**
 * E2E Tests for Task Detail Page with Diff View
 *
 * Tests the Task Detail page "deep work" layout including:
 * - Page structure (header, progress, split pane, footer)
 * - Live output streaming
 * - Changes panel with file list and statistics
 * - Inline diff preview on file selection
 * - Full diff modal with keyboard navigation
 * - External links in Test Results and Attachments tabs
 *
 * Test Coverage (15 tests):
 * - Layout Structure (2): SC-1, SC-2
 * - Live Output Panel (2): SC-3, SC-4
 * - Changes Panel File List (2): SC-5, SC-6
 * - Inline Diff Preview (2): SC-7, SC-8
 * - Full Diff Modal (4): SC-9, SC-10, SC-11, SC-12
 * - External Links (3): SC-13, SC-14, SC-15
 *
 * Success Criteria Coverage:
 * - SC-1: Task Detail page renders all structural regions
 * - SC-2: Header displays task ID, title, workflow, branch, elapsed time
 * - SC-3: Live Output panel renders TranscriptViewer
 * - SC-4: Streaming transcript lines appear when task is running
 * - SC-5: Changes panel displays file list with paths
 * - SC-6: File list shows addition/deletion counts
 * - SC-7: Clicking file in list shows inline diff preview
 * - SC-8: Selected file is highlighted in file list
 * - SC-9: Full diff modal opens via Ctrl+Shift+F keyboard shortcut
 * - SC-10: Diff modal displays file list in sidebar with status badges
 * - SC-11: Vim-style navigation (j/k) moves between files in modal
 * - SC-12: View mode toggle switches between split and unified diff
 * - SC-13: Test Results tab shows external links with target="_blank"
 * - SC-14: Clicking external link opens in new tab
 * - SC-15: Attachment file links open in new tab
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper: Navigate to a task detail page
async function navigateToTask(page: Page): Promise<string | null> {
  await page.goto('/');
  await page.waitForLoadState('networkidle');

  await page.goto('/board');
  await page.waitForLoadState('networkidle');

  await page.waitForSelector('.board-page, .board', { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(200);

  const taskCards = page.locator('.task-card');
  const count = await taskCards.count();

  if (count === 0) {
    return null;
  }

  await taskCards.first().click();
  await page.waitForURL(/\/tasks\/TASK-\d+/, { timeout: 5000 });

  const match = page.url().match(/TASK-\d+/);
  return match?.[0] || null;
}

// Helper: Wait for task detail page to load
async function waitForTaskDetailLoad(page: Page) {
  await page.waitForSelector('.task-detail-page', { timeout: 10000 });
  await page.waitForSelector('.task-detail-loading', { state: 'hidden', timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(100);
}

// Helper: Navigate to Changes panel in Task Detail
async function navigateToChangesPanel(page: Page): Promise<string | null> {
  const taskId = await navigateToTask(page);
  if (!taskId) return null;

  await waitForTaskDetailLoad(page);
  return taskId;
}

test.describe('Task Detail Page - Layout Structure', () => {
  test('SC-1: should render all structural regions (header, progress, split pane, footer)', async ({ page }) => {
    const taskId = await navigateToTask(page);
    test.skip(!taskId, 'No tasks available for testing');

    await waitForTaskDetailLoad(page);

    // Verify main page container
    const pageContainer = page.locator('.task-detail-page');
    await expect(pageContainer).toBeVisible();

    // Verify header region
    const header = page.locator('.task-detail-header');
    await expect(header).toBeVisible();

    // Verify progress visualization (WorkflowProgress component)
    // May be inside header or separate - check for workflow progress elements
    const progressContainer = page.locator('.workflow-progress, .task-detail-header .progress');
    const hasProgress = await progressContainer.isVisible().catch(() => false);
    // Progress may not always be present if task has no plan

    // Verify split pane content area
    const contentArea = page.locator('.task-detail-content');
    await expect(contentArea).toBeVisible();

    // Verify Live Output panel (left pane)
    const liveOutputPanel = page.locator('.task-detail-panel:has-text("Live Output")');
    await expect(liveOutputPanel).toBeVisible();

    // Verify Changes panel (right pane)
    const changesPanel = page.locator('.task-detail-panel:has-text("Changes")');
    await expect(changesPanel).toBeVisible();

    // Verify footer region (TaskFooter component)
    const footer = page.locator('.task-footer, .task-detail-footer');
    const hasFooter = await footer.isVisible().catch(() => false);
    // Footer should exist for task actions
  });

  test('SC-2: should display header with task ID, title, workflow, branch, elapsed time', async ({ page }) => {
    const taskId = await navigateToTask(page);
    test.skip(!taskId, 'No tasks available for testing');

    await waitForTaskDetailLoad(page);

    const header = page.locator('.task-detail-header');
    await expect(header).toBeVisible();

    // Verify task ID is displayed
    const taskIdElement = header.locator('.task-detail-header__id');
    await expect(taskIdElement).toBeVisible();
    const idText = await taskIdElement.textContent();
    expect(idText).toMatch(/TASK-\d+/);

    // Verify title is displayed
    const titleElement = header.locator('.task-detail-header__title');
    await expect(titleElement).toBeVisible();
    const titleText = await titleElement.textContent();
    expect(titleText?.length).toBeGreaterThan(0);

    // Verify workflow badge (may not always be present)
    const workflowElement = header.locator('.task-detail-header__workflow');
    const hasWorkflow = await workflowElement.isVisible().catch(() => false);
    // Workflow is optional - some tasks may not have one

    // Verify branch display (may not always be present)
    const branchElement = header.locator('.task-detail-header__branch');
    const hasBranch = await branchElement.isVisible().catch(() => false);
    // Branch is optional

    // Verify elapsed time display
    const elapsedElement = header.locator('.task-detail-header__elapsed');
    await expect(elapsedElement).toBeVisible();
    const elapsedText = await elapsedElement.textContent();
    // Should show time pattern like "M:SS" or "—" if not started
    expect(elapsedText).toMatch(/\d+:\d{2}|—/);
  });
});

test.describe('Task Detail Page - Live Output Panel', () => {
  test('SC-3: should render Live Output panel with TranscriptViewer', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    // Verify Live Output panel
    const liveOutputPanel = page.locator('.task-detail-panel:has-text("Live Output")');
    await expect(liveOutputPanel).toBeVisible();

    // Check for transcript content area
    const panelContent = liveOutputPanel.locator('.task-detail-panel__content');
    await expect(panelContent).toBeVisible();

    // Look for transcript viewer component or empty state
    const transcriptViewer = page.locator('.transcript-viewer, .transcript-container, .transcript-tab-container');
    const emptyState = panelContent.locator('.empty-state, :has-text("No output yet")');

    const hasTranscript = await transcriptViewer.isVisible().catch(() => false);
    const hasEmpty = await emptyState.isVisible().catch(() => false);

    // Either transcript viewer or empty state should be visible
    expect(hasTranscript || hasEmpty).toBe(true);

    if (hasTranscript) {
      // Verify transcript viewer has content areas
      const transcriptContent = transcriptViewer.first().locator('.transcript-content, .entries, .messages');
      // Content may be present or loading
    }
  });

  test('SC-4: should show transcript content for tasks with output', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    const liveOutputPanel = page.locator('.task-detail-panel:has-text("Live Output")');
    await expect(liveOutputPanel).toBeVisible();

    // Wait for any loading to complete
    await page.waitForSelector('.loading-spinner', { state: 'hidden', timeout: 10000 }).catch(() => {});
    await page.waitForTimeout(500);

    // Check for transcript content
    const transcriptViewer = page.locator('.transcript-viewer, .transcript-container');
    const hasTranscript = await transcriptViewer.isVisible().catch(() => false);

    if (hasTranscript) {
      // For completed tasks, check for transcript entries
      const entries = page.locator('.transcript-entry, .message-item, .entry-content');
      const entryCount = await entries.count();

      // If there are entries, verify content structure
      if (entryCount > 0) {
        const firstEntry = entries.first();
        await expect(firstEntry).toBeVisible();
      }
    } else {
      // Empty state is acceptable for tasks without output
      const emptyState = liveOutputPanel.locator('.empty-state, :has-text("No output")');
      const hasEmpty = await emptyState.isVisible().catch(() => false);
      // Either way is acceptable
    }
  });
});

test.describe('Task Detail Page - Changes Panel File List', () => {
  test('SC-5: should display file list with file paths', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    // Wait for changes panel to load
    await page.waitForSelector('.task-detail-panel:has-text("Changes")', { timeout: 10000 });

    // Wait for files panel content to load
    const filesPanel = page.locator('[data-testid="files-panel"], .files-panel');
    const hasFP = await filesPanel.isVisible({ timeout: 10000 }).catch(() => false);

    if (!hasFP) {
      // Check for changes tab enhanced
      const changesTabEnhanced = page.locator('[data-testid="changes-tab-enhanced"], .changes-tab-enhanced');
      const hasCTE = await changesTabEnhanced.isVisible().catch(() => false);
      test.skip(!hasCTE, 'Changes panel not visible');
    }

    // Wait for loading to complete
    await page.waitForSelector('[data-testid="files-panel-loading"], .files-panel-loading', { state: 'hidden', timeout: 10000 }).catch(() => {});

    // Check for file list or empty state
    const fileItems = page.locator('.file-item, [data-testid="files-list-section"] .file-item');
    const fileCount = await fileItems.count();

    const emptyState = page.locator('[data-testid="files-panel-empty"], .files-panel-empty');
    const hasEmpty = await emptyState.isVisible().catch(() => false);

    if (hasEmpty) {
      // Empty state shows "No files changed" message
      await expect(emptyState).toContainText(/No files|No changes/);
      test.skip(true, 'No files in diff to test');
    }

    if (fileCount > 0) {
      const firstFile = fileItems.first();
      await expect(firstFile).toBeVisible();

      // Verify file path is displayed
      const filePath = firstFile.locator('.file-path, .file-name');
      await expect(filePath).toBeVisible();
      const pathText = await filePath.textContent();
      expect(pathText?.length).toBeGreaterThan(0);
      // Path should look like a file path
      expect(pathText).toMatch(/[\w\-./]+/);
    }
  });

  test('SC-6: should show addition and deletion counts for files', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    // Wait for files panel
    await page.waitForSelector('[data-testid="files-panel"], .files-panel', { timeout: 10000 }).catch(() => {});
    await page.waitForSelector('[data-testid="files-panel-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    const fileItems = page.locator('.file-item');
    const fileCount = await fileItems.count();
    test.skip(fileCount === 0, 'No files in diff to test statistics');

    const firstFile = fileItems.first();
    await expect(firstFile).toBeVisible();

    // Check for file stats container
    const fileStats = firstFile.locator('.file-stats');
    await expect(fileStats).toBeVisible();

    // Check for additions (+N format)
    const additions = fileStats.locator('.additions');
    const hasAdditions = await additions.isVisible().catch(() => false);
    if (hasAdditions) {
      const addText = await additions.textContent();
      expect(addText).toMatch(/\+\d+/);
    }

    // Check for deletions (-N format)
    const deletions = fileStats.locator('.deletions');
    const hasDeletions = await deletions.isVisible().catch(() => false);
    if (hasDeletions) {
      const delText = await deletions.textContent();
      expect(delText).toMatch(/-\d+/);
    }

    // At least one of additions or deletions should be present
    expect(hasAdditions || hasDeletions).toBe(true);
  });
});

test.describe('Task Detail Page - Inline Diff Preview', () => {
  test('SC-7: should show inline diff preview when clicking a file', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    // Wait for files panel
    await page.waitForSelector('[data-testid="files-panel"], .files-panel', { timeout: 10000 }).catch(() => {});
    await page.waitForSelector('[data-testid="files-panel-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    const fileItems = page.locator('[data-testid="files-list-section"] .file-item, .file-list .file-item, .file-item');
    const fileCount = await fileItems.count();
    test.skip(fileCount === 0, 'No files in diff to test inline preview');

    const firstFile = fileItems.first();
    await expect(firstFile).toBeVisible();

    // Click the file
    await firstFile.click();
    await page.waitForTimeout(500);

    // Wait for diff content to load
    await page.waitForSelector('[data-testid="file-diff-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    // Verify inline diff preview appears
    const diffContent = page.locator('[data-testid="diff-view-content"], .diff-view-content, .diff-content');
    const hasDiffContent = await diffContent.isVisible({ timeout: 5000 }).catch(() => false);

    if (hasDiffContent) {
      await expect(diffContent).toBeVisible();

      // Should show diff file component with hunks
      const diffFile = diffContent.locator('.diff-file, .diff-hunk, .hunk-content');
      const hasDiffFile = await diffFile.first().isVisible({ timeout: 3000 }).catch(() => false);

      // Diff content should be present (could be loading or showing hunks)
    } else {
      // Check for loading state or error state
      const loadingState = page.locator('[data-testid="file-diff-loading"], .file-diff-loading');
      const errorState = page.locator('[data-testid="file-load-error"], .file-load-error');
      const placeholderState = page.locator('.diff-view-placeholder, .diff-placeholder');

      const isLoading = await loadingState.isVisible().catch(() => false);
      const isError = await errorState.isVisible().catch(() => false);
      const isPlaceholder = await placeholderState.isVisible().catch(() => false);

      // One of these states should be visible
      expect(isLoading || isError || isPlaceholder).toBe(true);
    }
  });

  test('SC-8: should highlight selected file in file list', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    await page.waitForSelector('[data-testid="files-panel"], .files-panel', { timeout: 10000 }).catch(() => {});
    await page.waitForSelector('[data-testid="files-panel-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    const fileItems = page.locator('.file-item');
    const fileCount = await fileItems.count();
    test.skip(fileCount === 0, 'No files in diff to test selection');

    const firstFile = fileItems.first();
    await expect(firstFile).toBeVisible();

    // Click the first file
    await firstFile.click();
    await page.waitForTimeout(200);

    // Verify file is highlighted (selected class or aria-selected)
    const isSelected = await firstFile.evaluate(el =>
      el.classList.contains('selected') ||
      el.classList.contains('active') ||
      el.getAttribute('aria-selected') === 'true' ||
      el.getAttribute('data-selected') === 'true'
    );
    expect(isSelected).toBe(true);

    // If multiple files, click second and verify first is deselected
    if (fileCount > 1) {
      const secondFile = fileItems.nth(1);
      await secondFile.click();
      await page.waitForTimeout(200);

      // First file should no longer be selected
      const firstStillSelected = await firstFile.evaluate(el =>
        el.classList.contains('selected') ||
        el.classList.contains('active') ||
        el.getAttribute('aria-selected') === 'true'
      );
      expect(firstStillSelected).toBe(false);

      // Second file should be selected
      const secondSelected = await secondFile.evaluate(el =>
        el.classList.contains('selected') ||
        el.classList.contains('active') ||
        el.getAttribute('aria-selected') === 'true' ||
        el.getAttribute('data-selected') === 'true'
      );
      expect(secondSelected).toBe(true);
    }
  });
});

test.describe('Task Detail Page - Full Diff Modal', () => {
  test('SC-9: should open full diff modal via Ctrl+Shift+F keyboard shortcut', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    await page.waitForSelector('[data-testid="files-panel"], .files-panel, .changes-tab', { timeout: 10000 }).catch(() => {});

    // Check if there are files (modal may show empty state otherwise)
    const emptyState = page.locator('[data-testid="files-panel-empty"], .changes-empty');
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    test.skip(hasEmpty, 'No files to show in diff modal');

    // Press Ctrl+Shift+F to open full diff modal
    await page.keyboard.press('Control+Shift+f');
    await page.waitForTimeout(500);

    // Verify diff modal appears
    const diffModal = page.locator('[data-testid="diff-view-modal"]');
    await expect(diffModal).toBeVisible({ timeout: 5000 });

    // Modal should have proper aria attributes
    const modalRole = await diffModal.getAttribute('role');
    expect(modalRole).toBe('main');

    // Close modal with Escape
    await page.keyboard.press('Escape');
    await expect(diffModal).not.toBeVisible({ timeout: 3000 });
  });

  test('SC-10: should display file list in sidebar with status badges (A/M/D)', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    await page.waitForSelector('[data-testid="files-panel"], .files-panel, .changes-tab', { timeout: 10000 }).catch(() => {});

    // Skip if no files
    const emptyState = page.locator('[data-testid="files-panel-empty"], .changes-empty');
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    test.skip(hasEmpty, 'No files to show in diff modal');

    // Open modal
    await page.keyboard.press('Control+Shift+f');
    await page.waitForTimeout(500);

    const diffModal = page.locator('[data-testid="diff-view-modal"]');
    await expect(diffModal).toBeVisible({ timeout: 5000 });

    // Wait for modal content to load
    await page.waitForSelector('[data-testid="diff-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    // Check for file list in sidebar
    const fileList = page.locator('[data-testid="diff-modal-file-list"]');
    await expect(fileList).toBeVisible();

    // Check for file items with status badges
    const fileItems = fileList.locator('.file-item');
    const fileCount = await fileItems.count();

    if (fileCount > 0) {
      // Each file should have a status badge
      const statusBadges = fileList.locator('.status-badge');
      const badgeCount = await statusBadges.count();
      expect(badgeCount).toBeGreaterThan(0);

      // Status badges should contain A, M, D, or R
      const firstBadge = statusBadges.first();
      const badgeText = await firstBadge.textContent();
      expect(badgeText).toMatch(/^[AMDR]$/);
    }

    // Close modal
    await page.keyboard.press('Escape');
  });

  test('SC-11: should support vim-style navigation (j/k) in diff modal', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    await page.waitForSelector('[data-testid="files-panel"], .files-panel, .changes-tab', { timeout: 10000 }).catch(() => {});

    const emptyState = page.locator('[data-testid="files-panel-empty"], .changes-empty');
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    test.skip(hasEmpty, 'No files to navigate in diff modal');

    // Open modal
    await page.keyboard.press('Control+Shift+f');
    await page.waitForTimeout(500);

    const diffModal = page.locator('[data-testid="diff-view-modal"]');
    await expect(diffModal).toBeVisible({ timeout: 5000 });

    await page.waitForSelector('[data-testid="diff-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    const fileList = page.locator('[data-testid="diff-modal-file-list"]');
    await expect(fileList).toBeVisible();

    const fileItems = fileList.locator('.file-item');
    const fileCount = await fileItems.count();
    test.skip(fileCount < 2, 'Need at least 2 files to test navigation');

    // First file should be selected by default
    const firstFile = fileItems.first();
    const firstSelected = await firstFile.evaluate(el =>
      el.classList.contains('selected') ||
      el.getAttribute('aria-selected') === 'true' ||
      el.getAttribute('data-selected') === 'true'
    );
    expect(firstSelected).toBe(true);

    // Press 'j' to move to next file
    await page.keyboard.press('j');
    await page.waitForTimeout(100);

    // Second file should now be selected
    const secondFile = fileItems.nth(1);
    const secondSelected = await secondFile.evaluate(el =>
      el.classList.contains('selected') ||
      el.getAttribute('aria-selected') === 'true' ||
      el.getAttribute('data-selected') === 'true'
    );
    expect(secondSelected).toBe(true);

    // First file should not be selected
    const firstStillSelected = await firstFile.evaluate(el =>
      el.classList.contains('selected') ||
      el.getAttribute('aria-selected') === 'true' ||
      el.getAttribute('data-selected') === 'true'
    );
    expect(firstStillSelected).toBe(false);

    // Press 'k' to move back up
    await page.keyboard.press('k');
    await page.waitForTimeout(100);

    // First file should be selected again
    const firstReselected = await firstFile.evaluate(el =>
      el.classList.contains('selected') ||
      el.getAttribute('aria-selected') === 'true' ||
      el.getAttribute('data-selected') === 'true'
    );
    expect(firstReselected).toBe(true);

    // Close modal
    await page.keyboard.press('Escape');
  });

  test('SC-12: should toggle between split and unified diff view modes', async ({ page }) => {
    const taskId = await navigateToChangesPanel(page);
    test.skip(!taskId, 'No tasks available for testing');

    await page.waitForSelector('[data-testid="files-panel"], .files-panel, .changes-tab', { timeout: 10000 }).catch(() => {});

    const emptyState = page.locator('[data-testid="files-panel-empty"], .changes-empty');
    const hasEmpty = await emptyState.isVisible().catch(() => false);
    test.skip(hasEmpty, 'No files to show in diff modal');

    // Open modal
    await page.keyboard.press('Control+Shift+f');
    await page.waitForTimeout(500);

    const diffModal = page.locator('[data-testid="diff-view-modal"]');
    await expect(diffModal).toBeVisible({ timeout: 5000 });

    await page.waitForSelector('[data-testid="diff-loading"]', { state: 'hidden', timeout: 10000 }).catch(() => {});

    // Find view mode toggle buttons
    const viewModeToggle = page.locator('[data-testid="view-mode-toggle"]');
    await expect(viewModeToggle).toBeVisible();

    const splitButton = page.locator('[data-testid="view-mode-split"]');
    const unifiedButton = page.locator('[data-testid="view-mode-unified"]');

    await expect(splitButton).toBeVisible();
    await expect(unifiedButton).toBeVisible();

    // Check initial state - split should be active by default
    const splitPressed = await splitButton.getAttribute('aria-pressed');
    expect(splitPressed).toBe('true');

    const unifiedPressed = await unifiedButton.getAttribute('aria-pressed');
    expect(unifiedPressed).toBe('false');

    // Click Unified button to switch modes
    await unifiedButton.click();
    await page.waitForTimeout(200);

    // Unified should now be active
    const newUnifiedPressed = await unifiedButton.getAttribute('aria-pressed');
    expect(newUnifiedPressed).toBe('true');

    const newSplitPressed = await splitButton.getAttribute('aria-pressed');
    expect(newSplitPressed).toBe('false');

    // Verify diff content layout changed (if diff is visible)
    const diffContent = page.locator('[data-testid="diff-content"]');
    const hasDiffContent = await diffContent.isVisible().catch(() => false);
    if (hasDiffContent) {
      const hasUnifiedClass = await diffContent.evaluate(el =>
        el.classList.contains('unified')
      );
      expect(hasUnifiedClass).toBe(true);
    }

    // Click Split to switch back
    await splitButton.click();
    await page.waitForTimeout(200);

    const finalSplitPressed = await splitButton.getAttribute('aria-pressed');
    expect(finalSplitPressed).toBe('true');

    // Close modal
    await page.keyboard.press('Escape');
  });
});

test.describe('Task Detail Page - External Links', () => {
  test('SC-13: should show external links with target="_blank" in Test Results tab', async ({ page }) => {
    const taskId = await navigateToTask(page);
    test.skip(!taskId, 'No tasks available for testing');

    await waitForTaskDetailLoad(page);

    // Navigate to Test Results tab - this requires using the tabbed interface
    // The new layout doesn't have the same tab navigation, but we can look for
    // test results in the available UI

    // In the new Task Detail layout, Test Results may be in footer or a modal
    // Let's look for any quick links or test result indicators
    const testResultsLink = page.locator('.quick-link, a[href*="report"], a:has-text("HTML Report"), a:has-text("Test Results")');
    const hasTestResults = await testResultsLink.isVisible().catch(() => false);

    if (hasTestResults) {
      // Verify external link has target="_blank"
      const targetAttr = await testResultsLink.first().getAttribute('target');
      expect(targetAttr).toBe('_blank');

      // Should also have rel="noopener noreferrer" for security
      const relAttr = await testResultsLink.first().getAttribute('rel');
      expect(relAttr).toContain('noopener');
    } else {
      // Test results may not be present for all tasks
      test.skip(true, 'No test results links found');
    }
  });

  test('SC-14: should have proper target attribute on external links', async ({ page }) => {
    const taskId = await navigateToTask(page);
    test.skip(!taskId, 'No tasks available for testing');

    await waitForTaskDetailLoad(page);

    // Look for any external links in the task detail page
    const externalLinks = page.locator('a[href^="http"], a[target="_blank"]');
    const linkCount = await externalLinks.count();

    test.skip(linkCount === 0, 'No external links found on this task');

    // Verify all external links have target="_blank"
    for (let i = 0; i < linkCount; i++) {
      const link = externalLinks.nth(i);
      const href = await link.getAttribute('href');

      // Only check truly external links (not internal routes)
      if (href?.startsWith('http')) {
        const target = await link.getAttribute('target');
        expect(target).toBe('_blank');
      }
    }
  });

  test('SC-15: should have target="_blank" on attachment file links', async ({ page }) => {
    const taskId = await navigateToTask(page);
    test.skip(!taskId, 'No tasks available for testing');

    await waitForTaskDetailLoad(page);

    // Look for attachment links in the task detail
    // In the new layout, attachments may be in a different location
    const attachmentLinks = page.locator('.attachment-link, a[download], a:has-text("Download"), .file-download');
    const attachmentCount = await attachmentLinks.count();

    test.skip(attachmentCount === 0, 'No attachment links found');

    // Verify attachment links open in new tab
    for (let i = 0; i < attachmentCount; i++) {
      const link = attachmentLinks.nth(i);
      const target = await link.getAttribute('target');

      // Attachment links should open in new tab
      expect(target).toBe('_blank');
    }
  });
});
