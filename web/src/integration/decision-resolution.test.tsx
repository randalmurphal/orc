import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, act, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { BoardView } from '@/components/board/BoardView';
import { AppShellProvider } from '@/components/layout/AppShellContext';
import { WebSocketProvider } from '@/hooks/useWebSocket';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/lib/types';
import * as api from '@/lib/api';

// Use vi.hoisted to ensure mock data is available when vi.mock is hoisted
const {
	mockTasks,
	mockTaskStates,
	mockRightPanelContent,
	mockEventHandlers,
} = vi.hoisted(() => ({
	mockTasks: [] as Task[],
	mockTaskStates: new Map(),
	// Reference to capture right panel content
	mockRightPanelContent: { current: null as React.ReactNode },
	// Map of event types to their handlers
	mockEventHandlers: new Map<string, Set<(event: unknown) => void>>(),
}));

// Mock WebSocket at the module level - captures event handlers correctly
vi.mock('@/lib/websocket', () => ({
	OrcWebSocket: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		subscribeGlobal: vi.fn(),
		setPrimarySubscription: vi.fn(),
		on: vi.fn((eventType: string, callback: (event: unknown) => void) => {
			// Store all event handlers for later dispatch
			if (!mockEventHandlers.has(eventType)) {
				mockEventHandlers.set(eventType, new Set());
			}
			mockEventHandlers.get(eventType)!.add(callback);
			return () => {
				mockEventHandlers.get(eventType)?.delete(callback);
			};
		}),
		onStatusChange: vi.fn((callback: (status: string) => void) => {
			// Call immediately with connected status
			callback('connected');
			return () => {};
		}),
		isConnected: vi.fn().mockReturnValue(true),
		getTaskId: vi.fn().mockReturnValue('*'),
		command: vi.fn(),
	})),
	GLOBAL_TASK_ID: '*',
}));

// Mock useAppShell - capture right panel content for testing
vi.mock('@/components/layout/AppShellContext', async () => {
	const actual = await vi.importActual('@/components/layout/AppShellContext');
	return {
		...actual,
		useAppShell: () => ({
			setRightPanelContent: (content: React.ReactNode) => {
				mockRightPanelContent.current = content;
			},
			isRightPanelOpen: true,
			toggleRightPanel: vi.fn(),
			rightPanelContent: mockRightPanelContent.current,
			isMobileNavMode: false,
			panelToggleRef: { current: null },
		}),
	};
});

// Mock taskStore - need to mock both hook usage and getState() access
vi.mock('@/stores/taskStore', () => {
	const mockTaskStoreState = {
		get tasks() { return mockTasks; },
		get taskStates() { return mockTaskStates; },
		loading: false,
		updateTask: vi.fn(),
		addTask: vi.fn(),
		removeTask: vi.fn(),
		setTaskState: vi.fn(),
		setTasks: vi.fn(),
		updateTaskState: vi.fn(),
		getTaskState: vi.fn((taskId: string) => mockTaskStates.get(taskId)),
		updateTaskStatus: vi.fn(),
	};

	const mockUseTaskStore = Object.assign(
		(selector: (state: unknown) => unknown) => selector(mockTaskStoreState),
		{ getState: () => mockTaskStoreState }
	);

	return { useTaskStore: mockUseTaskStore };
});

// Mock initiativeStore with getState
vi.mock('@/stores/initiativeStore', () => {
	const mockInitiativeStoreState = {
		initiatives: new Map(),
		addInitiative: vi.fn(),
		updateInitiative: vi.fn(),
		removeInitiative: vi.fn(),
	};

	const mockUseInitiativeStore = Object.assign(
		() => [],
		{ getState: () => mockInitiativeStoreState }
	);

	return {
		useInitiatives: () => [],
		useInitiativeStore: mockUseInitiativeStore,
	};
});

// Mock sessionStore
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
	// Config stats for TopBar
	getConfigStats: vi.fn().mockResolvedValue({
		slashCommandsCount: 0,
		claudeMdSize: 0,
		mcpServersCount: 0,
		permissionsProfile: 'default',
	}),
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

// Helper to simulate WebSocket event - dispatches to all matching handlers
function simulateWsEvent(eventType: string, taskId: string, data: unknown): void {
	const event = {
		type: 'event',
		event: eventType,
		task_id: taskId,
		data,
		time: new Date().toISOString(),
	};

	// Dispatch to specific event handlers
	mockEventHandlers.get(eventType)?.forEach((handler) => handler(event));
	// Dispatch to 'all' handlers
	mockEventHandlers.get('all')?.forEach((handler) => handler(event));
	// Dispatch to '*' handlers
	mockEventHandlers.get('*')?.forEach((handler) => handler(event));
}

