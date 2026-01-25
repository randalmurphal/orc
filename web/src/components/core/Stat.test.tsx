import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import { Stat, type StatValueColor, type StatIconColor } from './Stat';
import { formatLargeNumber } from '@/lib/format';

describe('formatLargeNumber', () => {
	it('formats billions with B suffix', () => {
		expect(formatLargeNumber(1_234_567_890)).toBe('1.23B');
		expect(formatLargeNumber(5_000_000_000)).toBe('5B');
	});

	it('formats millions with M suffix', () => {
		expect(formatLargeNumber(1_234_567)).toBe('1.23M');
		expect(formatLargeNumber(2_400_000)).toBe('2.4M');
		expect(formatLargeNumber(5_000_000)).toBe('5M');
	});

	it('formats numbers >= 10K with K suffix', () => {
		expect(formatLargeNumber(847_000)).toBe('847K');
		expect(formatLargeNumber(10_000)).toBe('10K');
		expect(formatLargeNumber(127_500)).toBe('127.5K');
	});

	it('formats numbers between 1K and 10K with comma', () => {
		expect(formatLargeNumber(1_234)).toBe('1,234');
		expect(formatLargeNumber(9_999)).toBe('9,999');
	});

	it('returns small numbers as-is', () => {
		expect(formatLargeNumber(247)).toBe('247');
		expect(formatLargeNumber(0)).toBe('0');
		expect(formatLargeNumber(999)).toBe('999');
	});

	it('removes unnecessary trailing zeros', () => {
		expect(formatLargeNumber(1_000_000)).toBe('1M');
		expect(formatLargeNumber(2_500_000)).toBe('2.5M');
		expect(formatLargeNumber(1_200_000_000)).toBe('1.2B');
	});

	it('handles negative numbers', () => {
		expect(formatLargeNumber(-1_234_567)).toBe('-1.23M');
		expect(formatLargeNumber(-847_000)).toBe('-847K');
	});
});

