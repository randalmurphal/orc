/**
 * DiffViewModal Keyboard Navigation Tests (Lazygit-Style)
 *
 * Tests for lazygit-inspired keyboard shortcuts and navigation:
 * - File list navigation with j/k, arrow keys, Home/End (SC-6, SC-11)
 * - Vim-style navigation patterns
 * - Diff scrolling with Page Up/Down, Ctrl+U/D
 * - File selection with Enter, Space
 * - Modal dismissal with Escape, q
 * - View mode switching with Tab
 * - Quick file jumping with 1-9 number keys
 * - Search and filtering with /
 * - Focus management and accessibility
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

// Mock browser APIs
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
  Element.prototype.scroll = vi.fn();
  global.ResizeObserver = vi.fn().mockImplementation(() => ({
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn(),
  }));
});

import { taskClient } from '@/lib/client';
const mockTaskClient = taskClient as any;

// Extended mock diff data for navigation testing
const mockDiffResult: DiffResult = {
  $typeName: 'orc.v1.DiffResult',
  base: 'main',
  head: 'orc/TASK-123',
  stats: {
    $typeName: 'orc.v1.DiffStats',
    filesChanged: 8,
    additions: 120,
    deletions: 45,
  } as any,
  files: [
    { $typeName: 'orc.v1.FileDiff', path: 'src/components/Button.tsx', status: 'modified', additions: 25, deletions: 5, binary: false, syntax: 'typescript', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'src/utils/api.ts', status: 'added', additions: 30, deletions: 0, binary: false, syntax: 'typescript', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'docs/README.md', status: 'modified', additions: 10, deletions: 8, binary: false, syntax: 'markdown', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'src/types/models.ts', status: 'added', additions: 15, deletions: 0, binary: false, syntax: 'typescript', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'config/settings.json', status: 'modified', additions: 5, deletions: 2, binary: false, syntax: 'json', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'assets/logo.png', status: 'added', additions: 0, deletions: 0, binary: true, syntax: '', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'tests/button.test.tsx', status: 'added', additions: 20, deletions: 0, binary: false, syntax: 'typescript', hunks: [] },
    { $typeName: 'orc.v1.FileDiff', path: 'legacy/old-component.js', status: 'deleted', additions: 0, deletions: 30, binary: false, syntax: 'javascript', hunks: [] },
  ].map((file, index) => ({ ...file, index } as any)),
} as DiffResult;

const mockFileDiff: FileDiff = {
  $typeName: 'orc.v1.FileDiff',
  path: 'src/components/Button.tsx',
  status: 'modified',
  hunks: [
    {
      oldStart: 1, oldLines: 20, newStart: 1, newLines: 25,
      lines: Array.from({ length: 30 }, (_, i) => ({
        type: i % 3 === 0 ? 'addition' : i % 3 === 1 ? 'deletion' : 'context',
        content: `Line ${i + 1} content`,
        oldLine: i % 3 === 0 ? undefined : i + 1,
        newLine: i % 3 === 1 ? undefined : i + 1,
      })),
    },
  ],
} as any;

describe('DiffViewModal - Keyboard Navigation (Lazygit-Style)', () => {
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

  describe('File List Navigation (SC-6, SC-11)', () => {
    it('supports j/k keys for up/down navigation (vim-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' }); // Down
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

    it('supports arrow keys for navigation', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'ArrowDown' });
      });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });
      fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });

      await waitFor(() => {
        const thirdFile = screen.getByTestId('file-item-docs/README.md');
        expect(thirdFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'ArrowUp' });

      await waitFor(() => {
        const secondFile = screen.getByTestId('file-item-src/utils/api.ts');
        expect(secondFile).toHaveFocus();
      });
    });

    it('supports gg for jump to first file (vim-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      // Start at middle file
      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
        fireEvent.keyDown(document.activeElement!, { key: 'j' });
        fireEvent.keyDown(document.activeElement!, { key: 'j' });
      });

      // Type 'gg' quickly to jump to first
      fireEvent.keyDown(document.activeElement!, { key: 'g' });
      fireEvent.keyDown(document.activeElement!, { key: 'g' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('supports G for jump to last file (vim-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'G', shiftKey: true });
      });

      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-legacy/old-component.js');
        expect(lastFile).toHaveFocus();
      });
    });

    it('supports Home/End for first/last file navigation', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'End' });
      });

      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-legacy/old-component.js');
        expect(lastFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Home' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('wraps navigation at boundaries', async () => {
      render(<DiffViewModal {...defaultProps} />);

      // Go to last file
      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'End' });
      });

      // Try to go down from last file - should wrap to first
      fireEvent.keyDown(document.activeElement!, { key: 'j' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });

      // Try to go up from first file - should wrap to last
      fireEvent.keyDown(document.activeElement!, { key: 'k' });

      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-legacy/old-component.js');
        expect(lastFile).toHaveFocus();
      });
    });

    it('supports number keys 1-9 for quick file selection', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '3' });
      });

      await waitFor(() => {
        const thirdFile = screen.getByTestId('file-item-docs/README.md');
        expect(thirdFile).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: '1' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('ignores number keys greater than file count', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '9' }); // Valid key, 9th file doesn't exist but 8 files total
      });

      // Should focus the last (8th) file
      await waitFor(() => {
        const lastFile = screen.getByTestId('file-item-legacy/old-component.js');
        expect(lastFile).toHaveFocus();
      });

      // Try number beyond file count
      fireEvent.keyDown(document.activeElement!, { key: '0' }); // Should be ignored

      // Focus should remain on last file
      expect(document.activeElement).toBe(screen.getByTestId('file-item-legacy/old-component.js'));
    });
  });

  describe('File Selection and Activation', () => {
    it('supports Enter to select and load file diff', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' }); // Navigate to first file
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Enter' });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/components/Button.tsx',
        });
      });

      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toBeInTheDocument();
      });
    });

    it('supports Space to select file without loading diff', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
      });

      fireEvent.keyDown(document.activeElement!, { key: ' ' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveClass('selected');
      });

      // Should not load diff automatically
      expect(mockTaskClient.getFileDiff).not.toHaveBeenCalled();
    });

    it('supports o to open file diff (lazygit-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
      });

      fireEvent.keyDown(document.activeElement!, { key: 'o' });

      await waitFor(() => {
        expect(mockTaskClient.getFileDiff).toHaveBeenCalledWith({
          projectId: 'project-456',
          taskId: 'TASK-123',
          filePath: 'src/components/Button.tsx',
        });
      });
    });

    it('supports v to toggle file selection (visual mode)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
      });

      fireEvent.keyDown(document.activeElement!, { key: 'v' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).toHaveClass('visual-selected');
      });

      // Press v again to deselect
      fireEvent.keyDown(document.activeElement!, { key: 'v' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-item-src/components/Button.tsx');
        expect(firstFile).not.toHaveClass('visual-selected');
      });
    });
  });

  describe('Diff Content Navigation', () => {
    beforeEach(async () => {
      // Load a file diff for testing content navigation
      render(<DiffViewModal {...defaultProps} />);
      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
        fireEvent.keyDown(document.activeElement!, { key: 'Enter' });
      });
      await waitFor(() => {
        expect(screen.getByTestId('diff-content')).toBeInTheDocument();
      });
    });

    it('supports Ctrl+u/d for half-page scrolling (vim-style)', async () => {
      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        fireEvent.keyDown(diffContent, { key: 'd', ctrlKey: true });
      });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledWith({
          top: expect.any(Number),
          behavior: 'smooth'
        });
      });

      fireEvent.keyDown(document.activeElement!, { key: 'u', ctrlKey: true });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledTimes(2);
      });
    });

    it('supports Page Up/Down for page scrolling', async () => {
      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        fireEvent.keyDown(diffContent, { key: 'PageDown' });
      });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledWith({
          top: expect.any(Number),
          behavior: 'smooth'
        });
      });

      fireEvent.keyDown(document.activeElement!, { key: 'PageUp' });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledTimes(2);
      });
    });

    it('supports h/l for horizontal scrolling (vim-style)', async () => {
      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        fireEvent.keyDown(diffContent, { key: 'l' });
      });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledWith({
          left: expect.any(Number),
          behavior: 'smooth'
        });
      });

      fireEvent.keyDown(document.activeElement!, { key: 'h' });

      await waitFor(() => {
        expect(Element.prototype.scroll).toHaveBeenCalledTimes(2);
      });
    });

    it('supports n/N for next/previous hunk navigation', async () => {
      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        fireEvent.keyDown(diffContent, { key: 'n' });
      });

      await waitFor(() => {
        const nextHunk = screen.getByTestId('diff-hunk-1');
        expect(nextHunk.scrollIntoView).toHaveBeenCalled();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'N', shiftKey: true });

      await waitFor(() => {
        const prevHunk = screen.getByTestId('diff-hunk-0');
        expect(prevHunk.scrollIntoView).toHaveBeenCalled();
      });
    });
  });

  describe('View Mode and Display Controls', () => {
    it('supports Tab to switch between split/unified view', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'Tab' });
      });

      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        expect(unifiedButton).toHaveClass('active');
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Tab' });

      await waitFor(() => {
        const splitButton = screen.getByRole('button', { name: 'Split' });
        expect(splitButton).toHaveClass('active');
      });
    });

    it('supports t to toggle view mode', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 't' });
      });

      await waitFor(() => {
        const unifiedButton = screen.getByRole('button', { name: 'Unified' });
        expect(unifiedButton).toHaveClass('active');
      });
    });

    it('supports w to toggle word wrap (when implemented)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'w' });
      });

      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        expect(diffContent).toHaveClass('word-wrap');
      });
    });

    it('supports s to toggle syntax highlighting', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 's' });
      });

      await waitFor(() => {
        const diffContent = screen.getByTestId('diff-content');
        expect(diffContent).toHaveClass('no-syntax-highlight');
      });
    });
  });

  describe('Search and Filtering', () => {
    it('supports / to open search mode', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '/' });
      });

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search-input');
        expect(searchInput).toBeVisible();
        expect(searchInput).toHaveFocus();
      });
    });

    it('supports Escape to close search mode', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '/' });
      });

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search-input');
        fireEvent.keyDown(searchInput, { key: 'Escape' });
      });

      await waitFor(() => {
        expect(screen.queryByTestId('file-search-input')).not.toBeVisible();
      });
    });

    it('supports Enter to apply search filter', async () => {
      const user = userEvent.setup();
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '/' });
      });

      await waitFor(() => {
        const searchInput = screen.getByTestId('file-search-input');
        return user.type(searchInput, 'Button');
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Enter' });

      await waitFor(() => {
        expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
        expect(screen.queryByText('src/utils/api.ts')).not.toBeInTheDocument();
      });
    });

    it('supports f to filter by file status', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'f' });
      });

      await waitFor(() => {
        const statusFilter = screen.getByTestId('status-filter-menu');
        expect(statusFilter).toBeVisible();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'a' }); // Filter by added files

      await waitFor(() => {
        expect(screen.getByText('src/utils/api.ts')).toBeInTheDocument();
        expect(screen.queryByText('src/components/Button.tsx')).not.toBeInTheDocument();
      });
    });
  });

  describe('Modal Dismissal and Navigation', () => {
    it('supports Escape to close modal', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'Escape' });
      });

      await waitFor(() => {
        expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('supports q to close modal (vim-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'q' });
      });

      await waitFor(() => {
        expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('supports Ctrl+w to close modal (editor-style)', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'w', ctrlKey: true });
      });

      await waitFor(() => {
        expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('prevents event propagation for handled keys', async () => {
      const parentKeyHandler = vi.fn();
      render(
        <div onKeyDown={parentKeyHandler}>
          <DiffViewModal {...defaultProps} />
        </div>
      );

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
      });

      // Parent should not receive the 'j' key event
      expect(parentKeyHandler).not.toHaveBeenCalled();
    });
  });

  describe('Focus Management and Accessibility', () => {
    it('sets initial focus to file list on open', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        expect(fileList).toHaveFocus();
      });
    });

    it('restores focus to trigger element on close', async () => {
      const triggerButton = document.createElement('button');
      triggerButton.textContent = 'Trigger';
      document.body.appendChild(triggerButton);
      triggerButton.focus();

      const { rerender } = render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
      });

      rerender(<DiffViewModal {...defaultProps} open={false} />);

      await waitFor(() => {
        expect(triggerButton).toHaveFocus();
      });

      document.body.removeChild(triggerButton);
    });

    it('traps focus within modal during navigation', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'Tab', shiftKey: true }); // Shift+Tab from beginning
      });

      // Should focus on the close button (last focusable element)
      await waitFor(() => {
        const closeButton = screen.getByRole('button', { name: 'Close diff view' });
        expect(closeButton).toHaveFocus();
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Tab' }); // Tab forward

      // Should wrap to file list (first focusable element)
      await waitFor(() => {
        const fileList = screen.getByTestId('diff-modal-file-list');
        expect(fileList).toHaveFocus();
      });
    });

    it('announces file selection changes to screen readers', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'j' });
      });

      await waitFor(() => {
        const announcer = screen.getByTestId('sr-announcer');
        expect(announcer).toHaveTextContent('Selected file: src/components/Button.tsx, Modified');
      });

      fireEvent.keyDown(document.activeElement!, { key: 'j' });

      await waitFor(() => {
        const announcer = screen.getByTestId('sr-announcer');
        expect(announcer).toHaveTextContent('Selected file: src/utils/api.ts, Added');
      });
    });

    it('announces navigation state changes', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: 'End' });
      });

      await waitFor(() => {
        const announcer = screen.getByTestId('sr-announcer');
        expect(announcer).toHaveTextContent('Last file: legacy/old-component.js, Deleted');
      });

      fireEvent.keyDown(document.activeElement!, { key: 'Home' });

      await waitFor(() => {
        const announcer = screen.getByTestId('sr-announcer');
        expect(announcer).toHaveTextContent('First file: src/components/Button.tsx, Modified');
      });
    });

    it('provides keyboard shortcut help with ?', async () => {
      render(<DiffViewModal {...defaultProps} />);

      await waitFor(() => {
        const modal = screen.getByRole('dialog');
        fireEvent.keyDown(modal, { key: '?', shiftKey: true });
      });

      await waitFor(() => {
        const helpDialog = screen.getByTestId('keyboard-shortcuts-help');
        expect(helpDialog).toBeVisible();
        expect(screen.getByText('Keyboard Shortcuts')).toBeInTheDocument();
        expect(screen.getByText('j/k - Navigate files')).toBeInTheDocument();
        expect(screen.getByText('Enter - Select file')).toBeInTheDocument();
        expect(screen.getByText('Escape/q - Close')).toBeInTheDocument();
      });
    });
  });
});