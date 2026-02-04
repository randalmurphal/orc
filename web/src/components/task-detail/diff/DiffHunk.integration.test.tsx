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
import { render, screen, waitFor } from '@testing-library/react';
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
			const user = userEvent.setup();
			render(<DiffHunk {...defaultProps} />);

			// Hover over line 10 to show the + button
			const lineNumberCell = screen.getByText('10').closest('.line-number');
			await user.hover(lineNumberCell!);

			// Click the + button
			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// This verifies InlineFeedbackInput is rendered by DiffHunk
			// If DiffHunk doesn't import/render it, this will fail
			await waitFor(() => {
				expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
				expect(screen.getByText(/src\/config\/loader\.ts:10/)).toBeInTheDocument();
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
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Type and submit feedback
			const input = screen.getByPlaceholderText(/add feedback/i);
			await user.type(input, 'Consider using a validation library');
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
			expect(screen.getByText('💬')).toBeInTheDocument();
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

			// Should be one indicator with count 2
			const indicators = screen.getAllByText('💬');
			expect(indicators).toHaveLength(1);
			expect(screen.getByText('2')).toBeInTheDocument();
		});
	});

	describe('Hover button and indicator coexistence', () => {
		it('shows both hover button and existing indicator on same line', async () => {
			/**
			 * When a line already has feedback (shows indicator),
			 * hovering should still show the + button to add more feedback.
			 */
			const user = userEvent.setup();
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/config/loader.ts',
					line: 10,
				}),
			];

			render(<DiffHunk {...defaultProps} inlineFeedback={feedback} />);

			// Verify indicator is present
			expect(screen.getByText('💬')).toBeInTheDocument();

			// Hover over line 10
			const lineNumberCell = screen.getByText('10').closest('.line-number');
			await user.hover(lineNumberCell!);

			// Both should be visible
			await waitFor(() => {
				expect(screen.getByText('💬')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
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

			await user.hover(newLineNumber!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

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
					line: 11, // This is a new line (addition)
				}),
			];

			render(<DiffHunk {...defaultProps} viewMode="split" inlineFeedback={feedback} />);

			// Indicator should appear in the new (right) side
			const indicator = screen.getByText('💬');
			const cell = indicator.closest('td');
			expect(cell).toHaveClass('new');
		});
	});
});
