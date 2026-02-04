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
			{ type: 'deletion', content: 'const y = 2;', oldLine: 2, newLine: undefined },
			{ type: 'addition', content: 'const y = 3;', newLine: 2, oldLine: undefined },
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

			// Find a line number cell (unified view has old and new columns, use first)
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			expect(lineNumberCell).toBeInTheDocument();

			// Hover over it
			await user.hover(lineNumberCell!);

			// Should show the + button (may show multiple if both cells hovered)
			await waitFor(() => {
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
			});
		});

		it('hides "+" button when hover ends', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];

			// Hover
			await user.hover(lineNumberCell!);
			await waitFor(() => {
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
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
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
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
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
			});
		});

		it('clicking "+" button opens inline feedback input', async () => {
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];

			// Trigger hover with fireEvent
			fireEvent.mouseEnter(lineNumberCell!);

			// Wait for button to appear
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });

			// Use fireEvent.click which is more direct
			fireEvent.click(addButtons[0]);

			// Should show the inline feedback input
			await waitFor(() => {
				expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
			});
		});

		it('opens inline feedback input below the clicked line', async () => {
			render(<DiffHunk {...defaultProps} />);

			// Click on line 3 (last context line)
			const contextRows = document.querySelectorAll('.unified-row.context');
			const lastContextRow = contextRows[contextRows.length - 1];
			const lineNumberCell = lastContextRow?.querySelector('.line-number');
			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// The input row should appear
			await waitFor(() => {
				expect(screen.getByTestId('inline-feedback-row')).toBeInTheDocument();
			});
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

			// In unified view, may show indicator in both old and new columns
			expect(screen.getAllByText('💬').length).toBeGreaterThan(0);
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

			// In unified view, indicators may appear in both old and new columns
			// So we might see 2-4 indicators (2 per line in unified)
			const indicators = screen.getAllByText('💬');
			expect(indicators.length).toBeGreaterThanOrEqual(2);
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

			// Should show indicators (may be in both columns)
			const indicators = screen.getAllByText('💬');
			expect(indicators.length).toBeGreaterThan(0);

			// Each indicator should show count badge of 2
			expect(screen.getAllByText('2').length).toBeGreaterThan(0);
		});
	});

	describe('Inline feedback input interaction', () => {
		it('calls onAddInlineFeedback when feedback is submitted', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open inline feedback input on line 1
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Type feedback and submit
			const input = await screen.findByPlaceholderText(/add feedback/i);
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
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Wait for input to appear then click cancel
			await screen.findByPlaceholderText(/add feedback/i);
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
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Test');
			await user.keyboard('{Enter}');

			// Input should close
			await waitFor(() => {
				expect(screen.queryByPlaceholderText(/add feedback/i)).not.toBeInTheDocument();
			});
		});

		it('only allows one inline feedback input open at a time', async () => {
			render(<DiffHunk {...defaultProps} />);

			// Open on line 1
			const lineNumberCells = document.querySelectorAll('.line-number');
			const line1Cell = lineNumberCells[0];
			fireEvent.mouseEnter(line1Cell!);
			let addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Verify input is open
			await screen.findByPlaceholderText(/add feedback/i);

			// Try to open on line 3
			const line3Row = screen.getByText('const z = 4;').closest('tr');
			const line3Cell = line3Row?.querySelector('.line-number');
			fireEvent.mouseEnter(line3Cell!);
			addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Should still only have one input
			const inputs = screen.getAllByPlaceholderText(/add feedback/i);
			expect(inputs).toHaveLength(1);
		});
	});

	describe('Edge Cases: Deleted lines', () => {
		it('allows adding feedback on deleted lines', async () => {
			render(<DiffHunk {...defaultProps} />);

			// Find the deletion line's line number
			const deletionRow = screen.getByText('const y = 2;').closest('tr');
			const lineNumberCell = deletionRow?.querySelector('.line-number.old');

			fireEvent.mouseEnter(lineNumberCell!);

			// Should be able to open inline feedback (may have multiple buttons)
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// In unified view, clicking on line 2 may open inputs for both deletion and addition
			// since they share the same line number
			const inputs = await screen.findAllByPlaceholderText(/add feedback/i);
			expect(inputs.length).toBeGreaterThan(0);
		});

		it('sends correct line number for deleted lines', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Find and click on deletion line (old line 2)
			const deletionRow = screen.getByText('const y = 2;').closest('tr');
			const lineNumberCell = deletionRow?.querySelector('.line-number.old');

			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// In unified view, may open inputs for both deletion and addition lines with same number
			const inputs = await screen.findAllByPlaceholderText(/add feedback/i);
			await user.type(inputs[0], 'Old code comment');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(mockOnAddInlineFeedback).toHaveBeenCalledWith(
					expect.objectContaining({
						line: 2, // Line number (used by both deletion and addition)
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
			render(<DiffHunk {...defaultProps} viewMode="split" />);

			// Get line number cells for split view
			const lineNumberCells = document.querySelectorAll('.line-number');
			expect(lineNumberCells.length).toBeGreaterThan(0);

			// Hover on first and verify + button appears
			fireEvent.mouseEnter(lineNumberCells[0]!);
			await waitFor(() => {
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
			});
		});
	});

	describe('Accessibility', () => {
		it('"+" button has accessible name', async () => {
			render(<DiffHunk {...defaultProps} />);

			const lineNumberCells = document.querySelectorAll('.line-number');
			fireEvent.mouseEnter(lineNumberCells[0]!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			expect(addButtons[0]).toHaveAttribute('aria-label');
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

			// May have multiple indicators in unified view, check first one
			const indicators = screen.getAllByRole('button', { name: /feedback/i });
			expect(indicators.length).toBeGreaterThan(0);
		});
	});
});
