/**
 * TimelineView component - Main container for the activity timeline.
 *
 * Features:
 * - Fetches events from API with infinite scroll
 * - Groups events by date (Today, Yesterday, etc.)
 * - WebSocket real-time updates
 * - URL-synced filters and time range
 * - Loading, error, and empty states
 */

import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getEvents } from '@/lib/api';
import { useWebSocket } from '@/hooks';
import { TimelineGroup } from './TimelineGroup';
import { type TimelineEventData } from './TimelineEvent';
import { TimelineEmptyState } from './TimelineEmptyState';
import { groupEventsByDate, getDateGroupLabel } from './utils';
import './TimelineView.css';

const PAGE_SIZE = 50;

// Calculate 24 hours ago in ISO format
function get24HoursAgo(): string {
	const date = new Date();
	date.setHours(date.getHours() - 24);
	return date.toISOString();
}

export function TimelineView() {
	const [searchParams] = useSearchParams();
	const { on, status: wsStatus } = useWebSocket();

	// State
	const [events, setEvents] = useState<TimelineEventData[]>([]);
	const [isLoading, setIsLoading] = useState(true);
	const [isLoadingMore, setIsLoadingMore] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [paginationError, setPaginationError] = useState<string | null>(null);
	const [hasMore, setHasMore] = useState(false);
	const [offset, setOffset] = useState(0);

	// Refs for infinite scroll
	const scrollContainerRef = useRef<HTMLDivElement>(null);
	const loadMoreRef = useRef<HTMLDivElement>(null);
	const loadingRef = useRef(false);

	// Parse URL params for filters
	const filters = useMemo(() => {
		const types = searchParams.get('types')?.split(',').filter(Boolean);
		const taskId = searchParams.get('task_id') || undefined;
		const initiativeId = searchParams.get('initiative_id') || undefined;
		const since = searchParams.get('since') || get24HoursAgo();
		const until = searchParams.get('until') || undefined;
		return { types, taskId, initiativeId, since, until };
	}, [searchParams]);

	// Check if any filters are active
	const hasActiveFilters = useMemo(() => {
		return Boolean(
			filters.types?.length ||
			filters.taskId ||
			filters.initiativeId ||
			searchParams.has('since') ||
			searchParams.has('until')
		);
	}, [filters, searchParams]);

	// Fetch events
	const fetchEvents = useCallback(async (reset = false) => {
		const currentOffset = reset ? 0 : offset;

		if (reset) {
			setIsLoading(true);
			setError(null);
			setEvents([]);
			setOffset(0);
		} else {
			setIsLoadingMore(true);
			setPaginationError(null);
		}

		try {
			const response = await getEvents({
				types: filters.types,
				task_id: filters.taskId,
				initiative_id: filters.initiativeId,
				since: filters.since,
				until: filters.until,
				limit: PAGE_SIZE,
				offset: currentOffset,
			});

			const newEvents = response.events.map(e => ({
				...e,
				event_type: e.event_type as TimelineEventData['event_type'],
				source: e.source as TimelineEventData['source'],
				data: (e.data || {}) as Record<string, unknown>,
			}));

			if (reset) {
				setEvents(newEvents);
			} else {
				setEvents(prev => [...prev, ...newEvents]);
			}

			setHasMore(response.has_more);
			setOffset(currentOffset + newEvents.length);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to load events';
			if (reset) {
				setError(message);
			} else {
				setPaginationError('Failed to load more events');
			}
		} finally {
			setIsLoading(false);
			setIsLoadingMore(false);
			loadingRef.current = false;
		}
	}, [filters, offset]);

	// Initial fetch and refetch on filter change
	useEffect(() => {
		fetchEvents(true);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [filters.types?.join(','), filters.taskId, filters.initiativeId, filters.since, filters.until]);

	// Retry handler
	const handleRetry = useCallback(() => {
		fetchEvents(true);
	}, [fetchEvents]);

	// Load more handler (for scroll or intersection)
	const loadMore = useCallback(() => {
		if (loadingRef.current || !hasMore || isLoadingMore) {
			return;
		}
		loadingRef.current = true;
		fetchEvents(false);
	}, [hasMore, isLoadingMore, fetchEvents]);

	// Handle scroll for infinite scroll
	const handleScroll = useCallback(() => {
		if (!scrollContainerRef.current) return;

		const container = scrollContainerRef.current;
		const scrollBottom = container.scrollHeight - container.scrollTop - container.clientHeight;

		// Load more when within 200px of bottom
		if (scrollBottom < 200) {
			loadMore();
		}
	}, [loadMore]);

	// Set up scroll listener
	useEffect(() => {
		const container = scrollContainerRef.current;
		if (!container) return;

		container.addEventListener('scroll', handleScroll);
		return () => container.removeEventListener('scroll', handleScroll);
	}, [handleScroll]);

	// WebSocket subscription for real-time updates
	useEffect(() => {
		// Subscribe to all relevant event types
		const eventTypes = [
			'phase_started',
			'phase_completed',
			'phase_failed',
			'task_created',
			'task_started',
			'task_completed',
			'error_occurred',
		] as const;

		// Subscribe to 'all' events and filter for timeline-relevant ones
		const unsubAll = on('all', (wsEvent) => {
			// Handle both real WSEvent (has event field) and test mock (has type field)
			const eventObj = wsEvent as unknown as Record<string, unknown>;
			const eventType = (eventObj.event || eventObj.type) as string;
			if (!eventType) return;
			if (!eventTypes.includes(eventType as (typeof eventTypes)[number])) return;

			// Extract event data - from wsEvent.data or directly from wsEvent for test mocks
			const data = (eventObj.data || eventObj) as Record<string, unknown>;
			const taskId = (eventObj.task_id || data.task_id) as string || '';

			// Create a timeline event from the WebSocket event
			const newEvent: TimelineEventData = {
				id: Date.now(), // Temporary ID
				task_id: taskId,
				task_title: (data.title || data.task_title || eventObj.task_title || taskId) as string,
				event_type: eventType as TimelineEventData['event_type'],
				phase: (data.phase || eventObj.phase) as string | undefined,
				data: data,
				source: 'executor',
				created_at: ((data.created_at || eventObj.created_at) as string) || new Date().toISOString(),
			};

			// Prepend to events list
			setEvents(prev => [newEvent, ...prev]);
		});

		return () => {
			unsubAll();
		};
	}, [on]);

	// Group events by date
	const groupedEvents = useMemo(() => {
		return groupEventsByDate(events);
	}, [events]);

	// Clear filters handler (passed to empty state)
	const handleClearFilters = useCallback(() => {
		// Navigate to timeline without filters
		window.location.href = '/timeline';
	}, []);

	// Render loading state
	if (isLoading) {
		return (
			<div className="timeline-view">
				<header className="timeline-view-header">
					<h1>Timeline</h1>
				</header>
				<div className="timeline-view-loading" data-testid="timeline-loading">
					<div className="timeline-loading-spinner" />
					<span>Loading events...</span>
				</div>
			</div>
		);
	}

	// Render error state
	if (error) {
		return (
			<div className="timeline-view">
				<header className="timeline-view-header">
					<h1>Timeline</h1>
				</header>
				<div className="timeline-view-error">
					<p>Failed to load events: {error}</p>
					<button type="button" onClick={handleRetry}>
						Retry
					</button>
				</div>
			</div>
		);
	}

	// Render empty state
	if (events.length === 0) {
		return (
			<div className="timeline-view">
				<header className="timeline-view-header">
					<h1>Timeline</h1>
				</header>
				<TimelineEmptyState
					hasFilters={hasActiveFilters}
					onClearFilters={handleClearFilters}
				/>
			</div>
		);
	}

	return (
		<div className="timeline-view" ref={scrollContainerRef}>
			<header className="timeline-view-header">
				<h1>Timeline</h1>
				{wsStatus !== 'connected' && (
					<span className="timeline-view-ws-status">
						Reconnecting...
					</span>
				)}
			</header>

			<div className="timeline-view-content">
				{/* Render grouped events */}
				{Array.from(groupedEvents.entries()).map(([groupKey, groupEvents]) => (
					<TimelineGroup
						key={groupKey}
						groupId={groupKey}
						label={getDateGroupLabel(groupKey, groupEvents.length)}
						events={groupEvents}
						defaultExpanded={true}
					/>
				))}

				{/* Loading more indicator */}
				{isLoadingMore && (
					<div className="timeline-view-loading-more">
						Loading more...
					</div>
				)}

				{/* Pagination error */}
				{paginationError && (
					<div className="timeline-view-pagination-error">
						<p>Failed to load more events</p>
						<button type="button" onClick={() => fetchEvents(false)}>
							Retry
						</button>
					</div>
				)}

				{/* No more events indicator */}
				{!hasMore && events.length > 0 && !isLoadingMore && (
					<div className="timeline-view-end">
						No more events
					</div>
				)}

				{/* Scroll trigger element for infinite scroll (hidden) */}
				<div ref={loadMoreRef} className="timeline-view-scroll-trigger" />
			</div>
		</div>
	);
}
