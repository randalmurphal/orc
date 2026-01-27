/**
 * useTimelineEvents - React hook for real-time timeline event streaming
 *
 * Subscribes to Connect RPC event stream and converts events to TimelineEventData
 * for display in TimelineView.
 *
 * Features:
 * - Automatic subscription/unsubscription lifecycle
 * - Filter by taskId
 * - Deduplication by event ID
 * - Prepends new events (newest first)
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { useEvents } from '@/hooks';
import type { Event } from '@/gen/orc/v1/events_pb';
import type { TimelineEventData } from '@/components/timeline/TimelineEvent';

interface UseTimelineEventsOptions {
	/** Filter events by task ID */
	taskId?: string;
	/** Set of existing event IDs to exclude (for deduplication) */
	existingIds?: Set<number>;
}

interface UseTimelineEventsResult {
	/** New events received since last clear */
	newEvents: TimelineEventData[];
	/** Clear all new events */
	clearEvents: () => void;
}

// Event payload type from proto
type EventPayload = {
	case: string | undefined;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	value?: any;
};

/**
 * Map proto event payload case to TimelineEventData event_type
 */
function mapPayloadToEventType(payload: EventPayload): TimelineEventData['event_type'] | null {
	switch (payload.case) {
		case 'taskCreated':
			return 'task_created';
		case 'taskUpdated':
			return 'task_started';
		case 'phaseChanged': {
			const status = payload.value?.status;
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
		case 'heartbeat':
			return null; // Filter out heartbeat events
		default:
			return 'task_started';
	}
}

/**
 * Extract data from proto event payload
 */
function extractEventData(payload: EventPayload): {
	taskTitle?: string;
	phase?: string;
	iteration?: number;
	source?: TimelineEventData['source'];
	data?: Record<string, unknown>;
} {
	const value = payload.value;
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

/**
 * Convert Timestamp to ISO string
 */
function timestampToIso(ts: { seconds: bigint; nanos: number } | undefined): string {
	if (!ts) return new Date().toISOString();
	const millis = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1000000);
	return new Date(millis).toISOString();
}

/**
 * Simple string hash function for generating deterministic numeric IDs from strings.
 * Uses djb2 algorithm - fast and produces good distribution.
 */
function hashString(str: string): number {
	let hash = 5381;
	for (let i = 0; i < str.length; i++) {
		hash = (hash * 33) ^ str.charCodeAt(i);
	}
	// Make positive and within safe integer range
	return Math.abs(hash >>> 0);
}

/**
 * Convert a proto Event to TimelineEventData
 *
 * @param event - The proto Event to convert
 * @returns TimelineEventData or null if event should be filtered (e.g., heartbeat)
 */
export function convertEventToTimelineData(event: Event): TimelineEventData | null {
	const payload = event.payload as EventPayload;
	const eventType = mapPayloadToEventType(payload);

	// Filter out heartbeat and other null-returning events
	if (eventType === null) {
		return null;
	}

	const eventData = extractEventData(payload);

	// Parse event ID - use numeric ID if parseable, otherwise use deterministic hash
	let eventId: number;
	if (event.id) {
		const parsed = parseInt(event.id, 10);
		if (!isNaN(parsed)) {
			eventId = parsed;
		} else {
			// Use deterministic hash for non-numeric IDs (enables deduplication)
			eventId = hashString(event.id);
		}
	} else {
		// No ID - generate timestamp-based fallback
		eventId = Date.now() + Math.floor(Math.random() * 1000);
	}

	return {
		id: eventId,
		task_id: event.taskId ?? '',
		task_title: eventData.taskTitle ?? '',
		phase: eventData.phase,
		iteration: eventData.iteration,
		event_type: eventType,
		source: eventData.source ?? 'executor',
		data: eventData.data ?? {},
		created_at: timestampToIso(event.timestamp),
	};
}

/**
 * Hook for subscribing to real-time timeline events
 *
 * @param options - Configuration options
 * @returns Object with newEvents array and clearEvents function
 */
export function useTimelineEvents(
	options: UseTimelineEventsOptions = {}
): UseTimelineEventsResult {
	const { taskId, existingIds } = options;
	const { onEvent } = useEvents();
	const [newEvents, setNewEvents] = useState<TimelineEventData[]>([]);

	// Track seen event IDs to prevent duplicates within newEvents
	const seenIdsRef = useRef<Set<number>>(new Set());

	// Reset seen IDs when existingIds changes
	useEffect(() => {
		seenIdsRef.current = new Set();
	}, [existingIds]);

	// Subscribe to events
	useEffect(() => {
		const handleEvent = (event: Event) => {
			// Convert event to timeline data
			const timelineEvent = convertEventToTimelineData(event);
			if (!timelineEvent) return; // Skip filtered events (heartbeat, etc.)

			// Filter by taskId if specified
			if (taskId && event.taskId !== taskId) return;

			// Check if already in existingIds (from initial load)
			if (existingIds?.has(timelineEvent.id)) return;

			// Check if already seen in this session
			if (seenIdsRef.current.has(timelineEvent.id)) return;

			// Track as seen
			seenIdsRef.current.add(timelineEvent.id);

			// Prepend event (newest first)
			setNewEvents((prev) => [timelineEvent, ...prev]);
		};

		const unsubscribe = onEvent(handleEvent);
		return unsubscribe;
	}, [onEvent, taskId, existingIds]);

	const clearEvents = useCallback(() => {
		setNewEvents([]);
		seenIdsRef.current = new Set();
	}, []);

	return { newEvents, clearEvents };
}
