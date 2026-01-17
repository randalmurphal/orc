/**
 * WebSocket React integration
 *
 * Provides WebSocketProvider context and hooks for real-time task updates.
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
import { OrcWebSocket, GLOBAL_TASK_ID } from '@/lib/websocket';
import type {
	ConnectionStatus,
	WSEventType,
	WSEvent,
	WSError,
	WSCallback,
	Task,
	TaskState,
	Initiative,
	ActivityUpdate,
	ActivityState,
} from '@/lib/types';
import { useUIStore, useTaskStore, useInitiativeStore, toast } from '@/stores';

export { GLOBAL_TASK_ID };

// Context value type
interface WebSocketContextValue {
	/** Current connection status */
	status: ConnectionStatus;
	/** Subscribe to a specific task's events */
	subscribe: (taskId: string) => void;
	/** Unsubscribe from current task */
	unsubscribe: () => void;
	/** Subscribe to all task events (global) */
	subscribeGlobal: () => void;
	/** Add event listener for specific event types */
	on: (eventType: WSEventType | 'all', callback: WSCallback) => () => void;
	/** Send a command to control a task */
	command: (taskId: string, action: 'pause' | 'resume' | 'cancel') => void;
	/** Check if connected */
	isConnected: () => boolean;
	/** Get current subscribed task ID */
	getTaskId: () => string | null;
}

const WebSocketContext = createContext<WebSocketContextValue | null>(null);

interface WebSocketProviderProps {
	children: ReactNode;
	/** Base URL for WebSocket connection (optional, defaults to window.location.host) */
	baseUrl?: string;
	/** Whether to automatically connect on mount (default: true) */
	autoConnect?: boolean;
	/** Whether to automatically subscribe to global events (default: true) */
	autoSubscribeGlobal?: boolean;
}

/**
 * WebSocketProvider
 *
 * Wraps the app to provide WebSocket functionality via context.
 * Manages a single WebSocket instance and handles:
 * - Auto-connect on mount
 * - Auto-subscribe to global events
 * - Connection status updates to UIStore
 * - Event routing to TaskStore
 */
