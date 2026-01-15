import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import DependencySidebar from './DependencySidebar.svelte';
import type { Task } from '$lib/types';

// Mock the API module
vi.mock('$lib/api', () => ({
	getTaskDependencies: vi.fn(),
	addBlocker: vi.fn(),
	removeBlocker: vi.fn(),
	addRelated: vi.fn(),
	removeRelated: vi.fn(),
	listTasks: vi.fn()
}));

// Mock goto
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

import { getTaskDependencies, addBlocker, removeBlocker, addRelated, removeRelated } from '$lib/api';
import { goto } from '$app/navigation';

describe('DependencySidebar', () => {
	const mockTask: Task = {
		id: 'TASK-001',
		title: 'Test Task',
		weight: 'medium',
		status: 'planned',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	};

	const mockDependencies = {
		task_id: 'TASK-001',
		blocked_by: [
			{ id: 'TASK-002', title: 'Blocking Task', status: 'completed', is_met: true },
			{ id: 'TASK-003', title: 'Another Blocker', status: 'running', is_met: false }
		],
		blocks: [{ id: 'TASK-004', title: 'Blocked Task', status: 'planned' }],
		related_to: [{ id: 'TASK-005', title: 'Related Task', status: 'completed' }],
		referenced_by: [{ id: 'TASK-006', title: 'Referencing Task', status: 'running' }],
		unmet_dependencies: ['TASK-003'],
		can_run: false
	};

	const mockOnTaskUpdated = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(getTaskDependencies).mockResolvedValue(mockDependencies);
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders with Dependencies header', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			expect(screen.getByText('Dependencies')).toBeInTheDocument();
		});

		it('loads dependencies on mount', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(getTaskDependencies).toHaveBeenCalledWith('TASK-001');
			});
		});

		it('displays blocked banner when task has unmet dependencies', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText(/Blocked by 1 incomplete task/)).toBeInTheDocument();
			});
		});

		it('displays blocking info when task blocks others', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText(/Blocking 1 task/)).toBeInTheDocument();
			});
		});

		it('displays blocked by section with tasks', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Blocked by')).toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.getByText('Blocking Task')).toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
			});
		});

		it('displays blocks section with tasks', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Blocks')).toBeInTheDocument();
				expect(screen.getByText('TASK-004')).toBeInTheDocument();
			});
		});

		it('displays related section', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Related')).toBeInTheDocument();
				expect(screen.getByText('TASK-005')).toBeInTheDocument();
			});
		});

		it('displays referenced in section', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Referenced in')).toBeInTheDocument();
				expect(screen.getByText('TASK-006')).toBeInTheDocument();
			});
		});
	});

	describe('status icons', () => {
		it('shows green checkmark for completed tasks', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				// TASK-002 is completed, should have status-completed class
				const completedIcons = document.querySelectorAll('.status-completed');
				expect(completedIcons.length).toBeGreaterThan(0);
			});
		});

		it('shows blue dot for running tasks', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				// TASK-003 is running, should have status-running class
				const runningIcons = document.querySelectorAll('.status-running');
				expect(runningIcons.length).toBeGreaterThan(0);
			});
		});
	});

	describe('empty states', () => {
		it('shows empty message when no blockers', async () => {
			vi.mocked(getTaskDependencies).mockResolvedValue({
				task_id: 'TASK-001',
				blocked_by: [],
				blocks: [],
				related_to: [],
				referenced_by: [],
				can_run: true
			});

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('No blocking dependencies')).toBeInTheDocument();
			});
		});

		it('shows empty message when no related tasks', async () => {
			vi.mocked(getTaskDependencies).mockResolvedValue({
				task_id: 'TASK-001',
				blocked_by: [],
				blocks: [],
				related_to: [],
				referenced_by: [],
				can_run: true
			});

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('No related tasks')).toBeInTheDocument();
			});
		});

		it('does not show blocks section when empty', async () => {
			vi.mocked(getTaskDependencies).mockResolvedValue({
				task_id: 'TASK-001',
				blocked_by: [],
				blocks: [],
				related_to: [],
				referenced_by: [],
				can_run: true
			});

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.queryByText('Blocks')).not.toBeInTheDocument();
			});
		});
	});

	describe('navigation', () => {
		it('navigates to task when dependency is clicked', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			// Find the button containing TASK-002 and click it
			const taskLink = screen.getByText('TASK-002').closest('button');
			if (taskLink) {
				await fireEvent.click(taskLink);
			}

			expect(goto).toHaveBeenCalledWith('/tasks/TASK-002');
		});
	});

	describe('removing dependencies', () => {
		it('removes blocker when remove button is clicked', async () => {
			const updatedTask = { ...mockTask, blocked_by: ['TASK-003'] };
			vi.mocked(removeBlocker).mockResolvedValue(updatedTask);

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			// Find and click the remove button for TASK-002
			const removeButtons = screen.getAllByTitle('Remove blocker');
			await fireEvent.click(removeButtons[0]);

			await waitFor(() => {
				expect(removeBlocker).toHaveBeenCalledWith('TASK-001', 'TASK-002');
				expect(mockOnTaskUpdated).toHaveBeenCalledWith(updatedTask);
			});
		});

		it('removes related task when remove button is clicked', async () => {
			const updatedTask = { ...mockTask, related_to: [] };
			vi.mocked(removeRelated).mockResolvedValue(updatedTask);

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-005')).toBeInTheDocument();
			});

			// Find and click the remove button for related task
			const removeButton = screen.getByTitle('Remove relation');
			await fireEvent.click(removeButton);

			await waitFor(() => {
				expect(removeRelated).toHaveBeenCalledWith('TASK-001', 'TASK-005');
				expect(mockOnTaskUpdated).toHaveBeenCalledWith(updatedTask);
			});
		});
	});

	describe('collapse/expand', () => {
		it('can collapse sidebar', async () => {
			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
			});

			// Click header to collapse
			const header = screen.getByText('Dependencies').closest('button');
			if (header) {
				await fireEvent.click(header);
			}

			// Content should be hidden
			expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
		});
	});

	describe('error handling', () => {
		it('shows error message when loading fails', async () => {
			vi.mocked(getTaskDependencies).mockRejectedValue(new Error('Network error'));

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Network error')).toBeInTheDocument();
				expect(screen.getByText('Retry')).toBeInTheDocument();
			});
		});

		it('can retry after error', async () => {
			vi.mocked(getTaskDependencies)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValue(mockDependencies);

			render(DependencySidebar, {
				props: {
					task: mockTask,
					onTaskUpdated: mockOnTaskUpdated
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
});
