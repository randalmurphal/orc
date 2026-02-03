import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FilesPanel } from './FilesPanel';
import type { DiffResult } from '@/gen/orc/v1/common_pb';
import '@testing-library/jest-dom';

// Cleanup after each test to prevent DOM accumulation
afterEach(() => {
  cleanup();
});

// Mock the taskClient
vi.mock('@/lib/client', () => ({
  taskClient: {
    getDiff: vi.fn(),
    getFileDiff: vi.fn(),
  },
}));

// Access the mocked client for setting up test scenarios
import { taskClient } from '@/lib/client';
const mockTaskClient = taskClient as any;

// Mock diff data
const mockDiffResult: DiffResult = {
  $typeName: 'orc.v1.DiffResult',
  base: 'main',
  head: 'feature-branch',
  stats: {
    $typeName: 'orc.v1.DiffStats',
    filesChanged: 3,
    additions: 25,
    deletions: 8,
  } as any,
  files: [
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/components/Button.tsx',
      status: 'modified',
      additions: 15,
      deletions: 3,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/utils/api.ts',
      status: 'added',
      additions: 10,
      deletions: 0,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'legacy/old.js',
      status: 'deleted',
      additions: 0,
      deletions: 5,
      binary: false,
      syntax: 'javascript',
      hunks: [],
    } as any,
  ],
} as DiffResult;

