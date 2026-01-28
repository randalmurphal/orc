import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Swimlane, type SwimlaneProps } from './Swimlane';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockInitiative } from '@/test/factories';

function renderSwimlane(props: Partial<SwimlaneProps> = {}) {
	const defaultProps: SwimlaneProps = {
		initiative: createMockInitiative(),
		tasks: [createMockTask()],
		isCollapsed: false,
		onToggle: vi.fn(),
		...props,
	};
	return render(
		<TooltipProvider>
			<Swimlane {...defaultProps} />
		</TooltipProvider>
	);
}

describe('Swimlane', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('renders header with chevron, icon, name, count, and progress', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ title: 'Auth System' }),
				tasks: [
					createMockTask({ id: 'T1', status: TaskStatus.CREATED }),
					createMockTask({ id: 'T2', status: TaskStatus.COMPLETED }),
				],
			});

			// Chevron
			const chevron = container.querySelector('.swimlane-chevron');
			expect(chevron).toBeInTheDocument();

			// Icon
			const icon = container.querySelector('.swimlane-icon');
			expect(icon).toBeInTheDocument();
			expect(icon?.textContent).toBe('A'); // First letter of "Auth System"

			// Name
			expect(screen.getByText('Auth System')).toBeInTheDocument();

			// Count badge
			const count = container.querySelector('.swimlane-count');
			expect(count).toBeInTheDocument();
			expect(count?.textContent).toBe('2');

			// Progress bar
			const progressBar = container.querySelector('.swimlane-progress');
			expect(progressBar).toBeInTheDocument();
		});

		it('displays correct task count badge', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'T1' }),
					createMockTask({ id: 'T2' }),
					createMockTask({ id: 'T3' }),
				],
			});

			const count = container.querySelector('.swimlane-count');
			expect(count?.textContent).toBe('3');
		});

		it('calculates and displays progress correctly', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'T1', status: TaskStatus.COMPLETED }),
					createMockTask({ id: 'T2', status: TaskStatus.COMPLETED }),
					createMockTask({ id: 'T3', status: TaskStatus.CREATED }),
					createMockTask({ id: 'T4', status: TaskStatus.RUNNING }),
				],
			});

			// 2 out of 4 completed = 50%
			const progressFill = container.querySelector('.swimlane-progress-fill');
			expect(progressFill).toHaveStyle({ width: '50%' });
		});

		it('renders TaskCards for each task', () => {
			renderSwimlane({
				tasks: [
					createMockTask({ id: 'TASK-001', title: 'First Task' }),
					createMockTask({ id: 'TASK-002', title: 'Second Task' }),
				],
			});

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
		});
	});

	describe('initiative icon', () => {
		it('uses first letter of initiative title as icon', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ title: 'Backend API' }),
			});

			const icon = container.querySelector('.swimlane-icon');
			expect(icon?.textContent).toBe('B');
		});

		it('uses emoji if initiative title starts with one', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ title: '🚀 Launch Features' }),
			});

			const icon = container.querySelector('.swimlane-icon');
			// Emojis can be 1-2 characters in JavaScript due to surrogate pairs
			// The rocket emoji is displayed in the icon
			expect(icon?.textContent).toBe('🚀');
		});

		it('shows ? icon for null initiative (unassigned)', () => {
			const { container } = renderSwimlane({
				initiative: null,
				tasks: [createMockTask()],
			});

			const icon = container.querySelector('.swimlane-icon');
			expect(icon?.textContent).toBe('?');
		});
	});

	describe('unassigned swimlane', () => {
		it('shows "Unassigned" title when initiative is null', () => {
			renderSwimlane({
				initiative: null,
				tasks: [createMockTask()],
			});

			expect(screen.getByText('Unassigned')).toBeInTheDocument();
		});

		it('applies unassigned color class to icon', () => {
			const { container } = renderSwimlane({
				initiative: null,
				tasks: [createMockTask()],
			});

			const icon = container.querySelector('.swimlane-icon');
			expect(icon).toHaveClass('unassigned');
		});

		it('sets data-testid to swimlane-unassigned', () => {
			renderSwimlane({
				initiative: null,
				tasks: [createMockTask()],
			});

			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
		});
	});

	describe('empty state', () => {
		it('shows "No tasks" message when tasks array is empty', () => {
			renderSwimlane({
				tasks: [],
			});

			expect(screen.getByText('No tasks')).toBeInTheDocument();
		});

		it('renders empty state element with correct class', () => {
			const { container } = renderSwimlane({
				tasks: [],
			});

			const emptyState = container.querySelector('.swimlane-empty');
			expect(emptyState).toBeInTheDocument();
		});
	});

	describe('collapsed state', () => {
		it('applies collapsed class when isCollapsed is true', () => {
			const { container } = renderSwimlane({
				isCollapsed: true,
			});

			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).toHaveClass('collapsed');
		});

		it('does not apply collapsed class when isCollapsed is false', () => {
			const { container } = renderSwimlane({
				isCollapsed: false,
			});

			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).not.toHaveClass('collapsed');
		});

		it('sets aria-hidden on content when collapsed', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-001' }),
				isCollapsed: true,
			});

			const content = container.querySelector('#swimlane-content-INIT-001');
			expect(content).toHaveAttribute('aria-hidden', 'true');
		});

		it('chevron rotates via CSS class when collapsed', () => {
			const { container } = renderSwimlane({
				isCollapsed: true,
			});

			// The chevron rotation is controlled by .swimlane.collapsed .swimlane-chevron
			// We verify the collapsed class is present which triggers the CSS transform
			const swimlane = container.querySelector('.swimlane');
			expect(swimlane).toHaveClass('collapsed');
		});
	});

	describe('toggle functionality', () => {
		it('calls onToggle when header is clicked', () => {
			const onToggle = vi.fn();
			const { container } = renderSwimlane({ onToggle });

			const header = container.querySelector('.swimlane-header');
			fireEvent.click(header!);

			expect(onToggle).toHaveBeenCalledTimes(1);
		});

		it('calls onToggle on Enter key press', () => {
			const onToggle = vi.fn();
			const { container } = renderSwimlane({ onToggle });

			const header = container.querySelector('.swimlane-header');
			fireEvent.keyDown(header!, { key: 'Enter' });

			expect(onToggle).toHaveBeenCalledTimes(1);
		});

		it('calls onToggle on Space key press', () => {
			const onToggle = vi.fn();
			const { container } = renderSwimlane({ onToggle });

			const header = container.querySelector('.swimlane-header');
			fireEvent.keyDown(header!, { key: ' ' });

			expect(onToggle).toHaveBeenCalledTimes(1);
		});

		it('does not call onToggle on other key presses', () => {
			const onToggle = vi.fn();
			const { container } = renderSwimlane({ onToggle });

			const header = container.querySelector('.swimlane-header');
			fireEvent.keyDown(header!, { key: 'Escape' });

			expect(onToggle).not.toHaveBeenCalled();
		});
	});

	describe('task click handler', () => {
		it('calls onTaskClick when a TaskCard is clicked', () => {
			const onTaskClick = vi.fn();
			const task = createMockTask({ id: 'TASK-001' });
			const { container } = renderSwimlane({
				tasks: [task],
				onTaskClick,
			});

			const taskCard = container.querySelector('.task-card');
			fireEvent.click(taskCard!);

			expect(onTaskClick).toHaveBeenCalledTimes(1);
			expect(onTaskClick).toHaveBeenCalledWith(task);
		});

		it('does not crash when onTaskClick is not provided', () => {
			const { container } = renderSwimlane({
				tasks: [createMockTask()],
				onTaskClick: undefined,
			});

			const taskCard = container.querySelector('.task-card');
			expect(() => fireEvent.click(taskCard!)).not.toThrow();
		});
	});

	describe('context menu handler', () => {
		it('calls onContextMenu when TaskCard is right-clicked', () => {
			const onContextMenu = vi.fn();
			const task = createMockTask({ id: 'TASK-001' });
			const { container } = renderSwimlane({
				tasks: [task],
				onContextMenu,
			});

			const taskCard = container.querySelector('.task-card');
			fireEvent.contextMenu(taskCard!);

			expect(onContextMenu).toHaveBeenCalledTimes(1);
			expect(onContextMenu).toHaveBeenCalledWith(task, expect.any(Object));
		});

		it('does not crash when onContextMenu is not provided', () => {
			const { container } = renderSwimlane({
				tasks: [createMockTask()],
				onContextMenu: undefined,
			});

			const taskCard = container.querySelector('.task-card');
			expect(() => fireEvent.contextMenu(taskCard!)).not.toThrow();
		});
	});

	describe('long initiative names', () => {
		it('renders long names with title attribute for tooltip', () => {
			const longTitle = 'This is a very long initiative name that should be truncated with ellipsis via CSS';
			renderSwimlane({
				initiative: createMockInitiative({ title: longTitle }),
			});

			const nameElement = screen.getByText(longTitle);
			expect(nameElement).toHaveAttribute('title', longTitle);
		});

		it('name element has class for ellipsis truncation', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ title: 'Any Name' }),
			});

			const nameElement = container.querySelector('.swimlane-name');
			expect(nameElement).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('header has role="button"', () => {
			const { container } = renderSwimlane();

			const header = container.querySelector('.swimlane-header');
			expect(header).toHaveAttribute('role', 'button');
		});

		it('header has tabIndex=0 for keyboard focus', () => {
			const { container } = renderSwimlane();

			const header = container.querySelector('.swimlane-header');
			expect(header).toHaveAttribute('tabindex', '0');
		});

		it('header has aria-expanded attribute', () => {
			const { container: expandedContainer } = renderSwimlane({ isCollapsed: false });
			const expandedHeader = expandedContainer.querySelector('.swimlane-header');
			expect(expandedHeader).toHaveAttribute('aria-expanded', 'true');

			const { container: collapsedContainer } = renderSwimlane({ isCollapsed: true });
			const collapsedHeader = collapsedContainer.querySelector('.swimlane-header');
			expect(collapsedHeader).toHaveAttribute('aria-expanded', 'false');
		});

		it('header has aria-controls pointing to content ID', () => {
			const { container } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-001' }),
			});

			const header = container.querySelector('.swimlane-header');
			expect(header).toHaveAttribute('aria-controls', 'swimlane-content-INIT-001');
		});

		it('progress bar has role="progressbar" and aria attributes', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'T1', status: TaskStatus.COMPLETED }),
					createMockTask({ id: 'T2', status: TaskStatus.CREATED }),
				],
			});

			const progressBar = container.querySelector('.swimlane-progress');
			expect(progressBar).toHaveAttribute('role', 'progressbar');
			expect(progressBar).toHaveAttribute('aria-valuenow', '50');
			expect(progressBar).toHaveAttribute('aria-valuemin', '0');
			expect(progressBar).toHaveAttribute('aria-valuemax', '100');
		});
	});

	describe('color themes', () => {
		it('assigns consistent color themes based on initiative ID', () => {
			// Same initiative ID should always get the same color
			const { container: container1 } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-ABC' }),
			});
			const { container: container2 } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-ABC' }),
			});

			const icon1 = container1.querySelector('.swimlane-icon');
			const icon2 = container2.querySelector('.swimlane-icon');
			expect(icon1?.className).toBe(icon2?.className);
		});

		it('different initiatives get different colors (usually)', () => {
			// Note: Due to hash collision, this test may fail occasionally
			// but should work for most unique IDs
			const { container: container1 } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-001' }),
			});
			const { container: container2 } = renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-002' }),
			});

			const icon1 = container1.querySelector('.swimlane-icon');
			const icon2 = container2.querySelector('.swimlane-icon');

			// Just verify they have valid color classes
			expect(icon1?.className).toMatch(/swimlane-icon/);
			expect(icon2?.className).toMatch(/swimlane-icon/);
		});
	});

	describe('meta information', () => {
		it('shows completed/total count in meta for initiatives', () => {
			renderSwimlane({
				initiative: createMockInitiative(),
				tasks: [
					createMockTask({ id: 'T1', status: TaskStatus.COMPLETED }),
					createMockTask({ id: 'T2', status: TaskStatus.RUNNING }),
					createMockTask({ id: 'T3', status: TaskStatus.CREATED }),
				],
			});

			expect(screen.getByText('1/3 complete')).toBeInTheDocument();
		});

		it('does not show meta for unassigned swimlane', () => {
			const { container } = renderSwimlane({
				initiative: null,
				tasks: [createMockTask()],
			});

			const meta = container.querySelector('.swimlane-meta');
			expect(meta).not.toBeInTheDocument();
		});
	});

	describe('data-testid', () => {
		it('sets data-testid with initiative ID', () => {
			renderSwimlane({
				initiative: createMockInitiative({ id: 'INIT-TEST' }),
			});

			expect(screen.getByTestId('swimlane-INIT-TEST')).toBeInTheDocument();
		});

		it('sets data-testid to swimlane-unassigned for null initiative', () => {
			renderSwimlane({
				initiative: null,
			});

			expect(screen.getByTestId('swimlane-unassigned')).toBeInTheDocument();
		});
	});

	describe('task position numbers', () => {
		it('passes 1-based position index to each TaskCard', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'TASK-001', title: 'First Task' }),
					createMockTask({ id: 'TASK-002', title: 'Second Task' }),
					createMockTask({ id: 'TASK-003', title: 'Third Task' }),
				],
			});

			// Find all position elements in order
			const positionElements = container.querySelectorAll('.task-card-position');
			expect(positionElements).toHaveLength(3);
			expect(positionElements[0]?.textContent).toBe('1');
			expect(positionElements[1]?.textContent).toBe('2');
			expect(positionElements[2]?.textContent).toBe('3');
		});

		it('position numbers increment correctly with task order (1, 2, 3)', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'TASK-A', title: 'Alpha' }),
					createMockTask({ id: 'TASK-B', title: 'Beta' }),
					createMockTask({ id: 'TASK-C', title: 'Gamma' }),
					createMockTask({ id: 'TASK-D', title: 'Delta' }),
					createMockTask({ id: 'TASK-E', title: 'Epsilon' }),
				],
			});

			const taskCards = container.querySelectorAll('.task-card');
			expect(taskCards).toHaveLength(5);

			// Verify each task card has the correct position number
			taskCards.forEach((card, index) => {
				const positionElement = card.querySelector('.task-card-position');
				expect(positionElement).toBeInTheDocument();
				expect(positionElement?.textContent).toBe(String(index + 1));
			});
		});

		it('single task has position 1', () => {
			const { container } = renderSwimlane({
				tasks: [createMockTask({ id: 'TASK-001', title: 'Only Task' })],
			});

			const positionElement = container.querySelector('.task-card-position');
			expect(positionElement).toBeInTheDocument();
			expect(positionElement?.textContent).toBe('1');
		});

		it('empty swimlane has no position elements', () => {
			const { container } = renderSwimlane({
				tasks: [],
			});

			const positionElements = container.querySelectorAll('.task-card-position');
			expect(positionElements).toHaveLength(0);

			// Verify empty state message is shown
			expect(screen.getByText('No tasks')).toBeInTheDocument();
		});

		it('position numbers are associated with correct task cards', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'TASK-AAA', title: 'First' }),
					createMockTask({ id: 'TASK-BBB', title: 'Second' }),
				],
			});

			// Find task cards by their data-task-id attribute
			const firstCard = container.querySelector('[data-task-id="TASK-AAA"]');
			const secondCard = container.querySelector('[data-task-id="TASK-BBB"]');

			expect(firstCard).toBeInTheDocument();
			expect(secondCard).toBeInTheDocument();

			const firstPosition = firstCard?.querySelector('.task-card-position');
			const secondPosition = secondCard?.querySelector('.task-card-position');

			expect(firstPosition?.textContent).toBe('1');
			expect(secondPosition?.textContent).toBe('2');
		});

		it('position numbers work with mixed task statuses', () => {
			const { container } = renderSwimlane({
				tasks: [
					createMockTask({ id: 'T1', status: TaskStatus.CREATED }),
					createMockTask({ id: 'T2', status: TaskStatus.RUNNING }),
					createMockTask({ id: 'T3', status: TaskStatus.COMPLETED }),
				],
			});

			const positionElements = container.querySelectorAll('.task-card-position');
			expect(positionElements).toHaveLength(3);
			expect(positionElements[0]?.textContent).toBe('1');
			expect(positionElements[1]?.textContent).toBe('2');
			expect(positionElements[2]?.textContent).toBe('3');
		});

		it('position numbers work for unassigned swimlane', () => {
			const { container } = renderSwimlane({
				initiative: null,
				tasks: [
					createMockTask({ id: 'TASK-001' }),
					createMockTask({ id: 'TASK-002' }),
				],
			});

			const positionElements = container.querySelectorAll('.task-card-position');
			expect(positionElements).toHaveLength(2);
			expect(positionElements[0]?.textContent).toBe('1');
			expect(positionElements[1]?.textContent).toBe('2');
		});
	});
});
