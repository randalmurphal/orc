/**
 * TasksBarChart Component Tests
 *
 * Tests for the TasksBarChart component including:
 * - Basic rendering
 * - Bar height scaling
 * - Edge cases (zero values, empty data)
 * - Loading state
 * - Accessibility
 * - Ref forwarding
 */

import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createRef } from 'react';
import {
	TasksBarChart,
	calculateBarHeight,
	defaultWeekData,
	type DayData,
} from './TasksBarChart';
import { TooltipProvider } from '../ui/Tooltip';

// Mock browser APIs not available in jsdom
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

// Helper to wrap with TooltipProvider
function renderWithProvider(ui: React.ReactElement) {
	return render(<TooltipProvider delayDuration={0}>{ui}</TooltipProvider>);
}

// =============================================================================
// Test Data
// =============================================================================

const mockWeekData: DayData[] = [
	{ day: 'Mon', count: 12 },
	{ day: 'Tue', count: 18 },
	{ day: 'Wed', count: 9 },
	{ day: 'Thu', count: 24 },
	{ day: 'Fri', count: 16 },
	{ day: 'Sat', count: 6 },
	{ day: 'Sun', count: 20 },
];

const allZeroData: DayData[] = [
	{ day: 'Mon', count: 0 },
	{ day: 'Tue', count: 0 },
	{ day: 'Wed', count: 0 },
	{ day: 'Thu', count: 0 },
	{ day: 'Fri', count: 0 },
	{ day: 'Sat', count: 0 },
	{ day: 'Sun', count: 0 },
];

// =============================================================================
// Utility Function Tests
// =============================================================================

describe('calculateBarHeight', () => {
	it('returns minimum height for zero count', () => {
		expect(calculateBarHeight(0, 100)).toBe(4);
		expect(calculateBarHeight(0, 0)).toBe(4);
	});

	it('scales height proportionally to max count', () => {
		// With max=100, count=50 should be ~50% of max height (140)
		const height = calculateBarHeight(50, 100);
		expect(height).toBe(70); // (50/100) * 140 = 70
	});

	it('returns max height for max count value', () => {
		const height = calculateBarHeight(100, 100);
		expect(height).toBe(140); // Full height
	});

	it('returns minimum height when calculated height is below minimum', () => {
		// Very small count relative to max
		const height = calculateBarHeight(1, 1000);
		expect(height).toBe(4); // Should be minimum since (1/1000)*140 = 0.14 < 4
	});

	it('handles max count of zero', () => {
		// When max is 0, should still return min height
		const height = calculateBarHeight(5, 0);
		expect(height).toBe(140); // 5/1 * 140 = 140 (maxCount becomes 1)
	});
});

// =============================================================================
// TasksBarChart Component Tests
// =============================================================================

