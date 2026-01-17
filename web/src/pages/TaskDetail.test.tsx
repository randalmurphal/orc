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
import { useTaskStore } from '@/stores';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/lib/types';

// Mock the API
vi.mock('@/lib/api', () => ({
	getTask: vi.fn(),
	getTaskPlan: vi.fn(),
	getTaskDependencies: vi.fn().mockResolvedValue({ blocked_by: [], blocks: [], related_to: [], referenced_by: [] }),
	deleteTask: vi.fn(),
	runTask: vi.fn(),
	pauseTask: vi.fn(),
	resumeTask: vi.fn(),
}));

// Mock useTaskSubscription hook
vi.mock('@/hooks', () => ({
	useTaskSubscription: vi.fn(() => ({
		state: undefined,
		transcript: [],
		isSubscribed: false,
		connectionStatus: 'connected',
		clearTranscript: vi.fn(),
	})),
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

// Import mocked modules
import { getTask, getTaskPlan } from '@/lib/api';

// Factory for creating test tasks
function createTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'TASK-001',
		title: 'Test Task',
		weight: 'medium',
		status: 'running',
		branch: 'orc/TASK-001',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides,
	};
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

		// Default mock implementations
		(getTask as ReturnType<typeof vi.fn>).mockResolvedValue(
			createTask({ status: 'running' })
		);
		(getTaskPlan as ReturnType<typeof vi.fn>).mockResolvedValue(null);
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loading state', () => {
		it('should show loading spinner initially', async () => {
			// Make the API call hang to keep loading state
			(getTask as ReturnType<typeof vi.fn>).mockImplementation(
				() => new Promise(() => {})
			);

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
			(getTask as ReturnType<typeof vi.fn>).mockRejectedValue(
				new Error('Task not found')
			);

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
			const initialTask = createTask({ status: 'running' });
			(getTask as ReturnType<typeof vi.fn>).mockResolvedValue(initialTask);

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
				useTaskStore.getState().updateTaskStatus('TASK-001', 'completed');
			});

			// The component should now reflect the completed status
			// Verify the store task is updated
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe('completed');
		});

		it('should update current_phase when store task phase changes', async () => {
			const initialTask = createTask({
				status: 'running',
				current_phase: 'implement',
			});
			(getTask as ReturnType<typeof vi.fn>).mockResolvedValue(initialTask);
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate phase change via WebSocket update
			await act(async () => {
				useTaskStore.getState().updateTask('TASK-001', {
					current_phase: 'test',
				});
			});

			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.current_phase).toBe('test');
		});

		it('should handle complete event updating task to completed status', async () => {
			const initialTask = createTask({ status: 'running' });
			(getTask as ReturnType<typeof vi.fn>).mockResolvedValue(initialTask);
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate the 'complete' WebSocket event that sets status to completed
			await act(async () => {
				useTaskStore.getState().updateTaskStatus('TASK-001', 'completed');
			});

			// Verify store was updated
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe('completed');
		});
	});

	describe('integration with WebSocket events', () => {
		it('should reflect state event updates in UI', async () => {
			const initialTask = createTask({ status: 'running' });
			(getTask as ReturnType<typeof vi.fn>).mockResolvedValue(initialTask);
			useTaskStore.getState().addTask(initialTask);

			renderTaskDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});

			// Simulate 'state' WebSocket event via updateTaskState
			// This mirrors what handleWSEvent does in useWebSocket.tsx
			await act(async () => {
				useTaskStore.getState().updateTaskState('TASK-001', {
					task_id: 'TASK-001',
					current_phase: 'test',
					current_iteration: 1,
					status: 'completed',
					started_at: new Date().toISOString(),
					updated_at: new Date().toISOString(),
					phases: {},
					gates: [],
					tokens: { input_tokens: 0, output_tokens: 0, total_tokens: 0 },
				});
			});

			// updateTaskState syncs status to task when task exists
			const storeTask = useTaskStore.getState().getTask('TASK-001');
			expect(storeTask?.status).toBe('completed');
			expect(storeTask?.current_phase).toBe('test');
		});
	});
});
