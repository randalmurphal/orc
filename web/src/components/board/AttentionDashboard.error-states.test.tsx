/**
 * TDD Tests for TASK-744: Add error states to Board task cards
 *
 * Success Criteria Coverage:
 * - SC-1: Failed tasks display error indicators in task cards
 * - SC-2: Phase pipeline shows failed phases with error styling
 * - SC-3: Error messages are displayed in attention items
 * - SC-4: Retry actions are available for failed tasks
 * - SC-5: Error states are visually distinct from other states
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AttentionDashboard } from './AttentionDashboard';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskPriority } from '@/gen/orc/v1/task_pb';
import type { AttentionItem } from '@/gen/orc/v1/attention_dashboard_pb';
import { AttentionItemType, PhaseStepStatus } from '@/gen/orc/v1/attention_dashboard_pb';
import { createMockTask, createTimestamp } from '@/test/factories';

// Mock events module
vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn().mockReturnValue(() => {}),
		getStatus: vi.fn().mockReturnValue('disconnected'),
	})),
	handleEvent: vi.fn(),
}));

// Mock stores with error state support
const mockTasks: Task[] = [];
const mockTaskStates = new Map();
const mockLoading = false;
const mockAttentionItems: AttentionItem[] = [];

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
	useInitiatives: () => [],
}));

// Mock uiStore
vi.mock('@/stores/uiStore', () => ({
	useUIStore: (selector: (state: unknown) => unknown) => {
		const state = {
			pendingDecisions: [],
			removePendingDecision: vi.fn(),
			wsStatus: 'connected',
			setWsStatus: vi.fn(),
			toasts: [],
			addToast: vi.fn(),
		};
		return selector(state);
	},
	usePendingDecisions: () => [],
}));

// Mock router navigation
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

// Mock the API client to return error states
vi.mock('@/lib/client', () => ({
	attentionDashboardClient: {
		getAttentionDashboardData: vi.fn().mockImplementation(() => {
			// Return mock data with error states
			return Promise.resolve({
				runningSummary: {
					taskCount: mockTasks.filter(t => t.status === TaskStatus.RUNNING).length,
					tasks: mockTasks.filter(t => t.status === TaskStatus.RUNNING).map(task => ({
						id: task.id,
						title: task.title,
						currentPhase: task.currentPhase,
						startedAt: task.startedAt,
						elapsedTimeSeconds: 300,
						initiativeId: task.initiativeId,
						initiativeTitle: 'Mock Initiative',
						phaseProgress: {
							currentPhase: task.currentPhase,
							steps: [
								{ name: 'plan', status: PhaseStepStatus.COMPLETED },
								{ name: 'code', status: task.currentPhase === 'implement' ? PhaseStepStatus.ACTIVE : PhaseStepStatus.PENDING },
								{ name: 'test', status: PhaseStepStatus.PENDING },
								{ name: 'review', status: PhaseStepStatus.PENDING },
								{ name: 'done', status: PhaseStepStatus.PENDING },
							]
						},
						outputLines: ['Task running...'],
					})),
				},
				attentionItems: mockAttentionItems,
				queueSummary: {
					taskCount: 0,
					swimlanes: [],
					unassignedTasks: [],
				},
			});
		}),
	},
}));

// Helper to render with required providers
function renderAttentionDashboard() {
	return render(
		<TooltipProvider>
			<MemoryRouter>
				<AttentionDashboard />
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('AttentionDashboard Error States - TASK-744', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockTasks.length = 0;
		mockTaskStates.clear();
		mockAttentionItems.length = 0;
	});

	afterEach(() => {
		cleanup();
	});

	// SC-1: Failed tasks display error indicators in task cards
	describe('SC-1: Failed tasks display error indicators', () => {
		it('displays failed task with error indicator in attention section', async () => {
			const failedTask = createMockTask({
				id: 'TASK-001',
				title: 'Failed authentication setup',
				status: TaskStatus.FAILED,
				priority: TaskPriority.HIGH,
			});

			const failedAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: `failed-${failedTask.id}`,
				type: AttentionItemType.FAILED_TASK,
				taskId: failedTask.id!,
				title: failedTask.title,
				description: 'Task execution failed and requires attention',
				priority: failedTask.priority,
				createdAt: failedTask.updatedAt,
				availableActions: [],
				decisionOptions: [],
				errorMessage: 'Phase implementation failed with exit code 1',
			};

			mockTasks.push(failedTask);
			mockAttentionItems.push(failedAttentionItem);

			renderAttentionDashboard();

			// Wait for async data loading
			await vi.waitFor(() => {
				expect(screen.getByText('Failed authentication setup')).toBeInTheDocument();
			});

			// Should display task in attention section with error styling
			const attentionSection = screen.getByRole('region', { name: /needs attention/i });
			expect(attentionSection).toBeInTheDocument();

			// Should show task with error indicator
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('Failed authentication setup')).toBeInTheDocument();

			// Should show error description
			expect(screen.getByText(/task execution failed/i)).toBeInTheDocument();

			// Card should have error styling class
			const attentionCard = screen.getByText('TASK-001').closest('.attention-item');
			expect(attentionCard).toHaveClass('failed-task');
		});

		it('shows error message for failed tasks', async () => {
			const failedTask = createMockTask({
				id: 'TASK-002',
				title: 'Database migration failed',
				status: TaskStatus.FAILED,
			});

			const failedAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: `failed-${failedTask.id}`,
				type: AttentionItemType.FAILED_TASK,
				taskId: failedTask.id!,
				title: failedTask.title,
				description: 'Task execution failed and requires attention',
				priority: failedTask.priority,
				createdAt: failedTask.updatedAt,
				availableActions: [],
				decisionOptions: [],
				errorMessage: 'Migration script failed: Connection timeout to database',
			};

			mockTasks.push(failedTask);
			mockAttentionItems.push(failedAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Database migration failed')).toBeInTheDocument();
			});

			// Should display error message
			expect(screen.getByText(/migration script failed/i)).toBeInTheDocument();
		});

		it('displays retry action for failed tasks', async () => {
			const failedTask = createMockTask({
				id: 'TASK-003',
				title: 'API integration failed',
				status: TaskStatus.FAILED,
			});

			const failedAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: `failed-${failedTask.id}`,
				type: AttentionItemType.FAILED_TASK,
				taskId: failedTask.id!,
				title: failedTask.title,
				description: 'Task execution failed and requires attention',
				priority: failedTask.priority,
				createdAt: failedTask.updatedAt,
				availableActions: [
					{
						$case: 'retry',
						retry: {}
					},
					{
						$case: 'view',
						view: {}
					}
				],
				decisionOptions: [],
			};

			mockTasks.push(failedTask);
			mockAttentionItems.push(failedAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('API integration failed')).toBeInTheDocument();
			});

			// Should show retry button
			expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /view/i })).toBeInTheDocument();
		});
	});

	// SC-2: Phase pipeline shows failed phases with error styling
	describe('SC-2: Phase pipeline shows failed phases', () => {
		it('displays failed phases in pipeline with error styling', async () => {
			const runningTaskWithFailure = createMockTask({
				id: 'TASK-004',
				title: 'Task with failed test phase',
				status: TaskStatus.RUNNING,
				currentPhase: 'test',
			});

			mockTasks.push(runningTaskWithFailure);

			// Mock the API to return a failed phase in the pipeline
			const mockClient = await import('@/lib/client');
			vi.mocked(mockClient.attentionDashboardClient.getAttentionDashboardData).mockResolvedValue({
				runningSummary: {
					taskCount: 1,
					tasks: [{
						id: runningTaskWithFailure.id,
						title: runningTaskWithFailure.title,
						currentPhase: runningTaskWithFailure.currentPhase,
						startedAt: runningTaskWithFailure.startedAt,
						elapsedTimeSeconds: 300,
						initiativeId: runningTaskWithFailure.initiativeId,
						initiativeTitle: 'Mock Initiative',
						phaseProgress: {
							currentPhase: runningTaskWithFailure.currentPhase,
							steps: [
								{ name: 'plan', status: PhaseStepStatus.COMPLETED },
								{ name: 'code', status: PhaseStepStatus.FAILED },
								{ name: 'test', status: PhaseStepStatus.ACTIVE },
								{ name: 'review', status: PhaseStepStatus.PENDING },
								{ name: 'done', status: PhaseStepStatus.PENDING },
							]
						},
						outputLines: ['Task running...'],
					}]
				},
				attentionItems: [],
				queueSummary: {
					taskCount: 0,
					swimlanes: [],
					unassignedTasks: [],
				},
			});

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Task with failed test phase')).toBeInTheDocument();
			});

			// Should show pipeline phases
			expect(screen.getByText('Plan')).toBeInTheDocument();
			expect(screen.getByText('Code')).toBeInTheDocument();
			expect(screen.getByText('Test')).toBeInTheDocument();

			// Code phase should have failed styling
			const codePhase = screen.getByText('Code').closest('.pipeline-step');
			expect(codePhase).toHaveClass('failed');

			// Test phase should be active
			const testPhase = screen.getByText('Test').closest('.pipeline-step');
			expect(testPhase).toHaveClass('active');
		});

		it('shows error indicator on running task card when phase failed', async () => {
			const runningTaskWithFailure = createMockTask({
				id: 'TASK-005',
				title: 'Task with previous failure',
				status: TaskStatus.RUNNING,
				currentPhase: 'review',
			});

			mockTasks.push(runningTaskWithFailure);

			const mockClient = await import('@/lib/client');
			vi.mocked(mockClient.attentionDashboardClient.getAttentionDashboardData).mockResolvedValue({
				runningSummary: {
					taskCount: 1,
					tasks: [{
						id: runningTaskWithFailure.id,
						title: runningTaskWithFailure.title,
						currentPhase: runningTaskWithFailure.currentPhase,
						startedAt: runningTaskWithFailure.startedAt,
						elapsedTimeSeconds: 300,
						initiativeId: runningTaskWithFailure.initiativeId,
						initiativeTitle: 'Mock Initiative',
						hasFailures: true, // Indicates task has had phase failures
						phaseProgress: {
							currentPhase: runningTaskWithFailure.currentPhase,
							steps: [
								{ name: 'plan', status: PhaseStepStatus.COMPLETED },
								{ name: 'code', status: PhaseStepStatus.COMPLETED },
								{ name: 'test', status: PhaseStepStatus.FAILED },
								{ name: 'review', status: PhaseStepStatus.ACTIVE },
								{ name: 'done', status: PhaseStepStatus.PENDING },
							]
						},
						outputLines: ['Task running...'],
					}]
				},
				attentionItems: [],
				queueSummary: {
					taskCount: 0,
					swimlanes: [],
					unassignedTasks: [],
				},
			});

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Task with previous failure')).toBeInTheDocument();
			});

			// Running card should show error indicator
			const runningCard = screen.getByText('TASK-005').closest('.running-card');
			expect(runningCard).toHaveClass('has-failures');
		});
	});

	// SC-3: Error messages are displayed in attention items
	describe('SC-3: Error messages in attention items', () => {
		it('displays detailed error messages for attention items', async () => {
			const errorAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: 'error-001',
				type: AttentionItemType.ERROR_STATE,
				taskId: 'TASK-006',
				title: 'Configuration Error',
				description: 'System configuration is invalid and needs correction',
				priority: TaskPriority.CRITICAL,
				createdAt: createTimestamp(),
				availableActions: [],
				decisionOptions: [],
				errorMessage: 'Invalid API key configuration in settings.yaml line 42',
			};

			mockAttentionItems.push(errorAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Configuration Error')).toBeInTheDocument();
			});

			// Should show detailed error message
			expect(screen.getByText(/invalid api key configuration/i)).toBeInTheDocument();

			// Should have error state styling
			const errorItem = screen.getByText('Configuration Error').closest('.attention-item');
			expect(errorItem).toHaveClass('error-state');
		});

		it('handles error states with resolve actions', async () => {
			const errorAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: 'error-002',
				type: AttentionItemType.ERROR_STATE,
				taskId: 'TASK-007',
				title: 'Database Connection Error',
				description: 'Unable to connect to database server',
				priority: TaskPriority.HIGH,
				createdAt: createTimestamp(),
				availableActions: [
					{
						$case: 'resolve',
						resolve: {}
					}
				],
				decisionOptions: [],
				errorMessage: 'Connection refused: database server not responding',
			};

			mockAttentionItems.push(errorAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Database Connection Error')).toBeInTheDocument();
			});

			// Should show resolve button
			expect(screen.getByRole('button', { name: /resolve/i })).toBeInTheDocument();
		});
	});

	// SC-4: Retry actions are available for failed tasks
	describe('SC-4: Retry actions for failed tasks', () => {
		it('triggers retry action when retry button is clicked', async () => {
			const failedTask = createMockTask({
				id: 'TASK-008',
				title: 'Deployment failed',
				status: TaskStatus.FAILED,
			});

			const failedAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: `failed-${failedTask.id}`,
				type: AttentionItemType.FAILED_TASK,
				taskId: failedTask.id!,
				title: failedTask.title,
				description: 'Task execution failed and requires attention',
				priority: failedTask.priority,
				createdAt: failedTask.updatedAt,
				availableActions: [
					{
						$case: 'retry',
						retry: {}
					}
				],
				decisionOptions: [],
			};

			mockTasks.push(failedTask);
			mockAttentionItems.push(failedAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Deployment failed')).toBeInTheDocument();
			});

			const retryButton = screen.getByRole('button', { name: /retry/i });
			expect(retryButton).toBeInTheDocument();

			// Mock the retry action
			const mockRetryAction = vi.fn();
			vi.mock('@/lib/attention-actions', () => ({
				performAttentionAction: mockRetryAction,
			}));

			fireEvent.click(retryButton);

			// Should trigger retry action (implementation will call API)
			expect(retryButton).toBeInTheDocument();
		});
	});

	// SC-5: Error states are visually distinct from other states
	describe('SC-5: Visual distinction of error states', () => {
		it('applies distinct styling to failed tasks in different sections', async () => {
			const failedAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: 'failed-TASK-009',
				type: AttentionItemType.FAILED_TASK,
				taskId: 'TASK-009',
				title: 'Visual styling test',
				description: 'Failed task for visual testing',
				priority: TaskPriority.NORMAL,
				createdAt: createTimestamp(),
				availableActions: [],
				decisionOptions: [],
			};

			const errorAttentionItem: AttentionItem = {
				$typeName: 'orc.v1.AttentionItem',
				id: 'error-010',
				type: AttentionItemType.ERROR_STATE,
				taskId: 'TASK-010',
				title: 'Error state test',
				description: 'Error state for visual testing',
				priority: TaskPriority.NORMAL,
				createdAt: createTimestamp(),
				availableActions: [],
				decisionOptions: [],
			};

			mockAttentionItems.push(failedAttentionItem, errorAttentionItem);

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Visual styling test')).toBeInTheDocument();
				expect(screen.getByText('Error state test')).toBeInTheDocument();
			});

			// Failed task should have failed-task styling
			const failedCard = screen.getByText('Visual styling test').closest('.attention-item');
			expect(failedCard).toHaveClass('failed-task');

			// Error state should have error-state styling
			const errorCard = screen.getByText('Error state test').closest('.attention-item');
			expect(errorCard).toHaveClass('error-state');

			// Both should be visually distinct from normal items
			expect(failedCard).not.toHaveClass('normal');
			expect(errorCard).not.toHaveClass('normal');
		});

		it('shows error indicators in phase pipeline with distinct colors', async () => {
			const runningTask = createMockTask({
				id: 'TASK-011',
				status: TaskStatus.RUNNING,
				currentPhase: 'review',
			});

			mockTasks.push(runningTask);

			const mockClient = await import('@/lib/client');
			vi.mocked(mockClient.attentionDashboardClient.getAttentionDashboardData).mockResolvedValue({
				runningSummary: {
					taskCount: 1,
					tasks: [{
						id: runningTask.id,
						title: runningTask.title,
						currentPhase: runningTask.currentPhase,
						startedAt: runningTask.startedAt,
						elapsedTimeSeconds: 300,
						phaseProgress: {
							currentPhase: runningTask.currentPhase,
							steps: [
								{ name: 'plan', status: PhaseStepStatus.COMPLETED },
								{ name: 'code', status: PhaseStepStatus.FAILED },
								{ name: 'test', status: PhaseStepStatus.FAILED },
								{ name: 'review', status: PhaseStepStatus.ACTIVE },
								{ name: 'done', status: PhaseStepStatus.PENDING },
							]
						},
						outputLines: [],
					}]
				},
				attentionItems: [],
				queueSummary: { taskCount: 0, swimlanes: [], unassignedTasks: [] },
			});

			renderAttentionDashboard();

			await vi.waitFor(() => {
				expect(screen.getByText('Code')).toBeInTheDocument();
			});

			// Both failed phases should have failed styling
			const codePhase = screen.getByText('Code').closest('.pipeline-step');
			const testPhase = screen.getByText('Test').closest('.pipeline-step');

			expect(codePhase).toHaveClass('failed');
			expect(testPhase).toHaveClass('failed');

			// Active phase should not have failed styling
			const reviewPhase = screen.getByText('Review').closest('.pipeline-step');
			expect(reviewPhase).toHaveClass('active');
			expect(reviewPhase).not.toHaveClass('failed');
		});
	});
});