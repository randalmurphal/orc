import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { ChangesTabEnhanced } from './ChangesTabEnhanced';
import '@testing-library/jest-dom';

// Cleanup after each test to prevent DOM accumulation
afterEach(() => {
  cleanup();
  // Reset window size
  Object.defineProperty(window, 'innerWidth', {
    writable: true,
    configurable: true,
    value: 1024,
  });
});

// Mock the sub-components
vi.mock('./FilesPanel', () => ({
  FilesPanel: ({ projectId, taskId }: any) => (
    <div data-testid="files-panel" className="transition-all">
      <span data-testid="files-panel-project">{projectId}</span>
      <span data-testid="files-panel-task">{taskId}</span>
    </div>
  ),
}));

vi.mock('./ChangesTab', () => ({
  ChangesTab: ({ taskId }: any) => (
    <div data-testid="classic-changes-tab">
      Classic view for {taskId}
    </div>
  ),
}));

// Mock the useCurrentProjectId hook
vi.mock('@/stores', () => ({
  useCurrentProjectId: vi.fn().mockReturnValue('test-project-id'),
}));

describe('ChangesTabEnhanced', () => {
  const defaultProps = {
    taskId: 'TASK-123',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset window size to desktop
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1024,
    });
    window.dispatchEvent(new Event('resize'));
  });

  describe('Enhanced Changes Tab Layout', () => {
    it('should render enhanced changes tab with file panel', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toBeInTheDocument();
      });

      expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      expect(screen.getByTestId('diff-viewer-section')).toBeInTheDocument();
    });

    it('should pass correct props to FilesPanel', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });

      expect(screen.getByTestId('files-panel-project')).toHaveTextContent('test-project-id');
      expect(screen.getByTestId('files-panel-task')).toHaveTextContent('TASK-123');
    });

    it('should have classic view toggle button', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('classic-diff-toggle')).toBeInTheDocument();
      });
    });

    it('should have proper ARIA label', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveAttribute(
          'aria-label',
          'Enhanced file changes view'
        );
      });
    });
  });

  describe('View Mode Toggle', () => {
    it('should switch to classic view when toggle is clicked', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('classic-diff-toggle')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('classic-diff-toggle'));

      await waitFor(() => {
        expect(screen.getByTestId('classic-diff-view')).toBeInTheDocument();
        expect(screen.getByTestId('classic-changes-tab')).toBeInTheDocument();
        expect(screen.queryByTestId('files-panel')).not.toBeInTheDocument();
      });
    });

    it('should switch back to enhanced view from classic view', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('classic-diff-toggle')).toBeInTheDocument();
      });

      // Switch to classic view
      fireEvent.click(screen.getByTestId('classic-diff-toggle'));

      await waitFor(() => {
        expect(screen.getByTestId('enhanced-view-toggle')).toBeInTheDocument();
      });

      // Switch back to enhanced view
      fireEvent.click(screen.getByTestId('enhanced-view-toggle'));

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toBeInTheDocument();
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });
    });
  });

  describe('Keyboard Shortcuts', () => {
    it('should toggle view mode with Ctrl+T', async () => {
      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toBeInTheDocument();
      });

      // Initially tree-view-active is hidden (viewMode is 'list')
      const treeIndicator = screen.getByTestId('tree-view-active');
      expect(treeIndicator).toHaveStyle({ display: 'none' });

      // Press Ctrl+T to toggle to tree mode
      fireEvent.keyDown(document, { key: 't', ctrlKey: true });

      await waitFor(() => {
        expect(treeIndicator).toHaveStyle({ display: 'block' });
      });
    });
  });

  describe('Responsive Design', () => {
    it('should show mobile layout on small screens', async () => {
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 500, // Mobile width (<=600)
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      // Trigger resize event - wrap in act() since it causes state updates
      await act(async () => {
        window.dispatchEvent(new Event('resize'));
      });

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveClass('mobile-layout');
      });

      // Mobile should show collapse button
      expect(screen.getByTestId('files-panel-collapsed')).toBeInTheDocument();
    });

    it('should show tablet layout on medium screens', async () => {
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 700, // Tablet width (600-800)
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await act(async () => {
        window.dispatchEvent(new Event('resize'));
      });

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveClass('tablet-layout');
      });

      expect(screen.getByTestId('collapse-file-panel')).toBeInTheDocument();
    });

    it('should toggle panel collapsed state on mobile', async () => {
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 500,
      });

      render(<ChangesTabEnhanced {...defaultProps} />);

      await act(async () => {
        window.dispatchEvent(new Event('resize'));
      });

      await waitFor(() => {
        expect(screen.getByTestId('files-panel-collapsed')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('files-panel-collapsed'));

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toHaveClass('panel-collapsed');
      });
    });
  });

  describe('Error Handling', () => {
    it('should show error boundary when error is triggered', async () => {
      // Mock console.error to avoid test noise
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('trigger-panel-error')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('trigger-panel-error'));

      await waitFor(() => {
        expect(screen.getByTestId('file-panel-error-boundary')).toBeInTheDocument();
        expect(screen.getByText('Unable to load file list')).toBeInTheDocument();
      });

      consoleError.mockRestore();
    });

    it('should show fallback classic view on error', async () => {
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('trigger-panel-error')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId('trigger-panel-error'));

      await waitFor(() => {
        expect(screen.getByTestId('fallback-classic-view')).toBeInTheDocument();
        expect(screen.getByTestId('classic-changes-tab')).toBeInTheDocument();
      });

      consoleError.mockRestore();
    });

    it('should allow switching back to enhanced view after error', async () => {
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

      render(<ChangesTabEnhanced {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByTestId('trigger-panel-error')).toBeInTheDocument();
      });

      // Trigger error
      fireEvent.click(screen.getByTestId('trigger-panel-error'));

      await waitFor(() => {
        expect(screen.getByText('Switch back to enhanced view')).toBeInTheDocument();
      });

      // Click to switch back
      fireEvent.click(screen.getByText('Switch back to enhanced view'));

      await waitFor(() => {
        expect(screen.getByTestId('changes-tab-enhanced')).toBeInTheDocument();
        expect(screen.getByTestId('files-panel')).toBeInTheDocument();
      });

      consoleError.mockRestore();
    });
  });

  describe('Loading State', () => {
    it('should show loading state when project is not available', async () => {
      // Override the mock to return null
      const { useCurrentProjectId } = await import('@/stores');
      (useCurrentProjectId as any).mockReturnValueOnce(null);

      render(<ChangesTabEnhanced {...defaultProps} />);

      expect(screen.getByText('Loading project...')).toBeInTheDocument();
    });
  });
});
