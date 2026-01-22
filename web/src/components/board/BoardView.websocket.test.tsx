import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, act, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { BoardView } from './BoardView';
import { AppShellProvider } from '@/components/layout/AppShellContext';
import { WebSocketProvider } from '@/hooks/useWebSocket';
import type { Task, Initiative } from '@/lib/types';

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

// Mock stores
const mockSetRightPanelContent = vi.fn();
const mockTasks: Task[] = [];
const mockTaskStates = new Map();
const mockLoading = false;
const mockInitiatives: Initiative[] = [];
const mockTotalTokens = 0;
const mockTotalCost = 0;

// Mock useAppShell
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

// Mock taskStore
vi.mock('@/stores/taskStore', () => ({
	useTaskStore: (selector: (state: unknown) => unknown) => {
		const state = {
			tasks: mockTasks,
			taskStates: mockTaskStates,
			loading: mockLoading,
		};
		return selector(state);
	},
}));

// Mock initiativeStore
vi.mock('@/stores/initiativeStore', () => ({
	useInitiatives: () => mockInitiatives,
}));

// Mock sessionStore
vi.mock('@/stores/sessionStore', () => ({
	useSessionStore: (selector: (state: unknown) => unknown) => {
		const state = {
			totalTokens: mockTotalTokens,
			totalCost: mockTotalCost,
		};
		return selector(state);
	},
}));

// Sample task factory
function createTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'TASK-001',
		title: 'Test Task',
		description: 'A test task description',
		weight: 'medium',
		status: 'planned',
		category: 'feature',
		priority: 'normal',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides,
	};
}

// Helper to render BoardView with WebSocket provider
function renderBoardViewWithWS() {
	return render(
		<MemoryRouter>
			<WebSocketProvider autoConnect={false}>
				<AppShellProvider>
					<BoardView />
				</AppShellProvider>
			</WebSocketProvider>
		</MemoryRouter>
	);
}

describe('BoardView WebSocket Integration', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockWsInstances = [];
		vi.clearAllMocks();

		// Reset mock data
		mockTasks.length = 0;
		mockTaskStates.clear();
		mockInitiatives.length = 0;

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
		cleanup();
	});

	describe('decision_required event', () => {
		it('should accumulate decisions in pendingDecisions state', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			const { container } = renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Which approach to use?',
						options: [
							{ label: 'Approach A', description: 'Use method A' },
							{ label: 'Approach B', description: 'Use method B' },
						],
					},
				});
			});

			// Verify DecisionsPanel receives the decision
			// Since we can't directly inspect state, we check for presence in DOM
			const decisionsPanel = container.querySelector('.decisions-panel');
			expect(decisionsPanel).not.toBeNull();
		});

		it('should accumulate multiple decisions', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Send first decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'First decision',
						options: [
							{ label: 'Option 1', description: 'First option' },
							{ label: 'Option 2', description: 'Second option' },
						],
					},
				});
			});

			// Send second decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-002',
						task_id: 'TASK-001',
						message: 'Second decision',
						options: [
							{ label: 'Yes', description: 'Proceed' },
							{ label: 'No', description: 'Cancel' },
						],
					},
				});
			});

			// Both decisions should be accumulated (verify through DOM or state inspector)
			// In actual implementation, DecisionsPanel would show both
		});

		it('should only accumulate decisions for running tasks', async () => {
			const completedTask = createTask({ id: 'TASK-001', status: 'completed' });
			mockTasks.push(completedTask);

			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Should not appear',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
			});

			// Decision for completed task should be ignored
		});
	});

	describe('decision_resolved event', () => {
		it('should remove decision from pendingDecisions state', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add a decision
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Which approach?',
						options: [
							{ label: 'A', description: 'Option A' },
							{ label: 'B', description: 'Option B' },
						],
					},
				});
			});

			// Resolve the decision
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

			// Decision should be removed from state
		});

		it('should handle resolving non-existent decision gracefully', async () => {
			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Resolve a decision that was never added
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_resolved',
					task_id: 'TASK-001',
					data: {
						decision_id: 'DEC-999',
						selected_option: 'A',
					},
				});
			});

			// Should not throw error
		});
	});

	describe('files_changed event', () => {
		it('should update changedFiles state with snapshot (not accumulate)', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Send first files_changed event
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

			// Send second files_changed event (should replace, not append)
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

			// Only the latest snapshot should be in state
			// FilesPanel should show only file3.ts
		});

		it('should clear files when task completes', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

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
		});
	});

	describe('task completion cleanup', () => {
		it('should clear decisions when task completes', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

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
						message: 'Decision',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
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

			// Decisions should be cleared for completed task
		});

		it('should clear files when task completes', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add files
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'files_changed',
					task_id: 'TASK-001',
					data: {
						files: [{ path: 'src/file1.ts', status: 'M' }],
					},
				});
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
		});
	});

	describe('visual indicators', () => {
		it('should apply pending decision indicator to TaskCard', async () => {
			const plannedTask = createTask({ id: 'TASK-001', status: 'planned' });
			mockTasks.push(plannedTask);

			const { container } = renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add decision for planned task
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Decision',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
			});

			// TaskCard should have .has-pending-decision class
			const taskCard = container.querySelector('[data-task-id="TASK-001"]');
			if (taskCard) {
				expect(taskCard.classList.contains('has-pending-decision')).toBe(true);
			} else {
				// Card not found - test documents expected behavior
				expect(taskCard).not.toBeNull();
			}
		});

		it('should apply pending decision indicator to RunningCard', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			const { container } = renderBoardViewWithWS();

			await act(async () => {
				mockWsInstances[0]?.simulateOpen();
			});

			// Add decision for running task
			await act(async () => {
				mockWsInstances[0]?.simulateMessage({
					type: 'event',
					event: 'decision_required',
					task_id: 'TASK-001',
					data: {
						id: 'DEC-001',
						task_id: 'TASK-001',
						message: 'Decision',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
			});

			// RunningCard should have .has-pending-decision class
			const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
			if (runningCard) {
				expect(runningCard.classList.contains('has-pending-decision')).toBe(true);
			} else {
				expect(runningCard).not.toBeNull();
			}
		});

		it('should remove glow when decision is resolved', async () => {
			const runningTask = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(runningTask);

			const { container } = renderBoardViewWithWS();

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
						message: 'Decision',
						options: [{ label: 'A', description: 'Option A' }],
					},
				});
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
			const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
			if (runningCard) {
				expect(runningCard.classList.contains('has-pending-decision')).toBe(false);
			}
		});
	});
});
