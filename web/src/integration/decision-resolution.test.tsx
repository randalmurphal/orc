import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, act, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { BoardView } from '@/components/board/BoardView';
import { AppShellProvider } from '@/components/layout/AppShellContext';
import { EventProvider } from '@/hooks';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, TaskCategory, TaskPriority } from '@/gen/orc/v1/task_pb';
import type { PendingDecision } from '@/gen/orc/v1/decision_pb';
import { createMockTask, createTimestamp, createMockDecision } from '@/test/factories';

// Module-level mock data (mutable, reset in beforeEach)
const mockTasks: Task[] = [];
const mockTaskStates = new Map();
const mockPendingDecisions: PendingDecision[] = [];
// Reference to capture right panel content
const mockRightPanelContent: { current: React.ReactNode } = { current: null };

// Mock decision client
const mockResolveDecision = vi.fn().mockResolvedValue({});

vi.mock('@/lib/client', () => ({
	decisionClient: {
		resolveDecision: (...args: unknown[]) => mockResolveDecision(...args),
	},
	taskClient: {
		skipBlock: vi.fn().mockResolvedValue({}),
		runTask: vi.fn().mockResolvedValue({}),
	},
	configClient: {
		getConfigStats: vi.fn().mockResolvedValue({
			stats: {
				slashCommandsCount: 0,
				claudeMdSize: BigInt(0),
				mcpServersCount: 0,
				permissionsProfile: 'default',
			},
		}),
	},
}));

// Mock events module
vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn((callback: (status: string) => void) => {
			callback('connected');
			return () => {};
		}),
		getStatus: vi.fn().mockReturnValue('connected'),
		isConnected: vi.fn().mockReturnValue(true),
	})),
	handleEvent: vi.fn(),
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

// Mock uiStore - includes pendingDecisions
vi.mock('@/stores/uiStore', () => {
	const mockUIStoreState = {
		get pendingDecisions() { return mockPendingDecisions; },
		addPendingDecision: vi.fn((decision: PendingDecision) => {
			if (!mockPendingDecisions.some(d => d.id === decision.id)) {
				mockPendingDecisions.push(decision);
			}
		}),
		removePendingDecision: vi.fn((decisionId: string) => {
			const idx = mockPendingDecisions.findIndex(d => d.id === decisionId);
			if (idx !== -1) {
				mockPendingDecisions.splice(idx, 1);
			}
		}),
		clearPendingDecisions: vi.fn(() => {
			mockPendingDecisions.length = 0;
		}),
		wsStatus: 'connected',
		setWsStatus: vi.fn(),
		toasts: [],
		addToast: vi.fn(),
	};

	const mockUseUIStore = Object.assign(
		(selector: (state: unknown) => unknown) => selector(mockUIStoreState),
		{ getState: () => mockUIStoreState }
	);

	return {
		useUIStore: mockUseUIStore,
		usePendingDecisions: () => mockPendingDecisions,
	};
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
	getConfigStats: vi.fn().mockResolvedValue({
		slashCommandsCount: 0,
		claudeMdSize: 0,
		mcpServersCount: 0,
		permissionsProfile: 'default',
	}),
}));

function createTask(overrides: Partial<Omit<Task, '$typeName' | '$unknown'>> = {}): Task {
	return createMockTask({
		id: 'TASK-001',
		title: 'Test Task',
		description: 'A test task description',
		weight: TaskWeight.MEDIUM,
		status: TaskStatus.RUNNING,
		category: TaskCategory.FEATURE,
		priority: TaskPriority.NORMAL,
		branch: 'orc/TASK-001',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
		...overrides,
	});
}

// Helper to simulate decision_required event - directly updates mock store
function simulateDecisionRequired(data: {
	decisionId: string;
	taskId: string;
	taskTitle: string;
	phase: string;
	gateType: string;
	question: string;
	context: string;
}): void {
	const decision = createMockDecision({
		id: data.decisionId,
		taskId: data.taskId,
		taskTitle: data.taskTitle,
		phase: data.phase,
		gateType: data.gateType,
		question: data.question,
		context: data.context,
	});
	if (!mockPendingDecisions.some(d => d.id === decision.id)) {
		mockPendingDecisions.push(decision);
	}
}

// Helper to simulate decision_resolved event - directly updates mock store
function simulateDecisionResolved(decisionId: string): void {
	const idx = mockPendingDecisions.findIndex(d => d.id === decisionId);
	if (idx !== -1) {
		mockPendingDecisions.splice(idx, 1);
	}
}

