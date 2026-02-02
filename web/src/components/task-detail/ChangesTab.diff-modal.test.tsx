/**
 * ChangesTab - Diff Modal Integration Tests
 *
 * Tests for integration between Task Detail ChangesTab and DiffViewModal:
 * - Modal triggering from "Expand Diff" button (SC-5)
 * - Modal triggering from fullscreen actions
 * - File-specific modal opening from diff viewer
 * - Context preservation between tab and modal
 * - Split pane state management during modal interactions
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChangesTab } from './ChangesTab';
import type { DiffResult } from '@/gen/orc/v1/common_pb';
import '@testing-library/jest-dom';

// Mock the DiffViewModal
vi.mock('@/components/overlays/DiffViewModal', () => ({
  DiffViewModal: vi.fn(({ open, taskId, projectId, selectedFile, onClose }) => (
    open ? (
      <div role="dialog" data-testid="diff-view-modal">
        <div>DiffViewModal: {taskId}/{projectId}</div>
        <div>Selected: {selectedFile || 'none'}</div>
        <button onClick={onClose}>Close</button>
      </div>
    ) : null
  )),
}));

// Mock the taskClient
vi.mock('@/lib/client', () => ({
  taskClient: {
    getDiff: vi.fn(),
    getFileDiff: vi.fn(),
  },
}));

// Mock diff data
const mockDiffResult: DiffResult = {
  $typeName: 'orc.v1.DiffResult',
  base: 'main',
  head: 'orc/TASK-123',
  stats: {
    $typeName: 'orc.v1.DiffStats',
    filesChanged: 2,
    additions: 30,
    deletions: 10,
  } as any,
  files: [
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/components/Button.tsx',
      status: 'modified',
      additions: 20,
      deletions: 5,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
    {
      $typeName: 'orc.v1.FileDiff',
      path: 'src/utils/helper.ts',
      status: 'added',
      additions: 10,
      deletions: 0,
      binary: false,
      syntax: 'typescript',
      hunks: [],
    } as any,
  ],
} as DiffResult;

// Access the mocked client
import { taskClient } from '@/lib/client';
const mockTaskClient = taskClient as any;

describe('ChangesTab - DiffViewModal Integration', () => {
  const defaultProps = {
    taskId: 'TASK-123',
    projectId: 'project-456',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockTaskClient.getDiff.mockResolvedValue({ diff: mockDiffResult });
  });

  describe('Modal Triggering from ChangesTab (SC-5)', () => {
    it('renders "Expand Diff" button in tab toolbar', async () => {
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        expect(expandButton).toBeInTheDocument();
        expect(expandButton).toHaveAttribute('title', 'Open full diff view in modal');
      });
    });

    it('renders fullscreen icon button as alternative trigger', async () => {
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const fullscreenButton = screen.getByRole('button', { name: 'Fullscreen Diff' });
        expect(fullscreenButton).toBeInTheDocument();
        expect(fullscreenButton).toHaveAttribute('aria-label', 'Open diff in fullscreen modal');
      });
    });

    it('opens DiffViewModal when "Expand Diff" button is clicked', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
        expect(screen.getByText('DiffViewModal: TASK-123/project-456')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal when fullscreen button is clicked', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const fullscreenButton = screen.getByRole('button', { name: 'Fullscreen Diff' });
        return user.click(fullscreenButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal with keyboard shortcut Ctrl+Shift+F', async () => {
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const changesTab = screen.getByTestId('changes-tab-content');
        fireEvent.keyDown(changesTab, {
          key: 'F',
          ctrlKey: true,
          shiftKey: true,
        });
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal when double-clicking on split pane divider', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const splitPaneDivider = screen.getByTestId('split-pane-divider');
        return user.dblClick(splitPaneDivider);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });
  });

  describe('File-Specific Modal Opening', () => {
    it('opens modal with specific file when file header is Shift+clicked', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Wait for diff to load and file to be displayed
      await waitFor(() => {
        const fileHeader = screen.getByTestId('file-header-src/components/Button.tsx');
        return user.click(fileHeader, { shiftKey: true });
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
        expect(screen.getByText('Selected: src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('opens modal with specific file when line number is Ctrl+clicked', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Wait for diff to load and expand file to show content
      await waitFor(() => {
        const expandFileButton = screen.getByTestId('expand-file-src/components/Button.tsx');
        return user.click(expandFileButton);
      });

      await waitFor(() => {
        const lineNumber = screen.getByTestId('line-number-5');
        return user.click(lineNumber, { ctrlKey: true });
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
        expect(screen.getByText('Selected: src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('opens modal from file context menu "View in Modal" option', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const fileHeader = screen.getByTestId('file-header-src/components/Button.tsx');
        return user.click(fileHeader, { button: 2 }); // Right click
      });

      await waitFor(() => {
        const viewInModalOption = screen.getByRole('menuitem', { name: 'View in Modal' });
        return user.click(viewInModalOption);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
        expect(screen.getByText('Selected: src/components/Button.tsx')).toBeInTheDocument();
      });
    });
  });

  describe('Context Preservation', () => {
    it('preserves split pane ratio when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Adjust split pane ratio
      await waitFor(() => {
        const splitPaneDivider = screen.getByTestId('split-pane-divider');
        fireEvent.mouseDown(splitPaneDivider);
        fireEvent.mouseMove(splitPaneDivider, { clientX: 400 });
        fireEvent.mouseUp(splitPaneDivider);
      });

      // Open and close modal
      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Split pane ratio should be preserved
      const splitPane = screen.getByTestId('changes-split-pane');
      expect(splitPane).toHaveAttribute('data-ratio', expect.stringContaining('60')); // Or whatever the adjusted ratio was
    });

    it('preserves file expansion state when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Expand a file
      await waitFor(() => {
        const expandButton = screen.getByTestId('expand-file-src/components/Button.tsx');
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('file-content-src/components/Button.tsx')).toBeInTheDocument();
      });

      // Open and close modal
      await waitFor(() => {
        const expandDiffButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandDiffButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // File should still be expanded
      await waitFor(() => {
        expect(screen.getByTestId('file-content-src/components/Button.tsx')).toBeInTheDocument();
      });
    });

    it('preserves diff view mode (split/unified) when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Switch to unified mode in tab
      await waitFor(() => {
        const unifiedToggle = screen.getByRole('button', { name: 'Unified' });
        return user.click(unifiedToggle);
      });

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-content')).toHaveClass('unified-mode');
      });

      // Open and close modal
      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Should still be in unified mode
      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-content')).toHaveClass('unified-mode');
      });
    });

    it('preserves comment threads when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Add a comment thread
      await waitFor(() => {
        const addCommentButton = screen.getByTestId('add-comment-line-10');
        return user.click(addCommentButton);
      });

      await waitFor(() => {
        const commentTextarea = screen.getByTestId('comment-textarea');
        return user.type(commentTextarea, 'Test comment');
      });

      await waitFor(() => {
        const submitButton = screen.getByRole('button', { name: 'Add Comment' });
        return user.click(submitButton);
      });

      // Open and close modal
      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Comment should still be there
      await waitFor(() => {
        expect(screen.getByText('Test comment')).toBeInTheDocument();
      });
    });
  });

  describe('Split Pane State Management', () => {
    it('disables split pane interaction when modal is open', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });

      // Split pane divider should be disabled
      const splitPaneDivider = screen.getByTestId('split-pane-divider');
      expect(splitPaneDivider).toHaveAttribute('data-disabled', 'true');
    });

    it('restores split pane interaction when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Split pane divider should be enabled
      const splitPaneDivider = screen.getByTestId('split-pane-divider');
      expect(splitPaneDivider).not.toHaveAttribute('data-disabled');
    });

    it('dims split pane content when modal is open', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-content')).toHaveClass('modal-open-dimmed');
      });
    });

    it('removes dimming when modal is closed', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-content')).not.toHaveClass('modal-open-dimmed');
      });
    });
  });

  describe('Integration Wiring Verification', () => {
    it('modal integration is properly wired into ChangesTab', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // This test FAILS if the modal integration is not wired
      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      }, { timeout: 1000 });
    });

    it('modal integration does not break existing diff functionality', async () => {
      const user = userEvent.setup();
      render(<ChangesTab {...defaultProps} />);

      // Existing functionality should still work
      await waitFor(() => {
        expect(screen.getByText('2 files')).toBeInTheDocument();
        expect(screen.getByText('+30')).toBeInTheDocument();
        expect(screen.getByText('-10')).toBeInTheDocument();
      });

      // File expansion should still work
      await waitFor(() => {
        const expandButton = screen.getByTestId('expand-file-src/components/Button.tsx');
        return user.click(expandButton);
      });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalled();
      });
    });

    it('modal integration handles component unmounting gracefully', async () => {
      const user = userEvent.setup();
      const { unmount } = render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      // Unmount component while modal is open
      unmount();

      // Should not crash or leave orphaned modals
      expect(screen.queryByTestId('diff-view-modal')).not.toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('handles modal opening failure gracefully', async () => {
      const user = userEvent.setup();
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        return user.click(expandButton);
      });

      // ChangesTab should not crash if modal fails to open
      expect(screen.getByTestId('changes-tab-content')).toBeInTheDocument();

      consoleError.mockRestore();
    });

    it('disables modal triggers when diff loading fails', async () => {
      mockTaskClient.getDiff.mockRejectedValue(new Error('Failed to load diff'));

      render(<ChangesTab {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.queryByRole('button', { name: 'Expand Diff' });
        expect(expandButton).toHaveAttribute('disabled');
      });
    });

    it('re-enables modal triggers when diff loading recovers', async () => {
      const user = userEvent.setup();
      mockTaskClient.getDiff.mockRejectedValueOnce(new Error('Network error'))
                            .mockResolvedValue({ diff: mockDiffResult });

      render(<ChangesTab {...defaultProps} />);

      // Wait for error state
      await waitFor(() => {
        const retryButton = screen.getByTestId('retry-load-diff');
        return user.click(retryButton);
      });

      // Modal trigger should be re-enabled
      await waitFor(() => {
        const expandButton = screen.getByRole('button', { name: 'Expand Diff' });
        expect(expandButton).not.toHaveAttribute('disabled');
      });
    });
  });
});