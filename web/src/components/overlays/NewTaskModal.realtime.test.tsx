/**
 * TDD Integration Tests for NewTaskModal - Real-time Board Updates
 *
 * Tests for TASK-555: Board doesn't update in real-time after creating task from modal
 *
 * These tests verify the integration between NewTaskModal, API, events, and the board.
 *
 * Success Criteria Coverage:
 * - SC-4: The NewTaskModal should call onCreate callback with the created task
 * - SC-5: The onCreate callback should add the task to the store for immediate UI update
 * - SC-6: The board should display the newly created task in the Queue column
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NewTaskModal } from './NewTaskModal';
import { TaskStatus, TaskWeight } from '@/gen/orc/v1/task_pb';
import { useTaskStore } from '@/stores/taskStore';
import {
	createMockTask,
	createMockCreateTaskResponse,
	createMockWorkflow,
	createMockListWorkflowsResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	taskClient: {
		createTask: vi.fn(),
	},
	workflowClient: {
		listWorkflows: vi.fn(),
	},
}));

// Mock the stores
vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'test-project',
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

// Import mocked modules for assertions
import { taskClient, workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';

// Mock workflows
const mockWorkflows = [
	createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true }),
	createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
	createMockWorkflow({ id: 'large', name: 'Large', isBuiltin: true }),
];

// Mock browser APIs for Radix
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	Element.prototype.hasPointerCapture = vi.fn().mockReturnValue(false);
	Element.prototype.setPointerCapture = vi.fn();
	Element.prototype.releasePointerCapture = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

describe('NewTaskModal - Real-time Board Updates', () => {
	const mockOnClose = vi.fn();
	const mockOnCreate = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		useTaskStore.getState().reset();

		// Setup workflow list mock
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse(mockWorkflows)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-4: onCreate callback should be called with created task', () => {
		it('should call onCreate callback with the created task after successful creation', async () => {
			const user = userEvent.setup();

			// Create a mock task that will be returned by the API
			const createdTask = createMockTask({
				id: 'TASK-NEW-001',
				title: 'My New Task',
				status: TaskStatus.CREATED,
				weight: TaskWeight.MEDIUM,
			});

			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(createdTask)
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			// Wait for workflows to load
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill in title
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'My New Task');

			// Submit the form
			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: onCreate should be called with the created task
			await waitFor(() => {
				expect(mockOnCreate).toHaveBeenCalledTimes(1);
				expect(mockOnCreate).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'TASK-NEW-001',
						title: 'My New Task',
					})
				);
			});
		});

		it('should NOT call onCreate when API returns error', async () => {
			const user = userEvent.setup();

			vi.mocked(taskClient.createTask).mockRejectedValue(
				new Error('Server error')
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill in title and submit
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Will Fail');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: onCreate should NOT be called
			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith('Server error');
			});
			expect(mockOnCreate).not.toHaveBeenCalled();
		});

		it('should NOT call onCreate when API returns empty response', async () => {
			const user = userEvent.setup();

			// API returns response without task
			vi.mocked(taskClient.createTask).mockResolvedValue({
				$typeName: 'orc.v1.CreateTaskResponse',
				task: undefined,
			});

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Empty Response');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: onCreate should NOT be called when task is undefined
			await waitFor(() => {
				expect(mockOnCreate).not.toHaveBeenCalled();
			});
		});
	});

	describe('SC-5: onCreate callback should add task to store', () => {
		it('should add task to store via onCreate callback for immediate UI update', async () => {
			const user = userEvent.setup();

			const createdTask = createMockTask({
				id: 'TASK-STORE-001',
				title: 'Store Test Task',
				status: TaskStatus.CREATED,
			});

			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(createdTask)
			);

			// Custom onCreate that adds to store (what the parent component should do)
			const onCreateWithStoreUpdate = vi.fn((task) => {
				useTaskStore.getState().addTask(task);
			});

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill and submit
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Store Test Task');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: Task should be in the store
			await waitFor(() => {
				const tasks = useTaskStore.getState().tasks;
				expect(tasks).toHaveLength(1);
				expect(tasks[0].id).toBe('TASK-STORE-001');
			});
		});

		it('should not add duplicate if task already exists (from WebSocket event)', async () => {
			const user = userEvent.setup();

			const createdTask = createMockTask({
				id: 'TASK-DUP-001',
				title: 'Duplicate Prevention Test',
				status: TaskStatus.CREATED,
			});

			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(createdTask)
			);

			// Simulate WebSocket event arriving first (adds task to store)
			useTaskStore.getState().addTask(createdTask);
			expect(useTaskStore.getState().tasks).toHaveLength(1);

			// Custom onCreate that tries to add to store
			const onCreateWithStoreUpdate = vi.fn((task) => {
				useTaskStore.getState().addTask(task);
			});

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Duplicate Prevention Test');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: Should still have only one task (no duplicates)
			await waitFor(() => {
				expect(useTaskStore.getState().tasks).toHaveLength(1);
			});
		});
	});

	describe('SC-6: Board should display newly created task', () => {
		it('should create task with PLANNED status so it appears in Queue', async () => {
			const user = userEvent.setup();

			// Task created with PLANNED status (what the API should return)
			const createdTask = createMockTask({
				id: 'TASK-QUEUE-001',
				title: 'Queue Task',
				status: TaskStatus.PLANNED, // This status makes it appear in Queue column
			});

			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(createdTask)
			);

			const onCreateWithStoreUpdate = vi.fn((task) => {
				useTaskStore.getState().addTask(task);
			});

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Queue Task');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: Task in store should have PLANNED status
			await waitFor(() => {
				const task = useTaskStore.getState().getTask('TASK-QUEUE-001');
				expect(task).toBeDefined();
				expect(task?.status).toBe(TaskStatus.PLANNED);
			});
		});

		it('should close modal after successful creation', async () => {
			const user = userEvent.setup();

			const createdTask = createMockTask({
				id: 'TASK-CLOSE-001',
				title: 'Close Test',
			});

			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(createdTask)
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Close Test');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Assert: onClose should be called after successful creation
			await waitFor(() => {
				expect(mockOnClose).toHaveBeenCalled();
			});
		});
	});

	describe('Edge cases for real-time updates', () => {
		it('should handle multiple task creations accumulating in store', async () => {
			const user = userEvent.setup();
			let taskCounter = 0;

			vi.mocked(taskClient.createTask).mockImplementation(async () => {
				taskCounter++;
				const task = createMockTask({
					id: `TASK-RAPID-${taskCounter}`,
					title: `Rapid Task ${taskCounter}`,
				});
				return createMockCreateTaskResponse(task);
			});

			const onCreateWithStoreUpdate = vi.fn((task) => {
				useTaskStore.getState().addTask(task);
			});

			const { rerender } = render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Create first task
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Rapid Task 1');

			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(useTaskStore.getState().tasks).toHaveLength(1);
			});

			// Close modal, then reopen (triggers form reset via useEffect)
			rerender(
				<NewTaskModal
					open={false}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			rerender(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={onCreateWithStoreUpdate}
				/>
			);

			// Wait for workflows to reload and form to reset
			await waitFor(() => {
				const input = screen.getByLabelText(/title/i);
				expect((input as HTMLInputElement).value).toBe(''); // Reset on reopen
			});

			const titleInput2 = screen.getByLabelText(/title/i);
			await user.type(titleInput2, 'Rapid Task 2');

			const createButton2 = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton2);

			// Assert: Both tasks should be in store
			await waitFor(() => {
				const tasks = useTaskStore.getState().tasks;
				expect(tasks).toHaveLength(2);
				expect(tasks.map(t => t.id).sort()).toEqual(['TASK-RAPID-1', 'TASK-RAPID-2']);
			});
		});
	});
});
