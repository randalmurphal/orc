/**
 * EventProvider - React context provider for Connect RPC event streaming
 *
 * Split from useEvents.tsx to avoid react-refresh/only-export-components warnings.
 * Contains only the EventProvider component; hooks and types remain in useEvents.tsx.
 */

import {
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
import { useUIStore, useCurrentProjectId } from '@/stores';
import { EventContext, type EventContextValue } from './useEvents';

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
	const projectId = useCurrentProjectId();

	// Create subscription instance on mount
	useEffect(() => {
		subscriptionRef.current = new EventSubscription();

		// Subscribe to status changes
		const unsubStatus = subscriptionRef.current.onStatusChange((newStatus) => {
			setStatus(newStatus);
			setWsStatus(newStatus);
		});

		// Subscribe to events and route to stores
		const unsubEvents = subscriptionRef.current.on(handleEvent);

		// Cleanup on unmount
		return () => {
			unsubStatus();
			unsubEvents();
			subscriptionRef.current?.disconnect();
			subscriptionRef.current = null;
		};
	}, [setWsStatus]);

	// Connect/reconnect when project changes
	useEffect(() => {
		if (!autoConnect || !subscriptionRef.current) return;

		// Connect with current project ID filter
		const projectIds = projectId ? [projectId] : [];
		subscriptionRef.current.connect({ projectIds, includeHeartbeat: true });
	}, [autoConnect, projectId]);

	// Stable callbacks for context value
	const subscribe = useCallback((taskId: string) => {
		const projectIds = projectId ? [projectId] : [];
		subscriptionRef.current?.connect({ projectIds, taskId, includeHeartbeat: true });
	}, [projectId]);

	const subscribeGlobal = useCallback(() => {
		const projectIds = projectId ? [projectId] : [];
		subscriptionRef.current?.connect({ projectIds, includeHeartbeat: true });
	}, [projectId]);

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
