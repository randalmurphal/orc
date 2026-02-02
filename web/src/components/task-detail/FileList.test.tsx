import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FileList } from './FileList';
import type { FileDiff, DiffStats } from '@/gen/orc/v1/common_pb';
import '@testing-library/jest-dom';

// Mock data
const mockStats: DiffStats = {
  $typeName: 'orc.v1.DiffStats',
  filesChanged: 5,
  additions: 45,
  deletions: 12,
} as DiffStats;

const mockFiles: FileDiff[] = [
  {
    $typeName: 'orc.v1.FileDiff',
    path: 'src/components/ui/Button.tsx',
    status: 'modified',
    additions: 15,
    deletions: 3,
    binary: false,
    syntax: 'typescript',
    hunks: [],
    oldPath: undefined,
    loadError: undefined,
  } as FileDiff,
  {
    $typeName: 'orc.v1.FileDiff',
    path: 'src/utils/helpers.ts',
    status: 'added',
    additions: 25,
    deletions: 0,
    binary: false,
    syntax: 'typescript',
    hunks: [],
    oldPath: undefined,
    loadError: undefined,
  } as FileDiff,
  {
    $typeName: 'orc.v1.FileDiff',
    path: 'tests/utils.test.ts',
    status: 'deleted',
    additions: 0,
    deletions: 8,
    binary: false,
    syntax: 'typescript',
    hunks: [],
    oldPath: undefined,
    loadError: undefined,
  } as FileDiff,
  {
    $typeName: 'orc.v1.FileDiff',
    path: 'package.json',
    status: 'modified',
    additions: 2,
    deletions: 1,
    binary: false,
    syntax: 'json',
    hunks: [],
    oldPath: undefined,
    loadError: undefined,
  } as FileDiff,
  {
    $typeName: 'orc.v1.FileDiff',
    path: 'docs/README.md',
    status: 'modified',
    additions: 3,
    deletions: 0,
    binary: false,
    syntax: 'markdown',
    hunks: [],
    oldPath: undefined,
    loadError: undefined,
  } as FileDiff,
];