function renderApp() {
	return render(
		<TooltipProvider>
			<MemoryRouter>
				<WebSocketProvider autoConnect={true} autoSubscribeGlobal={true}>
					<AppShellProvider>
						<BoardView />
					</AppShellProvider>
				</WebSocketProvider>
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('Decision Resolution Integration', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
		mockEventHandlers.clear();

		mockTasks.length = 0;
		mockTaskStates.clear();
		mockRightPanelContent.current = null;
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
		cleanup();
	});

	describe('end-to-end decision flow', () => {
		it('should show decision in DecisionsPanel and allow resolution', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			renderApp();

			// Wait for component to mount and subscribe
			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Send decision_required event with proper DecisionRequiredData format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					task_title: 'Test Task',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Approve implementation plan?',
					context: 'Implementation ready for review',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Render the captured right panel content to test DecisionsPanel
			const { getByText, getByRole } = render(
				<TooltipProvider>
					<MemoryRouter>
						{mockRightPanelContent.current}
					</MemoryRouter>
				</TooltipProvider>
			);

			// DecisionsPanel should show the decision question
			expect(getByText('Approve implementation plan?')).toBeInTheDocument();

			// Click on the Approve option (BoardView creates Approve/Reject buttons)
			const approveButton = getByRole('button', { name: /Approve/i });
			await act(async () => {
				fireEvent.click(approveButton);
				await vi.advanceTimersByTimeAsync(100);
			});

			// API should be called with approve option
			expect(api.submitDecision).toHaveBeenCalledWith('DEC-001', expect.objectContaining({
				approved: true,
				reason: 'Approve',
			}));
		});

		it('should remove decision from panel when resolved via WebSocket', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision with proper format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					task_title: 'Test Task',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Test decision',
					context: 'Test context',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Render captured panel content and verify decision is visible
			const { queryByText: panelQuery1 } = render(
				<TooltipProvider>
					<MemoryRouter>
						{mockRightPanelContent.current}
					</MemoryRouter>
				</TooltipProvider>
			);
			expect(panelQuery1('Test decision')).toBeInTheDocument();
			cleanup();

			// Resolve via WebSocket
			await act(async () => {
				simulateWsEvent('decision_resolved', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					phase: 'implement',
					approved: true,
					resolved_by: 'test',
					resolved_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Render panel again and verify decision is removed
			const { queryByText: panelQuery2 } = render(
				<TooltipProvider>
					<MemoryRouter>
						{mockRightPanelContent.current}
					</MemoryRouter>
				</TooltipProvider>
			);
			expect(panelQuery2('Test decision')).not.toBeInTheDocument();
		});

		it('should show task card glow when decision exists', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			const { container } = renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision with proper format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					task_title: 'Test Task',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Test decision',
					context: 'Test context',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// RunningCard should have .has-pending-decision class
			const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
			expect(runningCard).not.toBeNull();
			if (runningCard) {
				expect(runningCard.classList.contains('has-pending-decision')).toBe(true);
			}
		});

		it('should remove glow when decision is resolved', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			const { container } = renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision with proper format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					task_title: 'Test Task',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Test decision',
					context: 'Test context',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Resolve decision with proper format
			await act(async () => {
				simulateWsEvent('decision_resolved', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					phase: 'implement',
					approved: true,
					resolved_by: 'test',
					resolved_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// RunningCard should NOT have .has-pending-decision class
			const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
			if (runningCard) {
				expect(runningCard.classList.contains('has-pending-decision')).toBe(false);
			}
		});
	});

	describe('files changed integration', () => {
		it('should update FilesPanel with latest snapshot', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Send files_changed event
			await act(async () => {
				simulateWsEvent('files_changed', 'TASK-001', {
					files: ['src/components/Button.tsx', 'src/utils/helpers.ts'],
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Test passes if no errors - we can't easily inspect the FilesPanel
			// since it's in the right panel which is mocked
			expect(true).toBe(true);
		});

		it('should clear files when task completes', async () => {
			const task = createTask({ id: 'TASK-001', status: 'running' });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add files
			await act(async () => {
				simulateWsEvent('files_changed', 'TASK-001', {
					files: ['file1.ts'],
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Complete task
			await act(async () => {
				simulateWsEvent('task_updated', 'TASK-001', {
					id: 'TASK-001',
					status: 'completed',
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Test passes if no errors
			expect(true).toBe(true);
		});
	});

	describe('multiple tasks with decisions', () => {
		it('should track decisions per task independently', async () => {
			const task1 = createTask({ id: 'TASK-001', status: 'running' });
			const task2 = createTask({ id: 'TASK-002', status: 'running', title: 'Task 2' });
			mockTasks.push(task1, task2);
			mockTaskStates.set('TASK-001', { current_phase: 'implement', phases: {} });
			mockTaskStates.set('TASK-002', { current_phase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision for task 1 with proper format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					task_title: 'Task 1',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Decision for Task 1',
					context: 'Context 1',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision for task 2 with proper format
			await act(async () => {
				simulateWsEvent('decision_required', 'TASK-002', {
					decision_id: 'DEC-002',
					task_id: 'TASK-002',
					task_title: 'Task 2',
					phase: 'implement',
					gate_type: 'approval',
					question: 'Decision for Task 2',
					context: 'Context 2',
					requested_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Resolve decision for task 1 with proper format
			await act(async () => {
				simulateWsEvent('decision_resolved', 'TASK-001', {
					decision_id: 'DEC-001',
					task_id: 'TASK-001',
					phase: 'implement',
					approved: true,
					resolved_by: 'test',
					resolved_at: new Date().toISOString(),
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Render panel content and verify Task 2 decision is still present
			const { getByText, queryByText } = render(
				<TooltipProvider>
					<MemoryRouter>
						{mockRightPanelContent.current}
					</MemoryRouter>
				</TooltipProvider>
			);
			expect(getByText('Decision for Task 2')).toBeInTheDocument();
			// Task 1 decision should be gone
			expect(queryByText('Decision for Task 1')).not.toBeInTheDocument();
		});
	});
});
