/**
 * EventSubscription - Connect RPC streaming client for real-time events
 *
 * Replaces the WebSocket-based OrcWebSocket class with Connect server streaming.
 */

import { create } from '@bufbuild/protobuf';
import { eventClient } from '../client';
import {
	SubscribeRequestSchema,
	type Event,
	type SubscribeRequest,
} from '@/gen/orc/v1/events_pb';

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';
export type EventHandler = (event: Event) => void;
export type StatusHandler = (status: ConnectionStatus) => void;

interface ConnectOptions {
	/** Filter by project IDs (empty = current project context) */
	projectIds?: string[];
	/** Subscribe to specific task events only */
	taskId?: string;
	/** Subscribe to specific initiative events only */
	initiativeId?: string;
	/** Event types to receive (empty = all) */
	eventTypes?: string[];
	/** Include heartbeat events for connection health */
	includeHeartbeat?: boolean;
}

/**
 * EventSubscription manages a Connect RPC server-streaming subscription
 * for real-time events from the orc API.
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Event listeners with unsubscribe support
 * - Connection status tracking
 */
export class EventSubscription {
	private abortController: AbortController | null = null;
	private statusListeners = new Set<StatusHandler>();
	private eventListeners = new Set<EventHandler>();
	private status: ConnectionStatus = 'disconnected';
	private reconnectAttempts = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

	private readonly maxReconnects = 5;
	private readonly baseDelay = 1000; // 1 second

	// Stored options for reconnection
	private options: ConnectOptions = {};

	/**
	 * Start the event subscription.
	 * If already connected, disconnects first.
	 */
	async connect(options: ConnectOptions = {}): Promise<void> {
		// Store options for reconnection
		this.options = options;

		// Clean up any existing connection
		this.cleanup();

		this.setStatus('connecting');
		this.abortController = new AbortController();

		try {
			const request = create(SubscribeRequestSchema, {
				projectIds: options.projectIds ?? [],
				taskId: options.taskId,
				initiativeId: options.initiativeId,
				eventTypes: options.eventTypes ?? [],
				includeHeartbeat: options.includeHeartbeat ?? true,
			} satisfies Partial<SubscribeRequest>);

			const stream = eventClient.subscribe(request, {
				signal: this.abortController.signal,
			});

			this.setStatus('connected');
			this.reconnectAttempts = 0;

			// Process stream events
			for await (const response of stream) {
				if (response.event) {
					this.notifyEventListeners(response.event);
				}
			}

			// Stream ended normally (server closed)
			this.attemptReconnect();
		} catch (error) {
			// Check if this was an intentional abort/cancel
			// AbortError = native fetch abort
			// ConnectError with [canceled] = Connect RPC abort via signal
			const isAbort =
				(error as Error).name === 'AbortError' ||
				(error instanceof Error && error.message.includes('[canceled]'));

			if (isAbort) {
				// Don't change status here - if a new connection was started,
				// it will set its own status. If this was a true disconnect(),
				// status was already set to 'disconnected'.
				return;
			}

			// Connection error - attempt reconnect
			console.error('Event subscription error:', error);
			this.attemptReconnect();
		}
	}

	/**
	 * Register an event handler.
	 * @returns Unsubscribe function
	 */
	on(handler: EventHandler): () => void {
		this.eventListeners.add(handler);
		return () => this.eventListeners.delete(handler);
	}

	/**
	 * Register a connection status handler.
	 * Handler is called immediately with current status.
	 * @returns Unsubscribe function
	 */
	onStatusChange(handler: StatusHandler): () => void {
		this.statusListeners.add(handler);
		handler(this.status);
		return () => this.statusListeners.delete(handler);
	}

	/**
	 * Disconnect and stop reconnection attempts.
	 */
	disconnect(): void {
		this.cleanup();
		this.setStatus('disconnected');
	}

	/**
	 * Get current connection status.
	 */
	getStatus(): ConnectionStatus {
		return this.status;
	}

	/**
	 * Check if currently connected.
	 */
	isConnected(): boolean {
		return this.status === 'connected';
	}

	private cleanup(): void {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		if (this.abortController) {
			this.abortController.abort();
			this.abortController = null;
		}
	}

	private setStatus(status: ConnectionStatus): void {
		if (this.status === status) return;
		this.status = status;
		this.statusListeners.forEach((handler) => handler(status));
	}

	private notifyEventListeners(event: Event): void {
		this.eventListeners.forEach((handler) => {
			try {
				handler(event);
			} catch (error) {
				console.error('Event handler error:', error);
			}
		});
	}

	private attemptReconnect(): void {
		if (this.reconnectAttempts >= this.maxReconnects) {
			console.warn(`Event subscription: max reconnect attempts (${this.maxReconnects}) reached`);
			this.setStatus('disconnected');
			return;
		}

		this.setStatus('reconnecting');
		this.reconnectAttempts++;

		// Exponential backoff: 1s, 2s, 4s, 8s, 16s
		const delay = this.baseDelay * Math.pow(2, this.reconnectAttempts - 1);
		console.warn(`Event subscription: reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

		this.reconnectTimer = setTimeout(() => {
			this.connect(this.options);
		}, delay);
	}
}
