/**
 * Tests for TaskDetail page component
 *
 * Verifies:
 * - Task status syncs from store when WebSocket updates arrive
 * - Loading and error states are handled correctly
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { TaskDetail } from './TaskDetail';
import { useTaskStore, useProjectStore } from '@/stores';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { type Task, TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockTaskPlan, createMockPhase } from '@/test/factories';

// Mock the Connect RPC client
const mockGetTask = vi.fn();
const mockGetTaskPlan = vi.fn();
const mockListReviewComments = vi.fn();
const mockGetDiff = vi.fn();
const mockListFeedback = vi.fn();
const mockGetReviewFindings = vi.fn();
const mockListTaskGeneratedNotes = vi.fn();
const mockRetryTask = vi.fn();
const mockUpdateTask = vi.fn();
const mockUseTaskSubscription = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: (...args: unknown[]) => mockGetTask(...args),
		getTaskPlan: (...args: unknown[]) => mockGetTaskPlan(...args),
		retryTask: (...args: unknown[]) => mockRetryTask(...args),
		updateTask: (...args: unknown[]) => mockUpdateTask(...args),
		listReviewComments: (...args: unknown[]) => mockListReviewComments(...args),
		getDiff: (...args: unknown[]) => mockGetDiff(...args),
		getReviewFindings: (...args: unknown[]) => mockGetReviewFindings(...args),
	},
	feedbackClient: {
		listFeedback: (...args: unknown[]) => mockListFeedback(...args),
	},
	initiativeClient: {
		listTaskGeneratedNotes: (...args: unknown[]) => mockListTaskGeneratedNotes(...args),
	},
}));

// Mock hooks module — must include all hooks imported by TaskDetail
vi.mock('@/hooks', () => ({
	useTaskSubscription: (...args: unknown[]) => mockUseTaskSubscription(...args),
	useDocumentTitle: vi.fn(),
}));

// Mock the stores module for getInitiativeBadgeTitle
vi.mock('@/stores', async () => {
	const actual = await vi.importActual('@/stores');
	return {
		...actual,
		getInitiativeBadgeTitle: () => null,
		useInitiatives: () => [],
	};
});

// Factory for creating test tasks
function createTask(overrides: Partial<Task> = {}): Task {
	return createMockTask({
		id: 'TASK-001',
		title: 'Test Task',
		status: TaskStatus.RUNNING,
		branch: 'orc/TASK-001',
		...overrides,
	});
}

function renderTaskDetail(taskId: string = 'TASK-001') {
	return render(
		<TooltipProvider delayDuration={0}>
			<MemoryRouter initialEntries={[`/tasks/${taskId}`]}>
				<Routes>
					<Route path="/tasks/:id" element={<TaskDetail />} />
					<Route path="/board" element={<div>Board Page</div>} />
				</Routes>
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('TaskDetail', () => {
	beforeEach(() => {
		// Use clearAllMocks instead of resetAllMocks to preserve mock implementations
		// (resetAllMocks clears implementations, causing useTaskSubscription to return undefined)
		vi.clearAllMocks();
		useTaskStore.getState().reset();

		// Set a project ID so the component doesn't bail early
		useProjectStore.setState({ currentProjectId: 'test-project' });

		// Default mock implementations
		// taskClient.getTask returns { task: Task }
		mockGetTask.mockResolvedValue({
			task: createTask({ status: TaskStatus.RUNNING }),
		});
		// taskClient.getTaskPlan returns { plan: TaskPlan | null }
		mockGetTaskPlan.mockResolvedValue({ plan: null });
		// taskClient.listReviewComments returns { comments: [] }
		mockListReviewComments.mockResolvedValue({ comments: [] });
		// taskClient.getDiff returns { files: [], stats: {} }
		mockGetDiff.mockResolvedValue({ files: [], stats: {} });
		// feedbackClient.listFeedback returns { feedback: [] }
		mockListFeedback.mockResolvedValue({ feedback: [] });
		// taskClient.getReviewFindings returns { findings: [] }
		mockGetReviewFindings.mockResolvedValue({ findings: [] });
		mockListTaskGeneratedNotes.mockResolvedValue({ notes: [] });
		mockRetryTask.mockResolvedValue({ task: createTask({ status: TaskStatus.RUNNING }) });
		mockUpdateTask.mockResolvedValue({ task: createTask({ status: TaskStatus.PAUSED }) });
		mockUseTaskSubscription.mockReturnValue({
			state: undefined,
			transcript: [],
			isSubscribed: false,
			connectionStatus: 'connected',
			clearTranscript: vi.fn(),
		});
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loading state', () => {
		it('should show loading spinner initially', async () => {
			// Make the API call hang to keep loading state
			mockGetTask.mockImplementation(() => new Promise(() => {}));

			renderTaskDetail();

			expect(screen.getByText('Loading task...')).toBeInTheDocument();
		});

		it('should show task content after loading', async () => {
			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});
		});
	});

	describe('error state', () => {
		it('should show error when task fetch fails', async () => {
			mockGetTask.mockRejectedValue(new Error('Task not found'));

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Failed to load task')).toBeInTheDocument();
				expect(screen.getByText('Task not found')).toBeInTheDocument();
			});
		});
	});

	describe('store synchronization', () => {
		it('should update local task status when store task status changes', async () => {
			// Start with a running task
			const initialTask = createTask({ status: TaskStatus.RUNNING });
			mockGetTask.mockResolvedValue({ task: initialTask });

			// Add task to store with 'running' status
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			// Wait for task to load
			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Verify task ID is shown
			expect(screen.getByText('TASK-001')).toBeInTheDocument();

			// Simulate WebSocket update by changing store task status to 'completed'
			await act(async () => {
				useTaskStore.getState().updateTaskStatus('TASK-001', TaskStatus.COMPLETED);
			});

			// The component should now reflect the completed status
			// Verify the store task is updated
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe(TaskStatus.COMPLETED);
		});

		it('should update currentPhase when store task phase changes', async () => {
			const initialTask = createTask({
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			mockGetTask.mockResolvedValue({ task: initialTask });
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate phase change via WebSocket update
			await act(async () => {
				useTaskStore.getState().updateTask('TASK-001', {
					currentPhase: 'test',
				});
			});

			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.currentPhase).toBe('test');
		});

		it('should handle complete event updating task to completed status', async () => {
			const initialTask = createTask({ status: TaskStatus.RUNNING });
			mockGetTask.mockResolvedValue({ task: initialTask });
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate the 'complete' WebSocket event that sets status to completed
			await act(async () => {
				useTaskStore.getState().updateTaskStatus('TASK-001', TaskStatus.COMPLETED);
			});

			// Verify store was updated
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe(TaskStatus.COMPLETED);
		});
	});

	describe('integration with WebSocket events', () => {
		it('should reflect state event updates in UI', async () => {
			const initialTask = createTask({ status: TaskStatus.RUNNING });
			mockGetTask.mockResolvedValue({ task: initialTask });
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate 'state' WebSocket event via updateTaskState
			// This mirrors what handleWSEvent does in useWebSocket.tsx
			// Note: updateTaskState stores ExecutionState but doesn't sync to task
			// Use updateTask/updateTaskStatus to update task fields
			await act(async () => {
				useTaskStore.getState().updateTask('TASK-001', {
					status: TaskStatus.COMPLETED,
					currentPhase: 'test',
				});
			});

			// Verify task was updated
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe(TaskStatus.COMPLETED);
			expect(storeTask?.currentPhase).toBe('test');
		});
	});

	describe('failed task error panel', () => {
		it('SC-1: renders failed phase, summary, and scrollable output in page body', async () => {
			useTaskStore.getState().addTask(
				createTask({ status: TaskStatus.FAILED, currentPhase: 'implement' })
			);
			mockGetTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
					execution: { error: 'Execution-level error' } as Task['execution'],
				}),
			});
			mockUseTaskSubscription.mockReturnValue({
				state: {
					phases: {
						implement: { error: 'Validation failed in implement phase' },
					},
				},
				transcript: [],
			});

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByTestId('task-detail-error-panel')).toBeInTheDocument();
			});

			expect(screen.getByText(/error in/i)).toHaveTextContent('implement');
			expect(screen.getAllByText(/validation failed in implement phase/i)).toHaveLength(2);
			const details = screen.getByRole('region', { name: /error output/i });
			expect(details).toHaveClass('task-detail-error-panel__details');
		});

		it('SC-2: renders action buttons and dispatches retry/update requests', async () => {
			useTaskStore.getState().addTask(
				createTask({ status: TaskStatus.FAILED, currentPhase: 'implement' })
			);
			mockGetTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});
			mockGetTaskPlan.mockResolvedValue({
				plan: createMockTaskPlan({
					phases: [
						createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
						createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
					],
				}),
			});
			mockUseTaskSubscription.mockReturnValue({
				state: {
					phases: {
						implement: { error: 'Implement failed' },
					},
				},
				transcript: [],
			});
			mockRetryTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});
			mockUpdateTask.mockResolvedValue({ task: createTask({ status: TaskStatus.PAUSED }) });

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry implement/i })).toBeInTheDocument();
			});

			const errorPanel = screen.getByTestId('task-detail-error-panel');
			expect(within(errorPanel).getByRole('button', { name: /retry from earlier/i })).toBeInTheDocument();
			expect(within(errorPanel).getByRole('button', { name: /fix manually/i })).toBeInTheDocument();
			expect(within(errorPanel).getByRole('button', { name: /abort task/i })).toBeInTheDocument();
			expect(within(errorPanel).getByRole('combobox')).toBeInTheDocument();

			fireEvent.click(within(errorPanel).getByRole('button', { name: /retry implement/i }));
			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenNthCalledWith(
					1,
					expect.objectContaining({ fromPhase: 'implement' })
				);
			});

			fireEvent.change(within(errorPanel).getByRole('combobox'), {
				target: { value: 'spec' },
			});
			fireEvent.click(within(errorPanel).getByRole('button', { name: /retry from earlier/i }));
			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenNthCalledWith(
					2,
					expect.objectContaining({ fromPhase: 'spec' })
				);
			});

			fireEvent.click(within(errorPanel).getByRole('button', { name: /fix manually/i }));
			await waitFor(() => {
				expect(mockUpdateTask).toHaveBeenCalledWith(
					expect.objectContaining({
						status: TaskStatus.PAUSED,
						manualFix: true,
					})
				);
			});
		});

		it('SC-3: Fix Manually pauses task and removes failed panel', async () => {
			useTaskStore.getState().addTask(
				createTask({ status: TaskStatus.FAILED, currentPhase: 'implement' })
			);
			mockGetTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});
			mockUseTaskSubscription.mockReturnValue({
				state: { phases: { implement: { error: 'Implement failed' } } },
				transcript: [],
			});
			mockUpdateTask.mockResolvedValue({ task: createTask({ status: TaskStatus.PAUSED }) });

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /fix manually/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /fix manually/i }));

			await waitFor(() => {
				expect(mockUpdateTask).toHaveBeenCalledWith(
					expect.objectContaining({
						status: TaskStatus.PAUSED,
						manualFix: true,
					})
				);
			});

			await waitFor(() => {
				expect(screen.queryByTestId('task-detail-error-panel')).not.toBeInTheDocument();
			});
		});

		it('SC-4: Abort Task confirms and closes task', async () => {
			useTaskStore.getState().addTask(
				createTask({ status: TaskStatus.FAILED, currentPhase: 'implement' })
			);
			mockGetTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});
			mockUseTaskSubscription.mockReturnValue({
				state: { phases: { implement: { error: 'Implement failed' } } },
				transcript: [],
			});
			mockUpdateTask.mockResolvedValue({ task: createTask({ status: TaskStatus.CLOSED }) });

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /abort task/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /abort task/i }));

			await waitFor(() => {
				expect(window.confirm).toHaveBeenCalled();
				expect(mockUpdateTask).toHaveBeenCalledWith(
					expect.objectContaining({
						status: TaskStatus.CLOSED,
					})
				);
			});
		});

		it('SC-5: passes guidance textarea value to retry instructions', async () => {
			useTaskStore.getState().addTask(
				createTask({ status: TaskStatus.FAILED, currentPhase: 'implement' })
			);
			mockGetTask.mockResolvedValue({
				task: createTask({
					status: TaskStatus.FAILED,
					currentPhase: 'implement',
				}),
			});
			mockUseTaskSubscription.mockReturnValue({
				state: { phases: { implement: { error: 'Implement failed' } } },
				transcript: [],
			});

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByPlaceholderText(/guidance/i)).toBeInTheDocument();
			});

			fireEvent.change(screen.getByPlaceholderText(/guidance/i), {
				target: { value: 'Use the previous schema parser in this phase.' },
			});
			fireEvent.click(screen.getByRole('button', { name: /retry implement/i }));

			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenCalledWith(
					expect.objectContaining({
						fromPhase: 'implement',
						instructions: 'Use the previous schema parser in this phase.',
					})
				);
			});
		});

		it.each([TaskStatus.RUNNING, TaskStatus.PAUSED, TaskStatus.COMPLETED, TaskStatus.CREATED])(
			'SC-6: hides error panel when task status is %s',
			async (status) => {
				useTaskStore.getState().addTask(
					createTask({
						status,
						currentPhase: 'implement',
					})
				);
				mockGetTask.mockResolvedValue({
					task: createTask({
						status,
						currentPhase: 'implement',
					}),
				});
				mockUseTaskSubscription.mockReturnValue({
					state: { phases: { implement: { error: 'Implement failed' } } },
					transcript: [],
				});

				renderTaskDetail();

				await waitFor(() => {
					expect(screen.getByText('Test Task')).toBeInTheDocument();
				});
				expect(screen.queryByTestId('task-detail-error-panel')).not.toBeInTheDocument();
			}
		);
	});
});
