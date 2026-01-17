/**
 * Stat component - displays large numeric values with labels and trend indicators.
 * Used for dashboard statistics like task counts, token usage, costs, etc.
 */

import { forwardRef, type HTMLAttributes, type ReactNode } from 'react';
import './Stat.css';

export type StatValueColor =
	| 'default'
	| 'purple'
	| 'green'
	| 'amber'
	| 'blue'
	| 'red'
	| 'cyan';

export type StatIconColor =
	| 'default'
	| 'purple'
	| 'green'
	| 'amber'
	| 'blue'
	| 'red'
	| 'cyan';

export interface StatTrend {
	/** Direction of the trend: 'up' or 'down' */
	direction: 'up' | 'down';
	/** Value to display (e.g., "+23%", "-8%", "+12 this week") */
	value: string;
	/** Whether this trend is positive (green) or negative (red).
	 * Defaults to: up = positive, down = negative.
	 * Use this to override (e.g., cost reduction where down is good) */
	positive?: boolean;
}

export interface StatProps extends HTMLAttributes<HTMLDivElement> {
	/** Label text shown above the value */
	label: string;
	/** The value to display. Can be a number (will be formatted) or string (displayed as-is) */
	value: number | string | null | undefined;
	/** Color variant for the value text */
	valueColor?: StatValueColor;
	/** Trend indicator with direction and value */
	trend?: StatTrend;
	/** Icon element to display in the header */
	icon?: ReactNode;
	/** Color variant for the icon background */
	iconColor?: StatIconColor;
}

/**
 * Formats large numbers into abbreviated form.
 * Examples: 1234567 -> '1.23M', 847000 -> '847K', 1234 -> '1,234'
 */
export function formatLargeNumber(value: number): string {
	const absValue = Math.abs(value);

	if (absValue >= 1_000_000_000) {
		const formatted = (value / 1_000_000_000).toFixed(2);
		// Remove trailing zeros after decimal
		const cleaned = formatted.replace(/\.?0+$/, '');
		return `${cleaned}B`;
	}

	if (absValue >= 1_000_000) {
		const formatted = (value / 1_000_000).toFixed(2);
		const cleaned = formatted.replace(/\.?0+$/, '');
		return `${cleaned}M`;
	}

	if (absValue >= 1_000) {
		// For values >= 10K, use K abbreviation
		if (absValue >= 10_000) {
			const formatted = (value / 1_000).toFixed(1);
			const cleaned = formatted.replace(/\.?0+$/, '');
			return `${cleaned}K`;
		}
		// For values between 1K and 10K, use comma formatting
		return value.toLocaleString('en-US');
	}

	// Small numbers: just return as string
	return value.toString();
}

/**
 * Up arrow icon for positive trends
 */
function TrendUpIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<polyline points="18 15 12 9 6 15" />
		</svg>
	);
}

/**
 * Down arrow icon for negative trends
 */
function TrendDownIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<polyline points="6 9 12 15 18 9" />
		</svg>
	);
}

/**
 * Stat component for displaying dashboard statistics.
 *
 * @example
 * // Basic stat with value
 * <Stat label="Tasks Completed" value={247} />
 *
 * @example
 * // Stat with colored value and icon
 * <Stat
 *   label="Total Cost"
 *   value="$47.82"
 *   valueColor="green"
 *   icon={<DollarIcon />}
 *   iconColor="green"
 * />
 *
 * @example
 * // Stat with trend indicator
 * <Stat
 *   label="Success Rate"
 *   value="94.2%"
 *   trend={{ direction: 'up', value: '+2.1% improvement' }}
 * />
 *
 * @example
 * // Stat where down trend is positive (cost reduction)
 * <Stat
 *   label="Total Cost"
 *   value="$47.82"
 *   trend={{ direction: 'down', value: '-8% from last week', positive: true }}
 * />
 */
export const Stat = forwardRef<HTMLDivElement, StatProps>(
	(
		{
			label,
			value,
			valueColor = 'default',
			trend,
			icon,
			iconColor = 'default',
			className = '',
			...props
		},
		ref
	) => {
		// Format the display value
		const displayValue =
			value === null || value === undefined
				? '—'
				: typeof value === 'number'
					? formatLargeNumber(value)
					: value;

		const isPlaceholder = value === null || value === undefined;

		// Determine trend color: positive prop overrides default behavior
		const trendIsPositive =
			trend?.positive !== undefined
				? trend.positive
				: trend?.direction === 'up';

		const classes = ['stat', className].filter(Boolean).join(' ');

		const valueClasses = [
			'stat-value',
			isPlaceholder ? 'stat-value-placeholder' : `stat-value-${valueColor}`,
		].join(' ');

		const iconClasses = ['stat-icon', `stat-icon-${iconColor}`].join(' ');

		const trendClasses = [
			'stat-trend',
			trendIsPositive ? 'stat-trend-positive' : 'stat-trend-negative',
		].join(' ');

		return (
			<div ref={ref} className={classes} {...props}>
				<div className="stat-header">
					<span className="stat-label">{label}</span>
					{icon && <div className={iconClasses}>{icon}</div>}
				</div>

				<div className={valueClasses}>{displayValue}</div>

				{trend && (
					<div className={trendClasses}>
						{trend.direction === 'up' ? <TrendUpIcon /> : <TrendDownIcon />}
						<span>{trend.value}</span>
					</div>
				)}
			</div>
		);
	}
);

Stat.displayName = 'Stat';
