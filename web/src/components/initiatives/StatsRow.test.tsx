import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import {
	StatsRow,
	StatCard,
	type InitiativeStats,
} from './StatsRow';

const defaultStats: InitiativeStats = {
	activeInitiatives: 0,
	totalTasks: 0,
	tasksThisWeek: 0,
	completionRate: 0,
	totalCost: 0,
};
import {
	formatNumber,
	formatCost,
	formatPercentage,
	formatTrend,
} from '@/lib/format';

// =============================================================================
// Utility Function Tests
// =============================================================================

describe('formatNumber', () => {
	it('formats millions with M suffix', () => {
		expect(formatNumber(1_500_000)).toBe('1.5M');
		expect(formatNumber(2_000_000)).toBe('2M');
		expect(formatNumber(10_000_000)).toBe('10M');
	});

	it('formats thousands with K suffix', () => {
		expect(formatNumber(1_500)).toBe('1.5K');
		expect(formatNumber(10_000)).toBe('10K');
		expect(formatNumber(847_000)).toBe('847K');
	});

	it('returns small numbers as-is', () => {
		expect(formatNumber(0)).toBe('0');
		expect(formatNumber(42)).toBe('42');
		expect(formatNumber(999)).toBe('999');
	});

	it('handles negative numbers', () => {
		expect(formatNumber(-5_000)).toBe('-5K');
		expect(formatNumber(-42)).toBe('-42');
	});
});

describe('formatCost', () => {
	it('formats dollar amounts with $ prefix', () => {
		expect(formatCost(47.82)).toBe('$47.82');
		expect(formatCost(0)).toBe('$0.00');
		expect(formatCost(100)).toBe('$100.00');
	});

	it('formats large costs with K suffix', () => {
		expect(formatCost(1_500)).toBe('$1.5K');
		expect(formatCost(10_000)).toBe('$10K');
	});

	it('formats very large costs with M suffix', () => {
		expect(formatCost(1_500_000)).toBe('$1.50M');
		expect(formatCost(2_000_000)).toBe('$2.00M');
	});
});

describe('formatPercentage', () => {
	it('formats percentage values with % suffix', () => {
		expect(formatPercentage(68)).toBe('68%');
		expect(formatPercentage(100)).toBe('100%');
		expect(formatPercentage(0)).toBe('0%');
	});

	it('rounds decimal percentages', () => {
		expect(formatPercentage(68.5)).toBe('69%');
		expect(formatPercentage(68.4)).toBe('68%');
	});
});

describe('formatTrend', () => {
	it('adds + prefix to positive numbers', () => {
		expect(formatTrend(5)).toBe('+5');
		expect(formatTrend(12)).toBe('+12');
	});

	it('keeps - prefix for negative numbers', () => {
		expect(formatTrend(-5)).toBe('-5');
		expect(formatTrend(-12)).toBe('-12');
	});

	it('handles zero', () => {
		expect(formatTrend(0)).toBe('0');
	});
});

// =============================================================================
// StatCard Component Tests
// =============================================================================

