import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TaskHeader } from './TaskHeader';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, TaskCategory, TaskPriority, TaskQueue, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createTimestamp } from '@/test/factories';

// Mock the navigate function
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

// Mock the API functions
vi.mock('@/lib/api', () => ({
	deleteTask: vi.fn(),
	runTask: vi.fn(),
	pauseTask: vi.fn(),
	resumeTask: vi.fn(),
}));

// Mock the stores
vi.mock('@/stores', () => ({
	getInitiativeBadgeTitle: (id: string) => {
		if (id === 'INIT-001') {
			return { display: 'Test Init', full: 'Test Initiative' };
		}
		return null;
	},
	// useInitiatives is used by TaskEditModal (imported by TaskHeader)
	useInitiatives: () => [
		{ id: 'INIT-001', title: 'Test Initiative', status: 'active' },
	],
	useCurrentProjectId: () => 'test-project',
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

describe('TaskHeader', () => {
	const createTask = (overrides: Partial<Omit<Task, '$typeName' | '$unknown'>> = {}): Task => {
		return createMockTask({
			id: overrides.id ?? 'TASK-001',
			title: overrides.title ?? 'Test Task',
			description: overrides.description ?? 'Test description',
			status: overrides.status ?? TaskStatus.CREATED,
			weight: overrides.weight ?? TaskWeight.SMALL,
			branch: overrides.branch ?? 'orc/TASK-001',
			priority: overrides.priority ?? TaskPriority.NORMAL,
			category: overrides.category ?? TaskCategory.FEATURE,
			queue: overrides.queue ?? TaskQueue.ACTIVE,
			createdAt: overrides.createdAt ?? createTimestamp('2024-01-01T00:00:00Z'),
			updatedAt: overrides.updatedAt ?? createTimestamp('2024-01-01T00:00:00Z'),
			initiativeId: overrides.initiativeId,
			targetBranch: overrides.targetBranch,
			blockedBy: overrides.blockedBy ?? [],
			blocks: overrides.blocks ?? [],
			relatedTo: overrides.relatedTo ?? [],
			referencedBy: overrides.referencedBy ?? [],
			isBlocked: overrides.isBlocked ?? false,
			unmetBlockers: overrides.unmetBlockers ?? [],
			currentPhase: overrides.currentPhase,
			startedAt: overrides.startedAt,
			completedAt: overrides.completedAt,
			metadata: overrides.metadata ?? {},
		});
	};

	const defaultProps = {
		task: createTask(),
		onTaskUpdate: vi.fn(),
		onTaskDelete: vi.fn(),
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	const renderTaskHeader = (props = {}) => {
		return render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TaskHeader {...defaultProps} {...props} />
				</MemoryRouter>
			</TooltipProvider>
		);
	};

	describe('Initiative Badge', () => {
		it('renders initiative badge when task.initiativeId is set', () => {
			renderTaskHeader({
				task: createTask({ initiativeId: 'INIT-001' }),
			});

			const badge = screen.getByRole('button', { name: /test init/i });
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('initiative-badge');
		});

		it('hides initiative badge when task.initiativeId is undefined', () => {
			renderTaskHeader({
				task: createTask({ initiativeId: undefined }),
			});

			expect(screen.queryByText(/test init/i)).not.toBeInTheDocument();
		});

		it('navigates to initiative detail page when clicked', () => {
			renderTaskHeader({
				task: createTask({ initiativeId: 'INIT-001' }),
			});

			const badge = screen.getByRole('button', { name: /test init/i });
			fireEvent.click(badge);

			expect(mockNavigate).toHaveBeenCalledWith('/initiatives/INIT-001');
		});
	});

	describe('Priority Badge', () => {
		it('renders priority badge for critical priority', () => {
			renderTaskHeader({
				task: createTask({ priority: TaskPriority.CRITICAL }),
			});

			const badge = screen.getByText('Critical');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-critical');
		});

		it('renders priority badge for high priority', () => {
			renderTaskHeader({
				task: createTask({ priority: TaskPriority.HIGH }),
			});

			const badge = screen.getByText('High');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-high');
		});

		it('renders priority badge for low priority', () => {
			renderTaskHeader({
				task: createTask({ priority: TaskPriority.LOW }),
			});

			const badge = screen.getByText('Low');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-low');
		});

		it('renders priority badge for normal priority with subtle styling', () => {
			renderTaskHeader({
				task: createTask({ priority: TaskPriority.NORMAL }),
			});

			const badge = screen.getByText('Normal');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-normal');
		});

		it('defaults to normal priority when priority is not set', () => {
			renderTaskHeader({
				task: createTask({ priority: TaskPriority.UNSPECIFIED }),
			});

			const badge = screen.getByText('Normal');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-normal');
		});
	});

	describe('Badge Order', () => {
		it('renders badges in correct order: ID, status, weight, category, priority, initiative', () => {
			const { container } = renderTaskHeader({
				task: createTask({
					initiativeId: 'INIT-001',
					priority: TaskPriority.HIGH,
					category: TaskCategory.BUG,
					weight: TaskWeight.MEDIUM,
				}),
			});

			const identity = container.querySelector('.task-identity');
			expect(identity).toBeInTheDocument();

			const children = Array.from(identity!.children);
			const classes = children.map((c) => c.className);

			// Verify order of badges (StatusIndicator is rendered between task-id and weight)
			expect(classes[0]).toContain('task-id');
			// StatusIndicator is a more complex component
			expect(classes[2]).toContain('weight-badge');
			expect(classes[3]).toContain('category-badge');
			expect(classes[4]).toContain('priority-badge');
			expect(classes[5]).toContain('initiative-badge');
		});
	});
});

describe('Running Status Badge (TASK-312)', () => {
	// Helper to create a plan with phases
	// Phase status is completion-only (PENDING, COMPLETED, SKIPPED)
	// Running state is determined by being the current phase with PENDING status
	const createPlan = (phases: string[], currentPhase?: string) => ({
		version: 1,
		weight: TaskWeight.SMALL,
		description: 'Test plan',
		phases: phases.map((name, idx) => ({
			id: `phase-${idx}`,
			name,
			status: phases.indexOf(name) < phases.indexOf(currentPhase ?? '') ? PhaseStatus.COMPLETED : PhaseStatus.PENDING,
			iterations: 1,
		})),
	});

	const renderTaskHeader = (props = {}) => {
		const defaultProps = {
			task: createMockTask({
				id: 'TASK-001',
				title: 'Test Task',
				description: 'Test description',
				status: TaskStatus.CREATED,
				weight: TaskWeight.SMALL,
				branch: 'orc/TASK-001',
				priority: TaskPriority.NORMAL,
				category: TaskCategory.FEATURE,
				queue: TaskQueue.ACTIVE,
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
				updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
			onTaskUpdate: vi.fn(),
			onTaskDelete: vi.fn(),
		};
		return render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TaskHeader {...defaultProps} {...props} />
				</MemoryRouter>
			</TooltipProvider>
		);
	};

	describe('SC-1: Running status badge with phase name', () => {
		it('displays "Running: implement" badge when task is running with currentPhase=implement', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			});

			// Should display "Running: implement" prominently
			expect(screen.getByText(/Running.*implement/i)).toBeInTheDocument();
		});

		it('displays "Running: review" badge when task is running with currentPhase=review', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'review',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			});

			expect(screen.getByText(/Running.*review/i)).toBeInTheDocument();
		});

		it('does NOT display running badge when task is not running', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.COMPLETED,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			});

			expect(screen.queryByText(/Running.*implement/i)).not.toBeInTheDocument();
		});

		it('has pulse animation class for running status', () => {
			const { container } = renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			});

			// Running badge should have animation class
			const runningBadge = container.querySelector('.running-status-badge');
			expect(runningBadge).toBeInTheDocument();
			expect(runningBadge).toHaveClass('pulse');
		});
	});

	describe('SC-2: Phase progress indicator', () => {
		it('displays "2 of 4" when on second phase of four-phase plan', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
				plan: createPlan(['spec', 'implement', 'review', 'docs'], 'implement'),
			});

			// Should show phase progress
			expect(screen.getByText(/2 of 4/i)).toBeInTheDocument();
		});

		it('displays "3 of 5" when on third phase of five-phase plan', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.MEDIUM,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
				plan: createPlan(['spec', 'tdd_write', 'implement', 'review', 'docs'], 'implement'),
			});

			expect(screen.getByText(/3 of 5/i)).toBeInTheDocument();
		});

		it('does NOT display phase progress when plan is not provided', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.RUNNING,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
			});

			// Should NOT have progress like "X of Y" without plan
			expect(screen.queryByText(/\d+ of \d+/i)).not.toBeInTheDocument();
		});

		it('does NOT display phase progress when task is not running', () => {
			renderTaskHeader({
				task: createMockTask({
					id: 'TASK-001',
					title: 'Test Task',
					status: TaskStatus.COMPLETED,
					weight: TaskWeight.SMALL,
					branch: 'orc/TASK-001',
					currentPhase: 'implement',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
					updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
				plan: createPlan(['spec', 'implement', 'review', 'docs'], 'implement'),
			});

			expect(screen.queryByText(/\d+ of \d+/i)).not.toBeInTheDocument();
		});
	});
});
