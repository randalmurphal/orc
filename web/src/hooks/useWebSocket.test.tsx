import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import {
	WebSocketProvider,
	useWebSocket,
	useTaskSubscription,
	useConnectionStatus,
	GLOBAL_TASK_ID,
} from './useWebSocket';
import { useUIStore, useTaskStore } from '@/stores';

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
}

let mockWsInstances: MockWebSocket[] = [];

describe('WebSocket hooks', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockWsInstances = [];

		// Reset stores
		useUIStore.getState().reset();
		useTaskStore.getState().reset();

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
	});

	describe('WebSocketProvider', () => {
		it('should render children', () => {
			render(
				<WebSocketProvider>
					<div>Test Child</div>
				</WebSocketProvider>
			);

			expect(screen.getByText('Test Child')).toBeInTheDocument();
		});

		it('should auto-connect by default', () => {
			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			expect(mockWsInstances).toHaveLength(1);
		});

		it('should not auto-connect when autoConnect is false', () => {
			render(
				<WebSocketProvider autoConnect={false}>
					<div>Test</div>
				</WebSocketProvider>
			);

			expect(mockWsInstances).toHaveLength(0);
		});

		it('should subscribe to global events by default', async () => {
			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			const sentMessages = mockWsInstances[0].sentMessages.map((m) => JSON.parse(m));
			expect(sentMessages).toContainEqual({
				type: 'subscribe',
				task_id: GLOBAL_TASK_ID,
			});
		});

		it('should update UIStore wsStatus on connection', async () => {
			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			expect(useUIStore.getState().wsStatus).toBe('connecting');

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			expect(useUIStore.getState().wsStatus).toBe('connected');
		});

		it('should disconnect on unmount', async () => {
			const { unmount } = render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			unmount();

			expect(mockWsInstances[0].readyState).toBe(MockWebSocket.CLOSED);
		});
	});

	describe('useWebSocket', () => {
		function wrapper({ children }: { children: ReactNode }) {
			return <WebSocketProvider autoConnect={false}>{children}</WebSocketProvider>;
		}

		it('should throw error when used outside provider', () => {
			// Suppress console.error for this test
			const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

			expect(() => {
				renderHook(() => useWebSocket());
			}).toThrow('useWebSocket must be used within a WebSocketProvider');

			consoleError.mockRestore();
		});

		it('should return status', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(result.current.status).toBe('disconnected');
		});

		it('should return subscribe function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.subscribe).toBe('function');
		});

		it('should return unsubscribe function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.unsubscribe).toBe('function');
		});

		it('should return subscribeGlobal function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.subscribeGlobal).toBe('function');
		});

		it('should return on function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.on).toBe('function');
		});

		it('should return command function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.command).toBe('function');
		});

		it('should return isConnected function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.isConnected).toBe('function');
			expect(result.current.isConnected()).toBe(false);
		});

		it('should return getTaskId function', () => {
			const { result } = renderHook(() => useWebSocket(), { wrapper });

			expect(typeof result.current.getTaskId).toBe('function');
			expect(result.current.getTaskId()).toBeNull();
		});
	});

	describe('useConnectionStatus', () => {
		function wrapper({ children }: { children: ReactNode }) {
			return <WebSocketProvider autoConnect={false}>{children}</WebSocketProvider>;
		}

		it('should return current connection status', () => {
			const { result } = renderHook(() => useConnectionStatus(), { wrapper });

			expect(result.current).toBe('disconnected');
		});
	});

	describe('useTaskSubscription', () => {
		function wrapper({ children }: { children: ReactNode }) {
			return <WebSocketProvider>{children}</WebSocketProvider>;
		}

		it('should return undefined state when no taskId', async () => {
			const { result } = renderHook(() => useTaskSubscription(undefined), { wrapper });

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			expect(result.current.state).toBeUndefined();
			expect(result.current.isSubscribed).toBe(false);
		});

		it('should return empty transcript initially', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			expect(result.current.transcript).toEqual([]);
		});

		it('should be subscribed when taskId provided', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			expect(result.current.isSubscribed).toBe(true);
		});

		it('should clear transcript when task changes', async () => {
			const { result, rerender } = renderHook(
				({ taskId }) => useTaskSubscription(taskId),
				{ wrapper, initialProps: { taskId: 'TASK-001' } }
			);

			// First, open the connection
			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			// Then, in a separate act, send the message (allows effect to run)
			await act(async () => {
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'transcript',
					task_id: 'TASK-001',
					data: { timestamp: '2024-01-01', type: 'response', content: 'Hello' },
				});
			});

			expect(result.current.transcript).toHaveLength(1);

			rerender({ taskId: 'TASK-002' });

			expect(result.current.transcript).toEqual([]);
		});

		it('should receive transcript events for subscribed task', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			// First, open the connection
			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			// Then, in a separate act, send the message (allows effect to run)
			await act(async () => {
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'transcript',
					task_id: 'TASK-001',
					data: { timestamp: '2024-01-01T00:00:00Z', type: 'response', content: 'Hello' },
				});
			});

			expect(result.current.transcript).toHaveLength(1);
			expect(result.current.transcript[0].content).toBe('Hello');
		});

		it('should not receive transcript events for other tasks', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'transcript',
					task_id: 'TASK-002',
					data: { timestamp: '2024-01-01', type: 'response', content: 'Other task' },
				});
			});

			expect(result.current.transcript).toHaveLength(0);
		});

		it('should return connection status', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			expect(result.current.connectionStatus).toBe('connecting');

			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			expect(result.current.connectionStatus).toBe('connected');
		});

		it('should provide clearTranscript function', async () => {
			const { result } = renderHook(() => useTaskSubscription('TASK-001'), { wrapper });

			// First, open the connection
			await act(async () => {
				mockWsInstances[0].simulateOpen();
			});

			// Then, in a separate act, send the message (allows effect to run)
			await act(async () => {
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'transcript',
					task_id: 'TASK-001',
					data: { timestamp: '2024-01-01', type: 'response', content: 'Hello' },
				});
			});

			expect(result.current.transcript).toHaveLength(1);

			act(() => {
				result.current.clearTranscript();
			});

			expect(result.current.transcript).toHaveLength(0);
		});
	});

	describe('WebSocket event handling', () => {
		it('should update TaskStore on state event', async () => {
			// Add a task first
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Test Task',
				weight: 'small',
				status: 'created',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'state',
					task_id: 'TASK-001',
					data: {
						task_id: 'TASK-001',
						status: 'running',
						current_phase: 'implement',
						current_iteration: 1,
						started_at: '2024-01-01',
						updated_at: '2024-01-01',
						phases: {},
						gates: [],
						tokens: { input_tokens: 0, output_tokens: 0, total_tokens: 0 },
					},
				});
			});

			const taskState = useTaskStore.getState().getTaskState('TASK-001');
			expect(taskState).toBeDefined();
			expect(taskState?.status).toBe('running');
		});

		it('should add task on task_created event', async () => {
			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'task_created',
					task_id: 'TASK-002',
					data: {
						id: 'TASK-002',
						title: 'New Task',
						weight: 'small',
						status: 'created',
						branch: 'test',
						created_at: '2024-01-01',
						updated_at: '2024-01-01',
					},
				});
			});

			const task = useTaskStore.getState().getTask('TASK-002');
			expect(task).toBeDefined();
			expect(task?.title).toBe('New Task');
		});

		it('should update task on task_updated event', async () => {
			// Add task first
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Old Title',
				weight: 'small',
				status: 'created',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'task_updated',
					task_id: 'TASK-001',
					data: {
						id: 'TASK-001',
						title: 'New Title',
						weight: 'small',
						status: 'running',
						branch: 'test',
						created_at: '2024-01-01',
						updated_at: '2024-01-01',
					},
				});
			});

			const task = useTaskStore.getState().getTask('TASK-001');
			expect(task?.title).toBe('New Title');
			expect(task?.status).toBe('running');
		});

		it('should remove task on task_deleted event', async () => {
			// Add task first
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Test Task',
				weight: 'small',
				status: 'created',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'task_deleted',
					task_id: 'TASK-001',
					data: null,
				});
			});

			const task = useTaskStore.getState().getTask('TASK-001');
			expect(task).toBeUndefined();
		});

		it('should update task status on complete event', async () => {
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Test Task',
				weight: 'small',
				status: 'running',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'complete',
					task_id: 'TASK-001',
					data: { status: 'completed', phase: 'finalize' },
				});
			});

			const task = useTaskStore.getState().getTask('TASK-001');
			expect(task?.status).toBe('completed');
		});

		it('should update task status on finalize event', async () => {
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Test Task',
				weight: 'small',
				status: 'running',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'finalize',
					task_id: 'TASK-001',
					data: { step: 'sync', status: 'running', progress: 50 },
				});
			});

			const task = useTaskStore.getState().getTask('TASK-001');
			expect(task?.status).toBe('finalizing');
		});

		it('should update task status to finished on finalize completed', async () => {
			useTaskStore.getState().addTask({
				id: 'TASK-001',
				title: 'Test Task',
				weight: 'small',
				status: 'finalizing',
				branch: 'test',
				created_at: '2024-01-01',
				updated_at: '2024-01-01',
			});

			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'finalize',
					task_id: 'TASK-001',
					data: { step: 'merge', status: 'completed' },
				});
			});

			const task = useTaskStore.getState().getTask('TASK-001');
			expect(task?.status).toBe('completed');
		});

		it('should show toast on error event', async () => {
			render(
				<WebSocketProvider>
					<div>Test</div>
				</WebSocketProvider>
			);

			await act(async () => {
				mockWsInstances[0].simulateOpen();
				mockWsInstances[0].simulateMessage({
					type: 'event',
					event: 'error',
					task_id: 'TASK-001',
					data: { message: 'Something went wrong' },
				});
			});

			// Check that a toast was added
			const toasts = useUIStore.getState().toasts;
			expect(toasts.some((t) => t.message === 'Something went wrong')).toBe(true);
		});
	});
});
