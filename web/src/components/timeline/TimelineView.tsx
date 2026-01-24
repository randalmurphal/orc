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
import { getEvents, listTasks, listInitiatives } from '@/lib/api';
import { useWebSocket } from '@/hooks';
import { TimelineGroup } from './TimelineGroup';
import { type TimelineEventData } from './TimelineEvent';
import { TimelineEmptyState } from './TimelineEmptyState';
import { TimelineFilters } from './TimelineFilters';
import { TimeRangeSelector, type TimeRange } from './TimeRangeSelector';
import { groupEventsByDate, getDateGroupLabel } from './utils';
import './TimelineView.css';

const PAGE_SIZE = 50;

// Calculate date for preset time ranges
function getPresetSince(preset: string): string | undefined {
	const now = new Date();
	switch (preset) {
		case 'today': {
			const today = new Date(now);
			today.setHours(0, 0, 0, 0);
			return today.toISOString();
		}
		case 'this_week': {
			const weekAgo = new Date(now);
			weekAgo.setDate(weekAgo.getDate() - 7);
			return weekAgo.toISOString();
		}
		case 'this_month': {
			const monthAgo = new Date(now);
			monthAgo.setMonth(monthAgo.getMonth() - 1);
			return monthAgo.toISOString();
		}
		case 'all':
			return undefined;
		default:
			return undefined;
	}
}

// Calculate 24 hours ago in ISO format (default)
function get24HoursAgo(): string {
	const date = new Date();
	date.setHours(date.getHours() - 24);
	return date.toISOString();
}

