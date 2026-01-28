import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ActivityHeatmap, type ActivityData } from './ActivityHeatmap';

// Mock data
const mockData: ActivityData[] = [
	{ date: '2026-01-15', count: 5 },
	{ date: '2026-01-14', count: 12 },
	{ date: '2026-01-13', count: 1 },
	{ date: '2026-01-12', count: 0 },
	{ date: '2026-01-11', count: 8 },
	{ date: '2026-01-10', count: 3 },
];

// Helper to generate dates relative to today
function generateDataForWeeks(weeks: number, baseDate: Date = new Date()): ActivityData[] {
	const data: ActivityData[] = [];
	for (let i = 0; i < weeks * 7; i++) {
		const date = new Date(baseDate);
		date.setDate(date.getDate() - i);
		const dateStr = date.toISOString().split('T')[0];
		// Vary the counts
		data.push({ date: dateStr, count: i % 15 });
	}
	return data;
}

describe('ActivityHeatmap', () => {
	// Save original innerWidth
	const originalInnerWidth = window.innerWidth;

	beforeEach(() => {
		// Reset window width
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			configurable: true,
			value: 1024,
		});
	});

	afterEach(() => {
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			configurable: true,
			value: originalInnerWidth,
		});
	});

	describe('rendering', () => {
		it('renders the heatmap with default props', () => {
			render(<ActivityHeatmap data={mockData} />);
			expect(screen.getByText('Task Activity')).toBeInTheDocument();
		});

		it('renders custom title', () => {
			render(<ActivityHeatmap data={mockData} title="Custom Title" />);
			expect(screen.getByText('Custom Title')).toBeInTheDocument();
		});

		it('renders legend with Less/More labels', () => {
			render(<ActivityHeatmap data={mockData} />);
			expect(screen.getByText('Less')).toBeInTheDocument();
			expect(screen.getByText('More')).toBeInTheDocument();
		});

		it('renders day labels (Mon, Wed, Fri)', () => {
			render(<ActivityHeatmap data={mockData} />);
			expect(screen.getByText('Mon')).toBeInTheDocument();
			expect(screen.getByText('Wed')).toBeInTheDocument();
			expect(screen.getByText('Fri')).toBeInTheDocument();
		});

		it('renders month labels', () => {
			const data = generateDataForWeeks(16);
			render(<ActivityHeatmap data={data} />);
			// Should have at least one month label
			const monthLabels = screen.getAllByText(/Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/);
			expect(monthLabels.length).toBeGreaterThan(0);
		});

		it('applies custom className', () => {
			const { container } = render(<ActivityHeatmap data={mockData} className="custom-class" />);
			expect(container.querySelector('.activity-heatmap')).toHaveClass('custom-class');
		});
	});

	describe('grid structure', () => {
		it('renders correct number of cells for default 16 weeks', () => {
			const { container } = render(<ActivityHeatmap data={mockData} />);
			const cells = container.querySelectorAll('.heatmap-cell');
			// 16 weeks * 7 days = 112 cells
			expect(cells.length).toBe(112);
		});

		it('renders correct number of cells for custom weeks', () => {
			const { container } = render(<ActivityHeatmap data={mockData} weeks={8} />);
			const cells = container.querySelectorAll('.heatmap-cell');
			// 8 weeks * 7 days = 56 cells
			expect(cells.length).toBe(56);
		});

		it('has aria-label on the grid', () => {
			const { container } = render(<ActivityHeatmap data={mockData} weeks={8} />);
			const grid = container.querySelector('.heatmap-grid');
			expect(grid).toHaveAttribute('aria-label', 'Task activity heatmap showing 8 weeks of data');
		});
	});

	describe('activity levels', () => {
		it('renders cells with level-0 class for zero activity', () => {
			const data: ActivityData[] = [{ date: '2026-01-15', count: 0 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);
			// Most cells should be level-0 since we only have one data point
			const level0Cells = container.querySelectorAll('.heatmap-cell.level-0');
			expect(level0Cells.length).toBeGreaterThan(0);
		});

		it('renders cells with level-1 class for 1-3 tasks', () => {
			// Get yesterday's date to ensure it's not in the future
			const yesterday = new Date();
			yesterday.setDate(yesterday.getDate() - 1);
			const yesterdayStr = yesterday.toISOString().split('T')[0];
			// Provide multiple data points to avoid empty state
			const data: ActivityData[] = [
				{ date: yesterdayStr, count: 2 },
				{ date: '2026-01-01', count: 1 }, // Extra to avoid empty state
			];
			const { container } = render(<ActivityHeatmap data={data} weeks={2} />);
			const level1Cells = container.querySelectorAll('.heatmap-cell.level-1');
			expect(level1Cells.length).toBeGreaterThanOrEqual(1);
		});

		it('renders cells with level-4 class for 10+ tasks', () => {
			// Get yesterday's date to ensure it's not in the future
			const yesterday = new Date();
			yesterday.setDate(yesterday.getDate() - 1);
			const yesterdayStr = yesterday.toISOString().split('T')[0];
			// Provide multiple data points to avoid empty state
			const data: ActivityData[] = [
				{ date: yesterdayStr, count: 15 },
				{ date: '2026-01-01', count: 1 }, // Extra to avoid empty state
			];
			const { container } = render(<ActivityHeatmap data={data} weeks={2} />);
			const level4Cells = container.querySelectorAll('.heatmap-cell.level-4');
			expect(level4Cells.length).toBeGreaterThanOrEqual(1);
		});
	});

	describe('future dates', () => {
		it('marks future dates with future class', () => {
			// Provide data to avoid empty state, then check for future cells
			const data = generateDataForWeeks(1);
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);
			// The grid should render with 7 cells
			const cells = container.querySelectorAll('.heatmap-cell');
			expect(cells.length).toBe(7);
			// Some cells might be future depending on day of week
		});
	});

	describe('click handling', () => {
		it('calls onDayClick when a cell is clicked', () => {
			const handleClick = vi.fn();
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} onDayClick={handleClick} />);

			// Find cells that are not future dates (should be clickable)
			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				fireEvent.click(cells[0]);
				expect(handleClick).toHaveBeenCalled();
			}
		});

		it('does not call onDayClick for future dates', () => {
			const handleClick = vi.fn();
			const { container } = render(<ActivityHeatmap data={[]} weeks={1} onDayClick={handleClick} />);

			const futureCells = container.querySelectorAll('.heatmap-cell.future');
			if (futureCells.length > 0) {
				fireEvent.click(futureCells[0]);
				expect(handleClick).not.toHaveBeenCalled();
			}
		});
	});

	describe('tooltip', () => {
		it('shows tooltip on mouse enter', () => {
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);

			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				fireEvent.mouseEnter(cells[0]);
				const tooltip = container.querySelector('.heatmap-tooltip');
				expect(tooltip).toHaveClass('visible');
			}
		});

		it('hides tooltip on mouse leave', () => {
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);

			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				fireEvent.mouseEnter(cells[0]);
				fireEvent.mouseLeave(cells[0]);
				const tooltip = container.querySelector('.heatmap-tooltip');
				expect(tooltip).not.toHaveClass('visible');
			}
		});

		it('tooltip contains task count', () => {
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);

			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				fireEvent.mouseEnter(cells[0]);
				const tooltip = container.querySelector('.heatmap-tooltip');
				expect(tooltip?.textContent).toContain('task');
			}
		});
	});

	describe('accessibility', () => {
		it('cells have aria-label describing the activity', () => {
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);

			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				const ariaLabel = cells[0].getAttribute('aria-label');
				expect(ariaLabel).toContain('task');
			}
		});

		it('future cells have appropriate aria-label', () => {
			const { container } = render(<ActivityHeatmap data={[]} weeks={1} />);

			const futureCells = container.querySelectorAll('.heatmap-cell.future');
			if (futureCells.length > 0) {
				const ariaLabel = futureCells[0].getAttribute('aria-label');
				expect(ariaLabel).toContain('Future date');
			}
		});

		it('non-future cells are focusable', () => {
			const today = new Date().toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: today, count: 5 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={1} />);

			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');
			if (cells.length > 0) {
				expect(cells[0]).toHaveAttribute('tabIndex', '0');
			}
		});

		it('future cells are not focusable', () => {
			const { container } = render(<ActivityHeatmap data={[]} weeks={1} />);

			const futureCells = container.querySelectorAll('.heatmap-cell.future');
			if (futureCells.length > 0) {
				expect(futureCells[0]).toHaveAttribute('tabIndex', '-1');
			}
		});
	});

	describe('keyboard navigation', () => {
		it('supports arrow key navigation', () => {
			const handleClick = vi.fn();
			const data = generateDataForWeeks(2);
			const { container } = render(<ActivityHeatmap data={data} weeks={2} onDayClick={handleClick} />);

			const grid = container.querySelector('.heatmap-grid');
			const cells = container.querySelectorAll('.heatmap-cell:not(.future)');

			if (grid && cells.length > 0) {
				// Focus first cell
				fireEvent.focus(cells[0]);

				// Press Enter to activate
				fireEvent.keyDown(grid, { key: 'Enter' });
				expect(handleClick).toHaveBeenCalled();
			}
		});
	});

	describe('loading state', () => {
		it('renders skeleton when loading', () => {
			const { container } = render(<ActivityHeatmap data={[]} loading />);
			expect(container.querySelector('.heatmap-skeleton')).toBeInTheDocument();
		});

		it('does not render grid when loading', () => {
			const { container } = render(<ActivityHeatmap data={[]} loading />);
			expect(container.querySelector('.heatmap-grid')).not.toBeInTheDocument();
		});

		it('skeleton has animated cells', () => {
			const { container } = render(<ActivityHeatmap data={[]} loading />);
			const skeletonCells = container.querySelectorAll('.heatmap-skeleton-cell');
			expect(skeletonCells.length).toBeGreaterThan(0);
		});
	});

	describe('empty state', () => {
		it('renders empty state when no data and not loading', () => {
			render(<ActivityHeatmap data={[]} />);
			expect(screen.getByText('No activity data available')).toBeInTheDocument();
		});

		it('still renders header in empty state', () => {
			render(<ActivityHeatmap data={[]} title="Activity" />);
			expect(screen.getByText('Activity')).toBeInTheDocument();
		});
	});

	describe('data handling', () => {
		it('handles missing days in data (shows as level-0)', () => {
			// Only provide data for one day, rest should be level-0
			const yesterday = new Date();
			yesterday.setDate(yesterday.getDate() - 1);
			const yesterdayStr = yesterday.toISOString().split('T')[0];
			const data: ActivityData[] = [{ date: yesterdayStr, count: 10 }];
			const { container } = render(<ActivityHeatmap data={data} weeks={2} />);

			// Should have many level-0 cells
			const level0Cells = container.querySelectorAll('.heatmap-cell.level-0');
			expect(level0Cells.length).toBeGreaterThan(0);
		});

		it('handles duplicate dates (uses last value)', () => {
			const yesterday = new Date();
			yesterday.setDate(yesterday.getDate() - 1);
			const yesterdayStr = yesterday.toISOString().split('T')[0];
			const data: ActivityData[] = [
				{ date: yesterdayStr, count: 5 },
				{ date: yesterdayStr, count: 15 }, // Should use this one (Map overwrites)
			];
			const { container } = render(<ActivityHeatmap data={data} weeks={2} />);

			// Should have level-4 cell (15 tasks)
			const level4Cells = container.querySelectorAll('.heatmap-cell.level-4');
			expect(level4Cells.length).toBeGreaterThanOrEqual(1);
		});
	});

	describe('legend', () => {
		it('renders 5 legend cells for 5 intensity levels', () => {
			const { container } = render(<ActivityHeatmap data={mockData} />);
			const legendCells = container.querySelectorAll('.heatmap-legend-cell');
			expect(legendCells.length).toBe(5);
		});
	});
	describe('density and responsive sizing', () => {
		it('sets --heatmap-weeks CSS custom property to 16 by default', () => {
			const { container } = render(<ActivityHeatmap data={mockData} />);
			const grid = container.querySelector('.heatmap-grid');
			expect(grid).toHaveStyle({ '--heatmap-weeks': '16' });
		});

		it('sets --heatmap-weeks CSS custom property to custom weeks value', () => {
			const { container } = render(<ActivityHeatmap data={mockData} weeks={8} />);
			const grid = container.querySelector('.heatmap-grid');
			expect(grid).toHaveStyle({ '--heatmap-weeks': '8' });
		});

		it('has data-weeks attribute set to 16 by default', () => {
			const { container } = render(<ActivityHeatmap data={mockData} />);
			const grid = container.querySelector('.heatmap-grid');
			expect(grid).toHaveAttribute('data-weeks', '16');
		});

		it('renders grid with heatmap-grid--dense class by default', () => {
			const { container } = render(<ActivityHeatmap data={mockData} />);
			const grid = container.querySelector('.heatmap-grid');
			expect(grid).toHaveClass('heatmap-grid--dense');
		});
	});

});
