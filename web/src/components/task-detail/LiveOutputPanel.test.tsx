/**
 * TDD Tests for LiveOutputPanel Component
 *
 * Tests for TASK-774: Restore test coverage for components with deleted tests
 *
 * Success Criteria Coverage:
 * - SC-1: Displays transcript messages with proper styling
 * - SC-2: Shows loading state during connection
 * - SC-3: Shows error state with retry button
 * - SC-4: Auto-scrolls to new messages when streaming
 * - SC-5: Uses virtual scrolling for large transcripts (>100 items)
 * - SC-6: Disables auto-scroll when user scrolls up manually
 * - SC-7: Shows streaming indicator when isStreaming is true
 * - SC-8: Shows empty state when no messages
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { TranscriptLine } from '@/hooks/useEvents';

// Mock the useTaskSubscription hook
const mockTranscript: TranscriptLine[] = [];
const mockUseTaskSubscription = vi.fn((_taskId?: string) => ({ transcript: mockTranscript }));

vi.mock('@/hooks/useEvents', () => ({
	useTaskSubscription: (taskId: string) => mockUseTaskSubscription(taskId),
}));

// Import after mocks are set up
import { LiveOutputPanel } from './LiveOutputPanel';

/** Create a mock TranscriptLine */
function createMockTranscriptLine(overrides: Partial<TranscriptLine> = {}): TranscriptLine {
	return {
		type: 'response',
		content: 'Test message content',
		timestamp: '2024-01-01T12:00:00Z',
		phase: 'implement',
		...overrides,
	};
}

/** Create multiple transcript lines for testing */
function createMockTranscript(count: number): TranscriptLine[] {
	return Array.from({ length: count }, (_, i) =>
		createMockTranscriptLine({
			content: `Message ${i + 1}`,
			timestamp: new Date(Date.now() + i * 1000).toISOString(),
		})
	);
}

