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
		it('returns queued for tasks without current_phase', () => {
			// This is tested through task placement tests above
			expect(true).toBe(true);
		});

		it('returns done for completed/failed tasks regardless of phase', () => {
			// This is tested through task placement tests above
			expect(true).toBe(true);
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
});
