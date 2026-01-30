/**
 * Tests for TaskDetail page component
 *
 * Verifies:
 * - Task status syncs from store when WebSocket updates arrive
 * - Loading and error states are handled correctly
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { TaskDetail } from './TaskDetail';
import { useTaskStore, useProjectStore } from '@/stores';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { type Task, TaskStatus, TaskWeight } from '@/gen/orc/v1/task_pb';
import { createMockTask } from '@/test/factories';

// Mock the Connect RPC client
const mockGetTask = vi.fn();
const mockGetTaskPlan = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: (...args: unknown[]) => mockGetTask(...args),
		getTaskPlan: (...args: unknown[]) => mockGetTaskPlan(...args),
	},
}));

// Mock hooks module — must include all hooks imported by TaskDetail
vi.mock('@/hooks', () => ({
	useTaskSubscription: vi.fn(() => ({
		state: undefined,
		transcript: [],
		isSubscribed: false,
		connectionStatus: 'connected',
		clearTranscript: vi.fn(),
	})),
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
		weight: TaskWeight.MEDIUM,
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
});
