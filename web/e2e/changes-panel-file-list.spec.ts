/**
 * E2E Tests for Changes Panel File List Enhancement
 *
 * Tests the enhanced file list functionality within the Changes tab on Task Detail page.
 * Covers the integration of simplified file navigation, tree view, and diff viewer.
 *
 * Test Coverage (12 tests):
 * - File List Display (3): file visibility, statistics, status indicators
 * - File Tree Navigation (3): hierarchical view, expand/collapse, directory navigation
 * - File Selection Integration (3): diff loading, selection sync, keyboard navigation
 * - View Mode Switching (2): list/tree toggle, responsive behavior
 * - Error Handling (1): graceful error states
 *
 * Success Criteria Mapping:
 * - SC-1: Display condensed file list → File List Display tests
 * - SC-2: Hierarchical file tree → File Tree Navigation tests
 * - SC-3: Directory expand/collapse → File Tree Navigation tests
 * - SC-4: Diff viewer integration → File Selection Integration tests
 * - SC-5: Keyboard accessibility → File Selection Integration tests
 * - SC-6: Loading/error states → Error Handling tests
 * - SC-7: Status filtering → File List Display tests
 */
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';

// Helper: Navigate to task detail changes tab
async function navigateToChangesTab(page: Page): Promise<string | null> {
  await page.goto('/');
  await page.waitForLoadState('networkidle');

  await page.goto('/board');
  await page.waitForLoadState('networkidle');

  const taskCards = page.locator('.task-card');
  const count = await taskCards.count();

  if (count === 0) {
    return null;
  }

  // Click first task to navigate to detail
  await taskCards.first().click();
  await page.waitForURL(/\/tasks\/TASK-\d+/);

  // Click Changes tab
  const changesTab = page.locator('[role="tab"]:has-text("Changes")');
  await changesTab.click();
  await page.waitForTimeout(200);

  const match = page.url().match(/TASK-\d+/);
  return match?.[0] || null;
}

