/**
 * TDD Tests for DiffViewModal Component
 *
 * Tests for TASK-774: Restore test coverage for components with deleted tests
 *
 * Success Criteria Coverage:
 * - SC-1: Displays modal with file list and diff statistics
 * - SC-2: Supports keyboard navigation (j/k, arrow keys) between files
 * - SC-3: Toggles between split and unified view modes
 * - SC-4: Shows loading state during diff fetch
 * - SC-5: Handles error states gracefully with retry option
 * - SC-6: Closes on Escape key or q key
 * - SC-7: Supports search/filter functionality
 * - SC-8: Handles file selection via click and keyboard
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { create } from '@bufbuild/protobuf';
import { DiffResultSchema, FileDiffSchema, DiffStatsSchema, DiffHunkSchema } from '@/gen/orc/v1/common_pb';
import type { DiffResult, FileDiff, DiffStats, DiffHunk } from '@/gen/orc/v1/common_pb';

// Mock the task client
const mockGetDiff = vi.fn();
const mockGetFileDiff = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getDiff: (...args: unknown[]) => mockGetDiff(...args),
		getFileDiff: (...args: unknown[]) => mockGetFileDiff(...args),
	},
}));

// Import after mocks are set up
import { DiffViewModal } from './DiffViewModal';

/** Create a mock DiffHunk */
function createMockDiffHunk(overrides: Partial<DiffHunk> = {}): DiffHunk {
	const base = create(DiffHunkSchema, {
		oldStart: 1,
		oldLines: 5,
		newStart: 1,
		newLines: 8,
		lines: [],
	});
	return Object.assign(base, overrides);
}

/** Create a mock DiffStats */
function createMockDiffStats(overrides: Partial<DiffStats> = {}): DiffStats {
	const base = create(DiffStatsSchema, {
		filesChanged: 3,
		additions: 100,
		deletions: 50,
	});
	return Object.assign(base, overrides);
}

/** Create a mock FileDiff */
function createMockFileDiff(overrides: Partial<FileDiff> = {}): FileDiff {
	const base = create(FileDiffSchema, {
		path: 'src/components/Button.tsx',
		status: 'modified',
		additions: 20,
		deletions: 5,
		binary: false,
		syntax: 'typescript',
		hunks: [],
	});
	return Object.assign(base, overrides);
}

/** Create a mock DiffResult */
function createMockDiffResult(overrides: Partial<DiffResult> = {}): DiffResult {
	const base = create(DiffResultSchema, {
		stats: createMockDiffStats(),
		files: [
			createMockFileDiff({ path: 'src/components/Button.tsx', status: 'modified' }),
			createMockFileDiff({ path: 'src/utils/helpers.ts', status: 'added', additions: 50, deletions: 0 }),
			createMockFileDiff({ path: 'tests/old.test.ts', status: 'deleted', additions: 0, deletions: 30 }),
		],
	});
	return Object.assign(base, overrides);
}

