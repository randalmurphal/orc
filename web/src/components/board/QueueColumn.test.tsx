import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueueColumn, type QueueColumnProps } from './QueueColumn';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import { createMockTask, createMockInitiative } from '@/test/factories';

function renderQueueColumn(props: Partial<QueueColumnProps> = {}) {
	const defaultProps: QueueColumnProps = {
		tasks: [],
		initiatives: [],
		...props,
	};
	return render(
		<TooltipProvider>
			<QueueColumn {...defaultProps} />
		</TooltipProvider>
	);
}

describe('QueueColumn', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('column header', () => {
		it('renders header with "Queue" title', () => {
			renderQueueColumn();
			expect(screen.getByText('Queue')).toBeInTheDocument();
		});

		it('displays correct task count in badge', () => {
			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1' }),
					createMockTask({ id: 'T2' }),
					createMockTask({ id: 'T3' }),
				],
			});

			// Find the count badge
			expect(screen.getByLabelText('3 tasks')).toHaveTextContent('3');
		});

		it('displays zero count when no tasks', () => {
			renderQueueColumn({ tasks: [] });
			expect(screen.getByLabelText('0 tasks')).toHaveTextContent('0');
		});

		it('has indicator dot in header', () => {
			const { container } = renderQueueColumn();
			const indicator = container.querySelector('.queue-column-indicator');
			expect(indicator).toBeInTheDocument();
		});
	});

	describe('task grouping', () => {
		it('groups tasks by initiativeId into separate Swimlane components', () => {
			const init1 = createMockInitiative({ id: 'INIT-001', title: 'Auth System' });
			const init2 = createMockInitiative({ id: 'INIT-002', title: 'Dashboard' });

			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-001' }),
					createMockTask({ id: 'T2', initiativeId: 'INIT-001' }),
					createMockTask({ id: 'T3', initiativeId: 'INIT-002' }),
				],
				initiatives: [init1, init2],
			});

			// Should render two swimlanes
			expect(screen.getByTestId('swimlane-INIT-001')).toBeInTheDocument();
			expect(screen.getByTestId('swimlane-INIT-002')).toBeInTheDocument();

			// Initiative titles should appear
			expect(screen.getByText('Auth System')).toBeInTheDocument();
			expect(screen.getByText('Dashboard')).toBeInTheDocument();
		});

		it('places tasks without initiativeId in "Unassigned" swimlane', () => {
			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: undefined }),
					createMockTask({ id: 'T2', initiativeId: undefined }),
				],
				initiatives: [],
			});

			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
			expect(screen.getByText('Unassigned')).toBeInTheDocument();
		});

		it('handles mix of assigned and unassigned tasks', () => {
			const init1 = createMockInitiative({ id: 'INIT-001', title: 'Feature Work' });

			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-001' }),
					createMockTask({ id: 'T2', initiativeId: undefined }),
				],
				initiatives: [init1],
			});

			expect(screen.getByTestId('swimlane-INIT-001')).toBeInTheDocument();
			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
		});
	});

	describe('swimlane sorting', () => {
		it('sorts active initiatives first, then by task count', () => {
			const activeInit = createMockInitiative({
				id: 'INIT-ACTIVE',
				title: 'Active Initiative',
				status: InitiativeStatus.ACTIVE,
			});
			const draftInit = createMockInitiative({
				id: 'INIT-DRAFT',
				title: 'Draft Initiative',
				status: InitiativeStatus.DRAFT,
			});

			const { container } = renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-DRAFT' }),
					createMockTask({ id: 'T2', initiativeId: 'INIT-DRAFT' }),
					createMockTask({ id: 'T3', initiativeId: 'INIT-DRAFT' }),
					createMockTask({ id: 'T4', initiativeId: 'INIT-ACTIVE' }),
				],
				initiatives: [draftInit, activeInit],
			});

			const swimlanes = container.querySelectorAll('.swimlane');
			// Active initiative should come first even though it has fewer tasks
			expect(swimlanes[0]).toHaveAttribute('data-testid', 'swimlane-INIT-ACTIVE');
			expect(swimlanes[1]).toHaveAttribute('data-testid', 'swimlane-INIT-DRAFT');
		});

		it('sorts by task count (descending) when status is the same', () => {
			const init1 = createMockInitiative({
				id: 'INIT-FEW',
				title: 'Few Tasks',
				status: InitiativeStatus.ACTIVE,
			});
			const init2 = createMockInitiative({
				id: 'INIT-MANY',
				title: 'Many Tasks',
				status: InitiativeStatus.ACTIVE,
			});

			const { container } = renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-FEW' }),
					createMockTask({ id: 'T2', initiativeId: 'INIT-MANY' }),
					createMockTask({ id: 'T3', initiativeId: 'INIT-MANY' }),
					createMockTask({ id: 'T4', initiativeId: 'INIT-MANY' }),
				],
				initiatives: [init1, init2],
			});

			const swimlanes = container.querySelectorAll('.swimlane');
			// Initiative with more tasks should come first
			expect(swimlanes[0]).toHaveAttribute('data-testid', 'swimlane-INIT-MANY');
			expect(swimlanes[1]).toHaveAttribute('data-testid', 'swimlane-INIT-FEW');
		});

		it('places "Unassigned" swimlane at the bottom', () => {
			const init = createMockInitiative({
				id: 'INIT-001',
				title: 'Some Initiative',
				status: InitiativeStatus.ACTIVE,
			});

			const { container } = renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: undefined }),
					createMockTask({ id: 'T2', initiativeId: undefined }),
					createMockTask({ id: 'T3', initiativeId: 'INIT-001' }),
				],
				initiatives: [init],
			});

			const swimlanes = container.querySelectorAll('.swimlane');
			// Unassigned should be last even though it has more tasks
			expect(swimlanes[swimlanes.length - 1]).toHaveAttribute('data-testid', 'swimlane-unassigned');
		});
	});

	describe('empty state', () => {
		it('shows "No queued tasks" when no tasks provided', () => {
			renderQueueColumn({ tasks: [], initiatives: [] });
			expect(screen.getByText('No queued tasks')).toBeInTheDocument();
		});

		it('has empty state styling class', () => {
			const { container } = renderQueueColumn({ tasks: [] });
			const emptyState = container.querySelector('.queue-column-empty');
			expect(emptyState).toBeInTheDocument();
		});
	});

	describe('task click handling', () => {
		it('passes onTaskClick through to Swimlane components', () => {
			const onTaskClick = vi.fn();
			const task = createMockTask({ id: 'TASK-001', initiativeId: undefined });

			const { container } = renderQueueColumn({
				tasks: [task],
				initiatives: [],
				onTaskClick,
			});

			const taskCard = container.querySelector('.task-card');
			fireEvent.click(taskCard!);

			expect(onTaskClick).toHaveBeenCalledTimes(1);
			expect(onTaskClick).toHaveBeenCalledWith(task);
		});

		it('works with tasks in different swimlanes', () => {
			const onTaskClick = vi.fn();
			const init = createMockInitiative({ id: 'INIT-001' });
			const task1 = createMockTask({ id: 'T1', initiativeId: 'INIT-001' });
			const task2 = createMockTask({ id: 'T2', initiativeId: undefined });

			const { container } = renderQueueColumn({
				tasks: [task1, task2],
				initiatives: [init],
				onTaskClick,
			});

			const taskCards = container.querySelectorAll('.task-card');
			fireEvent.click(taskCards[0]);
			fireEvent.click(taskCards[1]);

			expect(onTaskClick).toHaveBeenCalledTimes(2);
		});
	});

	describe('context menu handling', () => {
		it('passes onContextMenu through to Swimlane components', () => {
			const onContextMenu = vi.fn();
			const task = createMockTask({ id: 'TASK-001', initiativeId: undefined });

			const { container } = renderQueueColumn({
				tasks: [task],
				initiatives: [],
				onContextMenu,
			});

			const taskCard = container.querySelector('.task-card');
			fireEvent.contextMenu(taskCard!);

			expect(onContextMenu).toHaveBeenCalledTimes(1);
			expect(onContextMenu).toHaveBeenCalledWith(task, expect.any(Object));
		});
	});

	describe('collapse state management', () => {
		it('calls onToggleSwimlane when swimlane header clicked', () => {
			const onToggleSwimlane = vi.fn();
			const init = createMockInitiative({ id: 'INIT-001', title: 'Test' });

			const { container } = renderQueueColumn({
				tasks: [createMockTask({ id: 'T1', initiativeId: 'INIT-001' })],
				initiatives: [init],
				onToggleSwimlane,
			});

			const header = container.querySelector('.swimlane-header');
			fireEvent.click(header!);

			expect(onToggleSwimlane).toHaveBeenCalledTimes(1);
			expect(onToggleSwimlane).toHaveBeenCalledWith('INIT-001');
		});

		it('calls onToggleSwimlane with "unassigned" for unassigned swimlane', () => {
			const onToggleSwimlane = vi.fn();

			const { container } = renderQueueColumn({
				tasks: [createMockTask({ id: 'T1', initiativeId: undefined })],
				initiatives: [],
				onToggleSwimlane,
			});

			const header = container.querySelector('.swimlane-header');
			fireEvent.click(header!);

			expect(onToggleSwimlane).toHaveBeenCalledWith('unassigned');
		});

		it('applies collapsed state from collapsedSwimlanes prop', () => {
			const init = createMockInitiative({ id: 'INIT-001', title: 'Test' });
			const collapsedSwimlanes = new Set(['INIT-001']);

			const { container } = renderQueueColumn({
				tasks: [createMockTask({ id: 'T1', initiativeId: 'INIT-001' })],
				initiatives: [init],
				collapsedSwimlanes,
			});

			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).toHaveClass('collapsed');
		});

		it('does not collapse swimlanes not in collapsedSwimlanes set', () => {
			const init = createMockInitiative({ id: 'INIT-001', title: 'Test' });
			const collapsedSwimlanes = new Set(['INIT-OTHER']);

			const { container } = renderQueueColumn({
				tasks: [createMockTask({ id: 'T1', initiativeId: 'INIT-001' })],
				initiatives: [init],
				collapsedSwimlanes,
			});

			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).not.toHaveClass('collapsed');
		});
	});

	describe('accessibility', () => {
		it('has role="region" with aria-label', () => {
			const { container } = renderQueueColumn();

			const column = container.querySelector('.queue-column');
			expect(column).toHaveAttribute('role', 'region');
			expect(column).toHaveAttribute('aria-label', 'Queue column');
		});

		it('count badge has aria-label for screen readers', () => {
			renderQueueColumn({
				tasks: [createMockTask({ id: 'T1' }), createMockTask({ id: 'T2' })],
			});

			const countBadge = screen.getByLabelText('2 tasks');
			expect(countBadge).toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('handles tasks with initiativeId that does not exist in initiatives array', () => {
			// Tasks pointing to non-existent initiative should go to unassigned
			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-NONEXISTENT' }),
				],
				initiatives: [],
			});

			// Should fall back to unassigned
			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
		});

		it('handles empty initiatives array with assigned tasks', () => {
			renderQueueColumn({
				tasks: [
					createMockTask({ id: 'T1', initiativeId: 'INIT-001' }),
				],
				initiatives: [],
			});

			// Task should go to unassigned since initiative not found
			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
		});

		it('handles undefined collapsedSwimlanes prop', () => {
			const init = createMockInitiative({ id: 'INIT-001' });

			const { container } = renderQueueColumn({
				tasks: [createMockTask({ id: 'T1', initiativeId: 'INIT-001' })],
				initiatives: [init],
				collapsedSwimlanes: undefined,
			});

			// Should render without error, swimlanes not collapsed
			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).not.toHaveClass('collapsed');
		});
	});
});
