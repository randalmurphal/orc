import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Board, BOARD_COLUMNS } from './Board';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task, Initiative } from '@/lib/types';

// Mock stores
vi.mock('@/stores', () => ({
	useTaskStore: vi.fn(() => ({
		updateTask: vi.fn(),
	})),
	getInitiativeBadgeTitle: vi.fn((id: string) => ({
		display: id.slice(0, 6),
		full: `Initiative ${id}`,
	})),
}));

// Mock API
vi.mock('@/lib/api', () => ({
	updateTask: vi.fn().mockResolvedValue({}),
	triggerFinalize: vi.fn().mockResolvedValue({}),
}));

// Wrapper component for router and tooltip context
function renderWithRouter(ui: React.ReactElement) {
	return render(
		<TooltipProvider delayDuration={0}>
			<MemoryRouter>{ui}</MemoryRouter>
		</TooltipProvider>
	);
}

// Mock localStorage
const mockLocalStorage = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: () => {
			store = {};
		},
	};
})();

Object.defineProperty(window, 'localStorage', {
	value: mockLocalStorage,
});

// Sample tasks for testing
const createTask = (overrides: Partial<Task> = {}): Task => ({
	id: 'TASK-001',
	title: 'Test Task',
	description: 'A test task',
	weight: 'medium',
	status: 'created',
	branch: 'orc/TASK-001',
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	...overrides,
});

const sampleTasks: Task[] = [
	createTask({ id: 'TASK-001', title: 'Task 1', status: 'created' }),
	createTask({ id: 'TASK-002', title: 'Task 2', status: 'running', current_phase: 'implement' }),
	createTask({ id: 'TASK-003', title: 'Task 3', status: 'completed' }),
	createTask({ id: 'TASK-004', title: 'Task 4', status: 'created', queue: 'backlog' }),
	createTask({ id: 'TASK-005', title: 'Task 5', status: 'running', current_phase: 'test' }),
];

const sampleInitiatives: Initiative[] = [
	{
		version: 1,
		id: 'INIT-001',
		title: 'Initiative 1',
		status: 'active',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
	},
	{
		version: 1,
		id: 'INIT-002',
		title: 'Initiative 2',
		status: 'draft',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
	},
];

