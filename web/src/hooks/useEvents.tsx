/**
 * useEvents - React context and hooks for Connect RPC event streaming
 *
 * Provides useEvents/useConnectionStatus/useTaskSubscription hooks,
 * types, and the EventContext definition.
 * The EventProvider component lives in EventProvider.tsx.
 */

import {
	createContext,
	useContext,
	useEffect,
	useState,
} from 'react';
import type { ConnectionStatus } from '@/lib/events';
import { useTaskState } from '@/stores/taskStore';
import { ActivityState } from '@/gen/orc/v1/events_pb';

// Re-export for convenience
export type { ConnectionStatus } from '@/lib/events';

export interface EventContextValue {
	/** Current connection status */
	status: ConnectionStatus;
	/** Subscribe to a specific task's events */
	subscribe: (taskId: string) => void;
	/** Subscribe to all events (global subscription) */
	subscribeGlobal: () => void;
	/** Disconnect from event stream */
	disconnect: () => void;
	/** Check if currently connected */
	isConnected: () => boolean;
	/** Register a custom event handler. Returns unsubscribe function. */
	onEvent: (handler: (event: import('@/gen/orc/v1/events_pb').Event) => void) => () => void;
}

export const EventContext = createContext<EventContextValue | null>(null);

/**
 * useEvents hook
 *
 * Access event subscription functionality from any component.
 * Must be used within an EventProvider.
 */
export function useEvents(): EventContextValue {
	const context = useContext(EventContext);
	if (!context) {
		throw new Error('useEvents must be used within an EventProvider');
	}
	return context;
}

/**
 * useConnectionStatus hook
 *
 * Simple hook to get the current connection status.
 */
export function useConnectionStatus(): ConnectionStatus {
	const { status } = useEvents();
	return status;
}

/**
 * TranscriptLine - streaming transcript data
 */
export interface TranscriptLine {
	content: string;
	timestamp: string;
	type: 'prompt' | 'response' | 'tool' | 'error';
	phase?: string;
	tokens?: {
		input: number;
		output: number;
	};
}

/**
 * Global task ID constant for non-task-specific subscriptions
 */
export const GLOBAL_TASK_ID = '';

/**
 * useTaskSubscription hook
 *
 * Subscribe to task-specific events and track execution state + streaming transcript.
 * When taskId is undefined or empty, uses global subscription.
 *
 * @param taskId - The task ID to subscribe to
 * @returns Object with state and transcript array
 */
export function useTaskSubscription(taskId: string | undefined): {
	state: import('@/gen/orc/v1/task_pb').ExecutionState | null;
	transcript: TranscriptLine[];
} {
	const { subscribe, subscribeGlobal, onEvent } = useEvents();
	const [transcript, setTranscript] = useState<TranscriptLine[]>([]);

	// Get execution state from taskStore (updated via event handlers)
	const state = useTaskState(taskId ?? '') ?? null;

	// Handle transcript events
	useEffect(() => {
		if (!taskId) return;

		const unsubscribe = onEvent((event) => {
			// Check if this is a transcript chunk event for our task
			if (
				event.payload?.case === 'activity' &&
				event.payload.value.activity === ActivityState.STREAMING &&
				event.taskId === taskId
			) {
				try {
					// Parse transcript data from event details
					const details = event.payload.value.details;
					if (details) {
						const transcriptData = JSON.parse(details);

						// Convert to TranscriptLine format
						const line: TranscriptLine = {
							content: transcriptData.content || '',
							timestamp: transcriptData.timestamp || new Date().toISOString(),
							type: transcriptData.type || 'response',
							phase: transcriptData.phase,
							tokens: transcriptData.tokens ? {
								input: transcriptData.tokens.input || 0,
								output: transcriptData.tokens.output || 0,
							} : undefined,
						};

						// Add to transcript
						setTranscript(prev => [...prev, line]);
					}
				} catch (e) {
					// Ignore malformed transcript data
					console.warn('Failed to parse transcript event data:', e);
				}
			}
		});

		return unsubscribe;
	}, [taskId, onEvent]);

	// Subscribe to task-specific events when taskId changes
	useEffect(() => {
		if (taskId) {
			subscribe(taskId);
		} else {
			subscribeGlobal();
		}

		// Reset transcript when task changes
		setTranscript([]);
	}, [taskId, subscribe, subscribeGlobal]);

	return { state, transcript };
}