describe('TASK-774: DiffViewModal Component', () => {
	const defaultProps = {
		open: true,
		taskId: 'TASK-001',
		projectId: 'project-123',
		onClose: vi.fn(),
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockGetDiff.mockResolvedValue({ diff: createMockDiffResult() });
		mockGetFileDiff.mockResolvedValue({
			file: createMockFileDiff({
				hunks: [createMockDiffHunk()],
			}),
		});
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Displays modal with file list and diff statistics', () => {
		it('renders modal with file list when open', async () => {
			render(<DiffViewModal {...defaultProps} />);

			// Wait for diff to load
			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Should show all files
			expect(screen.getByText('src/utils/helpers.ts')).toBeInTheDocument();
			expect(screen.getByText('tests/old.test.ts')).toBeInTheDocument();
		});

		it('displays diff statistics', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByTestId('diff-stats')).toBeInTheDocument();
			});

			// Should display file count and line changes
			expect(screen.getByText(/3 files/i)).toBeInTheDocument();
			expect(screen.getByText(/\+100/)).toBeInTheDocument();
			expect(screen.getByText(/-50/)).toBeInTheDocument();
		});

		it('shows file status indicators (added, modified, deleted)', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByTestId('file-status-modified')).toBeInTheDocument();
			});

			expect(screen.getByTestId('file-status-added')).toBeInTheDocument();
			expect(screen.getByTestId('file-status-deleted')).toBeInTheDocument();
		});

		it('does not render when open is false', () => {
			render(<DiffViewModal {...defaultProps} open={false} />);

			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});
	});

	describe('SC-2: Supports keyboard navigation (j/k, arrow keys)', () => {
		it('navigates to next file with j key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// First file should be selected initially
			expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveAttribute('data-selected', 'true');

			// Press j to go to next file
			fireEvent.keyDown(document, { key: 'j' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('navigates to previous file with k key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Navigate to second file first
			fireEvent.keyDown(document, { key: 'j' });
			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
			});

			// Press k to go back
			fireEvent.keyDown(document, { key: 'k' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('navigates with arrow keys', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Arrow down
			fireEvent.keyDown(document, { key: 'ArrowDown' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
			});

			// Arrow up
			fireEvent.keyDown(document, { key: 'ArrowUp' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('navigates to first file with Home key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Navigate to last file
			fireEvent.keyDown(document, { key: 'End' });

			// Press Home to go to first
			fireEvent.keyDown(document, { key: 'Home' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('navigates to last file with End key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Press End to go to last file
			fireEvent.keyDown(document, { key: 'End' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-tests/old.test.ts')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('wraps around when navigating past last file', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Navigate to last file
			fireEvent.keyDown(document, { key: 'End' });
			await waitFor(() => {
				expect(screen.getByTestId('file-item-tests/old.test.ts')).toHaveAttribute('data-selected', 'true');
			});

			// Press j one more time to wrap to first
			fireEvent.keyDown(document, { key: 'j' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/components/Button.tsx')).toHaveAttribute('data-selected', 'true');
			});
		});
	});

	describe('SC-3: Toggles between split and unified view modes', () => {
		it('defaults to split view mode', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			expect(screen.getByTestId('view-mode-split')).toHaveAttribute('aria-pressed', 'true');
		});

		it('toggles to unified view with t key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			fireEvent.keyDown(document, { key: 't' });

			await waitFor(() => {
				expect(screen.getByTestId('view-mode-unified')).toHaveAttribute('aria-pressed', 'true');
			});
		});

		it('toggles view mode via Tab key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			fireEvent.keyDown(document, { key: 'Tab' });

			await waitFor(() => {
				expect(screen.getByTestId('view-mode-unified')).toHaveAttribute('aria-pressed', 'true');
			});
		});

		it('toggles view mode via button click', async () => {
			const user = userEvent.setup();
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			await user.click(screen.getByTestId('view-mode-unified'));

			expect(screen.getByTestId('view-mode-unified')).toHaveAttribute('aria-pressed', 'true');
		});
	});

	describe('SC-4: Shows loading state during diff fetch', () => {
		it('displays loading indicator while fetching diff', async () => {
			// Delay the mock to show loading state
			mockGetDiff.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ diff: createMockDiffResult() }), 100))
			);

			render(<DiffViewModal {...defaultProps} />);

			// Should show loading indicator
			expect(screen.getByTestId('diff-loading')).toBeInTheDocument();
			expect(screen.getByText(/loading/i)).toBeInTheDocument();

			// Wait for loading to complete
			await waitFor(() => {
				expect(screen.queryByTestId('diff-loading')).not.toBeInTheDocument();
			});
		});

		it('displays loading indicator when loading individual file diff', async () => {
			mockGetFileDiff.mockImplementation(
				() =>
					new Promise((resolve) =>
						setTimeout(
							() =>
								resolve({
									file: createMockFileDiff({
										hunks: [createMockDiffHunk()],
									}),
								}),
							100
						)
					)
			);

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Should show file diff loading indicator
			expect(screen.getByTestId('file-diff-loading')).toBeInTheDocument();

			await waitFor(() => {
				expect(screen.queryByTestId('file-diff-loading')).not.toBeInTheDocument();
			});
		});
	});

	describe('SC-5: Handles error states gracefully with retry option', () => {
		it('displays error message when diff fetch fails', async () => {
			mockGetDiff.mockRejectedValue(new Error('Network error'));

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByTestId('diff-error')).toBeInTheDocument();
			});

			expect(screen.getByText(/network error/i)).toBeInTheDocument();
		});

		it('displays retry button on error', async () => {
			mockGetDiff.mockRejectedValue(new Error('Network error'));

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries fetch when retry button is clicked', async () => {
			const user = userEvent.setup();
			mockGetDiff.mockRejectedValueOnce(new Error('Network error')).mockResolvedValueOnce({ diff: createMockDiffResult() });

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByTestId('diff-error')).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			expect(mockGetDiff).toHaveBeenCalledTimes(2);
		});

		it('displays error for individual file diff failure', async () => {
			mockGetFileDiff.mockRejectedValue(new Error('Failed to load file'));

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			await waitFor(() => {
				expect(screen.getByTestId('file-diff-error')).toBeInTheDocument();
			});

			expect(screen.getByText(/failed to load file/i)).toBeInTheDocument();
		});
	});

	describe('SC-6: Closes on Escape key or q key', () => {
		it('calls onClose when Escape is pressed', async () => {
			const onClose = vi.fn();
			render(<DiffViewModal {...defaultProps} onClose={onClose} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			fireEvent.keyDown(document, { key: 'Escape' });

			expect(onClose).toHaveBeenCalledTimes(1);
		});

		it('calls onClose when q is pressed', async () => {
			const onClose = vi.fn();
			render(<DiffViewModal {...defaultProps} onClose={onClose} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			fireEvent.keyDown(document, { key: 'q' });

			expect(onClose).toHaveBeenCalledTimes(1);
		});

		it('calls onClose when close button is clicked', async () => {
			const user = userEvent.setup();
			const onClose = vi.fn();
			render(<DiffViewModal {...defaultProps} onClose={onClose} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /close/i }));

			expect(onClose).toHaveBeenCalledTimes(1);
		});
	});

	describe('SC-7: Supports search/filter functionality', () => {
		it('opens search with / key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			fireEvent.keyDown(document, { key: '/' });

			await waitFor(() => {
				expect(screen.getByTestId('search-input')).toBeInTheDocument();
			});
		});

		it('filters files based on search query', async () => {
			const user = userEvent.setup();
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Open search
			fireEvent.keyDown(document, { key: '/' });

			await waitFor(() => {
				expect(screen.getByTestId('search-input')).toBeInTheDocument();
			});

			// Type search query
			await user.type(screen.getByTestId('search-input'), 'Button');

			await waitFor(() => {
				// Only Button.tsx should be visible
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
				expect(screen.queryByText('src/utils/helpers.ts')).not.toBeInTheDocument();
				expect(screen.queryByText('tests/old.test.ts')).not.toBeInTheDocument();
			});
		});

		it('cycles through status filter with f key', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Press f to filter by 'added'
			fireEvent.keyDown(document, { key: 'f' });

			await waitFor(() => {
				// Only added file should be visible
				expect(screen.getByText('src/utils/helpers.ts')).toBeInTheDocument();
				expect(screen.queryByText('src/components/Button.tsx')).not.toBeInTheDocument();
			});

			// Press f again to filter by 'modified'
			fireEvent.keyDown(document, { key: 'f' });

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
				expect(screen.queryByText('src/utils/helpers.ts')).not.toBeInTheDocument();
			});
		});

		it('clears search with Escape', async () => {
			const user = userEvent.setup();
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Open search and type
			fireEvent.keyDown(document, { key: '/' });
			await user.type(screen.getByTestId('search-input'), 'Button');

			// Press Escape to clear
			fireEvent.keyDown(screen.getByTestId('search-input'), { key: 'Escape' });

			await waitFor(() => {
				// All files should be visible again
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
				expect(screen.getByText('src/utils/helpers.ts')).toBeInTheDocument();
				expect(screen.getByText('tests/old.test.ts')).toBeInTheDocument();
			});
		});
	});

	describe('SC-8: Handles file selection via click and keyboard', () => {
		it('selects file when clicked', async () => {
			const user = userEvent.setup();
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			await user.click(screen.getByTestId('file-item-src/utils/helpers.ts'));

			expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
		});

		it('loads file diff when Enter is pressed', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Navigate to second file
			fireEvent.keyDown(document, { key: 'j' });

			// Press Enter to load diff
			fireEvent.keyDown(document, { key: 'Enter' });

			await waitFor(() => {
				expect(mockGetFileDiff).toHaveBeenCalledWith(
					expect.objectContaining({
						filePath: 'src/utils/helpers.ts',
					})
				);
			});
		});

		it('selects file by number key (1-9)', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Press 2 to select second file
			fireEvent.keyDown(document, { key: '2' });

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
			});
		});

		it('navigates to pre-selected file from prop', async () => {
			render(<DiffViewModal {...defaultProps} selectedFile="src/utils/helpers.ts" />);

			await waitFor(() => {
				expect(screen.getByTestId('file-item-src/utils/helpers.ts')).toHaveAttribute('data-selected', 'true');
			});
		});
	});

	describe('Edge Cases', () => {
		it('handles empty diff result', async () => {
			mockGetDiff.mockResolvedValue({
				diff: createMockDiffResult({ files: [], stats: createMockDiffStats({ filesChanged: 0 }) }),
			});

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByTestId('diff-empty')).toBeInTheDocument();
			});

			expect(screen.getByText(/no changes/i)).toBeInTheDocument();
		});

		it('handles binary files gracefully', async () => {
			mockGetDiff.mockResolvedValue({
				diff: createMockDiffResult({
					files: [createMockFileDiff({ path: 'assets/logo.png', binary: true })],
				}),
			});

			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('assets/logo.png')).toBeInTheDocument();
			});

			expect(screen.getByText(/binary file/i)).toBeInTheDocument();
			// Should not attempt to load diff for binary file
			expect(mockGetFileDiff).not.toHaveBeenCalled();
		});

		it('caches file diffs to avoid redundant fetches', async () => {
			render(<DiffViewModal {...defaultProps} />);

			await waitFor(() => {
				expect(screen.getByText('src/components/Button.tsx')).toBeInTheDocument();
			});

			// Wait for first file diff to load
			await waitFor(() => {
				expect(mockGetFileDiff).toHaveBeenCalledTimes(1);
			});

			// Navigate away and back
			fireEvent.keyDown(document, { key: 'j' });
			await waitFor(() => {
				expect(mockGetFileDiff).toHaveBeenCalledTimes(2);
			});

			fireEvent.keyDown(document, { key: 'k' });

			// Should not make another API call (cached)
			await waitFor(() => {
				expect(mockGetFileDiff).toHaveBeenCalledTimes(2);
			});
		});
	});
});