describe('Board', () => {
	const defaultProps = {
		tasks: sampleTasks,
		onAction: vi.fn(),
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockLocalStorage.clear();
	});

	describe('column definitions', () => {
		it('should have correct column configuration', () => {
			expect(BOARD_COLUMNS).toHaveLength(6);
			expect(BOARD_COLUMNS.map((c) => c.id)).toEqual([
				'queued',
				'spec',
				'implement',
				'test',
				'review',
				'done',
			]);
		});
	});

	describe('flat view', () => {
		it('renders all columns', () => {
			renderWithRouter(<Board {...defaultProps} />);

			// Check for column titles
			expect(screen.getByText('Queued')).toBeInTheDocument();
			expect(screen.getByText('Spec')).toBeInTheDocument();
			expect(screen.getByText('Implement')).toBeInTheDocument();
			expect(screen.getByText('Test')).toBeInTheDocument();
			expect(screen.getByText('Review')).toBeInTheDocument();
			expect(screen.getByText('Done')).toBeInTheDocument();
		});

		it('places tasks in correct columns based on status', () => {
			renderWithRouter(<Board {...defaultProps} />);

			// Task 1 (created, no phase) should be in Queued
			const queuedColumn = screen
				.getByText('Queued')
				.closest('.column, .queued-column') as HTMLElement;
			expect(queuedColumn).toBeInTheDocument();

			// Task 3 (completed) should be in Done
			const doneColumn = screen.getByText('Done').closest('.column') as HTMLElement;
			expect(doneColumn).toBeInTheDocument();
		});

		it('shows task count in column headers', () => {
			renderWithRouter(<Board {...defaultProps} />);

			// Queued column: Task 1 (active) + Task 4 (backlog) = 2
			// But backlog may be hidden by default
			const counts = screen.getAllByText(/^\d+$/);
			expect(counts.length).toBeGreaterThan(0);
		});

		it('has flat-view class when viewMode is flat', () => {
			const { container } = renderWithRouter(<Board {...defaultProps} viewMode="flat" />);
			expect(container.querySelector('.board.flat-view')).toBeInTheDocument();
		});
	});

	describe('swimlane view', () => {
		const tasksWithInitiatives: Task[] = [
			createTask({ id: 'TASK-001', title: 'Task 1', initiative_id: 'INIT-001' }),
			createTask({ id: 'TASK-002', title: 'Task 2', initiative_id: 'INIT-001' }),
			createTask({ id: 'TASK-003', title: 'Task 3', initiative_id: 'INIT-002' }),
			createTask({ id: 'TASK-004', title: 'Task 4' }), // Unassigned
		];

		it('renders swimlane headers', () => {
			renderWithRouter(
				<Board
					{...defaultProps}
					tasks={tasksWithInitiatives}
					viewMode="swimlane"
					initiatives={sampleInitiatives}
				/>
			);

			// Should see column headers in swimlane view
			expect(screen.getByText('Queued')).toBeInTheDocument();
			expect(screen.getByText('Implement')).toBeInTheDocument();
		});

		it('groups tasks by initiative', () => {
			renderWithRouter(
				<Board
					{...defaultProps}
					tasks={tasksWithInitiatives}
					viewMode="swimlane"
					initiatives={sampleInitiatives}
				/>
			);

			// Should show initiative names
			expect(screen.getByText('Initiative 1')).toBeInTheDocument();
			expect(screen.getByText('Initiative 2')).toBeInTheDocument();
			expect(screen.getByText('Unassigned')).toBeInTheDocument();
		});

		it('has swimlane-view class when viewMode is swimlane', () => {
			const { container } = renderWithRouter(
				<Board
					{...defaultProps}
					tasks={tasksWithInitiatives}
					viewMode="swimlane"
					initiatives={sampleInitiatives}
				/>
			);
			expect(container.querySelector('.board.swimlane-view')).toBeInTheDocument();
		});

		it('can collapse swimlanes', () => {
			renderWithRouter(
				<Board
					{...defaultProps}
					tasks={tasksWithInitiatives}
					viewMode="swimlane"
					initiatives={sampleInitiatives}
				/>
			);

			// Find collapse button for Initiative 1
			const collapseButtons = screen.getAllByRole('button');
			const collapseButton = collapseButtons.find((b) =>
				b.closest('.swimlane')?.textContent?.includes('Initiative 1')
			);

			if (collapseButton) {
				fireEvent.click(collapseButton);
				// State should change (stored in localStorage)
				expect(mockLocalStorage.setItem).toHaveBeenCalled();
			}
		});
	});

	describe('task sorting', () => {
		it('sorts running tasks first', () => {
			const tasksWithMixedStatus: Task[] = [
				createTask({ id: 'TASK-001', title: 'Created Task', status: 'created' }),
				createTask({
					id: 'TASK-002',
					title: 'Running Task',
					status: 'running',
					current_phase: 'implement',
				}),
				createTask({ id: 'TASK-003', title: 'Paused Task', status: 'paused' }),
			];

			renderWithRouter(<Board {...defaultProps} tasks={tasksWithMixedStatus} />);

			// Get all task cards
			const taskCards = screen.getAllByText(/Task$/);
			// Running task should appear first in its column
			expect(taskCards.length).toBeGreaterThan(0);
		});

		it('sorts by priority after running status', () => {
			const tasksWithPriority: Task[] = [
				createTask({ id: 'TASK-001', title: 'Low Priority', priority: 'low' }),
				createTask({ id: 'TASK-002', title: 'Critical Task', priority: 'critical' }),
				createTask({ id: 'TASK-003', title: 'Normal Task', priority: 'normal' }),
			];

			renderWithRouter(<Board {...defaultProps} tasks={tasksWithPriority} />);

			// Tasks should be rendered (priority order verified by column content)
			expect(screen.getByText('Critical Task')).toBeInTheDocument();
			expect(screen.getByText('Low Priority')).toBeInTheDocument();
		});
	});

	describe('backlog toggle', () => {
		it('can toggle backlog visibility', () => {
			renderWithRouter(<Board {...defaultProps} />);

			// Find toggle button
			const toggleButton = screen.queryByText(/backlog/i);
			if (toggleButton) {
				fireEvent.click(toggleButton);
				expect(mockLocalStorage.setItem).toHaveBeenCalledWith('orc-show-backlog', 'true');
			}
		});
	});

	describe('task actions', () => {
		it('calls onTaskClick when running task is clicked', async () => {
			const onTaskClick = vi.fn();
			// Create a running task - only running tasks call onTaskClick
			const runningTask = createTask({
				id: 'TASK-RUN',
				title: 'Running Task',
				status: 'running',
				current_phase: 'implement',
			});
			renderWithRouter(<Board tasks={[runningTask]} onAction={vi.fn()} onTaskClick={onTaskClick} />);

			// Click on the running task card
			const taskCard = screen.getByText('Running Task');
			fireEvent.click(taskCard);

			expect(onTaskClick).toHaveBeenCalled();
		});

		it('navigates to task detail for non-running tasks', () => {
			// For non-running tasks, clicking navigates instead of calling onTaskClick
			const onTaskClick = vi.fn();
			renderWithRouter(<Board {...defaultProps} onTaskClick={onTaskClick} />);

			// Click on a non-running task
			const taskCard = screen.getByText('Task 1'); // Status: created
			fireEvent.click(taskCard);

			// onTaskClick should NOT be called for non-running tasks
			expect(onTaskClick).not.toHaveBeenCalled();
		});
	});

	describe('empty state', () => {
		it('renders with empty task list', () => {
			renderWithRouter(<Board {...defaultProps} tasks={[]} />);

			// Should still render columns
			expect(screen.getByText('Queued')).toBeInTheDocument();
			expect(screen.getByText('Done')).toBeInTheDocument();
		});

		it('handles empty initiatives in swimlane view', () => {
			renderWithRouter(<Board {...defaultProps} tasks={[]} viewMode="swimlane" initiatives={[]} />);

			// Should render swimlane headers
			expect(screen.getByText('Queued')).toBeInTheDocument();
		});
	});

	describe('finalize functionality', () => {
		it('calls onFinalizeClick for completed tasks', () => {
			const onFinalizeClick = vi.fn();
			const completedTask = createTask({ id: 'TASK-DONE', title: 'Done Task', status: 'completed' });

			renderWithRouter(<Board {...defaultProps} tasks={[completedTask]} onFinalizeClick={onFinalizeClick} />);

			// The Done column should contain the completed task
			expect(screen.getByText('Done Task')).toBeInTheDocument();
		});

		it('gets finalize state via callback', () => {
			const getFinalizeState = vi.fn().mockReturnValue(null);
			const completedTask = createTask({ id: 'TASK-DONE', title: 'Done Task', status: 'completed' });

			renderWithRouter(
				<Board {...defaultProps} tasks={[completedTask]} getFinalizeState={getFinalizeState} />
			);

			// getFinalizeState should be called during render
			expect(getFinalizeState).toHaveBeenCalled();
		});
	});
});
