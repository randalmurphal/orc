import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FilesPanel } from './FilesPanel';
import type { FileDiff, DiffResult } from '@/gen/orc/v1/common_pb';
import '@testing-library/jest-dom';

// Mock the taskClient
const mockTaskClient = {
  getDiff: vi.fn(),
  getFileDiff: vi.fn(),
};

vi.mock('@/lib/client', () => ({
  taskClient: mockTaskClient,
}));

// Mock diff data
const mockDiffResult: DiffResult = {
  base: 'main',
  head: 'feature-branch',
  stats: {
    filesChanged: 3,
    additions: 25,
    deletions: 8,
  },
  files: [
    {
      path: 'src/components/Button.tsx',
      status: 'modified',
      additions: 15,
      deletions: 3,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    },
    {
      path: 'src/utils/api.ts',
      status: 'added',
      additions: 10,
      deletions: 0,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    },
    {
      path: 'legacy/old.js',
      status: 'deleted',
      additions: 0,
      deletions: 5,
      binary: false,
      syntax: 'javascript',
      hunks: [],
    },
  ],
};

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

        // Should start in list mode
        expect(screen.getByText('List')).toHaveClass('active');

        // Switch to tree mode
        fireEvent.click(screen.getByText('Tree'));
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

      await waitFor(() => {
        const filterDropdown = screen.getByTestId('status-filter');
        fireEvent.click(filterDropdown);
        fireEvent.click(screen.getByText('Added'));

        // Should only show added files
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

      await waitFor(() => {
        // Switch to tree mode
        fireEvent.click(screen.getByText('Tree'));

        const expandAllButton = screen.getByTestId('expand-all');
        fireEvent.click(expandAllButton);

        // All directories should be expanded
        expect(screen.getByTestId('directory-src')).toHaveAttribute('aria-expanded', 'true');
      });
    });

    it('should collapse all files in tree mode', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        // Switch to tree mode and expand all first
        fireEvent.click(screen.getByText('Tree'));
        fireEvent.click(screen.getByTestId('expand-all'));

        // Then collapse all
        const collapseAllButton = screen.getByTestId('collapse-all');
        fireEvent.click(collapseAllButton);

        expect(screen.getByTestId('directory-src')).toHaveAttribute('aria-expanded', 'false');
      });
    });

    it('should toggle diff view mode (split/unified)', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const diffModeToggle = screen.getByTestId('diff-mode-toggle');
        expect(diffModeToggle).toBeInTheDocument();

        // Should start in split mode
        expect(screen.getByText('Split')).toHaveClass('active');

        // Switch to unified mode
        fireEvent.click(screen.getByText('Unified'));
        expect(screen.getByText('Unified')).toHaveClass('active');
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

      await waitFor(() => {
        const retryButton = screen.getByTestId('retry-load-diff');
        expect(retryButton).toBeInTheDocument();

        // Clear the error and return success
        mockTaskClient.getDiff.mockResolvedValue({ diff: mockDiffResult });

        fireEvent.click(retryButton);
      });

      await waitFor(() => {
        expect(screen.queryByTestId('files-panel-error')).not.toBeInTheDocument();
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('should handle file loading errors gracefully', async () => {
      mockTaskClient.getFileDiff.mockRejectedValue(new Error('File too large'));

      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const file = screen.getByTestId('file-src/components/Button.tsx');
        fireEvent.click(file);
      });

      await waitFor(() => {
        expect(screen.getByTestId('file-load-error')).toBeInTheDocument();
        expect(screen.getByText('File too large')).toBeInTheDocument();
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

      await waitFor(() => {
        const file = screen.getByTestId('file-src/components/Button.tsx');
        fireEvent.click(file);

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

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search');
        fireEvent.change(searchInput, { target: { value: 'nonexistent' } });

        expect(screen.getByTestId('search-no-results')).toBeInTheDocument();
        expect(screen.getByText('No files match your search')).toBeInTheDocument();
      });
    });
  });

  describe('Keyboard Navigation', () => {
    it('should support keyboard navigation in file list', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const filesList = screen.getByTestId('files-list-section');
        fireEvent.keyDown(filesList, { key: 'ArrowDown' });

        const firstFile = screen.getByTestId('file-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('should select file with Enter key', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-src/components/Button.tsx');
        firstFile.focus();
        fireEvent.keyDown(firstFile, { key: 'Enter' });

        expect(firstFile).toHaveClass('selected');
      });
    });

    it('should support Tab key to move between panel sections', async () => {
      render(<FilesPanel {...defaultProps} />);

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search');
        searchInput.focus();

        fireEvent.keyDown(searchInput, { key: 'Tab' });

        const filesList = screen.getByTestId('files-list-section');
        expect(filesList).toHaveFocus();
      });
    });
  });
});