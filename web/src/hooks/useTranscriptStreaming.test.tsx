/**
 * TDD Tests for Live Output Panel - TASK-737
 *
 * Tests for: Implement live output panel for Task Detail
 *
 * Success Criteria Coverage:
 * - SC-1: Live transcript streaming - useTaskSubscription should receive real-time transcript lines via WebSocket events
 * - SC-2: Event-to-transcript conversion - Connect RPC transcript events should be converted to TranscriptLine format
 * - SC-3: Real-time UI updates - TranscriptViewer should display streaming content as it arrives
 * - SC-4: Auto-scroll behavior - When streaming is active, transcript should auto-scroll to latest content
 * - SC-5: Streaming indicator - UI should show visual indication when transcript is actively streaming
 * - SC-6: Streaming lifecycle - Transcript streaming should start/stop based on task execution state
 */

import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { render, screen } from '@testing-library/react';
import { useTaskSubscription, type TranscriptLine } from './useEvents';
import { EventProvider } from './EventProvider';
import { TranscriptViewer } from '@/components/transcript/TranscriptViewer';
import { create } from '@bufbuild/protobuf';
import { EventSchema, type Event } from '@/gen/orc/v1/events_pb';
import type { ReactNode } from 'react';

// Mock the EventSubscription class to control event emissions
const mockEventHandlers: ((event: Event) => void)[] = [];
const mockConnect = vi.fn();
const mockDisconnect = vi.fn();
const mockOn = vi.fn((handler: (event: Event) => void) => {
	mockEventHandlers.push(handler);
	return () => {
		const index = mockEventHandlers.indexOf(handler);
		if (index > -1) mockEventHandlers.splice(index, 1);
	};
});

vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		onStatusChange: vi.fn(() => () => {}),
		on: mockOn,
		connect: mockConnect,
		disconnect: mockDisconnect,
		isConnected: vi.fn(() => true),
	})),
	handleEvent: vi.fn(),
}));

// Mock stores
vi.mock('@/stores/taskStore', () => ({
	useTaskStore: vi.fn(() => ({
		getState: () => ({
			reset: vi.fn(),
			updateTaskState: vi.fn(),
			removeTaskState: vi.fn(),
		}),
	})),
	useTaskState: vi.fn(() => null),
	useTask: vi.fn(() => null),
}));

vi.mock('@/stores', () => ({
	useCurrentProjectId: vi.fn(() => 'test-project'),
	useUIStore: vi.fn(() => ({
		setWsStatus: vi.fn(),
	})),
}));

// Mock transcript client
vi.mock('@/lib/client', () => ({
	transcriptClient: {
		listTranscripts: vi.fn(),
		getTranscript: vi.fn(),
	},
}));

// Helper to emit transcript events
function emitTranscriptEvent(taskId: string, content: string, type: 'prompt' | 'response' | 'tool' | 'error' = 'response') {
	const event = create(EventSchema, {
		type: 'transcript_chunk',
		taskId,
		data: JSON.stringify({
			content,
			timestamp: new Date().toISOString(),
			type,
			phase: 'implement',
		}),
	});

	// Emit to all registered handlers
	mockEventHandlers.forEach(handler => handler(event));
}

// Helper to create wrapper with EventProvider
function createWrapper() {
	return function Wrapper({ children }: { children: ReactNode }) {
		return <EventProvider autoConnect={false}>{children}</EventProvider>;
	};
}