export function WebSocketProvider({
	children,
	baseUrl,
	autoConnect = true,
	autoSubscribeGlobal = true,
}: WebSocketProviderProps) {
	const wsRef = useRef<OrcWebSocket | null>(null);
	const [status, setStatus] = useState<ConnectionStatus>('disconnected');
	const setWsStatus = useUIStore((state) => state.setWsStatus);

	// Create WebSocket instance on mount
	useEffect(() => {
		wsRef.current = new OrcWebSocket(baseUrl);

		// Subscribe to status changes
		const unsubStatus = wsRef.current.onStatusChange((newStatus) => {
			setStatus(newStatus);
			setWsStatus(newStatus);
		});

		// Set up global event handler
		const unsubEvents = wsRef.current.on('all', (event) => {
			if ('event' in event) {
				handleWSEvent(event);
			} else if (event.type === 'error') {
				handleWSError(event);
			}
		});

		// Auto-connect if enabled
		if (autoConnect) {
			if (autoSubscribeGlobal) {
				wsRef.current.setPrimarySubscription(GLOBAL_TASK_ID);
				wsRef.current.connect(GLOBAL_TASK_ID);
			} else {
				wsRef.current.connect();
			}
		}

		// Cleanup on unmount
		return () => {
			unsubStatus();
			unsubEvents();
			wsRef.current?.disconnect();
			wsRef.current = null;
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [baseUrl, autoConnect, autoSubscribeGlobal]);

	// Stable callback refs for context value
	const subscribe = useCallback((taskId: string) => {
		wsRef.current?.subscribe(taskId);
	}, []);

	const unsubscribe = useCallback(() => {
		wsRef.current?.unsubscribe();
	}, []);

	const subscribeGlobal = useCallback(() => {
		wsRef.current?.subscribeGlobal();
	}, []);

	const on = useCallback((eventType: WSEventType | 'all', callback: WSCallback) => {
		// wsRef.current is accessed at call time, not at callback creation time
		if (wsRef.current) {
			return wsRef.current.on(eventType, callback);
		}
		// If no WebSocket yet, return a no-op cleanup function
		return () => {};
	}, []);

	const command = useCallback((taskId: string, action: 'pause' | 'resume' | 'cancel') => {
		wsRef.current?.command(taskId, action);
	}, []);

	const isConnected = useCallback(() => {
		return wsRef.current?.isConnected() ?? false;
	}, []);

	const getTaskId = useCallback(() => {
		return wsRef.current?.getTaskId() ?? null;
	}, []);

	const contextValue = useMemo<WebSocketContextValue>(
		() => ({
			status,
			subscribe,
			unsubscribe,
			subscribeGlobal,
			on,
			command,
			isConnected,
			getTaskId,
		}),
		[status, subscribe, unsubscribe, subscribeGlobal, on, command, isConnected, getTaskId]
	);

	return <WebSocketContext.Provider value={contextValue}>{children}</WebSocketContext.Provider>;
}

/**
 * useWebSocket hook
 *
 * Access WebSocket functionality from any component.
 * Must be used within a WebSocketProvider.
 */
export function useWebSocket(): WebSocketContextValue {
	const context = useContext(WebSocketContext);
	if (!context) {
		throw new Error('useWebSocket must be used within a WebSocketProvider');
	}
	return context;
}

/**
 * Handle incoming WebSocket events
 * Routes events to the appropriate stores
 */
function handleWSEvent(event: WSEvent): void {
	const { event: eventType, task_id, data } = event;
	const taskStore = useTaskStore.getState();

	switch (eventType) {
		case 'state': {
			// Update task execution state
			const state = data as TaskState;
			taskStore.updateTaskState(task_id, state);
			break;
		}

		case 'phase': {
			// Phase transition event
			const phaseData = data as { phase: string; status: string };
			const existingState = taskStore.getTaskState(task_id);
			if (existingState) {
				taskStore.updateTaskState(task_id, {
					...existingState,
					current_phase: phaseData.phase,
				});
			}
			// Also update task's current_phase
			taskStore.updateTask(task_id, { current_phase: phaseData.phase });
			break;
		}

		case 'tokens': {
			// Token usage update
			const tokenData = data as TaskState['tokens'];
			const existingState = taskStore.getTaskState(task_id);
			if (existingState) {
				taskStore.updateTaskState(task_id, {
					...existingState,
					tokens: tokenData,
				});
			}
			break;
		}

		case 'complete': {
			// Task completed
			const completeData = data as { status: string; phase?: string };
			taskStore.updateTaskStatus(
				task_id,
				completeData.status as Task['status'],
				completeData.phase
			);
			break;
		}

		case 'finalize': {
			// Finalize phase update
			const finalizeData = data as { step: string; status: string; progress?: number };
			// Update task status if needed
			if (finalizeData.status === 'running') {
				taskStore.updateTaskStatus(task_id, 'finalizing');
			} else if (finalizeData.status === 'completed') {
				taskStore.updateTaskStatus(task_id, 'completed');
			} else if (finalizeData.status === 'failed') {
				taskStore.updateTaskStatus(task_id, 'failed');
			}
			break;
		}

		case 'task_created': {
			// New task created (file watcher event)
			const task = data as Task;
			taskStore.addTask(task);
			break;
		}

		case 'task_updated': {
			// Task updated (file watcher event)
			const task = data as Task;
			taskStore.updateTask(task_id, task);
			break;
		}

		case 'task_deleted': {
			// Task deleted (file watcher event)
			taskStore.removeTask(task_id);
			toast.info(`Task ${task_id} was deleted`);
			break;
		}

		case 'initiative_created': {
			// New initiative created (file watcher event)
			const initiative = data as Initiative;
			const initiativeStore = useInitiativeStore.getState();
			initiativeStore.addInitiative(initiative);
			break;
		}

		case 'initiative_updated': {
			// Initiative updated (file watcher event)
			// task_id field contains initiative_id for initiative events
			const initiative = data as Initiative;
			const initiativeStore = useInitiativeStore.getState();
			initiativeStore.updateInitiative(initiative.id, initiative);
			break;
		}

		case 'initiative_deleted': {
			// Initiative deleted (file watcher event)
			// task_id field contains initiative_id for initiative events
			const initiativeStore = useInitiativeStore.getState();
			initiativeStore.removeInitiative(task_id);
			toast.info(`Initiative ${task_id} was deleted`);
			break;
		}

		case 'error': {
			// Error event from server
			const errorData = data as { message: string };
			toast.error(errorData.message || 'An error occurred');
			break;
		}

		case 'transcript':
			// Transcript events are handled by useTaskSubscription, not here
			// They're task-specific and streamed to components that subscribe
			break;

		case 'activity': {
			// Activity state update (spec_analyzing, spec_writing, etc.)
			const activityData = data as ActivityUpdate;
			taskStore.updateTaskActivity(task_id, activityData.phase, activityData.activity as ActivityState);
			break;
		}

		case 'heartbeat':
		case 'warning':
			// These events are informational; no state update needed
			// Could be used for UI indicators if desired
			break;

		default:
			// Unknown event type - log for debugging
			console.log('Unhandled WebSocket event:', eventType, data);
	}
}

/**
 * Handle WebSocket errors
 */
function handleWSError(error: WSError): void {
	console.error('WebSocket error:', error.error);
	toast.error(`WebSocket: ${error.error}`);
}

/**
 * Transcript line from streaming events
 */
export interface TranscriptLine {
	phase: string;
	iteration: number;
	type: 'prompt' | 'response' | 'tool' | 'error' | 'chunk';
	content: string;
	timestamp: string;
}

/**
 * useTaskSubscription hook
 *
 * Subscribe to a specific task's events for streaming updates.
 * Returns the current execution state and transcript lines.
 *
 * @param taskId - The task ID to subscribe to (optional)
 * @returns Object with state, transcript, and subscription status
 */
export function useTaskSubscription(taskId: string | undefined) {
	const { on, status: wsStatus } = useWebSocket();
	const taskState = useTaskStore((state) => (taskId ? state.taskStates.get(taskId) : undefined));
	const [transcript, setTranscript] = useState<TranscriptLine[]>([]);
	const [isSubscribed, setIsSubscribed] = useState(false);

	useEffect(() => {
		if (!taskId) {
			setIsSubscribed(false);
			setTranscript([]);
			return;
		}

		// Wait for WebSocket to be ready (connected or reconnecting has an instance)
		// We can't subscribe to events if the WebSocket isn't connected yet
		if (wsStatus === 'disconnected') {
			return;
		}

		// Clear transcript when task changes
		setTranscript([]);

		// Subscribe to transcript events for this task
		const unsubTranscript = on('transcript', (event) => {
			if ('event' in event && event.task_id === taskId) {
				const line = event.data as TranscriptLine;
				setTranscript((prev) => [...prev, line]);
			}
		});

		// Subscribe to state events for this task (handled globally, but track subscription)
		const unsubState = on('state', (event) => {
			if ('event' in event && event.task_id === taskId) {
				// State updates are handled by the global handler
				// This just ensures we're tracking the subscription
			}
		});

		setIsSubscribed(true);

		return () => {
			unsubTranscript();
			unsubState();
			setIsSubscribed(false);
		};
	}, [taskId, on, wsStatus]);

	return {
		/** Current execution state for the task */
		state: taskState,
		/** Streaming transcript lines */
		transcript,
		/** Whether actively subscribed to this task */
		isSubscribed,
		/** WebSocket connection status */
		connectionStatus: wsStatus,
		/** Clear transcript (useful when restarting task) */
		clearTranscript: useCallback(() => setTranscript([]), []),
	};
}

/**
 * useConnectionStatus hook
 *
 * Simple hook to get the WebSocket connection status.
 */
export function useConnectionStatus(): ConnectionStatus {
	const { status } = useWebSocket();
	return status;
}
