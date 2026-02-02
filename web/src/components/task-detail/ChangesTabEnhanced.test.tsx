import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ChangesTabEnhanced } from './ChangesTabEnhanced';
import '@testing-library/jest-dom';

// Mock the sub-components
vi.mock('./FilesPanel', () => ({
  FilesPanel: ({ onFileSelect, onViewModeChange }: any) => (
    <div data-testid="files-panel">
      <button data-testid="mock-file-1" onClick={() => onFileSelect('file1.ts')}>
        file1.ts
      </button>
      <button data-testid="view-toggle" onClick={() => onViewModeChange('tree')}>
        Tree View
      </button>
    </div>
  ),
}));

vi.mock('./DiffFile', () => ({
  DiffFile: ({ file }: any) => (
    <div data-testid={`diff-file-${file.path}`}>
      Diff for {file.path}
    </div>
  ),
}));

// Mock the task client
const mockTaskClient = {
  getDiff: vi.fn(),
  getFileDiff: vi.fn(),
};

vi.mock('@/lib/client', () => ({
  taskClient: mockTaskClient,
}));

describe('ChangesTabEnhanced', () => {
  const defaultProps = {
    taskId: 'TASK-123',
  };

  beforeEach(() => {
    vi.clearAllMocks();

    mockTaskClient.getDiff.mockResolvedValue({
      diff: {
        base: 'main',
        head: 'feature',
        stats: { filesChanged: 2, additions: 10, deletions: 5 },
        files: [
          {
            path: 'src/app.ts',
            status: 'modified',
            additions: 8,
            deletions: 3,
            binary: false,
            syntax: 'typescript',
            hunks: [],
          },
          {
            path: 'test/app.test.ts',
            status: 'added',
            additions: 2,
            deletions: 0,
            binary: false,
            syntax: 'typescript',
            hunks: [],
          },
        ],
      },
    });
  });

  describe('Enhanced Changes Tab Layout', () => {
    it('should render enhanced changes tab with new file panel', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toBeInTheDocument();
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });
    });

    it('should maintain backward compatibility with existing diff features', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        // Should still have classic diff functionality
        expect(screen.getByTestId('diff-viewer-section')).toBeInTheDocument();
        expect(screen.getByTestId('classic-diff-toggle')).toBeInTheDocument();
      });
    });

    it('should allow switching between enhanced and classic views', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const classicToggle = screen.getByTestId('classic-diff-toggle');
        fireEvent.click(classicToggle);

        expect(screen.getByTestId('classic-diff-view')).toBeInTheDocument();
        expect(screen.queryByTestId('files-panel')).not.toBeInTheDocument();
      });
    });
  });

  describe('File Selection Integration', () => {
    it('should update diff viewer when file is selected from file panel', async () => {
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: {
          path: 'file1.ts',
          hunks: [{ lines: [{ content: 'test content' }] }],
        },
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const fileButton = screen.getByTestId('mock-file-1');
        fireEvent.click(fileButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-file-file1.ts')).toBeInTheDocument();
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith(
          expect.objectContaining({
            filePath: 'file1.ts',
          })
        );
      });
    });

    it('should highlight selected file in both panels', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const fileButton = screen.getByTestId('mock-file-1');
        fireEvent.click(fileButton);

        expect(screen.getByTestId('selected-file-indicator')).toBeInTheDocument();
      });
    });

    it('should sync file expansion state between panels', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const expandButton = screen.getByTestId('expand-file-file1.ts');
        fireEvent.click(expandButton);

        expect(screen.getByTestId('file-expanded-file1.ts')).toBeInTheDocument();
      });
    });
  });

  describe('View Mode Synchronization', () => {
    it('should sync view mode changes between file panel and diff viewer', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const viewToggle = screen.getByTestId('view-toggle');
        fireEvent.click(viewToggle);

        // Both panels should update their view mode
        expect(screen.getByTestId('tree-view-active')).toBeInTheDocument();
        expect(screen.getByTestId('diff-tree-mode')).toBeInTheDocument();
      });
    });

    it('should maintain separate split/unified diff mode', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const diffModeToggle = screen.getByTestId('diff-mode-toggle');
        fireEvent.click(diffModeToggle);

        expect(screen.getByTestId('unified-diff-mode')).toBeInTheDocument();
      });
    });
  });

  describe('Performance and UX Enhancements', () => {
    it('should lazy load file diffs only when needed', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        // Initially, should not have loaded individual file diffs
        expect(mockTaskClient.getFileDiff).not.toHaveBeenCalled();

        // Only overview should be loaded
        expect(mockTaskClient.getDiff).toHaveBeenCalledTimes(1);
      });
    });

    it('should cache file diff results to avoid reloading', async () => {
      mockTaskClient.getFileDiff.mockResolvedValue({
        file: { path: 'file1.ts', hunks: [] },
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        // Select file twice
        const fileButton = screen.getByTestId('mock-file-1');
        fireEvent.click(fileButton);
      });

      await waitFor(() => {
        fireEvent.click(screen.getByTestId('mock-file-1'));
      });

      // Should only call API once due to caching
      expect(mockTaskClient.getFileDiff).toHaveBeenCalledTimes(1);
    });

    it('should provide smooth transitions between view states', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const panel = screen.getByTestId('files-panel');

        // Should have transition classes
        expect(panel).toHaveClass('transition-all');

        // Transition should be smooth when switching
        fireEvent.click(screen.getByTestId('view-toggle'));
        expect(panel).toHaveClass('transitioning');
      });
    });
  });

  describe('Accessibility Enhancements', () => {
    it('should maintain focus management between panels', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const fileButton = screen.getByTestId('mock-file-1');
        fileButton.focus();

        fireEvent.keyDown(fileButton, { key: 'Enter' });

        // Focus should move to diff content
        expect(screen.getByTestId('diff-file-file1.ts')).toHaveFocus();
      });
    });

    it('should provide proper ARIA labels for enhanced components', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveAttribute(
          'aria-label',
          'Enhanced file changes view'
        );

        expect(screen.getByTestId('files-panel')).toHaveAttribute(
          'role',
          'complementary'
        );
      });
    });

    it('should support keyboard shortcuts for common actions', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        // 'f' should focus file list
        fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
        expect(screen.getByTestId('files-panel')).toHaveFocus();

        // 't' should toggle view mode
        fireEvent.keyDown(document, { key: 't', ctrlKey: true });
        expect(screen.getByTestId('tree-view-active')).toBeInTheDocument();
      });
    });
  });

  describe('Responsive Design', () => {
    it('should adapt layout for mobile screens', async () => {
      // Mock window size
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 600, // Mobile width
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveClass('mobile-layout');
        expect(screen.getByTestId('files-panel-collapsed')).toBeInTheDocument();
      });
    });

    it('should provide collapsible panels on tablet screens', async () => {
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 800, // Tablet width
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        const collapseButton = screen.getByTestId('collapse-file-panel');
        fireEvent.click(collapseButton);

        expect(screen.getByTestId('files-panel')).toHaveClass('collapsed');
      });
    });
  });

  describe('Error Boundary Integration', () => {
    it('should handle file panel errors gracefully', async () => {
      // Mock console.error to avoid test noise
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      const ThrowError = () => {
        throw new Error('File panel error');
      };

      // Mock files panel to throw error
      vi.mocked(vi.importMeta.env).VITE_TEST = 'true';

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('file-panel-error-boundary')).toBeInTheDocument();
        expect(screen.getByText('Unable to load file list')).toBeInTheDocument();
      });

      consoleError.mockRestore();
    });

    it('should provide fallback to classic view on enhanced panel errors', async () => {
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<ChangesTabEnhanced {...defaultProps} />);

      // Trigger error in enhanced view
      fireEvent.click(screen.getByTestId('trigger-panel-error'));

      await waitFor(() => {
        expect(screen.getByTestId('fallback-classic-view')).toBeInTheDocument();
        expect(screen.getByText('Switch back to enhanced view')).toBeInTheDocument();
      });

      consoleError.mockRestore();
    });
  });
});