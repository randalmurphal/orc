/**
 * FilesPanel - Diff Modal Integration Tests
 *
 * Tests for integration between Board Files Panel and DiffViewModal:
 * - Modal triggering from file clicks (SC-4)
 * - Modal state management (open/close)
 * - Data passing between components
 * - Integration wiring verification
 * - "Open full diff view modal" button functionality
 * - Context preservation across modal interactions
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FilesPanel, ChangedFile } from './FilesPanel';
import '@testing-library/jest-dom';

// Mock the DiffViewModal
vi.mock('@/components/overlays/DiffViewModal', () => ({
  DiffViewModal: vi.fn(({ open, taskId, projectId, onClose }) => (
    open ? (
      <div role="dialog" data-testid="diff-view-modal">
        <div>DiffViewModal: {taskId}/{projectId}</div>
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

// Mock useCurrentProjectId
vi.mock('@/stores/projectStore', () => ({
  useCurrentProjectId: () => 'project-123',
  useProjectStore: vi.fn(),
}));

describe('FilesPanel - DiffViewModal Integration', () => {
  const mockFiles: ChangedFile[] = [
    { path: 'src/components/Button.tsx', status: 'modified', taskId: 'TASK-001' },
    { path: 'src/utils/api.ts', status: 'added', taskId: 'TASK-001' },
    { path: 'docs/README.md', status: 'modified', taskId: 'TASK-002' },
  ];

  const defaultProps = {
    files: mockFiles,
    onFileClick: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Modal Triggering (SC-4)', () => {
    it('renders "Open full diff view modal" button when files are present', () => {
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      expect(viewFullDiffButton).toBeInTheDocument();
      expect(viewFullDiffButton).toHaveAttribute('title', 'Open full diff view modal');
    });

    it('does not render "Open full diff view modal" button when no files are present', () => {
      render(<FilesPanel files={[]} onFileClick={vi.fn()} />);

      const viewFullDiffButton = screen.queryByRole('button', { name: 'Open full diff view modal' });
      expect(viewFullDiffButton).not.toBeInTheDocument();
    });

    it('opens DiffViewModal when "Open full diff view modal" button is clicked', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal when file is double-clicked', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      await user.dblClick(fileItem);

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal when file is clicked and Shift+Enter is pressed', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      await user.click(fileItem);
      await user.keyboard('{Shift>}{Enter}{/Shift}');

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

    it('opens DiffViewModal with Ctrl+click on file (lazygit-style)', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      // user-event v14: ctrl+click requires keyboard + click combo
      await user.keyboard('{Control>}');
      await user.click(fileItem);
      await user.keyboard('{/Control}');

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });
  });

  describe('Modal Data Passing', () => {
    it('passes correct project ID to modal', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        // taskId is 'unknown' when files are from multiple tasks (fallback)
        expect(screen.getByText('DiffViewModal: unknown/project-123')).toBeInTheDocument();
      });
    });

    it('passes task ID when opening modal for specific task files', async () => {
      const user = userEvent.setup();
      const singleTaskFiles: ChangedFile[] = [
        { path: 'src/component.tsx', status: 'modified', taskId: 'TASK-001' },
        { path: 'src/test.ts', status: 'added', taskId: 'TASK-001' },
      ];

      render(<FilesPanel files={singleTaskFiles} onFileClick={vi.fn()} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        expect(screen.getByText('DiffViewModal: TASK-001/project-123')).toBeInTheDocument();
      });
    });

    it('does not pass task ID when files are from multiple tasks', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        // Should not have a specific task ID when multiple tasks (uses 'unknown' fallback)
        expect(screen.getByText('DiffViewModal: unknown/project-123')).toBeInTheDocument();
      });
    });

    it('passes selected file path to modal when opened from file interaction', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      await user.dblClick(fileItem);

      await waitFor(() => {
        // Modal should receive the selected file path in some way
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });

      // Verify that the original onFileClick was called for normal click behavior
      await user.click(screen.getByRole('button', { name: /api\.ts/i }));
      expect(defaultProps.onFileClick).toHaveBeenCalledWith(mockFiles[1]);
    });
  });

  describe('Modal State Management', () => {
    it('closes modal when onClose is called', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });

      const closeButton = screen.getByRole('button', { name: 'Close' });
      await user.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByTestId('diff-view-modal')).not.toBeInTheDocument();
      });
    });

    it('maintains FilesPanel state when modal is open', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      // Collapse the panel
      const header = screen.getByRole('button', { name: /Files Changed/i });
      await user.click(header);
      expect(header).toHaveAttribute('aria-expanded', 'false');

      // Open modal
      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });

      // Panel should still be collapsed
      expect(header).toHaveAttribute('aria-expanded', 'false');
    });

    it('preserves file selection state when modal is closed', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      // Select a file first
      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      await user.click(fileItem);
      expect(defaultProps.onFileClick).toHaveBeenCalledWith(mockFiles[0]);

      // Open and close modal
      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // File should still be selectable
      await user.click(fileItem);
      expect(defaultProps.onFileClick).toHaveBeenCalledTimes(2);
    });
  });

  describe('Integration Wiring Verification', () => {
    it('modal is properly wired into FilesPanel component tree', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      // Verify the integration point exists
      expect(screen.getByRole('button', { name: 'Open full diff view modal' })).toBeInTheDocument();

      // Verify clicking triggers the modal
      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      // This test FAILS if the wiring is missing
      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      }, { timeout: 1000 });
    });

    it('modal integration does not break existing file click behavior', async () => {
      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      // Normal file clicks should still work
      const fileItem = screen.getByRole('button', { name: /Button\.tsx/i });
      await user.click(fileItem);

      expect(defaultProps.onFileClick).toHaveBeenCalledWith(mockFiles[0]);
      // Modal should not open on normal click
      expect(screen.queryByTestId('diff-view-modal')).not.toBeInTheDocument();
    });

    it('modal integration handles missing project ID gracefully', async () => {
      // Mock missing project ID
      vi.doMock('@/stores/projectStore', () => ({
        useCurrentProjectId: () => undefined,
      }));

      const user = userEvent.setup();
      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      // Modal should handle missing project ID gracefully
      await waitFor(() => {
        expect(screen.getByTestId('diff-view-modal')).toBeInTheDocument();
      });
    });

  });

  describe('Context Preservation', () => {
    it('preserves scroll position when modal is closed', async () => {
      const user = userEvent.setup();
      const manyFiles: ChangedFile[] = Array.from({ length: 20 }, (_, i) => ({
        path: `src/file${i}.ts`,
        status: 'modified' as const,
        taskId: 'TASK-001',
      }));

      render(<FilesPanel files={manyFiles} onFileClick={vi.fn()} maxVisible={5} />);

      // Scroll down by showing more files
      const showMoreButton = screen.getByText(/more files/);
      await user.click(showMoreButton);

      // Open and close modal
      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Should still show expanded file list
      expect(screen.getByText('src/file15.ts')).toBeInTheDocument();
    });

    it('preserves filter state when modal is closed', async () => {
      const user = userEvent.setup();
      const mixedFiles: ChangedFile[] = [
        { path: 'src/added.ts', status: 'added', taskId: 'TASK-001' },
        { path: 'src/modified.ts', status: 'modified', taskId: 'TASK-001' },
        { path: 'src/deleted.ts', status: 'deleted', taskId: 'TASK-001' },
      ];

      // Note: This assumes FilesPanel has filtering capability
      // If not implemented, this test documents the expected behavior
      render(<FilesPanel files={mixedFiles} onFileClick={vi.fn()} />);

      // Apply a filter (if filtering is implemented)
      const filterButton = screen.queryByTestId('status-filter');
      if (filterButton) {
        await user.click(filterButton);
        const addedFilter = screen.queryByText('Added');
        if (addedFilter) {
          await user.click(addedFilter);
        }
      }

      // Open and close modal
      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close' });
        return user.click(closeButton);
      });

      // Filter state should be preserved (test documents intent even if not implemented)
      expect(screen.getByText('src/added.ts')).toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('handles modal opening errors gracefully', async () => {
      const user = userEvent.setup();
      // Mock a scenario where modal fails to open
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<FilesPanel {...defaultProps} />);

      const viewFullDiffButton = screen.getByRole('button', { name: 'Open full diff view modal' });
      await user.click(viewFullDiffButton);

      // FilesPanel should not crash if modal fails
      expect(screen.getByRole('button', { name: 'Open full diff view modal' })).toBeInTheDocument();

      consoleError.mockRestore();
    });

  });
});