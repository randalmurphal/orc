import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TaskHeader } from './TaskHeader';
import { TooltipProvider } from '@/components/ui/Tooltip';
import type { Task } from '@/lib/types';

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
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

describe('TaskHeader', () => {
	const createTask = (overrides: Partial<Task> = {}): Task => ({
		id: 'TASK-001',
		title: 'Test Task',
		description: 'Test description',
		status: 'created',
		weight: 'small',
		priority: 'normal',
		category: 'feature',
		queue: 'active',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides,
	});

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
		it('renders initiative badge when task.initiative_id is set', () => {
			renderTaskHeader({
				task: createTask({ initiative_id: 'INIT-001' }),
			});

			const badge = screen.getByRole('button', { name: /test init/i });
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('initiative-badge');
		});

		it('hides initiative badge when task.initiative_id is null', () => {
			renderTaskHeader({
				task: createTask({ initiative_id: null }),
			});

			expect(screen.queryByText(/test init/i)).not.toBeInTheDocument();
		});

		it('hides initiative badge when task.initiative_id is undefined', () => {
			renderTaskHeader({
				task: createTask({ initiative_id: undefined }),
			});

			expect(screen.queryByText(/test init/i)).not.toBeInTheDocument();
		});

		it('navigates to initiative detail page when clicked', () => {
			renderTaskHeader({
				task: createTask({ initiative_id: 'INIT-001' }),
			});

			const badge = screen.getByRole('button', { name: /test init/i });
			fireEvent.click(badge);

			expect(mockNavigate).toHaveBeenCalledWith('/initiatives/INIT-001');
		});
	});

	describe('Priority Badge', () => {
		it('renders priority badge for critical priority', () => {
			renderTaskHeader({
				task: createTask({ priority: 'critical' }),
			});

			const badge = screen.getByText('Critical');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-critical');
		});

		it('renders priority badge for high priority', () => {
			renderTaskHeader({
				task: createTask({ priority: 'high' }),
			});

			const badge = screen.getByText('High');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-high');
		});

		it('renders priority badge for low priority', () => {
			renderTaskHeader({
				task: createTask({ priority: 'low' }),
			});

			const badge = screen.getByText('Low');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-low');
		});

		it('renders priority badge for normal priority with subtle styling', () => {
			renderTaskHeader({
				task: createTask({ priority: 'normal' }),
			});

			const badge = screen.getByText('Normal');
			expect(badge).toBeInTheDocument();
			expect(badge).toHaveClass('priority-badge', 'priority-normal');
		});

		it('defaults to normal priority when priority is not set', () => {
			renderTaskHeader({
				task: createTask({ priority: undefined }),
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
					initiative_id: 'INIT-001',
					priority: 'high',
					category: 'bug',
					weight: 'medium',
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
