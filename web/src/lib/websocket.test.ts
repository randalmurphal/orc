import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { OrcWebSocket, GLOBAL_TASK_ID } from './websocket';

// Mock WebSocket
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	url: string;
	readyState: number = MockWebSocket.CONNECTING;
	onopen: (() => void) | null = null;
	onclose: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;
	onerror: ((error: unknown) => void) | null = null;
	sentMessages: string[] = [];

	constructor(url: string) {
		this.url = url;
	}

	send(data: string) {
		this.sentMessages.push(data);
	}

	close() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}

	// Test helpers
	simulateOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	simulateMessage(data: unknown) {
		this.onmessage?.({ data: JSON.stringify(data) });
	}

	simulateClose() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}

	simulateError(error: unknown) {
		this.onerror?.(error);
	}
}

// Store mock instances for test access
let mockWsInstances: MockWebSocket[] = [];

// Mock window.location
const originalLocation = globalThis.location;

describe('OrcWebSocket', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockWsInstances = [];

		// Mock WebSocket constructor
		globalThis.WebSocket = vi.fn((url: string) => {
			const ws = new MockWebSocket(url);
			mockWsInstances.push(ws);
			return ws;
		}) as unknown as typeof WebSocket;

		// Set WebSocket constants
		(globalThis.WebSocket as unknown as Record<string, number>).OPEN = MockWebSocket.OPEN;
		(globalThis.WebSocket as unknown as Record<string, number>).CLOSED = MockWebSocket.CLOSED;
		(globalThis.WebSocket as unknown as Record<string, number>).CONNECTING =
			MockWebSocket.CONNECTING;
		(globalThis.WebSocket as unknown as Record<string, number>).CLOSING = MockWebSocket.CLOSING;

		// Mock window.location
		Object.defineProperty(globalThis, 'location', {
			value: {
				protocol: 'http:',
				host: 'localhost:5174',
			},
			writable: true,
		});
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
		Object.defineProperty(globalThis, 'location', {
			value: originalLocation,
			writable: true,
		});
	});

	describe('constructor', () => {
		it('should create WebSocket URL from window.location', () => {
			const ws = new OrcWebSocket();
			ws.connect();

			expect(mockWsInstances[0].url).toBe('ws://localhost:5174/api/ws');
		});

		it('should use wss for https', () => {
			Object.defineProperty(globalThis, 'location', {
				value: { protocol: 'https:', host: 'example.com' },
				writable: true,
			});

			const ws = new OrcWebSocket();
			ws.connect();

			expect(mockWsInstances[0].url).toBe('wss://example.com/api/ws');
		});

		it('should accept custom base URL', () => {
			const ws = new OrcWebSocket('custom-host:8080');
			ws.connect();

			expect(mockWsInstances[0].url).toBe('ws://custom-host:8080/api/ws');
		});
	});

	describe('connect', () => {
		it('should create WebSocket connection', () => {
			const ws = new OrcWebSocket();
			ws.connect();

			expect(mockWsInstances).toHaveLength(1);
		});

		it('should set status to connecting', () => {
			const ws = new OrcWebSocket();
			const statusCallback = vi.fn();
			ws.onStatusChange(statusCallback);

			ws.connect();

			expect(statusCallback).toHaveBeenCalledWith('connecting');
		});

		it('should set status to connected on open', () => {
			const ws = new OrcWebSocket();
			const statusCallback = vi.fn();
			ws.onStatusChange(statusCallback);

			ws.connect();
			mockWsInstances[0].simulateOpen();

			expect(statusCallback).toHaveBeenCalledWith('connected');
			expect(ws.getStatus()).toBe('connected');
		});

		it('should subscribe to task on connect if taskId provided', () => {
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');
			mockWsInstances[0].simulateOpen();

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'subscribe',
				task_id: 'TASK-001',
			});
		});

		it('should not create new connection if already connected', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.connect('TASK-002');

			expect(mockWsInstances).toHaveLength(1);
		});

		it('should subscribe to new task if already connected', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.connect('TASK-002');

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'subscribe',
				task_id: 'TASK-002',
			});
		});
	});

	describe('disconnect', () => {
		it('should close WebSocket connection', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.disconnect();

			expect(mockWsInstances[0].readyState).toBe(MockWebSocket.CLOSED);
		});

		it('should set status to disconnected', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			const statusCallback = vi.fn();
			ws.onStatusChange(statusCallback);
			ws.disconnect();

			expect(statusCallback).toHaveBeenCalledWith('disconnected');
		});

		it('should clear task ID', () => {
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');
			mockWsInstances[0].simulateOpen();

			ws.disconnect();

			expect(ws.getTaskId()).toBeNull();
		});

		it('should clear primary subscription', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();
			ws.subscribeGlobal();

			ws.disconnect();

			expect(ws.getPrimarySubscription()).toBeNull();
		});
	});

	describe('subscribe', () => {
		it('should send subscribe message', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.subscribe('TASK-001');

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'subscribe',
				task_id: 'TASK-001',
			});
		});

		it('should update current task ID', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.subscribe('TASK-001');

			expect(ws.getTaskId()).toBe('TASK-001');
		});

		it('should queue subscription if not connected', () => {
			const ws = new OrcWebSocket();
			ws.subscribe('TASK-001');

			expect(ws.getTaskId()).toBe('TASK-001');
			expect(mockWsInstances).toHaveLength(0);
		});
	});

	describe('subscribeGlobal', () => {
		it('should subscribe to global task ID', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.subscribeGlobal();

			expect(ws.getTaskId()).toBe(GLOBAL_TASK_ID);
		});

		it('should set primary subscription to global', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.subscribeGlobal();

			expect(ws.getPrimarySubscription()).toBe(GLOBAL_TASK_ID);
		});

		it('should return true for isGlobalSubscription', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.subscribeGlobal();

			expect(ws.isGlobalSubscription()).toBe(true);
		});
	});

	describe('unsubscribe', () => {
		it('should send unsubscribe message', () => {
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');
			mockWsInstances[0].simulateOpen();

			ws.unsubscribe();

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'unsubscribe',
				task_id: 'TASK-001',
			});
		});

		it('should clear task ID', () => {
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');
			mockWsInstances[0].simulateOpen();

			ws.unsubscribe();

			expect(ws.getTaskId()).toBeNull();
		});
	});

	describe('command', () => {
		it('should send pause command', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.pause('TASK-001');

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'command',
				task_id: 'TASK-001',
				action: 'pause',
			});
		});

		it('should send resume command', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.resume('TASK-001');

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'command',
				task_id: 'TASK-001',
				action: 'resume',
			});
		});

		it('should send cancel command', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.cancel('TASK-001');

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'command',
				task_id: 'TASK-001',
				action: 'cancel',
			});
		});
	});

	describe('event listeners', () => {
		it('should notify listeners on event', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			ws.on('state', callback);

			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateMessage({
				type: 'event',
				event: 'state',
				task_id: 'TASK-001',
				data: { status: 'running' },
			});

			expect(callback).toHaveBeenCalled();
		});

		it('should notify "all" listeners for any event', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			ws.on('all', callback);

			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateMessage({
				type: 'event',
				event: 'state',
				task_id: 'TASK-001',
				data: {},
			});

			expect(callback).toHaveBeenCalled();
		});

		it('should return unsubscribe function', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			const unsubscribe = ws.on('state', callback);

			unsubscribe();

			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateMessage({
				type: 'event',
				event: 'state',
				task_id: 'TASK-001',
				data: {},
			});

			expect(callback).not.toHaveBeenCalled();
		});
	});

	describe('status listeners', () => {
		it('should notify on status change', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			ws.onStatusChange(callback);

			ws.connect();

			expect(callback).toHaveBeenCalledWith('connecting');
		});

		it('should immediately notify current status', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			ws.onStatusChange(callback);

			expect(callback).toHaveBeenCalledWith('disconnected');
		});

		it('should return unsubscribe function', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			const unsubscribe = ws.onStatusChange(callback);

			callback.mockClear();
			unsubscribe();

			ws.connect();

			expect(callback).not.toHaveBeenCalled();
		});
	});

	describe('reconnection', () => {
		it('should attempt reconnect on close', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateClose();

			expect(ws.getStatus()).toBe('reconnecting');
		});

		it('should use exponential backoff', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateClose();

			// First reconnect at 1s
			expect(mockWsInstances).toHaveLength(1);
			vi.advanceTimersByTime(1000);
			expect(mockWsInstances).toHaveLength(2);

			// Second reconnect at 2s
			mockWsInstances[1].simulateClose();
			vi.advanceTimersByTime(2000);
			expect(mockWsInstances).toHaveLength(3);

			// Third reconnect at 4s
			mockWsInstances[2].simulateClose();
			vi.advanceTimersByTime(4000);
			expect(mockWsInstances).toHaveLength(4);
		});

		it('should stop after max reconnects', () => {
			const ws = new OrcWebSocket();
			ws.connect(); // First connection (instance 0)

			// Simulate 5 failed connection attempts (connection never opens, just closes)
			// Each close triggers a reconnect, but since we never call simulateOpen(),
			// the reconnect counter keeps incrementing
			mockWsInstances[0].simulateClose(); // Triggers reconnect attempt 1
			vi.advanceTimersByTime(1000);
			expect(mockWsInstances).toHaveLength(2); // Instance 1 created

			mockWsInstances[1].simulateClose(); // Triggers reconnect attempt 2
			vi.advanceTimersByTime(2000);
			expect(mockWsInstances).toHaveLength(3); // Instance 2 created

			mockWsInstances[2].simulateClose(); // Triggers reconnect attempt 3
			vi.advanceTimersByTime(4000);
			expect(mockWsInstances).toHaveLength(4); // Instance 3 created

			mockWsInstances[3].simulateClose(); // Triggers reconnect attempt 4
			vi.advanceTimersByTime(8000);
			expect(mockWsInstances).toHaveLength(5); // Instance 4 created

			mockWsInstances[4].simulateClose(); // Triggers reconnect attempt 5 (max)
			vi.advanceTimersByTime(16000);
			expect(mockWsInstances).toHaveLength(6); // Instance 5 created

			// Now at max reconnects - this close should NOT trigger another reconnect
			mockWsInstances[5].simulateClose();
			vi.advanceTimersByTime(100000);

			// Still 6 (no new reconnect attempted, max reached)
			expect(mockWsInstances).toHaveLength(6);
		});

		it('should restore primary subscription on reconnect', () => {
			const ws = new OrcWebSocket();
			ws.setPrimarySubscription(GLOBAL_TASK_ID);
			ws.connect();
			mockWsInstances[0].simulateOpen();

			// Subscribe to specific task
			ws.subscribe('TASK-001');

			// Disconnect
			mockWsInstances[0].simulateClose();

			// Reconnect
			vi.advanceTimersByTime(1000);
			mockWsInstances[1].simulateOpen();

			// Should restore global subscription
			const sentMessages = mockWsInstances[1].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'subscribe',
				task_id: GLOBAL_TASK_ID,
			});
		});

		it('should reset reconnect attempts on successful connection', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateClose();

			// First reconnect
			vi.advanceTimersByTime(1000);
			mockWsInstances[1].simulateOpen();

			// Successful connection resets counter
			// Now simulate another disconnect
			mockWsInstances[1].simulateClose();

			// Should use base delay (1s) again, not exponential
			vi.advanceTimersByTime(1000);
			expect(mockWsInstances).toHaveLength(3);
		});
	});

	describe('ping/pong heartbeat', () => {
		it('should send ping every 30 seconds', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			vi.advanceTimersByTime(30000);

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({ type: 'ping' });
		});

		it('should stop ping on disconnect', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			ws.disconnect();
			mockWsInstances[0].sentMessages = [];

			vi.advanceTimersByTime(60000);

			expect(mockWsInstances[0].sentMessages).toHaveLength(0);
		});
	});

	describe('error handling', () => {
		it('should notify error listeners on error', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			ws.on('error', callback);

			ws.connect();
			mockWsInstances[0].simulateError(new Error('Connection failed'));

			expect(callback).toHaveBeenCalledWith({
				type: 'error',
				error: 'Connection error',
			});
		});

		it('should handle error message type', () => {
			const ws = new OrcWebSocket();
			const callback = vi.fn();
			ws.on('error', callback);

			ws.connect();
			mockWsInstances[0].simulateOpen();
			mockWsInstances[0].simulateMessage({
				type: 'error',
				error: 'Server error',
			});

			expect(callback).toHaveBeenCalledWith({
				type: 'error',
				error: 'Server error',
			});
		});
	});

	describe('isConnected', () => {
		it('should return true when connected', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();

			expect(ws.isConnected()).toBe(true);
		});

		it('should return false when not connected', () => {
			const ws = new OrcWebSocket();

			expect(ws.isConnected()).toBe(false);
		});

		it('should return false after disconnect', () => {
			const ws = new OrcWebSocket();
			ws.connect();
			mockWsInstances[0].simulateOpen();
			ws.disconnect();

			expect(ws.isConnected()).toBe(false);
		});
	});
});
