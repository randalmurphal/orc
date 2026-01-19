import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { OutcomesDonut } from './OutcomesDonut';

describe('OutcomesDonut', () => {
	describe('rendering', () => {
		it('renders with sample data', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={2} failed={1} />
			);

			// Check container exists
			expect(container.querySelector('.outcomes-donut-container')).toBeInTheDocument();

			// Check donut element exists
			expect(container.querySelector('.outcomes-donut')).toBeInTheDocument();

			// Check legend items exist
			expect(screen.getByText('Completed')).toBeInTheDocument();
			expect(screen.getByText('With Retries')).toBeInTheDocument();
			expect(screen.getByText('Failed')).toBeInTheDocument();
		});

		it('displays correct total (sum of all props)', () => {
			render(<OutcomesDonut completed={10} withRetries={2} failed={1} />);

			// Total should be 10 + 2 + 1 = 13
			expect(screen.getByText('13')).toBeInTheDocument();
			expect(screen.getByText('Total')).toBeInTheDocument();
		});

		it('displays individual counts in legend', () => {
			render(<OutcomesDonut completed={232} withRetries={11} failed={4} />);

			// Check each count appears (they're in legend)
			expect(screen.getByText('232')).toBeInTheDocument();
			expect(screen.getByText('11')).toBeInTheDocument();
			expect(screen.getByText('4')).toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('handles all-zero values without errors', () => {
			const { container } = render(
				<OutcomesDonut completed={0} withRetries={0} failed={0} />
			);

			// Should render with total of 0 (use specific selector to avoid duplicates)
			const totalValue = container.querySelector('.outcomes-donut-value');
			expect(totalValue).toHaveTextContent('0');

			// Donut should have neutral background (no gradient)
			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			expect(donut).toBeInTheDocument();
			expect(donut.style.background).toBe('var(--bg-surface)');
		});

		it('handles single-category (completed only)', () => {
			const { container } = render(
				<OutcomesDonut completed={5} withRetries={0} failed={0} />
			);

			// Should show total of 5 (use specific selector)
			const totalValue = container.querySelector('.outcomes-donut-value');
			expect(totalValue).toHaveTextContent('5');

			// Donut should be solid green (full circle)
			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			expect(donut.style.background).toBe('var(--green)');
		});

		it('handles single-category (withRetries only)', () => {
			const { container } = render(
				<OutcomesDonut completed={0} withRetries={3} failed={0} />
			);

			// Total should be 3 (use specific selector)
			const totalValue = container.querySelector('.outcomes-donut-value');
			expect(totalValue).toHaveTextContent('3');

			// Donut should be solid amber
			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			expect(donut.style.background).toBe('var(--amber)');
		});

		it('handles single-category (failed only)', () => {
			const { container } = render(
				<OutcomesDonut completed={0} withRetries={0} failed={7} />
			);

			// Total should be 7 (use specific selector)
			const totalValue = container.querySelector('.outcomes-donut-value');
			expect(totalValue).toHaveTextContent('7');

			// Donut should be solid red
			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			expect(donut.style.background).toBe('var(--red)');
		});
	});

	describe('legend', () => {
		it('shows all three categories with correct colors', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={2} failed={1} />
			);

			// Check colored dots exist with correct classes
			const completedDot = container.querySelector('.outcomes-donut-legend-dot--completed');
			const retriesDot = container.querySelector('.outcomes-donut-legend-dot--retries');
			const failedDot = container.querySelector('.outcomes-donut-legend-dot--failed');

			expect(completedDot).toBeInTheDocument();
			expect(retriesDot).toBeInTheDocument();
			expect(failedDot).toBeInTheDocument();
		});

		it('displays correct labels', () => {
			render(<OutcomesDonut completed={10} withRetries={2} failed={1} />);

			expect(screen.getByText('Completed')).toBeInTheDocument();
			expect(screen.getByText('With Retries')).toBeInTheDocument();
			expect(screen.getByText('Failed')).toBeInTheDocument();
		});
	});

	describe('conic-gradient', () => {
		it('uses conic-gradient for multiple categories', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={5} failed={5} />
			);

			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			expect(donut.style.background).toContain('conic-gradient');
		});

		it('includes all colors in gradient when all categories present', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={5} failed={5} />
			);

			const donut = container.querySelector('.outcomes-donut') as HTMLElement;
			const gradient = donut.style.background;

			expect(gradient).toContain('var(--green)');
			expect(gradient).toContain('var(--amber)');
			expect(gradient).toContain('var(--red)');
		});
	});

	describe('structure', () => {
		it('has center element for total display', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={2} failed={1} />
			);

			const center = container.querySelector('.outcomes-donut-center');
			expect(center).toBeInTheDocument();

			const value = container.querySelector('.outcomes-donut-value');
			expect(value).toBeInTheDocument();
			expect(value).toHaveTextContent('13');

			const label = container.querySelector('.outcomes-donut-label');
			expect(label).toBeInTheDocument();
			expect(label).toHaveTextContent('Total');
		});

		it('has legend below chart', () => {
			const { container } = render(
				<OutcomesDonut completed={10} withRetries={2} failed={1} />
			);

			const legend = container.querySelector('.outcomes-donut-legend');
			expect(legend).toBeInTheDocument();

			// Should have 3 legend items
			const items = container.querySelectorAll('.outcomes-donut-legend-item');
			expect(items).toHaveLength(3);
		});
	});
});
