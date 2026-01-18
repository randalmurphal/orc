import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TaskCard } from './TaskCard';
import type { Task } from '@/lib/types';

// Sample task for testing
const createTask = (overrides: Partial<Task> = {}): Task => ({
	id: 'TASK-001',
	title: 'Test Task',
	description: 'A test task description',
	weight: 'medium',
	status: 'created',
	category: 'feature',
	priority: 'normal',
	branch: 'orc/TASK-001',
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	...overrides,
});

function renderTaskCard(task: Task, props: Partial<Parameters<typeof TaskCard>[0]> = {}) {
	return render(<TaskCard task={task} {...props} />);
}

describe('TaskCard', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('renders task ID, title, priority dot, and category icon', () => {
			const { container } = renderTaskCard(createTask());

			// Task ID
			expect(screen.getByText('TASK-001')).toBeInTheDocument();

			// Title
			expect(screen.getByText('Test Task')).toBeInTheDocument();

			// Priority dot
			const priorityDot = container.querySelector('.task-card-priority');
			expect(priorityDot).toBeInTheDocument();

			// Category icon
			const categoryIcon = container.querySelector('.task-card-category');
			expect(categoryIcon).toBeInTheDocument();
		});

		it('truncates long titles with ellipsis at 2 lines max via CSS class', () => {
			const longTitle =
				'This is a very long task title that should be truncated when it exceeds two lines of text in the compact card display format';
			const { container } = renderTaskCard(createTask({ title: longTitle }));

			const titleElement = container.querySelector('.task-card-title');
			expect(titleElement).toBeInTheDocument();
			expect(titleElement).toHaveClass('task-card-title');
			// Verify the title content is there (CSS handles truncation)
			expect(titleElement?.textContent).toBe(longTitle);
		});

		it('renders correct category icon based on task category', () => {
			const { container: featureContainer } = renderTaskCard(createTask({ category: 'feature' }));
			expect(featureContainer.querySelector('.task-card-category')).toBeInTheDocument();

			const { container: bugContainer } = renderTaskCard(createTask({ category: 'bug' }));
			expect(bugContainer.querySelector('.task-card-category')).toBeInTheDocument();
		});

		it('renders different priority dot colors', () => {
			const { container: criticalContainer } = renderTaskCard(
				createTask({ priority: 'critical' })
			);
			const criticalDot = criticalContainer.querySelector('.task-card-priority');
			expect(criticalDot).toHaveStyle({ backgroundColor: 'var(--red)' });

			const { container: highContainer } = renderTaskCard(createTask({ priority: 'high' }));
			const highDot = highContainer.querySelector('.task-card-priority');
			expect(highDot).toHaveStyle({ backgroundColor: 'var(--orange)' });

			const { container: normalContainer } = renderTaskCard(createTask({ priority: 'normal' }));
			const normalDot = normalContainer.querySelector('.task-card-priority');
			expect(normalDot).toHaveStyle({ backgroundColor: 'var(--blue)' });

			const { container: lowContainer } = renderTaskCard(createTask({ priority: 'low' }));
			const lowDot = lowContainer.querySelector('.task-card-priority');
			expect(lowDot).toHaveStyle({ backgroundColor: 'var(--text-muted)' });
		});
	});

	describe('state classes', () => {
		it('applies hover and selected state classes correctly', () => {
			const { container } = renderTaskCard(createTask(), { isSelected: true });

			const card = container.querySelector('.task-card');
			expect(card).toHaveClass('selected');
		});

		it('has running class when task is running', () => {
			const { container } = renderTaskCard(
				createTask({ status: 'running', current_phase: 'implement' })
			);

			expect(container.querySelector('.task-card')).toHaveClass('running');
		});

		it('has blocked class when task is blocked', () => {
			const { container } = renderTaskCard(
				createTask({ is_blocked: true, unmet_blockers: ['TASK-002'] })
			);

			expect(container.querySelector('.task-card')).toHaveClass('blocked');
		});
	});

	describe('click handler', () => {
		it('calls onClick handler when clicked', () => {
			const onClick = vi.fn();
			const { container } = renderTaskCard(createTask(), { onClick });

			const card = container.querySelector('.task-card')!;
			fireEvent.click(card);

			expect(onClick).toHaveBeenCalledTimes(1);
		});

		it('does not crash when onClick is not provided', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card')!;
			expect(() => fireEvent.click(card)).not.toThrow();
		});
	});

	describe('context menu handler', () => {
		it('calls onContextMenu handler on right-click', () => {
			const onContextMenu = vi.fn();
			const { container } = renderTaskCard(createTask(), { onContextMenu });

			const card = container.querySelector('.task-card')!;
			fireEvent.contextMenu(card);

			expect(onContextMenu).toHaveBeenCalledTimes(1);
			expect(onContextMenu).toHaveBeenCalledWith(expect.any(Object));
		});

		it('does not crash when onContextMenu is not provided', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card')!;
			expect(() => fireEvent.contextMenu(card)).not.toThrow();
		});
	});

	describe('keyboard navigation', () => {
		it('triggers onClick on Enter key', () => {
			const onClick = vi.fn();
			const { container } = renderTaskCard(createTask(), { onClick });

			const card = container.querySelector('.task-card')!;
			fireEvent.keyDown(card, { key: 'Enter' });

			expect(onClick).toHaveBeenCalledTimes(1);
		});

		it('triggers onClick on Space key', () => {
			const onClick = vi.fn();
			const { container } = renderTaskCard(createTask(), { onClick });

			const card = container.querySelector('.task-card')!;
			fireEvent.keyDown(card, { key: ' ' });

			expect(onClick).toHaveBeenCalledTimes(1);
		});

		it('does not trigger onClick on other keys', () => {
			const onClick = vi.fn();
			const { container } = renderTaskCard(createTask(), { onClick });

			const card = container.querySelector('.task-card')!;
			fireEvent.keyDown(card, { key: 'Escape' });

			expect(onClick).not.toHaveBeenCalled();
		});
	});

	describe('aria-label', () => {
		it('has correct aria-label format', () => {
			renderTaskCard(createTask({ priority: 'high', category: 'feature' }));

			const card = screen.getByRole('button');
			expect(card).toHaveAttribute(
				'aria-label',
				'TASK-001: Test Task, high priority, feature'
			);
		});

		it('includes blocked in aria-label when task is blocked', () => {
			renderTaskCard(createTask({ is_blocked: true, unmet_blockers: ['TASK-002'] }));

			const card = screen.getByRole('button');
			expect(card.getAttribute('aria-label')).toContain('blocked');
		});

		it('includes running in aria-label when task is running', () => {
			renderTaskCard(createTask({ status: 'running' }));

			const card = screen.getByRole('button');
			expect(card.getAttribute('aria-label')).toContain('running');
		});
	});

	describe('blocked tasks', () => {
		it('shows warning icon for blocked tasks', () => {
			const { container } = renderTaskCard(
				createTask({ is_blocked: true, unmet_blockers: ['TASK-002', 'TASK-003'] })
			);

			const blockedIcon = container.querySelector('.task-card-blocked');
			expect(blockedIcon).toBeInTheDocument();
		});

		it('does not show warning icon when not blocked', () => {
			const { container } = renderTaskCard(createTask({ is_blocked: false }));

			const blockedIcon = container.querySelector('.task-card-blocked');
			expect(blockedIcon).not.toBeInTheDocument();
		});
	});

	describe('running tasks', () => {
		it('shows mini progress indicator for running tasks', () => {
			const { container } = renderTaskCard(
				createTask({ status: 'running', current_phase: 'implement' })
			);

			const runningIndicator = container.querySelector('.task-card-running');
			expect(runningIndicator).toBeInTheDocument();

			const runningDot = container.querySelector('.task-card-running-dot');
			expect(runningDot).toBeInTheDocument();
		});

		it('does not show running indicator when not running', () => {
			const { container } = renderTaskCard(createTask({ status: 'created' }));

			const runningIndicator = container.querySelector('.task-card-running');
			expect(runningIndicator).not.toBeInTheDocument();
		});
	});

	describe('missing initiative', () => {
		it('handles missing initiative gracefully (no crash)', () => {
			expect(() => {
				renderTaskCard(createTask({ initiative_id: undefined }));
			}).not.toThrow();
		});

		it('shows initiative badge when showInitiative is true and initiative exists', () => {
			const { container } = renderTaskCard(
				createTask({ initiative_id: 'INIT-001' }),
				{ showInitiative: true }
			);

			expect(screen.getByText('INIT-001')).toBeInTheDocument();
			const initiativeBadge = container.querySelector('.task-card-initiative');
			expect(initiativeBadge).toBeInTheDocument();
		});

		it('does not show initiative badge when showInitiative is false', () => {
			const { container } = renderTaskCard(
				createTask({ initiative_id: 'INIT-001' }),
				{ showInitiative: false }
			);

			const initiativeBadge = container.querySelector('.task-card-initiative');
			expect(initiativeBadge).not.toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has role="button"', () => {
			renderTaskCard(createTask());

			const card = screen.getByRole('button');
			expect(card).toBeInTheDocument();
		});

		it('has tabIndex=0 for keyboard focus', () => {
			const { container } = renderTaskCard(createTask());

			const card = container.querySelector('.task-card');
			expect(card).toHaveAttribute('tabindex', '0');
		});
	});

	describe('className prop', () => {
		it('applies additional className', () => {
			const { container } = renderTaskCard(createTask(), { className: 'custom-class' });

			const card = container.querySelector('.task-card');
			expect(card).toHaveClass('custom-class');
		});
	});
});
