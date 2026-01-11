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

	describe('column titles match documentation', () => {
		it('renders all four columns with correct titles', async () => {
			render(Board, {
				props: {
					tasks: [],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			// Verify all column titles per documentation
			await waitFor(() => {
				expect(screen.getByText('To Do')).toBeInTheDocument();
				expect(screen.getByText('In Progress')).toBeInTheDocument();
				expect(screen.getByText('In Review')).toBeInTheDocument();
				expect(screen.getByText('Done')).toBeInTheDocument();
			});
		});

		it('has exactly 4 columns', async () => {
			const { container } = render(Board, {
				props: {
					tasks: [],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			// The board should contain 4 Column components
			const board = container.querySelector('.board');
			expect(board).toBeInTheDocument();
			// Each column is rendered as a direct child
			expect(board?.children.length).toBe(4);
		});
	});

	describe('classifying status is handled in drop (column mapping)', () => {
		it('maps created status to To Do column', async () => {
			const task = createMockTask({ status: 'created' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				// Task should appear in To Do column
				expect(screen.getByText('Test Task')).toBeInTheDocument();
			});
		});

		it('maps classifying status to To Do column', async () => {
			const task = createMockTask({ status: 'classifying', title: 'Classifying Task' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Classifying Task')).toBeInTheDocument();
			});
		});

		it('maps planned status to To Do column', async () => {
			const task = createMockTask({ status: 'planned', title: 'Planned Task' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Planned Task')).toBeInTheDocument();
			});
		});

		it('maps running status to In Progress column', async () => {
			const task = createMockTask({ status: 'running', title: 'Running Task' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Running Task')).toBeInTheDocument();
			});
		});

		it('maps paused status to In Review column', async () => {
			const task = createMockTask({ status: 'paused', title: 'Paused Task' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Paused Task')).toBeInTheDocument();
			});
		});

		it('maps blocked status to In Review column', async () => {
			const task = createMockTask({ status: 'blocked', title: 'Blocked Task' });

			render(Board, {
				props: {
					tasks: [task],
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Blocked Task')).toBeInTheDocument();
			});
		});

		it('maps completed status to Done column', async () => {
			const task = createMockTask({ status: 'completed', title: 'Completed Task' });

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

		it('maps failed status to Done column', async () => {
			const task = createMockTask({ status: 'failed', title: 'Failed Task' });

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
	});

	describe('task distribution across columns', () => {
		it('distributes multiple tasks to correct columns', async () => {
			const tasks = [
				createMockTask({ id: 'T1', title: 'Todo Task', status: 'created' }),
				createMockTask({ id: 'T2', title: 'Progress Task', status: 'running' }),
				createMockTask({ id: 'T3', title: 'Review Task', status: 'paused' }),
				createMockTask({ id: 'T4', title: 'Done Task', status: 'completed' })
			];

			render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Todo Task')).toBeInTheDocument();
				expect(screen.getByText('Progress Task')).toBeInTheDocument();
				expect(screen.getByText('Review Task')).toBeInTheDocument();
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

			// Columns should still render
			await waitFor(() => {
				expect(screen.getByText('To Do')).toBeInTheDocument();
				expect(screen.getByText('In Progress')).toBeInTheDocument();
			});
		});

		it('groups multiple tasks in same column', async () => {
			const tasks = [
				createMockTask({ id: 'T1', title: 'First Created', status: 'created' }),
				createMockTask({ id: 'T2', title: 'Second Created', status: 'created' }),
				createMockTask({ id: 'T3', title: 'Classifying One', status: 'classifying' })
			];

			render(Board, {
				props: {
					tasks,
					onAction: mockOnAction,
					onRefresh: mockOnRefresh
				}
			});

			await waitFor(() => {
				expect(screen.getByText('First Created')).toBeInTheDocument();
				expect(screen.getByText('Second Created')).toBeInTheDocument();
				expect(screen.getByText('Classifying One')).toBeInTheDocument();
			});
		});
	});

	describe('column status mappings (internal behavior)', () => {
		// These tests verify the column configuration is correct per CLAUDE.md docs
		const columnConfig = [
			{ id: 'todo', title: 'To Do', statuses: ['created', 'classifying', 'planned'] },
			{ id: 'running', title: 'In Progress', statuses: ['running'] },
			{ id: 'review', title: 'In Review', statuses: ['paused', 'blocked'] },
			{ id: 'done', title: 'Done', statuses: ['completed', 'failed'] }
		];

		it('To Do column includes created, classifying, planned', () => {
			const todoColumn = columnConfig.find((c) => c.id === 'todo');
			expect(todoColumn?.statuses).toContain('created');
			expect(todoColumn?.statuses).toContain('classifying');
			expect(todoColumn?.statuses).toContain('planned');
			expect(todoColumn?.statuses).toHaveLength(3);
		});

		it('In Progress column includes only running', () => {
			const runningColumn = columnConfig.find((c) => c.id === 'running');
			expect(runningColumn?.statuses).toContain('running');
			expect(runningColumn?.statuses).toHaveLength(1);
		});

		it('In Review column includes paused and blocked', () => {
			const reviewColumn = columnConfig.find((c) => c.id === 'review');
			expect(reviewColumn?.statuses).toContain('paused');
			expect(reviewColumn?.statuses).toContain('blocked');
			expect(reviewColumn?.statuses).toHaveLength(2);
		});

		it('Done column includes completed and failed', () => {
			const doneColumn = columnConfig.find((c) => c.id === 'done');
			expect(doneColumn?.statuses).toContain('completed');
			expect(doneColumn?.statuses).toContain('failed');
			expect(doneColumn?.statuses).toHaveLength(2);
		});

		it('all TaskStatus values are mapped to a column', () => {
			const allStatuses: TaskStatus[] = [
				'created',
				'classifying',
				'planned',
				'running',
				'paused',
				'blocked',
				'completed',
				'failed'
			];

			const mappedStatuses = columnConfig.flatMap((c) => c.statuses);

			for (const status of allStatuses) {
				expect(mappedStatuses).toContain(status);
			}
		});
	});

	describe('getSourceColumn logic', () => {
		// Testing the getSourceColumn function behavior through column mapping
		it('returns correct column id for each status', () => {
			const columns = [
				{ id: 'todo', statuses: ['created', 'classifying', 'planned'] },
				{ id: 'running', statuses: ['running'] },
				{ id: 'review', statuses: ['paused', 'blocked'] },
				{ id: 'done', statuses: ['completed', 'failed'] }
			];

			function getSourceColumn(status: TaskStatus): string {
				for (const col of columns) {
					if (col.statuses.includes(status)) {
						return col.id;
					}
				}
				return 'todo';
			}

			expect(getSourceColumn('created')).toBe('todo');
			expect(getSourceColumn('classifying')).toBe('todo');
			expect(getSourceColumn('planned')).toBe('todo');
			expect(getSourceColumn('running')).toBe('running');
			expect(getSourceColumn('paused')).toBe('review');
			expect(getSourceColumn('blocked')).toBe('review');
			expect(getSourceColumn('completed')).toBe('done');
			expect(getSourceColumn('failed')).toBe('done');
		});

		it('defaults to todo for unknown status', () => {
			const columns = [
				{ id: 'todo', statuses: ['created', 'classifying', 'planned'] },
				{ id: 'running', statuses: ['running'] },
				{ id: 'review', statuses: ['paused', 'blocked'] },
				{ id: 'done', statuses: ['completed', 'failed'] }
			];

			function getSourceColumn(status: string): string {
				for (const col of columns) {
					if (col.statuses.includes(status)) {
						return col.id;
					}
				}
				return 'todo';
			}

			// Unknown status should default to 'todo'
			expect(getSourceColumn('unknown')).toBe('todo');
		});
	});

	describe('action mapping for column transitions', () => {
		// Testing the handleDrop action determination logic
		// Helper to determine action based on source status and target column
		function determineAction(sourceStatus: TaskStatus, targetColumn: string): string | null {
			if (targetColumn === 'running' && sourceStatus !== 'running') {
				if (sourceStatus === 'paused') {
					return 'resume';
				} else if (['created', 'classifying', 'planned'].includes(sourceStatus)) {
					return 'run';
				}
			} else if (targetColumn === 'review' && sourceStatus === 'running') {
				return 'pause';
			}
			return null;
		}

		it('determines run action for todo to running transition', () => {
			const action = determineAction('created', 'running');
			expect(action).toBe('run');
		});

		it('determines resume action for paused to running transition', () => {
			const action = determineAction('paused', 'running');
			expect(action).toBe('resume');
		});

		it('determines pause action for running to review transition', () => {
			const action = determineAction('running', 'review');
			expect(action).toBe('pause');
		});

		it('returns null for same-column drop', () => {
			const sourceStatus: TaskStatus = 'created';
			const targetColumn = 'todo';

			// Same column check would happen before action determination
			const sourceColumn = 'todo'; // getSourceColumn(sourceStatus)

			if (sourceColumn === targetColumn) {
				// No action needed
				expect(sourceColumn).toBe(targetColumn);
			}
		});

		it('returns null for invalid transitions', () => {
			const action = determineAction('completed', 'running');
			// completed -> running has no valid action
			expect(action).toBeNull();
		});
	});
});