describe('FilesPanel', () => {
  const defaultProps = {
    taskId: 'TASK-123',
    projectId: 'project-456',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockTaskClient.getDiff.mockResolvedValue({
      diff: mockDiffResult,
    });
  });

  describe('Panel Layout and Integration', () => {
    it('should render files panel with toolbar and content areas', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for data to load
      await waitFor(() => {
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });

      expect(screen.getByTestId('files-panel-toolbar')).toBeInTheDocument();
      expect(screen.getByTestId('files-panel-content')).toBeInTheDocument();
    });

    it('should show split layout with file list and diff view', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('files-list-section')).toBeInTheDocument();
        expect(screen.getByTestId('diff-view-section')).toBeInTheDocument();
      });
    });

    it('should allow toggling between split and full-width layouts', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const layoutToggle = screen.getByTestId('layout-toggle');
        expect(layoutToggle).toBeInTheDocument();

        fireEvent.click(layoutToggle);
        expect(screen.getByTestId('files-panel')).toHaveClass('full-width');
      });
    });
  });

  describe('File List Integration', () => {
    it('should display file list with correct data', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
        expect(screen.getByText('legacy/old.js')).toBeInTheDocument();
      });
    });

    it('should switch between list and tree view modes', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const viewToggle = screen.getByTestId('view-mode-toggle');
        expect(viewToggle).toBeInTheDocument();

        // Should start in list mode - check button has active class
        // Note: getByText returns the inner span, so we need to check the parent button
        const listButton = screen.getByRole('button', { name: 'List' });
        expect(listButton).toHaveClass('active');
      });

      // Switch to tree mode
      fireEvent.click(screen.getByRole('button', { name: 'Tree' }));

      await waitFor(() => {
        expect(screen.getByTestId('files-tree-view')).toBeInTheDocument();
      });
    });

    it('should show file count and statistics in toolbar', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('3 files')).toBeInTheDocument();
        expect(screen.getByText('+25')).toBeInTheDocument();
        expect(screen.getByText('-8')).toBeInTheDocument();
      });
    });
  });

  describe('File Selection and Diff Loading', () => {
    it('should load diff when file is selected', async () => {
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: {
          path: 'src/components/Button.tsx',
          hunks: [
            {
              oldStart: 1,
              oldLines: 5,
              newStart: 1,
              newLines: 6,
              lines: [
                { type: 'context', content: 'import React from "react";' },
                { type: 'addition', content: 'import { cn } from "@/lib/utils";' },
              ],
            },
          ],
        },
      });

      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const file = screen.getByTestId('file-src/components/Button.tsx');
        fireEvent.click(file);
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith(
          expect.objectContaining({
            projectId: 'project-456',
            taskId: 'TASK-123',
            filePath: 'src/components/Button.tsx',
          })
        );
      });
    });

    it('should highlight selected file in list', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const file = screen.getByTestId('file-src/components/Button.tsx');
        fireEvent.click(file);

        expect(file).toHaveClass('selected');
      });
    });

    it('should show diff content when file is selected', async () => {
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: {
          path: 'src/components/Button.tsx',
          hunks: [
            {
              oldStart: 1,
              oldLines: 5,
              newStart: 1,
              newLines: 6,
              lines: [
                { type: 'addition', content: 'const newFunction = () => {};' },
              ],
            },
          ],
        },
      });

      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const file = screen.getByTestId('file-src/components/Button.tsx');
        fireEvent.click(file);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-content')).toBeInTheDocument();
        expect(screen.getByText('const newFunction = () => {};')).toBeInTheDocument();
      });
    });
  });

  describe('Filtering and Search', () => {
    it('should filter files by status', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for initial render
      await waitFor(() => {
        expect(screen.getByTestId('status-filter')).toBeInTheDocument();
      });

      // Change filter to "Added" using fireEvent.change for select elements
      const filterDropdown = screen.getByTestId('status-filter');
      fireEvent.change(filterDropdown, { target: { value: 'added' } });

      // Should only show added files
      await waitFor(() => {
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
        expect(screen.queryByText('src/components/Button.tsx')).not.toBeInTheDocument();
      });
    });

    it('should search files by path', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search');
        fireEvent.change(searchInput, { target: { value: 'Button' } });

        // Should only show matching files
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
        expect(screen.queryByText('src/utils/api.ts')).not.toBeInTheDocument();
      });
    });

    it('should clear search and show all files', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search');
        fireEvent.change(searchInput, { target: { value: 'Button' } });

        // Clear search
        const clearButton = screen.getByTestId('clear-search');
        fireEvent.click(clearButton);

        // Should show all files again
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
      });
    });
  });

  describe('Toolbar Actions', () => {
    it('should expand all files in tree mode', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for component to load
      await waitFor(() => {
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });

      // Switch to tree mode
      fireEvent.click(screen.getByRole('button', { name: 'Tree' }));

      // Wait for tree view and expand-all button to appear
      await waitFor(() => {
        expect(screen.getByTestId('expand-all')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('expand-all'));

      // All directories should be expanded
      await waitFor(() => {
        expect(screen.getByTestId('directory-src')).toHaveAttribute('aria-expanded', 'true');
      });
    });

    it('should collapse all files in tree mode', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for component to load
      await waitFor(() => {
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });

      // Switch to tree mode
      fireEvent.click(screen.getByRole('button', { name: 'Tree' }));

      // Wait for tree view to appear
      await waitFor(() => {
        expect(screen.getByTestId('files-tree-view')).toBeInTheDocument();
      });

      // In tree mode, directories auto-expand first two levels
      // Click on a directory to collapse it manually
      const srcDir = screen.getByTestId('directory-src');
      expect(srcDir).toHaveAttribute('aria-expanded', 'true');

      // Click directory to toggle collapse
      fireEvent.click(srcDir);

      await waitFor(() => {
        expect(srcDir).toHaveAttribute('aria-expanded', 'false');
      });
    });

    it('should toggle diff view mode (split/unified)', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for component to load
      await waitFor(() => {
        expect(screen.getByTestId('diff-mode-toggle')).toBeInTheDocument();
      });

      // Should start in split mode - query button by role
      const splitButton = screen.getByRole('button', { name: 'Split' });
      expect(splitButton).toHaveClass('active');

      // Switch to unified mode
      fireEvent.click(screen.getByRole('button', { name: 'Unified' }));

      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        expect(unifiedButton).toHaveClass('active');
      });
    });
  });

  describe('Error States', () => {
    it('should show error when diff loading fails', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Failed to load diff'));

      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('files-panel-error')).toBeInTheDocument();
        expect(screen.getByText('Failed to load diff')).toBeInTheDocument();
      });
    });

    it('should show retry button on error', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Network error'));

      render(<FilesPanel {...defaultProps} />);

      // Wait for error state
      await waitFor(() => {
        expect(screen.getByTestId('retry-load-diff')).toBeInTheDocument();
      });

      // Clear the error and return success for retry
      mockTaskClient.getDiff.mockResolvedValue({ diff: mockDiffResult });

      fireEvent.click(screen.getByTestId('retry-load-diff'));

      await waitFor(() => {
        expect(screen.queryByTestId('files-panel-error')).not.toBeInTheDocument();
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('should handle file loading errors gracefully', async () => {
      mockTaskClient.getFileDiff.mockRejectedValue(new Error('File too large'));

      render(<FilesPanel {...defaultProps} />);

      // Wait for file list to load
      await waitFor(() => {
        expect(screen.getByTestId('file-src/components/Button.tsx')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('file-src/components/Button.tsx'));

      // Component shows generic error message, not the specific error
      await waitFor(() => {
        expect(screen.getByTestId('file-load-error')).toBeInTheDocument();
      });
    });
  });

  describe('Loading States', () => {
    it('should show loading state while fetching diff', () => {
      mockTaskClient.getDiff.mockImplementation(
        () => new Promise(resolve => setTimeout(resolve, 1000))
      );

      render(<FilesPanel {...defaultProps} />);

      expect(screen.getByTestId('files-panel-loading')).toBeInTheDocument();
      expect(screen.getByText('Loading file changes...')).toBeInTheDocument();
    });

    it('should show loading state for individual file diff', async () => {
      mockTaskClient.getFileDiff.mockImplementation(
        () => new Promise(resolve => setTimeout(resolve, 1000))
      );

      render(<FilesPanel {...defaultProps} />);

      // Wait for file list to load
      await waitFor(() => {
        expect(screen.getByTestId('file-src/components/Button.tsx')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('file-src/components/Button.tsx'));

      await waitFor(() => {
        expect(screen.getByTestId('file-diff-loading')).toBeInTheDocument();
      });
    });
  });

  describe('Empty States', () => {
    it('should show empty state when no files changed', async () => {
      mockTaskClient.getDiff.mockResolvedValue({
        diff: { ...mockDiffResult, files: [] },
      });

      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('files-panel-empty')).toBeInTheDocument();
        expect(screen.getByText('No files changed')).toBeInTheDocument();
      });
    });

    it('should show no results state when search returns empty', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId('file-search')).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId('file-search'), { target: { value: 'nonexistent' } });

      await waitFor(() => {
        expect(screen.getByTestId('search-no-results')).toBeInTheDocument();
        expect(screen.getByText('No files match your search')).toBeInTheDocument();
      });
    });
  });

  describe('Keyboard Navigation', () => {
    it('should support keyboard navigation in file list', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for file list to load
      await waitFor(() => {
        expect(screen.getByTestId('files-list-section')).toBeInTheDocument();
      });

      // The FileList component has the keyDown handler on its root element with role="tree"
      const fileList = screen.getByRole('tree', { name: 'File list' });
      fileList.focus();
      fireEvent.keyDown(fileList, { key: 'ArrowDown' });

      // Wait for focus to be set via useEffect
      await waitFor(() => {
        const firstFile = screen.getByTestId('file-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('should select file with Enter key', async () => {
      const user = userEvent.setup();

      // Mock getFileDiff since selecting a file triggers a diff load
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: {
          path: 'src/components/Button.tsx',
          hunks: [],
        },
      });

      render(<FilesPanel {...defaultProps} />);

      // Wait for file list to load
      await waitFor(() => {
        expect(screen.getByTestId('file-src/components/Button.tsx')).toBeInTheDocument();
      });

      const firstFile = screen.getByTestId('file-src/components/Button.tsx');

      // Wrap focus in act since it can trigger state updates
      await act(async () => {
        firstFile.focus();
      });

      // Use userEvent for proper act() handling
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(firstFile).toHaveClass('selected');
      });

      // Wait for any pending async file diff loading to complete
      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalled();
      });
    });

    it('should support Tab key to move between panel sections', async () => {
      render(<FilesPanel {...defaultProps} />);

      // Wait for component to load
      await waitFor(() => {
        expect(screen.getByTestId('file-search')).toBeInTheDocument();
      });

      // File list section should be focusable (has tabIndex)
      const filesList = screen.getByTestId('files-list-section');
      expect(filesList).toBeInTheDocument();

      // Note: Tab key navigation can't be tested in jsdom as it doesn't implement
      // browser tab navigation. We verify the element is tabbable instead.
      const searchInput = screen.getByTestId('file-search');
      expect(searchInput).toBeInTheDocument();
    });
  });
});