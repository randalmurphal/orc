import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TaskCard } from './TaskCard';
import type { Task } from '@/lib/types';

// Mock navigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

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

// Sample task for testing
const createTask = (overrides: Partial<Task> = {}): Task => ({
	id: 'TASK-001',
	title: 'Test Task',
	description: 'A test task description',
	weight: 'medium',
	status: 'created',
	branch: 'orc/TASK-001',
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	...overrides,
});

function renderTaskCard(task: Task, props: Partial<Parameters<typeof TaskCard>[0]> = {}) {
	return render(
		<MemoryRouter>
			<TaskCard task={task} onAction={vi.fn()} {...props} />
		</MemoryRouter>
	);
}

describe('TaskCard', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('renders task ID and title', () => {
			renderTaskCard(createTask());

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('Test Task')).toBeInTheDocument();
		});

		it('renders task description', () => {
			renderTaskCard(createTask());

			expect(screen.getByText('A test task description')).toBeInTheDocument();
		});

		it('renders weight badge', () => {
			renderTaskCard(createTask({ weight: 'large' }));

			expect(screen.getByText('large')).toBeInTheDocument();
		});

		it('has correct aria-label', () => {
			renderTaskCard(createTask());

			expect(screen.getByRole('article')).toHaveAttribute(
				'aria-label',
				'Task TASK-001: Test Task'
			);
		});
	});

	describe('priority badge', () => {
		it('does not show priority badge for normal priority', () => {
			renderTaskCard(createTask({ priority: 'normal' }));

			const priorityBadge = screen.queryByTitle(/priority/i);
			expect(priorityBadge).not.toBeInTheDocument();
		});

		it('shows priority badge for critical priority', () => {
			renderTaskCard(createTask({ priority: 'critical' }));

			const priorityBadge = screen.getByTitle('Critical priority');
			expect(priorityBadge).toBeInTheDocument();
		});

		it('shows priority badge for high priority', () => {
			renderTaskCard(createTask({ priority: 'high' }));

			const priorityBadge = screen.getByTitle('High priority');
			expect(priorityBadge).toBeInTheDocument();
		});

		it('shows priority badge for low priority', () => {
			renderTaskCard(createTask({ priority: 'low' }));

			const priorityBadge = screen.getByTitle('Low priority');
			expect(priorityBadge).toBeInTheDocument();
		});
	});

	describe('status classes', () => {
		it('has running class when task is running', () => {
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }));

			expect(screen.getByRole('article')).toHaveClass('running');
		});

		it('has finalizing class when task is finalizing', () => {
			renderTaskCard(createTask({ status: 'finalizing' }));

			expect(screen.getByRole('article')).toHaveClass('finalizing');
		});

		it('has finished class when task is finished', () => {
			renderTaskCard(createTask({ status: 'finished' }));

			expect(screen.getByRole('article')).toHaveClass('finished');
		});

		it('has completed class when task is completed', () => {
			renderTaskCard(createTask({ status: 'completed' }));

			expect(screen.getByRole('article')).toHaveClass('completed');
		});
	});

	describe('current phase', () => {
		it('shows current phase when present', () => {
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }));

			expect(screen.getByText('Phase:')).toBeInTheDocument();
			expect(screen.getByText('implement')).toBeInTheDocument();
		});

		it('does not show phase when not present', () => {
			renderTaskCard(createTask({ status: 'created' }));

			expect(screen.queryByText('Phase:')).not.toBeInTheDocument();
		});
	});

	describe('blocked indicator', () => {
		it('shows blocked badge when task is blocked', () => {
			renderTaskCard(
				createTask({
					is_blocked: true,
					unmet_blockers: ['TASK-002', 'TASK-003'],
				})
			);

			expect(screen.getByText('Blocked')).toBeInTheDocument();
		});

		it('has title with blocker list', () => {
			renderTaskCard(
				createTask({
					is_blocked: true,
					unmet_blockers: ['TASK-002', 'TASK-003'],
				})
			);

			const blockedBadge = screen.getByText('Blocked').closest('.blocked-badge');
			expect(blockedBadge).toHaveAttribute('title', 'Blocked by TASK-002, TASK-003');
		});

		it('does not show blocked badge when not blocked', () => {
			renderTaskCard(createTask({ is_blocked: false }));

			expect(screen.queryByText('Blocked')).not.toBeInTheDocument();
		});
	});

	describe('initiative badge', () => {
		it('shows initiative badge when task has initiative', () => {
			renderTaskCard(createTask({ initiative_id: 'INIT-001' }));

			expect(screen.getByRole('button', { name: /init-0/i })).toBeInTheDocument();
		});

		it('calls onInitiativeClick when initiative badge is clicked', () => {
			const onInitiativeClick = vi.fn();
			renderTaskCard(createTask({ initiative_id: 'INIT-001' }), { onInitiativeClick });

			const initiativeBadge = screen.getByRole('button', { name: /init-0/i });
			fireEvent.click(initiativeBadge);

			expect(onInitiativeClick).toHaveBeenCalledWith('INIT-001');
		});

		it('does not show initiative badge when no initiative', () => {
			renderTaskCard(createTask({ initiative_id: undefined }));

			expect(screen.queryByRole('button', { name: /init/i })).not.toBeInTheDocument();
		});
	});

	describe('action buttons', () => {
		it('shows run button for created tasks', () => {
			renderTaskCard(createTask({ status: 'created' }));

			const runButton = screen.getByTitle('Run task');
			expect(runButton).toBeInTheDocument();
		});

		it('shows run button for planned tasks', () => {
			renderTaskCard(createTask({ status: 'planned' }));

			const runButton = screen.getByTitle('Run task');
			expect(runButton).toBeInTheDocument();
		});

		it('shows pause button for running tasks', () => {
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }));

			const pauseButton = screen.getByTitle('Pause task');
			expect(pauseButton).toBeInTheDocument();
		});

		it('shows resume button for paused tasks', () => {
			renderTaskCard(createTask({ status: 'paused' }));

			const resumeButton = screen.getByTitle('Resume task');
			expect(resumeButton).toBeInTheDocument();
		});

		it('shows finalize button for completed tasks', () => {
			renderTaskCard(createTask({ status: 'completed' }));

			const finalizeButton = screen.getByTitle('Finalize and merge');
			expect(finalizeButton).toBeInTheDocument();
		});

		it('calls onAction when run button is clicked', async () => {
			const onAction = vi.fn().mockResolvedValue(undefined);
			renderTaskCard(createTask({ status: 'created' }), { onAction });

			const runButton = screen.getByTitle('Run task');
			fireEvent.click(runButton);

			await waitFor(() => {
				expect(onAction).toHaveBeenCalledWith('TASK-001', 'run');
			});
		});

		it('calls onAction when pause button is clicked', async () => {
			const onAction = vi.fn().mockResolvedValue(undefined);
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }), { onAction });

			const pauseButton = screen.getByTitle('Pause task');
			fireEvent.click(pauseButton);

			await waitFor(() => {
				expect(onAction).toHaveBeenCalledWith('TASK-001', 'pause');
			});
		});

		it('calls onFinalizeClick when finalize button is clicked', async () => {
			const onFinalizeClick = vi.fn();
			renderTaskCard(createTask({ status: 'completed' }), { onFinalizeClick });

			const finalizeButton = screen.getByTitle('Finalize and merge');
			fireEvent.click(finalizeButton);

			await waitFor(() => {
				expect(onFinalizeClick).toHaveBeenCalledWith(
					expect.objectContaining({ id: 'TASK-001', status: 'completed' })
				);
			});
		});
	});

	describe('quick menu', () => {
		it('shows quick menu button', () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByTitle('Quick actions');
			expect(menuButton).toBeInTheDocument();
		});

		it('opens quick menu on button click', async () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByTitle('Quick actions');
			fireEvent.click(menuButton);

			await waitFor(() => {
				expect(screen.getByText('Queue')).toBeInTheDocument();
				expect(screen.getByText('Priority')).toBeInTheDocument();
			});
		});

		it('shows queue options in menu', async () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByTitle('Quick actions');
			fireEvent.click(menuButton);

			await waitFor(() => {
				expect(screen.getByRole('menuitem', { name: /active/i })).toBeInTheDocument();
				expect(screen.getByRole('menuitem', { name: /backlog/i })).toBeInTheDocument();
			});
		});

		it('shows priority options in menu', async () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByTitle('Quick actions');
			fireEvent.click(menuButton);

			await waitFor(() => {
				expect(screen.getByRole('menuitem', { name: /critical/i })).toBeInTheDocument();
				expect(screen.getByRole('menuitem', { name: /high/i })).toBeInTheDocument();
				expect(screen.getByRole('menuitem', { name: /normal/i })).toBeInTheDocument();
				expect(screen.getByRole('menuitem', { name: /low/i })).toBeInTheDocument();
			});
		});

		it('highlights current queue selection', async () => {
			renderTaskCard(createTask({ queue: 'backlog' }));

			const menuButton = screen.getByTitle('Quick actions');
			fireEvent.click(menuButton);

			await waitFor(() => {
				const backlogItem = screen.getByRole('menuitem', { name: /backlog/i });
				expect(backlogItem).toHaveClass('selected');
			});
		});

		it('highlights current priority selection', async () => {
			renderTaskCard(createTask({ priority: 'high' }));

			const menuButton = screen.getByTitle('Quick actions');
			fireEvent.click(menuButton);

			await waitFor(() => {
				const highItem = screen.getByRole('menuitem', { name: /high/i });
				expect(highItem).toHaveClass('selected');
			});
		});
	});

	describe('drag and drop', () => {
		it('is draggable', () => {
			renderTaskCard(createTask());

			expect(screen.getByRole('article')).toHaveAttribute('draggable', 'true');
		});

		it('adds dragging class on drag start', () => {
			renderTaskCard(createTask());

			const card = screen.getByRole('article');
			fireEvent.dragStart(card, {
				dataTransfer: { setData: vi.fn(), effectAllowed: '' },
			});

			expect(card).toHaveClass('dragging');
		});

		it('removes dragging class on drag end', () => {
			renderTaskCard(createTask());

			const card = screen.getByRole('article');
			fireEvent.dragStart(card, {
				dataTransfer: { setData: vi.fn(), effectAllowed: '' },
			});
			fireEvent.dragEnd(card);

			expect(card).not.toHaveClass('dragging');
		});

		it('sets task data on drag start', () => {
			const task = createTask();
			renderTaskCard(task);

			const setData = vi.fn();
			const card = screen.getByRole('article');
			fireEvent.dragStart(card, {
				dataTransfer: { setData, effectAllowed: '' },
			});

			expect(setData).toHaveBeenCalledWith('application/json', JSON.stringify(task));
		});
	});

	describe('navigation', () => {
		it('navigates to task detail on click', () => {
			renderTaskCard(createTask({ status: 'created' }));

			const card = screen.getByRole('article');
			fireEvent.click(card);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('calls onTaskClick for running tasks instead of navigating', () => {
			const onTaskClick = vi.fn();
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }), {
				onTaskClick,
			});

			const card = screen.getByRole('article');
			fireEvent.click(card);

			expect(onTaskClick).toHaveBeenCalled();
			expect(mockNavigate).not.toHaveBeenCalled();
		});

		it('navigates on Enter key', () => {
			renderTaskCard(createTask());

			const card = screen.getByRole('article');
			fireEvent.keyDown(card, { key: 'Enter' });

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('navigates on Space key', () => {
			renderTaskCard(createTask());

			const card = screen.getByRole('article');
			fireEvent.keyDown(card, { key: ' ' });

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});
	});

	describe('finalize state', () => {
		it('shows finalize progress when finalizing', () => {
			renderTaskCard(
				createTask({ status: 'finalizing' }),
				{
					finalizeState: {
						task_id: 'TASK-001',
						status: 'running',
						step: 'Syncing branch',
						progress: '50%',
						step_percent: 50,
					},
				}
			);

			expect(screen.getByText('Syncing branch')).toBeInTheDocument();
		});

		it('shows commit info when finished', () => {
			renderTaskCard(
				createTask({ status: 'finished' }),
				{
					finalizeState: {
						task_id: 'TASK-001',
						status: 'completed',
						result: {
							synced: true,
							conflicts_resolved: 0,
							tests_passed: true,
							risk_level: 'low',
							files_changed: 5,
							lines_changed: 100,
							needs_review: false,
							commit_sha: 'abc1234567890',
							target_branch: 'main',
						},
					},
				}
			);

			expect(screen.getByText('abc1234')).toBeInTheDocument();
			expect(screen.getByText('merged to main')).toBeInTheDocument();
		});
	});
});
