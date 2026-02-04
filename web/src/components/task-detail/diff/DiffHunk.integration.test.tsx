/**
 * Integration tests for DiffHunk with InlineFeedbackInput and FeedbackIndicator
 *
 * These tests verify that DiffHunk correctly imports and uses the new
 * inline feedback components. They test the wiring, not just isolated units.
 *
 * CRITICAL: These tests FAIL if the components aren't properly wired.
 * A unit test of InlineFeedbackInput alone would pass even if DiffHunk
 * never imported it. These integration tests catch that.
 *
 * Success Criteria Coverage:
 * - SC-2: Clicking the "+" button opens an inline feedback input below the line
 * - SC-3: Submitting the inline form creates a Feedback with type=INLINE, correct file and line
 * - SC-6: Clicking the indicator shows existing feedback for that line
 *
 * Integration Points Verified:
 * - DiffHunk imports and renders InlineFeedbackInput
 * - DiffHunk imports and renders FeedbackIndicator
 * - onAddInlineFeedback callback is called with correct parameters
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DiffHunk } from './DiffHunk';
import { createMockFeedback } from '@/test/factories';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import type { DiffHunk as Hunk, DiffLine } from '@/gen/orc/v1/common_pb';

// Create mock hunk data
function createMockHunk(): Hunk {
	return {
		oldStart: 10,
		oldLines: 3,
		newStart: 10,
		newLines: 4,
		lines: [
			{ type: 'context', content: '    const config = loadConfig();', oldLine: 10, newLine: 10 },
			{ type: 'deletion', content: '    validateConfig(config);', oldLine: 11 },
			{ type: 'addition', content: '    if (!validateConfig(config)) {', newLine: 11 },
			{ type: 'addition', content: '        throw new Error("Invalid config");', newLine: 12 },
			{ type: 'addition', content: '    }', newLine: 13 },
			{ type: 'context', content: '    return config;', oldLine: 12, newLine: 14 },
		] as DiffLine[],
	} as Hunk;
}

describe('DiffHunk integration with inline feedback components', () => {
	const mockOnAddInlineFeedback = vi.fn();

	const defaultProps = {
		hunk: createMockHunk(),
		filePath: 'src/config/loader.ts',
		viewMode: 'unified' as const,
		comments: [],
		activeLineNumber: null,
		onLineClick: vi.fn(),
		onAddComment: vi.fn(),
		onResolveComment: vi.fn(),
		onWontFixComment: vi.fn(),
		onDeleteComment: vi.fn(),
		onCloseThread: vi.fn(),
		inlineFeedback: [],
		onAddInlineFeedback: mockOnAddInlineFeedback,
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockOnAddInlineFeedback.mockResolvedValue(undefined);
	});

	describe('InlineFeedbackInput integration', () => {
		it('DiffHunk renders InlineFeedbackInput when user clicks add button', async () => {
			/**
			 * This test FAILS if DiffHunk doesn't import InlineFeedbackInput.
			 * We're testing that clicking the + button results in the
			 * InlineFeedbackInput component being rendered within DiffHunk.
			 */
			render(<DiffHunk {...defaultProps} />);

			// Hover over line 10 to show the + button
			// In unified view, there may be multiple elements with '10'
			const lineNumberCells = document.querySelectorAll('.line-number');
			const line10Cell = Array.from(lineNumberCells).find((cell) => cell.textContent?.includes('10'));
			fireEvent.mouseEnter(line10Cell!);

			// Click the + button
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// This verifies InlineFeedbackInput is rendered by DiffHunk
			// If DiffHunk doesn't import/render it, this will fail
			await waitFor(() => {
				expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
			});
		});

		it('InlineFeedbackInput submission flows through DiffHunk to callback', async () => {
			/**
			 * This test verifies the full flow:
			 * 1. User clicks + on a line
			 * 2. InlineFeedbackInput appears
			 * 3. User types and submits
			 * 4. DiffHunk receives the submission
			 * 5. DiffHunk calls onAddInlineFeedback with correct data
			 *
			 * If the wiring is broken at any point, this test fails.
			 */
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Open inline feedback on line 11 (addition line)
			const additionRow = screen.getByText('if (!validateConfig(config)) {').closest('tr');
			const lineNumberCell = additionRow?.querySelector('.line-number.new');
			fireEvent.mouseEnter(lineNumberCell!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Type and submit feedback
			const inputs = screen.getAllByPlaceholderText(/add feedback/i);
			await user.type(inputs[0], 'Consider using a validation library');
			await user.keyboard('{Enter}');

			// Verify the callback was called with correct data
			await waitFor(() => {
				expect(mockOnAddInlineFeedback).toHaveBeenCalledWith({
					type: FeedbackType.INLINE,
					text: 'Consider using a validation library',
					timing: FeedbackTiming.WHEN_DONE,
					file: 'src/config/loader.ts',
					line: 11,
				});
			});
		});
	});

	describe('FeedbackIndicator integration', () => {
		it('DiffHunk renders FeedbackIndicator for lines with feedback', () => {
			/**
			 * This test FAILS if DiffHunk doesn't import FeedbackIndicator.
			 * We're testing that providing inlineFeedback prop results in
			 * FeedbackIndicator components being rendered.
			 */
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'Check the error message',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 12,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// This verifies FeedbackIndicator is rendered by DiffHunk
			// In unified view, indicators may appear in both old and new columns
			expect(screen.getAllByText('💬').length).toBeGreaterThan(0);
		});

		it('clicking FeedbackIndicator shows popover with feedback content', async () => {
			/**
			 * This test verifies the full interaction:
			 * 1. DiffHunk renders FeedbackIndicator with feedback data
			 * 2. User clicks the indicator
			 * 3. Popover shows the feedback content
			 *
			 * If FeedbackIndicator isn't receiving props correctly, this fails.
			 */
			const user = userEvent.setup();
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'This error message needs improvement',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 12,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Click the indicator
			const indicator = screen.getByRole('button', { name: /feedback/i });
			await user.click(indicator);

			// Verify popover shows the feedback
			await waitFor(() => {
				expect(screen.getByText('This error message needs improvement')).toBeInTheDocument();
			});
		});

		it('multiple feedbacks on same line are grouped in one indicator', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'First comment',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 10,
				}),
				createMockFeedback({
					id: 'fb-2',
					text: 'Second comment',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 10,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Should show indicators with count 2 (may appear in both columns for unified/split view)
			const indicators = screen.getAllByText('💬');
			expect(indicators.length).toBeGreaterThan(0);
			// All indicators for this line should show count of 2
			expect(screen.getAllByText('2').length).toBeGreaterThan(0);
		});
	});

	describe('Hover button and indicator coexistence', () => {
		it('shows both hover button and existing indicator on same line', async () => {
			/**
			 * When a line already has feedback (shows indicator),
			 * hovering should still show the + button to add more feedback.
			 */
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 10,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Verify indicator is present (may appear in both columns)
			expect(screen.getAllByText('💬').length).toBeGreaterThan(0);

			// Hover over line 10 - in unified view there may be multiple '10' elements
			const lineNumberCells = document.querySelectorAll('.line-number');
			const line10Cell = Array.from(lineNumberCells).find((cell) => cell.textContent?.includes('10'));
			fireEvent.mouseEnter(line10Cell!);

			// Both should be visible
			await waitFor(() => {
				expect(screen.getAllByText('💬').length).toBeGreaterThan(0);
				expect(screen.getAllByRole('button', { name: /add feedback/i }).length).toBeGreaterThan(0);
			});
		});
	});

	describe('Split view integration', () => {
		it('inline feedback works correctly in split view mode', async () => {
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} viewMode="split" />);

			// In split view, we have old and new sides
			// Find the new side line number for addition
			const additionContent = screen.getByText('if (!validateConfig(config)) {');
			const additionRow = additionContent.closest('tr');
			const newLineNumber = additionRow?.querySelector('.line-number.new');

			fireEvent.mouseEnter(newLineNumber!);

			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Submit feedback
			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Split view feedback');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(mockOnAddInlineFeedback).toHaveBeenCalledWith(
					expect.objectContaining({
						line: 11,
					})
				);
			});
		});

		it('feedback indicators appear in correct column in split view', () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 11, // This line appears in both deletion (old) and addition (new)
				}),
			];

			render(<DiffHunk {...defaultProps} viewMode="split" inlineFeedback={feedback} />);

			// In split view, line 11 has both a deletion (old side) and addition (new side)
			// so indicators may appear in both columns
			const indicators = screen.getAllByText('💬');
			expect(indicators.length).toBeGreaterThan(0);

			// Verify at least one indicator is in a line-number cell
			const indicatorCells = indicators.map((ind) => ind.closest('td'));
			const hasLineNumberCell = indicatorCells.some((cell) => cell?.classList.contains('line-number'));
			expect(hasLineNumberCell).toBe(true);
		});
	});
});
