import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { BoardView } from '@/components/board/BoardView';
import { AppShellProvider } from '@/components/layout/AppShellContext';
import { WebSocketProvider } from '@/hooks/useWebSocket';
import type { Task } from '@/lib/types';
import * as api from '@/lib/api';

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
}

let mockWsInstances: MockWebSocket[] = [];

// Mock stores
const mockSetRightPanelContent = vi.fn();
const mockTasks: Task[] = [];
const mockTaskStates = new Map();

vi.mock('@/components/layout/AppShellContext', async () => {
	const actual = await vi.importActual('@/components/layout/AppShellContext');
	return {
		...actual,
		useAppShell: () => ({
			setRightPanelContent: mockSetRightPanelContent,
			isRightPanelOpen: true,
			toggleRightPanel: vi.fn(),
			rightPanelContent: null,
			isMobileNavMode: false,
			panelToggleRef: { current: null },
		}),
	};
});

vi.mock('@/stores/taskStore', () => ({
	useTaskStore: (selector: (state: unknown) => unknown) => {
		const state = {
			tasks: mockTasks,
			taskStates: mockTaskStates,
			loading: false,
		};
		return selector(state);
	},
}));

vi.mock('@/stores/initiativeStore', () => ({
	useInitiatives: () => [],
}));

vi.mock('@/stores/sessionStore', () => ({
	useSessionStore: (selector: (state: unknown) => unknown) => {
		const state = {
			totalTokens: 0,
			totalCost: 0,
		};
		return selector(state);
	},
}));

// Mock API
vi.mock('@/lib/api', () => ({
	submitDecision: vi.fn(),
}));

function createTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'TASK-001',
		title: 'Test Task',
		description: 'A test task description',
		weight: 'medium',
		status: 'running',
		category: 'feature',
		priority: 'normal',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides,
	};
}

function renderApp() {
	return render(
		<MemoryRouter>
			<WebSocketProvider>
				<AppShellProvider>
					<BoardView />
				</AppShellProvider>
			</WebSocketProvider>
		</MemoryRouter>
	);
}