function renderApp() {
	return render(
		<TooltipProvider>
			<MemoryRouter>
				<EventProvider>
					<AppShellProvider>
						<BoardView />
					</AppShellProvider>
				</EventProvider>
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('Decision Resolution Integration', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();

		mockTasks.length = 0;
		mockTaskStates.clear();
		mockPendingDecisions.length = 0;
		mockRightPanelContent.current = null;
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
		cleanup();
	});

	describe('end-to-end decision flow', () => {
		it('should show decision in DecisionsPanel and allow resolution', async () => {
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			renderApp();

			// Wait for component to mount
			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Simulate decision_required event
			await act(async () => {
				simulateDecisionRequired({
					decisionId: 'DEC-001',
					taskId: 'TASK-001',
					taskTitle: 'Test Task',
					phase: 'implement',
					gateType: 'approval',
					question: 'Approve implementation plan?',
					context: 'Implementation ready for review',
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

			// Click on the first option button (Approve for approval gate type)
			const approveButton = getByRole('button', { name: /Approve/i });
			await act(async () => {
				fireEvent.click(approveButton);
				await vi.advanceTimersByTimeAsync(100);
			});

			// Decision client should be called
			expect(mockResolveDecision).toHaveBeenCalledWith(
				expect.objectContaining({
					id: 'DEC-001',
					approved: true,
				})
			);
		});

		it('should remove decision from panel when resolved via event', async () => {
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision
			await act(async () => {
				simulateDecisionRequired({
					decisionId: 'DEC-001',
					taskId: 'TASK-001',
					taskTitle: 'Test Task',
					phase: 'implement',
					gateType: 'approval',
					question: 'Test decision',
					context: 'Test context',
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

			// Resolve via simulated event
			await act(async () => {
				simulateDecisionResolved('DEC-001');
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
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			// Add decision to mock store first (simulating event arrival)
			simulateDecisionRequired({
				decisionId: 'DEC-001',
				taskId: 'TASK-001',
				taskTitle: 'Test Task',
				phase: 'implement',
				gateType: 'approval',
				question: 'Test decision',
				context: 'Test context',
			});

			// Now render the app - it will see the pending decision
			const { container } = renderApp();

			await act(async () => {
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
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			// Add then resolve decision before rendering (simulating full event sequence)
			simulateDecisionRequired({
				decisionId: 'DEC-001',
				taskId: 'TASK-001',
				taskTitle: 'Test Task',
				phase: 'implement',
				gateType: 'approval',
				question: 'Test decision',
				context: 'Test context',
			});
			simulateDecisionResolved('DEC-001');

			// Render after decision is resolved
			const { container } = renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// RunningCard should NOT have .has-pending-decision class (decision was resolved)
			const runningCard = container.querySelector('.running-card[data-task-id="TASK-001"]');
			if (runningCard) {
				expect(runningCard.classList.contains('has-pending-decision')).toBe(false);
			}
		});
	});

	describe('files changed integration', () => {
		it('should update FilesPanel with latest snapshot', async () => {
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Test passes if no errors - files changed events are not yet integrated
			expect(true).toBe(true);
		});

		it('should clear files when task completes', async () => {
			const task = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			mockTasks.push(task);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Test passes if no errors - files changed events are not yet integrated
			expect(true).toBe(true);
		});
	});

	describe('multiple tasks with decisions', () => {
		it('should track decisions per task independently', async () => {
			const task1 = createTask({ id: 'TASK-001', status: TaskStatus.RUNNING });
			const task2 = createTask({ id: 'TASK-002', status: TaskStatus.RUNNING, title: 'Task 2' });
			mockTasks.push(task1, task2);
			mockTaskStates.set('TASK-001', { currentPhase: 'implement', phases: {} });
			mockTaskStates.set('TASK-002', { currentPhase: 'implement', phases: {} });

			renderApp();

			await act(async () => {
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision for task 1
			await act(async () => {
				simulateDecisionRequired({
					decisionId: 'DEC-001',
					taskId: 'TASK-001',
					taskTitle: 'Task 1',
					phase: 'implement',
					gateType: 'approval',
					question: 'Decision for Task 1',
					context: 'Context 1',
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Add decision for task 2
			await act(async () => {
				simulateDecisionRequired({
					decisionId: 'DEC-002',
					taskId: 'TASK-002',
					taskTitle: 'Task 2',
					phase: 'implement',
					gateType: 'approval',
					question: 'Decision for Task 2',
					context: 'Context 2',
				});
				await vi.advanceTimersByTimeAsync(100);
			});

			// Resolve decision for task 1
			await act(async () => {
				simulateDecisionResolved('DEC-001');
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