describe('TASK-774: LiveOutputPanel Component', () => {
	const defaultProps = {
		taskId: 'TASK-001',
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockTranscript.length = 0;
		mockUseTaskSubscription.mockReturnValue({ transcript: mockTranscript });
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Displays transcript messages with proper styling', () => {
		it('renders transcript messages', async () => {
			mockTranscript.push(
				createMockTranscriptLine({ content: 'First message', type: 'prompt' }),
				createMockTranscriptLine({ content: 'Second message', type: 'response' })
			);
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			expect(screen.getByText('First message')).toBeInTheDocument();
			expect(screen.getByText('Second message')).toBeInTheDocument();
		});

		it('applies correct styling for prompt messages', async () => {
			mockTranscript.push(createMockTranscriptLine({ type: 'prompt', content: 'Prompt text' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			const message = screen.getByText('Prompt text').closest('.transcript-message');
			expect(message).toHaveClass('transcript-message--prompt');
		});

		it('applies correct styling for response messages', async () => {
			mockTranscript.push(createMockTranscriptLine({ type: 'response', content: 'Response text' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			const message = screen.getByText('Response text').closest('.transcript-message');
			expect(message).toHaveClass('transcript-message--response');
		});

		it('applies correct styling for tool messages', async () => {
			mockTranscript.push(createMockTranscriptLine({ type: 'tool', content: 'Tool call result' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			const message = screen.getByText('Tool call result').closest('.transcript-message');
			expect(message).toHaveClass('transcript-message--tool');
		});

		it('applies correct styling for error messages', async () => {
			mockTranscript.push(createMockTranscriptLine({ type: 'error', content: 'Error occurred' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			const message = screen.getByText('Error occurred').closest('.transcript-message');
			expect(message).toHaveClass('transcript-message--error');
		});

		it('displays phase name when provided', async () => {
			mockTranscript.push(createMockTranscriptLine({ phase: 'implement' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			expect(screen.getByText('implement')).toBeInTheDocument();
		});

		it('displays formatted timestamp', async () => {
			mockTranscript.push(createMockTranscriptLine({ timestamp: '2024-01-15T14:30:45Z' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			// Should display formatted time (format varies by locale)
			expect(screen.getByText(/\d{1,2}:\d{2}/)).toBeInTheDocument();
		});

		it('displays token counts when provided', async () => {
			mockTranscript.push(
				createMockTranscriptLine({
					tokens: { input: 500, output: 300 },
				})
			);
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			expect(screen.getByText(/500 input tokens/)).toBeInTheDocument();
			expect(screen.getByText(/300 output tokens/)).toBeInTheDocument();
		});
	});

	describe('SC-2: Shows loading state during connection', () => {
		it('displays loading indicator when loading is true', async () => {
			render(<LiveOutputPanel {...defaultProps} loading={true} />);

			expect(screen.getByTestId('live-output-panel')).toBeInTheDocument();
			expect(screen.getByText(/connecting/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/loading indicator/i)).toBeInTheDocument();
		});

		it('does not show loading when loading is false', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} loading={false} />);

			expect(screen.queryByText(/connecting/i)).not.toBeInTheDocument();
		});
	});

	describe('SC-3: Shows error state with retry button', () => {
		it('displays error message when error prop is provided', async () => {
			render(<LiveOutputPanel {...defaultProps} error="Connection failed" />);

			expect(screen.getByText('Connection failed')).toBeInTheDocument();
		});

		it('displays retry button when onRetry callback is provided', async () => {
			render(
				<LiveOutputPanel {...defaultProps} error="Connection failed" onRetry={vi.fn()} />
			);

			expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
		});

		it('calls onRetry when retry button is clicked', async () => {
			const user = userEvent.setup();
			const onRetry = vi.fn();

			render(
				<LiveOutputPanel {...defaultProps} error="Connection failed" onRetry={onRetry} />
			);

			await user.click(screen.getByRole('button', { name: /retry/i }));

			expect(onRetry).toHaveBeenCalledTimes(1);
		});

		it('does not display retry button when onRetry is not provided', async () => {
			render(<LiveOutputPanel {...defaultProps} error="Connection failed" />);

			expect(screen.queryByRole('button', { name: /retry/i })).not.toBeInTheDocument();
		});
	});

	describe('SC-4: Auto-scrolls to new messages when streaming', () => {
		it('has auto-scroll enabled by default when streaming', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			const scrollContainer = screen.getByRole('log');
			expect(scrollContainer).toHaveAttribute('data-auto-scroll', 'true');
		});

		it('has data-auto-scroll attribute that reflects scroll state', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			const scrollContainer = screen.getByRole('log');
			expect(scrollContainer).toHaveAttribute('data-auto-scroll');
		});
	});

	describe('SC-5: Uses virtual scrolling for large transcripts', () => {
		it('uses regular rendering for small transcripts (<= 100)', async () => {
			mockTranscript.push(...createMockTranscript(50));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			// Should NOT have virtual scrolling container
			expect(screen.queryByTestId('transcript-virtual-list')).not.toBeInTheDocument();
		});

		it('uses virtualized rendering for large transcripts (> 100)', async () => {
			mockTranscript.push(...createMockTranscript(150));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			// Should have virtual scrolling container
			expect(screen.getByTestId('transcript-virtual-list')).toBeInTheDocument();
			expect(screen.getByTestId('transcript-virtual-list')).toHaveAttribute('data-virtual', 'true');
		});

		it('only renders visible portion of large transcripts', async () => {
			mockTranscript.push(...createMockTranscript(200));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			// With virtualization, not all 200 messages should be in the DOM
			const messages = screen.getAllByText(/Message \d+/);
			expect(messages.length).toBeLessThan(200);
		});
	});

	describe('SC-6: Disables auto-scroll when user scrolls up manually', () => {
		it('detects when user scrolls away from bottom', async () => {
			mockTranscript.push(...createMockTranscript(20));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			const scrollContainer = screen.getByRole('log');

			// Simulate scroll up
			Object.defineProperty(scrollContainer, 'scrollTop', { value: 0, writable: true });
			Object.defineProperty(scrollContainer, 'scrollHeight', { value: 1000, writable: true });
			Object.defineProperty(scrollContainer, 'clientHeight', { value: 500, writable: true });

			fireEvent.scroll(scrollContainer);

			await waitFor(() => {
				expect(scrollContainer).toHaveAttribute('data-auto-scroll', 'false');
			});
		});

		it('re-enables auto-scroll when user scrolls to bottom', async () => {
			mockTranscript.push(...createMockTranscript(20));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			const scrollContainer = screen.getByRole('log');

			// Simulate scroll to bottom
			Object.defineProperty(scrollContainer, 'scrollTop', { value: 500, writable: true });
			Object.defineProperty(scrollContainer, 'scrollHeight', { value: 1000, writable: true });
			Object.defineProperty(scrollContainer, 'clientHeight', { value: 500, writable: true });

			fireEvent.scroll(scrollContainer);

			await waitFor(() => {
				expect(scrollContainer).toHaveAttribute('data-auto-scroll', 'true');
			});
		});
	});

	describe('SC-7: Shows streaming indicator when isStreaming is true', () => {
		it('displays streaming indicator in header when streaming', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			expect(screen.getByLabelText(/streaming indicator/i)).toBeInTheDocument();
			// Check for the header "Streaming" text (exact match to avoid matching "Live streaming..." in footer)
			expect(screen.getByText(/^Streaming$/)).toBeInTheDocument();
		});

		it('displays streaming status in footer when streaming with messages', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={true} />);

			expect(screen.getByText(/live streaming/i)).toBeInTheDocument();
		});

		it('does not display streaming indicator when not streaming', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} isStreaming={false} />);

			expect(screen.queryByLabelText(/streaming indicator/i)).not.toBeInTheDocument();
		});
	});

	describe('SC-8: Shows empty state when no messages', () => {
		it('displays empty state message when transcript is empty', async () => {
			mockUseTaskSubscription.mockReturnValue({ transcript: [] });

			render(<LiveOutputPanel {...defaultProps} />);

			expect(screen.getByText(/no output yet/i)).toBeInTheDocument();
			expect(screen.getByText(/waiting for task/i)).toBeInTheDocument();
		});

		it('does not show empty state when there are messages', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			expect(screen.queryByText(/no output yet/i)).not.toBeInTheDocument();
		});
	});

	describe('Edge Cases', () => {
		it('handles compact mode', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} compact={true} />);

			const panel = screen.getByTestId('live-output-panel');
			expect(panel).toHaveClass('live-output-panel--compact');
		});

		it('renders accessible aria-label with task ID', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} taskId="TASK-123" />);

			expect(screen.getByLabelText(/live output for TASK-123/i)).toBeInTheDocument();
		});

		it('handles malformed timestamp gracefully', async () => {
			mockTranscript.push(createMockTranscriptLine({ timestamp: 'invalid-timestamp' }));
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			// Should not crash and should display the raw timestamp
			expect(screen.getByText('invalid-timestamp')).toBeInTheDocument();
		});

		it('resets scroll state when taskId changes', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			const { rerender } = render(<LiveOutputPanel {...defaultProps} taskId="TASK-001" />);

			// Change task ID
			rerender(<LiveOutputPanel {...defaultProps} taskId="TASK-002" />);

			// Hook should be called with new task ID
			expect(mockUseTaskSubscription).toHaveBeenCalledWith('TASK-002');
		});

		it('has focusable scroll container for keyboard navigation', async () => {
			mockTranscript.push(createMockTranscriptLine());
			mockUseTaskSubscription.mockReturnValue({ transcript: [...mockTranscript] });

			render(<LiveOutputPanel {...defaultProps} />);

			const scrollContainer = screen.getByRole('log');
			expect(scrollContainer).toHaveAttribute('tabIndex', '0');
		});
	});
});