describe('Stat', () => {
	describe('rendering', () => {
		it('renders a div element', () => {
			const { container } = render(<Stat label="Test" value={42} />);
			const stat = container.querySelector('.stat');
			expect(stat).toBeInTheDocument();
			expect(stat?.tagName).toBe('DIV');
		});

		it('renders the label', () => {
			render(<Stat label="Tasks Completed" value={247} />);
			expect(screen.getByText('Tasks Completed')).toBeInTheDocument();
		});

		it('renders numeric values', () => {
			render(<Stat label="Count" value={247} />);
			expect(screen.getByText('247')).toBeInTheDocument();
		});

		it('renders string values', () => {
			render(<Stat label="Cost" value="$47.82" />);
			expect(screen.getByText('$47.82')).toBeInTheDocument();
		});

		it('formats large numeric values', () => {
			render(<Stat label="Tokens" value={2_400_000} />);
			expect(screen.getByText('2.4M')).toBeInTheDocument();
		});
	});

	describe('null/undefined values', () => {
		it('shows placeholder dash for null value', () => {
			render(<Stat label="Test" value={null} />);
			expect(screen.getByText('—')).toBeInTheDocument();
		});

		it('shows placeholder dash for undefined value', () => {
			render(<Stat label="Test" value={undefined} />);
			expect(screen.getByText('—')).toBeInTheDocument();
		});

		it('applies placeholder class for null value', () => {
			const { container } = render(<Stat label="Test" value={null} />);
			const value = container.querySelector('.stat-value');
			expect(value).toHaveClass('stat-value-placeholder');
		});
	});

	describe('value colors', () => {
		const colors: StatValueColor[] = [
			'default',
			'purple',
			'green',
			'amber',
			'blue',
			'red',
			'cyan',
		];

		it.each(colors)('renders %s value color with correct class', (color) => {
			const { container } = render(
				<Stat label="Test" value={42} valueColor={color} />
			);
			const value = container.querySelector('.stat-value');
			expect(value).toHaveClass(`stat-value-${color}`);
		});

		it('uses default color by default', () => {
			const { container } = render(<Stat label="Test" value={42} />);
			const value = container.querySelector('.stat-value');
			expect(value).toHaveClass('stat-value-default');
		});
	});

	describe('icon', () => {
		const TestIcon = () => (
			<svg data-testid="test-icon">
				<circle cx="12" cy="12" r="10" />
			</svg>
		);

		it('renders icon when provided', () => {
			render(<Stat label="Test" value={42} icon={<TestIcon />} />);
			expect(screen.getByTestId('test-icon')).toBeInTheDocument();
		});

		it('does not render icon container when no icon provided', () => {
			const { container } = render(<Stat label="Test" value={42} />);
			const iconContainer = container.querySelector('.stat-icon');
			expect(iconContainer).not.toBeInTheDocument();
		});

		const iconColors: StatIconColor[] = [
			'default',
			'purple',
			'green',
			'amber',
			'blue',
			'red',
			'cyan',
		];

		it.each(iconColors)('renders %s icon color with correct class', (color) => {
			const { container } = render(
				<Stat label="Test" value={42} icon={<TestIcon />} iconColor={color} />
			);
			const icon = container.querySelector('.stat-icon');
			expect(icon).toHaveClass(`stat-icon-${color}`);
		});

		it('uses default icon color by default', () => {
			const { container } = render(
				<Stat label="Test" value={42} icon={<TestIcon />} />
			);
			const icon = container.querySelector('.stat-icon');
			expect(icon).toHaveClass('stat-icon-default');
		});
	});

	describe('trend indicator', () => {
		it('renders up trend with up arrow', () => {
			const { container } = render(
				<Stat
					label="Test"
					value={42}
					trend={{ direction: 'up', value: '+23%' }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toBeInTheDocument();
			expect(screen.getByText('+23%')).toBeInTheDocument();
			// Check for up arrow (polyline points="18 15 12 9 6 15")
			const svg = trend?.querySelector('svg');
			expect(svg).toBeInTheDocument();
			const polyline = svg?.querySelector('polyline');
			expect(polyline).toHaveAttribute('points', '18 15 12 9 6 15');
		});

		it('renders down trend with down arrow', () => {
			const { container } = render(
				<Stat
					label="Test"
					value={42}
					trend={{ direction: 'down', value: '-8%' }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toBeInTheDocument();
			expect(screen.getByText('-8%')).toBeInTheDocument();
			// Check for down arrow (polyline points="6 9 12 15 18 9")
			const svg = trend?.querySelector('svg');
			expect(svg).toBeInTheDocument();
			const polyline = svg?.querySelector('polyline');
			expect(polyline).toHaveAttribute('points', '6 9 12 15 18 9');
		});

		it('does not render trend when not provided', () => {
			const { container } = render(<Stat label="Test" value={42} />);
			const trend = container.querySelector('.stat-trend');
			expect(trend).not.toBeInTheDocument();
		});

		it('applies positive class for up trend by default', () => {
			const { container } = render(
				<Stat
					label="Test"
					value={42}
					trend={{ direction: 'up', value: '+23%' }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toHaveClass('stat-trend-positive');
		});

		it('applies negative class for down trend by default', () => {
			const { container } = render(
				<Stat
					label="Test"
					value={42}
					trend={{ direction: 'down', value: '-8%' }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toHaveClass('stat-trend-negative');
		});

		it('respects positive override when direction is down', () => {
			const { container } = render(
				<Stat
					label="Cost"
					value="$47.82"
					trend={{ direction: 'down', value: '-8% from last week', positive: true }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toHaveClass('stat-trend-positive');
		});

		it('respects positive override when direction is up', () => {
			const { container } = render(
				<Stat
					label="Bugs"
					value={15}
					trend={{ direction: 'up', value: '+5 more bugs', positive: false }}
				/>
			);
			const trend = container.querySelector('.stat-trend');
			expect(trend).toHaveClass('stat-trend-negative');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<Stat ref={ref} label="Test" value={42} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current?.tagName).toBe('DIV');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(
				<Stat label="Test" value={42} className="custom-class" />
			);
			const stat = container.querySelector('.stat');
			expect(stat).toHaveClass('custom-class');
			expect(stat).toHaveClass('stat');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Stat
					label="Test"
					value={42}
					className="class-a class-b"
					valueColor="purple"
				/>
			);
			const stat = container.querySelector('.stat');
			expect(stat).toHaveClass('stat');
			expect(stat).toHaveClass('class-a');
			expect(stat).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(
				<Stat
					label="Test"
					value={42}
					data-testid="test-stat"
					title="Test tooltip"
				/>
			);
			const stat = screen.getByTestId('test-stat');
			expect(stat).toHaveAttribute('title', 'Test tooltip');
		});

		it('supports aria attributes', () => {
			const { container } = render(
				<Stat label="Test" value={42} aria-label="Tasks completed: 42" />
			);
			const stat = container.querySelector('.stat');
			expect(stat).toHaveAttribute('aria-label', 'Tasks completed: 42');
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const TestIcon = () => <svg data-testid="icon" />;
			const { container } = render(
				<Stat
					label="Total Cost"
					value="$47.82"
					valueColor="green"
					icon={<TestIcon />}
					iconColor="green"
					trend={{ direction: 'down', value: '-8% from last week', positive: true }}
					className="custom"
					data-testid="test-stat"
				/>
			);
			const stat = container.querySelector('.stat');
			expect(stat).toHaveClass('stat');
			expect(stat).toHaveClass('custom');
			expect(stat).toHaveAttribute('data-testid', 'test-stat');

			expect(screen.getByText('Total Cost')).toBeInTheDocument();
			expect(screen.getByText('$47.82')).toBeInTheDocument();
			expect(screen.getByTestId('icon')).toBeInTheDocument();
			expect(screen.getByText('-8% from last week')).toBeInTheDocument();

			const value = container.querySelector('.stat-value');
			expect(value).toHaveClass('stat-value-green');

			const icon = container.querySelector('.stat-icon');
			expect(icon).toHaveClass('stat-icon-green');

			const trend = container.querySelector('.stat-trend');
			expect(trend).toHaveClass('stat-trend-positive');
		});
	});
});
