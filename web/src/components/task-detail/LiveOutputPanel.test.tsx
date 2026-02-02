/**
 * TDD Tests for LiveOutputPanel Component - TASK-737
 *
 * Tests for: Implement live output panel for Task Detail
 *
 * Success Criteria Coverage:
 * - SC-11: Live output panel shows real-time transcript updates
 * - SC-12: Panel displays different message types with appropriate styling
 * - SC-13: Auto-scroll behavior works correctly during streaming
 * - SC-14: Panel handles long transcripts with virtual scrolling
 * - SC-15: Loading states and error handling are properly displayed
 * - SC-16: Panel integrates seamlessly with existing Task Detail layout
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { LiveOutputPanel } from './LiveOutputPanel';
import { EventProvider } from '@/hooks/EventProvider';
import type { TranscriptLine } from '@/hooks/useEvents';
import type { ReactNode } from 'react';

// Mock the scroll behavior for auto-scroll tests
const mockScrollIntoView = vi.fn();
Element.prototype.scrollIntoView = mockScrollIntoView;

// Mock hooks
vi.mock('@/hooks/useEvents', () => ({
	useTaskSubscription: vi.fn(() => ({
		state: null,
		transcript: [],
	})),
}));

vi.mock('@/stores', () => ({
	useCurrentProjectId: vi.fn(() => 'test-project'),
}));

function createWrapper() {
	return function Wrapper({ children }: { children: ReactNode }) {
		return <EventProvider autoConnect={false}>{children}</EventProvider>;
	};
}

describe('SC-11: Live output panel shows real-time transcript updates', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should display streaming transcript lines in real-time', async () => {
		// Arrange: Mock hook to return streaming transcript
		const streamingTranscript: TranscriptLine[] = [
			{
				content: 'Starting task implementation...',
				timestamp: new Date().toISOString(),
				type: 'prompt',
				phase: 'implement',
			},
			{
				content: 'I will implement the feature step by step.',
				timestamp: new Date().toISOString(),
				type: 'response',
				phase: 'implement',
				tokens: { input: 50, output: 120 },
			},
		];

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: streamingTranscript,
		});

		// Act: Render the live output panel
		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should display both transcript lines
		await waitFor(() => {
			expect(screen.getByText(/starting task implementation/i)).toBeInTheDocument();
			expect(screen.getByText(/i will implement the feature/i)).toBeInTheDocument();
		});

		// Should show streaming indicator
		expect(screen.getByText(/streaming/i)).toBeInTheDocument();
		expect(screen.getByLabelText(/streaming indicator/i)).toBeInTheDocument();
	});

	it('should update when new transcript lines are added', async () => {
		// Arrange: Start with empty transcript
		const { useTaskSubscription } = await import('@/hooks/useEvents');
		const mockSubscription = vi.mocked(useTaskSubscription);

		mockSubscription.mockReturnValue({
			state: null,
			transcript: [],
		});

		const { rerender } = render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Initially empty
		expect(screen.queryByText(/implementing/i)).not.toBeInTheDocument();

		// Act: Add new transcript line
		const newTranscript: TranscriptLine[] = [
			{
				content: 'Implementing the feature...',
				timestamp: new Date().toISOString(),
				type: 'response',
				phase: 'implement',
			},
		];

		mockSubscription.mockReturnValue({
			state: null,
			transcript: newTranscript,
		});

		rerender(<LiveOutputPanel taskId="TASK-001" isStreaming={true} />);

		// Assert: New content should be visible
		await waitFor(() => {
			expect(screen.getByText(/implementing the feature/i)).toBeInTheDocument();
		});
	});

	it('should handle empty transcript gracefully', () => {
		// Arrange: Empty transcript
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={false} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should show empty state
		expect(screen.getByText(/no output yet/i)).toBeInTheDocument();
		expect(screen.getByText(/waiting for task to start/i)).toBeInTheDocument();
	});
});

describe('SC-12: Panel displays different message types with appropriate styling', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should style different message types appropriately', async () => {
		// Arrange: Different message types
		const mixedTranscript: TranscriptLine[] = [
			{
				content: 'User prompt message',
				timestamp: new Date().toISOString(),
				type: 'prompt',
				phase: 'implement',
			},
			{
				content: 'Assistant response message',
				timestamp: new Date().toISOString(),
				type: 'response',
				phase: 'implement',
			},
			{
				content: 'Tool execution',
				timestamp: new Date().toISOString(),
				type: 'tool',
				phase: 'implement',
			},
			{
				content: 'Error occurred',
				timestamp: new Date().toISOString(),
				type: 'error',
				phase: 'implement',
			},
		];

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: mixedTranscript,
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Each message type should have appropriate styling
		const promptMessage = screen.getByText(/user prompt message/i).closest('[data-message-type]');
		expect(promptMessage).toHaveAttribute('data-message-type', 'prompt');
		expect(promptMessage).toHaveClass('transcript-message--prompt');

		const responseMessage = screen.getByText(/assistant response message/i).closest('[data-message-type]');
		expect(responseMessage).toHaveAttribute('data-message-type', 'response');
		expect(responseMessage).toHaveClass('transcript-message--response');

		const toolMessage = screen.getByText(/tool execution/i).closest('[data-message-type]');
		expect(toolMessage).toHaveAttribute('data-message-type', 'tool');
		expect(toolMessage).toHaveClass('transcript-message--tool');

		const errorMessage = screen.getByText(/error occurred/i).closest('[data-message-type]');
		expect(errorMessage).toHaveAttribute('data-message-type', 'error');
		expect(errorMessage).toHaveClass('transcript-message--error');
	});

	it('should display timestamps and phase information', async () => {
		const testTimestamp = '2024-01-01T12:00:00Z';
		const transcript: TranscriptLine[] = [
			{
				content: 'Test message with metadata',
				timestamp: testTimestamp,
				type: 'response',
				phase: 'spec',
			},
		];

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript,
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Metadata should be displayed
		expect(screen.getByText(/spec/i)).toBeInTheDocument(); // Phase
		expect(screen.getByText(/12:00/)).toBeInTheDocument(); // Timestamp (formatted)
	});

	it('should display token counts when available', async () => {
		const transcript: TranscriptLine[] = [
			{
				content: 'Response with token info',
				timestamp: new Date().toISOString(),
				type: 'response',
				phase: 'implement',
				tokens: {
					input: 250,
					output: 500,
				},
			},
		];

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript,
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Token information should be displayed
		expect(screen.getByText(/250.*tokens/i)).toBeInTheDocument(); // Input tokens
		expect(screen.getByText(/500.*tokens/i)).toBeInTheDocument(); // Output tokens
	});
});

describe('SC-13: Auto-scroll behavior works correctly during streaming', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockScrollIntoView.mockClear();
	});

	it('should auto-scroll to bottom when new messages arrive during streaming', async () => {
		const { useTaskSubscription } = await import('@/hooks/useEvents');
		const mockSubscription = vi.mocked(useTaskSubscription);

		// Start with one message
		mockSubscription.mockReturnValue({
			state: null,
			transcript: [
				{
					content: 'First message',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
			],
		});

		const { rerender } = render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} autoScroll={true} />,
			{ wrapper: createWrapper() }
		);

		// Add a new message
		mockSubscription.mockReturnValue({
			state: null,
			transcript: [
				{
					content: 'First message',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
				{
					content: 'Second message',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
			],
		});

		rerender(<LiveOutputPanel taskId="TASK-001" isStreaming={true} autoScroll={true} />);

		// Assert: Should auto-scroll to the new message
		await waitFor(() => {
			expect(mockScrollIntoView).toHaveBeenCalledWith({
				behavior: 'smooth',
				block: 'end',
			});
		});
	});

	it('should not auto-scroll when autoScroll is disabled', async () => {
		const { useTaskSubscription } = await import('@/hooks/useEvents');
		const mockSubscription = vi.mocked(useTaskSubscription);

		mockSubscription.mockReturnValue({
			state: null,
			transcript: [
				{
					content: 'Message without auto-scroll',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
			],
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} autoScroll={false} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should not trigger auto-scroll
		expect(mockScrollIntoView).not.toHaveBeenCalled();
	});

	it('should allow manual scroll override during streaming', async () => {
		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [
				{
					content: 'Test message',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
			],
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} autoScroll={true} />,
			{ wrapper: createWrapper() }
		);

		// Act: User manually scrolls up
		const scrollContainer = screen.getByRole('log');
		fireEvent.scroll(scrollContainer, { target: { scrollTop: 0 } });

		// Wait a bit then add new content
		await waitFor(() => {
			// Auto-scroll should be temporarily disabled after manual scroll
			expect(scrollContainer).toHaveAttribute('data-auto-scroll', 'false');
		});
	});
});

describe('SC-14: Panel handles long transcripts with virtual scrolling', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should enable virtual scrolling for large transcripts', async () => {
		// Arrange: Large transcript
		const largeTranscript: TranscriptLine[] = Array.from({ length: 1000 }, (_, i) => ({
			content: `Message ${i + 1} - This is a long transcript with many entries`,
			timestamp: new Date().toISOString(),
			type: 'response' as const,
			phase: 'implement',
		}));

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: largeTranscript,
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={false} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should use virtual scrolling for performance
		const virtualContainer = screen.getByTestId('transcript-virtual-list');
		expect(virtualContainer).toBeInTheDocument();
		expect(virtualContainer).toHaveAttribute('data-virtual', 'true');

		// Should not render all items at once (only visible ones)
		const visibleMessages = screen.getAllByText(/Message \d+ -/);
		expect(visibleMessages.length).toBeLessThan(1000);
		expect(visibleMessages.length).toBeGreaterThan(0);
	});

	it('should use regular rendering for small transcripts', async () => {
		// Arrange: Small transcript
		const smallTranscript: TranscriptLine[] = Array.from({ length: 20 }, (_, i) => ({
			content: `Message ${i + 1}`,
			timestamp: new Date().toISOString(),
			type: 'response' as const,
			phase: 'implement',
		}));

		const { useTaskSubscription } = await import('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: smallTranscript,
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={false} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should use regular rendering (no virtual scrolling)
		expect(screen.queryByTestId('transcript-virtual-list')).not.toBeInTheDocument();

		// Should render all items
		const allMessages = screen.getAllByText(/Message \d+/);
		expect(allMessages).toHaveLength(20);
	});
});

describe('SC-15: Loading states and error handling', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should show loading state while connecting to stream', () => {
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} loading={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Should show loading indicator
		expect(screen.getByText(/connecting to live output/i)).toBeInTheDocument();
		expect(screen.getByLabelText(/loading indicator/i)).toBeInTheDocument();
	});

	it('should handle stream connection errors', () => {
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<LiveOutputPanel
				taskId="TASK-001"
				isStreaming={false}
				error="Failed to connect to live output stream"
			/>,
			{ wrapper: createWrapper() }
		);

		// Assert: Should show error message
		expect(screen.getByText(/failed to connect to live output stream/i)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /retry connection/i })).toBeInTheDocument();
	});

	it('should allow retry after connection failure', () => {
		const mockRetry = vi.fn();

		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<LiveOutputPanel
				taskId="TASK-001"
				isStreaming={false}
				error="Connection failed"
				onRetry={mockRetry}
			/>,
			{ wrapper: createWrapper() }
		);

		// Act: Click retry button
		const retryButton = screen.getByRole('button', { name: /retry connection/i });
		fireEvent.click(retryButton);

		// Assert: Retry callback should be called
		expect(mockRetry).toHaveBeenCalledOnce();
	});
});

describe('SC-16: Panel integrates with existing Task Detail layout', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should render within the provided container dimensions', () => {
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<div style={{ height: '400px', width: '600px' }}>
				<LiveOutputPanel taskId="TASK-001" isStreaming={false} />
			</div>,
			{ wrapper: createWrapper() }
		);

		const panel = screen.getByTestId('live-output-panel');
		expect(panel).toHaveStyle({ height: '100%', width: '100%' });
	});

	it('should support responsive design for different screen sizes', () => {
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [],
		});

		render(
			<LiveOutputPanel
				taskId="TASK-001"
				isStreaming={false}
				compact={true} // Compact mode for small screens
			/>,
			{ wrapper: createWrapper() }
		);

		const panel = screen.getByTestId('live-output-panel');
		expect(panel).toHaveClass('live-output-panel--compact');
	});

	it('should maintain accessibility standards', () => {
		const { useTaskSubscription } = require('@/hooks/useEvents');
		vi.mocked(useTaskSubscription).mockReturnValue({
			state: null,
			transcript: [
				{
					content: 'Accessible transcript content',
					timestamp: new Date().toISOString(),
					type: 'response',
					phase: 'implement',
				},
			],
		});

		render(
			<LiveOutputPanel taskId="TASK-001" isStreaming={true} />,
			{ wrapper: createWrapper() }
		);

		// Assert: Proper ARIA attributes and roles
		expect(screen.getByRole('log')).toBeInTheDocument();
		expect(screen.getByLabelText(/live output for task-001/i)).toBeInTheDocument();

		// Should support keyboard navigation
		const panel = screen.getByRole('log');
		expect(panel).toHaveAttribute('tabindex', '0');
	});
});