describe('StatCard', () => {
	describe('rendering', () => {
		it('renders an article element', () => {
			const { container } = render(<StatCard label="Test" value={42} />);
			const card = container.querySelector('article');
			expect(card).toBeInTheDocument();
			expect(card).toHaveClass('stats-row-card');
		});

		it('renders the label', () => {
			render(<StatCard label="Active Initiatives" value={3} />);
			expect(screen.getByText('Active Initiatives')).toBeInTheDocument();
		});

		it('renders numeric values', () => {
			render(<StatCard label="Count" value={42} />);
			expect(screen.getByText('42')).toBeInTheDocument();
		});

		it('renders string values', () => {
			render(<StatCard label="Cost" value="$47.82" />);
			expect(screen.getByText('$47.82')).toBeInTheDocument();
		});
	});

	describe('colors', () => {
		const colors = ['default', 'purple', 'green', 'amber', 'red'] as const;

		it.each(colors)('applies %s color variant', (color) => {
			const { container } = render(
				<StatCard label="Test" value={42} color={color} />
			);
			const value = container.querySelector('.stats-row-card-value');
			expect(value).toHaveClass(`stats-row-card-value-${color}`);
		});

		it('uses default color by default', () => {
			const { container } = render(<StatCard label="Test" value={42} />);
			const value = container.querySelector('.stats-row-card-value');
			expect(value).toHaveClass('stats-row-card-value-default');
		});
	});

	describe('trend indicator', () => {
		it('renders positive trend with up arrow', () => {
			const { container } = render(
				<StatCard label="Test" value={42} trend={5} />
			);
			const trend = container.querySelector('.stats-row-card-trend');
			expect(trend).toBeInTheDocument();
			expect(trend).toHaveClass('stats-row-card-trend-positive');
			expect(screen.getByText('+5')).toBeInTheDocument();
		});

		it('renders negative trend with down arrow', () => {
			const { container } = render(
				<StatCard label="Test" value={42} trend={-3} />
			);
			const trend = container.querySelector('.stats-row-card-trend');
			expect(trend).toBeInTheDocument();
			expect(trend).toHaveClass('stats-row-card-trend-negative');
			expect(screen.getByText('-3')).toBeInTheDocument();
		});

		it('does not render trend when undefined', () => {
			const { container } = render(<StatCard label="Test" value={42} />);
			const trend = container.querySelector('.stats-row-card-trend');
			expect(trend).not.toBeInTheDocument();
		});

		it('does not render trend when zero', () => {
			const { container } = render(
				<StatCard label="Test" value={42} trend={0} />
			);
			const trend = container.querySelector('.stats-row-card-trend');
			expect(trend).not.toBeInTheDocument();
		});
	});

	describe('loading state', () => {
		it('renders skeleton when loading', () => {
			const { container } = render(
				<StatCard label="Test" value={42} loading />
			);
			expect(container.querySelector('.stats-row-card-loading')).toBeInTheDocument();
			expect(container.querySelector('.stats-row-card-label-skeleton')).toBeInTheDocument();
			expect(container.querySelector('.stats-row-card-value-skeleton')).toBeInTheDocument();
		});

		it('has aria-busy when loading', () => {
			const { container } = render(<StatCard label="Test" value={42} loading />);
			const card = container.querySelector('article');
			expect(card).toHaveAttribute('aria-busy', 'true');
		});

		it('does not render value when loading', () => {
			render(<StatCard label="Test" value={42} loading />);
			expect(screen.queryByText('42')).not.toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has appropriate aria-label', () => {
			const { container } = render(<StatCard label="Total Tasks" value={71} />);
			const card = container.querySelector('article');
			expect(card).toHaveAttribute('aria-label', 'Total Tasks: 71');
		});

		it('trend has aria-live for screen reader announcements', () => {
			const { container } = render(
				<StatCard label="Test" value={42} trend={5} />
			);
			const trend = container.querySelector('.stats-row-card-trend');
			expect(trend).toHaveAttribute('aria-live', 'polite');
		});
	});
});

// =============================================================================
// StatsRow Component Tests
// =============================================================================

