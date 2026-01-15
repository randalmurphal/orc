/**
 * WebSocket client for real-time task updates
 */

export type WSEventType =
	| 'state'
	| 'transcript'
	| 'phase'
	| 'tokens'
	| 'error'
	| 'complete'
	| 'finalize'
	// File watcher events (triggered by external file changes)
	| 'task_created'
	| 'task_updated'
	| 'task_deleted';

// Special task ID for subscribing to all task events
export const GLOBAL_TASK_ID = '*';

export interface WSEvent {
	type: 'event';
	event: WSEventType;
	task_id: string;
	data: unknown;
	time: string;
}

export interface WSMessage {
	type: string;
	task_id?: string;
	action?: string;
	data?: unknown;
}

export interface WSError {
	type: 'error';
	error: string;
}

export type WSCallback = (event: WSEvent | WSError) => void;

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

export class OrcWebSocket {
	private ws: WebSocket | null = null;
	private taskId: string | null = null;
	private primarySubscription: string | null = null; // Default subscription to restore on reconnect
	private listeners = new Map<string, Set<WSCallback>>();
	private statusListeners = new Set<(status: ConnectionStatus) => void>();
	private reconnectAttempts = 0;
	private maxReconnects = 5;
	private reconnectDelay = 1000;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private pingInterval: ReturnType<typeof setInterval> | null = null;
	private status: ConnectionStatus = 'disconnected';
	private url: string;

	constructor(baseUrl?: string) {
		// Determine WebSocket URL based on current location
		const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = baseUrl || window.location.host;
		this.url = `${wsProtocol}//${host}/api/ws`;
	}

	/**
	 * Connect to WebSocket server and optionally subscribe to a task
	 */
	connect(taskId?: string): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			// Already connected, just subscribe if taskId provided
			if (taskId) {
				this.subscribe(taskId);
			}
			return;
		}

		this.setStatus('connecting');
		this.ws = new WebSocket(this.url);

		this.ws.onopen = () => {
			this.setStatus('connected');
			this.reconnectAttempts = 0;

			// Subscribe to task if provided
			if (taskId) {
				this.subscribe(taskId);
			}

			// Start ping interval to keep connection alive
			this.startPingInterval();
		};

		this.ws.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data);
				this.handleMessage(msg);
			} catch (e) {
				console.error('Failed to parse WebSocket message:', e);
			}
		};

		this.ws.onclose = () => {
			this.setStatus('disconnected');
			this.stopPingInterval();
			this.attemptReconnect();
		};

		this.ws.onerror = (error) => {
			console.error('WebSocket error:', error);
			this.notifyListeners('error', { type: 'error', error: 'Connection error' });
		};
	}

	/**
	 * Disconnect from WebSocket server
	 */
	disconnect(): void {
		this.clearReconnectTimer();
		this.stopPingInterval();

		if (this.ws) {
			this.ws.close();
			this.ws = null;
		}

		this.taskId = null;
		this.primarySubscription = null;
		this.setStatus('disconnected');
	}

	/**
	 * Subscribe to a task's events
	 */
	subscribe(taskId: string): void {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
			// Queue subscription for after connection
			this.taskId = taskId;
			return;
		}

		this.taskId = taskId;
		this.send({ type: 'subscribe', task_id: taskId });
	}

	/**
	 * Subscribe to ALL task events (global subscription)
	 * This also sets global as the primary subscription for reconnection.
	 */
	subscribeGlobal(): void {
		this.primarySubscription = GLOBAL_TASK_ID;
		this.subscribe(GLOBAL_TASK_ID);
	}

	/**
	 * Set the primary subscription to restore on reconnection.
	 * Use GLOBAL_TASK_ID for global subscription.
	 */
	setPrimarySubscription(taskId: string | null): void {
		this.primarySubscription = taskId;
	}

	/**
	 * Get the primary subscription
	 */
	getPrimarySubscription(): string | null {
		return this.primarySubscription;
	}

	/**
	 * Check if currently subscribed globally
	 */
	isGlobalSubscription(): boolean {
		return this.taskId === GLOBAL_TASK_ID;
	}

	/**
	 * Unsubscribe from current task
	 */
	unsubscribe(): void {
		if (this.ws?.readyState === WebSocket.OPEN && this.taskId) {
			this.send({ type: 'unsubscribe', task_id: this.taskId });
		}
		this.taskId = null;
	}

	/**
	 * Send a command to control a task
	 */
	command(taskId: string, action: 'pause' | 'resume' | 'cancel'): void {
		this.send({
			type: 'command',
			task_id: taskId,
			action
		});
	}

	/**
	 * Pause a running task
	 */
	pause(taskId: string): void {
		this.command(taskId, 'pause');
	}

	/**
	 * Resume a paused task
	 */
	resume(taskId: string): void {
		this.command(taskId, 'resume');
	}

	/**
	 * Cancel a running task
	 */
	cancel(taskId: string): void {
		this.command(taskId, 'cancel');
	}

	/**
	 * Add event listener for specific event types
	 */
	on(eventType: WSEventType | 'all', callback: WSCallback): () => void {
		const key = eventType === 'all' ? '*' : eventType;
		if (!this.listeners.has(key)) {
			this.listeners.set(key, new Set());
		}
		this.listeners.get(key)!.add(callback);

		// Return unsubscribe function
		return () => {
			this.listeners.get(key)?.delete(callback);
		};
	}

	/**
	 * Add listener for connection status changes
	 */
	onStatusChange(callback: (status: ConnectionStatus) => void): () => void {
		this.statusListeners.add(callback);
		// Immediately notify of current status
		callback(this.status);
		return () => this.statusListeners.delete(callback);
	}

	/**
	 * Get current connection status
	 */
	getStatus(): ConnectionStatus {
		return this.status;
	}

	/**
	 * Get current subscribed task ID
	 */
	getTaskId(): string | null {
		return this.taskId;
	}

	/**
	 * Check if connected
	 */
	isConnected(): boolean {
		return this.ws?.readyState === WebSocket.OPEN;
	}

	private send(message: WSMessage): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(JSON.stringify(message));
		}
	}

	private handleMessage(msg: unknown): void {
		const message = msg as Record<string, unknown>;

		switch (message.type) {
			case 'event':
				this.notifyListeners(message.event as WSEventType, message as unknown as WSEvent);
				break;
			case 'subscribed':
				console.log('Subscribed to task:', message.task_id);
				break;
			case 'command_result':
				console.log('Command result:', message);
				break;
			case 'error':
				this.notifyListeners('error', { type: 'error', error: String(message.error || 'Unknown error') });
				break;
			default:
				console.log('Unknown message type:', message);
		}
	}

	private notifyListeners(eventType: WSEventType | 'error', data: WSEvent | WSError): void {
		// Notify specific listeners
		this.listeners.get(eventType)?.forEach((cb) => cb(data));
		// Notify 'all' listeners
		this.listeners.get('*')?.forEach((cb) => cb(data));
	}

	private setStatus(status: ConnectionStatus): void {
		this.status = status;
		this.statusListeners.forEach((cb) => cb(status));
	}

	private attemptReconnect(): void {
		if (this.reconnectAttempts >= this.maxReconnects) {
			console.log('Max reconnect attempts reached');
			return;
		}

		this.setStatus('reconnecting');
		this.reconnectAttempts++;

		const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
		console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

		this.reconnectTimer = setTimeout(() => {
			// Prefer primary subscription (e.g., global) over current taskId
			// This ensures we restore global subscription after temporary task-specific ones
			const subscriptionToRestore = this.primarySubscription || this.taskId || undefined;
			this.connect(subscriptionToRestore);
		}, delay);
	}

	private clearReconnectTimer(): void {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
	}

	private startPingInterval(): void {
		this.pingInterval = setInterval(() => {
			if (this.ws?.readyState === WebSocket.OPEN) {
				this.send({ type: 'ping' });
			}
		}, 30000); // Ping every 30 seconds
	}

	private stopPingInterval(): void {
		if (this.pingInterval) {
			clearInterval(this.pingInterval);
			this.pingInterval = null;
		}
	}
}