describe('FileList', () => {
  const defaultProps = {
    files: mockFiles,
    stats: mockStats,
    loading: false,
    onFileSelect: vi.fn(),
    onFileExpand: vi.fn(),
    selectedFile: null,
    expandedFiles: new Set<string>(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('SC-1: Display condensed file list with file names, status, and statistics', () => {
    it('should display all files in the list', () => {
      render(<FileList {...defaultProps} />);

      expect(screen.getByText('src/components/ui/Button.tsx')).toBeInTheDocument();
      expect(screen.getByText('src/utils/helpers.ts')).toBeInTheDocument();
      expect(screen.getByText('tests/utils.test.ts')).toBeInTheDocument();
      expect(screen.getByText('package.json')).toBeInTheDocument();
      expect(screen.getByText('docs/README.md')).toBeInTheDocument();
    });

    it('should show file status icons for each file', () => {
      render(<FileList {...defaultProps} />);

      // Check for status indicators
      expect(screen.getByTestId('file-status-modified')).toBeInTheDocument();
      expect(screen.getByTestId('file-status-added')).toBeInTheDocument();
      expect(screen.getByTestId('file-status-deleted')).toBeInTheDocument();
    });

    it('should display addition and deletion counts for each file', () => {
      render(<FileList {...defaultProps} />);

      // Check for change statistics
      expect(screen.getByText('+15')).toBeInTheDocument(); // Button.tsx
      expect(screen.getByText('-3')).toBeInTheDocument();  // Button.tsx
      expect(screen.getByText('+25')).toBeInTheDocument(); // helpers.ts
      expect(screen.getByText('-8')).toBeInTheDocument();  // utils.test.ts deleted
    });

    it('should show overall diff statistics', () => {
      render(<FileList {...defaultProps} />);

      expect(screen.getByText('5 files changed')).toBeInTheDocument();
      expect(screen.getByText('45 additions')).toBeInTheDocument();
      expect(screen.getByText('12 deletions')).toBeInTheDocument();
    });
  });

  describe('SC-2: Support hierarchical file tree navigation grouped by directory', () => {
    it('should group files by directory in tree structure', () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      // Check for directory nodes
      expect(screen.getByText('src/')).toBeInTheDocument();
      expect(screen.getByText('tests/')).toBeInTheDocument();
      expect(screen.getByText('docs/')).toBeInTheDocument();

      // Check for nested structure
      expect(screen.getByText('components/')).toBeInTheDocument();
      expect(screen.getByText('utils/')).toBeInTheDocument();
    });

    it('should show proper indentation for nested directories', () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const componentDir = screen.getByTestId('directory-src/components');
      const uiDir = screen.getByTestId('directory-src/components/ui');

      expect(componentDir).toHaveStyle({ paddingLeft: '1rem' });
      expect(uiDir).toHaveStyle({ paddingLeft: '2rem' });
    });

    it('should aggregate statistics at directory level', () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const srcDir = screen.getByTestId('directory-src');
      expect(srcDir).toHaveTextContent('+40 -3'); // Sum of src files
    });
  });

  describe('SC-3: Allow collapsing/expanding directories in tree view', () => {
    it('should expand directory when clicked', async () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const srcDirectory = screen.getByTestId('directory-src');
      fireEvent.click(srcDirectory);

      await waitFor(() => {
        expect(screen.getByText('components/')).toBeVisible();
        expect(screen.getByText('utils/')).toBeVisible();
      });
    });

    it('should collapse directory when clicked again', async () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const srcDirectory = screen.getByTestId('directory-src');

      // Expand first
      fireEvent.click(srcDirectory);
      await waitFor(() => {
        expect(screen.getByText('components/')).toBeVisible();
      });

      // Then collapse
      fireEvent.click(srcDirectory);
      await waitFor(() => {
        expect(screen.queryByText('components/')).not.toBeVisible();
      });
    });

    it('should show expand/collapse chevron icons', () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const chevronIcon = screen.getByTestId('chevron-src');
      expect(chevronIcon).toBeInTheDocument();
      expect(chevronIcon).toHaveClass('chevron-right'); // Collapsed by default
    });
  });

  describe('SC-4: Integrate with existing diff viewer when files are selected', () => {
    it('should call onFileSelect when file is clicked', () => {
      const onFileSelect = vi.fn();
      render(<FileList {...defaultProps} onFileSelect={onFileSelect} />);

      const file = screen.getByTestId('file-src/components/ui/Button.tsx');
      fireEvent.click(file);

      expect(onFileSelect).toHaveBeenCalledWith('src/components/ui/Button.tsx');
    });

    it('should highlight selected file', () => {
      render(<FileList {...defaultProps} selectedFile="src/utils/helpers.ts" />);

      const selectedFile = screen.getByTestId('file-src/utils/helpers.ts');
      expect(selectedFile).toHaveClass('selected');
    });

    it('should call onFileExpand for expandable files in list view', () => {
      const onFileExpand = vi.fn();
      render(<FileList {...defaultProps} onFileExpand={onFileExpand} />);

      const expandButton = screen.getByTestId('expand-src/components/ui/Button.tsx');
      fireEvent.click(expandButton);

      expect(onFileExpand).toHaveBeenCalledWith('src/components/ui/Button.tsx');
    });
  });

  describe('SC-5: Maintain keyboard accessibility for navigation', () => {
    it('should support arrow key navigation between files', async () => {
      render(<FileList {...defaultProps} />);

      const fileList = screen.getByRole('tree');
      fileList.focus();

      // Press down arrow
      fireEvent.keyDown(fileList, { key: 'ArrowDown' });

      await waitFor(() => {
        const firstFile = screen.getByTestId('file-src/components/ui/Button.tsx');
        expect(firstFile).toHaveFocus();
      });
    });

    it('should select file with Enter or Space key', async () => {
      const onFileSelect = vi.fn();
      render(<FileList {...defaultProps} onFileSelect={onFileSelect} />);

      const firstFile = screen.getByTestId('file-src/components/ui/Button.tsx');
      firstFile.focus();

      fireEvent.keyDown(firstFile, { key: 'Enter' });

      expect(onFileSelect).toHaveBeenCalledWith('src/components/ui/Button.tsx');
    });

    it('should expand/collapse directories with arrow keys in tree mode', async () => {
      render(<FileList {...defaultProps} viewMode="tree" />);

      const srcDir = screen.getByTestId('directory-src');
      srcDir.focus();

      // Right arrow should expand
      fireEvent.keyDown(srcDir, { key: 'ArrowRight' });

      await waitFor(() => {
        expect(screen.getByText('components/')).toBeVisible();
      });

      // Left arrow should collapse
      fireEvent.keyDown(srcDir, { key: 'ArrowLeft' });

      await waitFor(() => {
        expect(screen.queryByText('components/')).not.toBeVisible();
      });
    });

    it('should have proper ARIA labels and roles', () => {
      render(<FileList {...defaultProps} />);

      expect(screen.getByRole('tree')).toHaveAttribute('aria-label', 'File list');
      expect(screen.getByTestId('file-src/components/ui/Button.tsx')).toHaveAttribute('role', 'treeitem');
    });
  });

  describe('SC-6: Show appropriate loading and error states', () => {
    it('should show loading spinner when loading', () => {
      render(<FileList {...defaultProps} loading={true} />);

      expect(screen.getByTestId('file-list-loading')).toBeInTheDocument();
      expect(screen.getByText('Loading files...')).toBeInTheDocument();
    });

    it('should show empty state when no files', () => {
      render(<FileList {...defaultProps} files={[]} />);

      expect(screen.getByTestId('file-list-empty')).toBeInTheDocument();
      expect(screen.getByText('No files changed')).toBeInTheDocument();
    });

    it('should show error state for files with load errors', () => {
      const filesWithError: FileDiff[] = [
        {
          ...mockFiles[0],
          loadError: 'Failed to load diff for this file',
        },
      ];

      render(<FileList {...defaultProps} files={filesWithError} />);

      expect(screen.getByTestId('file-error-src/components/ui/Button.tsx')).toBeInTheDocument();
      expect(screen.getByText('Failed to load diff for this file')).toBeInTheDocument();
    });
  });

  describe('SC-7: Support filtering files by status', () => {
    it('should filter files by status when filter is applied', () => {
      render(<FileList {...defaultProps} statusFilter="added" />);

      // Should only show added file
      expect(screen.getByText('src/utils/helpers.ts')).toBeInTheDocument();
      expect(screen.queryByText('src/components/ui/Button.tsx')).not.toBeInTheDocument();
      expect(screen.queryByText('tests/utils.test.ts')).not.toBeInTheDocument();
    });

    it('should show filter dropdown with status options', () => {
      render(<FileList {...defaultProps} showFilter={true} />);

      const filterSelect = screen.getByTestId('status-filter');
      expect(filterSelect).toBeInTheDocument();

      fireEvent.click(filterSelect);

      expect(screen.getByText('All files')).toBeInTheDocument();
      expect(screen.getByText('Added')).toBeInTheDocument();
      expect(screen.getByText('Modified')).toBeInTheDocument();
      expect(screen.getByText('Deleted')).toBeInTheDocument();
    });

    it('should update file count in stats when filter is applied', () => {
      render(<FileList {...defaultProps} statusFilter="modified" />);

      expect(screen.getByText('3 files changed')).toBeInTheDocument(); // Only modified files
    });
  });

  describe('Integration Tests', () => {
    it('should switch between list and tree view modes', async () => {
      const { rerender } = render(<FileList {...defaultProps} viewMode="list" />);

      // In list mode, files should be flat
      expect(screen.getByTestId('file-src/components/ui/Button.tsx')).toBeInTheDocument();
      expect(screen.queryByTestId('directory-src')).not.toBeInTheDocument();

      // Switch to tree mode
      rerender(<FileList {...defaultProps} viewMode="tree" />);

      expect(screen.getByTestId('directory-src')).toBeInTheDocument();
    });

    it('should handle binary files appropriately', () => {
      const binaryFile: FileDiff = {
        $typeName: 'orc.v1.FileDiff',
        path: 'assets/logo.png',
        status: 'added',
        additions: 0,
        deletions: 0,
        binary: true,
        syntax: '',
        hunks: [],
        oldPath: undefined,
        loadError: undefined,
      } as FileDiff;

      render(<FileList {...defaultProps} files={[binaryFile]} />);

      const file = screen.getByTestId('file-assets/logo.png');
      expect(file).toHaveTextContent('Binary file');
      expect(screen.queryByText('+0')).not.toBeInTheDocument(); // No line counts for binary
    });
  });
});