export function TimelineView() {
	const [searchParams, setSearchParams] = useSearchParams();
	const { on, status: wsStatus } = useWebSocket();

	// State
	const [events, setEvents] = useState<TimelineEventData[]>([]);
	const [isLoading, setIsLoading] = useState(true);
	const [isLoadingMore, setIsLoadingMore] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [paginationError, setPaginationError] = useState<string | null>(null);
	const [hasMore, setHasMore] = useState(false);
	const [offset, setOffset] = useState(0);

	// Filter dropdown data
	const [tasks, setTasks] = useState<Array<{ id: string; title: string }>>([]);
	const [initiatives, setInitiatives] = useState<Array<{ id: string; title: string }>>([]);

	// Refs for infinite scroll
	const scrollContainerRef = useRef<HTMLDivElement>(null);
	const loadMoreRef = useRef<HTMLDivElement>(null);
	const loadingRef = useRef(false);

	// Parse URL params for filters
	const filters = useMemo(() => {
		const types = searchParams.get('types')?.split(',').filter(Boolean) || [];
		const taskId = searchParams.get('task_id') || undefined;
		const initiativeId = searchParams.get('initiative_id') || undefined;
		const since = searchParams.get('since') || get24HoursAgo();
		const until = searchParams.get('until') || undefined;
		return { types, taskId, initiativeId, since, until };
	}, [searchParams]);

	// Determine current time range from URL params
	const timeRange = useMemo<TimeRange>(() => {
		const since = searchParams.get('since');
		const until = searchParams.get('until');
		
		if (since && until) {
			return { type: 'custom', since, until };
		}
		
		// Try to match against preset ranges
		const range = searchParams.get('range');
		if (range && ['today', 'this_week', 'this_month', 'all'].includes(range)) {
			return range as TimeRange;
		}
		
		// Default to today
		return 'today';
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

	// Fetch tasks and initiatives for filter dropdowns
	useEffect(() => {
		async function loadFilterData() {
			try {
				const [tasksRes, initiativesRes] = await Promise.all([
					listTasks(),
					listInitiatives()
				]);
				
				// Handle paginated or array response for tasks
				const tasksList = Array.isArray(tasksRes) ? tasksRes : tasksRes.tasks;
				setTasks(tasksList.map(t => ({ id: t.id, title: t.title })));
				setInitiatives(initiativesRes.map(i => ({ id: i.id, title: i.title })));
			} catch {
				// Silently fail - filter dropdowns will just be empty
			}
		}
		loadFilterData();
	}, []);

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
				types: filters.types.length > 0 ? filters.types : undefined,
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

	// Filter handlers
	const handleTypesChange = useCallback((types: string[]) => {
		const newParams = new URLSearchParams(searchParams);
		if (types.length > 0) {
			newParams.set('types', types.join(','));
		} else {
			newParams.delete('types');
		}
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	const handleTaskChange = useCallback((taskId: string | undefined) => {
		const newParams = new URLSearchParams(searchParams);
		if (taskId) {
			newParams.set('task_id', taskId);
		} else {
			newParams.delete('task_id');
		}
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	const handleInitiativeChange = useCallback((initiativeId: string | undefined) => {
		const newParams = new URLSearchParams(searchParams);
		if (initiativeId) {
			newParams.set('initiative_id', initiativeId);
		} else {
			newParams.delete('initiative_id');
		}
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	const handleClearAllFilters = useCallback(() => {
		const newParams = new URLSearchParams(searchParams);
		newParams.delete('types');
		newParams.delete('task_id');
		newParams.delete('initiative_id');
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	const handleTimeRangeChange = useCallback((range: TimeRange) => {
		const newParams = new URLSearchParams(searchParams);
		
		// Clear old range params
		newParams.delete('since');
		newParams.delete('until');
		newParams.delete('range');
		
		if (typeof range === 'object' && range.type === 'custom') {
			newParams.set('since', range.since);
			newParams.set('until', range.until);
		} else {
			// Preset range (TypeScript knows it's a string here)
			const presetRange = range as string;
			newParams.set('range', presetRange);
			const since = getPresetSince(presetRange);
			if (since) {
				newParams.set('since', since);
			}
		}
		
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	// Clear filters handler (passed to empty state)
	const handleClearFilters = useCallback(() => {
		// Navigate to timeline without filters
		setSearchParams(new URLSearchParams());
	}, [setSearchParams]);

	// Header content with filters (reused across all states)
	const headerContent = (
		<header className="timeline-view-header">
			<div className="timeline-view-header-title">
				<h1>Timeline</h1>
				{wsStatus !== 'connected' && (
					<span className="timeline-view-ws-status ws-status">
						Reconnecting...
					</span>
				)}
			</div>
			<div className="timeline-view-header-controls">
				<TimeRangeSelector
					value={timeRange}
					onChange={handleTimeRangeChange}
				/>
				<TimelineFilters
					selectedTypes={filters.types}
					selectedTaskId={filters.taskId}
					selectedInitiativeId={filters.initiativeId}
					tasks={tasks}
					initiatives={initiatives}
					onTypesChange={handleTypesChange}
					onTaskChange={handleTaskChange}
					onInitiativeChange={handleInitiativeChange}
					onClearAll={handleClearAllFilters}
				/>
			</div>
		</header>
	);

	// Render loading state
	if (isLoading) {
		return (
			<div className="timeline-view timeline-page">
				{headerContent}
				<div className="timeline-view-loading timeline-loading" data-testid="timeline-loading">
					<div className="timeline-loading-spinner" />
					<span>Loading events...</span>
				</div>
			</div>
		);
	}

	// Render error state
	if (error) {
		return (
			<div className="timeline-view timeline-page timeline-error">
				{headerContent}
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
			<div className="timeline-view timeline-page">
				{headerContent}
				<TimelineEmptyState
					hasFilters={hasActiveFilters}
					onClearFilters={handleClearFilters}
				/>
			</div>
		);
	}

	return (
		<div className="timeline-view timeline-page" ref={scrollContainerRef}>
			{headerContent}

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
					<div className="timeline-view-loading-more timeline-loading-more">
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
					<div className="timeline-view-end timeline-no-more">
						No more events
					</div>
				)}

				{/* Scroll trigger element for infinite scroll (hidden) */}
				<div ref={loadMoreRef} className="timeline-view-scroll-trigger" />
			</div>
		</div>
	);
}
