import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock WebSocket class
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	static instances: MockWebSocket[] = [];

	url: string;
	readyState: number = MockWebSocket.OPEN; // Start as OPEN for simpler testing
	onopen: (() => void) | null = null;
	onclose: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;
	onerror: ((error: unknown) => void) | null = null;

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send = vi.fn();
	close = vi.fn(() => {
		this.readyState = MockWebSocket.CLOSED;
	});

	// Test helpers
	triggerOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	triggerMessage(data: unknown) {
		this.onmessage?.({ data: JSON.stringify(data) });
	}

	triggerError(error: unknown) {
		this.onerror?.(error);
	}

	triggerClose() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}
}

// Set up global mocks
vi.stubGlobal('WebSocket', MockWebSocket);
vi.stubGlobal('location', {
	protocol: 'http:',
	host: 'localhost:5173'
});

describe('OrcWebSocket', () => {
	beforeEach(() => {
		MockWebSocket.instances = [];
		vi.spyOn(console, 'log').mockImplementation(() => {});
		vi.spyOn(console, 'error').mockImplementation(() => {});
	});

	afterEach(() => {
		vi.clearAllMocks();
		vi.clearAllTimers();
	});

	// Import module fresh for each test to avoid singleton issues
	async function getModule() {
		vi.resetModules();
		return await import('./websocket');
	}

	describe('OrcWebSocket class', () => {
		it('creates WebSocket with correct URL', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect();

			expect(MockWebSocket.instances).toHaveLength(1);
			expect(MockWebSocket.instances[0].url).toBe('ws://localhost:5173/api/ws');
		});

		it('sends subscribe message on connect with taskId', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');

			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();

			expect(mockWS.send).toHaveBeenCalledWith(
				JSON.stringify({ type: 'subscribe', task_id: 'TASK-001' })
			);
		});

		it('returns status correctly', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();

			expect(ws.getStatus()).toBe('disconnected');
		});

		it('subscribe stores taskId when not connected', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.subscribe('TASK-001');

			expect(ws.getTaskId()).toBe('TASK-001');
		});

		it('on() returns unsubscribe function', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			const unsubscribe = ws.on('state', callback);
			expect(typeof unsubscribe).toBe('function');
		});

		it('onStatusChange immediately notifies with current status', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			ws.onStatusChange(callback);
			expect(callback).toHaveBeenCalledWith('disconnected');
		});

		it('onStatusChange returns unsubscribe function', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			const unsubscribe = ws.onStatusChange(callback);
			expect(typeof unsubscribe).toBe('function');
		});

		it('disconnect closes WebSocket', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect();
			const mockWS = MockWebSocket.instances[0];

			ws.disconnect();

			expect(mockWS.close).toHaveBeenCalled();
			expect(ws.getTaskId()).toBeNull();
		});

		it('pause sends correct command', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect();
			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();

			ws.pause('TASK-001');

			expect(mockWS.send).toHaveBeenCalledWith(
				JSON.stringify({ type: 'command', task_id: 'TASK-001', action: 'pause' })
			);
		});

		it('resume sends correct command', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect();
			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();

			ws.resume('TASK-001');

			expect(mockWS.send).toHaveBeenCalledWith(
				JSON.stringify({ type: 'command', task_id: 'TASK-001', action: 'resume' })
			);
		});

		it('cancel sends correct command', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect();
			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();

			ws.cancel('TASK-001');

			expect(mockWS.send).toHaveBeenCalledWith(
				JSON.stringify({ type: 'command', task_id: 'TASK-001', action: 'cancel' })
			);
		});

		it('handles event messages and notifies listeners', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			ws.on('state', callback);
			ws.connect();

			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();
			mockWS.triggerMessage({
				type: 'event',
				event: 'state',
				task_id: 'TASK-001',
				data: { status: 'running' },
				time: '2025-01-01T00:00:00Z'
			});

			expect(callback).toHaveBeenCalled();
		});

		it('handles "all" event listener', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			ws.on('all', callback);
			ws.connect();

			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();
			mockWS.triggerMessage({
				type: 'event',
				event: 'transcript',
				task_id: 'TASK-001',
				data: { content: 'hello' },
				time: '2025-01-01T00:00:00Z'
			});

			expect(callback).toHaveBeenCalled();
		});

		it('handles error messages', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			const callback = vi.fn();

			ws.on('error', callback);
			ws.connect();

			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();
			mockWS.triggerMessage({
				type: 'error',
				error: 'Something went wrong'
			});

			expect(callback).toHaveBeenCalledWith({ type: 'error', error: 'Something went wrong' });
		});

		it('unsubscribe sends correct message', async () => {
			const { OrcWebSocket } = await getModule();
			const ws = new OrcWebSocket();
			ws.connect('TASK-001');
			const mockWS = MockWebSocket.instances[0];
			mockWS.triggerOpen();

			ws.unsubscribe();

			expect(mockWS.send).toHaveBeenCalledWith(
				JSON.stringify({ type: 'unsubscribe', task_id: 'TASK-001' })
			);
			expect(ws.getTaskId()).toBeNull();
		});
	});

	describe('getWebSocket singleton', () => {
		it('returns singleton instance', async () => {
			const { getWebSocket } = await getModule();

			const ws1 = getWebSocket();
			const ws2 = getWebSocket();

			expect(ws1).toBe(ws2);
		});
	});

	describe('subscribeToTaskWS', () => {
		it('returns cleanup function', async () => {
			const { subscribeToTaskWS } = await getModule();
			const onEvent = vi.fn();

			const cleanup = subscribeToTaskWS('TASK-001', onEvent);

			expect(typeof cleanup).toBe('function');
		});

		it('calls status callback with current status', async () => {
			const { subscribeToTaskWS } = await getModule();
			const onEvent = vi.fn();
			const onStatus = vi.fn();

			subscribeToTaskWS('TASK-001', onEvent, onStatus);

			expect(onStatus).toHaveBeenCalled();
		});
	});
});
