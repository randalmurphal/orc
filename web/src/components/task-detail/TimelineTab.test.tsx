import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TimelineTab } from './TimelineTab';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task, TaskState, Plan } from '@/lib/types';

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
	const createTask = (overrides: Partial<Task> = {}): Task => ({
		id: 'TASK-001',
		title: 'Test Task',
		description: 'Test description',
		status: 'created',
		weight: 'small',
		branch: 'orc/TASK-001',
		priority: 'normal',
		category: 'feature',
		queue: 'active',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T12:00:00Z',
		...overrides,
	});

	const createPlan = (overrides: Partial<Plan> = {}): Plan => ({
		version: 1,
		weight: 'small',
		description: 'Test plan',
		phases: [
			{
				id: 'phase-1',
				name: 'implement',
				status: 'pending',
				iterations: 1,
			},
			{
				id: 'phase-2',
				name: 'test',
				status: 'pending',
				iterations: 1,
			},
		],
		...overrides,
	});

	const createTaskState = (overrides: Partial<TaskState> = {}): TaskState => ({
		task_id: 'TASK-001',
		current_phase: 'implement',
		current_iteration: 1,
		status: 'running',
		started_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		phases: {},
		gates: [],
		tokens: {
			input_tokens: 1000,
			output_tokens: 500,
			total_tokens: 1500,
		},
		...overrides,
	});

	beforeEach(() => {
		vi.clearAllMocks();
	});

	const renderTimelineTab = (props: {
		task?: Task;
		taskState?: TaskState | null;
		plan?: Plan | null;
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
				task: createTask({ priority: 'critical' }),
			});

			const priority = screen.getByText('Critical');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-critical');
		});

		it('renders priority field with correct styling for high', () => {
			renderTimelineTab({
				task: createTask({ priority: 'high' }),
			});

			const priority = screen.getByText('High');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-high');
		});

		it('renders priority field with correct styling for normal', () => {
			renderTimelineTab({
				task: createTask({ priority: 'normal' }),
			});

			const priority = screen.getByText('Normal');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-normal');
		});

		it('renders priority field with correct styling for low', () => {
			renderTimelineTab({
				task: createTask({ priority: 'low' }),
			});

			const priority = screen.getByText('Low');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-low');
		});

		it('defaults to normal priority when not set', () => {
			renderTimelineTab({
				task: createTask({ priority: undefined }),
			});

			const priority = screen.getByText('Normal');
			expect(priority).toBeInTheDocument();
			expect(priority).toHaveClass('info-priority', 'priority-normal');
		});
	});

	describe('Task Info - Category', () => {
		it('renders category field with icon for feature', () => {
			const { container } = renderTimelineTab({
				task: createTask({ category: 'feature' }),
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
				task: createTask({ category: 'bug' }),
			});

			expect(screen.getByText('Bug')).toBeInTheDocument();
		});

		it('renders category field with icon for refactor', () => {
			renderTimelineTab({
				task: createTask({ category: 'refactor' }),
			});

			expect(screen.getByText('Refactor')).toBeInTheDocument();
		});

		it('renders category field with icon for chore', () => {
			renderTimelineTab({
				task: createTask({ category: 'chore' }),
			});

			expect(screen.getByText('Chore')).toBeInTheDocument();
		});

		it('renders category field with icon for docs', () => {
			renderTimelineTab({
				task: createTask({ category: 'docs' }),
			});

			expect(screen.getByText('Docs')).toBeInTheDocument();
		});

		it('renders category field with icon for test', () => {
			renderTimelineTab({
				task: createTask({ category: 'test' }),
			});

			expect(screen.getByText('Test')).toBeInTheDocument();
		});

		it('hides category field when not set', () => {
			renderTimelineTab({
				task: createTask({ category: undefined }),
			});

			expect(screen.queryByText('Feature')).not.toBeInTheDocument();
			expect(screen.queryByText('Bug')).not.toBeInTheDocument();
		});
	});

	describe('Task Info - Initiative', () => {
		it('renders initiative as clickable link when set', () => {
			renderTimelineTab({
				task: createTask({ initiative_id: 'INIT-001' }),
			});

			const link = screen.getByRole('link', { name: /test init/i });
			expect(link).toBeInTheDocument();
			expect(link).toHaveAttribute('href', '/initiatives/INIT-001');
			expect(link).toHaveClass('info-initiative-link');
		});

		it('hides initiative field when not set', () => {
			renderTimelineTab({
				task: createTask({ initiative_id: undefined }),
			});

			expect(screen.queryByRole('link', { name: /test init/i })).not.toBeInTheDocument();
		});

		it('hides initiative field when getInitiativeBadgeTitle returns null', () => {
			renderTimelineTab({
				task: createTask({ initiative_id: 'INIT-UNKNOWN' }),
			});

			expect(screen.queryByText('INIT-UNKNOWN')).not.toBeInTheDocument();
		});
	});

	describe('Task Info - Blocked By', () => {
		it('renders blocked_by count when blockers exist', () => {
			renderTimelineTab({
				task: createTask({ blocked_by: ['TASK-002', 'TASK-003'] }),
			});

			expect(screen.getByText('2 tasks')).toBeInTheDocument();
		});

		it('renders singular form for single blocker', () => {
			renderTimelineTab({
				task: createTask({ blocked_by: ['TASK-002'] }),
			});

			expect(screen.getByText('1 task')).toBeInTheDocument();
		});

		it('hides blocked_by field when empty array', () => {
			renderTimelineTab({
				task: createTask({ blocked_by: [] }),
			});

			// The label 'Blocked By' should not appear
			expect(screen.queryByText('Blocked By')).not.toBeInTheDocument();
		});

		it('hides blocked_by field when undefined', () => {
			renderTimelineTab({
				task: createTask({ blocked_by: undefined }),
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
				task: createTask({ target_branch: 'main' }),
			});

			expect(screen.getByText('main')).toBeInTheDocument();
		});

		it('hides target branch when not set', () => {
			renderTimelineTab({
				task: createTask({ target_branch: undefined }),
			});

			expect(screen.queryByText('Target')).not.toBeInTheDocument();
		});

		it('shows updated timestamp', () => {
			renderTimelineTab({
				task: createTask({ updated_at: '2024-06-15T10:30:00Z' }),
			});

			// Look for the Updated label
			expect(screen.getByText('Updated')).toBeInTheDocument();
		});

		it('shows queue field', () => {
			renderTimelineTab({
				task: createTask({ queue: 'backlog' }),
			});

			expect(screen.getByText('backlog')).toBeInTheDocument();
		});

		it('defaults queue to active when not set', () => {
			renderTimelineTab({
				task: createTask({ queue: undefined }),
			});

			expect(screen.getByText('active')).toBeInTheDocument();
		});
	});

	describe('Task Info - Execution Info', () => {
		it('shows current phase when running', () => {
			renderTimelineTab({
				task: createTask({ status: 'running' }),
				taskState: createTaskState({ current_phase: 'implement' }),
			});

			// The phase name should appear in the phase section
			expect(screen.getAllByText('implement').length).toBeGreaterThan(0);
		});

		it('shows retries when greater than zero', () => {
			renderTimelineTab({
				task: createTask(),
				taskState: createTaskState({ retries: 3 }),
			});

			expect(screen.getByText('3')).toBeInTheDocument();
			expect(screen.getByText('Retries')).toBeInTheDocument();
		});

		it('hides retries when zero', () => {
			renderTimelineTab({
				task: createTask(),
				taskState: createTaskState({ retries: 0 }),
			});

			expect(screen.queryByText('Retries')).not.toBeInTheDocument();
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

	/**
	 * Zero Timestamp Tests (SC-1, SC-2)
	 *
	 * These tests verify that Go's zero time value (0001-01-01T00:00:00Z) displays
	 * as "Never" in the Updated field instead of "Dec 31, 1" which is the incorrect
	 * timezone-shifted display.
	 *
	 * TDD Note: These tests are written BEFORE the fix is implemented.
	 */
	describe('Task Info - Updated field with zero timestamp (SC-1)', () => {
		it('displays "Never" when updated_at is Go zero time (0001-01-01T00:00:00Z)', () => {
			// Go's zero time value - this is what the backend sends when a task
			// has never been modified after creation
			renderTimelineTab({
				task: createTask({ updated_at: '0001-01-01T00:00:00Z' }),
			});

			// The Updated field should show "Never" instead of "Dec 31, 1, 06:09 PM"
			const updatedLabel = screen.getByText('Updated');
			expect(updatedLabel).toBeInTheDocument();

			// Find the value next to the "Updated" label
			const infoItem = updatedLabel.closest('.info-item');
			expect(infoItem).toBeInTheDocument();

			const valueElement = infoItem?.querySelector('dd');
			expect(valueElement).toBeInTheDocument();
			expect(valueElement?.textContent).toBe('Never');
		});

		it('displays "Never" when updated_at is zero time with milliseconds', () => {
			renderTimelineTab({
				task: createTask({ updated_at: '0001-01-01T00:00:00.000Z' }),
			});

			const updatedLabel = screen.getByText('Updated');
			const infoItem = updatedLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');
			expect(valueElement?.textContent).toBe('Never');
		});

		it('displays "Never" for any year 1 date (edge case: 0001-12-31T23:59:59Z)', () => {
			// Any date in year 1 should be treated as "never set"
			renderTimelineTab({
				task: createTask({ updated_at: '0001-12-31T23:59:59Z' }),
			});

			const updatedLabel = screen.getByText('Updated');
			const infoItem = updatedLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');
			expect(valueElement?.textContent).toBe('Never');
		});
	});

	describe('Task Info - Updated field with valid timestamps (SC-2)', () => {
		it('displays formatted date for valid updated_at timestamp', () => {
			renderTimelineTab({
				task: createTask({ updated_at: '2024-06-15T10:30:00Z' }),
			});

			const updatedLabel = screen.getByText('Updated');
			const infoItem = updatedLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');

			// Should display a formatted date, not "Never" or empty
			expect(valueElement?.textContent).not.toBe('Never');
			expect(valueElement?.textContent).not.toBe('');
			// The exact format depends on locale, but should contain "Jun" or similar
			expect(valueElement?.textContent).toMatch(/\w{3}\s+\d{1,2}/); // e.g., "Jun 15"
		});

		it('displays formatted date for Unix epoch (1970-01-01T00:00:00Z)', () => {
			// Unix epoch is a valid date and should NOT be treated as zero time
			renderTimelineTab({
				task: createTask({ updated_at: '1970-01-01T00:00:00Z' }),
			});

			const updatedLabel = screen.getByText('Updated');
			const infoItem = updatedLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');

			// Should display a formatted date, not "Never"
			expect(valueElement?.textContent).not.toBe('Never');
			expect(valueElement?.textContent).not.toBe('');
		});

		it('displays formatted date for dates from year 2 and beyond', () => {
			// Year 2 is valid and should be formatted normally
			renderTimelineTab({
				task: createTask({ updated_at: '0002-01-01T00:00:00Z' }),
			});

			const updatedLabel = screen.getByText('Updated');
			const infoItem = updatedLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');

			// Should display a formatted date, not "Never"
			expect(valueElement?.textContent).not.toBe('Never');
		});
	});

	describe('Task Info - created_at timestamp formatting', () => {
		it('displays formatted date for valid created_at timestamp', () => {
			renderTimelineTab({
				task: createTask({ created_at: '2024-06-15T10:30:00Z' }),
			});

			const createdLabel = screen.getByText('Created');
			const infoItem = createdLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');

			// Should display a formatted date
			expect(valueElement?.textContent).not.toBe('');
			expect(valueElement?.textContent).toMatch(/\w{3}\s+\d{1,2}/);
		});

		it('displays "Never" for zero created_at timestamp', () => {
			// While unlikely, created_at could also be zero time
			renderTimelineTab({
				task: createTask({ created_at: '0001-01-01T00:00:00Z' }),
			});

			const createdLabel = screen.getByText('Created');
			const infoItem = createdLabel.closest('.info-item');
			const valueElement = infoItem?.querySelector('dd');

			expect(valueElement?.textContent).toBe('Never');
		});
	});
});