describe('Decision Resolution Integration', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockWsInstances = [];
		vi.clearAllMocks();

		mockTasks.length = 0;
		mockTaskStates.clear();

		globalThis.WebSocket = vi.fn((url: string) => {
			const ws = new MockWebSocket(url);
			mockWsInstances.push(ws);
			return ws;
		}) as unknown as typeof WebSocket;

		(globalThis.WebSocket as unknown as Record<string, number>).OPEN = MockWebSocket.OPEN;
		(globalThis.WebSocket as unknown as Record<string, number>).CLOSED = MockWebSocket.CLOSED;
		(globalThis.WebSocket as unknown as Record<string, number>).CONNECTING =
			MockWebSocket.CONNECTING;
		(globalThis.WebSocket as unknown as Record<string, number>).CLOSING = MockWebSocket.CLOSING;

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

	describe('end-to-end decision flow', () => {
		it('should show decision in DecisionsPanel and allow resolution', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			renderApp();

			// Open WebSocket connection
			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Send decision_required event
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Which database to use?',
						options: [
							{ label: 'PostgreSQL', description: 'Robust SQL database' },
							{ label: 'SQLite', description: 'Lightweight embedded database' },
						],
					},
				});
			});

			// DecisionsPanel should show the decision
			await waitFor(() => {
				expect(screen.getByText('Which database to use?')).toBeInTheDocument();
			});

			// Click on the first option
			const user = userEvent.setup({ delay: null });
			const optionButton = screen.getByRole('button', { name: /PostgreSQL/i });
			await user.click(optionButton);

			// API should be called
			expect(api.submitDecision).toHaveBeenCalledWith('DEC-001', expect.objectContaining({
				selected_option: 'PostgreSQL',
			}));
		});

		it('should remove decision from panel when resolved via WebSocket', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Test decision',
						options: [
							{ label: 'Yes', description: 'Proceed' },
							{ label: 'No', description: 'Cancel' },
						],
					},
				});
			});

			// Verify decision is visible
			await waitFor(() => {
				expect(screen.getByText('Test decision')).toBeInTheDocument();
			});

			// Resolve via WebSocket
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_resolved',
					task_id: 'TASK-001',
					data: {
						decision_id: 'DEC-001',
						selected_option: 'Yes',
					},
				});
			});

			// Decision should be removed
			await waitFor(() => {
				expect(screen.queryByText('Test decision')).not.toBeInTheDocument();
			});
		});

		it('should show task card glow when decision exists', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			const { container } = renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Test decision',
						options: [
							{ label: 'A', description: 'Option A' },
							{ label: 'B', description: 'Option B' },
						],
					},
				});
			});

			// Task card should have glow
			await waitFor(() => {
				const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
				expect(runningCard).toHaveClass('has-pending-decision');
			});
		});

		it('should remove glow when decision is resolved', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			const { container } = renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Test decision',
						options: [
							{ label: 'A', description: 'Option A' },
							{ label: 'B', description: 'Option B' },
						],
					},
				});
			});

			// Wait for glow
			await waitFor(() => {
				const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
				expect(runningCard).toHaveClass('has-pending-decision');
			});

			// Resolve decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_resolved',
					task_id: 'TASK-001',
					data: {
						decision_id: 'DEC-001',
						selected_option: 'A',
					},
				});
			});

			// Glow should be removed
			await waitFor(() => {
				const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
				expect(runningCard).not.toHaveClass('has-pending-decision');
			});
		});
	});

	describe('files changed integration', () => {
		it('should update FilesPanel with latest snapshot', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Send files_changed event
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'files_changed',
					task_id: 'TASK-001',
					data: {
						files: [
							{ path: 'src/file1.ts', status: 'M' },
							{ path: 'src/file2.ts', status: 'A' },
						],
					},
				});
			});

			// FilesPanel should show files
			await waitFor(() => {
				expect(screen.getByText('src/file1.ts')).toBeInTheDocument();
				expect(screen.getByText('src/file2.ts')).toBeInTheDocument();
			});

			// Send another files_changed (should replace)
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'files_changed',
					task_id: 'TASK-001',
					data: {
						files: [
							{ path: 'src/file3.ts', status: 'M' },
						],
					},
				});
			});

			// Only new snapshot should be visible
			await waitFor(() => {
				expect(screen.getByText('src/file3.ts')).toBeInTheDocument();
				expect(screen.queryByText('src/file1.ts')).not.toBeInTheDocument();
				expect(screen.queryByText('src/file2.ts')).not.toBeInTheDocument();
			});
		});

		it('should clear files when task completes', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);

			renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Send files_changed
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'files_changed',
					task_id: 'TASK-001',
					data: {
						files: [
							{ path: 'src/file1.ts', status: 'M' },
						],
					},
				});
			});

			await waitFor(() => {
				expect(screen.getByText('src/file1.ts')).toBeInTheDocument();
			});

			// Task completes
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'complete',
					task_id: 'TASK-001',
					data: { status: 'completed', phase: 'finalize' },
				});
			});

			// Files should be cleared
			await waitFor(() => {
				expect(screen.queryByText('src/file1.ts')).not.toBeInTheDocument();
			});
		});
	});

	describe('multiple tasks with decisions', () => {
		it('should track decisions per task independently', async () => {
			const task1 = createTask({ id: 'TASK-001', status: 'running' });
			const task2 = createTask({ id: 'TASK-002', status: 'running', title: 'Second Task' });
			mockTasks.push(task1, task2);

			renderApp();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Decision for task 1
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Decision for task 1',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
			});

			// Decision for task 2
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-002',
					data: {
						id: 'DEC-002',
						task_id: 'TASK-002',
						message: 'Decision for task 2',
						options: [{ label: 'B', description: 'Option B' }],
					},
				});
			});

			// Both decisions should be visible
			await waitFor(() => {
				expect(screen.getByText('Decision for task 1')).toBeInTheDocument();
				expect(screen.getByText('Decision for task 2')).toBeInTheDocument();
			});

			// Resolve task 1 decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_resolved',
					task_id: 'TASK-001',
					data: {
						decision_id: 'DEC-001',
						selected_option: 'A',
					},
				});
			});

			// Only task 2 decision should remain
			await waitFor(() => {
				expect(screen.queryByText('Decision for task 1')).not.toBeInTheDocument();
				expect(screen.getByText('Decision for task 2')).toBeInTheDocument();
			});
		});
	});
});
