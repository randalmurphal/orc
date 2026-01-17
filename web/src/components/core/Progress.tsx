/**
 * Progress component - horizontal progress bars with color variants.
 * Displays a visual indicator of progress with customizable colors and sizes.
 */

import { forwardRef, useEffect, useState, type HTMLAttributes } from 'react';
import './Progress.css';

export type ProgressColor = 'purple' | 'green' | 'amber' | 'blue';
export type ProgressSize = 'sm' | 'md';

export interface ProgressProps extends Omit<HTMLAttributes<HTMLDivElement>, 'color'> {
	/** Current progress value */
	value: number;
	/** Maximum value (default 100) */
	max?: number;
	/** Color variant */
	color?: ProgressColor;
	/** Show percentage label */
	showLabel?: boolean;
	/** Size variant */
	size?: ProgressSize;
}

/**
 * Progress component for displaying horizontal progress bars.
 *
 * @example
 * // Basic progress bar
 * <Progress value={50} />
 *
 * @example
 * // With color variant
 * <Progress value={75} color="green" />
 *
 * @example
 * // With label
 * <Progress value={30} max={100} showLabel />
 *
 * @example
 * // Small size
 * <Progress value={60} size="sm" />
 */
export const Progress = forwardRef<HTMLDivElement, ProgressProps>(
	(
		{ value, max = 100, color = 'purple', showLabel = false, size = 'md', className = '', ...props },
		ref
	) => {
		// Track if component has mounted for initial animation
		const [mounted, setMounted] = useState(false);

		useEffect(() => {
			// Trigger animation on next frame after mount
			const frame = requestAnimationFrame(() => {
				setMounted(true);
			});
			return () => cancelAnimationFrame(frame);
		}, []);

		// Clamp value between 0 and max, then convert to percentage
		const clampedValue = Math.min(Math.max(value, 0), max);
		const percentage = max > 0 ? (clampedValue / max) * 100 : 0;

		const classes = [
			'progress',
			`progress-${size}`,
			`progress-${color}`,
			showLabel && 'progress-with-label',
			!mounted && 'progress-animate',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<div
				ref={ref}
				className={classes}
				role="progressbar"
				aria-valuenow={clampedValue}
				aria-valuemin={0}
				aria-valuemax={max}
				{...props}
			>
				<div className="progress-track">
					<div className="progress-fill" style={{ width: `${percentage}%` }} />
				</div>
				{showLabel && <span className="progress-label">{Math.round(percentage)}%</span>}
			</div>
		);
	}
);

Progress.displayName = 'Progress';
