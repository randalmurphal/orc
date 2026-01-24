/**
 * Integration tests for TimelineView component
 *
 * TimelineView is the main container component that:
 * - Fetches events from the API
 * - Groups events by date
 * - Renders TimelineGroup components
 * - Handles real-time WebSocket updates
 * - Implements infinite scroll pagination
 *
 * Success Criteria covered:
 * - SC-1: Timeline page renders at /timeline route
 * - SC-2: Initial load fetches events from last 24 hours
 * - SC-3: Each event displays task ID, task title, event type, and relative timestamp
 * - SC-8: Scrolling to bottom triggers next page load
 * - SC-9: Infinite scroll respects current filters
 * - SC-10: WebSocket events prepend to timeline in real-time
 * - SC-12: Empty state shows when no events match filters
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The TimelineView.tsx file does not yet exist.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

// Import from the file we're going to create
// This will fail until implementation exists
import { TimelineView } from './TimelineView';
import type { TimelineEventData } from './TimelineEvent';

// Mock the API module
vi.mock('@/lib/api', () => ({
	getEvents: vi.fn(),
}));

// Mock WebSocket hook
const mockWsOn = vi.fn<(eventType: string, callback: (event: unknown) => void) => () => void>(() => () => {});
vi.mock('@/hooks', () => ({
	useWebSocket: vi.fn(() => ({
		on: mockWsOn,
		off: vi.fn(),
		status: 'connected',
	})),
}));

// Import mocked modules
import { getEvents } from '@/lib/api';
import { useWebSocket } from '@/hooks';

// Helper to create mock event
function createMockEvent(
	id: number,
	overrides: Partial<TimelineEventData> = {}
): TimelineEventData {
	return {
		id,
		task_id: `TASK-${String(id).padStart(3, '0')}`,
		task_title: `Test Task ${id}`,
		event_type: 'phase_completed',
		phase: 'implement',
		data: {},
		source: 'executor',
		created_at: new Date(Date.now() - id * 60000).toISOString(), // Each event 1 minute older
		...overrides,
	};
}

// Helper to create mock API response
function createMockResponse(
	events: TimelineEventData[],
	options: { hasMore?: boolean; total?: number } = {}
) {
	return {
		events,
		total: options.total ?? events.length,
		limit: 50,
		offset: 0,
		has_more: options.hasMore ?? false,
	};
}

// Helper to render TimelineView with necessary providers
function renderTimelineView(initialRoute = '/timeline') {
	return render(
		<MemoryRouter initialEntries={[initialRoute]}>
			<TimelineView />
		</MemoryRouter>
	);
}

describe('TimelineView', () => {
	beforeEach(() => {
		vi.resetAllMocks();
		// Re-setup the mocks after reset
		mockWsOn.mockImplementation(() => () => {});
		vi.mocked(useWebSocket).mockReturnValue({
			on: mockWsOn,
			off: vi.fn(),
			status: 'connected',
		} as unknown as ReturnType<typeof useWebSocket>);
		vi.useFakeTimers({ shouldAdvanceTime: true });
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	describe('initial render and loading state', () => {
		it('shows loading state while fetching events', async () => {
			// Delay the API response to see loading state
			vi.mocked(getEvents).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(createMockResponse([])), 100))
			);

			renderTimelineView();

			// Should show loading indicator
			expect(screen.getByTestId('timeline-loading')).toBeInTheDocument();
		});

		it('renders timeline header', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByRole('heading', { name: /timeline/i })).toBeInTheDocument();
			});
		});
	});

	describe('event fetching (SC-2)', () => {
		it('fetches events from last 24 hours on initial load', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView();

			await waitFor(() => {
				expect(getEvents).toHaveBeenCalled();
			});

			// Check that the API was called with a 'since' parameter for ~24h ago
			const callArgs = vi.mocked(getEvents).mock.calls[0][0];
			expect(callArgs).toHaveProperty('since');

			const sinceDate = new Date(callArgs!.since!);
			const now = new Date();
			const hoursAgo = (now.getTime() - sinceDate.getTime()) / (1000 * 60 * 60);

			// Should be roughly 24 hours (allow some margin)
			expect(hoursAgo).toBeGreaterThan(23);
			expect(hoursAgo).toBeLessThan(25);
		});

		it('fetches events with limit parameter', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView();

			await waitFor(() => {
				expect(getEvents).toHaveBeenCalled();
			});

			const callArgs = vi.mocked(getEvents).mock.calls[0][0];
			expect(callArgs).toHaveProperty('limit');
			expect(callArgs!.limit).toBeGreaterThan(0);
		});
	});

	describe('event display (SC-3)', () => {
		it('renders events with task ID and title', async () => {
			const events = [
				createMockEvent(1, { task_id: 'TASK-042', task_title: 'Add pagination' }),
			];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText('TASK-042')).toBeInTheDocument();
				expect(screen.getByText('Add pagination')).toBeInTheDocument();
			});
		});

		it('renders event type label', async () => {
			const events = [createMockEvent(1, { event_type: 'phase_completed', phase: 'implement' })];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText(/phase completed/i)).toBeInTheDocument();
			});
		});

		it('renders relative timestamp', async () => {
			const events = [createMockEvent(1)];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			// Should show relative time like "1m ago" or "just now"
			await waitFor(() => {
				const timeElement = screen.getByText(/ago|just now/i);
				expect(timeElement).toBeInTheDocument();
			});
		});
	});

	describe('date grouping (SC-4)', () => {
		it('groups events by date', async () => {
			const today = new Date();
			const yesterday = new Date(today);
			yesterday.setDate(yesterday.getDate() - 1);

			const events = [
				createMockEvent(1, { created_at: today.toISOString() }),
				createMockEvent(2, { created_at: yesterday.toISOString() }),
			];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			await waitFor(() => {
				// Should see "Today" and "Yesterday" group headers
				expect(screen.getByText(/today/i)).toBeInTheDocument();
				expect(screen.getByText(/yesterday/i)).toBeInTheDocument();
			});
		});

		it('shows event count in group headers', async () => {
			const events = [
				createMockEvent(1),
				createMockEvent(2),
				createMockEvent(3),
			];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			await waitFor(() => {
				// Should show count like "Today (3 events)"
				expect(screen.getByText(/3\s*events?/i)).toBeInTheDocument();
			});
		});
	});

	describe('empty state (SC-12)', () => {
		it('shows empty state when no events returned', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText(/no events found/i)).toBeInTheDocument();
			});
		});

		it('shows empty state with clear filters message when filters active', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView('/timeline?types=error_occurred');

			await waitFor(() => {
				expect(screen.getByText(/no events found/i)).toBeInTheDocument();
				expect(screen.getByText(/adjust.*filters/i)).toBeInTheDocument();
			});
		});

		it('shows different message for empty time range vs filtered', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			// Without filters - should show time period message
			renderTimelineView('/timeline');

			await waitFor(() => {
				expect(screen.getByText(/no events.*time period/i)).toBeInTheDocument();
			});
		});
	});

	describe('infinite scroll pagination (SC-8)', () => {
		it('loads more events when scrolling to bottom', async () => {
			// First page with has_more = true
			const firstPageEvents = Array.from({ length: 50 }, (_, i) => createMockEvent(i + 1));
			vi.mocked(getEvents)
				.mockResolvedValueOnce(createMockResponse(firstPageEvents, { hasMore: true, total: 100 }))
				.mockResolvedValueOnce(
					createMockResponse(
						Array.from({ length: 50 }, (_, i) => createMockEvent(i + 51)),
						{ hasMore: false, total: 100 }
					)
				);

			const { container } = renderTimelineView();

			// Wait for initial load and ensure hasMore state is set
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Wait a tick for React to update the event handlers with new hasMore state
			await vi.advanceTimersByTimeAsync(0);

			// Simulate scroll to bottom by mocking scroll properties
			const scrollContainer = container.querySelector('.timeline-view');
			if (scrollContainer) {
				// Mock scroll dimensions - JSDOM has scrollHeight = 0 by default
				Object.defineProperty(scrollContainer, 'scrollHeight', { value: 1000, configurable: true });
				Object.defineProperty(scrollContainer, 'scrollTop', { value: 800, writable: true, configurable: true });
				Object.defineProperty(scrollContainer, 'clientHeight', { value: 100, configurable: true });

				fireEvent.scroll(scrollContainer);
			}

			// Wait for second page to load
			await waitFor(() => {
				expect(getEvents).toHaveBeenCalledTimes(2);
			});
		});

		it('shows "Loading more..." indicator during pagination', async () => {
			const firstPageEvents = Array.from({ length: 50 }, (_, i) => createMockEvent(i + 1));

			vi.mocked(getEvents)
				.mockResolvedValueOnce(createMockResponse(firstPageEvents, { hasMore: true, total: 100 }))
				.mockImplementationOnce(
					() =>
						new Promise((resolve) =>
							setTimeout(
								() =>
									resolve(
										createMockResponse(
											Array.from({ length: 50 }, (_, i) => createMockEvent(i + 51))
										)
									),
								100
							)
						)
				);

			const { container } = renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Wait a tick for React to update the event handlers
			await vi.advanceTimersByTimeAsync(0);

			// Trigger scroll with mocked dimensions
			const scrollContainer = container.querySelector('.timeline-view');
			if (scrollContainer) {
				// Mock scroll dimensions for near-bottom position
				Object.defineProperty(scrollContainer, 'scrollHeight', { value: 1000, configurable: true });
				Object.defineProperty(scrollContainer, 'scrollTop', { value: 800, writable: true, configurable: true });
				Object.defineProperty(scrollContainer, 'clientHeight', { value: 100, configurable: true });

				fireEvent.scroll(scrollContainer);
			}

			// Should show loading indicator
			await waitFor(() => {
				expect(screen.getByText(/loading more/i)).toBeInTheDocument();
			});
		});

		it('shows "No more events" when all events loaded', async () => {
			const events = Array.from({ length: 25 }, (_, i) => createMockEvent(i + 1));
			vi.mocked(getEvents).mockResolvedValue(
				createMockResponse(events, { hasMore: false, total: 25 })
			);

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText(/no more events/i)).toBeInTheDocument();
			});
		});
	});

	describe('filters respect pagination (SC-9)', () => {
		it('passes filter params to pagination requests', async () => {
			const events = Array.from({ length: 50 }, (_, i) => createMockEvent(i + 1));
			vi.mocked(getEvents).mockResolvedValue(
				createMockResponse(events, { hasMore: true, total: 100 })
			);

			renderTimelineView('/timeline?types=phase_completed,error_occurred');

			await waitFor(() => {
				expect(getEvents).toHaveBeenCalled();
			});

			const callArgs = vi.mocked(getEvents).mock.calls[0][0];
			expect(callArgs!.types).toContain('phase_completed');
			expect(callArgs!.types).toContain('error_occurred');
		});

		it('resets pagination when filters change', async () => {
			const events = Array.from({ length: 50 }, (_, i) => createMockEvent(i + 1));
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events, { hasMore: true }));

			const { rerender } = renderTimelineView('/timeline?types=phase_completed');

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Change filters
			rerender(
				<MemoryRouter initialEntries={['/timeline?types=error_occurred']}>
					<TimelineView />
				</MemoryRouter>
			);

			// Should refetch from beginning (offset 0)
			await waitFor(() => {
				const lastCall = vi.mocked(getEvents).mock.calls[vi.mocked(getEvents).mock.calls.length - 1][0];
				expect(lastCall!.offset).toBe(0);
			});
		});
	});

	describe('WebSocket real-time updates (SC-10)', () => {
		it('subscribes to WebSocket events on mount', async () => {
			vi.mocked(getEvents).mockResolvedValue(createMockResponse([]));

			renderTimelineView();

			await waitFor(() => {
				expect(mockWsOn).toHaveBeenCalled();
			});
		});

		it('prepends new events received via WebSocket', async () => {
			let wsCallback: (event: unknown) => void = () => {};
			mockWsOn.mockImplementation((_eventType, callback) => {
				wsCallback = callback;
				return () => {};
			});

			const initialEvents = [createMockEvent(1)];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(initialEvents));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Simulate WebSocket event
			wsCallback({
				type: 'phase_completed',
				task_id: 'TASK-999',
				task_title: 'New Real-time Task',
				phase: 'implement',
				created_at: new Date().toISOString(),
			});

			// New event should appear at the top
			await waitFor(() => {
				expect(screen.getByText('TASK-999')).toBeInTheDocument();
				expect(screen.getByText('New Real-time Task')).toBeInTheDocument();
			});
		});

		it('shows reconnecting indicator when WebSocket disconnects', async () => {
			vi.mocked(useWebSocket).mockReturnValue({
				on: vi.fn(() => () => {}),
				off: vi.fn(),
				status: 'disconnected',
			} as unknown as ReturnType<typeof useWebSocket>);

			vi.mocked(getEvents).mockResolvedValue(createMockResponse([createMockEvent(1)]));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText(/reconnecting/i)).toBeInTheDocument();
			});
		});
	});

	describe('error handling', () => {
		it('shows error state when API fails', async () => {
			vi.mocked(getEvents).mockRejectedValue(new Error('Network error'));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText(/failed to load events/i)).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			vi.mocked(getEvents).mockRejectedValue(new Error('Network error'));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries fetch when retry button is clicked', async () => {
			vi.mocked(getEvents)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValueOnce(createMockResponse([createMockEvent(1)]));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});
		});

		it('shows error at bottom when pagination fails', async () => {
			const events = Array.from({ length: 50 }, (_, i) => createMockEvent(i + 1));
			vi.mocked(getEvents)
				.mockResolvedValueOnce(createMockResponse(events, { hasMore: true }))
				.mockRejectedValueOnce(new Error('Pagination failed'));

			const { container } = renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Wait a tick for React to update the event handlers
			await vi.advanceTimersByTimeAsync(0);

			// Trigger pagination with mocked scroll dimensions
			const scrollContainer = container.querySelector('.timeline-view');
			if (scrollContainer) {
				// Mock scroll dimensions for near-bottom position
				Object.defineProperty(scrollContainer, 'scrollHeight', { value: 1000, configurable: true });
				Object.defineProperty(scrollContainer, 'scrollTop', { value: 800, writable: true, configurable: true });
				Object.defineProperty(scrollContainer, 'clientHeight', { value: 100, configurable: true });

				fireEvent.scroll(scrollContainer);
			}

			await waitFor(() => {
				expect(screen.getByText(/failed to load more/i)).toBeInTheDocument();
			});
		});
	});

	describe('task link navigation', () => {
		it('renders task links that navigate to task detail', async () => {
			const events = [createMockEvent(1, { task_id: 'TASK-042' })];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			await waitFor(() => {
				const taskLink = screen.getByRole('link', { name: /TASK-042/i });
				expect(taskLink).toHaveAttribute('href', '/tasks/TASK-042');
			});
		});
	});

	describe('edge cases', () => {
		it('handles events with null phase gracefully', async () => {
			const events = [createMockEvent(1, { phase: undefined })];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			// Should not throw, should render event without phase
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});
		});

		it('handles events with null task_title gracefully', async () => {
			const events = [
				createMockEvent(1, {
					task_id: 'TASK-001',
					task_title: '', // Empty title
				}),
			];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			renderTimelineView();

			// Should show task_id as fallback
			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});
		});

		it('handles very long task titles with truncation', async () => {
			const longTitle = 'A'.repeat(200);
			const events = [createMockEvent(1, { task_title: longTitle })];
			vi.mocked(getEvents).mockResolvedValue(createMockResponse(events));

			const { container } = renderTimelineView();

			await waitFor(() => {
				// Title element should have truncation styles
				const titleElement = container.querySelector('.timeline-event-task-title');
				expect(titleElement).toBeInTheDocument();
			});
		});

		it('handles rapid WebSocket events without UI freeze', async () => {
			let wsCallback: (event: unknown) => void = () => {};
			mockWsOn.mockImplementation((_eventType, callback) => {
				wsCallback = callback;
				return () => {};
			});

			vi.mocked(getEvents).mockResolvedValue(createMockResponse([createMockEvent(1)]));

			renderTimelineView();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
			});

			// Send many events rapidly
			for (let i = 0; i < 20; i++) {
					wsCallback({
						type: 'phase_completed',
						task_id: `TASK-${100 + i}`,
						task_title: `Rapid Task ${i}`,
						created_at: new Date().toISOString(),
					});
			}

			// Should still render without blocking
			await waitFor(
				() => {
					expect(screen.getByText('TASK-001')).toBeInTheDocument();
				},
				{ timeout: 1000 }
			);
		});
	});
});
