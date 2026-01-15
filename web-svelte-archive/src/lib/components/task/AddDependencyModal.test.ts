import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import AddDependencyModal from './AddDependencyModal.svelte';

// Mock the API module
vi.mock('$lib/api', () => ({
	listTasks: vi.fn()
}));

import { listTasks } from '$lib/api';

describe('AddDependencyModal', () => {
	const mockOnClose = vi.fn();
	const mockOnSelect = vi.fn();

	const mockTasks = [
		{ id: 'TASK-001', title: 'Current Task', status: 'planned', weight: 'medium', branch: 'test', created_at: '', updated_at: '' },
		{ id: 'TASK-002', title: 'Completed Task', status: 'completed', weight: 'small', branch: 'test', created_at: '', updated_at: '' },
		{ id: 'TASK-003', title: 'Running Task', status: 'running', weight: 'large', branch: 'test', created_at: '', updated_at: '' },
		{ id: 'TASK-004', title: 'Pending Task', status: 'planned', weight: 'medium', branch: 'test', created_at: '', updated_at: '' },
		{ id: 'TASK-005', title: 'Blocked Task', status: 'blocked', weight: 'small', branch: 'test', created_at: '', updated_at: '' }
	];

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(listTasks).mockResolvedValue(mockTasks as any);
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders modal when open is true', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Add Blocking Task')).toBeInTheDocument();
			});
		});

		it('does not render modal when open is false', () => {
			render(AddDependencyModal, {
				props: {
					open: false,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			expect(screen.queryByText('Add Blocking Task')).not.toBeInTheDocument();
		});

		it('renders correct title for related type', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'related',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Add Related Task')).toBeInTheDocument();
			});
		});

		it('loads tasks when modal opens', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(listTasks).toHaveBeenCalled();
			});
		});
	});

	describe('task filtering', () => {
		it('excludes current task from list', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});
		});

		it('excludes existing blockers when type is blocker', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: ['TASK-002'],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
			});
		});

		it('excludes existing related when type is related', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'related',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: ['TASK-003']
				}
			});

			await waitFor(() => {
				expect(screen.queryByText('TASK-003')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});
		});
	});

	describe('search functionality', () => {
		it('filters tasks by search query', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			const searchInput = screen.getByPlaceholderText('Search by ID or title...');
			await fireEvent.input(searchInput, { target: { value: 'Running' } });

			await waitFor(() => {
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
			});
		});

		it('filters tasks by task ID', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			const searchInput = screen.getByPlaceholderText('Search by ID or title...');
			await fireEvent.input(searchInput, { target: { value: '004' } });

			await waitFor(() => {
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-004')).toBeInTheDocument();
			});
		});

		it('shows empty state when no tasks match search', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			const searchInput = screen.getByPlaceholderText('Search by ID or title...');
			await fireEvent.input(searchInput, { target: { value: 'nonexistent' } });

			await waitFor(() => {
				expect(screen.getByText(/No tasks matching "nonexistent"/)).toBeInTheDocument();
			});
		});
	});

	describe('task selection', () => {
		it('calls onSelect when task is clicked', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			const taskItem = screen.getByText('TASK-002').closest('button');
			if (taskItem) {
				await fireEvent.click(taskItem);
			}

			expect(mockOnSelect).toHaveBeenCalledWith('TASK-002');
		});
	});

	describe('cancel behavior', () => {
		it('calls onClose when cancel button is clicked', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Cancel')).toBeInTheDocument();
			});

			await fireEvent.click(screen.getByText('Cancel'));

			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	describe('loading state', () => {
		it('shows loading indicator while fetching tasks', async () => {
			vi.mocked(listTasks).mockImplementation(() => new Promise(() => {})); // Never resolves

			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			expect(screen.getByText('Loading tasks...')).toBeInTheDocument();
		});
	});

	describe('error handling', () => {
		it('shows error when task loading fails', async () => {
			vi.mocked(listTasks).mockRejectedValue(new Error('Failed to load'));

			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('can retry loading tasks after error', async () => {
			vi.mocked(listTasks)
				.mockRejectedValueOnce(new Error('Failed to load'))
				.mockResolvedValue(mockTasks as any);

			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Retry')).toBeInTheDocument();
			});

			await fireEvent.click(screen.getByText('Retry'));

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});
		});
	});

	describe('status display', () => {
		it('shows correct status label for completed tasks', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Completed')).toBeInTheDocument();
			});
		});

		it('shows correct status label for running tasks', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Running')).toBeInTheDocument();
			});
		});

		it('shows correct status icon classes', async () => {
			render(AddDependencyModal, {
				props: {
					open: true,
					onClose: mockOnClose,
					onSelect: mockOnSelect,
					type: 'blocker',
					currentTaskId: 'TASK-001',
					existingBlockers: [],
					existingRelated: []
				}
			});

			await waitFor(() => {
				expect(document.querySelector('.status-completed')).toBeInTheDocument();
				expect(document.querySelector('.status-running')).toBeInTheDocument();
			});
		});
	});
});
