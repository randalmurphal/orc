/**
 * Tests for FeedbackIndicator component
 *
 * TDD tests for the 💬 indicator that appears on lines with existing feedback.
 *
 * Success Criteria Coverage:
 * - SC-5: Lines with existing inline feedback show a comment indicator icon (💬)
 * - SC-6: Clicking the indicator shows existing feedback for that line
 *
 * Edge Cases:
 * - Multiple feedbacks on same line → Each shows as separate item in tooltip
 * - Feedback text truncated in indicator popover at 200 chars
 */

import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FeedbackIndicator } from './FeedbackIndicator';
import { createMockFeedback } from '@/test/factories';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';

describe('FeedbackIndicator', () => {
	const mockFeedback = createMockFeedback({
		id: 'feedback-1',
		text: 'Use validateSession instead',
		type: FeedbackType.INLINE,
		timing: FeedbackTiming.WHEN_DONE,
		file: 'src/main.go',
		line: 47,
		received: false,
	});

	describe('SC-5: Renders feedback indicator', () => {
		it('renders the 💬 indicator icon when feedback exists', () => {
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			expect(screen.getByText('💬')).toBeInTheDocument();
		});

		it('does not render when no feedback exists', () => {
			render(<FeedbackIndicator feedback={[]} />);

			expect(screen.queryByText('💬')).not.toBeInTheDocument();
		});

		it('has appropriate ARIA label for accessibility', () => {
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			const indicator = screen.getByRole('button');
			expect(indicator).toHaveAttribute('aria-label', expect.stringMatching(/feedback/i));
		});

		it('shows count badge when multiple feedbacks exist', () => {
			const feedbacks = [
				mockFeedback,
				createMockFeedback({ id: 'feedback-2', text: 'Another comment' }),
			];
			render(<FeedbackIndicator feedback={feedbacks} />);

			expect(screen.getByText('2')).toBeInTheDocument();
		});

		it('does not show count badge for single feedback', () => {
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			expect(screen.queryByText('1')).not.toBeInTheDocument();
		});
	});

	describe('SC-6: Clicking indicator shows feedback', () => {
		it('shows popover with feedback text when clicked', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				expect(screen.getByText('Use validateSession instead')).toBeInTheDocument();
			});
		});

		it('shows feedback timing in popover', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				expect(screen.getByText(/when done/i)).toBeInTheDocument();
			});
		});

		it('shows pending/received status', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				expect(screen.getByText(/pending/i)).toBeInTheDocument();
			});
		});

		it('closes popover on click outside', async () => {
			const user = userEvent.setup();
			render(
				<div>
					<FeedbackIndicator feedback={[mockFeedback]} />
					<button data-testid="outside">Outside</button>
				</div>
			);

			// Open popover
			await user.click(screen.getByRole('button', { name: /feedback/i }));
			await waitFor(() => {
				expect(screen.getByText('Use validateSession instead')).toBeInTheDocument();
			});

			// Click outside
			await user.click(screen.getByTestId('outside'));

			await waitFor(() => {
				expect(screen.queryByText('Use validateSession instead')).not.toBeInTheDocument();
			});
		});

		it('closes popover on Escape key', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			// Open popover
			await user.click(screen.getByRole('button'));
			await waitFor(() => {
				expect(screen.getByText('Use validateSession instead')).toBeInTheDocument();
			});

			// Press Escape
			await user.keyboard('{Escape}');

			await waitFor(() => {
				expect(screen.queryByText('Use validateSession instead')).not.toBeInTheDocument();
			});
		});
	});

	describe('Edge Cases: Multiple feedbacks on same line', () => {
		it('shows all feedback items in popover', async () => {
			const user = userEvent.setup();
			const feedbacks = [
				createMockFeedback({ id: 'fb-1', text: 'First comment' }),
				createMockFeedback({ id: 'fb-2', text: 'Second comment' }),
				createMockFeedback({ id: 'fb-3', text: 'Third comment' }),
			];

			render(<FeedbackIndicator feedback={feedbacks} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				expect(screen.getByText('First comment')).toBeInTheDocument();
				expect(screen.getByText('Second comment')).toBeInTheDocument();
				expect(screen.getByText('Third comment')).toBeInTheDocument();
			});
		});
	});

	describe('Edge Cases: Long feedback text truncation', () => {
		it('truncates feedback text at 200 chars in popover', async () => {
			const user = userEvent.setup();
			const longText = 'A'.repeat(250);
			const truncatedFeedback = createMockFeedback({
				id: 'long-fb',
				text: longText,
			});

			render(<FeedbackIndicator feedback={[truncatedFeedback]} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				// Should show truncated text (200 chars + ellipsis)
				const displayedText = screen.getByTestId('feedback-text').textContent;
				expect(displayedText?.length).toBeLessThanOrEqual(203); // 200 + '...'
				expect(displayedText).toContain('...');
			});
		});

		it('shows full text if under 200 chars', async () => {
			const user = userEvent.setup();
			const shortText = 'This is a short comment';
			const shortFeedback = createMockFeedback({
				id: 'short-fb',
				text: shortText,
			});

			render(<FeedbackIndicator feedback={[shortFeedback]} />);

			await user.click(screen.getByRole('button'));

			await waitFor(() => {
				const displayedText = screen.getByTestId('feedback-text').textContent;
				expect(displayedText).toBe(shortText);
			});
		});
	});

	describe('Styling and visual states', () => {
		it('applies different style for received vs pending feedback', () => {
			const receivedFeedback = createMockFeedback({ id: 'fb-1', received: true });
			const pendingFeedback = createMockFeedback({ id: 'fb-2', received: false });

			const { rerender } = render(<FeedbackIndicator feedback={[receivedFeedback]} />);
			const receivedIndicator = screen.getByRole('button');
			expect(receivedIndicator).toHaveClass('received');

			rerender(<FeedbackIndicator feedback={[pendingFeedback]} />);
			const pendingIndicator = screen.getByRole('button');
			expect(pendingIndicator).toHaveClass('pending');
		});

		it('shows mixed state when some feedback is received and some is pending', () => {
			const feedbacks = [
				createMockFeedback({ id: 'fb-1', received: true }),
				createMockFeedback({ id: 'fb-2', received: false }),
			];

			render(<FeedbackIndicator feedback={feedbacks} />);

			// Should show pending style if any feedback is pending
			const indicator = screen.getByRole('button');
			expect(indicator).toHaveClass('pending');
		});
	});

	describe('Accessibility', () => {
		it('indicator is keyboard focusable', async () => {
			const user = userEvent.setup();
			render(
				<div>
					<button>Before</button>
					<FeedbackIndicator feedback={[mockFeedback]} />
				</div>
			);

			const beforeButton = screen.getByText('Before');
			beforeButton.focus();
			await user.tab();

			const indicator = screen.getByRole('button', { name: /feedback/i });
			expect(indicator).toHaveFocus();
		});

		it('popover can be opened with Enter key', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			const indicator = screen.getByRole('button');
			indicator.focus();
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(screen.getByText('Use validateSession instead')).toBeInTheDocument();
			});
		});

		it('popover can be opened with Space key', async () => {
			const user = userEvent.setup();
			render(<FeedbackIndicator feedback={[mockFeedback]} />);

			const indicator = screen.getByRole('button');
			indicator.focus();
			await user.keyboard(' ');

			await waitFor(() => {
				expect(screen.getByText('Use validateSession instead')).toBeInTheDocument();
			});
		});
	});
});
