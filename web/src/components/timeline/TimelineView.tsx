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
import { create } from '@bufbuild/protobuf';
import { TimestampSchema, type Timestamp } from '@bufbuild/protobuf/wkt';
import { eventClient, taskClient, initiativeClient } from '@/lib/client';
import { GetEventsRequestSchema } from '@/gen/orc/v1/events_pb';
import { ListTasksRequestSchema } from '@/gen/orc/v1/task_pb';
import { ListInitiativesRequestSchema } from '@/gen/orc/v1/initiative_pb';
import { PageRequestSchema } from '@/gen/orc/v1/common_pb';
import { useConnectionStatus, useTimelineEvents } from '@/hooks';
import { useCurrentProjectId } from '@/stores/projectStore';
import { TimelineGroup } from './TimelineGroup';
import { type TimelineEventData } from './TimelineEvent';
import { TimelineEmptyState } from './TimelineEmptyState';
import { TimelineFilters } from './TimelineFilters';
import { TimeRangeSelector, getDateRange, type TimeRange, type CustomDateRange } from './TimeRangeSelector';
import { groupEventsByDate, getDateGroupLabel } from './utils';
import { Button } from '@/components/ui/Button';
import './TimelineView.css';

// Helper to convert ISO string to Timestamp
function isoToTimestamp(iso: string): Timestamp {
	const date = new Date(iso);
	return create(TimestampSchema, {
		seconds: BigInt(Math.floor(date.getTime() / 1000)),
		nanos: (date.getTime() % 1000) * 1000000,
	});
}

// Helper to convert Timestamp to ISO string
function timestampToIso(ts: Timestamp | undefined): string {
	if (!ts) return new Date().toISOString();
	const millis = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1000000);
	return new Date(millis).toISOString();
}

// Event payload type from proto
type EventPayload = {
	case: string | undefined;
	value?: unknown;
};

// Map proto event payload case to TimelineEventData event_type
function mapPayloadToEventType(payload: EventPayload): TimelineEventData['event_type'] {
	switch (payload.case) {
		case 'taskCreated':
			return 'task_created';
		case 'taskUpdated':
			return 'task_started'; // Map to closest equivalent
		case 'phaseChanged': {
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
			const status = (payload.value as any)?.status;
			if (status === 1) return 'phase_started'; // RUNNING
			if (status === 2) return 'phase_completed'; // COMPLETED
			if (status === 3) return 'phase_failed'; // FAILED
			return 'phase_started';
		}
		case 'activity':
			return 'activity_changed';
		case 'error':
			return 'error_occurred';
		case 'warning':
			return 'warning_issued';
		case 'tokensUpdated':
			return 'token_update';
		case 'decisionRequired':
		case 'decisionResolved':
			return 'gate_decision';
		default:
			return 'task_started';
	}
}

// Extract data from proto event payload
function extractEventData(payload: EventPayload): {
	taskTitle?: string;
	phase?: string;
	iteration?: number;
	source?: TimelineEventData['source'];
	data?: Record<string, unknown>;
} {
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const value = payload.value as any;
	if (!value) return {};

	switch (payload.case) {
		case 'taskCreated':
			return {
				taskTitle: value.title,
				source: 'executor',
				data: { weight: value.weight },
			};
		case 'phaseChanged':
			return {
				phase: value.phaseName,
				iteration: value.iteration,
				source: 'executor',
				data: {
					status: value.status,
					commitSha: value.commitSha,
					error: value.error,
				},
			};
		case 'activity':
			return {
				phase: value.phaseId,
				source: 'executor',
				data: {
					activity: value.activity,
					details: value.details,
				},
			};
		case 'error':
			return {
				phase: value.phase,
				source: 'executor',
				data: {
					error: value.error,
					stackTrace: value.stackTrace,
				},
			};
		case 'warning':
			return {
				phase: value.phase,
				source: 'executor',
				data: { message: value.message },
			};
		case 'tokensUpdated':
			return {
				phase: value.phaseId,
				source: 'executor',
				data: { tokens: value.tokens },
			};
		case 'decisionRequired':
			return {
				taskTitle: value.taskTitle,
				phase: value.phase,
				source: 'executor',
				data: {
					decisionId: value.decisionId,
					gateType: value.gateType,
					question: value.question,
					context: value.context,
				},
			};
		case 'decisionResolved':
			return {
				phase: value.phase,
				source: 'executor',
				data: {
					decisionId: value.decisionId,
					approved: value.approved,
					reason: value.reason,
					resolvedBy: value.resolvedBy,
				},
			};
		default:
			return { source: 'executor' };
	}
}

const PAGE_SIZE = 50;

