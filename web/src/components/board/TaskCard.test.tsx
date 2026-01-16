import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TaskCard } from './TaskCard';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/lib/types';

// Mock browser APIs not available in jsdom (required by Radix DropdownMenu)
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

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
		display: id, // Now shows full initiative ID like "INIT-001"
		full: `${id}: Test Initiative`,
		id,
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
		<TooltipProvider delayDuration={0}>
			<MemoryRouter>
				<TaskCard task={task} onAction={vi.fn()} {...props} />
			</MemoryRouter>
		</TooltipProvider>
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

		it('renders without draggable attribute', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card');
			expect(card).not.toHaveAttribute('draggable');
		});

		it('renders task description', () => {
			renderTaskCard(createTask());

			expect(screen.getByText('A test task description')).toBeInTheDocument();
		});

		it('truncates description to 3 lines via CSS class', () => {
			const longDescription =
				'This is a very long description that could potentially span many lines if not truncated properly by the CSS styling applied to the task card component';
			const { container } = renderTaskCard(createTask({ description: longDescription }));

			const descriptionElement = container.querySelector('.task-description');
			expect(descriptionElement).toBeInTheDocument();
			// The CSS class applies line-clamp: 3 truncation
			expect(descriptionElement).toHaveClass('task-description');
		});

		it('normalizes markdown formatting in description display', () => {
			const markdownDescription = `# Heading

**Bold text** and *italic text*

- List item 1
- List item 2

Some \`inline code\` here`;

			const { container } = renderTaskCard(createTask({ description: markdownDescription }));

			const descriptionElement = container.querySelector('.task-description');
			expect(descriptionElement).toBeInTheDocument();
			// Should be normalized - no markdown markers visible
			expect(descriptionElement?.textContent).not.toContain('#');
			expect(descriptionElement?.textContent).not.toContain('**');
			expect(descriptionElement?.textContent).not.toContain('*italic');
			expect(descriptionElement?.textContent).not.toContain('- List');
			expect(descriptionElement?.textContent).not.toContain('`');
			// Should contain the actual text content
			expect(descriptionElement?.textContent).toContain('Heading');
			expect(descriptionElement?.textContent).toContain('Bold text');
			expect(descriptionElement?.textContent).toContain('inline code');
		});

		it('wraps description in tooltip showing full text', () => {
			const longDescription =
				'This is a very long description that would be truncated in the card display';
			const { container } = renderTaskCard(createTask({ description: longDescription }));

			// The description element is wrapped in a Radix Tooltip trigger
			// Radix uses asChild so the trigger attributes are applied to the child element
			const descriptionElement = container.querySelector('.task-description');
			expect(descriptionElement).toBeInTheDocument();
			// Radix Tooltip trigger adds data-state attribute to the child
			expect(descriptionElement).toHaveAttribute('data-state');
		});

		it('renders weight badge', () => {
			renderTaskCard(createTask({ weight: 'large' }));

			expect(screen.getByText('large')).toBeInTheDocument();
		});

		it('has correct aria-label', () => {
			renderTaskCard(createTask());

			const card = screen.getByLabelText('Task TASK-001: Test Task');
			expect(card).toBeInTheDocument();
		});
	});

	describe('priority badge', () => {
		it('does not show priority badge for normal priority', () => {
			const { container } = renderTaskCard(createTask({ priority: 'normal' }));

			const priorityBadge = container.querySelector('.priority-badge');
			expect(priorityBadge).not.toBeInTheDocument();
		});

		it('shows priority badge for critical priority', () => {
			const { container } = renderTaskCard(createTask({ priority: 'critical' }));

			const priorityBadge = container.querySelector('.priority-badge.critical');
			expect(priorityBadge).toBeInTheDocument();
		});

		it('shows priority badge for high priority', () => {
			const { container } = renderTaskCard(createTask({ priority: 'high' }));

			const priorityBadge = container.querySelector('.priority-badge.high');
			expect(priorityBadge).toBeInTheDocument();
		});

		it('shows priority badge for low priority', () => {
			const { container } = renderTaskCard(createTask({ priority: 'low' }));

			const priorityBadge = container.querySelector('.priority-badge.low');
			expect(priorityBadge).toBeInTheDocument();
		});
	});

	describe('status classes', () => {
		it('has running class when task is running', () => {
			const { container } = renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }));

			expect(container.querySelector('.task-card')).toHaveClass('running');
		});

		it('has finalizing class when task is finalizing', () => {
			const { container } = renderTaskCard(createTask({ status: 'finalizing' }));

			expect(container.querySelector('.task-card')).toHaveClass('finalizing');
		});

		it('has completed class when task is completed', () => {
			const { container } = renderTaskCard(createTask({ status: 'completed' }));

			expect(container.querySelector('.task-card')).toHaveClass('completed');
		});

		it('has blocked class when task status is blocked', () => {
			const { container } = renderTaskCard(createTask({ status: 'blocked' }));

			expect(container.querySelector('.task-card')).toHaveClass('blocked');
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

		it('has blocked badge with correct class', () => {
			const { container } = renderTaskCard(
				createTask({
					is_blocked: true,
					unmet_blockers: ['TASK-002', 'TASK-003'],
				})
			);

			const blockedBadge = container.querySelector('.blocked-badge');
			expect(blockedBadge).toBeInTheDocument();
		});

		it('does not show blocked badge when not blocked', () => {
			renderTaskCard(createTask({ is_blocked: false }));

			expect(screen.queryByText('Blocked')).not.toBeInTheDocument();
		});
	});

	describe('initiative badge', () => {
		it('shows initiative badge with initiative ID when task has initiative', () => {
			renderTaskCard(createTask({ initiative_id: 'INIT-001' }));

			// Badge should display the initiative ID (INIT-001)
			expect(screen.getByRole('button', { name: /INIT-001/i })).toBeInTheDocument();
		});

		it('calls onInitiativeClick when initiative badge is clicked', () => {
			const onInitiativeClick = vi.fn();
			renderTaskCard(createTask({ initiative_id: 'INIT-001' }), { onInitiativeClick });

			const initiativeBadge = screen.getByRole('button', { name: /INIT-001/i });
			fireEvent.click(initiativeBadge);

			expect(onInitiativeClick).toHaveBeenCalledWith('INIT-001');
		});

		it('does not show initiative badge when no initiative', () => {
			renderTaskCard(createTask({ initiative_id: undefined }));

			expect(screen.queryByRole('button', { name: /init/i })).not.toBeInTheDocument();
		});

		it('initiative badge has correct styling class', () => {
			const { container } = renderTaskCard(createTask({ initiative_id: 'INIT-001' }));

			const initiativeBadge = container.querySelector('.initiative-badge');
			expect(initiativeBadge).toBeInTheDocument();
		});
	});

	describe('action buttons', () => {
		it('shows run button for created tasks', () => {
			renderTaskCard(createTask({ status: 'created' }));

			const runButton = screen.getByLabelText('Run task');
			expect(runButton).toBeInTheDocument();
		});

		it('shows run button for planned tasks', () => {
			renderTaskCard(createTask({ status: 'planned' }));

			const runButton = screen.getByLabelText('Run task');
			expect(runButton).toBeInTheDocument();
		});

		it('shows pause button for running tasks', () => {
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }));

			const pauseButton = screen.getByLabelText('Pause task');
			expect(pauseButton).toBeInTheDocument();
		});

		it('shows resume button for paused tasks', () => {
			renderTaskCard(createTask({ status: 'paused' }));

			const resumeButton = screen.getByLabelText('Resume task');
			expect(resumeButton).toBeInTheDocument();
		});

		it('shows finalize button for completed tasks', () => {
			renderTaskCard(createTask({ status: 'completed' }));

			const finalizeButton = screen.getByLabelText('Finalize and merge');
			expect(finalizeButton).toBeInTheDocument();
		});

		it('calls onAction when run button is clicked', async () => {
			const onAction = vi.fn().mockResolvedValue(undefined);
			renderTaskCard(createTask({ status: 'created' }), { onAction });

			const runButton = screen.getByLabelText('Run task');
			fireEvent.click(runButton);

			await waitFor(() => {
				expect(onAction).toHaveBeenCalledWith('TASK-001', 'run');
			});
		});

		it('calls onAction when pause button is clicked', async () => {
			const onAction = vi.fn().mockResolvedValue(undefined);
			renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }), { onAction });

			const pauseButton = screen.getByLabelText('Pause task');
			fireEvent.click(pauseButton);

			await waitFor(() => {
				expect(onAction).toHaveBeenCalledWith('TASK-001', 'pause');
			});
		});

		it('calls onFinalizeClick when finalize button is clicked', async () => {
			const onFinalizeClick = vi.fn();
			renderTaskCard(createTask({ status: 'completed' }), { onFinalizeClick });

			const finalizeButton = screen.getByLabelText('Finalize and merge');
			fireEvent.click(finalizeButton);

			await waitFor(() => {
				expect(onFinalizeClick).toHaveBeenCalledWith(
					expect.objectContaining({ id: 'TASK-001', status: 'completed' })
				);
			});
		});
	});

	describe('quick menu', () => {
		it('shows quick menu trigger button', () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByLabelText('Quick actions');
			expect(menuButton).toBeInTheDocument();
		});

		it('quick menu trigger has correct ARIA attributes', () => {
			renderTaskCard(createTask());

			const menuButton = screen.getByLabelText('Quick actions');
			expect(menuButton).toHaveAttribute('aria-haspopup', 'menu');
			expect(menuButton).toHaveAttribute('aria-expanded', 'false');
			expect(menuButton).toHaveAttribute('data-state', 'closed');
		});

		// Note: Testing menu content and interaction is better done via E2E tests
		// because Radix DropdownMenu uses portals which have limitations in jsdom.
		// The following tests verify the trigger renders correctly and the component
		// is properly wired up to Radix DropdownMenu.
	});

	describe('navigation', () => {
		it('navigates to task detail on click', () => {
			const { container } = renderTaskCard(createTask({ status: 'created' }));

			const card = container.querySelector('.task-card')!;
			fireEvent.click(card);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('calls onTaskClick for running tasks instead of navigating', () => {
			const onTaskClick = vi.fn();
			const { container } = renderTaskCard(createTask({ status: 'running', current_phase: 'implement' }), {
				onTaskClick,
			});

			const card = container.querySelector('.task-card')!;
			fireEvent.click(card);

			expect(onTaskClick).toHaveBeenCalled();
			expect(mockNavigate).not.toHaveBeenCalled();
		});

		it('navigates on Enter key', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card')!;
			fireEvent.keyDown(card, { key: 'Enter' });

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('navigates on Space key', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card')!;
			fireEvent.keyDown(card, { key: ' ' });

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});
	});

	describe('updated time display', () => {
		it('shows relative time for recent updates', () => {
			// Set updated_at to 5 minutes ago
			const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000).toISOString();
			const { container } = renderTaskCard(createTask({ updated_at: fiveMinutesAgo }));

			const timeElement = container.querySelector('.updated-time');
			expect(timeElement).toBeInTheDocument();
			expect(timeElement?.textContent).toMatch(/5m ago/);
		});

		it('shows empty string for empty date', () => {
			// Test with empty updated_at - shouldn't crash
			const { container } = renderTaskCard(createTask({ updated_at: '' }));

			const timeElement = container.querySelector('.updated-time');
			expect(timeElement).toBeInTheDocument();
			expect(timeElement?.textContent).toBe('');
		});

		it('shows empty string for invalid date', () => {
			// Test with invalid date string - shouldn't crash
			const { container } = renderTaskCard(createTask({ updated_at: 'not-a-date' }));

			const timeElement = container.querySelector('.updated-time');
			expect(timeElement).toBeInTheDocument();
			expect(timeElement?.textContent).toBe('');
		});

		it('shows formatted date with 4-digit year for old dates', () => {
			// Set updated_at to more than 7 days ago
			const { container } = renderTaskCard(createTask({ updated_at: '2024-01-15T00:00:00Z' }));

			const timeElement = container.querySelector('.updated-time');
			expect(timeElement).toBeInTheDocument();
			// Should contain 4-digit year (2024), not truncated
			expect(timeElement?.textContent).toMatch(/2024/);
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

		it('shows commit info when completed with finalize result', () => {
			renderTaskCard(
				createTask({ status: 'completed' }),
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