describe('StatsRow', () => {
	const mockStats: InitiativeStats = {
		activeInitiatives: 3,
		totalTasks: 71,
		tasksThisWeek: 12,
		completionRate: 68,
		totalCost: 47.82,
	};

	describe('rendering', () => {
		it('renders a div element with stats-row class', () => {
			const { container } = render(<StatsRow stats={mockStats} />);
			const row = container.querySelector('.stats-row');
			expect(row).toBeInTheDocument();
			expect(row?.tagName).toBe('DIV');
		});

		it('renders 4 stat cards', () => {
			const { container } = render(<StatsRow stats={mockStats} />);
			const cards = container.querySelectorAll('.stats-row-card');
			expect(cards).toHaveLength(4);
		});

		it('renders Active Initiatives card', () => {
			render(<StatsRow stats={mockStats} />);
			expect(screen.getByText('Active Initiatives')).toBeInTheDocument();
			expect(screen.getByText('3')).toBeInTheDocument();
		});

		it('renders Total Tasks card', () => {
			render(<StatsRow stats={mockStats} />);
			expect(screen.getByText('Total Tasks')).toBeInTheDocument();
			expect(screen.getByText('71')).toBeInTheDocument();
		});

		it('renders Completion Rate card', () => {
			render(<StatsRow stats={mockStats} />);
			expect(screen.getByText('Completion Rate')).toBeInTheDocument();
			expect(screen.getByText('68%')).toBeInTheDocument();
		});

		it('renders Total Cost card', () => {
			render(<StatsRow stats={mockStats} />);
			expect(screen.getByText('Total Cost')).toBeInTheDocument();
			expect(screen.getByText('$47.82')).toBeInTheDocument();
		});
	});

	describe('formatting', () => {
		it('formats large task counts', () => {
			render(
				<StatsRow
					stats={{
						...mockStats,
						totalTasks: 12500,
					}}
				/>
			);
			expect(screen.getByText('12.5K')).toBeInTheDocument();
		});

		it('formats large costs', () => {
			render(
				<StatsRow
					stats={{
						...mockStats,
						totalCost: 1500,
					}}
				/>
			);
			expect(screen.getByText('$1.5K')).toBeInTheDocument();
		});
	});

	describe('completion rate colors', () => {
		it('shows green for high completion rate (>=80%)', () => {
			const { container } = render(
				<StatsRow stats={{ ...mockStats, completionRate: 85 }} />
			);
			const values = container.querySelectorAll('.stats-row-card-value');
			// Completion rate is the 3rd card (index 2)
			expect(values[2]).toHaveClass('stats-row-card-value-green');
		});

		it('shows amber for medium completion rate (50-79%)', () => {
			const { container } = render(
				<StatsRow stats={{ ...mockStats, completionRate: 65 }} />
			);
			const values = container.querySelectorAll('.stats-row-card-value');
			expect(values[2]).toHaveClass('stats-row-card-value-amber');
		});

		it('shows red for low completion rate (<50%)', () => {
			const { container } = render(
				<StatsRow stats={{ ...mockStats, completionRate: 30 }} />
			);
			const values = container.querySelectorAll('.stats-row-card-value');
			expect(values[2]).toHaveClass('stats-row-card-value-red');
		});
	});

	describe('trends', () => {
		it('renders trends when provided', () => {
			render(
				<StatsRow
					stats={{
						...mockStats,
						trends: {
							initiatives: 1,
							tasks: 12,
							completionRate: 5,
							cost: -2,
						},
					}}
				/>
			);
			expect(screen.getByText('+1')).toBeInTheDocument();
			expect(screen.getByText('+12')).toBeInTheDocument();
			expect(screen.getByText('+5')).toBeInTheDocument();
			expect(screen.getByText('-2')).toBeInTheDocument();
		});
	});

	describe('loading state', () => {
		it('shows loading skeletons when loading', () => {
			const { container } = render(<StatsRow stats={mockStats} loading />);
			const skeletons = container.querySelectorAll('.stats-row-card-loading');
			expect(skeletons).toHaveLength(4);
		});

		it('does not render values when loading', () => {
			render(<StatsRow stats={mockStats} loading />);
			expect(screen.queryByText('3')).not.toBeInTheDocument();
			expect(screen.queryByText('71')).not.toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has region role with aria-label', () => {
			render(<StatsRow stats={mockStats} />);
			const region = screen.getByRole('region', {
				name: 'Initiative statistics overview',
			});
			expect(region).toBeInTheDocument();
		});

		it('each card is an article element', () => {
			const { container } = render(<StatsRow stats={mockStats} />);
			const articles = container.querySelectorAll('article');
			expect(articles).toHaveLength(4);
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<StatsRow ref={ref} stats={mockStats} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current?.tagName).toBe('DIV');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(
				<StatsRow stats={mockStats} className="custom-class" />
			);
			const row = container.querySelector('.stats-row');
			expect(row).toHaveClass('custom-class');
			expect(row).toHaveClass('stats-row');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(<StatsRow stats={mockStats} data-testid="test-stats-row" />);
			const row = screen.getByTestId('test-stats-row');
			expect(row).toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('handles zero values gracefully', () => {
			render(<StatsRow stats={defaultStats} />);
			// Active initiatives and Total tasks both show "0", so use getAllByText
			expect(screen.getAllByText('0')).toHaveLength(2);
			expect(screen.getByText('0%')).toBeInTheDocument(); // Completion rate
			expect(screen.getByText('$0.00')).toBeInTheDocument(); // Total cost
		});

		it('handles very large numbers', () => {
			render(
				<StatsRow
					stats={{
						...mockStats,
						totalTasks: 5_000_000,
						totalCost: 1_500_000,
					}}
				/>
			);
			expect(screen.getByText('5M')).toBeInTheDocument();
			expect(screen.getByText('$1.50M')).toBeInTheDocument();
		});
	});
});

// =============================================================================
// Default Stats Tests
// =============================================================================

describe('defaultStats', () => {
	it('has all required properties set to zero', () => {
		expect(defaultStats.activeInitiatives).toBe(0);
		expect(defaultStats.totalTasks).toBe(0);
		expect(defaultStats.tasksThisWeek).toBe(0);
		expect(defaultStats.completionRate).toBe(0);
		expect(defaultStats.totalCost).toBe(0);
	});

	it('can be used as initial state', () => {
		render(<StatsRow stats={defaultStats} />);
		// Should render without errors
		expect(
			screen.getByRole('region', { name: 'Initiative statistics overview' })
		).toBeInTheDocument();
	});
});
