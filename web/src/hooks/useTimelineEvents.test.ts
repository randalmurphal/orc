/**
 * Tests for useTimelineEvents hook
 *
 * TDD tests for the hook that converts Connect RPC events to TimelineEventData
 * and handles real-time event subscription for TimelineView.
 *
 * Success Criteria:
 * - SC-1: TimelineView subscribes to event stream on mount, unsubscribes on unmount
 * - SC-2: New events matching filters are prepended to timeline in real-time
 * - SC-3: Duplicate events (same ID) are not added to timeline
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import type { Event } from '@/gen/orc/v1/events_pb';

// Mock the useEvents hook
const mockOnEvent = vi.fn<(callback: (event: Event) => void) => () => void>();
vi.mock('@/hooks', async (importOriginal) => {
	const original = await importOriginal() as object;
	return {
		...original,
		useEvents: vi.fn(() => ({
			status: 'connected',
			subscribe: vi.fn(),
			subscribeGlobal: vi.fn(),
			disconnect: vi.fn(),
			isConnected: vi.fn().mockReturnValue(true),
			onEvent: mockOnEvent,
		})),
	};
});

// Import after mocks are set up
import { useTimelineEvents, convertEventToTimelineData } from './useTimelineEvents';

// Helper to create mock Event proto
function createMockEvent(
	id: string,
	options: {
		taskId?: string;
		payloadCase?: 'taskCreated' | 'phaseChanged' | 'activity' | 'error' | 'warning';
		taskTitle?: string;
		phase?: string;
	} = {}
): Event {
	const taskId = options.taskId ?? 'TASK-001';
	const now = new Date();
	
	// Create a minimal Event object matching the proto structure
	const baseEvent = {
		id,
		timestamp: {
			seconds: BigInt(Math.floor(now.getTime() / 1000)),
			nanos: (now.getTime() % 1000) * 1000000,
		},
		taskId,
	};
	
	switch (options.payloadCase) {
		case 'taskCreated':
			return {
				...baseEvent,
				payload: {
					case: 'taskCreated' as const,
					value: {
						taskId,
						title: options.taskTitle ?? 'Test Task',
						weight: 2, // SMALL
					},
				},
			} as unknown as Event;
		case 'phaseChanged':
			return {
				...baseEvent,
				payload: {
					case: 'phaseChanged' as const,
					value: {
						taskId,
						phaseId: 'phase-1',
						phaseName: options.phase ?? 'implement',
						status: 2, // COMPLETED
						iteration: 1,
					},
				},
			} as unknown as Event;
		case 'error':
			return {
				...baseEvent,
				payload: {
					case: 'error' as const,
					value: {
						taskId,
						error: 'Test error',
						phase: options.phase,
					},
				},
			} as unknown as Event;
		default:
			return {
				...baseEvent,
				payload: {
					case: 'phaseChanged' as const,
					value: {
						taskId,
						phaseId: 'phase-1',
						phaseName: options.phase ?? 'implement',
						status: 2,
						iteration: 1,
					},
				},
			} as unknown as Event;
	}
}

describe('useTimelineEvents', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockOnEvent.mockImplementation(() => vi.fn()); // Return unsubscribe function
	});

	describe('SC-1: Subscription lifecycle', () => {
		it('subscribes to event stream on mount', () => {
			renderHook(() => useTimelineEvents());
			
			expect(mockOnEvent).toHaveBeenCalled();
		});

		it('unsubscribes from event stream on unmount', () => {
			const unsubscribe = vi.fn();
			mockOnEvent.mockReturnValue(unsubscribe);
			
			const { unmount } = renderHook(() => useTimelineEvents());
			unmount();
			
			expect(unsubscribe).toHaveBeenCalled();
		});

		it('re-subscribes when filters change', () => {
			const { rerender } = renderHook(
				({ taskId }) => useTimelineEvents({ taskId }),
				{ initialProps: { taskId: undefined as string | undefined } }
			);
			
			expect(mockOnEvent).toHaveBeenCalledTimes(1);
			
			// Change filter
			rerender({ taskId: 'TASK-001' });
			
			// Should re-subscribe (unsubscribe old, subscribe new)
			expect(mockOnEvent).toHaveBeenCalledTimes(2);
		});
	});

	describe('SC-2: Event prepending', () => {
		it('adds new events via callback', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			const { result } = renderHook(() => useTimelineEvents());
			
			// Initially no events
			expect(result.current.newEvents).toHaveLength(0);
			
			// Simulate receiving an event
			act(() => {
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' }));
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(1);
				expect(result.current.newEvents[0].task_id).toBe('TASK-001');
			});
		});

		it('prepends events (newest first)', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			const { result } = renderHook(() => useTimelineEvents());
			
			// Add events in order
			act(() => {
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' }));
				eventCallback?.(createMockEvent('evt-2', { taskId: 'TASK-002' }));
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(2);
				// Newest should be first
				expect(result.current.newEvents[0].task_id).toBe('TASK-002');
				expect(result.current.newEvents[1].task_id).toBe('TASK-001');
			});
		});

		it('filters events by taskId when specified', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			const { result } = renderHook(() => useTimelineEvents({ taskId: 'TASK-001' }));
			
			act(() => {
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' }));
				eventCallback?.(createMockEvent('evt-2', { taskId: 'TASK-002' })); // Should be filtered
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(1);
				expect(result.current.newEvents[0].task_id).toBe('TASK-001');
			});
		});
	});

	describe('SC-3: Deduplication', () => {
		it('does not add duplicate events with same ID', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			const { result } = renderHook(() => useTimelineEvents());
			
			// Send same event twice
			act(() => {
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' }));
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' })); // Duplicate
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(1);
			});
		});

		it('excludes events already in existingIds set', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			// Pass existing IDs to hook
			const existingIds = new Set([123]); // Event ID 123 already loaded
			const { result } = renderHook(() => useTimelineEvents({ existingIds }));
			
			act(() => {
				eventCallback?.(createMockEvent('123', { taskId: 'TASK-001' })); // Should be filtered
				eventCallback?.(createMockEvent('456', { taskId: 'TASK-002' })); // Should be added
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(1);
				expect(result.current.newEvents[0].id).toBe(456);
			});
		});

		it('provides clearEvents function to reset', async () => {
			let eventCallback: ((event: Event) => void) | undefined;
			mockOnEvent.mockImplementation((callback) => {
				eventCallback = callback;
				return () => {};
			});
			
			const { result } = renderHook(() => useTimelineEvents());
			
			// Add an event
			act(() => {
				eventCallback?.(createMockEvent('evt-1', { taskId: 'TASK-001' }));
			});
			
			await waitFor(() => {
				expect(result.current.newEvents).toHaveLength(1);
			});
			
			// Clear events
			act(() => {
				result.current.clearEvents();
			});
			
			expect(result.current.newEvents).toHaveLength(0);
		});
	});
});

describe('convertEventToTimelineData', () => {
	it('converts taskCreated event correctly', () => {
		const event = createMockEvent('evt-1', {
			taskId: 'TASK-001',
			payloadCase: 'taskCreated',
			taskTitle: 'New Feature',
		});
		
		const result = convertEventToTimelineData(event);
		
		expect(result?.event_type).toBe('task_created');
		expect(result?.task_id).toBe('TASK-001');
		expect(result?.task_title).toBe('New Feature');
	});

	it('converts phaseChanged event to correct event_type', () => {
		const event = createMockEvent('evt-1', {
			taskId: 'TASK-001',
			payloadCase: 'phaseChanged',
			phase: 'implement',
		});
		
		const result = convertEventToTimelineData(event);
		
		// phaseChanged with status=COMPLETED should be phase_completed
		expect(result?.event_type).toBe('phase_completed');
		expect(result?.phase).toBe('implement');
	});

	it('converts error event correctly', () => {
		const event = createMockEvent('evt-1', {
			taskId: 'TASK-001',
			payloadCase: 'error',
			phase: 'implement',
		});
		
		const result = convertEventToTimelineData(event);
		
		expect(result?.event_type).toBe('error_occurred');
		expect(result?.task_id).toBe('TASK-001');
	});

	it('returns null for heartbeat events', () => {
		const event = {
			id: 'evt-1',
			timestamp: { seconds: BigInt(1234567890), nanos: 0 },
			payload: { case: 'heartbeat' as const, value: {} },
		} as unknown as Event;
		
		const result = convertEventToTimelineData(event);
		
		expect(result).toBeNull();
	});

	it('parses numeric event ID correctly', () => {
		const event = createMockEvent('12345', { taskId: 'TASK-001' });
		
		const result = convertEventToTimelineData(event);
		
		expect(result?.id).toBe(12345);
	});

	it('generates fallback ID for non-numeric event IDs', () => {
		const event = createMockEvent('non-numeric-id', { taskId: 'TASK-001' });
		
		const result = convertEventToTimelineData(event);
		
		// Should generate a timestamp-based fallback ID
		expect(typeof result?.id).toBe('number');
		expect(result?.id).toBeGreaterThan(0);
	});
});
