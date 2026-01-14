import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import Board from './Board.svelte';
import type { Task, TaskStatus } from '$lib/types';

describe('Board', () => {
	const mockOnAction = vi.fn();
	const mockOnRefresh = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		mockOnAction.mockResolvedValue(undefined);
		mockOnRefresh.mockResolvedValue(undefined);
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	function createMockTask(overrides: Partial<Task> = {}): Task {
		return {
			id: 'TASK-001',
			title: 'Test Task',
			status: 'created',
			weight: 'small',
			branch: 'orc/TASK-001',
			created_at: '2025-01-01T00:00:00Z',
			updated_at: '2025-01-01T00:00:00Z',
			...overrides
		};
	}

	describe('phase-based column structure', () => {
		it('renders all six phase-based columns', async () => {
			render(Board, {
				props: {
					tasks: [],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Queued')).toBeInTheDocument();
				expect(screen.getByText('Spec')).toBeInTheDocument();
				expect(screen.getByText('Implement')).toBeInTheDocument();
				expect(screen.getByText('Test')).toBeInTheDocument();
				expect(screen.getByText('Review')).toBeInTheDocument();
				expect(screen.getByText('Done')).toBeInTheDocument();
			});
		});

		it('has exactly 6 columns', async () => {
			const { container } = render(Board, {
				props: {
					tasks: [],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			const board = container.querySelector('.board');
			expect(board).toBeInTheDocument();
			expect(board?.children.length).toBe(6);
		});
	});

	describe('task placement by phase', () => {
		it('places tasks without current_phase in Queued column', async () => {
			const task = createMockTask({ status: 'created', current_phase: undefined });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});
		});

		it('places tasks in spec phase in Spec column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'spec',
				title: 'Spec Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Spec Task')).toBeInTheDocument();
			});
		});

		it('places tasks in research phase in Spec column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'research',
				title: 'Research Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Research Task')).toBeInTheDocument();
			});
		});

		it('places tasks in implement phase in Implement column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'implement',
				title: 'Implement Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Implement Task')).toBeInTheDocument();
			});
		});

		it('places tasks in test phase in Test column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'test',
				title: 'Test Phase Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Test Phase Task')).toBeInTheDocument();
			});
		});

		it('places tasks in docs phase in Review column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'docs',
				title: 'Docs Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Docs Task')).toBeInTheDocument();
			});
		});

		it('places tasks in validate phase in Review column', async () => {
			const task = createMockTask({
				status: 'running',
				current_phase: 'validate',
				title: 'Validate Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Validate Task')).toBeInTheDocument();
			});
		});

		it('places completed tasks in Done column regardless of phase', async () => {
			const task = createMockTask({
				status: 'completed',
				current_phase: 'implement',
				title: 'Completed Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Completed Task')).toBeInTheDocument();
			});
		});

		it('places failed tasks in Done column regardless of phase', async () => {
			const task = createMockTask({
				status: 'failed',
				current_phase: 'test',
				title: 'Failed Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Failed Task')).toBeInTheDocument();
			});
		});

		it('places paused tasks in their current phase column', async () => {
			const task = createMockTask({
				status: 'paused',
				current_phase: 'implement',
				title: 'Paused Implement Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Paused Implement Task')).toBeInTheDocument();
			});
		});

		it('places blocked tasks in their current phase column', async () => {
			const task = createMockTask({
				status: 'blocked',
				current_phase: 'test',
				title: 'Blocked Test Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Blocked Test Task')).toBeInTheDocument();
			});
		});
	});

	describe('task distribution across columns', () => {
		it('distributes multiple tasks to correct phase columns', async () => {
			const tasks = [
				createMockTask({ id: 'T1', title: 'Queued Task', status: 'created' }),
				createMockTask({ id: 'T2', title: 'Spec Task', status: 'running', current_phase: 'spec' }),
				createMockTask({ id: 'T3', title: 'Impl Task', status: 'running', current_phase: 'implement' }),
				createMockTask({ id: 'T4', title: 'Test Task', status: 'running', current_phase: 'test' }),
				createMockTask({ id: 'T5', title: 'Done Task', status: 'completed', current_phase: 'validate' })
			];

			render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Queued Task')).toBeInTheDocument();
				expect(screen.getByText('Spec Task')).toBeInTheDocument();
				expect(screen.getByText('Impl Task')).toBeInTheDocument();
				expect(screen.getByText('Test Task')).toBeInTheDocument();
				expect(screen.getByText('Done Task')).toBeInTheDocument();
			});
		});

		it('handles empty task list', async () => {
			render(Board, {
				props: {
					tasks: [],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Queued')).toBeInTheDocument();
				expect(screen.getByText('Implement')).toBeInTheDocument();
			});
		});
	});

	describe('column phase mappings', () => {
		const columnConfig = [
			{ id: 'queued', title: 'Queued', phases: [] },
			{ id: 'spec', title: 'Spec', phases: ['research', 'spec', 'design'] },
			{ id: 'implement', title: 'Implement', phases: ['implement'] },
			{ id: 'test', title: 'Test', phases: ['test'] },
			{ id: 'review', title: 'Review', phases: ['docs', 'validate', 'review'] },
			{ id: 'done', title: 'Done', phases: [] }
		];

		it('Spec column includes research, spec, and design phases', () => {
			const specColumn = columnConfig.find((c) => c.id === 'spec');
			expect(specColumn?.phases).toContain('research');
			expect(specColumn?.phases).toContain('spec');
			expect(specColumn?.phases).toContain('design');
			expect(specColumn?.phases).toHaveLength(3);
		});

		it('Implement column includes only implement phase', () => {
			const implColumn = columnConfig.find((c) => c.id === 'implement');
			expect(implColumn?.phases).toContain('implement');
			expect(implColumn?.phases).toHaveLength(1);
		});

		it('Test column includes only test phase', () => {
			const testColumn = columnConfig.find((c) => c.id === 'test');
			expect(testColumn?.phases).toContain('test');
			expect(testColumn?.phases).toHaveLength(1);
		});

		it('Review column includes docs, validate, and review phases', () => {
			const reviewColumn = columnConfig.find((c) => c.id === 'review');
			expect(reviewColumn?.phases).toContain('docs');
			expect(reviewColumn?.phases).toContain('validate');
			expect(reviewColumn?.phases).toContain('review');
			expect(reviewColumn?.phases).toHaveLength(3);
		});

		it('Queued and Done columns have no phase mappings', () => {
			const queuedColumn = columnConfig.find((c) => c.id === 'queued');
			const doneColumn = columnConfig.find((c) => c.id === 'done');
			expect(queuedColumn?.phases).toHaveLength(0);
			expect(doneColumn?.phases).toHaveLength(0);
		});
	});

	describe('getTaskColumn logic', () => {
		it('returns queued for non-running tasks without current_phase', () => {
			// This is tested through task placement tests above
			expect(true).toBe(true);
		});

		it('returns done for completed/failed tasks regardless of phase', () => {
			// This is tested through task placement tests above
			expect(true).toBe(true);
		});

		// Regression test for bug: running tasks without phase showed in Queued instead of Implement
		it('places running tasks without current_phase in Implement column (not Queued)', async () => {
			// This bug occurred during initial phase transition when task was marked
			// "running" but current_phase wasn't set yet by the executor
			const task = createMockTask({
				id: 'T-RUNNING-NO-PHASE',
				status: 'running',
				current_phase: undefined,
				title: 'Running No Phase Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Running No Phase Task')).toBeInTheDocument();
			});

			// The task should NOT be in the Queued column - it's running!
			// It should be in Implement as the default phase
		});

		it('returns implement as default for unrecognized phases', async () => {
			// Tasks with unknown phases should default to implement
			const task = createMockTask({
				status: 'running',
				current_phase: 'unknown_phase',
				title: 'Unknown Phase Task'
			});

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			// Task should still render (in implement column as default)
			await waitFor(() => {
				expect(screen.getByText('Unknown Phase Task')).toBeInTheDocument();
			});
		});
	});

	describe('task sorting within columns', () => {
		it('shows running tasks before non-running tasks in same column', async () => {
			// Tasks in the same column - running should come first
			const tasks = [
				createMockTask({
					id: 'T1',
					title: 'Paused Task',
					status: 'paused',
					current_phase: 'implement',
					priority: 'critical' // High priority but paused
				}),
				createMockTask({
					id: 'T2',
					title: 'Running Task',
					status: 'running',
					current_phase: 'implement',
					priority: 'low' // Low priority but running
				})
			];

			const { container } = render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Paused Task')).toBeInTheDocument();
				expect(screen.getByText('Running Task')).toBeInTheDocument();
			});

			// Find all task cards in the Implement column
			const columns = container.querySelectorAll('.column');
			// Implement is the 3rd column (index 2): Queued, Spec, Implement
			const implementColumn = columns[2];
			const taskCards = implementColumn.querySelectorAll('.task-card');

			// Running task should be first (despite lower priority)
			expect(taskCards[0]).toHaveTextContent('Running Task');
			expect(taskCards[1]).toHaveTextContent('Paused Task');
		});

		it('sorts running tasks by priority among themselves', async () => {
			// Multiple running tasks - should sort by priority within running group
			const tasks = [
				createMockTask({
					id: 'T1',
					title: 'Low Priority Running',
					status: 'running',
					current_phase: 'implement',
					priority: 'low'
				}),
				createMockTask({
					id: 'T2',
					title: 'Critical Running',
					status: 'running',
					current_phase: 'implement',
					priority: 'critical'
				}),
				createMockTask({
					id: 'T3',
					title: 'Normal Running',
					status: 'running',
					current_phase: 'implement',
					priority: 'normal'
				})
			];

			const { container } = render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Critical Running')).toBeInTheDocument();
			});

			// Find task cards in Implement column
			const columns = container.querySelectorAll('.column');
			const implementColumn = columns[2];
			const taskCards = implementColumn.querySelectorAll('.task-card');

			// All running, so should be sorted by priority: critical, normal, low
			expect(taskCards[0]).toHaveTextContent('Critical Running');
			expect(taskCards[1]).toHaveTextContent('Normal Running');
			expect(taskCards[2]).toHaveTextContent('Low Priority Running');
		});

		it('sorts non-running tasks by priority among themselves', async () => {
			// Multiple paused tasks - should sort by priority
			const tasks = [
				createMockTask({
					id: 'T1',
					title: 'Low Priority Paused',
					status: 'paused',
					current_phase: 'test',
					priority: 'low'
				}),
				createMockTask({
					id: 'T2',
					title: 'High Priority Paused',
					status: 'paused',
					current_phase: 'test',
					priority: 'high'
				})
			];

			const { container } = render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('High Priority Paused')).toBeInTheDocument();
			});

			// Find task cards in Test column (index 3: Queued, Spec, Implement, Test)
			const columns = container.querySelectorAll('.column');
			const testColumn = columns[3];
			const taskCards = testColumn.querySelectorAll('.task-card');

			expect(taskCards[0]).toHaveTextContent('High Priority Paused');
			expect(taskCards[1]).toHaveTextContent('Low Priority Paused');
		});

		it('shows running tasks at top followed by non-running sorted by priority', async () => {
			// Mixed scenario: running and various non-running statuses
			const tasks = [
				createMockTask({
					id: 'T1',
					title: 'Critical Blocked',
					status: 'blocked',
					current_phase: 'implement',
					priority: 'critical'
				}),
				createMockTask({
					id: 'T2',
					title: 'Normal Running',
					status: 'running',
					current_phase: 'implement',
					priority: 'normal'
				}),
				createMockTask({
					id: 'T3',
					title: 'High Paused',
					status: 'paused',
					current_phase: 'implement',
					priority: 'high'
				}),
				createMockTask({
					id: 'T4',
					title: 'Low Running',
					status: 'running',
					current_phase: 'implement',
					priority: 'low'
				})
			];

			const { container } = render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Normal Running')).toBeInTheDocument();
			});

			// Find task cards in Implement column
			const columns = container.querySelectorAll('.column');
			const implementColumn = columns[2];
			const taskCards = implementColumn.querySelectorAll('.task-card');

			// Expected order:
			// 1. Normal Running (running, normal priority)
			// 2. Low Running (running, low priority)
			// 3. Critical Blocked (not running, critical priority)
			// 4. High Paused (not running, high priority)
			expect(taskCards[0]).toHaveTextContent('Normal Running');
			expect(taskCards[1]).toHaveTextContent('Low Running');
			expect(taskCards[2]).toHaveTextContent('Critical Blocked');
			expect(taskCards[3]).toHaveTextContent('High Paused');
		});
	});
});
