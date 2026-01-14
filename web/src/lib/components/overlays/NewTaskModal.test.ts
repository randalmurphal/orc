import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import NewTaskModal from './NewTaskModal.svelte';

// Mock the API module
vi.mock('$lib/api', () => ({
	createTask: vi.fn(),
	createProjectTask: vi.fn()
}));

// Mock the stores
vi.mock('$lib/stores/project', () => ({
	currentProjectId: {
		subscribe: vi.fn((cb) => {
			cb(null);
			return () => {};
		})
	}
}));

vi.mock('$lib/stores/tasks', () => ({
	addTask: vi.fn()
}));

vi.mock('$lib/stores/toast.svelte', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn()
	}
}));

// Mock goto
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

import { createTask, createProjectTask } from '$lib/api';
import { addTask } from '$lib/stores/tasks';
import { toast } from '$lib/stores/toast.svelte';
import { goto } from '$app/navigation';

describe('NewTaskModal', () => {
	const mockOnClose = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders modal when open is true', () => {
			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			expect(screen.getByText('Create New Task')).toBeInTheDocument();
			expect(screen.getByPlaceholderText('What needs to be done?')).toBeInTheDocument();
		});

		it('does not render modal when open is false', () => {
			render(NewTaskModal, {
				props: {
					open: false,
					onClose: mockOnClose
				}
			});

			expect(screen.queryByText('Create New Task')).not.toBeInTheDocument();
		});
	});

	describe('form submission', () => {
		it('creates task when form is submitted with title', async () => {
			const mockTask = { id: 'TASK-001', title: 'Test task' };
			vi.mocked(createTask).mockResolvedValue(mockTask as any);

			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const titleInput = screen.getByPlaceholderText('What needs to be done?');
			await fireEvent.input(titleInput, { target: { value: 'Test task' } });

			const submitButton = screen.getByRole('button', { name: /create task/i });
			await fireEvent.click(submitButton);

			await waitFor(() => {
				expect(createTask).toHaveBeenCalledWith('Test task', undefined, undefined, 'feature');
				expect(addTask).toHaveBeenCalledWith(mockTask);
				expect(toast.success).toHaveBeenCalled();
				expect(mockOnClose).toHaveBeenCalled();
				expect(goto).toHaveBeenCalledWith('/tasks/TASK-001');
			});
		});

		it('creates task with description when provided', async () => {
			const mockTask = { id: 'TASK-002', title: 'Test task', description: 'Test description' };
			vi.mocked(createTask).mockResolvedValue(mockTask as any);

			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const titleInput = screen.getByPlaceholderText('What needs to be done?');
			await fireEvent.input(titleInput, { target: { value: 'Test task' } });

			const descriptionInput = screen.getByPlaceholderText(/provide additional context/i);
			await fireEvent.input(descriptionInput, { target: { value: 'Test description' } });

			const submitButton = screen.getByRole('button', { name: /create task/i });
			await fireEvent.click(submitButton);

			await waitFor(() => {
				expect(createTask).toHaveBeenCalledWith('Test task', 'Test description', undefined, 'feature');
			});
		});

		it('disables submit button when title is empty', () => {
			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const submitButton = screen.getByRole('button', { name: /create task/i });
			expect(submitButton).toBeDisabled();
		});

		it('enables submit button when title is provided', async () => {
			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const titleInput = screen.getByPlaceholderText('What needs to be done?');
			await fireEvent.input(titleInput, { target: { value: 'Test task' } });

			const submitButton = screen.getByRole('button', { name: /create task/i });
			expect(submitButton).not.toBeDisabled();
		});
	});

	describe('error handling', () => {
		it('shows error message when task creation fails', async () => {
			vi.mocked(createTask).mockRejectedValue(new Error('Network error'));

			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const titleInput = screen.getByPlaceholderText('What needs to be done?');
			await fireEvent.input(titleInput, { target: { value: 'Test task' } });

			const submitButton = screen.getByRole('button', { name: /create task/i });
			await fireEvent.click(submitButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith('Network error');
				expect(mockOnClose).not.toHaveBeenCalled();
			});
		});
	});

	describe('cancel behavior', () => {
		it('calls onClose when cancel button is clicked', async () => {
			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await fireEvent.click(cancelButton);

			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	describe('keyboard shortcuts', () => {
		it('submits form on Cmd/Ctrl+Enter', async () => {
			const mockTask = { id: 'TASK-003', title: 'Keyboard task' };
			vi.mocked(createTask).mockResolvedValue(mockTask as any);

			render(NewTaskModal, {
				props: {
					open: true,
					onClose: mockOnClose
				}
			});

			const titleInput = screen.getByPlaceholderText('What needs to be done?');
			await fireEvent.input(titleInput, { target: { value: 'Keyboard task' } });

			// Test Cmd+Enter (Mac)
			await fireEvent.keyDown(titleInput, { key: 'Enter', metaKey: true });

			await waitFor(() => {
				expect(createTask).toHaveBeenCalledWith('Keyboard task', undefined, undefined, 'feature');
			});
		});
	});
});
