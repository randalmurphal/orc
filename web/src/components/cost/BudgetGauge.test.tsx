import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { BudgetGauge, type BudgetGaugeProps } from './BudgetGauge';

const createBudget = (overrides: Partial<BudgetGaugeProps['budget']> = {}): BudgetGaugeProps['budget'] => ({
	monthly_limit_usd: 500,
	current_spent_usd: 234.5,
	remaining_usd: 265.5,
	percent_used: 47,
	projected_monthly: 480,
	days_remaining: 15,
	on_track: true,
	...overrides,
});

describe('BudgetGauge', () => {
	// =========================================================================
	// SC-1: Displays budget data correctly
	// =========================================================================
	describe('budget data display', () => {
		it('displays current spent and monthly limit as currency', () => {
			render(<BudgetGauge budget={createBudget()} />);
			expect(screen.getByText(/\$234\.50/)).toBeInTheDocument();
			expect(screen.getByText(/\$500\.00/)).toBeInTheDocument();
		});

		it('displays remaining amount', () => {
			render(<BudgetGauge budget={createBudget({ remaining_usd: 265.5 })} />);
			expect(screen.getByText(/\$265\.50/)).toBeInTheDocument();
		});

		it('displays projected monthly spend', () => {
			render(<BudgetGauge budget={createBudget({ projected_monthly: 480 })} />);
			expect(screen.getByText(/\$480\.00/)).toBeInTheDocument();
		});

		it('displays days remaining', () => {
			render(<BudgetGauge budget={createBudget({ days_remaining: 15 })} />);
			expect(screen.getByText(/15 days/i)).toBeInTheDocument();
		});

		it('shows on track indicator when on_track is true', () => {
			render(<BudgetGauge budget={createBudget({ on_track: true })} />);
			expect(screen.getByText(/on track/i)).toBeInTheDocument();
		});

		it('shows off track indicator when on_track is false', () => {
			render(<BudgetGauge budget={createBudget({ on_track: false })} />);
			expect(screen.getByText(/off track/i)).toBeInTheDocument();
		});

		it('displays percentage used', () => {
			render(<BudgetGauge budget={createBudget({ percent_used: 47 })} />);
			expect(screen.getByText(/47%/)).toBeInTheDocument();
		});

		it('renders header title', () => {
			render(<BudgetGauge budget={createBudget()} />);
			expect(screen.getByText(/monthly budget/i)).toBeInTheDocument();
		});
	});

	// =========================================================================
	// SC-2: Progress bar with threshold-based colors
	// =========================================================================
	describe('progress bar colors', () => {
		it('renders progress bar with correct fill width', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 47 })} />);
			const fill = container.querySelector('.budget-fill') as HTMLElement;
			expect(fill).toBeInTheDocument();
			expect(fill.style.width).toBe('47%');
		});

		it('uses green color for 0-60% usage', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 30 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).not.toHaveClass('warning');
			expect(fill).not.toHaveClass('danger');
			expect(fill).not.toHaveClass('over');
		});

		it('uses green color at exactly 60%', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 60 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).not.toHaveClass('warning');
			expect(fill).not.toHaveClass('danger');
		});

		it('uses amber/warning color for 61-80% usage', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 70 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).toHaveClass('warning');
			expect(fill).not.toHaveClass('danger');
			expect(fill).not.toHaveClass('over');
		});

		it('uses amber/warning color at exactly 80%', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 80 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).toHaveClass('warning');
			expect(fill).not.toHaveClass('danger');
		});

		it('uses red/danger color for 81-100% usage', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 90 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).toHaveClass('danger');
			expect(fill).not.toHaveClass('warning');
			expect(fill).not.toHaveClass('over');
		});

		it('uses red/danger color at exactly 100%', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 100 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).toHaveClass('danger');
			expect(fill).not.toHaveClass('over');
		});

		it('uses red with pulse animation (over class) when >100%', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 120 })} />);
			const fill = container.querySelector('.budget-fill');
			expect(fill).toHaveClass('over');
		});

		it('caps fill width at 100% when percent_used exceeds 100', () => {
			const { container } = render(<BudgetGauge budget={createBudget({ percent_used: 150 })} />);
			const fill = container.querySelector('.budget-fill') as HTMLElement;
			expect(fill.style.width).toBe('100%');
		});

		it('renders budget bar container', () => {
			const { container } = render(<BudgetGauge budget={createBudget()} />);
			expect(container.querySelector('.budget-bar')).toBeInTheDocument();
		});
	});

	// =========================================================================
	// SC-3: Edit button behavior
	// =========================================================================
	describe('edit button', () => {
		it('renders Edit button when onEditLimit is provided', () => {
			const onEditLimit = vi.fn();
			render(<BudgetGauge budget={createBudget()} onEditLimit={onEditLimit} />);
			expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
		});

		it('does not render Edit button when onEditLimit is undefined', () => {
			render(<BudgetGauge budget={createBudget()} />);
			expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument();
		});

		it('calls onEditLimit when Edit button is clicked', () => {
			const onEditLimit = vi.fn();
			render(<BudgetGauge budget={createBudget()} onEditLimit={onEditLimit} />);
			const editButton = screen.getByRole('button', { name: /edit/i });
			fireEvent.click(editButton);
			expect(onEditLimit).toHaveBeenCalledTimes(1);
		});
	});

	// =========================================================================
	// Edge cases
	// =========================================================================
	describe('edge cases', () => {
		it('handles zero budget gracefully', () => {
			const { container } = render(
				<BudgetGauge
					budget={createBudget({
						monthly_limit_usd: 0,
						current_spent_usd: 0,
						remaining_usd: 0,
						percent_used: 0,
						projected_monthly: 0,
					})}
				/>
			);
			expect(container.querySelector('.budget-gauge')).toBeInTheDocument();
			const fill = container.querySelector('.budget-fill') as HTMLElement;
			expect(fill.style.width).toBe('0%');
		});

		it('handles very large values', () => {
			render(
				<BudgetGauge
					budget={createBudget({
						monthly_limit_usd: 99999.99,
						current_spent_usd: 12345.67,
						remaining_usd: 87654.32,
						percent_used: 12,
						projected_monthly: 25000.5,
					})}
				/>
			);
			expect(screen.getByText(/\$12,?345\.67/)).toBeInTheDocument();
		});

		it('handles 1 day remaining', () => {
			render(<BudgetGauge budget={createBudget({ days_remaining: 1 })} />);
			expect(screen.getByText(/1 day/i)).toBeInTheDocument();
		});
	});
});
