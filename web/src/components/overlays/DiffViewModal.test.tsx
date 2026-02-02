/**
 * DiffViewModal Component Tests
 *
 * Tests for lazygit-style full diff view modal component:
 * - Modal structure and accessibility (SC-1)
 * - Full task diff display with file navigation (SC-2)
 * - Split/unified view mode toggle (SC-3)
 * - Keyboard navigation (SC-6)
 * - Loading states and error handling (SC-7)
 * - Integration with existing diff components (SC-8)
 * - Focus management and accessibility (SC-9)
 * - Individual file diff display (SC-10)
 * - Lazygit-style file list navigation (SC-11)
 * - Proper modal cleanup and dismissal (SC-12)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
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

// Mock browser APIs not available in jsdom
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
  global.ResizeObserver = vi.fn().mockImplementation(() => ({
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn(),
  }));
});

// Access the mocked client for setting up test scenarios
import { taskClient } from '@/lib/client';
const mockTaskClient = taskClient as any;

// Mock diff data
const mockDiffResult: DiffResult = {
  $typeName: 'orc.v1.DiffResult',
  base: 'main',
  head: 'orc/TASK-123',
  stats: {
    $typeName: 'orc.v1.DiffStats',
    filesChanged: 3,
    additions: 45,
    deletions: 12,
  } as any,
  files: [
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/components/Button.tsx',
      status: 'modified',
      additions: 25,
      deletions: 5,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/utils/api.ts',
      status: 'added',
      additions: 15,
      deletions: 0,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'docs/README.md',
      status: 'modified',
      additions: 5,
      deletions: 7,
      binary: false,
      syntax: 'markdown',
      hunks: [],
    } as any,
  ],
} as DiffResult;

const mockFileDiff: FileDiff = {
  $typeName: 'orc.v1.FileDiff',
  path: 'src/components/Button.tsx',
  status: 'modified',
  additions: 25,
  deletions: 5,
  binary: false,
  syntax: 'typescript',
  hunks: [
    {
      oldStart: 1,
      oldLines: 10,
      newStart: 1,
      newLines: 15,
      lines: [
        { type: 'context', content: 'import React from "react";', oldLine: 1, newLine: 1 },
        { type: 'addition', content: 'import { cn } from "@/lib/utils";', oldLine: undefined, newLine: 2 },
        { type: 'context', content: '', oldLine: 2, newLine: 3 },
        { type: 'deletion', content: 'interface Props {', oldLine: 3, newLine: undefined },
        { type: 'addition', content: 'interface ButtonProps {', oldLine: undefined, newLine: 4 },
      ],
    },
  ],
} as any;

describe('DiffViewModal', () => {
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
    // Clean up any remaining portal content
    const portalContent = document.querySelector('.modal-backdrop');
    if (portalContent) {
      portalContent.remove();
    }
  });

  describe('Modal Structure and Accessibility (SC-1, SC-9)', () => {
    it('renders modal dialog when open is true', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(screen.getByRole('dialog')).toHaveAttribute('aria-labelledby');
      });
    });

    it('renders nothing when open is false', () => {
      render(<DiffViewModal {...defaultProps} open={false} />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('renders modal title with task ID', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('TASK-123 - Changes')).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: 'TASK-123 - Changes' })).toBeInTheDocument();
      });
    });

    it('renders close button with proper accessibility', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close diff view' });
        expect(closeButton).toBeInTheDocument();
        expect(closeButton).toHaveAttribute('aria-label', 'Close diff view');
      });
    });

    it('calls onClose when close button is clicked', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close diff view' });
        return user.click(closeButton);
      });

      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when Escape key is pressed', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        fireEvent.keyDown(dialog, { key: 'Escape' });
      });

      await waitFor(() => {
        expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('uses large modal size for full diff viewing', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(document.querySelector('.modal-content')).toHaveClass('max-width-xl');
      });
    });

    it('sets initial focus to file list when opened', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        expect(fileList).toHaveFocus();
      });
    });
  });

  describe('Full Task Diff Display (SC-2, SC-8, SC-10)', () => {
    it('fetches and displays task diff on mount', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(mockTaskClient.getDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
        });
      });

      await waitFor(() => {
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
        expect(screen.getByText('docs/README.md')).toBeInTheDocument();
      });
    });

    it('displays diff statistics in modal header', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText('3 files')).toBeInTheDocument();
        expect(screen.getByText('+45')).toBeInTheDocument();
        expect(screen.getByText('-12')).toBeInTheDocument();
      });
    });

    it('renders file list with status indicators', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('file-status-modified')).toBeInTheDocument();
        expect(screen.getByTestId('file-status-added')).toBeInTheDocument();
        expect(screen.getByText('M')).toBeInTheDocument(); // Modified badge
        expect(screen.getByText('A')).toBeInTheDocument(); // Added badge
      });
    });

    it('loads and displays individual file diff when file is selected', async () => {
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
        });
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toBeInTheDocument();
        expect(screen.getByText('import React from "react";')).toBeInTheDocument();
        expect(screen.getByText('import { cn } from "@/lib/utils";')).toBeInTheDocument();
      });
    });

    it('highlights selected file in file list', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveClass('selected');
      });
    });

    it('shows file path in content header when file is selected', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-content-header')).toBeInTheDocument();
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
      });
    });
  });

  describe('Split/Unified View Mode Toggle (SC-3)', () => {
    it('renders view mode toggle buttons', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('view-mode-toggle')).toBeInTheDocument();
        expect(screen.getByText('Split')).toBeInTheDocument();
        expect(screen.getByText('Unified')).toBeInTheDocument();
      });
    });

    it('defaults to split view mode', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const splitButton = screen.getByRole('button', { name: 'Split' });
        expect(splitButton).toHaveClass('active');
      });
    });

    it('switches to unified view when unified button is clicked', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        return user.click(unifiedButton);
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Unified' })).toHaveClass('active');
        expect(screen.getByRole('button', { name: 'Split' })).not.toHaveClass('active');
      });
    });

    it('applies view mode to diff content display', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      // Select a file first
      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      // Switch to unified mode
      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        return user.click(unifiedButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toHaveClass('unified');
      });
    });

    it('preserves view mode selection across file switches', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      // Switch to unified mode
      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        return user.click(unifiedButton);
      });

      // Select a file
      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      // Select another file
      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/utils/api.ts');
        return user.click(fileItem);
      });

      // Unified mode should still be active
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Unified' })).toHaveClass('active');
      });
    });
  });

  describe('Keyboard Navigation (SC-6, SC-11)', () => {
    it('supports arrow key navigation in file list', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        fireEvent.keyDown(fileList, { key: 'ArrowDown' });
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });

      // Navigate to next file
      fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });

      await waitFor(() => {
        const secondFile = screen.getByTestId('file-item-src/utils/api.ts');
        expect(secondFile).toHaveFocus();
      });
    });

    it('supports Enter key to select focused file', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        fireEvent.keyDown(fileList, { key: 'ArrowDown' });
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        fireEvent.keyDown(firstFile, { key: 'Enter' });
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/components/Button.tsx',
        });
      });
    });

    it('supports j/k keys for lazygit-style navigation', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        fireEvent.keyDown(fileList, { key: 'j' }); // Down
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'j' }); // Down

      await waitFor(() => {
        const secondFile = screen.getByTestId('file-item-src/utils/api.ts');
        expect(secondFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'k' }); // Up

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('wraps navigation at file list boundaries', async () => {
      render(<DiffViewModal {...defaultProps} />);

      // Navigate to last file
      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        fireEvent.keyDown(fileList, { key: 'End' });
      });

      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-docs/README.md');
        expect(lastFile).toHaveFocus();
      });

      // Navigate down from last file should wrap to first
      fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('supports Home/End keys for first/last file navigation', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        fireEvent.keyDown(fileList, { key: 'Home' });
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'End' });

      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-docs/README.md');
        expect(lastFile).toHaveFocus();
      });
    });
  });

  describe('Loading States and Error Handling (SC-7)', () => {
    it('shows loading state while fetching diff', () => {
      mockTaskClient.getDiff.mockImplementation(
        () => new Promise(resolve => setTimeout(resolve, 1000))
      );

      render(<DiffViewModal {...defaultProps} />);

      expect(screen.getByTestId('diff-modal-loading')).toBeInTheDocument();
      expect(screen.getByText('Loading diff...')).toBeInTheDocument();
    });

    it('shows error state when diff loading fails', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Failed to load diff'));

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('diff-modal-error')).toBeInTheDocument();
        expect(screen.getByText('Failed to load diff')).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Network error'));

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const retryButton = screen.getByTestId('retry-load-diff');
        expect(retryButton).toBeInTheDocument();
      });
    });

    it('retries diff loading when retry button is clicked', async () => {
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

    it('shows loading state for individual file diff', async () => {
      const user = userEvent.setup();
      mockTaskClient.getFileDiff.mockImplementation(
        () => new Promise(resolve => setTimeout(resolve, 1000))
      );

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileItem = screen.getByTestId('file-item-src/components/Button.tsx');
        return user.click(fileItem);
      });

      await waitFor(() => {
        expect(screen.getByTestId('file-diff-loading')).toBeInTheDocument();
      });
    });

    it('shows error state when file diff loading fails', async () => {
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
    });

    it('shows empty state when no files are changed', async () => {
      mockTaskClient.getDiff.mockResolvedValue({
        diff: { ...mockDiffResult, files: [] },
      });

      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('diff-modal-empty')).toBeInTheDocument();
        expect(screen.getByText('No files changed')).toBeInTheDocument();
      });
    });
  });

  describe('Modal Cleanup and Dismissal (SC-12)', () => {
    it('cleans up resources when modal is closed', async () => {
      const user = userEvent.setup();
      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
      });

      rerender(<DiffViewModal {...defaultProps} open={false} />);

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
    });

    it('calls onClose only once when Escape is pressed multiple times', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        fireEvent.keyDown(dialog, { key: 'Escape' });
        fireEvent.keyDown(dialog, { key: 'Escape' });
      });

      await waitFor(() => {
        expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('prevents modal from opening with invalid task ID', () => {
      render(<DiffViewModal {...defaultProps} taskId="" />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('prevents modal from opening with invalid project ID', () => {
      render(<DiffViewModal {...defaultProps} projectId="" />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });
});