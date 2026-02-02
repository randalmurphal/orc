import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AttentionDashboard } from './AttentionDashboard';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskPriority, TaskCategory } from '@/gen/orc/v1/task_pb';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import type { PendingDecision } from '@/gen/orc/v1/decision_pb';
import { createMockTask, createMockInitiative, createMockDecision, createTimestamp } from '@/test/factories';

// Mock events module
vi.mock('@/lib/events', () => ({
	EventSubscription: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn().mockReturnValue(() => {}),
		getStatus: vi.fn().mockReturnValue('disconnected'),
	})),
	handleEvent: vi.fn(),
}));

// Mock stores
const mockTasks: Task[] = [];
const mockTaskStates = new Map();
const mockLoading = false;
const mockInitiatives: Initiative[] = [];
const mockPendingDecisions: PendingDecision[] = [];
const mockBlockedTasks: Task[] = [];

// Mock taskStore
vi.mock('@/stores/taskStore', () => ({
	useTaskStore: (selector: (state: unknown) => unknown) => {
		const state = {
			tasks: mockTasks,
			taskStates: mockTaskStates,
			loading: mockLoading,
		};
		return selector(state);
	},
}));

// Mock initiativeStore
vi.mock('@/stores/initiativeStore', () => ({
	useInitiatives: () => mockInitiatives,
}));

// Mock uiStore
vi.mock('@/stores/uiStore', () => ({
	useUIStore: (selector: (state: unknown) => unknown) => {
		const state = {
			pendingDecisions: mockPendingDecisions,
			removePendingDecision: vi.fn(),
			wsStatus: 'connected',
			setWsStatus: vi.fn(),
			toasts: [],
			addToast: vi.fn(),
		};
		return selector(state);
	},
	usePendingDecisions: () => mockPendingDecisions,
}));

// Mock router navigation
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