test.describe('Changes Panel File List Enhancement', () => {
  test.describe('SC-1: File List Display', () => {
    test('should display files in condensed list format with file names and paths', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      // Wait for enhanced changes panel to load
      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Check for file list container
      const fileList = page.locator('.file-list-container');
      await expect(fileList).toBeVisible();

      // Files should be displayed with clear paths
      const fileItems = page.locator('.file-item');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        const firstFile = fileItems.first();

        // File should have visible path
        const filePath = firstFile.locator('.file-path');
        await expect(filePath).toBeVisible();

        const pathText = await filePath.textContent();
        expect(pathText?.length).toBeGreaterThan(0);
        expect(pathText).toMatch(/\.(ts|tsx|js|jsx|py|go|md|json)$/); // Common file extensions
      }
    });

    test('should show file status indicators (added, modified, deleted)', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      const fileItems = page.locator('.file-item');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        // Check for status indicators
        const statusIndicators = page.locator('.file-status-indicator');
        const indicatorCount = await statusIndicators.count();
        expect(indicatorCount).toBeGreaterThan(0);

        // Check for specific status types
        const addedFiles = page.locator('.file-status-added');
        const modifiedFiles = page.locator('.file-status-modified');
        const deletedFiles = page.locator('.file-status-deleted');

        const addedCount = await addedFiles.count();
        const modifiedCount = await modifiedFiles.count();
        const deletedCount = await deletedFiles.count();

        // At least one type should be present
        expect(addedCount + modifiedCount + deletedCount).toBeGreaterThan(0);
      }
    });

    test('should display addition and deletion counts for each file', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      const fileItems = page.locator('.file-item');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        const firstFile = fileItems.first();

        // Check for change statistics
        const fileStats = firstFile.locator('.file-stats');
        await expect(fileStats).toBeVisible();

        // Look for addition/deletion indicators
        const additions = fileStats.locator('.additions, [class*="addition"]');
        const deletions = fileStats.locator('.deletions, [class*="deletion"]');

        const hasAdditions = await additions.isVisible().catch(() => false);
        const hasDeletions = await deletions.isVisible().catch(() => false);

        // At least one should be visible (or both for modifications)
        expect(hasAdditions || hasDeletions).toBe(true);

        if (hasAdditions) {
          const addText = await additions.textContent();
          expect(addText).toMatch(/\+\d+/);
        }

        if (hasDeletions) {
          const delText = await deletions.textContent();
          expect(delText).toMatch(/-\d+/);
        }
      }
    });
  });

  test.describe('SC-2 & SC-3: File Tree Navigation and Directory Control', () => {
    test('should group files hierarchically by directory in tree view', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Switch to tree view
      const treeViewToggle = page.locator('.view-mode-toggle [data-mode="tree"], .tree-view-btn');
      const hasTreeToggle = await treeViewToggle.isVisible().catch(() => false);

      if (hasTreeToggle) {
        await treeViewToggle.click();
        await page.waitForTimeout(300);

        // Check for directory structure
        const directories = page.locator('.directory-node, .tree-directory');
        const dirCount = await directories.count();

        if (dirCount > 0) {
          // Directories should be expandable
          const firstDir = directories.first();
          const dirLabel = firstDir.locator('.directory-label, .dir-name');
          await expect(dirLabel).toBeVisible();

          // Directory should have expand/collapse indicator
          const expandIcon = firstDir.locator('.expand-icon, .chevron, [data-testid*="chevron"]');
          await expect(expandIcon).toBeVisible();
        }
      } else {
        test.skip('Tree view not available for this diff');
      }
    });

    test('should expand and collapse directories when clicked', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Switch to tree view
      const treeViewToggle = page.locator('.view-mode-toggle [data-mode="tree"], .tree-view-btn');
      const hasTreeToggle = await treeViewToggle.isVisible().catch(() => false);

      if (hasTreeToggle) {
        await treeViewToggle.click();
        await page.waitForTimeout(300);

        const directories = page.locator('.directory-node, .tree-directory');
        const dirCount = await directories.count();

        if (dirCount > 0) {
          const firstDir = directories.first();

          // Check initial state
          const initialExpanded = await firstDir.getAttribute('aria-expanded');

          // Click to toggle
          await firstDir.click();
          await page.waitForTimeout(200);

          // State should change
          const newExpanded = await firstDir.getAttribute('aria-expanded');
          expect(newExpanded).not.toBe(initialExpanded);

          // If expanded, should show child files/directories
          if (newExpanded === 'true') {
            const children = firstDir.locator('.tree-children, .directory-children');
            await expect(children).toBeVisible({ timeout: 1000 });
          }
        }
      }
    });

    test('should show proper indentation for nested directory levels', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Switch to tree view
      const treeViewToggle = page.locator('.view-mode-toggle [data-mode="tree"], .tree-view-btn');
      const hasTreeToggle = await treeViewToggle.isVisible().catch(() => false);

      if (hasTreeToggle) {
        await treeViewToggle.click();
        await page.waitForTimeout(300);

        // Expand directories to see nesting
        const directories = page.locator('.directory-node, .tree-directory');
        const dirCount = await directories.count();

        if (dirCount > 0) {
          // Expand first directory
          await directories.first().click();
          await page.waitForTimeout(200);

          // Look for nested items
          const nestedItems = page.locator('.tree-item[data-level="1"], .nested-item');
          const nestedCount = await nestedItems.count();

          if (nestedCount > 0) {
            const firstNested = nestedItems.first();

            // Nested items should have visual indentation
            const marginLeft = await firstNested.evaluate(el =>
              getComputedStyle(el).marginLeft || getComputedStyle(el).paddingLeft
            );

            // Should have some indentation (at least 1rem or 16px)
            expect(parseInt(marginLeft)).toBeGreaterThanOrEqual(16);
          }
        }
      }
    });
  });

  test.describe('SC-4 & SC-5: File Selection Integration and Keyboard Navigation', () => {
    test('should load diff content when file is selected', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      const fileItems = page.locator('.file-item, [data-testid*="file-"]');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        const firstFile = fileItems.first();

        // Click to select file
        await firstFile.click();
        await page.waitForTimeout(500);

        // File should be highlighted as selected
        const isSelected = await firstFile.evaluate(el =>
          el.classList.contains('selected') ||
          el.classList.contains('active') ||
          el.getAttribute('aria-selected') === 'true'
        );
        expect(isSelected).toBe(true);

        // Diff content should load
        const diffContent = page.locator('.diff-viewer, .diff-content, .file-diff');
        await expect(diffContent).toBeVisible({ timeout: 5000 });

        // Should show actual diff lines or loading indicator
        const diffLines = page.locator('.diff-line, .diff-hunk, .loading-spinner');
        await expect(diffLines.first()).toBeVisible({ timeout: 3000 });
      }
    });

    test('should support keyboard navigation through file list', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      const filesList = page.locator('.file-list-container, .files-panel');
      await expect(filesList).toBeVisible();

      // Focus the file list
      await filesList.focus();

      // Press down arrow to navigate
      await page.keyboard.press('ArrowDown');
      await page.waitForTimeout(100);

      // Some file should now have focus
      const focusedElement = page.locator(':focus');
      const hasFocus = await focusedElement.isVisible();
      expect(hasFocus).toBe(true);

      // Test Enter key to select
      await page.keyboard.press('Enter');
      await page.waitForTimeout(300);

      // Should trigger selection (diff loading)
      const diffViewer = page.locator('.diff-viewer, .diff-content');
      const hasDiff = await diffViewer.isVisible().catch(() => false);
      expect(hasDiff).toBe(true);
    });

    test('should maintain file selection state across view mode changes', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      const fileItems = page.locator('.file-item, [data-testid*="file-"]');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        // Select a file
        const firstFile = fileItems.first();
        const fileName = await firstFile.textContent();
        await firstFile.click();
        await page.waitForTimeout(300);

        // Switch view modes
        const viewToggle = page.locator('.view-mode-toggle');
        const hasViewToggle = await viewToggle.isVisible().catch(() => false);

        if (hasViewToggle) {
          const treeButton = viewToggle.locator('[data-mode="tree"], .tree-view-btn');
          const hasTreeButton = await treeButton.isVisible().catch(() => false);

          if (hasTreeButton) {
            await treeButton.click();
            await page.waitForTimeout(300);

            // File should still be selected in new view
            const selectedInTree = page.locator('.selected, .active, [aria-selected="true"]');
            const stillSelected = await selectedInTree.isVisible().catch(() => false);
            expect(stillSelected).toBe(true);

            // Switch back to list view
            const listButton = viewToggle.locator('[data-mode="list"], .list-view-btn');
            const hasListButton = await listButton.isVisible().catch(() => false);

            if (hasListButton) {
              await listButton.click();
              await page.waitForTimeout(300);

              // Selection should persist
              const selectedInList = page.locator('.selected, .active, [aria-selected="true"]');
              await expect(selectedInList).toBeVisible();
            }
          }
        }
      }
    });
  });

  test.describe('View Mode Switching', () => {
    test('should toggle between list and tree view modes', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Look for view mode toggle
      const viewToggle = page.locator('.view-mode-toggle, .view-switcher');
      const hasToggle = await viewToggle.isVisible().catch(() => false);

      if (hasToggle) {
        // Should have list and tree options
        const listOption = page.locator('.list-view-btn, [data-mode="list"]:has-text("List")');
        const treeOption = page.locator('.tree-view-btn, [data-mode="tree"]:has-text("Tree")');

        await expect(listOption).toBeVisible();
        await expect(treeOption).toBeVisible();

        // Default mode (usually list)
        const initialActive = await listOption.getAttribute('class');
        const isListActive = initialActive?.includes('active') ||
                           await listOption.getAttribute('aria-selected') === 'true';

        // Switch to tree view
        await treeOption.click();
        await page.waitForTimeout(300);

        // Tree view should be active
        const treeActiveClass = await treeOption.getAttribute('class');
        const isTreeActive = treeActiveClass?.includes('active') ||
                           await treeOption.getAttribute('aria-selected') === 'true';
        expect(isTreeActive).toBe(true);

        // Layout should change
        const treeView = page.locator('.tree-view, .hierarchical-view, .file-tree');
        await expect(treeView).toBeVisible();

        // Switch back to list
        await listOption.click();
        await page.waitForTimeout(300);

        const listView = page.locator('.list-view, .flat-view, .file-list');
        await expect(listView).toBeVisible();
      } else {
        test.skip('View mode toggle not available');
      }
    });

    test('should adapt layout for smaller screens', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      // Resize to mobile viewport
      await page.setViewportSize({ width: 600, height: 800 });
      await page.waitForTimeout(300);

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Panel should adapt to mobile
      const panel = page.locator('.enhanced-file-panel, .changes-panel');
      await expect(panel).toBeVisible();

      // Should have mobile-optimized layout classes or behavior
      const isMobileLayout = await panel.evaluate(el => {
        const style = getComputedStyle(el);
        return el.classList.contains('mobile') ||
               el.classList.contains('responsive') ||
               parseInt(style.width) <= 600;
      });

      expect(isMobileLayout).toBe(true);

      // File items should stack appropriately
      const fileItems = page.locator('.file-item');
      const fileCount = await fileItems.count();

      if (fileCount > 0) {
        const firstFile = fileItems.first();
        const fileWidth = await firstFile.evaluate(el => el.offsetWidth);

        // File items should use most of available width
        expect(fileWidth).toBeGreaterThan(400);
      }
    });
  });

  test.describe('SC-6: Error and Loading States', () => {
    test('should handle diff loading errors gracefully', async ({ page }) => {
      const taskId = await navigateToChangesTab(page);
      test.skip(!taskId, 'No tasks available for testing');

      await page.waitForSelector('.enhanced-file-panel', { timeout: 10000 });

      // Look for any error states that might exist
      const errorStates = page.locator('.error-state, .diff-error, .load-error, .error-message');
      const errorCount = await errorStates.count();

      if (errorCount > 0) {
        // Error should be user-friendly
        const firstError = errorStates.first();
        await expect(firstError).toBeVisible();

        const errorText = await firstError.textContent();
        expect(errorText?.length).toBeGreaterThan(0);
        expect(errorText?.toLowerCase()).toMatch(/error|failed|unable|problem/);

        // Should have retry mechanism
        const retryButton = page.locator('.retry-btn, .reload-btn, button:has-text("Retry")');
        const hasRetry = await retryButton.isVisible().catch(() => false);

        if (hasRetry) {
          expect(retryButton).toBeVisible();
        }
      } else {
        // No errors is also a valid state - check for loading completion
        const loadingStates = page.locator('.loading-spinner, .loading-state');
        const loadingCount = await loadingStates.count();
        expect(loadingCount).toBe(0); // Should not be stuck loading
      }
    });
  });
});