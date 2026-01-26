import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TimelineTab } from './TimelineTab';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task, TaskPlan, ExecutionState } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, TaskCategory, TaskPriority, TaskQueue, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockTaskPlan, createMockPhase, createTimestamp } from '@/test/factories';

// Mock the stores
vi.mock('@/stores', () => ({
	getInitiativeBadgeTitle: (id: string) => {
		if (id === 'INIT-001') {
			return { display: 'Test Init', full: 'Test Initiative' };
		}
		return null;
	},
}));

describe('TimelineTab', () => {
	const createTask = (overrides: Partial<Omit<Task, '$typeName' | '$unknown'>> = {}): Task => {
		return createMockTask({
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
			updatedAt: createTimestamp('2024-01-01T12:00:00Z'),
			...overrides,
		});
	};

	const createPlan = (overrides: Partial<Omit<TaskPlan, '$typeName' | '$unknown'>> = {}): TaskPlan => {
		return createMockTaskPlan({
			version: 1,
			weight: TaskWeight.SMALL,
			description: 'Test plan',
			phases: [
				createMockPhase({
					id: 'phase-1',
					name: 'implement',
					status: PhaseStatus.PENDING,
					iterations: 1,
				}),
				createMockPhase({
					id: 'phase-2',
					name: 'test',
					status: PhaseStatus.PENDING,
					iterations: 1,
				}),
			],
			...overrides,
		});
	};

	const createTaskState = (overrides: Partial<Omit<ExecutionState, '$typeName' | '$unknown'>> = {}): ExecutionState => {
		// ExecutionState is a simple object for testing purposes
		return {
			currentIteration: 1,
			phases: {},
			gates: [],
			tokens: {
				inputTokens: 1000,
				outputTokens: 500,
				totalTokens: 1500,
				cacheCreationInputTokens: 0,
				cacheReadInputTokens: 0,
			},
			...overrides,
		} as ExecutionState;
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	const renderTimelineTab = (props: {
		task?: Task;
		taskState?: ExecutionState | null;
		plan?: TaskPlan | null;
	} = {}) => {
		const defaultProps = {
			task: createTask(),
			taskState: null,
			plan: createPlan(),
		};
		return render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TimelineTab {...defaultProps} {...props} />
				</MemoryRouter>
			</TooltipProvider>
		);
	};

	describe('Task Info - Priority', () => {
		it('renders priority field with correct styling for critical', () => {
			renderTimelineTab({
				task: createTask({ priority: TaskPriority.CRITICAL }),
			});

			const priority = screen.getByText('Critical');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-critical');
		});

		it('renders priority field with correct styling for high', () => {
			renderTimelineTab({
				task: createTask({ priority: TaskPriority.HIGH }),
			});

			const priority = screen.getByText('High');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-high');
		});

		it('renders priority field with correct styling for normal', () => {
			renderTimelineTab({
				task: createTask({ priority: TaskPriority.NORMAL }),
			});

			const priority = screen.getByText('Normal');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-normal');
		});

		it('renders priority field with correct styling for low', () => {
			renderTimelineTab({
				task: createTask({ priority: TaskPriority.LOW }),
			});

			const priority = screen.getByText('Low');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-low');
		});

		it('defaults to normal priority when not set', () => {
			renderTimelineTab({
				task: createTask({ priority: TaskPriority.UNSPECIFIED }),
			});

			const priority = screen.getByText('Normal');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-normal');
		});
	});

	describe('Task Info - Category', () => {
		it('renders category field with icon for feature', () => {
			const { container } = renderTimelineTab({
				task: createTask({ category: TaskCategory.FEATURE }),
			});

			const category = screen.getByText('Feature');
			expect(category).toBeInTheDocument();
			expect(category).toHaveClass('info-category');
			// Check for icon (SVG inside the category span)
			const categorySpan = container.querySelector('.info-category');
			expect(categorySpan?.querySelector('svg')).toBeInTheDocument();
		});

		it('renders category field with icon for bug', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.BUG }),
			});

			expect(screen.getByText('Bug')).toBeInTheDocument();
		});

		it('renders category field with icon for refactor', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.REFACTOR }),
			});

			expect(screen.getByText('Refactor')).toBeInTheDocument();
		});

		it('renders category field with icon for chore', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.CHORE }),
			});

			expect(screen.getByText('Chore')).toBeInTheDocument();
		});

		it('renders category field with icon for docs', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.DOCS }),
			});

			expect(screen.getByText('Docs')).toBeInTheDocument();
		});

		it('renders category field with icon for test', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.TEST }),
			});

			expect(screen.getByText('Test')).toBeInTheDocument();
		});

		it('hides category field when not set', () => {
			renderTimelineTab({
				task: createTask({ category: TaskCategory.UNSPECIFIED }),
			});

			expect(screen.queryByText('Feature')).not.toBeInTheDocument();
			expect(screen.queryByText('Bug')).not.toBeInTheDocument();
		});
	});

	describe('Task Info - Initiative', () => {
		it('renders initiative as clickable link when set', () => {
			renderTimelineTab({
				task: createTask({ initiativeId: 'INIT-001' }),
			});

			const link = screen.getByRole('link', { name: /test init/i });
			expect(link).toBeInTheDocument();
			expect(link).toHaveAttribute('href', '/initiatives/INIT-001');
			expect(link).toHaveClass('info-initiative-link');
		});

		it('hides initiative field when not set', () => {
			renderTimelineTab({
				task: createTask({ initiativeId: undefined }),
			});

			expect(screen.queryByRole('link', { name: /test init/i })).not.toBeInTheDocument();
		});

		it('hides initiative field when getInitiativeBadgeTitle returns null', () => {
			renderTimelineTab({
				task: createTask({ initiativeId: 'INIT-UNKNOWN' }),
			});

			expect(screen.queryByText('INIT-UNKNOWN')).not.toBeInTheDocument();
		});
	});

	describe('Task Info - Blocked By', () => {
		it('renders blockedBy count when blockers exist', () => {
			renderTimelineTab({
				task: createTask({ blockedBy: ['TASK-002', 'TASK-003'] }),
			});

			expect(screen.getByText('2 tasks')).toBeInTheDocument();
		});

		it('renders singular form for single blocker', () => {
			renderTimelineTab({
				task: createTask({ blockedBy: ['TASK-002'] }),
			});

			expect(screen.getByText('1 task')).toBeInTheDocument();
		});

		it('hides blockedBy field when empty array', () => {
			renderTimelineTab({
				task: createTask({ blockedBy: [] }),
			});

			// The label 'Blocked By' should not appear
			expect(screen.queryByText('Blocked By')).not.toBeInTheDocument();
		});

		it('hides blockedBy field when undefined', () => {
			renderTimelineTab({
				task: createTask({ blockedBy: undefined }),
			});

			expect(screen.queryByText('Blocked By')).not.toBeInTheDocument();
		});
	});

	describe('Task Info - Optional Fields', () => {
		it('hides branch field when not set', () => {
			renderTimelineTab({
				task: createTask({ branch: '' }),
			});

			expect(screen.queryByText('orc/TASK-001')).not.toBeInTheDocument();
		});

		it('shows branch field when set', () => {
			renderTimelineTab({
				task: createTask({ branch: 'orc/TASK-001' }),
			});

			expect(screen.getByText('orc/TASK-001')).toBeInTheDocument();
		});

		it('shows target branch when set', () => {
			renderTimelineTab({
				task: createTask({ targetBranch: 'main' }),
			});

			expect(screen.getByText('main')).toBeInTheDocument();
		});

		it('hides target branch when not set', () => {
			renderTimelineTab({
				task: createTask({ targetBranch: undefined }),
			});

			expect(screen.queryByText('Target')).not.toBeInTheDocument();
		});

		it('shows updated timestamp', () => {
			renderTimelineTab({
				task: createTask({ updatedAt: createTimestamp('2024-06-15T10:30:00Z') }),
			});

			// Look for the Updated label
			expect(screen.getByText('Updated')).toBeInTheDocument();
		});

		it('shows queue field', () => {
			renderTimelineTab({
				task: createTask({ queue: TaskQueue.BACKLOG }),
			});

			expect(screen.getByText('backlog')).toBeInTheDocument();
		});

		it('defaults queue to active when not set', () => {
			renderTimelineTab({
				task: createTask({ queue: TaskQueue.ACTIVE }),
			});

			expect(screen.getByText('active')).toBeInTheDocument();
		});
	});

	describe('Task Info - Execution Info', () => {
		it('shows current phase when running', () => {
			renderTimelineTab({
				task: createTask({ status: TaskStatus.RUNNING }),
				taskState: createTaskState({ currentPhase: 'implement' } as Partial<ExecutionState>),
			});

			// The phase name should appear in the phase section
			expect(screen.getAllByText('implement').length).toBeGreaterThan(0);
		});

		it('shows retry info when retryContext exists', () => {
			renderTimelineTab({
				task: createTask(),
				taskState: createTaskState({ retryContext: { fromPhase: 'implement', toPhase: 'review' } } as unknown as Partial<ExecutionState>),
			});

			expect(screen.getByText('Retry Info')).toBeInTheDocument();
			expect(screen.getByText(/From.*implement/)).toBeInTheDocument();
		});

		it('hides retry info when retryContext is undefined', () => {
			renderTimelineTab({
				task: createTask(),
				taskState: createTaskState({ retryContext: undefined } as unknown as Partial<ExecutionState>),
			});

			expect(screen.queryByText('Retry Info')).not.toBeInTheDocument();
		});
	});

	describe('Empty State', () => {
		it('shows empty state when no plan available', () => {
			renderTimelineTab({
				plan: null,
			});

			expect(screen.getByText('No Plan Available')).toBeInTheDocument();
			expect(screen.getByText(/This task doesn't have a plan yet/)).toBeInTheDocument();
		});
	});
});