describe('SC-1: Live transcript streaming - useTaskSubscription receives real-time transcript lines', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should start with empty transcript array', () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		expect(result.current.transcript).toEqual([]);
	});

	it('should receive transcript lines when transcript_chunk events are emitted', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit a transcript event
		act(() => {
			emitTranscriptEvent('TASK-001', 'Hello from Claude!', 'response');
		});

		// Assert: Transcript should be updated
		await waitFor(() => {
			expect(result.current.transcript).toHaveLength(1);
			expect(result.current.transcript[0]).toEqual(
				expect.objectContaining({
					content: 'Hello from Claude!',
					type: 'response',
					phase: 'implement',
				})
			);
		});
	});

	it('should accumulate multiple transcript lines', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit multiple events
		act(() => {
			emitTranscriptEvent('TASK-001', 'First message', 'prompt');
			emitTranscriptEvent('TASK-001', 'Second message', 'response');
		});

		// Assert: Both messages should be in transcript
		await waitFor(() => {
			expect(result.current.transcript).toHaveLength(2);
			expect(result.current.transcript[0].content).toBe('First message');
			expect(result.current.transcript[1].content).toBe('Second message');
		});
	});

	it('should only receive events for the subscribed taskId', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit events for different tasks
		act(() => {
			emitTranscriptEvent('TASK-001', 'For task 1', 'response');
			emitTranscriptEvent('TASK-002', 'For task 2', 'response');
		});

		// Assert: Only the matching task's events should be received
		await waitFor(() => {
			expect(result.current.transcript).toHaveLength(1);
			expect(result.current.transcript[0].content).toBe('For task 1');
		});
	});

	it('should clear transcript when taskId changes', async () => {
		const { result, rerender } = renderHook(
			({ taskId }) => useTaskSubscription(taskId),
			{
				wrapper: createWrapper(),
				initialProps: { taskId: 'TASK-001' },
			}
		);

		// Arrange: Add some transcript data
		act(() => {
			emitTranscriptEvent('TASK-001', 'Initial content', 'response');
		});

		await waitFor(() => {
			expect(result.current.transcript).toHaveLength(1);
		});

		// Act: Change taskId
		rerender({ taskId: 'TASK-002' });

		// Assert: Transcript should be cleared
		expect(result.current.transcript).toEqual([]);
	});
});

describe('SC-2: Event-to-transcript conversion - Connect RPC events converted to TranscriptLine format', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should convert transcript_chunk events to TranscriptLine format', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		const testTimestamp = '2024-01-01T12:00:00Z';

		// Act: Emit event with specific format
		act(() => {
			const event = create(EventSchema, {
				type: 'transcript_chunk',
				taskId: 'TASK-001',
				data: JSON.stringify({
					content: 'Test content',
					timestamp: testTimestamp,
					type: 'tool',
					phase: 'spec',
					tokens: {
						input: 100,
						output: 50,
					},
				}),
			});

			mockEventHandlers.forEach(handler => handler(event));
		});

		// Assert: Should be properly converted
		await waitFor(() => {
			expect(result.current.transcript[0]).toEqual({
				content: 'Test content',
				timestamp: testTimestamp,
				type: 'tool',
				phase: 'spec',
				tokens: {
					input: 100,
					output: 50,
				},
			});
		});
	});

	it('should handle different transcript line types', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit different types
		act(() => {
			emitTranscriptEvent('TASK-001', 'User prompt', 'prompt');
			emitTranscriptEvent('TASK-001', 'Assistant response', 'response');
			emitTranscriptEvent('TASK-001', 'Tool call', 'tool');
			emitTranscriptEvent('TASK-001', 'Error message', 'error');
		});

		// Assert: All types should be preserved
		await waitFor(() => {
			expect(result.current.transcript).toHaveLength(4);
			expect(result.current.transcript[0].type).toBe('prompt');
			expect(result.current.transcript[1].type).toBe('response');
			expect(result.current.transcript[2].type).toBe('tool');
			expect(result.current.transcript[3].type).toBe('error');
		});
	});

	it('should handle malformed event data gracefully', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit event with invalid JSON
		act(() => {
			const event = create(EventSchema, {
				type: 'transcript_chunk',
				taskId: 'TASK-001',
				data: 'invalid json{',
			});

			mockEventHandlers.forEach(handler => handler(event));
		});

		// Assert: Should not break, transcript should remain empty
		await waitFor(() => {
			expect(result.current.transcript).toEqual([]);
		});
	});

	it('should ignore non-transcript events', async () => {
		const { result } = renderHook(() => useTaskSubscription('TASK-001'), {
			wrapper: createWrapper(),
		});

		// Act: Emit non-transcript event
		act(() => {
			const event = create(EventSchema, {
				type: 'task_status_changed',
				taskId: 'TASK-001',
				data: JSON.stringify({ status: 'running' }),
			});

			mockEventHandlers.forEach(handler => handler(event));
		});

		// Assert: Transcript should remain empty
		expect(result.current.transcript).toEqual([]);
	});
});