describe('TasksBarChart', () => {
	describe('rendering', () => {
		it('renders a div element with tasks-bar-chart class', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toBeInTheDocument();
			expect(chart?.tagName).toBe('DIV');
		});

		it('renders 7 bar-group elements', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const groups = container.querySelectorAll('.tasks-bar-chart-group');
			expect(groups).toHaveLength(7);
		});

		it('each bar-group contains a bar and label', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const groups = container.querySelectorAll('.tasks-bar-chart-group');

			groups.forEach((group) => {
				expect(group.querySelector('.tasks-bar-chart-bar')).toBeInTheDocument();
				expect(group.querySelector('.tasks-bar-chart-label')).toBeInTheDocument();
			});
		});

		it('renders day labels correctly (Mon-Sun)', () => {
			renderWithProvider(<TasksBarChart data={mockWeekData} />);

			expect(screen.getByText('Mon')).toBeInTheDocument();
			expect(screen.getByText('Tue')).toBeInTheDocument();
			expect(screen.getByText('Wed')).toBeInTheDocument();
			expect(screen.getByText('Thu')).toBeInTheDocument();
			expect(screen.getByText('Fri')).toBeInTheDocument();
			expect(screen.getByText('Sat')).toBeInTheDocument();
			expect(screen.getByText('Sun')).toBeInTheDocument();
		});
	});

	describe('bar heights', () => {
		it('scales bar heights proportionally (tallest bar = max height)', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const bars = container.querySelectorAll('.tasks-bar-chart-bar');

			// Thu has max count (24), should have max height (140px)
			const thuBar = bars[3]; // Thu is index 3
			expect(thuBar).toHaveStyle({ height: '140px' });
		});

		it('zero values render with minimum height (4px)', () => {
			const dataWithZero: DayData[] = [
				{ day: 'Mon', count: 0 },
				{ day: 'Tue', count: 10 },
				{ day: 'Wed', count: 0 },
				{ day: 'Thu', count: 20 },
				{ day: 'Fri', count: 0 },
				{ day: 'Sat', count: 5 },
				{ day: 'Sun', count: 0 },
			];

			const { container } = renderWithProvider(
				<TasksBarChart data={dataWithZero} />
			);
			const bars = container.querySelectorAll('.tasks-bar-chart-bar');

			// Zero count bars should have minimum height
			expect(bars[0]).toHaveStyle({ height: '4px' }); // Mon
			expect(bars[2]).toHaveStyle({ height: '4px' }); // Wed
			expect(bars[4]).toHaveStyle({ height: '4px' }); // Fri
			expect(bars[6]).toHaveStyle({ height: '4px' }); // Sun
		});

		it('all-zero dataset renders all bars at minimum height', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={allZeroData} />
			);
			const bars = container.querySelectorAll('.tasks-bar-chart-bar');

			bars.forEach((bar) => {
				expect(bar).toHaveStyle({ height: '4px' });
			});
		});
	});

	describe('tooltip interaction', () => {
		it('shows tooltip with exact count on hover', async () => {
			const user = userEvent.setup();
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const bars = container.querySelectorAll('.tasks-bar-chart-bar');

			// Hover over first bar (Mon, 12 tasks)
			await user.hover(bars[0]);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			expect(screen.getAllByText('12 tasks').length).toBeGreaterThan(0);
		});

		it('shows singular "task" for count of 1', async () => {
			const user = userEvent.setup();
			const dataWithOne: DayData[] = [{ day: 'Mon', count: 1 }];

			const { container } = renderWithProvider(
				<TasksBarChart data={dataWithOne} />
			);
			const bars = container.querySelectorAll('.tasks-bar-chart-bar');

			await user.hover(bars[0]);

			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			expect(screen.getAllByText('1 task').length).toBeGreaterThan(0);
		});
	});

	describe('empty data', () => {
		it('handles empty data array gracefully', () => {
			const { container } = renderWithProvider(<TasksBarChart data={[]} />);

			expect(container.querySelector('.tasks-bar-chart')).toBeInTheDocument();
			expect(screen.getByText('No data available')).toBeInTheDocument();
		});

		it('does not render bars when data is empty', () => {
			const { container } = renderWithProvider(<TasksBarChart data={[]} />);

			expect(
				container.querySelectorAll('.tasks-bar-chart-bar')
			).toHaveLength(0);
		});
	});

	describe('loading state', () => {
		it('shows loading skeletons when loading', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			const skeletons = container.querySelectorAll(
				'.tasks-bar-chart-bar-skeleton'
			);
			expect(skeletons).toHaveLength(7);
		});

		it('has aria-busy when loading', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveAttribute('aria-busy', 'true');
		});

		it('does not render actual bars when loading', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			expect(
				container.querySelectorAll('.tasks-bar-chart-bar')
			).toHaveLength(0);
		});

		it('renders label skeletons when loading', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			const labelSkeletons = container.querySelectorAll(
				'.tasks-bar-chart-label-skeleton'
			);
			expect(labelSkeletons).toHaveLength(7);
		});

		it('skeleton heights are deterministic across renders', () => {
			const { container, rerender } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			const firstRenderHeights = Array.from(
				container.querySelectorAll('.tasks-bar-chart-bar-skeleton')
			).map((el) => (el as HTMLElement).style.height);

			// Rerender and check heights are the same
			rerender(
				<TooltipProvider delayDuration={0}>
					<TasksBarChart data={mockWeekData} loading />
				</TooltipProvider>
			);
			const secondRenderHeights = Array.from(
				container.querySelectorAll('.tasks-bar-chart-bar-skeleton')
			).map((el) => (el as HTMLElement).style.height);

			expect(firstRenderHeights).toEqual(secondRenderHeights);
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			renderWithProvider(<TasksBarChart ref={ref} data={mockWeekData} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current?.tagName).toBe('DIV');
		});

		it('ref points to the container element', () => {
			const ref = createRef<HTMLDivElement>();
			renderWithProvider(<TasksBarChart ref={ref} data={mockWeekData} />);
			expect(ref.current).toHaveClass('tasks-bar-chart');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} className="custom-class" />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveClass('custom-class');
			expect(chart).toHaveClass('tasks-bar-chart');
		});

		it('preserves base class when custom class is added', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} className="my-chart" />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveClass('tasks-bar-chart');
			expect(chart).toHaveClass('my-chart');
		});
	});

	describe('accessibility', () => {
		it('has role="img" for chart container', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveAttribute('role', 'img');
		});

		it('has aria-label with chart data', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveAttribute(
				'aria-label',
				'Tasks per day chart showing Mon: 12, Tue: 18, Wed: 9, Thu: 24, Fri: 16, Sat: 6, Sun: 20'
			);
		});

		it('loading state has descriptive aria-label', () => {
			const { container } = renderWithProvider(
				<TasksBarChart data={mockWeekData} loading />
			);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveAttribute(
				'aria-label',
				'Tasks per day chart loading'
			);
		});

		it('empty state has descriptive aria-label', () => {
			const { container } = renderWithProvider(<TasksBarChart data={[]} />);
			const chart = container.querySelector('.tasks-bar-chart');
			expect(chart).toHaveAttribute(
				'aria-label',
				'Tasks per day chart - no data'
			);
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			renderWithProvider(
				<TasksBarChart data={mockWeekData} data-testid="test-chart" />
			);
			const chart = screen.getByTestId('test-chart');
			expect(chart).toBeInTheDocument();
		});

		it('applies id attribute', () => {
			renderWithProvider(
				<TasksBarChart data={mockWeekData} id="my-bar-chart" />
			);
			expect(document.getElementById('my-bar-chart')).toBeInTheDocument();
		});
	});
});

// =============================================================================
// Default Data Tests
// =============================================================================

describe('defaultWeekData', () => {
	it('has 7 days', () => {
		expect(defaultWeekData).toHaveLength(7);
	});

	it('has correct day names in order', () => {
		expect(defaultWeekData[0].day).toBe('Mon');
		expect(defaultWeekData[1].day).toBe('Tue');
		expect(defaultWeekData[2].day).toBe('Wed');
		expect(defaultWeekData[3].day).toBe('Thu');
		expect(defaultWeekData[4].day).toBe('Fri');
		expect(defaultWeekData[5].day).toBe('Sat');
		expect(defaultWeekData[6].day).toBe('Sun');
	});

	it('has all counts initialized to zero', () => {
		defaultWeekData.forEach((day) => {
			expect(day.count).toBe(0);
		});
	});

	it('can be used as initial state', () => {
		renderWithProvider(<TasksBarChart data={defaultWeekData} />);
		// Should render without errors
		expect(screen.getByRole('img')).toBeInTheDocument();
	});
});
