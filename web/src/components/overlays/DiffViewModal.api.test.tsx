/**
 * DiffViewModal API Integration Tests
 *
 * Tests for API integration and data loading behavior:
 * - Task diff API calls and data flow
 * - File diff API calls and caching
 * - Error handling and retry mechanisms
 * - Loading state management
 * - Performance optimizations (lazy loading, caching)
 * - Concurrent request handling
 * - Network failure scenarios
 * - Data consistency and validation
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DiffViewModal } from './DiffViewModal';
import type { DiffResult, FileDiff } from '@/gen/orc/v1/common_pb';
import '@testing-library/jest-dom';

// Mock the taskClient
vi.mock('@/lib/client', () => ({
  taskClient: {
    getDiff: vi.fn(),
    getFileDiff: vi.fn(),
  },
}));

// Mock browser APIs
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
  global.ResizeObserver = vi.fn().mockImplementation(() => ({
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn(),
  }));
});

import { taskClient } from '@/lib/client';
const mockTaskClient = taskClient as any;

// Mock responses
const mockDiffResult: DiffResult = {
  $typeName: 'orc.v1.DiffResult',
  base: 'main',
  head: 'orc/TASK-123',
  stats: {
    $typeName: 'orc.v1.DiffStats',
    filesChanged: 3,
    additions: 50,
    deletions: 20,
  } as any,
  files: [
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/components/Button.tsx',
      status: 'modified',
      additions: 30,
      deletions: 10,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/utils/api.ts',
      status: 'added',
      additions: 20,
      deletions: 0,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'assets/image.png',
      status: 'added',
      additions: 0,
      deletions: 0,
      binary: true,
      syntax: '',
      hunks: [],
    } as any,
  ],
} as DiffResult;

const mockFileDiff: FileDiff = {
  $typeName: 'orc.v1.FileDiff',
  path: 'src/components/Button.tsx',
  status: 'modified',
  additions: 30,
  deletions: 10,
  binary: false,
  syntax: 'typescript',
  hunks: [
    {
      oldStart: 1,
      oldLines: 15,
      newStart: 1,
      newLines: 20,
      lines: [
        { type: 'context', content: 'import React from "react";', oldLine: 1, newLine: 1 },
        { type: 'addition', content: 'import { cn } from "@/lib/utils";', oldLine: undefined, newLine: 2 },
        { type: 'context', content: '', oldLine: 2, newLine: 3 },
        { type: 'deletion', content: 'interface Props {', oldLine: 3, newLine: undefined },
        { type: 'addition', content: 'interface ButtonProps {', oldLine: undefined, newLine: 4 },
        { type: 'context', content: '  title: string;', oldLine: 4, newLine: 5 },
      ],
    },
  ],
} as any;

describe('DiffViewModal - API Integration', () => {
  const defaultProps = {
    open: true,
    taskId: 'TASK-123',
    projectId: 'project-456',
    onClose: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockTaskClient.getDiff.mockResolvedValue({ diff: mockDiffResult });
    mockTaskClient.getFileDiff.mockResolvedValue({ file: mockFileDiff });
  });

  afterEach(() => {
    cleanup();
    const portalContent = document.querySelector('.modal-backdrop');
    if (portalContent) {
      portalContent.remove();
    }
  });

  describe('Task Diff API Calls', () => {
    it('calls getDiff API with correct parameters on mount', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
        });
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
      });
    });

    it('does not call API when modal is closed', () => {
      render(<DiffViewModal {...defaultProps} open={false} />);

      expect(mockTaskClient.getDiff).not.toHaveBeenCalled();
    });

    it('calls API again when task or project ID changes', async () => {
      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
      });

      rerender(<DiffViewModal {...defaultProps} taskId="TASK-456" />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-456',
        });
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(2);
      });
    });

    it('does not call API again for same task/project when reopened', async () => {
      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
      });

      rerender(<DiffViewModal {...defaultProps} open={false} />);
      rerender(<DiffViewModal {...defaultProps} open={true} />);

      // Should still be called only once due to caching
      expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
    });

    it('includes request options for performance optimization', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          // Should include context options for diff calculation
          context: 3,
          includeStats: true,
          maxFiles: 100, // Reasonable limit
        });
      });
    });
  });

  describe('File Diff API Calls', () => {
    it('calls getFileDiff API when file is selected', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/components/Button.tsx',
          context: 3,
        });
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledTimes(1);
      });
    });

    it('does not call getFileDiff for binary files', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const binaryFile = screen.getByTestId('file-item-assets/image.png');
        return user.click(binaryFile);
      });

      // Should not call API for binary files
      expect(mockTaskClient.getFileDiff).not.toHaveBeenCalled();

      // Should show binary file message instead
      await waitFor(() => {
        expect(screen.getByTestId('binary-file-message')).toBeInTheDocument();
      });
    });

    it('caches file diff responses to avoid duplicate API calls', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      // Select file twice
      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledTimes(1);
      });

      // Select different file then return to first file
      await waitFor(() => {
        const secondFile = screen.getByTestId('file-item-src/utils/api.ts');
        return user.click(secondFile);
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(firstFile);
      });

      // Should only be called twice total (once for each unique file)
      expect(mockTaskClient.getFileDiff).toHaveBeenCalledTimes(2);
    });

    it('includes file-specific options in API call', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/components/Button.tsx',
          context: 3,
          whitespace: 'ignore-all',
          maxLines: 10000,
        });
      });
    });
  });

  describe('Concurrent Request Handling', () => {
    it('cancels in-flight requests when component unmounts', async () => {
      const abortSpy = vi.fn();
      const mockController = {
        abort: abortSpy,
        signal: { aborted: false },
      };

      vi.spyOn(window, 'AbortController').mockImplementation(() => mockController as any);

      mockTaskClient.getDiff.mockImplementation(
        () => new Promise(resolve => setTimeout(() => resolve({ diff: mockDiffResult }), 1000))
      );

      const { unmount } = render(<DiffViewModal {...defaultProps} />);

      // Unmount before request completes
      unmount();

      await waitFor(() => {
        expect(abortSpy).toHaveBeenCalled();
      });
    });

    it('handles rapid file selection without race conditions', async () => {
      const user = userEvent.setup();

      let fileCallCount = 0;
      mockTaskClient.getFileDiff.mockImplementation((request) => {
        fileCallCount++;
        return new Promise(resolve => {
          setTimeout(() => {
            resolve({
              file: {
                ...mockFileDiff,
                path: request.filePath,
              },
            });
          }, 100 * fileCallCount); // Staggered response times
        });
      });

      render(<DiffViewModal {...defaultProps} />);

      // Rapidly select multiple files
      await waitFor(() => {
        const file1 = screen.getByTestId('file-item-src/components/Button.tsx');
        const file2 = screen.getByTestId('file-item-src/utils/api.ts');

        return Promise.all([
          user.click(file1),
          user.click(file2),
        ]);
      });

      // Should display the last selected file's content
      await waitFor(() => {
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
        expect(screen.queryByText('src/components/Button.tsx')).not.toBeInTheDocument();
      });
    });

    it('batches multiple file diff requests when possible', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      // Enable batch mode (if supported)
      fireEvent.keyDown(screen.getByRole('dialog'), { key: 'b' });

      // Select multiple files rapidly
      await waitFor(() => {
        const file1 = screen.getByTestId('file-item-src/components/Button.tsx');
        const file2 = screen.getByTestId('file-item-src/utils/api.ts');

        return Promise.all([
          user.click(file1, { ctrlKey: true }),
          user.click(file2, { ctrlKey: true }),
        ]);
      });

      // Should call batch API if available
      await waitFor(() => {
        expect(mockTaskClient.getFileDiffBatch).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePaths: ['src/components/Button.tsx', 'src/utils/api.ts'],
          context: 3,
        });
      });
    });
  });

  describe('Error Handling', () => {
    it('displays error message when getDiff fails', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Failed to load task diff'));

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('diff-modal-error')).toBeInTheDocument();
        expect(screen.getByText('Failed to load task diff')).toBeInTheDocument();
      });
    });

    it('displays specific error for network timeouts', async () => {
      const timeoutError = new Error('Request timeout');
      timeoutError.name = 'TimeoutError';
      mockTaskClient.getDiff.mockRejectedValue(timeoutError);

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('Request timed out. Please try again.')).toBeInTheDocument();
      });
    });

    it('displays specific error for permission denied', async () => {
      const permissionError = new Error('Permission denied');
      permissionError.name = 'PermissionError';
      mockTaskClient.getDiff.mockRejectedValue(permissionError);

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('You do not have permission to view this diff.')).toBeInTheDocument();
      });
    });

    it('handles file diff errors gracefully without breaking modal', async () => {
      const user = userEvent.setup();
      mockTaskClient.getFileDiff.mockRejectedValue(new Error('File too large'));

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('file-diff-error')).toBeInTheDocument();
        expect(screen.getByText('File too large')).toBeInTheDocument();
      });

      // Modal should still be functional
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByTestId('diff-modal-file-list')).toBeInTheDocument();
    });

    it('provides retry functionality for failed requests', async () => {
      const user = userEvent.setup();
      mockTaskClient.getDiff.mockRejectedValueOnce(new Error('Network error'))
                            .mockResolvedValue({ diff: mockDiffResult });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const retryButton = screen.getByTestId('retry-load-diff');
        return user.click(retryButton);
      });

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(2);
        expect(screen.queryByTestId('diff-modal-error')).not.toBeInTheDocument();
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('handles malformed API responses gracefully', async () => {
      mockTaskClient.getDiff.mockResolvedValue({ diff: null });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('diff-modal-error')).toBeInTheDocument();
        expect(screen.getByText('Invalid response from server')).toBeInTheDocument();
      });
    });

    it('validates required fields in API responses', async () => {
      const invalidDiff = {
        ...mockDiffResult,
        files: [
          { path: 'invalid-file.ts' }, // Missing required fields
        ],
      };
      mockTaskClient.getDiff.mockResolvedValue({ diff: invalidDiff });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('diff-modal-error')).toBeInTheDocument();
        expect(screen.getByText('Invalid diff data received')).toBeInTheDocument();
      });
    });
  });

  describe('Performance Optimizations', () => {
    it('implements lazy loading for large file lists', async () => {
      const largeDiff = {
        ...mockDiffResult,
        files: Array.from({ length: 100 }, (_, i) => ({
          $typeName: 'orc.v1.FileDiff',
          path: `src/file${i}.ts`,
          status: 'modified',
          additions: 10,
          deletions: 5,
          binary: false,
          syntax: 'typescript',
          hunks: [],
        })),
      } as DiffResult;

      mockTaskClient.getDiff.mockResolvedValue({ diff: largeDiff });

      render(<DiffViewModal {...defaultProps} />);

      // Should only render first batch of files initially
      await waitFor(() => {
        expect(screen.getAllByTestId(/^file-item-/).length).toBeLessThan(100);
        expect(screen.getByTestId('load-more-files')).toBeInTheDocument();
      });
    });

    it('uses virtual scrolling for very large file lists', async () => {
      const massiveDiff = {
        ...mockDiffResult,
        files: Array.from({ length: 1000 }, (_, i) => ({
          $typeName: 'orc.v1.FileDiff',
          path: `src/file${i}.ts`,
          status: 'modified',
          additions: 10,
          deletions: 5,
          binary: false,
          syntax: 'typescript',
          hunks: [],
        })),
      } as DiffResult;

      mockTaskClient.getDiff.mockResolvedValue({ diff: massiveDiff });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        expect(fileList).toHaveAttribute('data-virtualized', 'true');
      });
    });

    it('preloads adjacent file diffs for faster navigation', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      // Should preload next file
      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/utils/api.ts', // Next file
          context: 3,
          priority: 'low',
        });
      });
    });

    it('debounces API calls during rapid navigation', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      // Rapidly navigate between files
      await waitFor(() => {
        const files = screen.getAllByTestId(/^file-item-/);
        return Promise.all(files.slice(0, 3).map(file => user.click(file)));
      });

      // Should only call API for the final selection
      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Data Consistency', () => {
    it('refetches diff when task state changes', async () => {
      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
      });

      // Simulate task state change (e.g., new commits)
      rerender(<DiffViewModal {...defaultProps} refreshKey="new-commit" />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(2);
      });
    });

    it('maintains data integrity during modal lifecycle', async () => {
      const user = userEvent.setup();
      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      // Load file diff
      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toBeInTheDocument();
      });

      // Close and reopen modal
      rerender(<DiffViewModal {...defaultProps} open={false} />);
      rerender(<DiffViewModal {...defaultProps} open={true} />);

      // Data should be preserved
      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toBeInTheDocument();
      });
    });

    it('validates data consistency between getDiff and getFileDiff responses', async () => {
      const user = userEvent.setup();

      // Mock inconsistent response
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: {
          ...mockFileDiff,
          path: 'different-file.ts', // Inconsistent with requested path
        },
      });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-consistency-error')).toBeInTheDocument();
        expect(screen.getByText('Data inconsistency detected. Please refresh.')).toBeInTheDocument();
      });
    });
  });
});