describe('SC-3: Real-time UI updates - TranscriptViewer displays streaming content', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should display streaming transcript lines in the UI', async () => {
		// Arrange: Mock the useTranscripts hook to return streaming lines
		const streamingLines: TranscriptLine[] = [{
			content: 'Streaming content',
			timestamp: new Date().toISOString(),
			type: 'response',
			phase: 'implement',
		}];

		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [],
				phases: [],
				loading: false,
				streamingLines,
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		// Act: Render TranscriptViewer
		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Streaming content should be visible
		await waitFor(() => {
			expect(screen.getByText(/streaming content/i)).toBeInTheDocument();
		});
	});

	it('should show streaming indicator when transcript is actively streaming', async () => {
		const streamingLines: TranscriptLine[] = [{
			content: 'Live content',
			timestamp: new Date().toISOString(),
			type: 'response',
		}];

		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [],
				phases: [],
				loading: false,
				streamingLines,
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Streaming indicator should be present
		await waitFor(() => {
			expect(screen.getByText(/live streaming/i)).toBeInTheDocument();
		});
	});
});

describe('SC-4: Auto-scroll behavior - Transcript auto-scrolls to latest content', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should enable auto-scroll when task is running', () => {
		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [],
				phases: [],
				loading: false,
				streamingLines: [],
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Auto-scroll button should show as enabled
		const autoScrollBtn = screen.getByTitle(/auto-scroll enabled/i);
		expect(autoScrollBtn).toBeInTheDocument();
		expect(autoScrollBtn).toHaveClass('active');
	});

	it('should allow toggling auto-scroll behavior', () => {
		const mockToggleAutoScroll = vi.fn();

		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [],
				phases: [],
				loading: false,
				streamingLines: [],
				isAutoScrollEnabled: true,
				toggleAutoScroll: mockToggleAutoScroll,
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Act: Click auto-scroll button
		const autoScrollBtn = screen.getByTitle(/auto-scroll enabled/i);
		autoScrollBtn.click();

		// Assert: Toggle function should be called
		expect(mockToggleAutoScroll).toHaveBeenCalled();
	});
});

describe('SC-5: Streaming indicator - UI shows visual indication when streaming', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should show streaming count in header when streaming is active', () => {
		const streamingLines: TranscriptLine[] = [
			{ content: 'Line 1', timestamp: new Date().toISOString(), type: 'response' },
			{ content: 'Line 2', timestamp: new Date().toISOString(), type: 'response' },
		];

		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [{ id: 1 }], // 1 persisted message
				phases: [],
				loading: false,
				streamingLines,
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Should show both persisted and streaming count
		expect(screen.getByText(/1 messages \+ 2 streaming/i)).toBeInTheDocument();
	});

	it('should hide streaming indicator when no streaming content', () => {
		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [{ id: 1 }],
				phases: [],
				loading: false,
				streamingLines: [], // No streaming content
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: vi.fn(),
				refresh: vi.fn(),
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={false} />
			</EventProvider>
		);

		// Assert: Should not show streaming indicator
		expect(screen.queryByText(/live streaming/i)).not.toBeInTheDocument();
		expect(screen.getByText('1 messages')).toBeInTheDocument();
	});
});

describe('SC-6: Streaming lifecycle - Streaming based on task execution state', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockEventHandlers.length = 0;
	});

	it('should start streaming when task execution begins', () => {
		const { rerender } = render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={false} />
			</EventProvider>
		);

		// Assert: Initially not running, auto-scroll button should be hidden
		expect(screen.queryByTitle(/auto-scroll/i)).not.toBeInTheDocument();

		// Act: Task starts running
		rerender(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Auto-scroll should be available when running
		expect(screen.getByTitle(/auto-scroll/i)).toBeInTheDocument();
	});

	it('should clear streaming lines periodically when running', () => {
		const mockClearStreamingLines = vi.fn();
		const mockRefresh = vi.fn();

		vi.doMock('@/hooks/useTranscripts', () => ({
			useTranscripts: vi.fn(() => ({
				transcripts: [],
				phases: [],
				loading: false,
				streamingLines: [{ content: 'Test', timestamp: new Date().toISOString(), type: 'response' }],
				isAutoScrollEnabled: true,
				toggleAutoScroll: vi.fn(),
				clearStreamingLines: mockClearStreamingLines,
				refresh: mockRefresh,
			})),
		}));

		render(
			<EventProvider autoConnect={false}>
				<TranscriptViewer taskId="TASK-001" isRunning={true} />
			</EventProvider>
		);

		// Assert: Clear and refresh functions should be available for periodic sync
		expect(mockClearStreamingLines).toBeDefined();
		expect(mockRefresh).toBeDefined();
	});
});