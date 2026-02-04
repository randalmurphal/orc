/**
 * Tests for DiffHunk inline feedback functionality
 *
 * TDD tests for the inline feedback features in DiffHunk:
 * - Hover "+" button on line gutter
 * - Feedback indicators on lines with existing feedback
 * - Opening inline feedback input
 *
 * Success Criteria Coverage:
 * - SC-1: Hovering over a diff line gutter reveals a "+" button
 * - SC-5: Lines with existing inline feedback show a comment indicator icon (💬)
 *
 * Edge Cases:
 * - Line deleted in diff (deletion type) → Can still add feedback on deleted lines
 * - Binary file → No "+" button shown (no line-level content)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DiffHunk } from './DiffHunk';
import { createMockFeedback } from '@/test/factories';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import type { DiffHunk as Hunk, DiffLine } from '@/gen/orc/v1/common_pb';

// Mock feedback client
vi.mock('@/lib/client', () => ({
	feedbackClient: {
		addFeedback: vi.fn(),
	},
}));

// Create mock hunk data
function createMockHunk(lines: Partial<DiffLine>[] = []): Hunk {
	return {
		oldStart: 1,
		oldLines: lines.length,
		newStart: 1,
		newLines: lines.length,
		lines: lines.map((line, index) => ({
			type: 'context' as const,
			content: `line ${index + 1}`,
			oldLine: index + 1,
			newLine: index + 1,
			...line,
		})) as DiffLine[],
	} as Hunk;
}

describe('DiffHunk inline feedback', () => {
	const mockOnLineClick = vi.fn();
	const mockOnAddComment = vi.fn();
	const mockOnResolveComment = vi.fn();
	const mockOnWontFixComment = vi.fn();
	const mockOnDeleteComment = vi.fn();
	const mockOnCloseThread = vi.fn();
	const mockOnAddInlineFeedback = vi.fn();

	const defaultProps = {
		hunk: createMockHunk([
			{ type: 'context', content: 'const x = 1;', oldLine: 1, newLine: 1 },
			{ type: 'deletion', content: 'const y = 2;', oldLine: 2 },
			{ type: 'addition', content: 'const y = 3;', newLine: 2 },
			{ type: 'context', content: 'const z = 4;', oldLine: 3, newLine: 3 },
		]),
		filePath: 'src/main.ts',
		viewMode: 'unified' as const,
		comments: [],
		activeLineNumber: null,
		onLineClick: mockOnLineClick,
		onAddComment: mockOnAddComment,
		onResolveComment: mockOnResolveComment,
		onWontFixComment: mockOnWontFixComment,
		onDeleteComment: mockOnDeleteComment,
		onCloseThread: mockOnCloseThread,
		// New props for inline feedback
		inlineFeedback: [],
		onAddInlineFeedback: mockOnAddInlineFeedback,
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockOnAddInlineFeedback.mockResolvedValue(undefined);
	});

	describe('SC-1: Hover "+" button on line gutter', () => {
		it('shows "+" button when hovering over a line number cell', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find a line number cell
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			expect(lineNumberCell).toBeInTheDocument();

			// Hover over it
			await user.hover(lineNumberCell!);

			// Should show the + button
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
			});
		});

		it('hides "+" button when hover ends', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCell = screen.getByText('1').closest('.line-number');

			// Hover
			await user.hover(lineNumberCell!);
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
			});

			// Unhover
			await user.unhover(lineNumberCell!);
			await waitFor(() => {
				expect(screen.queryByRole('button', { name: /add feedback/i })).not.toBeInTheDocument();
			});
		});

		it('shows "+" button on deletion lines', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find the deletion line (line 2 on old side)
			const deletionRow = screen.getByText('const y = 2;').closest('tr');
			const lineNumberCell = deletionRow?.querySelector('.line-number.old');

			await user.hover(lineNumberCell!);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
			});
		});

		it('shows "+" button on addition lines', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find the addition line
			const additionRow = screen.getByText('const y = 3;').closest('tr');
			const lineNumberCell = additionRow?.querySelector('.line-number.new');

			await user.hover(lineNumberCell!);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
			});
		});

		it('clicking "+" button opens inline feedback input', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Should show the inline feedback input
			await waitFor(() => {
				expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
			});
		});

		it('opens inline feedback input below the clicked line', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Click on line 2
			const lineNumberCell = screen.getByText('3').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// The input row should appear after line 2
			const inputRow = screen.getByTestId('inline-feedback-row');
			expect(inputRow).toBeInTheDocument();
		});
	});

	describe('SC-5: Feedback indicators on lines', () => {
		it('shows 💬 indicator on lines with existing feedback', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'Check this logic',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			expect(screen.getByText('💬')).toBeInTheDocument();
		});

		it('indicator appears in the correct line gutter', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'Check this logic',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 3,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// The indicator should be in the same row as line 3
			const lineRow = screen.getByText('const z = 4;').closest('tr');
			const indicator = lineRow?.querySelector('[data-testid="feedback-indicator"]');
			expect(indicator).toBeInTheDocument();
		});

		it('does not show indicator on lines without feedback', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Line 3 should not have an indicator
			const line3Row = screen.getByText('const z = 4;').closest('tr');
			const indicator = line3Row?.querySelector('[data-testid="feedback-indicator"]');
			expect(indicator).not.toBeInTheDocument();
		});

		it('shows multiple indicators for different lines', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
				createMockFeedback({
					id: 'fb-2',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 3,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			const indicators = screen.getAllByText('💬');
			expect(indicators).toHaveLength(2);
		});

		it('groups multiple feedbacks on same line into one indicator', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
				createMockFeedback({
					id: 'fb-2',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Should show only one indicator for line 1
			const indicators = screen.getAllByText('💬');
			expect(indicators).toHaveLength(1);

			// But it should show count badge
			expect(screen.getByText('2')).toBeInTheDocument();
		});
	});

	describe('Inline feedback input interaction', () => {
		it('calls onAddInlineFeedback when feedback is submitted', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open inline feedback input on line 1
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Type feedback and submit
			const input = screen.getByPlaceholderText(/add feedback/i);
			await user.type(input, 'Please check this');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(mockOnAddInlineFeedback).toHaveBeenCalledWith({
					type: FeedbackType.INLINE,
					text: 'Please check this',
					timing: FeedbackTiming.WHEN_DONE,
					file: 'src/main.ts',
					line: 1,
				});
			});
		});

		it('closes inline feedback input when Cancel is clicked', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open inline feedback input
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Click cancel
			await user.click(screen.getByRole('button', { name: /cancel/i }));

			// Input should be gone
			await waitFor(() => {
				expect(screen.queryByPlaceholderText(/add feedback/i)).not.toBeInTheDocument();
			});
		});

		it('closes inline feedback input after successful submission', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open and submit
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Test');
			await user.keyboard('{Enter}');

			// Input should close
			await waitFor(() => {
				expect(screen.queryByPlaceholderText(/add feedback/i)).not.toBeInTheDocument();
			});
		});

		it('only allows one inline feedback input open at a time', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open on line 1
			const line1Cell = screen.getByText('1').closest('.line-number');
			await user.hover(line1Cell!);
			let addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Verify input is open
			expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();

			// Try to open on line 3
			const line3Row = screen.getByText('const z = 4;').closest('tr');
			const line3Cell = line3Row?.querySelector('.line-number');
			await user.hover(line3Cell!);
			addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Should still only have one input
			const inputs = screen.getAllByPlaceholderText(/add feedback/i);
			expect(inputs).toHaveLength(1);
		});
	});

	describe('Edge Cases: Deleted lines', () => {
		it('allows adding feedback on deleted lines', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find the deletion line's line number
			const deletionRow = screen.getByText('const y = 2;').closest('tr');
			const lineNumberCell = deletionRow?.querySelector('.line-number.old');

			await user.hover(lineNumberCell!);

			// Should be able to open inline feedback
			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
		});

		it('sends correct line number for deleted lines', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find and click on deletion line (old line 2)
			const deletionRow = screen.getByText('const y = 2;').closest('tr');
			const lineNumberCell = deletionRow?.querySelector('.line-number.old');

			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Old code comment');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(mockOnAddInlineFeedback).toHaveBeenCalledWith(
					expect.objectContaining({
						line: 2, // Old line number
					})
				);
			});
		});
	});

	describe('Edge Cases: Binary files', () => {
		it('does not show "+" button for binary file diff', async () => {
			const binaryHunk = {
				...createMockHunk([]),
				binary: true,
			} as unknown as Hunk;

			render(<DiffHunk {...defaultProps} hunk={binaryHunk} />);

			// Binary files shouldn't show the + button
			const lineNumberCells = screen.queryAllByRole('cell', { name: /line-number/i });
			expect(lineNumberCells).toHaveLength(0);
		});
	});

	describe('Split view mode', () => {
		it('shows "+" button on both old and new side line numbers in split view', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} viewMode="split" />);

			// Get a context line (has both old and new line numbers)
			const line1Cells = screen.getAllByText('1');
			expect(line1Cells.length).toBeGreaterThanOrEqual(1);

			// Hover on each and verify + button appears
			for (const cell of line1Cells) {
				const lineNumberCell = cell.closest('.line-number');
				if (lineNumberCell) {
					await user.hover(lineNumberCell);
					await waitFor(() => {
						expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
					});
					await user.unhover(lineNumberCell);
				}
			}
		});
	});

	describe('Accessibility', () => {
		it('"+" button has accessible name', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			expect(addButton).toHaveAttribute('aria-label');
		});

		it('feedback indicator has accessible name', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			const indicator = screen.getByRole('button', { name: /feedback/i });
			expect(indicator).toBeInTheDocument();
		});
	});
});