export function TimelineView() {
	const [searchParams, setSearchParams] = useSearchParams();
	const wsStatus = useConnectionStatus();
	const projectId = useCurrentProjectId();

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

	// Time range state
	const [timeRange, setTimeRange] = useState<TimeRange>(() => {
		const range = searchParams.get('range');
		if (range && ['today', 'yesterday', 'this_week', 'this_month', 'custom'].includes(range)) {
			return range as TimeRange;
		}
		return 'today';
	});
	const [customRange, setCustomRange] = useState<CustomDateRange>(() => {
		const now = new Date();
		const weekAgo = new Date(now);
		weekAgo.setDate(weekAgo.getDate() - 7);
		return { start: weekAgo, end: now };
	});

	// Refs for infinite scroll
	const scrollContainerRef = useRef<HTMLDivElement>(null);
	const loadMoreRef = useRef<HTMLDivElement>(null);
	const loadingRef = useRef(false);

	// Parse URL params for filters
	const filters = useMemo(() => {
		const types = searchParams.get('types')?.split(',').filter(Boolean) || [];
		const taskId = searchParams.get('task_id') || undefined;
		const initiativeId = searchParams.get('initiative_id') || undefined;

		// Get date range from current timeRange setting
		const dateRange = getDateRange(timeRange, customRange);
		const since = dateRange.since.toISOString();
		const until = dateRange.until.toISOString();

		return { types, taskId, initiativeId, since, until };
	}, [searchParams, timeRange, customRange]);

	// Create set of existing event IDs for deduplication
	const existingIds = useMemo(() => new Set(events.map((e) => e.id)), [events]);

	// Subscribe to real-time events
	const { newEvents, clearEvents } = useTimelineEvents({
		taskId: filters.taskId,
		existingIds,
	});

	// Merge new events into events state
	useEffect(() => {
		if (newEvents.length > 0) {
			setEvents((prev) => {
				// Prepend new events, maintaining deduplication
				const existingSet = new Set(prev.map((e) => e.id));
				const uniqueNew = newEvents.filter((e) => !existingSet.has(e.id));
				return [...uniqueNew, ...prev];
			});
			clearEvents();
		}
	}, [newEvents, clearEvents]);

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
		if (!projectId) return;
		let mounted = true;
		async function loadFilterData() {
			try {
				const [tasksRes, initiativesRes] = await Promise.all([
					taskClient.listTasks(create(ListTasksRequestSchema, { projectId: projectId ?? undefined })),
					initiativeClient.listInitiatives(create(ListInitiativesRequestSchema, { projectId: projectId ?? undefined }))
				]);

				if (!mounted) return;
				setTasks(tasksRes.tasks.map(t => ({ id: t.id, title: t.title })));
				setInitiatives(initiativesRes.initiatives.map(i => ({ id: i.id, title: i.title })));
			} catch {
				// Silently fail - filter dropdowns will just be empty
			}
		}
		loadFilterData();
		return () => { mounted = false; };
	}, [projectId]);

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
			const request = create(GetEventsRequestSchema, {
				projectId: projectId ?? undefined,
				page: create(PageRequestSchema, {
					page: Math.floor(currentOffset / PAGE_SIZE) + 1,
					limit: PAGE_SIZE,
				}),
				taskId: filters.taskId,
				initiativeId: filters.initiativeId,
				since: filters.since ? isoToTimestamp(filters.since) : undefined,
				until: filters.until ? isoToTimestamp(filters.until) : undefined,
				types: filters.types.length > 0 ? filters.types : [],
			});

			const response = await eventClient.getEvents(request);

			// Map proto Event to TimelineEventData
			// Use proto event ID for stable identification and deduplication
			const newEvents: TimelineEventData[] = response.events.map((e, idx) => {
				const eventType = mapPayloadToEventType(e.payload);
				const eventData = extractEventData(e.payload);
				// Use proto event ID if available, fallback to offset-based ID
				const eventId = e.id ? parseInt(e.id, 10) : currentOffset + idx;

				return {
					id: eventId,
					task_id: e.taskId ?? '',
					task_title: eventData.taskTitle ?? '',
					phase: eventData.phase,
					iteration: eventData.iteration,
					event_type: eventType,
					source: eventData.source ?? 'executor',
					data: eventData.data ?? {},
					created_at: timestampToIso(e.timestamp),
				};
			});

			if (reset) {
				// Deduplicate events in case backend returns duplicates in single response
				const seenIds = new Set<number>();
				const dedupedEvents = newEvents.filter(e => {
					if (seenIds.has(e.id)) return false;
					seenIds.add(e.id);
					return true;
				});
				setEvents(dedupedEvents);
			} else {
				// Deduplicate events when appending (same event may appear in overlapping pages)
				setEvents(prev => {
					const existingIds = new Set(prev.map(e => e.id));
					const dedupedNew = newEvents.filter(e => !existingIds.has(e.id));
					return [...prev, ...dedupedNew];
				});
			}

			setHasMore(response.page?.hasMore ?? false);
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
	}, [filters, offset, projectId]);

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
		setTimeRange(range);
		
		// Update URL params
		const newParams = new URLSearchParams(searchParams);
		newParams.set('range', range);
		setSearchParams(newParams);
	}, [searchParams, setSearchParams]);

	const handleCustomRangeChange = useCallback((range: CustomDateRange) => {
		setCustomRange(range);
	}, []);

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
					customRange={customRange}
					onCustomRangeChange={handleCustomRangeChange}
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
					<Button variant="secondary" onClick={handleRetry}>
						Retry
					</Button>
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
						<Button variant="secondary" size="sm" onClick={() => fetchEvents(false)}>
							Retry
						</Button>
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
