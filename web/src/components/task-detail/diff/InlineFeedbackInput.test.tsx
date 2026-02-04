/**
 * Tests for InlineFeedbackInput component
 *
 * TDD tests for the inline feedback input form that appears when users click
 * the "+" button on a diff line gutter.
 *
 * Success Criteria Coverage:
 * - SC-2: Clicking the "+" button opens an inline feedback input below the line
 * - SC-3: Submitting the inline form creates a Feedback with type=INLINE, correct file and line
 * - SC-4: Inline input supports Enter to submit and Escape to cancel
 *
 * Failure Modes:
 * - API call fails when adding feedback → Input remains open, text preserved, toast error
 *
 * Edge Cases:
 * - Empty feedback text → Submit button disabled
 * - Very long feedback text → Allow submission, no truncation
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InlineFeedbackInput } from './InlineFeedbackInput';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';

// Mock toast for error handling
vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

describe('InlineFeedbackInput', () => {
	const defaultProps = {
		filePath: 'src/components/Button.tsx',
		lineNumber: 42,
		onSubmit: vi.fn(),
		onCancel: vi.fn(),
	};

	beforeEach(() => {
		vi.clearAllMocks();
		defaultProps.onSubmit.mockResolvedValue(undefined);
	});

	describe('SC-2: Renders inline feedback input form', () => {
		it('renders text input for feedback', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			expect(screen.getByRole('textbox')).toBeInTheDocument();
			expect(screen.getByPlaceholderText(/add feedback/i)).toBeInTheDocument();
		});

		it('renders Add button', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			expect(screen.getByRole('button', { name: /add/i })).toBeInTheDocument();
		});

		it('renders Cancel button', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
		});

		it('displays the file and line context', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			// Should show the file:line context somewhere in the component
			expect(screen.getByText(/src\/components\/Button\.tsx:42/)).toBeInTheDocument();
		});

		it('focuses the text input on mount', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			expect(screen.getByRole('textbox')).toHaveFocus();
		});
	});

	describe('SC-3: Submitting creates feedback with correct data', () => {
		it('calls onSubmit with type=INLINE, file, and line when Add is clicked', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Use validateSession instead');
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledWith({
					type: FeedbackType.INLINE,
					text: 'Use validateSession instead',
					timing: FeedbackTiming.WHEN_DONE,
					file: 'src/components/Button.tsx',
					line: 42,
				});
			});
		});

		it('closes the input after successful submission', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(defaultProps.onCancel).toHaveBeenCalled();
			});
		});

		it('trims whitespace from feedback text before submission', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), '  feedback with spaces  ');
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledWith(
					expect.objectContaining({
						text: 'feedback with spaces',
					})
				);
			});
		});

		it('defaults timing to WHEN_DONE', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledWith(
					expect.objectContaining({
						timing: FeedbackTiming.WHEN_DONE,
					})
				);
			});
		});
	});

	describe('SC-4: Keyboard shortcuts', () => {
		it('submits on Enter key', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			await user.type(textbox, 'Test feedback');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledWith(
					expect.objectContaining({
						text: 'Test feedback',
					})
				);
			});
		});

		it('cancels on Escape key', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			await user.type(textbox, 'Test feedback');
			await user.keyboard('{Escape}');

			expect(defaultProps.onCancel).toHaveBeenCalled();
			expect(defaultProps.onSubmit).not.toHaveBeenCalled();
		});

		it('does not submit on Enter when text is empty', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			textbox.focus();
			await user.keyboard('{Enter}');

			expect(defaultProps.onSubmit).not.toHaveBeenCalled();
		});

		it('does not submit on Shift+Enter (allows multiline)', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			await user.type(textbox, 'Line 1');
			await user.keyboard('{Shift>}{Enter}{/Shift}');
			await user.type(textbox, 'Line 2');

			// Should not have submitted
			expect(defaultProps.onSubmit).not.toHaveBeenCalled();
		});
	});

	describe('Failure Modes: API error handling', () => {
		it('preserves input text on API failure', async () => {
			const user = userEvent.setup();
			const errorMessage = 'Network error';
			defaultProps.onSubmit.mockRejectedValue(new Error(errorMessage));

			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			await user.type(textbox, 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add/i }));

			// Input should still have the text
			await waitFor(() => {
				expect(textbox).toHaveValue('Test feedback');
			});
		});

		it('keeps input open on API failure', async () => {
			const user = userEvent.setup();
			defaultProps.onSubmit.mockRejectedValue(new Error('API error'));

			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add/i }));

			// onCancel should NOT be called on failure
			await waitFor(() => {
				expect(defaultProps.onCancel).not.toHaveBeenCalled();
			});
		});

		it('shows error message on API failure', async () => {
			const user = userEvent.setup();
			defaultProps.onSubmit.mockRejectedValue(new Error('Network error'));

			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(screen.getByText(/failed to add feedback/i)).toBeInTheDocument();
			});
		});

		it('allows retry after error', async () => {
			const user = userEvent.setup();
			defaultProps.onSubmit.mockRejectedValueOnce(new Error('Network error'));
			defaultProps.onSubmit.mockResolvedValueOnce(undefined);

			render(<InlineFeedbackInput {...defaultProps} />);

			const textbox = screen.getByRole('textbox');
			await user.type(textbox, 'Test feedback');

			// First attempt fails
			await user.click(screen.getByRole('button', { name: /add/i }));
			await waitFor(() => {
				expect(screen.getByText(/failed to add feedback/i)).toBeInTheDocument();
			});

			// Second attempt succeeds
			await user.click(screen.getByRole('button', { name: /add/i }));
			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledTimes(2);
				expect(defaultProps.onCancel).toHaveBeenCalled();
			});
		});
	});

	describe('Edge Cases: Validation', () => {
		it('disables Add button when text is empty', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add/i });
			expect(addButton).toBeDisabled();
		});

		it('enables Add button when text is entered', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add/i });
			expect(addButton).toBeDisabled();

			await user.type(screen.getByRole('textbox'), 'Some text');

			expect(addButton).not.toBeDisabled();
		});

		it('disables Add button when text is only whitespace', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), '   ');

			const addButton = screen.getByRole('button', { name: /add/i });
			expect(addButton).toBeDisabled();
		});
	});

	describe('Edge Cases: Long text', () => {
		it('allows submission of very long feedback text', async () => {
			const user = userEvent.setup();
			const longText = 'A'.repeat(1000);

			render(<InlineFeedbackInput {...defaultProps} />);

			// Use fireEvent for very long text to avoid timeout
			fireEvent.change(screen.getByRole('textbox'), { target: { value: longText } });
			await user.click(screen.getByRole('button', { name: /add/i }));

			await waitFor(() => {
				expect(defaultProps.onSubmit).toHaveBeenCalledWith(
					expect.objectContaining({
						text: longText,
					})
				);
			});
		});
	});

	describe('UI State', () => {
		it('shows loading state while submitting', async () => {
			const user = userEvent.setup();
			// Make the submit slow
			defaultProps.onSubmit.mockImplementation(
				() => new Promise((resolve) => setTimeout(resolve, 100))
			);

			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			const addButton = screen.getByRole('button', { name: /add/i });
			await user.click(addButton);

			// Should show loading state
			expect(addButton).toBeDisabled();
			expect(screen.getByRole('textbox')).toBeDisabled();
		});

		it('Cancel button calls onCancel without submitting', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			await user.type(screen.getByRole('textbox'), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /cancel/i }));

			expect(defaultProps.onCancel).toHaveBeenCalled();
			expect(defaultProps.onSubmit).not.toHaveBeenCalled();
		});
	});

	describe('Accessibility', () => {
		it('has proper ARIA label for the input', () => {
			render(<InlineFeedbackInput {...defaultProps} />);

			expect(screen.getByRole('textbox')).toHaveAttribute('aria-label');
		});

		it('supports keyboard navigation between controls', async () => {
			const user = userEvent.setup();
			render(<InlineFeedbackInput {...defaultProps} />);

			// Start at textbox (auto-focused)
			expect(screen.getByRole('textbox')).toHaveFocus();

			// Tab to Cancel button
			await user.tab();
			expect(screen.getByRole('button', { name: /cancel/i })).toHaveFocus();

			// Tab to Add button
			await user.tab();
			expect(screen.getByRole('button', { name: /add/i })).toHaveFocus();
		});
	});
});