// Helper to render with required providers
function renderAttentionDashboard() {
	return render(
		<TooltipProvider>
			<MemoryRouter>
				<AttentionDashboard />
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('AttentionDashboard', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Reset mock data to defaults
		mockTasks.length = 0;
		mockTaskStates.clear();
		mockPendingDecisions.length = 0;
		mockBlockedTasks.length = 0;
		mockInitiatives.length = 0;
	});

	afterEach(() => {
		cleanup();
	});

	// SC-1: Three Main Sections (Running, Needs Attention, Queue)
	describe('SC-1: Three main sections', () => {
		it('renders all three main sections', () => {
			renderAttentionDashboard();

			expect(screen.getByRole('region', { name: /running/i })).toBeInTheDocument();
			expect(screen.getByRole('region', { name: /needs attention/i })).toBeInTheDocument();
			expect(screen.getByRole('region', { name: /queue/i })).toBeInTheDocument();
		});

		it('sections are arranged in correct priority order', () => {
			const { container } = renderAttentionDashboard();

			const dashboard = container.querySelector('.attention-dashboard');
			expect(dashboard).toBeInTheDocument();

			// Should have three main sections in order
			const sections = container.querySelectorAll('.attention-dashboard > section');
			expect(sections.length).toBe(3);

			expect(sections[0]).toHaveClass('running-section');
			expect(sections[1]).toHaveClass('attention-section');
			expect(sections[2]).toHaveClass('queue-section');
		});

		it('has proper accessibility labels for screen readers', () => {
			renderAttentionDashboard();

			const runningSection = screen.getByRole('region', { name: /running tasks/i });
			const attentionSection = screen.getByRole('region', { name: /needs attention/i });
			const queueSection = screen.getByRole('region', { name: /task queue/i });

			expect(runningSection).toHaveAttribute('aria-labelledby');
			expect(attentionSection).toHaveAttribute('aria-labelledby');
			expect(queueSection).toHaveAttribute('aria-labelledby');
		});
	});

	// SC-2: Running Section Features
	describe('SC-2: Running section features', () => {
		it('displays running tasks with timing and progress', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				title: 'Implement authentication',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
				startedAt: createTimestamp('2024-01-01T00:00:00Z'),
			});
			mockTasks.push(runningTask);

			vi.useFakeTimers();
			vi.setSystemTime(new Date('2024-01-01T00:05:30Z'));

			renderAttentionDashboard();

			const runningSection = screen.getByRole('region', { name: /running/i });
			expect(runningSection).toBeInTheDocument();

			// Should show task ID and title
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('Implement authentication')).toBeInTheDocument();

			// Should show current phase
			expect(screen.getByText(/implement/i)).toBeInTheDocument();

			// Should show elapsed time (5:30 from start time)
			expect(screen.getByText('5:30')).toBeInTheDocument();

			vi.useRealTimers();
		});

		it('displays 5-phase progress pipeline (Plan → Code → Test → Review → Done)', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			mockTasks.push(runningTask);

			renderAttentionDashboard();

			// Should show all 5 pipeline phases
			expect(screen.getByText('Plan')).toBeInTheDocument();
			expect(screen.getByText('Code')).toBeInTheDocument();
			expect(screen.getByText('Test')).toBeInTheDocument();
			expect(screen.getByText('Review')).toBeInTheDocument();
			expect(screen.getByText('Done')).toBeInTheDocument();

			// Should show current phase as active
			const codePhase = screen.getByText('Code').closest('.pipeline-step');
			expect(codePhase).toHaveClass('active');
		});

		it('displays initiative badge when task belongs to initiative', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
				initiativeId: 'INIT-001',
			});
			const initiative = createMockInitiative({
				id: 'INIT-001',
				title: 'User Auth System',
			});
			mockTasks.push(runningTask);
			mockInitiatives.push(initiative);

			renderAttentionDashboard();

			expect(screen.getByText('User Auth System')).toBeInTheDocument();
		});

		it('supports expandable output terminal', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
			});
			mockTasks.push(runningTask);

			renderAttentionDashboard();

			const taskCard = screen.getByText('TASK-001').closest('.running-card');
			expect(taskCard).toBeInTheDocument();

			// Should start collapsed
			const output = taskCard?.querySelector('.running-output');
			expect(output).not.toHaveClass('expanded');

			// Should expand when clicked
			fireEvent.click(taskCard!);
			expect(output).toHaveClass('expanded');
		});
	});

	// SC-3: Needs Attention Section Features
	describe('SC-3: Needs attention section features', () => {
		it('displays blocked tasks with action buttons', () => {
			const blockedTask = createMockTask({
				id: 'TASK-002',
				title: 'Deploy to production',
				status: TaskStatus.BLOCKED,
				blockedBy: ['TASK-001'],
			});
			mockTasks.push(blockedTask);
			mockBlockedTasks.push(blockedTask);

			renderAttentionDashboard();

			const attentionSection = screen.getByRole('region', { name: /needs attention/i });
			expect(attentionSection).toBeInTheDocument();

			// Should show blocked task
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('Deploy to production')).toBeInTheDocument();

			// Should show action buttons
			expect(screen.getByRole('button', { name: /skip/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /force/i })).toBeInTheDocument();
		});

		it('displays pending decisions with choice options', () => {
			const decision = createMockDecision({
				id: 'DEC-001',
				taskId: 'TASK-001',
				taskTitle: 'Add authentication',
				question: 'Which authentication method?',
				options: [
					{ $typeName: 'orc.v1.DecisionOption', id: 'jwt', label: 'JWT tokens', description: 'Stateless JWT tokens', recommended: true },
					{ $typeName: 'orc.v1.DecisionOption', id: 'sessions', label: 'Server sessions', description: 'Traditional sessions', recommended: false },
				],
			});
			mockPendingDecisions.push(decision);

			renderAttentionDashboard();

			// Should show decision question
			expect(screen.getByText('Which authentication method?')).toBeInTheDocument();

			// Should show choice options
			expect(screen.getByRole('button', { name: /jwt tokens/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /server sessions/i })).toBeInTheDocument();

			// Should highlight recommended option
			const jwtOption = screen.getByRole('button', { name: /jwt tokens/i });
			expect(jwtOption).toHaveClass('recommended');
		});

		it('displays gate approvals waiting for user action', () => {
			const approvalDecision = createMockDecision({
				id: 'DEC-002',
				taskId: 'TASK-003',
				taskTitle: 'Refactor database layer',
				gateType: 'approval',
				question: 'Ready for review?',
			});
			mockPendingDecisions.push(approvalDecision);

			renderAttentionDashboard();

			// Should show approval gate
			expect(screen.getByText('Ready for review?')).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /approve/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /reject/i })).toBeInTheDocument();
		});

		it('prioritizes high-priority attention items first', () => {
			const blockedTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.BLOCKED,
				priority: TaskPriority.HIGH,
			});
			const lowPriorityDecision = createMockDecision({
				id: 'DEC-001',
				taskId: 'TASK-002',
			});

			mockBlockedTasks.push(blockedTask);
			mockPendingDecisions.push(lowPriorityDecision);

			const { container } = renderAttentionDashboard();

			const attentionItems = container.querySelectorAll('.attention-item');
			expect(attentionItems.length).toBe(2);

			// High priority blocked task should appear first
			expect(attentionItems[0]).toContain('TASK-001');
			expect(attentionItems[1]).toContain('TASK-002');
		});
	});

	// SC-4: Queue Section Features
	describe('SC-4: Queue section features', () => {
		it('organizes ready tasks by initiative swimlanes', () => {
			const initiative = createMockInitiative({
				id: 'INIT-001',
				title: 'Frontend Polish',
			});
			const queuedTask1 = createMockTask({
				id: 'TASK-001',
				title: 'Refactor button variants',
				status: TaskStatus.PLANNED,
				initiativeId: 'INIT-001',
			});
			const queuedTask2 = createMockTask({
				id: 'TASK-002',
				title: 'Add loading states',
				status: TaskStatus.PLANNED,
				initiativeId: 'INIT-001',
			});

			mockInitiatives.push(initiative);
			mockTasks.push(queuedTask1, queuedTask2);

			renderAttentionDashboard();

			const queueSection = screen.getByRole('region', { name: /queue/i });
			expect(queueSection).toBeInTheDocument();

			// Should show initiative swimlane
			expect(screen.getByText('Frontend Polish')).toBeInTheDocument();

			// Should show tasks within swimlane
			expect(screen.getByText('Refactor button variants')).toBeInTheDocument();
			expect(screen.getByText('Add loading states')).toBeInTheDocument();
		});

		it('displays task position numbering within swimlanes', () => {
			const queuedTask1 = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.PLANNED,
				initiativeId: 'INIT-001',
			});
			const queuedTask2 = createMockTask({
				id: 'TASK-002',
				status: TaskStatus.PLANNED,
				initiativeId: 'INIT-001',
			});
			mockTasks.push(queuedTask1, queuedTask2);

			const { container } = renderAttentionDashboard();

			// Should show position numbers
			const positions = container.querySelectorAll('.task-position');
			expect(positions.length).toBe(2);
			expect(positions[0]).toHaveTextContent('1');
			expect(positions[1]).toHaveTextContent('2');
		});

		it('displays priority indicators for high-priority tasks', () => {
			const highPriorityTask = createMockTask({
				id: 'TASK-001',
				title: 'Fix critical bug',
				status: TaskStatus.PLANNED,
				priority: TaskPriority.HIGH,
			});
			mockTasks.push(highPriorityTask);

			renderAttentionDashboard();

			// Should show high priority badge
			expect(screen.getByText(/high/i)).toBeInTheDocument();
		});

		it('shows compact task cards with ID, title, and category', () => {
			const queuedTask = createMockTask({
				id: 'TASK-001',
				title: 'Implement caching layer',
				status: TaskStatus.PLANNED,
				category: TaskCategory.FEATURE,
			});
			mockTasks.push(queuedTask);

			renderAttentionDashboard();

			// Should show all key information compactly
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('Implement caching layer')).toBeInTheDocument();

			const taskCard = screen.getByText('TASK-001').closest('.task-card');
			expect(taskCard).toHaveClass('feature'); // category indicator
		});
	});

	// SC-5: Navigation
	describe('SC-5: Navigation', () => {
		it('navigates to Task Detail Page when clicking task in running section', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
			});
			mockTasks.push(runningTask);

			renderAttentionDashboard();

			const taskCard = screen.getByText('TASK-001').closest('.running-card');
			fireEvent.click(taskCard!);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});

		it('navigates to Task Detail Page when clicking task in queue section', () => {
			const queuedTask = createMockTask({
				id: 'TASK-002',
				status: TaskStatus.PLANNED,
			});
			mockTasks.push(queuedTask);

			renderAttentionDashboard();

			const taskCard = screen.getByText('TASK-002').closest('.task-card');
			fireEvent.click(taskCard!);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-002');
		});

		it('provides View action for attention items', () => {
			const decision = createMockDecision({
				id: 'DEC-001',
				taskId: 'TASK-001',
			});
			mockPendingDecisions.push(decision);

			renderAttentionDashboard();

			const viewButton = screen.getByRole('button', { name: /view/i });
			fireEvent.click(viewButton);

			expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001');
		});
	});

	// SC-6: Priority-based organization
	describe('SC-6: Priority-based organization', () => {
		it('sorts content by attention priority', () => {
			// Create items with different priority levels
			const criticalBlocked = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.BLOCKED,
				priority: TaskPriority.CRITICAL,
			});
			const highDecision = createMockDecision({
				id: 'DEC-001',
				taskId: 'TASK-002',
			});
			const normalQueued = createMockTask({
				id: 'TASK-003',
				status: TaskStatus.PLANNED,
				priority: TaskPriority.NORMAL,
			});

			mockBlockedTasks.push(criticalBlocked);
			mockPendingDecisions.push(highDecision);
			mockTasks.push(normalQueued);

			const { container } = renderAttentionDashboard();

			// Critical blocked task should appear first in attention section
			const attentionItems = container.querySelectorAll('.attention-section .attention-item');
			expect(attentionItems[0]).toContain('TASK-001');

			// Normal priority task should appear in queue section, not attention
			const queueSection = screen.getByRole('region', { name: /queue/i });
			expect(queueSection).toContain(screen.getByText('TASK-003'));
		});

		it('surfaces high-priority items prominently', () => {
			const highPriorityTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.PLANNED,
				priority: TaskPriority.HIGH,
			});
			mockTasks.push(highPriorityTask);

			renderAttentionDashboard();

			const taskCard = screen.getByText('TASK-001').closest('.task-card');
			expect(taskCard).toHaveClass('high-priority'); // visual prominence
		});
	});

	// SC-7: Real-time updates
	describe('SC-7: Real-time updates', () => {
		it('reflects live phase progression updates', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement',
			});
			mockTasks.push(runningTask);

			const { rerender } = renderAttentionDashboard();

			// Initially shows implement phase
			expect(screen.getByText('Code')).toBeInTheDocument();
			const codePhase = screen.getByText('Code').closest('.pipeline-step');
			expect(codePhase).toHaveClass('active');

			// Simulate phase change to review
			runningTask.currentPhase = 'review';
			rerender(
				<TooltipProvider>
					<MemoryRouter>
						<AttentionDashboard />
					</MemoryRouter>
				</TooltipProvider>
			);

			// Should now show review as active
			const reviewPhase = screen.getByText('Review').closest('.pipeline-step');
			expect(reviewPhase).toHaveClass('active');
		});

		it('updates timing information in real-time', () => {
			const runningTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
				startedAt: createTimestamp('2024-01-01T00:00:00Z'),
			});
			mockTasks.push(runningTask);

			vi.useFakeTimers();
			vi.setSystemTime(new Date('2024-01-01T00:05:00Z'));

			renderAttentionDashboard();

			// Should show initial time
			expect(screen.getByText('5:00')).toBeInTheDocument();

			// Advance time and trigger update
			vi.setSystemTime(new Date('2024-01-01T00:06:30Z'));
			vi.advanceTimersByTime(1000); // Trigger timer-based update

			// Should show updated time
			expect(screen.getByText('6:30')).toBeInTheDocument();

			vi.useRealTimers();
		});

		it('reflects dynamic status changes', () => {
			const task = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.PLANNED,
			});
			mockTasks.push(task);

			const { rerender } = renderAttentionDashboard();

			// Initially in queue section
			const queueSection = screen.getByRole('region', { name: /queue/i });
			expect(queueSection).toContain(screen.getByText('TASK-001'));

			// Simulate status change to running
			task.status = TaskStatus.RUNNING;
			task.currentPhase = 'implement';
			rerender(
				<TooltipProvider>
					<MemoryRouter>
						<AttentionDashboard />
					</MemoryRouter>
				</TooltipProvider>
			);

			// Should now appear in running section
			const runningSection = screen.getByRole('region', { name: /running/i });
			expect(runningSection).toContain(screen.getByText('TASK-001'));
		});
	});

	// SC-8: Responsive layout
	describe('SC-8: Responsive layout', () => {
		it('adapts to different screen sizes', () => {
			const { container } = renderAttentionDashboard();

			const dashboard = container.querySelector('.attention-dashboard');
			expect(dashboard).toHaveClass('responsive');

			// Should have responsive grid layout
			expect(dashboard).toHaveAttribute('style', expect.stringContaining('grid'));
		});

		it('supports collapsible panels and sections', () => {
			const queuedTask = createMockTask({
				id: 'TASK-001',
				status: TaskStatus.PLANNED,
				initiativeId: 'INIT-001',
			});
			mockTasks.push(queuedTask);

			renderAttentionDashboard();

			// Find swimlane header
			const swimlaneHeader = screen.getByText('INIT-001').closest('.swimlane-header');
			expect(swimlaneHeader).toBeInTheDocument();

			// Should be collapsible
			fireEvent.click(swimlaneHeader!);

			const swimlane = swimlaneHeader?.closest('.swimlane');
			expect(swimlane).toHaveClass('collapsed');
		});

		it('provides smooth animations and transitions', () => {
			const { container } = renderAttentionDashboard();

			// Check for animation classes
			const animatedElements = container.querySelectorAll('.animate-in, .transition-all');
			expect(animatedElements.length).toBeGreaterThan(0);
		});
	});

	// Integration tests
	describe('Integration tests', () => {
		it('correctly filters and distributes tasks across sections', () => {
			const runningTask = createMockTask({ id: 'R1', status: TaskStatus.RUNNING });
			const blockedTask = createMockTask({ id: 'B1', status: TaskStatus.BLOCKED });
			const queuedTask = createMockTask({ id: 'Q1', status: TaskStatus.PLANNED });
			const completedTask = createMockTask({ id: 'C1', status: TaskStatus.COMPLETED });

			mockTasks.push(runningTask, blockedTask, queuedTask, completedTask);
			mockBlockedTasks.push(blockedTask);

			renderAttentionDashboard();

			// Running section should have running task only
			const runningSection = screen.getByRole('region', { name: /running/i });
			expect(runningSection).toContain(screen.getByText('R1'));
			expect(runningSection).not.toContain(screen.getByText('B1'));

			// Attention section should have blocked task
			const attentionSection = screen.getByRole('region', { name: /needs attention/i });
			expect(attentionSection).toContain(screen.getByText('B1'));

			// Queue section should have planned task
			const queueSection = screen.getByRole('region', { name: /queue/i });
			expect(queueSection).toContain(screen.getByText('Q1'));

			// Completed task should not appear anywhere on dashboard
			expect(screen.queryByText('C1')).not.toBeInTheDocument();
		});
	});
});