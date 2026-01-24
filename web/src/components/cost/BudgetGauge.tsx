/**
 * BudgetGauge component - displays budget status with progress bar and metrics.
 * Shows current spend vs limit with threshold-based color coding.
 */

import { Pencil } from 'lucide-react';
import { Button } from '../core/Button';
import './BudgetGauge.css';

export interface BudgetGaugeProps {
	/** Budget data to display */
	budget: {
		monthly_limit_usd: number;
		current_spent_usd: number;
		remaining_usd: number;
		percent_used: number;
		projected_monthly: number;
		days_remaining: number;
		on_track: boolean;
	};
	/** Optional callback when edit button is clicked */
	onEditLimit?: () => void;
}

/**
 * Formats a number as USD currency.
 */
function formatCurrency(value: number): string {
	return new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: 'USD',
		minimumFractionDigits: 2,
		maximumFractionDigits: 2,
	}).format(value);
}

/**
 * Gets the CSS class for the progress bar fill based on percent used.
 * Thresholds: 0-60% green, 61-80% warning, 81-100% danger, >100% over
 */
function getProgressClass(percentUsed: number): string {
	if (percentUsed > 100) return 'over';
	if (percentUsed > 80) return 'danger';
	if (percentUsed > 60) return 'warning';
	return '';
}

/**
 * BudgetGauge displays budget status with a visual progress bar.
 *
 * @example
 * <BudgetGauge
 *   budget={{
 *     monthly_limit_usd: 500,
 *     current_spent_usd: 234.50,
 *     remaining_usd: 265.50,
 *     percent_used: 47,
 *     projected_monthly: 480,
 *     days_remaining: 15,
 *     on_track: true,
 *   }}
 *   onEditLimit={() => console.log('Edit clicked')}
 * />
 */
export function BudgetGauge({ budget, onEditLimit }: BudgetGaugeProps) {
	const {
		monthly_limit_usd,
		current_spent_usd,
		remaining_usd,
		percent_used,
		projected_monthly,
		days_remaining,
		on_track,
	} = budget;

	// Cap the visual width at 100% but allow percent_used to show actual value
	const fillWidth = Math.min(percent_used, 100);
	const progressClass = getProgressClass(percent_used);
	const fillClasses = ['budget-fill', progressClass].filter(Boolean).join(' ');

	// Handle singular/plural for days
	const daysLabel = days_remaining === 1 ? 'day' : 'days';

	return (
		<div className="budget-gauge">
			<div className="budget-header">
				<h3 className="budget-title">Monthly Budget</h3>
				{onEditLimit && (
					<Button
						variant="ghost"
						size="sm"
						icon={Pencil}
						onClick={onEditLimit}
						aria-label="Edit budget limit"
					>
						Edit
					</Button>
				)}
			</div>

			<div className="budget-summary">
				<div className="budget-amount">
					<span className="budget-spent">{formatCurrency(current_spent_usd)}</span>
					<span className="budget-separator">/</span>
					<span className="budget-limit">{formatCurrency(monthly_limit_usd)}</span>
				</div>
				<span className="budget-percent">{percent_used}%</span>
			</div>

			<div className="budget-bar">
				<div
					className={fillClasses}
					style={{ width: `${fillWidth}%` }}
				/>
			</div>

			<div className="budget-details">
				<div className="budget-stat">
					<span className="budget-stat-label">Remaining</span>
					<span className="budget-stat-value">{formatCurrency(remaining_usd)}</span>
				</div>
				<div className="budget-stat">
					<span className="budget-stat-label">Projected</span>
					<span className="budget-stat-value">{formatCurrency(projected_monthly)}</span>
				</div>
				<div className="budget-stat">
					<span className="budget-stat-label">Time Left</span>
					<span className="budget-stat-value">{days_remaining} {daysLabel} remaining</span>
				</div>
				<div className="budget-stat">
					<span className="budget-stat-label">Status</span>
					<span className={`budget-status ${on_track ? 'on-track' : 'off-track'}`}>
						{on_track ? 'On Track' : 'Off Track'}
					</span>
				</div>
			</div>
		</div>
	);
}
