/**
 * useEvents - React context and hooks for Connect RPC event streaming
 *
 * Replaces useWebSocket.tsx with Connect server streaming.
 * Provides EventProvider context and useEvents/useConnectionStatus hooks.
 */

import {
	createContext,
	useContext,
	useEffect,
	useRef,
	useState,
	useCallback,
	useMemo,
	type ReactNode,
} from 'react';
import {
	EventSubscription,
	type ConnectionStatus,
	handleEvent,
} from '@/lib/events';
import { useUIStore } from '@/stores';

// Re-export for convenience
export type { ConnectionStatus } from '@/lib/events';

interface EventContextValue {
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

const EventContext = createContext<EventContextValue | null>(null);

interface EventProviderProps {
	children: ReactNode;
	/** Whether to automatically connect on mount (default: true) */
	autoConnect?: boolean;
}

/**
 * EventProvider
 *
 * Wraps the app to provide event subscription functionality via context.
 * Manages a single EventSubscription instance and handles:
 * - Auto-connect on mount
 * - Connection status updates to UIStore
 * - Event routing to stores via handleEvent
 */
export function EventProvider({
	children,
	autoConnect = true,
}: EventProviderProps) {
	const subscriptionRef = useRef<EventSubscription | null>(null);
	const [status, setStatus] = useState<ConnectionStatus>('disconnected');
	const setWsStatus = useUIStore((state) => state.setWsStatus);

	// Create subscription instance and connect on mount
	useEffect(() => {
		subscriptionRef.current = new EventSubscription();

		// Subscribe to status changes
		const unsubStatus = subscriptionRef.current.onStatusChange((newStatus) => {
			setStatus(newStatus);
			setWsStatus(newStatus);
		});

		// Subscribe to events and route to stores
		const unsubEvents = subscriptionRef.current.on(handleEvent);

		// Auto-connect globally if enabled
		if (autoConnect) {
			subscriptionRef.current.connect({ includeHeartbeat: true });
		}

		// Cleanup on unmount
		return () => {
			unsubStatus();
			unsubEvents();
			subscriptionRef.current?.disconnect();
			subscriptionRef.current = null;
		};
	}, [autoConnect, setWsStatus]);

	// Stable callbacks for context value
	const subscribe = useCallback((taskId: string) => {
		subscriptionRef.current?.connect({ taskId, includeHeartbeat: true });
	}, []);

	const subscribeGlobal = useCallback(() => {
		subscriptionRef.current?.connect({ includeHeartbeat: true });
	}, []);

	const disconnect = useCallback(() => {
		subscriptionRef.current?.disconnect();
	}, []);

	const isConnected = useCallback(() => {
		return subscriptionRef.current?.isConnected() ?? false;
	}, []);

	const onEvent = useCallback(
		(handler: (event: import('@/gen/orc/v1/events_pb').Event) => void) => {
			if (!subscriptionRef.current) {
				// Return no-op unsubscribe if subscription not ready
				return () => {};
			}
			return subscriptionRef.current.on(handler);
		},
		[]
	);

	const contextValue = useMemo<EventContextValue>(
		() => ({
			status,
			subscribe,
			subscribeGlobal,
			disconnect,
			isConnected,
			onEvent,
		}),
		[status, subscribe, subscribeGlobal, disconnect, isConnected, onEvent]
	);

	return (
		<EventContext.Provider value={contextValue}>
			{children}
		</EventContext.Provider>
	);
}

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
	const { subscribe, subscribeGlobal } = useEvents();
	const [state, setState] = useState<import('@/gen/orc/v1/task_pb').ExecutionState | null>(null);
	const [transcript, setTranscript] = useState<TranscriptLine[]>([]);

	// Subscribe to task-specific events when taskId changes
	useEffect(() => {
		if (taskId) {
			subscribe(taskId);
		} else {
			subscribeGlobal();
		}

		// Reset state when task changes
		setState(null);
		setTranscript([]);
	}, [taskId, subscribe, subscribeGlobal]);

	// Get execution state from task store when available
	// This will be updated via the event handler when state_updated events arrive
	// For now, we return what's tracked locally
	// TODO: Connect to taskStore for execution state updates

	return { state, transcript };
}