// Singleton instance
let instance: OrcWebSocket | null = null;

/**
 * Get or create WebSocket instance
 */
export function getWebSocket(baseUrl?: string): OrcWebSocket {
	if (!instance) {
		instance = new OrcWebSocket(baseUrl);
	}
	return instance;
}

/**
 * Subscribe to task events using WebSocket
 * Returns cleanup function
 *
 * Note: This will override any existing subscription (including global).
 * For pages that need task-specific events while layout has global subscription,
 * consider just adding event listeners without resubscribing.
 */
export function subscribeToTaskWS(
	taskId: string,
	onEvent: (event: WSEventType, data: unknown) => void,
	onStatus?: (status: ConnectionStatus) => void
): () => void {
	const ws = getWebSocket();

	// Set up event listener
	const unsubEvent = ws.on('all', (event) => {
		if ('event' in event) {
			onEvent(event.event, event.data);
		} else if (event.type === 'error') {
			onEvent('error', { message: event.error });
		}
	});

	// Set up status listener if provided
	let unsubStatus: (() => void) | undefined;
	if (onStatus) {
		unsubStatus = ws.onStatusChange(onStatus);
	}

	// Connect and subscribe
	ws.connect(taskId);

	// Capture the taskId we subscribed to
	const subscribedTaskId = taskId;

	// Return cleanup function
	// Only unsubscribe if we're still subscribed to the same task
	// (prevents interfering with other subscriptions like global)
	return () => {
		unsubEvent();
		unsubStatus?.();
		// Only unsubscribe if our subscription is still active
		if (ws.getTaskId() === subscribedTaskId) {
			ws.unsubscribe();
		}
	};
}

/**
 * Initialize global WebSocket subscription for real-time task updates.
 * This subscribes to ALL task events and updates the global task store.
 * Should be called once at app startup (in +layout.svelte).
 * Returns cleanup function.
 */
export function initGlobalWebSocket(
	onEvent?: (taskId: string, eventType: WSEventType, data: unknown) => void,
	onStatus?: (status: ConnectionStatus) => void
): () => void {
	const ws = getWebSocket();

	// Set up event listener for all events
	const unsubEvent = ws.on('all', (event) => {
		if ('event' in event) {
			const wsEvent = event as WSEvent;
			onEvent?.(wsEvent.task_id, wsEvent.event, wsEvent.data);
		}
	});

	// Set up status listener if provided
	let unsubStatus: (() => void) | undefined;
	if (onStatus) {
		unsubStatus = ws.onStatusChange(onStatus);
	}

	// Set global as the primary subscription for reconnection
	ws.setPrimarySubscription(GLOBAL_TASK_ID);

	// Connect and subscribe to global events
	ws.connect(GLOBAL_TASK_ID);

	// Return cleanup function
	// Only unsubscribe if we're still subscribed globally
	return () => {
		unsubEvent();
		unsubStatus?.();
		// Only unsubscribe if global subscription is still active
		if (ws.getTaskId() === GLOBAL_TASK_ID) {
			ws.unsubscribe();
		}
	};
}
